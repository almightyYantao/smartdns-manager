package sender

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"smartdns-log-agent/config"
	"smartdns-log-agent/models"
)

type ClickHouseSender struct {
	conn driver.Conn
}

func NewClickHouseSender(cfg config.ClickHouseConfig) (*ClickHouseSender, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout: 10 * time.Second,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, err
	}

	// 测试连接
	ctx := context.Background()
	if err := conn.Ping(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	sender := &ClickHouseSender{conn: conn}

	// 自动创建表
	if err := sender.createTables(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	return sender, nil
}

// createTables 创建必要的表
func (s *ClickHouseSender) createTables(ctx context.Context) error {
	log.Println("🔨 检查并创建 ClickHouse 表结构...")

	// 创建 DNS 查询日志表
	createDNSTableSQL := `
    CREATE TABLE IF NOT EXISTS dns_query_log (
        timestamp DateTime64(3) COMMENT '查询时间（毫秒精度）',
        date Date DEFAULT toDate(timestamp) COMMENT '日期（用于分区）',
        node_id UInt32 COMMENT '节点ID',
        client_ip String COMMENT '客户端IP',
        domain String COMMENT '查询域名',
        query_type UInt16 COMMENT '查询类型（1=A, 28=AAAA, 65=HTTPS等）',
        time_ms UInt32 COMMENT '查询耗时（毫秒）',
        speed_ms Float32 COMMENT '速度检查耗时（毫秒）',
        result_count UInt8 COMMENT '返回IP数量',
        result_ips Array(String) COMMENT '返回的IP列表',
        raw_log String COMMENT '原始日志'
    ) ENGINE = MergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, node_id, timestamp)
    TTL date + INTERVAL 30 DAY
    SETTINGS index_granularity = 8192
    COMMENT 'DNS查询日志表'
    `

	if err := s.conn.Exec(ctx, createDNSTableSQL); err != nil {
		return fmt.Errorf("创建 dns_query_log 表失败: %w", err)
	}
	log.Println("✅ dns_query_log 表创建成功")

	// 创建物化视图（可选，用于加速查询）
	if err := s.createMaterializedViews(ctx); err != nil {
		log.Printf("⚠️ 创建物化视图失败（可忽略）: %v", err)
	}

	return nil
}

// createMaterializedViews 创建物化视图
func (s *ClickHouseSender) createMaterializedViews(ctx context.Context) error {
	log.Println("🔨 创建物化视图...")

	// 1. 按小时统计的物化视图
	hourlyStatsSQL := `
    CREATE MATERIALIZED VIEW IF NOT EXISTS dns_hourly_stats
    ENGINE = SummingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, hour, node_id, domain)
    POPULATE
    AS SELECT
        toDate(timestamp) as date,
        toHour(timestamp) as hour,
        node_id,
        domain,
        client_ip,
        count() as query_count,
        avg(time_ms) as avg_time_ms,
        max(time_ms) as max_time_ms,
        min(time_ms) as min_time_ms,
        uniqExact(client_ip) as unique_clients
    FROM dns_query_log
    GROUP BY date, hour, node_id, domain, client_ip
    `

	if err := s.conn.Exec(ctx, hourlyStatsSQL); err != nil {
		log.Printf("⚠️ 创建 dns_hourly_stats 视图失败: %v", err)
	} else {
		log.Println("✅ dns_hourly_stats 视图创建成功")
	}

	// 2. 热门域名统计的物化视图
	topDomainsSQL := `
    CREATE MATERIALIZED VIEW IF NOT EXISTS dns_top_domains
    ENGINE = SummingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, node_id, domain)
    POPULATE
    AS SELECT
        toDate(timestamp) as date,
        node_id,
        domain,
        count() as query_count,
        uniqExact(client_ip) as unique_clients,
        avg(time_ms) as avg_time_ms
    FROM dns_query_log
    GROUP BY date, node_id, domain
    `

	if err := s.conn.Exec(ctx, topDomainsSQL); err != nil {
		log.Printf("⚠️ 创建 dns_top_domains 视图失败: %v", err)
	} else {
		log.Println("✅ dns_top_domains 视图创建成功")
	}

	// 3. 客户端统计的物化视图
	clientStatsSQL := `
    CREATE MATERIALIZED VIEW IF NOT EXISTS dns_client_stats
    ENGINE = SummingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, node_id, client_ip)
    POPULATE
    AS SELECT
        toDate(timestamp) as date,
        node_id,
        client_ip,
        count() as query_count,
        uniqExact(domain) as unique_domains,
        avg(time_ms) as avg_time_ms
    FROM dns_query_log
    GROUP BY date, node_id, client_ip
    `

	if err := s.conn.Exec(ctx, clientStatsSQL); err != nil {
		log.Printf("⚠️ 创建 dns_client_stats 视图失败: %v", err)
	} else {
		log.Println("✅ dns_client_stats 视图创建成功")
	}

	return nil
}

// checkTableExists 检查表是否存在
func (s *ClickHouseSender) checkTableExists(ctx context.Context, tableName string) (bool, error) {
	query := `SELECT count() FROM system.tables WHERE database = currentDatabase() AND name = ?`

	var count uint64
	err := s.conn.QueryRow(ctx, query, tableName).Scan(&count)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetTableInfo 获取表信息（调试用）
func (s *ClickHouseSender) GetTableInfo(ctx context.Context) error {
	query := `
    SELECT 
        name,
        engine,
        total_rows,
        total_bytes,
        formatReadableSize(total_bytes) as size
    FROM system.tables 
    WHERE database = currentDatabase() AND name LIKE 'dns_%'
    ORDER BY name
    `

	rows, err := s.conn.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	log.Println("📊 ClickHouse 表信息:")
	for rows.Next() {
		var name, engine, size string
		var totalRows, totalBytes uint64

		if err := rows.Scan(&name, &engine, &totalRows, &totalBytes, &size); err != nil {
			continue
		}

		log.Printf("  - %s (%s): %d 行, %s", name, engine, totalRows, size)
	}

	return nil
}

func (s *ClickHouseSender) SendBatch(records []models.DNSLogRecord) error {
	if len(records) == 0 {
		return nil
	}

	ctx := context.Background()
	batch, err := s.conn.PrepareBatch(ctx,
		`INSERT INTO dns_query_log (
            timestamp, date, node_id, client_ip, domain, query_type, 
            time_ms, speed_ms, result_count, result_ips, raw_log
        )`)
	if err != nil {
		return err
	}

	for _, record := range records {
		err := batch.Append(
			record.Timestamp,
			record.Date,
			record.NodeID,
			record.ClientIP,
			record.Domain,
			record.QueryType,
			record.TimeMs,
			record.SpeedMs,
			record.ResultCount,
			record.ResultIPs,
			record.RawLog,
		)
		if err != nil {
			return err
		}
	}

	return batch.Send()
}

func (s *ClickHouseSender) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}
