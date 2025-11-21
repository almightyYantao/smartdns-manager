package services

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"smartdns-manager/models"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

// NodeMonitor 单个节点的监控器
type NodeMonitor struct {
	nodeID      uint
	node        *models.Node
	sshClient   *SSHClient
	ctx         context.Context
	cancel      context.CancelFunc
	isRunning   bool
	logRegex    *regexp.Regexp
	batchBuffer []*models.DNSLog
	mu          sync.Mutex
}

// LogMonitorService 日志监控服务（管理所有节点监控）
type LogMonitorService struct {
	db          *gorm.DB
	monitors    map[uint]*NodeMonitor // nodeID -> monitor
	mu          sync.RWMutex
	batchSize   int
	flushTicker *time.Ticker
	logRegex    *regexp.Regexp
}

// NewLogMonitorService 创建日志监控服务
func NewLogMonitorService(db *gorm.DB) *LogMonitorService {
	// 编译正则表达式
	logRegex := regexp.MustCompile(`\[([^\]]+)\]\s+(\S+)\s+query\s+(\S+),\s+type\s+(\d+),\s+time\s+(\d+)ms,\s+speed:\s+([-\d.]+)ms,\s+result\s*(.*)`)

	service := &LogMonitorService{
		db:          db,
		monitors:    make(map[uint]*NodeMonitor),
		batchSize:   100,
		flushTicker: time.NewTicker(5 * time.Second),
		logRegex:    logRegex,
	}

	// 启动批量刷新协程
	go service.flushLoop()

	return service
}

// StartNodeMonitor 启动指定节点的日志监控
func (s *LogMonitorService) StartNodeMonitor(nodeID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已在运行
	if monitor, exists := s.monitors[nodeID]; exists && monitor.isRunning {
		return fmt.Errorf("节点 %d 的监控已在运行", nodeID)
	}

	// 获取节点信息
	var node models.Node
	if err := s.db.First(&node, nodeID).Error; err != nil {
		return fmt.Errorf("节点不存在: %w", err)
	}

	// 创建 SSH 客户端
	sshClient, err := NewSSHClient(&node)
	if err != nil {
		return fmt.Errorf("SSH连接失败: %w", err)
	}

	// 检查日志文件是否存在
	logPath := node.LogPath
	if logPath == "" {
		logPath = "/var/log/smartdns/smartdns.log"
	}

	_, err = sshClient.ExecuteCommand(fmt.Sprintf("test -f %s && echo 'exists'", logPath))
	if err != nil {
		sshClient.Close()
		return fmt.Errorf("日志文件不存在: %s", logPath)
	}

	// 创建监控器
	ctx, cancel := context.WithCancel(context.Background())
	monitor := &NodeMonitor{
		nodeID:      nodeID,
		node:        &node,
		sshClient:   sshClient,
		ctx:         ctx,
		cancel:      cancel,
		isRunning:   true,
		logRegex:    s.logRegex,
		batchBuffer: make([]*models.DNSLog, 0, s.batchSize),
	}

	s.monitors[nodeID] = monitor

	// 启动监控协程
	go monitor.startMonitoring(s.db, s.batchSize)

	// 更新节点状态
	s.db.Model(&models.Node{}).Where("id = ?", nodeID).Updates(map[string]interface{}{
		"log_monitor_enabled": true,
	})

	log.Printf("已启动节点 %d (%s) 的日志监控", nodeID, node.Name)
	return nil
}

// StopNodeMonitor 停止指定节点的日志监控
func (s *LogMonitorService) StopNodeMonitor(nodeID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	monitor, exists := s.monitors[nodeID]
	if !exists || !monitor.isRunning {
		return fmt.Errorf("节点 %d 的监控未运行", nodeID)
	}

	// 停止监控
	monitor.stop(s.db)
	delete(s.monitors, nodeID)

	// 更新节点状态
	s.db.Model(&models.Node{}).Where("id = ?", nodeID).Updates(map[string]interface{}{
		"log_monitor_enabled": false,
	})

	log.Printf("已停止节点 %d 的日志监控", nodeID)
	return nil
}

