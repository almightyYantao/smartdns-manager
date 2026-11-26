package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"smartdns-manager/config"
	"smartdns-manager/models"
)

// SchedulerService 定时任务调度服务
type SchedulerService struct {
	db        *gorm.DB
	config    *config.Config
	cron      *cron.Cron
	s3        *S3Service
	mutex     sync.RWMutex
	running   bool
	taskExecs map[uint]context.CancelFunc // 正在执行的任务

	// 子服务
	dbBackup     *DatabaseBackupService
	nodeBackup   *NodeBackupService
	logCleanup   *LogCleanupService
	telemetry    *TelemetryService
	customScript *CustomScriptService
}

// NewSchedulerService 创建调度服务
func NewSchedulerService(db *gorm.DB, config *config.Config, s3 *S3Service) (*SchedulerService, error) {
	scheduler := &SchedulerService{
		db:        db,
		config:    config,
		s3:        s3,
		cron:      cron.New(cron.WithSeconds()),
		taskExecs: make(map[uint]context.CancelFunc),
	}

	// 初始化子服务
	dbBackupService := NewDatabaseBackupService(db, s3)
	scheduler.dbBackup = dbBackupService

	nodeBackupService, err := NewNodeBackupService(db, config, s3)
	if err != nil {
		return nil, fmt.Errorf("初始化节点备份服务失败: %w", err)
	}
	scheduler.nodeBackup = nodeBackupService

	logCleanupService, err := NewLogCleanupService(db, config)
	if err != nil {
		return nil, fmt.Errorf("初始化日志清理服务失败: %w", err)
	}
	scheduler.logCleanup = logCleanupService

	telemetryService, err := NewTelemetryService(db, config)
	if err != nil {
		return nil, fmt.Errorf("初始化遥测服务失败: %w", err)
	}
	scheduler.telemetry = telemetryService

	customScriptService, err := NewCustomScriptService(db, config)
	if err != nil {
		return nil, fmt.Errorf("初始化自定义脚本服务失败: %w", err)
	}
	scheduler.customScript = customScriptService

	return scheduler, nil
}

// Start 启动调度服务
func (s *SchedulerService) Start() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("调度服务已经在运行")
	}

	// 初始化默认任务
	if err := s.initializeDefaultTasks(); err != nil {
		log.Printf("⚠️ 初始化默认任务失败: %v", err)
		// 不返回错误，允许系统继续启动
	}

	if err := s.migrateDNSProbeSchedule(); err != nil {
		return fmt.Errorf("迁移 DNS 拨测计划失败: %w", err)
	}

	// 加载并注册所有任务
	if err := s.loadTasks(); err != nil {
		return fmt.Errorf("加载任务失败: %w", err)
	}

	s.cron.Start()
	s.running = true

	log.Printf("✅ 定时任务调度服务启动成功")
	return nil
}

// Stop 停止调度服务
func (s *SchedulerService) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	// 停止cron调度器
	s.cron.Stop()

	// 取消所有正在执行的任务
	for taskID, cancel := range s.taskExecs {
		log.Printf("🛑 取消正在执行的任务: %d", taskID)
		cancel()
	}

	s.running = false
	log.Printf("🛑 定时任务调度服务已停止")
}

// loadTasks 加载所有启用的任务
func (s *SchedulerService) loadTasks() error {
	if _, err := s.cron.AddFunc("0 0 3 * * *", s.telemetry.pruneDNSResults); err != nil {
		return err
	}

	var tasks []models.ScheduledTask
	if err := s.db.Where("enabled = ?", true).Find(&tasks).Error; err != nil {
		return fmt.Errorf("查询任务失败: %w", err)
	}

	log.Printf("🔄 开始加载定时任务，共找到 %d 个启用的任务", len(tasks))

	successCount := 0
	for _, task := range tasks {
		if err := s.addTaskToCron(task); err != nil {
			log.Printf("❌ 添加任务失败 [%s]: %v", task.Name, err)
			continue
		}
		log.Printf("📅 添加定时任务: %s (%s)", task.Name, task.CronExpr)
		successCount++
	}

	log.Printf("✅ 任务加载完成: 成功 %d/%d", successCount, len(tasks))
	return nil
}

