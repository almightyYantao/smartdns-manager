package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"smartdns-manager/models"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// LogMonitorServiceCH ClickHouse 实现
type LogMonitorServiceCH struct {
	conn driver.Conn // 使用 ClickHouse driver.Conn
}

// NewLogMonitorServiceCH 创建 ClickHouse 日志监控服务
func NewLogMonitorServiceCH(conn driver.Conn) LogMonitorInterface {
	if conn == nil {
		log.Fatal("❌ ClickHouse connection is nil")
	}

	service := &LogMonitorServiceCH{
		conn: conn,
	}

	// 确保表存在
	if err := service.EnsureTables(); err != nil {
		log.Printf("❌ 初始化表失败: %v", err)
	}

	log.Printf("✅ ClickHouse 日志监控服务初始化成功")
	return service
}

// GetLogs 获取DNS日志列表（实现接口）
func (s *LogMonitorServiceCH) GetLogs(page, pageSize int, filters map[string]interface{}) ([]models.DNSLog, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

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

	if group, ok := filters["group"].(string); ok && group != "" {
		where = append(where, "group = ?")
		args = append(args, group)
	}

	if domain, ok := filters["domain"].(string); ok && domain != "" {
		// 优化模糊查询
		where = append(where, "domain ILIKE ?")
		args = append(args, "%"+domain+"%")
	}

	if queryType, ok := filters["query_type"].(int); ok {
		where = append(where, "query_type = ?")
		args = append(args, uint16(queryType))
	}

	// 时间范围查询优化 - 确保有时间范围限制
	hasTimeFilter := false
	if startTime, ok := filters["start_time"].(time.Time); ok {
		where = append(where, "timestamp >= ?")
		args = append(args, startTime)
		hasTimeFilter = true
	}

	if endTime, ok := filters["end_time"].(time.Time); ok {
		where = append(where, "timestamp <= ?")
		args = append(args, endTime)
		hasTimeFilter = true
	}

	// 如果没有时间过滤条件，默认查询最近24小时
	if !hasTimeFilter {
		where = append(where, "timestamp >= ?")
		args = append(args, time.Now().Add(-24*time.Hour))
	}

	whereClause := strings.Join(where, " AND ")

	// 使用估算总数来提高性能（对于大数据集）
	var total uint64
	var err error

	// 先尝试精确计数，但设置较短超时
	countCtx, countCancel := context.WithTimeout(ctx, 5*time.Second)
	defer countCancel()

	countQuery := fmt.Sprintf("SELECT count() FROM dns_query_log WHERE %s", whereClause)
	err = s.conn.QueryRow(countCtx, countQuery, args...).Scan(&total)

	if err != nil {
		// 如果精确计数超时，使用估算
		log.Printf("⚠️ 精确计数超时，使用估算: %v", err)
		estimateQuery := fmt.Sprintf("SELECT round(count() * any(_sample_factor)) FROM dns_query_log SAMPLE 0.1 WHERE %s", whereClause)
		err = s.conn.QueryRow(ctx, estimateQuery, args...).Scan(&total)
		if err != nil {
			log.Printf("❌ 估算总数也失败: %v", err)
			// 如果估算也失败，设置一个默认值继续查询数据
			total = 0
		}
	}

	// 构建排序子句
	sortField := "timestamp"
	sortOrder := "DESC"
	
	if field, ok := filters["sort_field"].(string); ok {
		// 映射前端字段名到数据库字段名
		fieldMap := map[string]string{
			"timestamp": "timestamp",
			"time_ms":   "time_ms",
			"speed_ms":  "speed_ms",
			"domain":    "domain",
			"client_ip": "client_ip",
		}
		if dbField, exists := fieldMap[field]; exists {
			sortField = dbField
		}
	}
	
	if order, ok := filters["sort_order"].(string); ok {
		if strings.ToUpper(order) == "ASC" {
			sortOrder = "ASC"
		}
	}

	// 优化数据查询 - 移除 FINAL 关键字
	offset := (page - 1) * pageSize

	dataQuery := fmt.Sprintf(`
	       SELECT
	           timestamp,
	           node_id,
	           client_ip,
	           domain,
	           query_type,
	           time_ms,
	           speed_ms,
	           result_count,
	           result_ips,
	           raw_log,
	           group
	       FROM dns_query_log
	       WHERE %s
	       ORDER BY %s %s
	       LIMIT %d OFFSET %d
	       SETTINGS max_execution_time = 25
	   `, whereClause, sortField, sortOrder, pageSize, offset)

	rows, err := s.conn.Query(ctx, dataQuery, args...)
	if err != nil {
		log.Printf("❌ 查询数据失败: %v", err)
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.DNSLog

	// 预分配切片容量
	logs = make([]models.DNSLog, 0, pageSize)

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
			&logCK.Group,
		)
		if err != nil {
			log.Printf("⚠️ 扫描行失败: %v", err)
			continue
		}

		// 转换为通用格式
		logEntry := models.DNSLog{
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
			Group:     logCK.Group,
		}
		logs = append(logs, logEntry)
	}

	// 检查是否有行扫描错误
	if rows.Err() != nil {
		log.Printf("❌ 行迭代错误: %v", rows.Err())
		return nil, 0, rows.Err()
	}

	log.Printf("✅ 成功查询 %d 条日志，总数: %d", len(logs), total)
	return logs, int64(total), nil
}