// GetNodeMonitorStatus 获取节点监控状态
func (s *LogMonitorService) GetNodeMonitorStatus(nodeID uint) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	monitor, exists := s.monitors[nodeID]
	if !exists {
		return false, nil
	}
	return monitor.isRunning, nil
}

// StopAll 停止所有监控
func (s *LogMonitorService) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for nodeID, monitor := range s.monitors {
		monitor.stop(s.db)
		log.Printf("已停止节点 %d 的监控", nodeID)
	}
	s.monitors = make(map[uint]*NodeMonitor)
	s.flushTicker.Stop()
}

// flushLoop 定时刷新所有节点的批量数据
func (s *LogMonitorService) flushLoop() {
	for range s.flushTicker.C {
		s.mu.RLock()
		monitors := make([]*NodeMonitor, 0, len(s.monitors))
		for _, monitor := range s.monitors {
			monitors = append(monitors, monitor)
		}
		s.mu.RUnlock()

		for _, monitor := range monitors {
			monitor.flushBatch(s.db)
		}
	}
}

// startMonitoring 开始监控（在 SSH 上执行 tail -f）
func (m *NodeMonitor) startMonitoring(db *gorm.DB, batchSize int) {
	defer func() {
		m.isRunning = false
		m.sshClient.Close()
		log.Printf("🛑 节点 %d 监控已停止", m.nodeID)
	}()

	logPath := m.node.LogPath
	if logPath == "" {
		logPath = "/var/log/audit/audit.log"
	}

	// 使用 tail -f -n 0 只读取新增日志
	cmd := fmt.Sprintf("tail -f -n 0 %s", logPath)

	log.Printf("🔄 执行命令: %s", cmd)

	session, err := m.sshClient.client.NewSession()
	if err != nil {
		log.Printf("❌ 创建SSH会话失败 (节点%d): %v", m.nodeID, err)
		return
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Printf("❌ 获取标准输出失败 (节点%d): %v", m.nodeID, err)
		return
	}

	if err := session.Start(cmd); err != nil {
		log.Printf("❌ 执行命令失败 (节点%d): %v", m.nodeID, err)
		return
	}

	log.Printf("✅ 开始监控节点 %d 的日志", m.nodeID)

	scanner := bufio.NewScanner(stdout)
	lineCount := 0

	for {
		select {
		case <-m.ctx.Done():
			log.Printf("⏹️ 收到停止信号 (节点%d)", m.nodeID)
			session.Signal(ssh.SIGTERM)
			return
		default:
			if scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				lineCount++

				// 每处理 10 行打印一次日志
				if lineCount%10 == 0 {
					log.Printf("📊 节点 %d 已处理 %d 行日志", m.nodeID, lineCount)
				}

				if dnsLog := m.parseLine(line); dnsLog != nil {
					dnsLog.NodeID = m.nodeID
					m.addToBatch(dnsLog)

					// 达到批量大小，立即刷新
					if len(m.batchBuffer) >= batchSize {
						log.Printf("💾 节点 %d 批量缓冲区已满 (%d 条)，开始写入", m.nodeID, len(m.batchBuffer))
						m.flushBatch(db)
					}
				} else {
					// 解析失败时打印样本
					if lineCount <= 5 {
						log.Printf("⚠️ 解析失败 (节点%d): %s", m.nodeID, line)
					}
				}
			}

			if err := scanner.Err(); err != nil {
				log.Printf("❌ 读取日志出错 (节点%d): %v", m.nodeID, err)
				return
			}
		}
	}
}

// stop 停止监控
func (m *NodeMonitor) stop(db *gorm.DB) {
	m.cancel()
	m.flushBatch(db) // 刷新剩余数据
	m.isRunning = false
}

// addToBatch 添加到批量缓冲区
func (m *NodeMonitor) addToBatch(dnsLog *models.DNSLog) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchBuffer = append(m.batchBuffer, dnsLog)
}

