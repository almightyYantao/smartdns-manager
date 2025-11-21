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

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"smartdns-manager/database"
	"smartdns-manager/models"
)

// LogMonitorServiceCH ClickHouse 版本的日志监控服务
type LogMonitorServiceCH struct {
	db          *gorm.DB
	monitors    map[uint]*NodeMonitorCH
	mu          sync.RWMutex
	batchSize   int
	flushTicker *time.Ticker
	logRegex    *regexp.Regexp
}

// NodeMonitorCH 单个节点的监控器
type NodeMonitorCH struct {
	nodeID      uint
	node        *models.Node
	sshClient   *SSHClient
	ctx         context.Context
	cancel      context.CancelFunc
	isRunning   bool
	logRegex    *regexp.Regexp
	batchBuffer []*models.DNSLogCK
	mu          sync.Mutex
}

// NewLogMonitorServiceCH 创建 ClickHouse 版本的日志监控服务
func NewLogMonitorServiceCH(db *gorm.DB) *LogMonitorServiceCH {
	if db == nil {
		log.Fatal("❌ database connection is nil")
	}

	logRegex := regexp.MustCompile(`\[([^\]]+)\]\s+(\S+)\s+query\s+(\S+),\s+type\s+(\d+),\s+time\s+(\d+)ms,\s+speed:\s+([-\d.]+)ms,\s+result\s*(.*)`)

	service := &LogMonitorServiceCH{
		db:          db,
		monitors:    make(map[uint]*NodeMonitorCH),
		batchSize:   1000,                            // ClickHouse 可以处理更大的批次
		flushTicker: time.NewTicker(2 * time.Second), // 2秒刷新一次
		logRegex:    logRegex,
	}

	// 启动批量刷新协程
	// go service.flushLoop()

	log.Println("✅ ClickHouse 日志监控服务初始化成功")
	return service
}

// flushLoop 定时刷新所有节点的批量数据
func (s *LogMonitorServiceCH) flushLoop() {
	for range s.flushTicker.C {
		s.mu.RLock()
		monitors := make([]*NodeMonitorCH, 0, len(s.monitors))
		for _, monitor := range s.monitors {
			monitors = append(monitors, monitor)
		}
		s.mu.RUnlock()

		for _, monitor := range monitors {
			monitor.flushBatch()
		}
	}
}

// StartNodeMonitor 启动指定节点的日志监控
func (s *LogMonitorServiceCH) StartNodeMonitor(nodeID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("🚀 尝试启动节点 %d 的监控", nodeID)

	// 检查是否已在运行
	if monitor, exists := s.monitors[nodeID]; exists && monitor.isRunning {
		log.Printf("⚠️ 节点 %d 的监控已在运行", nodeID)
		return fmt.Errorf("节点 %d 的监控已在运行", nodeID)
	}

	// 获取节点信息
	var node models.Node
	if err := s.db.First(&node, nodeID).Error; err != nil {
		log.Printf("❌ 节点 %d 不存在: %v", nodeID, err)
		return fmt.Errorf("节点不存在: %w", err)
	}

	log.Printf("📝 节点信息: %s (%s:%d)", node.Name, node.Host, node.Port)

	// 创建 SSH 客户端
	sshClient, err := NewSSHClient(&node)
	if err != nil {
		log.Printf("❌ SSH连接失败: %v", err)
		return fmt.Errorf("SSH连接失败: %w", err)
	}

	// 检查日志文件是否存在
	logPath := node.LogPath
	if logPath == "" {
		logPath = "/var/log/audit/audit.log"
	}

	log.Printf("📂 检查日志文件: %s", logPath)

	checkCmd := fmt.Sprintf("test -f %s && echo 'exists' || echo 'not found'", logPath)
	output, err := sshClient.ExecuteCommand(checkCmd)
	log.Printf("📄 文件检查结果: %s", strings.TrimSpace(output))

	if err != nil || !strings.Contains(output, "exists") {
		sshClient.Close()
		log.Printf("❌ 日志文件不存在: %s", logPath)
		return fmt.Errorf("日志文件不存在: %s", logPath)
	}

	// 创建监控器
	ctx, cancel := context.WithCancel(context.Background())
	monitor := &NodeMonitorCH{
		nodeID:      nodeID,
		node:        &node,
		sshClient:   sshClient,
		ctx:         ctx,
		cancel:      cancel,
		isRunning:   true,
		logRegex:    s.logRegex,
		batchBuffer: make([]*models.DNSLogCK, 0, s.batchSize),
	}

	s.monitors[nodeID] = monitor

	// 启动监控协程
	go monitor.startMonitoring(s.batchSize)

	// 启动独立的刷新协程
	go monitor.autoFlushLoop()

	// 更新节点状态
	s.db.Model(&models.Node{}).Where("id = ?", nodeID).Updates(map[string]interface{}{
		"log_monitor_enabled": true,
	})

	log.Printf("✅ 节点 %d (%s) 的日志监控已启动", nodeID, node.Name)
	return nil
}