// addTaskToCron 添加任务到cron调度器
func (s *SchedulerService) addTaskToCron(task models.ScheduledTask) error {
	schedule, err := taskScheduleParser.Parse(task.CronExpr)
	if err != nil {
		return err
	}
	s.cron.Schedule(schedule, cron.FuncJob(func() {
		next := schedule.Next(time.Now())
		pending := s.db.Model(&models.ScheduledTask{}).Where("id = ? AND enabled = ? AND cron_expr = ? AND config = ?", task.ID, true, task.CronExpr, task.Config).UpdateColumn("next_run_at", next)
		if pending.Error == nil && pending.RowsAffected > 0 {
			s.executeTask(task)
		}
	}))
	s.db.Model(&models.ScheduledTask{}).Where("id = ?", task.ID).UpdateColumn("next_run_at", schedule.Next(time.Now()))

	return nil
}

// executeTask 执行任务
func (s *SchedulerService) executeTask(task models.ScheduledTask) {
	s.mutex.Lock()
	// 检查任务是否已在执行
	if _, exists := s.taskExecs[task.ID]; exists {
		log.Printf("⚠️ 任务 [%s] 正在执行中，跳过本次调度", task.Name)
		s.mutex.Unlock()
		return
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	s.taskExecs[task.ID] = cancel
	s.mutex.Unlock()

	// 执行完成后清理
	defer func() {
		s.mutex.Lock()
		delete(s.taskExecs, task.ID)
		s.mutex.Unlock()
	}()

	// 创建执行记录
	execution := &models.TaskExecution{
		TaskID:    task.ID,
		Status:    models.TaskStatusRunning,
		StartedAt: time.Now(),
	}

	if err := s.db.Create(execution).Error; err != nil {
		log.Printf("❌ 创建任务执行记录失败 [%s]: %v", task.Name, err)
		return
	}

	log.Printf("🚀 开始执行任务: %s", task.Name)

	// 执行具体任务
	var err error
	var output string

	switch task.Type {
	case models.TaskTypeDBBackup:
		output, err = s.executeDBBackup(ctx, task)
	case models.TaskTypeNodeBackup:
		output, err = s.executeNodeBackup(ctx, task)
	case models.TaskTypeLogCleanup:
		output, err = s.executeLogCleanup(ctx, task)
	case models.TaskTypeTelemetry:
		output, err = s.executeTelemetry(ctx, task)
	case models.TaskTypeCustomScript:
		output, err = s.executeCustomScript(ctx, task)
	default:
		err = fmt.Errorf("未知的任务类型: %s", task.Type)
	}

	// 更新执行记录
	endTime := time.Now()
	duration := endTime.Sub(execution.StartedAt).Milliseconds()

	updates := map[string]interface{}{
		"ended_at": &endTime,
		"duration": duration,
		"output":   output,
	}

	if err != nil {
		updates["status"] = models.TaskStatusFailed
		updates["error"] = err.Error()
		log.Printf("❌ 任务执行失败 [%s]: %v", task.Name, err)
	} else {
		updates["status"] = models.TaskStatusSuccess
		log.Printf("✅ 任务执行成功 [%s]: 耗时%dms", task.Name, duration)
	}

	// 更新执行记录
	if err := s.db.Model(execution).Updates(updates).Error; err != nil {
		log.Printf("❌ 更新任务执行记录失败: %v", err)
	}

	// 更新任务状态
	taskUpdates := map[string]interface{}{
		"last_run_at": &endTime,
		"last_status": updates["status"],
		"run_count":   gorm.Expr("run_count + 1"),
	}

	if err == nil {
		taskUpdates["success_count"] = gorm.Expr("success_count + 1")
		taskUpdates["last_error"] = ""
	} else {
		taskUpdates["last_error"] = err.Error()
	}

	if err := s.db.Model(&task).Updates(taskUpdates).Error; err != nil {
		log.Printf("❌ 更新任务状态失败: %v", err)
	}
}

// executeDBBackup 执行数据库备份任务
func (s *SchedulerService) executeDBBackup(ctx context.Context, task models.ScheduledTask) (string, error) {
	var config models.DBBackupConfig
	if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
		return "", fmt.Errorf("解析任务配置失败: %w", err)
	}

	// 创建备份配置
	backupConfig := models.BackupConfig{
		Name:               fmt.Sprintf("scheduled_backup_%s", time.Now().Format("20060102_150405")),
		Enabled:            true,
		BackupType:         "database",
		Schedule:           task.CronExpr,
		RetentionDays:      config.RetentionDays,
		S3Enabled:          true,
		S3AccessKey:        config.S3Config.AccessKey,
		S3SecretKey:        config.S3Config.SecretKey,
		S3Region:           config.S3Config.Region,
		S3Bucket:           config.S3Config.Bucket,
		S3Endpoint:         config.S3Config.Endpoint,
		S3Prefix:           config.S3Config.Prefix,
		CompressionEnabled: config.Compression,
		EncryptionEnabled:  config.Encryption,
	}

	// 创建备份历史记录
	history := &models.BackupHistory{
		ConfigID:   0, // 临时配置，无需保存到数据库
		BackupType: "database",
		Status:     "running",
		StartedAt:  time.Now(),
	}

	// 执行备份
	err := s.dbBackup.performBackup(ctx, &backupConfig, history)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("数据库备份成功: %s (大小: %d bytes)", history.FileName, history.FileSize), nil
}