// flushBatch 刷新批量数据到数据库
func (m *NodeMonitor) flushBatch(db *gorm.DB) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.batchBuffer) == 0 {
		return
	}

	batchCount := len(m.batchBuffer)
	log.Printf("💾 准备写入 %d 条日志到数据库 (节点%d)", batchCount, m.nodeID)

	startTime := time.Now()
	if err := db.CreateInBatches(m.batchBuffer, 100).Error; err != nil {
		log.Printf("❌ 插入DNS日志失败 (节点%d): %v", m.nodeID, err)
	} else {
		duration := time.Since(startTime)
		log.Printf("✅ 成功插入 %d 条DNS日志 (节点%d), 耗时: %v", batchCount, m.nodeID, duration)
	}

	m.batchBuffer = make([]*models.DNSLog, 0, 100)
}

// parseLine 解析日志行
func (m *NodeMonitor) parseLine(line string) *models.DNSLog {
	if line == "" {
		return nil
	}

	matches := m.logRegex.FindStringSubmatch(line)
	if matches == nil || len(matches) < 8 {
		return nil
	}

	// 解析时间戳 - 指定使用 UTC 时区
	var timestamp time.Time
	var err error

	// 尝试解析带毫秒的格式
	timestamp, err = time.ParseInLocation("2006-01-02 15:04:05,000", matches[1], time.UTC)
	if err != nil {
		// 如果失败，尝试不带毫秒的格式
		timestamp, err = time.ParseInLocation("2006-01-02 15:04:05", matches[1][:19], time.UTC)
		if err != nil {
			log.Printf("⚠️ 解析时间失败: %s, error: %v", matches[1], err)
			return nil
		}
	}

	queryType, _ := strconv.Atoi(matches[4])
	timeMs, _ := strconv.Atoi(matches[5])
	speedMs, _ := strconv.ParseFloat(matches[6], 64)

	resultStr := strings.TrimSpace(matches[7])
	var resultIPs []string
	if resultStr != "" {
		resultIPs = strings.Split(resultStr, ",")
		for i := range resultIPs {
			resultIPs[i] = strings.TrimSpace(resultIPs[i])
		}
	}

	return &models.DNSLog{
		Timestamp: timestamp,
		ClientIP:  matches[2],
		Domain:    matches[3],
		QueryType: queryType,
		TimeMs:    timeMs,
		SpeedMs:   speedMs,
		Result:    resultStr,
		ResultIPs: strings.Join(resultIPs, ","),
		IPCount:   len(resultIPs),
		RawLog:    line,
		CreatedAt: time.Now(),
	}
}

// 查询方法

