
package services

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"compress/flate"

	"github.com/robfig/cron/v3"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"smartdns-manager/config"
	"smartdns-manager/models"
)

type DatabaseBackupService struct {
	db          *gorm.DB
	s3Service   *S3Service
	cron        *cron.Cron
	activeJobs  map[uint]cron.EntryID // 配置ID -> cron任务ID的映射
	config      *config.Config
}

func NewDatabaseBackupService(db *gorm.DB, s3Service *S3Service) *DatabaseBackupService {
	return &DatabaseBackupService{
		db:          db,
		s3Service:   s3Service,
		cron:        cron.New(cron.WithParser(cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor))),
		activeJobs:  make(map[uint]cron.EntryID),
		config:      config.GetConfig(),
	}
}

// Start 启动备份服务
func (s *DatabaseBackupService) Start() error {
	s.cron.Start()
	
	// 加载所有启用的备份配置并创建定时任务
	var configs []models.BackupConfig
	if err := s.db.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		return fmt.Errorf("failed to load backup configs: %w", err)
	}

	for _, cfg := range configs {
		if err := s.scheduleBackup(&cfg); err != nil {
			fmt.Printf("Failed to schedule backup for config %d: %v\n", cfg.ID, err)
		}
	}

	return nil
}

// Stop 停止备份服务
func (s *DatabaseBackupService) Stop() {
	s.cron.Stop()
}

// scheduleBackup 调度备份任务
func (s *DatabaseBackupService) scheduleBackup(config *models.BackupConfig) error {
	// 如果已经有任务在运行，先移除
	if jobID, exists := s.activeJobs[config.ID]; exists {
		s.cron.Remove(jobID)
	}

	// 创建新的定时任务
	jobID, err := s.cron.AddFunc(config.Schedule, func() {
		s.executeBackup(config.ID)
	})
	if err != nil {
		return fmt.Errorf("failed to schedule backup: %w", err)
	}

	s.activeJobs[config.ID] = jobID

	// 更新下次执行时间
	nextTime := s.cron.Entry(jobID).Next
	config.NextBackupAt = &nextTime
	s.db.Save(config)

	return nil
}

// executeBackup 执行备份
func (s *DatabaseBackupService) executeBackup(configID uint) {
	ctx := context.Background()
	
	// 获取配置
	var config models.BackupConfig
	if err := s.db.First(&config, configID).Error; err != nil {
		fmt.Printf("Failed to load backup config %d: %v\n", configID, err)
		return
	}

	// 创建备份历史记录
	history := &models.BackupHistory{
		ConfigID:    configID,
		BackupType:  config.BackupType,
		Status:      "running",
		StartedAt:   time.Now(),
	}
	if err := s.db.Create(history).Error; err != nil {
		fmt.Printf("Failed to create backup history: %v\n", err)
		return
	}

	// 执行备份
	err := s.performBackup(ctx, &config, history)
	
	// 更新历史记录
	now := time.Now()
	history.CompletedAt = &now
	history.Duration = int64(now.Sub(history.StartedAt).Seconds())
	
	if err != nil {
		history.Status = "failed"
		history.ErrorMessage = err.Error()
		config.LastBackupStatus = "failed"
		config.LastBackupError = err.Error()
	} else {
		history.Status = "success"
		config.LastBackupStatus = "success"
		config.LastBackupError = ""
		config.LastBackupSize = history.FileSize
	}
	
	config.LastBackupAt = &now
	s.db.Save(&config)
	s.db.Save(history)

	// 清理过期备份
	s.cleanupExpiredBackups(&config)

	// 发送通知
	s.sendNotification(&config, history, err)
}