// executeNodeBackup 执行节点备份任务
func (s *SchedulerService) executeNodeBackup(ctx context.Context, task models.ScheduledTask) (string, error) {
	var config models.NodeBackupConfig
	if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
		return "", fmt.Errorf("解析任务配置失败: %w", err)
	}

	return s.nodeBackup.BackupNodes(ctx, config)
}

// executeLogCleanup 执行日志清理任务
func (s *SchedulerService) executeLogCleanup(ctx context.Context, task models.ScheduledTask) (string, error) {
	var config models.LogCleanupConfig
	if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
		return "", fmt.Errorf("解析任务配置失败: %w", err)
	}

	return s.logCleanup.CleanupLogs(ctx, config)
}

// executeTelemetry 执行遥测任务
func (s *SchedulerService) executeTelemetry(ctx context.Context, task models.ScheduledTask) (string, error) {
	var config models.TelemetryConfig
	if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
		return "", fmt.Errorf("解析任务配置失败: %w", err)
	}

	return s.telemetry.CheckTargets(ctx, config)
}

// executeCustomScript 执行自定义脚本任务
func (s *SchedulerService) executeCustomScript(ctx context.Context, task models.ScheduledTask) (string, error) {
	var config models.CustomScriptConfig
	if err := json.Unmarshal([]byte(task.Config), &config); err != nil {
		return "", fmt.Errorf("解析任务配置失败: %w", err)
	}

	// 验证脚本配置
	if err := s.customScript.ValidateScript(config); err != nil {
		return "", fmt.Errorf("脚本配置验证失败: %w", err)
	}

	return s.customScript.ExecuteScript(ctx, config)
}

// ReloadTasks 重新加载任务
func (s *SchedulerService) ReloadTasks() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return fmt.Errorf("调度服务未运行")
	}

	// 停止当前调度器
	s.cron.Stop()

	// 创建新的调度器
	s.cron = cron.New(cron.WithSeconds())

	// 重新加载任务
	if err := s.loadTasks(); err != nil {
		return err
	}

	// 启动新调度器
	s.cron.Start()

	log.Printf("🔄 定时任务重新加载完成")
	return nil
}

// GetTaskStats 获取任务统计信息
func (s *SchedulerService) GetTaskStats() (*models.TaskStats, error) {
	stats := &models.TaskStats{}

	// 统计任务数量
	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Model(&models.ScheduledTask{}).Count(&stats.TotalTasks).Error; err != nil {
		return nil, fmt.Errorf("统计总任务数失败: %w", err)
	}

	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Model(&models.ScheduledTask{}).Where("enabled = ?", true).Count(&stats.EnabledTasks).Error; err != nil {
		return nil, fmt.Errorf("统计启用任务数失败: %w", err)
	}

	// 统计执行记录
	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Model(&models.TaskExecution{}).Count(&stats.TotalExecutions).Error; err != nil {
		return nil, fmt.Errorf("统计总执行数失败: %w", err)
	}

	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Model(&models.TaskExecution{}).Where("status = ?", models.TaskStatusSuccess).Count(&stats.SuccessExecutions).Error; err != nil {
		return nil, fmt.Errorf("统计成功执行数失败: %w", err)
	}

	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Model(&models.TaskExecution{}).Where("status = ?", models.TaskStatusFailed).Count(&stats.FailedExecutions).Error; err != nil {
		return nil, fmt.Errorf("统计失败执行数失败: %w", err)
	}

	// 计算成功率
	if stats.TotalExecutions > 0 {
		stats.SuccessRate = float64(stats.SuccessExecutions) / float64(stats.TotalExecutions) * 100
	}

	// 获取最近执行和下次执行时间
	var lastExecution models.TaskExecution
	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Order("started_at DESC").First(&lastExecution).Error; err == nil {
		stats.LastExecutionAt = &lastExecution.StartedAt
	}

	var nextTask models.ScheduledTask
	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Where("enabled = ? AND next_run_at > ?", true, time.Now()).
		Order("next_run_at ASC").First(&nextTask).Error; err == nil {
		stats.NextExecutionAt = nextTask.NextRunAt
	}

	// 统计正在运行的任务数
	s.mutex.RLock()
	stats.RunningTasks = int64(len(s.taskExecs))
	s.mutex.RUnlock()

	return stats, nil
}