// GetLogs 获取日志列表（支持按节点过滤）
func (s *LogMonitorService) GetLogs(page, pageSize int, filters map[string]interface{}) ([]models.DNSLog, int64, error) {
	var logs []models.DNSLog
	var total int64

	query := s.db.Model(&models.DNSLog{}).Preload("Node")

	// 应用过滤条件
	if nodeID, ok := filters["node_id"]; ok && nodeID != nil {
		query = query.Where("node_id = ?", nodeID)
	}
	if clientIP, ok := filters["client_ip"]; ok && clientIP != "" {
		query = query.Where("client_ip = ?", clientIP)
	}
	if domain, ok := filters["domain"]; ok && domain != "" {
		query = query.Where("domain LIKE ?", "%"+domain.(string)+"%")
	}
	if queryType, ok := filters["query_type"]; ok {
		query = query.Where("query_type = ?", queryType)
	}
	if startTime, ok := filters["start_time"]; ok {
		query = query.Where("timestamp >= ?", startTime)
	}
	if endTime, ok := filters["end_time"]; ok {
		query = query.Where("timestamp <= ?", endTime)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	err := query.Order("timestamp DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&logs).Error

	return logs, total, err
}

// GetNodeStats 获取节点统计信息
func (s *LogMonitorService) GetNodeStats(nodeID uint, startTime, endTime time.Time) (*models.DNSLogStats, error) {
	stats := &models.DNSLogStats{
		TopDomains:  make([]models.DomainStat, 0),
		TopClients:  make([]models.ClientStat, 0),
		HourlyStats: make([]models.HourlyStat, 0),
	}

	// 总查询数
	var totalQueries int64
	s.db.Model(&models.DNSLog{}).
		Where("node_id = ? AND timestamp BETWEEN ? AND ?", nodeID, startTime, endTime).
		Count(&totalQueries)
	stats.TotalQueries = totalQueries

	// 如果没有数据，直接返回空统计
	if totalQueries == 0 {
		log.Printf("⚠️ 节点 %d 在指定时间范围内没有日志数据", nodeID)
		return stats, nil
	}

	// 唯一客户端数
	var uniqueClients int64
	s.db.Model(&models.DNSLog{}).
		Where("node_id = ? AND timestamp BETWEEN ? AND ?", nodeID, startTime, endTime).
		Distinct("client_ip").
		Count(&uniqueClients)
	stats.UniqueClients = uniqueClients

	// 唯一域名数
	var uniqueDomains int64
	s.db.Model(&models.DNSLog{}).
		Where("node_id = ? AND timestamp BETWEEN ? AND ?", nodeID, startTime, endTime).
		Distinct("domain").
		Count(&uniqueDomains)
	stats.UniqueDomains = uniqueDomains

	// 平均查询时间 - 处理空值
	var avgQueryTime *float64 // 使用指针
	s.db.Model(&models.DNSLog{}).
		Where("node_id = ? AND timestamp BETWEEN ? AND ?", nodeID, startTime, endTime).
		Select("AVG(time_ms)").
		Scan(&avgQueryTime)
	if avgQueryTime != nil {
		stats.AvgQueryTime = *avgQueryTime
	} else {
		stats.AvgQueryTime = 0
	}

	// 热门域名（Top 10）
	type domainCount struct {
		Domain string
		Count  int64
	}
	var topDomains []domainCount
	s.db.Model(&models.DNSLog{}).
		Select("domain, COUNT(*) as count").
		Where("node_id = ? AND timestamp BETWEEN ? AND ?", nodeID, startTime, endTime).
		Group("domain").
		Order("count DESC").
		Limit(10).
		Scan(&topDomains)

	for _, item := range topDomains {
		stats.TopDomains = append(stats.TopDomains, models.DomainStat{
			Domain: item.Domain,
			Count:  item.Count,
		})
	}

	// 热门客户端（Top 10）
	type clientCount struct {
		ClientIP string
		Count    int64
	}
	var topClients []clientCount
	s.db.Model(&models.DNSLog{}).
		Select("client_ip, COUNT(*) as count").
		Where("node_id = ? AND timestamp BETWEEN ? AND ?", nodeID, startTime, endTime).
		Group("client_ip").
		Order("count DESC").
		Limit(10).
		Scan(&topClients)

	for _, item := range topClients {
		stats.TopClients = append(stats.TopClients, models.ClientStat{
			ClientIP: item.ClientIP,
			Count:    item.Count,
		})
	}

	// 按小时统计
	type hourlyCount struct {
		Hour  int
		Count int64
	}
	var hourlyStats []hourlyCount

	// SQLite 使用 strftime 函数提取小时
	s.db.Model(&models.DNSLog{}).
		Select("CAST(strftime('%H', timestamp) AS INTEGER) as hour, COUNT(*) as count").
		Where("node_id = ? AND timestamp BETWEEN ? AND ?", nodeID, startTime, endTime).
		Group("hour").
		Order("hour").
		Scan(&hourlyStats)

	for _, item := range hourlyStats {
		stats.HourlyStats = append(stats.HourlyStats, models.HourlyStat{
			Hour:  item.Hour,
			Count: item.Count,
		})
	}

	return stats, nil
}

// CleanNodeLogs 清理指定节点的旧日志
func (s *LogMonitorService) CleanNodeLogs(nodeID uint, days int) error {
	cutoffTime := time.Now().AddDate(0, 0, -days)
	result := s.db.Where("node_id = ? AND timestamp < ?", nodeID, cutoffTime).
		Delete(&models.DNSLog{})

	if result.Error != nil {
		return result.Error
	}

	log.Printf("清理节点 %d 的 %d 条旧日志", nodeID, result.RowsAffected)
	return nil
}