// performBackup 执行实际的备份操作
func (s *DatabaseBackupService) performBackup(ctx context.Context, config *models.BackupConfig, history *models.BackupHistory) error {
	// 生成备份文件名
	timestamp := time.Now().Format("20060102_150405")
	fileName := fmt.Sprintf("smartdns_backup_%s.db", timestamp)
	if config.CompressionEnabled {
		fileName += ".zip"
	}
	
	history.FileName = fileName

	// 创建临时备份文件
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fileName)
	defer os.Remove(tempFile)

	// 备份数据库
	if err := s.backupDatabase(config, tempFile, history); err != nil {
		return fmt.Errorf("failed to backup database: %w", err)
	}

	// 获取文件大小
	if stat, err := os.Stat(tempFile); err == nil {
		history.FileSize = stat.Size()
	}

	// 上传到S3（如果启用）
	if config.S3Enabled {
		if err := s.uploadToS3(ctx, config, tempFile, history); err != nil {
			return fmt.Errorf("failed to upload to S3: %w", err)
		}
	}

	// 保存到本地（如果配置了本地路径）
	if config.LocalPath != "" {
		if err := s.saveToLocal(config, tempFile, history); err != nil {
			return fmt.Errorf("failed to save locally: %w", err)
		}
	}

	return nil
}

// backupDatabase 备份数据库
func (s *DatabaseBackupService) backupDatabase(config *models.BackupConfig, outputPath string, history *models.BackupHistory) error {
	// 获取原始数据库文件大小
	if stat, err := os.Stat(s.config.DBPath); err == nil {
		history.DatabaseSize = stat.Size()
	}

	if config.CompressionEnabled {
		return s.backupDatabaseCompressed(config, outputPath, history)
	} else {
		return s.backupDatabaseRaw(outputPath)
	}
}

// backupDatabaseRaw 原始备份（直接复制数据库文件）
func (s *DatabaseBackupService) backupDatabaseRaw(outputPath string) error {
	srcFile, err := os.Open(s.config.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open source database: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// backupDatabaseCompressed 压缩备份
func (s *DatabaseBackupService) backupDatabaseCompressed(config *models.BackupConfig, outputPath string, history *models.BackupHistory) error {
	// 创建zip文件
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// 设置压缩级别
	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, config.CompressionLevel)
	})

	// 添加数据库文件到zip
	dbFile, err := os.Open(s.config.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open database file: %w", err)
	}
	defer dbFile.Close()

	dbFileName := filepath.Base(s.config.DBPath)
	writer, err := zipWriter.Create(dbFileName)
	if err != nil {
		return fmt.Errorf("failed to create zip entry: %w", err)
	}

	written, err := io.Copy(writer, dbFile)
	if err != nil {
		return fmt.Errorf("failed to write to zip: %w", err)
	}

	// 计算压缩比
	if history.DatabaseSize > 0 {
		history.CompressionRatio = float64(written) / float64(history.DatabaseSize)
	}

	return nil
}

// uploadToS3 上传备份到S3
func (s *DatabaseBackupService) uploadToS3(ctx context.Context, config *models.BackupConfig, filePath string, history *models.BackupHistory) error {
	// 创建S3客户端（使用配置中的凭据）
	s3Config := S3Config{
		AccessKey: config.S3AccessKey,
		SecretKey: config.S3SecretKey,
		Region:    config.S3Region,
		Bucket:    config.S3Bucket,
		Endpoint:  config.S3Endpoint,
	}

	s3Client, err := NewS3Service(s3Config, s.db)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	// 打开文件
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// 读取文件内容
	fileContent, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// 加密（如果启用）
	if config.EncryptionEnabled && config.EncryptionKey != "" {
		encrypted, err := s.encryptData(fileContent, config.EncryptionKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt backup: %w", err)
		}
		fileContent = encrypted
		history.FileName += ".enc"
	}

	// 生成S3键
	s3Key := fmt.Sprintf("%s/%s", strings.TrimSuffix(config.S3Prefix, "/"), history.FileName)
	
	// 上传到S3
	_, err = s3Client.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(config.S3Bucket),
		Key:    aws.String(s3Key),
		Body:   bytes.NewReader(fileContent),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to S3: %w", err)
	}

	// 更新历史记录
	history.S3Key = s3Key
	history.S3Bucket = config.S3Bucket
	history.S3Region = config.S3Region

	return nil
}