// GetRunningTasks 获取正在运行的任务列表
func (s *SchedulerService) GetRunningTasks() []uint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	taskIDs := make([]uint, 0, len(s.taskExecs))
	for taskID := range s.taskExecs {
		taskIDs = append(taskIDs, taskID)
	}

	return taskIDs
}

// GetDB 获取数据库连接（用于handler）
func (s *SchedulerService) GetDB() *gorm.DB {
	return s.db
}

// GetTelemetryService 获取遥测服务（用于handler）
func (s *SchedulerService) GetTelemetryService() *TelemetryService {
	return s.telemetry
}

// GetCustomScriptService 获取自定义脚本服务（用于handler）
func (s *SchedulerService) GetCustomScriptService() *CustomScriptService {
	return s.customScript
}

// CreateTask 创建任务
func (s *SchedulerService) CreateTask(task *models.ScheduledTask) error {
	if _, err := taskScheduleParser.Parse(task.CronExpr); err != nil {
		return err
	}
	if err := s.db.Create(task).Error; err != nil {
		return err
	}

	// 如果启用，重新加载任务
	if task.Enabled && s.running {
		return s.ReloadTasks()
	}

	return nil
}

// UpdateTask 更新任务
func (s *SchedulerService) UpdateTask(task *models.ScheduledTask) error {
	if _, err := taskScheduleParser.Parse(task.CronExpr); err != nil {
		return err
	}
	if !task.Enabled {
		task.NextRunAt = nil
	}
	if err := s.db.Save(task).Error; err != nil {
		return err
	}

	// 重新加载任务
	if s.running {
		return s.ReloadTasks()
	}

	return nil
}

// DeleteTask 删除任务
func (s *SchedulerService) DeleteTask(taskID uint) error {
	if err := s.db.Delete(&models.ScheduledTask{}, taskID).Error; err != nil {
		return err
	}

	// 重新加载任务
	if s.running {
		return s.ReloadTasks()
	}

	return nil
}