// GetStats 获取统计信息（实现接口）
func (s *LogMonitorServiceCH) GetStats(nodeID uint, startTime, endTime time.Time) (*models.DNSLogStats, error) {
	ctx := context.Background()
	stats := &models.DNSLogStats{
		TopDomains:  make([]models.DomainStat, 0),
		TopClients:  make([]models.ClientStat, 0),
		HourlyStats: make([]models.HourlyStat, 0),
	}

	// 构建查询条件
	where := "timestamp BETWEEN ? AND ?"
	args := []interface{}{startTime, endTime}

	if nodeID > 0 {
		where += " AND node_id = ?"
		args = append(args, uint32(nodeID))
	}

	// 总查询数
	var totalQueries uint64
	err := s.conn.QueryRow(ctx,
		fmt.Sprintf("SELECT count() FROM dns_query_log WHERE %s", where),
		args...).Scan(&totalQueries)
	if err != nil {
		return nil, err
	}
	stats.TotalQueries = int64(totalQueries)

	if totalQueries == 0 {
		return stats, nil
	}

	// 唯一客户端数
	var uniqueClients uint64
	s.conn.QueryRow(ctx,
		fmt.Sprintf("SELECT uniqExact(client_ip) FROM dns_query_log WHERE %s", where),
		args...).Scan(&uniqueClients)
	stats.UniqueClients = int64(uniqueClients)

	// 唯一域名数
	var uniqueDomains uint64
	s.conn.QueryRow(ctx,
		fmt.Sprintf("SELECT uniqExact(domain) FROM dns_query_log WHERE %s", where),
		args...).Scan(&uniqueDomains)
	stats.UniqueDomains = int64(uniqueDomains)

	// 平均查询时间
	var avgQueryTime *float64
	s.conn.QueryRow(ctx,
		fmt.Sprintf("SELECT avgOrNull(time_ms) FROM dns_query_log WHERE %s", where),
		args...).Scan(&avgQueryTime)
	if avgQueryTime != nil {
		stats.AvgQueryTime = *avgQueryTime
	}

	// 热门域名
	rows, err := s.conn.Query(ctx,
		fmt.Sprintf("SELECT domain, count() as count FROM dns_query_log WHERE %s GROUP BY domain ORDER BY count DESC LIMIT 10", where),
		args...)
	if err == nil {
		for rows.Next() {
			var stat models.DomainStat
			var count uint64
			rows.Scan(&stat.Domain, &count)
			stat.Count = int64(count)
			stats.TopDomains = append(stats.TopDomains, stat)
		}
		rows.Close()
	}

	// 热门客户端
	rows, err = s.conn.Query(ctx,
		fmt.Sprintf("SELECT client_ip, count() as count FROM dns_query_log WHERE %s GROUP BY client_ip ORDER BY count DESC LIMIT 10", where),
		args...)
	if err == nil {
		for rows.Next() {
			var stat models.ClientStat
			var count uint64
			rows.Scan(&stat.ClientIP, &count)
			stat.Count = int64(count)
			stats.TopClients = append(stats.TopClients, stat)
		}
		rows.Close()
	}

	// 按小时统计
	rows, err = s.conn.Query(ctx,
		fmt.Sprintf("SELECT toHour(timestamp) as hour, count() as count FROM dns_query_log WHERE %s GROUP BY hour ORDER BY hour", where),
		args...)
	if err == nil {
		for rows.Next() {
			var stat models.HourlyStat
			var count uint64
			rows.Scan(&stat.Hour, &count)
			stat.Count = int64(count)
			stats.HourlyStats = append(stats.HourlyStats, stat)
		}
		rows.Close()
	}

	return stats, nil
}