// autoFlushLoop 每个节点独立的自动刷新协程
func (m *NodeMonitorCH) autoFlushLoop() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.flushBatch()
		}
	}
}

// StopNodeMonitor 停止指定节点的日志监控
func (s *LogMonitorServiceCH) StopNodeMonitor(nodeID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	monitor, exists := s.monitors[nodeID]
	if !exists || !monitor.isRunning {
		return fmt.Errorf("节点 %d 的监控未运行", nodeID)
	}

	// 停止监控
	monitor.stop()
	delete(s.monitors, nodeID)

	// 更新节点状态
	s.db.Model(&models.Node{}).Where("id = ?", nodeID).Updates(map[string]interface{}{
		"log_monitor_enabled": false,
	})

	log.Printf("✅ 已停止节点 %d 的监控", nodeID)
	return nil
}

// GetNodeMonitorStatus 获取节点监控状态
func (s *LogMonitorServiceCH) GetNodeMonitorStatus(nodeID uint) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	monitor, exists := s.monitors[nodeID]
	if !exists {
		return false, nil
	}
	return monitor.isRunning, nil
}

// StopAll 停止所有监控
func (s *LogMonitorServiceCH) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for nodeID, monitor := range s.monitors {
		monitor.stop()
		log.Printf("✅ 已停止节点 %d 的监控", nodeID)
	}
	s.monitors = make(map[uint]*NodeMonitorCH)
	s.flushTicker.Stop()
}

// startMonitoring 开始监控（在 SSH 上执行 tail -f）
func (m *NodeMonitorCH) startMonitoring(batchSize int) {
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

				// 每处理 100 行打印一次日志
				if lineCount%5000 == 0 {
					log.Printf("📊 节点 %d 已处理 %d 行日志", m.nodeID, lineCount)
				}

				if dnsLog := m.parseLine(line); dnsLog != nil {
					dnsLog.NodeID = uint32(m.nodeID)
					m.addToBatch(dnsLog)

					// 达到批量大小，立即刷新
					if len(m.batchBuffer) >= batchSize {
						log.Printf("💾 节点 %d 批量缓冲区已满 (%d 条)，开始写入", m.nodeID, len(m.batchBuffer))
						m.flushBatch()
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
func (m *NodeMonitorCH) stop() {
	m.cancel()
	m.flushBatch() // 刷新剩余数据
	m.isRunning = false
}

// addToBatch 添加到批量缓冲区
func (m *NodeMonitorCH) addToBatch(dnsLog *models.DNSLogCK) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.batchBuffer = append(m.batchBuffer, dnsLog)
}

// flushBatch 批量插入到 ClickHouse
func (m *NodeMonitorCH) flushBatch() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.batchBuffer) == 0 {
		return
	}

	batchCount := len(m.batchBuffer)
	log.Printf("💾 准备写入 %d 条日志到 ClickHouse (节点%d)", batchCount, m.nodeID)

	ctx := context.Background()
	batch, err := database.CHConn.PrepareBatch(ctx,
		"INSERT INTO dns_query_log (timestamp, date, node_id, client_ip, domain, query_type, time_ms, speed_ms, result_count, result_ips, raw_log)")

	if err != nil {
		log.Printf("❌ 准备批次失败 (节点%d): %v", m.nodeID, err)
		return
	}

	startTime := time.Now()
	successCount := 0

	for _, dnsLog := range m.batchBuffer {
		err := batch.Append(
			dnsLog.Timestamp,
			dnsLog.Date,
			dnsLog.NodeID,
			dnsLog.ClientIP,
			dnsLog.Domain,
			dnsLog.QueryType,
			dnsLog.TimeMs,
			dnsLog.SpeedMs,
			dnsLog.ResultCount,
			dnsLog.ResultIPs,
			dnsLog.RawLog,
		)
		if err != nil {
			log.Printf("❌ 添加记录失败 (节点%d): %v", m.nodeID, err)
		} else {
			successCount++
		}
	}

	if err := batch.Send(); err != nil {
		log.Printf("❌ 发送批次失败 (节点%d): %v", m.nodeID, err)
	} else {
		duration := time.Since(startTime)
		log.Printf("✅ 成功插入 %d/%d 条日志到 ClickHouse (节点%d), 耗时: %v", successCount, batchCount, m.nodeID, duration)
	}

	m.batchBuffer = make([]*models.DNSLogCK, 0, 1000)
}