// saveToLocal 保存到本地路径
func (s *DatabaseBackupService) saveToLocal(config *models.BackupConfig, srcPath string, history *models.BackupHistory) error {
	// 确保目录存在
	if err := os.MkdirAll(config.LocalPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	// 目标文件路径
	dstPath := filepath.Join(config.LocalPath, history.FileName)
	
	// 复制文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	history.FilePath = dstPath
	return nil
}

// encryptData 加密数据
func (s *DatabaseBackupService) encryptData(data []byte, key string) ([]byte, error) {
	// 生成32字节的密钥
	hasher := sha256.New()
	hasher.Write([]byte(key))
	keyBytes := hasher.Sum(nil)

	// 创建AES加密器
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}

	// 生成随机IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// 加密数据
	stream := cipher.NewCFBEncrypter(block, iv)
	encrypted := make([]byte, len(data))
	stream.XORKeyStream(encrypted, data)

	// 将IV和加密数据合并
	result := make([]byte, aes.BlockSize+len(encrypted))
	copy(result[:aes.BlockSize], iv)
	copy(result[aes.BlockSize:], encrypted)

	return result, nil
}

// cleanupExpiredBackups 清理过期备份
func (s *DatabaseBackupService) cleanupExpiredBackups(config *models.BackupConfig) {
	cutoffTime := time.Now().AddDate(0, 0, -config.RetentionDays)
	
	var expiredBackups []models.BackupHistory
	s.db.Where("config_id = ? AND created_at < ?", config.ID, cutoffTime).Find(&expiredBackups)

	for _, backup := range expiredBackups {
		// 删除S3文件
		if backup.S3Key != "" && config.S3Enabled {
			s.deleteS3File(config, backup.S3Key)
		}
		
		// 删除本地文件
		if backup.FilePath != "" {
			os.Remove(backup.FilePath)
		}
		
		// 删除数据库记录
		s.db.Delete(&backup)
	}
}

// deleteS3File 删除S3文件
func (s *DatabaseBackupService) deleteS3File(config *models.BackupConfig, s3Key string) {
	ctx := context.Background()
	
	s3Config := S3Config{
		AccessKey: config.S3AccessKey,
		SecretKey: config.S3SecretKey,
		Region:    config.S3Region,
		Bucket:    config.S3Bucket,
		Endpoint:  config.S3Endpoint,
	}

	s3Client, err := NewS3Service(s3Config, s.db)
	if err != nil {
		fmt.Printf("Failed to create S3 client for cleanup: %v\n", err)
		return
	}

	err = s3Client.DeleteFileFromS3(ctx, s3Key)
	if err != nil {
		fmt.Printf("Failed to delete S3 file %s: %v\n", s3Key, err)
	}
}

// sendNotification 发送通知
func (s *DatabaseBackupService) sendNotification(config *models.BackupConfig, history *models.BackupHistory, backupErr error) {
	// 根据配置决定是否发送通知
	shouldNotify := false
	if backupErr != nil && config.NotifyOnFailure {
		shouldNotify = true
	} else if backupErr == nil && config.NotifyOnSuccess {
		shouldNotify = true
	}

	if !shouldNotify {
		return
	}

	// TODO: 实现具体的通知逻辑
	// 这里可以集成现有的通知系统
	fmt.Printf("Backup notification: Config=%s, Status=%s\n", config.Name, history.Status)
}

// UpdateBackupConfig 更新备份配置
func (s *DatabaseBackupService) UpdateBackupConfig(config *models.BackupConfig) error {
	// 保存到数据库
	if err := s.db.Save(config).Error; err != nil {
		return err
	}

	// 重新调度任务
	if config.Enabled {
		return s.scheduleBackup(config)
	} else {
		// 如果禁用了，移除现有任务
		if jobID, exists := s.activeJobs[config.ID]; exists {
			s.cron.Remove(jobID)
			delete(s.activeJobs, config.ID)
		}
	}

	return nil
}