// SearchDomains 搜索域名（实现接口）
func (s *LogMonitorServiceCH) SearchDomains(keyword string, limit int) ([]string, error) {
	ctx := context.Background()

	query := "SELECT DISTINCT domain FROM dns_query_log WHERE domain LIKE ? ORDER BY domain LIMIT ?"
	rows, err := s.conn.Query(ctx, query, "%"+keyword+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			continue
		}
		domains = append(domains, domain)
	}

	return domains, nil
}

// CleanOldLogs 清理旧日志（实现接口）
func (s *LogMonitorServiceCH) CleanOldLogs(nodeID uint, days int) error {
	ctx := context.Background()
	cutoffTime := time.Now().AddDate(0, 0, -days)

	where := "timestamp < ?"
	args := []interface{}{cutoffTime}

	if nodeID > 0 {
		where += " AND node_id = ?"
		args = append(args, uint32(nodeID))
	}

	query := fmt.Sprintf("ALTER TABLE dns_query_log DELETE WHERE %s", where)
	err := s.conn.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	log.Printf("✅ 清理完成，删除 %d 天前的日志", days)
	return nil
}

// CheckHealth 检查服务健康状态（实现接口）
func (s *LogMonitorServiceCH) CheckHealth() error {
	ctx := context.Background()
	return s.conn.Ping(ctx)
}

// GetStorageType 获取存储类型（实现接口）
func (s *LogMonitorServiceCH) GetStorageType() string {
	return "clickhouse"
}

// GetStorageInfo 获取存储信息（实现接口）
func (s *LogMonitorServiceCH) GetStorageInfo() map[string]interface{} {
	info := map[string]interface{}{
		"type":      "clickhouse",
		"connected": false,
		"host":      os.Getenv("CLICKHOUSE_HOST"),
		"database":  os.Getenv("CLICKHOUSE_DB"),
	}

	// 检查连接
	if err := s.CheckHealth(); err == nil {
		info["connected"] = true
	}

	return info
}

// EnsureTables 确保数据库表存在（实现接口）
func (s *LogMonitorServiceCH) EnsureTables() error {
	ctx := context.Background()

	createTableSQL := `
    CREATE TABLE IF NOT EXISTS dns_query_log (
        timestamp DateTime64(3),
        date Date DEFAULT toDate(timestamp),
        node_id UInt32,
        client_ip String,
        domain String,
        query_type UInt16,
        time_ms UInt32,
        speed_ms Float32,
        result_count UInt8,
        result_ips Array(String),
        raw_log String
    ) ENGINE = MergeTree()
    PARTITION BY date
    ORDER BY (node_id, timestamp)
    TTL date + INTERVAL 30 DAY
    SETTINGS index_granularity = 8192`

	return s.conn.Exec(ctx, createTableSQL)
}

// GetTableStats 获取表统计信息（实现接口）
func (s *LogMonitorServiceCH) GetTableStats() (map[string]interface{}, error) {
	ctx := context.Background()
	stats := make(map[string]interface{})

	// 获取记录总数
	var totalRecords uint64
	err := s.conn.QueryRow(ctx, "SELECT count() FROM dns_query_log").Scan(&totalRecords)
	if err != nil {
		return nil, err
	}
	stats["total_records"] = totalRecords

	// 获取表大小
	var tableSize uint64
	s.conn.QueryRow(ctx,
		"SELECT sum(bytes_on_disk) FROM system.parts WHERE table = 'dns_query_log'").Scan(&tableSize)
	stats["table_size_bytes"] = tableSize

	return stats, nil
}