// parseLine 解析日志行
func (m *NodeMonitorCH) parseLine(line string) *models.DNSLogCK {
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
	speedMs, _ := strconv.ParseFloat(matches[6], 32)

	resultStr := strings.TrimSpace(matches[7])
	var resultIPs []string
	if resultStr != "" {
		resultIPs = strings.Split(resultStr, ",")
		for i := range resultIPs {
			resultIPs[i] = strings.TrimSpace(resultIPs[i])
		}
	}

	return &models.DNSLogCK{
		Timestamp:   timestamp,
		Date:        time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location()),
		ClientIP:    matches[2],
		Domain:      matches[3],
		QueryType:   uint16(queryType),
		TimeMs:      uint32(timeMs),
		SpeedMs:     float32(speedMs),
		ResultCount: uint8(len(resultIPs)),
		ResultIPs:   resultIPs,
		RawLog:      line,
	}
}

// GetLogs 查询日志
func (s *LogMonitorServiceCH) GetLogs(page, pageSize int, filters map[string]interface{}) ([]models.DNSLog, int64, error) {
	ctx := context.Background()

	// 构建查询条件
	where := []string{"1=1"}
	args := []interface{}{}

	if nodeID, ok := filters["node_id"].(uint); ok {
		where = append(where, "node_id = ?")
		args = append(args, uint32(nodeID))
	}

	if clientIP, ok := filters["client_ip"].(string); ok && clientIP != "" {
		where = append(where, "client_ip = ?")
		args = append(args, clientIP)
	}

	if domain, ok := filters["domain"].(string); ok && domain != "" {
		where = append(where, "domain LIKE ?")
		args = append(args, "%"+domain+"%")
	}

	if queryType, ok := filters["query_type"].(int); ok {
		where = append(where, "query_type = ?")
		args = append(args, uint16(queryType))
	}

	if startTime, ok := filters["start_time"].(time.Time); ok {
		where = append(where, "timestamp >= ?")
		args = append(args, startTime)
	}

	if endTime, ok := filters["end_time"].(time.Time); ok {
		where = append(where, "timestamp <= ?")
		args = append(args, endTime)
	}

	whereClause := strings.Join(where, " AND ")

	// 查询总数
	var total uint64
	countQuery := fmt.Sprintf("SELECT count() FROM dns_query_log WHERE %s", whereClause)
	err := database.CHConn.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		log.Printf("❌ 查询总数失败: %v", err)
		return nil, 0, err
	}

	// 查询数据
	offset := (page - 1) * pageSize
	dataQuery := fmt.Sprintf(
		"SELECT timestamp, node_id, client_ip, domain, query_type, time_ms, speed_ms, result_count, result_ips, raw_log "+
			"FROM dns_query_log WHERE %s ORDER BY timestamp DESC LIMIT %d OFFSET %d",
		whereClause, pageSize, offset)

	rows, err := database.CHConn.Query(ctx, dataQuery, args...)
	if err != nil {
		log.Printf("❌ 查询数据失败: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.DNSLog
	for rows.Next() {
		var logCK models.DNSLogCK
		err := rows.Scan(
			&logCK.Timestamp,
			&logCK.NodeID,
			&logCK.ClientIP,
			&logCK.Domain,
			&logCK.QueryType,
			&logCK.TimeMs,
			&logCK.SpeedMs,
			&logCK.ResultCount,
			&logCK.ResultIPs,
			&logCK.RawLog,
		)
		if err != nil {
			log.Printf("⚠️ 扫描行失败: %v", err)
			continue
		}

		// 转换为通用格式
		log := models.DNSLog{
			NodeID:    uint(logCK.NodeID),
			Timestamp: logCK.Timestamp,
			ClientIP:  logCK.ClientIP,
			Domain:    logCK.Domain,
			QueryType: int(logCK.QueryType),
			TimeMs:    int(logCK.TimeMs),
			SpeedMs:   float64(logCK.SpeedMs),
			Result:    strings.Join(logCK.ResultIPs, ", "),
			ResultIPs: strings.Join(logCK.ResultIPs, ","),
			IPCount:   int(logCK.ResultCount),
			RawLog:    logCK.RawLog,
		}
		logs = append(logs, log)
	}

	log.Printf("✅ 成功查询 %d 条日志，总数: %d", len(logs), total)
	return logs, int64(total), nil
}

// GetNodeStats 获取统计信息
func (s *LogMonitorServiceCH) GetNodeStats(nodeID uint, startTime, endTime time.Time) (*models.DNSLogStats, error) {
	ctx := context.Background()
	stats := &models.DNSLogStats{
		TopDomains:  make([]models.DomainStat, 0),
		TopClients:  make([]models.ClientStat, 0),
		HourlyStats: make([]models.HourlyStat, 0),
	}

	startDate := startTime.Format("2006-01-02")
	endDate := endTime.Format("2006-01-02")
	nodeID32 := uint32(nodeID)

	// 总查询数
	var totalQueries uint64
	err := database.CHConn.QueryRow(ctx,
		"SELECT count() FROM dns_query_log WHERE node_id = ? AND date BETWEEN ? AND ?",
		nodeID32, startDate, endDate).Scan(&totalQueries)
	if err != nil {
		log.Printf("❌ 查询总数失败: %v", err)
		return nil, err
	}
	stats.TotalQueries = int64(totalQueries)

	// 如果没有数据，直接返回空统计
	if totalQueries == 0 {
		log.Printf("⚠️ 节点 %d 在指定时间范围内没有日志数据", nodeID)
		return stats, nil
	}

	// 唯一客户端
	var uniqueClients uint64
	err = database.CHConn.QueryRow(ctx,
		"SELECT uniqExact(client_ip) FROM dns_query_log WHERE node_id = ? AND date BETWEEN ? AND ?",
		nodeID32, startDate, endDate).Scan(&uniqueClients)
	if err != nil {
		log.Printf("❌ 查询唯一客户端失败: %v", err)
	} else {
		stats.UniqueClients = int64(uniqueClients)
	}

	// 唯一域名
	var uniqueDomains uint64
	err = database.CHConn.QueryRow(ctx,
		"SELECT uniqExact(domain) FROM dns_query_log WHERE node_id = ? AND date BETWEEN ? AND ?",
		nodeID32, startDate, endDate).Scan(&uniqueDomains)
	if err != nil {
		log.Printf("❌ 查询唯一域名失败: %v", err)
	} else {
		stats.UniqueDomains = int64(uniqueDomains)
	}

	// 平均查询时间 - 处理 NaN 情况
	var avgQueryTime *float64 // 使用指针类型，可以接收 NULL
	err = database.CHConn.QueryRow(ctx,
		"SELECT avgOrNull(time_ms) FROM dns_query_log WHERE node_id = ? AND date BETWEEN ? AND ?",
		nodeID32, startDate, endDate).Scan(&avgQueryTime)
	if err != nil {
		log.Printf("❌ 查询平均时间失败: %v", err)
	} else if avgQueryTime != nil {
		stats.AvgQueryTime = *avgQueryTime
	} else {
		stats.AvgQueryTime = 0 // NULL 时设为 0
	}

	// 热门域名
	rows, err := database.CHConn.Query(ctx,
		"SELECT domain, count() as count FROM dns_query_log "+
			"WHERE node_id = ? AND date BETWEEN ? AND ? "+
			"GROUP BY domain ORDER BY count DESC LIMIT 10",
		nodeID32, startDate, endDate)
	if err != nil {
		log.Printf("❌ 查询热门域名失败: %v", err)
	} else {
		for rows.Next() {
			var stat models.DomainStat
			var count uint64
			if err := rows.Scan(&stat.Domain, &count); err != nil {
				log.Printf("⚠️ 扫描热门域名失败: %v", err)
				continue
			}
			stat.Count = int64(count)
			stats.TopDomains = append(stats.TopDomains, stat)
		}
		rows.Close()
	}

	// 热门客户端
	rows, err = database.CHConn.Query(ctx,
		"SELECT client_ip, count() as count FROM dns_query_log "+
			"WHERE node_id = ? AND date BETWEEN ? AND ? "+
			"GROUP BY client_ip ORDER BY count DESC LIMIT 10",
		nodeID32, startDate, endDate)
	if err != nil {
		log.Printf("❌ 查询热门客户端失败: %v", err)
	} else {
		for rows.Next() {
			var stat models.ClientStat
			var count uint64
			if err := rows.Scan(&stat.ClientIP, &count); err != nil {
				log.Printf("⚠️ 扫描热门客户端失败: %v", err)
				continue
			}
			stat.Count = int64(count)
			stats.TopClients = append(stats.TopClients, stat)
		}
		rows.Close()
	}

	// 按小时统计
	rows, err = database.CHConn.Query(ctx,
		"SELECT toHour(timestamp) as hour, count() as count FROM dns_query_log "+
			"WHERE node_id = ? AND timestamp BETWEEN ? AND ? "+
			"GROUP BY hour ORDER BY hour",
		nodeID32, startTime, endTime)
	if err != nil {
		log.Printf("❌ 查询按小时统计失败: %v", err)
	} else {
		for rows.Next() {
			var stat models.HourlyStat
			var count uint64
			if err := rows.Scan(&stat.Hour, &count); err != nil {
				log.Printf("⚠️ 扫描按小时统计失败: %v", err)
				continue
			}
			stat.Count = int64(count)
			stats.HourlyStats = append(stats.HourlyStats, stat)
		}
		rows.Close()
	}

	log.Printf("✅ 成功获取节点 %d 的统计信息 (总查询数: %d)", nodeID, totalQueries)
	return stats, nil
}

// CleanNodeLogs 清理节点旧日志
func (s *LogMonitorServiceCH) CleanNodeLogs(nodeID uint, days int) error {
	ctx := context.Background()
	cutoffDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	query := fmt.Sprintf(
		"ALTER TABLE dns_query_log DELETE WHERE node_id = %d AND date < '%s'",
		uint32(nodeID), cutoffDate)

	err := database.CHConn.Exec(ctx, query)
	if err != nil {
		log.Printf("❌ 清理节点 %d 旧日志失败: %v", nodeID, err)
		return err
	}

	log.Printf("✅ 成功清理节点 %d 的 %d 天前的日志", nodeID, days)
	return nil
}