// ManualBackup 手动触发备份
func (s *DatabaseBackupService) ManualBackup(configID uint) error {
	var config models.BackupConfig
	if err := s.db.First(&config, configID).Error; err != nil {
		return fmt.Errorf("backup config not found: %w", err)
	}

	// 异步执行备份
	go s.executeBackup(configID)
	
	return nil
}

// GetBackupStats 获取备份统计信息
func (s *DatabaseBackupService) GetBackupStats() (*models.BackupStats, error) {
	var stats models.BackupStats

	// 统计配置数量
	var totalConfigs, activeConfigs, totalBackups, successfulBackups, failedBackups int64
	s.db.Model(&models.BackupConfig{}).Count(&totalConfigs)
	s.db.Model(&models.BackupConfig{}).Where("enabled = ?", true).Count(&activeConfigs)

	// 统计备份数量
	s.db.Model(&models.BackupHistory{}).Count(&totalBackups)
	s.db.Model(&models.BackupHistory{}).Where("status = ?", "success").Count(&successfulBackups)
	s.db.Model(&models.BackupHistory{}).Where("status = ?", "failed").Count(&failedBackups)

	stats.TotalConfigs = int(totalConfigs)
	stats.ActiveConfigs = int(activeConfigs)
	stats.TotalBackups = int(totalBackups)
	stats.SuccessfulBackups = int(successfulBackups)
	stats.FailedBackups = int(failedBackups)

	// 计算成功率
	if totalBackups > 0 {
		stats.SuccessRate = float64(successfulBackups) / float64(totalBackups) * 100
	}

	// 统计总大小
	var totalSize int64
	s.db.Model(&models.BackupHistory{}).Select("COALESCE(SUM(file_size), 0)").Row().Scan(&totalSize)
	stats.TotalSize = totalSize

	// 获取最后备份时间
	var lastBackup models.BackupHistory
	if err := s.db.Order("created_at DESC").First(&lastBackup).Error; err == nil {
		stats.LastBackupAt = &lastBackup.CreatedAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询最后备份时间失败: %w", err)
	}

	// 获取下次备份时间
	var nextConfig models.BackupConfig
	if err := s.db.Session(&gorm.Session{Logger: s.db.Logger.LogMode(logger.Silent)}).
		Where("enabled = ? AND next_backup_at IS NOT NULL", true).
		Order("next_backup_at ASC").First(&nextConfig).Error; err == nil {
		stats.NextBackupAt = nextConfig.NextBackupAt
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("查询下次备份时间失败: %w", err)
	}

	return &stats, nil
}

// RestoreBackup 恢复备份
func (s *DatabaseBackupService) RestoreBackup(request *models.BackupRestoreRequest) error {
	// 获取备份历史
	var history models.BackupHistory
	if err := s.db.Preload("Config").First(&history, request.BackupHistoryID).Error; err != nil {
		return fmt.Errorf("backup history not found: %w", err)
	}

	// 验证备份状态
	if history.Status != "success" {
		return fmt.Errorf("cannot restore failed backup")
	}

	ctx := context.Background()
	var backupData []byte
	var err error

	// 从S3下载备份文件
	if history.S3Key != "" {
		backupData, err = s.downloadFromS3(ctx, &history.Config, history.S3Key)
		if err != nil {
			return fmt.Errorf("failed to download from S3: %w", err)
		}
	} else if history.FilePath != "" {
		// 从本地文件读取
		backupData, err = os.ReadFile(history.FilePath)
		if err != nil {
			return fmt.Errorf("failed to read local backup file: %w", err)
		}
	} else {
		return fmt.Errorf("no backup file available")
	}

	// 解密（如果需要）
	if history.Config.EncryptionEnabled && strings.HasSuffix(history.FileName, ".enc") {
		if request.BackupPassword == "" {
			return fmt.Errorf("backup password required for encrypted backup")
		}
		backupData, err = s.decryptData(backupData, request.BackupPassword)
		if err != nil {
			return fmt.Errorf("failed to decrypt backup: %w", err)
		}
	}

	// 创建临时文件
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, "restore_"+history.FileName)
	defer os.Remove(tempFile)

	if err := os.WriteFile(tempFile, backupData, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// 恢复数据库
	return s.restoreDatabase(tempFile, history.Config.CompressionEnabled)
}