// GetTask 获取单个任务
func (s *SchedulerService) GetTask(taskID uint) (*models.ScheduledTask, error) {
	var task models.ScheduledTask
	if err := s.db.First(&task, taskID).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// GetTasks 获取任务列表
func (s *SchedulerService) GetTasks(offset, limit int, taskType, status string) ([]models.ScheduledTask, int64, error) {
	var tasks []models.ScheduledTask
	var total int64

	query := s.db.Model(&models.ScheduledTask{})

	if taskType != "" {
		query = query.Where("type = ?", taskType)
	}
	if status != "" {
		if status == "enabled" {
			query = query.Where("enabled = ?", true)
		} else if status == "disabled" {
			query = query.Where("enabled = ?", false)
		}
	}

	query.Count(&total)

	err := query.Offset(offset).Limit(limit).Order("created_at DESC").Find(&tasks).Error
	return tasks, total, err
}

// GetTaskExecutions 获取任务执行历史
func (s *SchedulerService) GetTaskExecutions(taskID uint, offset, limit int) ([]models.TaskExecution, int64, error) {
	var executions []models.TaskExecution
	var total int64

	query := s.db.Model(&models.TaskExecution{}).Where("task_id = ?", taskID)
	query.Count(&total)

	err := query.Preload("Task").Offset(offset).Limit(limit).Order("started_at DESC").Find(&executions).Error
	return executions, total, err
}

// ExecuteTaskManually 手动执行任务
func (s *SchedulerService) ExecuteTaskManually(task models.ScheduledTask) error {
	log.Printf("🔧 手动执行任务: %s (ID: %d)", task.Name, task.ID)

	// 检查任务是否已在执行
	s.mutex.RLock()
	if _, exists := s.taskExecs[task.ID]; exists {
		s.mutex.RUnlock()
		return fmt.Errorf("任务正在执行中")
	}
	s.mutex.RUnlock()

	// 在后台执行任务
	go s.executeTask(task)
	return nil
}

// initializeDefaultTasks 初始化默认任务
func (s *SchedulerService) initializeDefaultTasks() error {
	// 创建默认节点备份任务
	if err := s.createDefaultNodeBackupTask(); err != nil {
		log.Printf("⚠️ 创建默认节点备份任务失败: %v", err)
	}
	
	// 创建默认日志清理任务
	if err := s.createDefaultLogCleanupTask(); err != nil {
		log.Printf("⚠️ 创建默认日志清理任务失败: %v", err)
	}
	
	return nil
}

// createDefaultNodeBackupTask 创建默认节点备份任务
func (s *SchedulerService) createDefaultNodeBackupTask() error {
	// 检查是否已存在默认节点备份任务
	var count int64
	if err := s.db.Model(&models.ScheduledTask{}).
		Where("name = ? AND type = ?", "默认节点备份", models.TaskTypeNodeBackup).
		Count(&count).Error; err != nil {
		return fmt.Errorf("检查默认节点备份任务失败: %w", err)
	}
	
	// 如果已存在，则跳过创建
	if count > 0 {
		log.Printf("📋 默认节点备份任务已存在，跳过创建")
		return nil
	}
	
	// 创建默认节点备份任务配置
	defaultConfig := models.NodeBackupConfig{
		StorageType:   "local",
		LocalPath:     "/etc/smartdns/backups",
		NodeIDs:       []uint{},
		BackupConfigs: true,
		BackupLogs:    false,
		Compression:   true,
		RetentionDays: 30,
	}
	
	// 序列化配置
	configJSON, err := json.Marshal(defaultConfig)
	if err != nil {
		return fmt.Errorf("序列化默认节点备份配置失败: %w", err)
	}
	
	// 创建默认任务
	defaultTask := &models.ScheduledTask{
		Name:        "默认节点备份",
		Type:        models.TaskTypeNodeBackup,
		Description: "系统默认创建的节点备份任务，每天凌晨3点自动执行",
		CronExpr:    "0 0 3 * * *", // 每天凌晨3点执行
		Config:      string(configJSON),
		Enabled:     true,
	}
	
	if err := s.db.Create(defaultTask).Error; err != nil {
		return fmt.Errorf("创建默认节点备份任务失败: %w", err)
	}
	
	log.Printf("✅ 已创建默认节点备份任务 (ID: %d)", defaultTask.ID)
	return nil
}

// createDefaultLogCleanupTask 创建默认日志清理任务
func (s *SchedulerService) createDefaultLogCleanupTask() error {
	// 检查是否已存在默认日志清理任务
	var count int64
	if err := s.db.Model(&models.ScheduledTask{}).
		Where("name = ? AND type = ?", "默认SmartDNS日志清理", models.TaskTypeLogCleanup).
		Count(&count).Error; err != nil {
		return fmt.Errorf("检查默认日志清理任务失败: %w", err)
	}
	
	// 如果已存在，则跳过创建
	if count > 0 {
		log.Printf("📋 默认日志清理任务已存在，跳过创建")
		return nil
	}
	
	// 创建默认日志清理任务配置
	defaultConfig := models.LogCleanupConfig{
		AgentLogDays:    7,  // agent日志保留7天
		BackendLogDays:  7,  // backend日志保留7天
		SmartDNSLogDays: 30, // SmartDNS日志保留30天
		LogPaths:        []string{}, // 使用默认路径
	}
	
	// 序列化配置
	configJSON, err := json.Marshal(defaultConfig)
	if err != nil {
		return fmt.Errorf("序列化默认日志清理配置失败: %w", err)
	}
	
	// 创建默认任务
	defaultTask := &models.ScheduledTask{
		Name:        "默认SmartDNS日志清理",
		Type:        models.TaskTypeLogCleanup,
		Description: "系统默认创建的SmartDNS日志清理任务，每天凌晨2点自动执行，保留30天内的日志",
		CronExpr:    "0 0 2 * * *", // 每天凌晨2点执行
		Config:      string(configJSON),
		Enabled:     true,
	}
	
	if err := s.db.Create(defaultTask).Error; err != nil {
		return fmt.Errorf("创建默认日志清理任务失败: %w", err)
	}
	
	log.Printf("✅ 已创建默认SmartDNS日志清理任务 (ID: %d)", defaultTask.ID)
	return nil
}