// downloadFromS3 从S3下载文件
func (s *DatabaseBackupService) downloadFromS3(ctx context.Context, config *models.BackupConfig, s3Key string) ([]byte, error) {
	s3Config := S3Config{
		AccessKey: config.S3AccessKey,
		SecretKey: config.S3SecretKey,
		Region:    config.S3Region,
		Bucket:    config.S3Bucket,
		Endpoint:  config.S3Endpoint,
	}

	s3Client, err := NewS3Service(s3Config, s.db)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	return s3Client.DownloadFile(ctx, s3Key)
}

// decryptData 解密数据
func (s *DatabaseBackupService) decryptData(data []byte, key string) ([]byte, error) {
	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// 生成32字节的密钥
	hasher := sha256.New()
	hasher.Write([]byte(key))
	keyBytes := hasher.Sum(nil)

	// 创建AES解密器
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, err
	}

	// 提取IV和加密数据
	iv := data[:aes.BlockSize]
	encrypted := data[aes.BlockSize:]

	// 解密数据
	stream := cipher.NewCFBDecrypter(block, iv)
	decrypted := make([]byte, len(encrypted))
	stream.XORKeyStream(decrypted, encrypted)

	return decrypted, nil
}

// restoreDatabase 恢复数据库
func (s *DatabaseBackupService) restoreDatabase(backupFile string, isCompressed bool) error {
	// 停止当前的数据库连接（需要谨慎处理）
	// 这是一个危险操作，实际使用时应该有更完善的停机流程
	
	var sourceFile string
	if isCompressed {
		// 解压缩
		tempDir := os.TempDir()
		extractDir := filepath.Join(tempDir, "restore_extract")
		defer os.RemoveAll(extractDir)
		
		if err := s.extractZip(backupFile, extractDir); err != nil {
			return fmt.Errorf("failed to extract backup: %w", err)
		}
		
		// 查找数据库文件
		dbFileName := filepath.Base(s.config.DBPath)
		sourceFile = filepath.Join(extractDir, dbFileName)
	} else {
		sourceFile = backupFile
	}

	// 备份当前数据库
	currentDBBackup := s.config.DBPath + ".restore_backup"
	if err := s.copyFile(s.config.DBPath, currentDBBackup); err != nil {
		return fmt.Errorf("failed to backup current database: %w", err)
	}

	// 恢复数据库文件
	if err := s.copyFile(sourceFile, s.config.DBPath); err != nil {
		// 恢复失败，还原原始文件
		s.copyFile(currentDBBackup, s.config.DBPath)
		return fmt.Errorf("failed to restore database: %w", err)
	}

	// 删除备份文件
	os.Remove(currentDBBackup)

	return nil
}

// extractZip 解压缩文件
func (s *DatabaseBackupService) extractZip(src, dest string) error {
	reader, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer reader.Close()

	os.MkdirAll(dest, 0755)

	for _, file := range reader.File {
		path := filepath.Join(dest, file.Name)
		
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.FileInfo().Mode())
			continue
		}

		rc, err := file.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		outFile, err := os.Create(path)
		if err != nil {
			return err
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, rc)
		if err != nil {
			return err
		}
	}

	return nil
}

// copyFile 复制文件
func (s *DatabaseBackupService) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}