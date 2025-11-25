package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"smartdns-manager/config"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var CHConn driver.Conn

// InitClickHouse 初始化 ClickHouse 连接
func InitClickHouse() {
	cfg := config.GetClickHouseConfig()
	log.Printf("🔗 正在连接 ClickHouse: %s:%d", cfg.Host, cfg.Port)

	// 第一步：连接到 ClickHouse（不指定数据库）
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: clickhouse.Auth{
			Username: cfg.Username,
			Password: cfg.Password,
		},
		DialTimeout: 10 * time.Second,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		MaxOpenConns:    20, // 增加连接数
		MaxIdleConns:    10, // 增加空闲连接数
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		log.Fatal("❌ 连接 ClickHouse 失败:", err)
	}

	// 测试连接
	ctx := context.Background()
	if err := conn.Ping(ctx); err != nil {
		log.Fatal("❌ Ping ClickHouse 失败:", err)
	}
	log.Println("✅ ClickHouse 连接成功")

	// 关闭初始连接
	conn.Close()

	CHConn, err = clickhouse.Open(&clickhouse.Options{
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
		MaxOpenConns:    20,
		MaxIdleConns:    10,
		ConnMaxLifetime: time.Hour,
	})
	if err != nil {
		log.Fatal("❌ 连接数据库失败:", err)
	}

	if err := createTablesIfNotExists(ctx, CHConn); err != nil {
		CHConn.Close()
		log.Fatal("❌ 创建表失败:", err)
	}

	log.Printf("✅ ClickHouse 初始化完成 - 数据库: %s", cfg.Database)
}

// createTablesIfNotExists 创建表结构
func createTablesIfNotExists(ctx context.Context, conn driver.Conn) error {
	log.Println("📋 开始创建表结构...")

	// 1. 创建主表
	if err := createMainTable(ctx, conn); err != nil {
		return err
	}

	// 2. 创建索引
	if err := createIndexes(ctx, conn); err != nil {
		log.Printf("⚠️ 创建索引失败: %v", err)
	}

	// 3. 创建物化视图（可选，用于加速查询）
	if err := createMaterializedViews(ctx, conn); err != nil {
		log.Printf("⚠️ 创建物化视图失败（可忽略）: %v", err)
	}

	log.Println("✅ 表结构创建完成")
	return nil
}

// createMainTable 创建主表
func createMainTable(ctx context.Context, conn driver.Conn) error {
	log.Println("🔨 创建 dns_query_log 表...")

	createTableSQL := `
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
    PRIMARY KEY (date, node_id)
    ORDER BY (date, node_id, timestamp, client_ip)
    TTL date + INTERVAL 90 DAY
    SETTINGS 
        index_granularity = 8192,
        merge_with_ttl_timeout = 86400,
        max_parts_in_total = 100000,
        parts_to_delay_insert = 150,
        parts_to_throw_insert = 300,
        max_compress_block_size = 1048576,
        min_compress_block_size = 65536
    COMMENT 'DNS查询日志表 - 大数据量优化版本'
    `

	if err := conn.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("创建主表失败: %w", err)
	}

	log.Println("✅ dns_query_log 表创建成功")
	return nil
}

// createIndexes 创建索引
func createIndexes(ctx context.Context, conn driver.Conn) error {
	log.Println("🔨 创建索引...")

	indexes := []struct {
		name string
		sql  string
	}{
		{
			name: "idx_timestamp",
			sql:  "ALTER TABLE dns_query_log ADD INDEX IF NOT EXISTS idx_timestamp (timestamp) TYPE minmax GRANULARITY 1",
		},
		{
			name: "idx_domain",
			sql:  "ALTER TABLE dns_query_log ADD INDEX IF NOT EXISTS idx_domain (domain) TYPE bloom_filter GRANULARITY 1",
		},
		{
			name: "idx_client_ip",
			sql:  "ALTER TABLE dns_query_log ADD INDEX IF NOT EXISTS idx_client_ip (client_ip) TYPE bloom_filter GRANULARITY 1",
		},
		{
			name: "idx_query_type",
			sql:  "ALTER TABLE dns_query_log ADD INDEX IF NOT EXISTS idx_query_type (query_type) TYPE set(100) GRANULARITY 1",
		},
		{
			name: "idx_node_timestamp",
			sql:  "ALTER TABLE dns_query_log ADD INDEX IF NOT EXISTS idx_node_timestamp (node_id, timestamp) TYPE minmax GRANULARITY 1",
		},
		{
			name: "idx_domain_fuzzy",
			sql:  "ALTER TABLE dns_query_log ADD INDEX IF NOT EXISTS idx_domain_fuzzy (domain) TYPE ngrambf_v1(3, 256, 2, 0) GRANULARITY 1",
		},
	}

	for _, idx := range indexes {
		log.Printf("  创建索引: %s", idx.name)
		if err := conn.Exec(ctx, idx.sql); err != nil {
			log.Printf("  ⚠️ 创建索引 %s 失败: %v", idx.name, err)
		} else {
			log.Printf("  ✅ 索引 %s 创建成功", idx.name)
		}
	}

	return nil
}

// createMaterializedViews 创建物化视图（用于加速统计查询）
func createMaterializedViews(ctx context.Context, conn driver.Conn) error {
	log.Println("🔨 创建物化视图...")

	// 1. 按小时统计的物化视图
	hourlyStatsSQL := `
    CREATE MATERIALIZED VIEW IF NOT EXISTS dns_stats_hourly
    ENGINE = AggregatingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, hour, node_id, domain)
    TTL date + INTERVAL 30 DAY
    AS SELECT
        toDate(timestamp) as date,
        toHour(timestamp) as hour,
        node_id,
        domain,
        countState() as query_count,
        avgState(time_ms) as avg_time_ms,
        maxState(time_ms) as max_time_ms,
        minState(time_ms) as min_time_ms,
        uniqState(client_ip) as unique_clients
    FROM dns_query_log
    GROUP BY date, hour, node_id, domain
    `

	if err := conn.Exec(ctx, hourlyStatsSQL); err != nil {
		log.Printf("⚠️ 创建 dns_stats_hourly 视图失败: %v", err)
	} else {
		log.Println("✅ dns_stats_hourly 视图创建成功")
	}

	// 2. 热门域名统计的物化视图
	topDomainsSQL := `
    CREATE MATERIALIZED VIEW IF NOT EXISTS dns_top_domains
    ENGINE = AggregatingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, node_id, domain)
    TTL date + INTERVAL 30 DAY
    AS SELECT
        toDate(timestamp) as date,
        node_id,
        domain,
        countState() as query_count,
        uniqState(client_ip) as unique_clients,
        avgState(time_ms) as avg_response_time
    FROM dns_query_log
    GROUP BY date, node_id, domain
    `

	if err := conn.Exec(ctx, topDomainsSQL); err != nil {
		log.Printf("⚠️ 创建 dns_top_domains 视图失败: %v", err)
	} else {
		log.Println("✅ dns_top_domains 视图创建成功")
	}

	// 3. 客户端统计的物化视图
	clientStatsSQL := `
    CREATE MATERIALIZED VIEW IF NOT EXISTS dns_client_stats
    ENGINE = AggregatingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, node_id, client_ip)
    TTL date + INTERVAL 30 DAY
    AS SELECT
        toDate(timestamp) as date,
        node_id,
        client_ip,
        countState() as query_count,
        uniqState(domain) as unique_domains,
        avgState(time_ms) as avg_response_time
    FROM dns_query_log
    GROUP BY date, node_id, client_ip
    `

	if err := conn.Exec(ctx, clientStatsSQL); err != nil {
		log.Printf("⚠️ 创建 dns_client_stats 视图失败: %v", err)
	} else {
		log.Println("✅ dns_client_stats 视图创建成功")
	}

	// 4. 每日摘要统计 - 新增
	dailySummarySQL := `
    CREATE MATERIALIZED VIEW IF NOT EXISTS dns_daily_summary
    ENGINE = ReplacingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, node_id)
    TTL date + INTERVAL 365 DAY
    AS SELECT
        toDate(timestamp) as date,
        node_id,
        count() as total_queries,
        uniqExact(domain) as unique_domains,
        uniqExact(client_ip) as unique_clients,
        avg(time_ms) as avg_response_time,
        quantile(0.95)(time_ms) as p95_response_time,
        countIf(time_ms > 1000) as slow_queries
    FROM dns_query_log
    GROUP BY date, node_id
    `

	if err := conn.Exec(ctx, dailySummarySQL); err != nil {
		log.Printf("⚠️ 创建 dns_daily_summary 视图失败: %v", err)
	} else {
		log.Println("✅ dns_daily_summary 视图创建成功")
	}

	return nil
}

// OptimizeTable
func OptimizeTable() error {
	if CHConn == nil {
		return fmt.Errorf("ClickHouse 连接未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Println("🔧 开始优化表...")

	// 优化主表
	if err := CHConn.Exec(ctx, "OPTIMIZE TABLE dns_query_log FINAL"); err != nil {
		log.Printf("⚠️ 优化主表失败: %v", err)
		return err
	}

	// 优化物化视图
	views := []string{
		"dns_stats_hourly",
		"dns_top_domains",
		"dns_client_stats",
		"dns_daily_summary",
	}

	for _, view := range views {
		if err := CHConn.Exec(ctx, fmt.Sprintf("OPTIMIZE TABLE %s FINAL", view)); err != nil {
			log.Printf("⚠️ 优化视图 %s 失败: %v", view, err)
		}
	}

	log.Println("✅ 表优化完成")
	return nil
}

// GetTableStats 获取表统计信息
func GetTableStats() (map[string]interface{}, error) {
	if CHConn == nil {
		return nil, fmt.Errorf("ClickHouse 连接未初始化")
	}

	ctx := context.Background()
	stats := make(map[string]interface{})

	// 获取主表统计
	var totalRows, totalSize uint64
	err := CHConn.QueryRow(ctx, `
        SELECT 
            sum(rows) as total_rows,
            sum(bytes_on_disk) as total_size
        FROM system.parts 
        WHERE table = 'dns_query_log' AND active =1
    `).Scan(&totalRows, &totalSize)

	if err != nil {
		return nil, err
	}

	stats["total_rows"] = totalRows
	stats["total_size_bytes"] = totalSize
	stats["total_size_mb"] = float64(totalSize) / 1024 / 1024

	// 获取分区信息
	var partitions uint64
	err = CHConn.QueryRow(ctx, `
        SELECT count(DISTINCT partition) 
        FROM system.parts 
        WHERE table = 'dns_query_log' AND active = 1
    `).Scan(&partitions)

	if err == nil {
		stats["partitions"] = partitions
	}

	return stats, nil
}

// CleanOldPartitions
func CleanOldPartitions(daysToKeep int) error {
	if CHConn == nil {
		return fmt.Errorf("ClickHouse 连接未初始化")
	}

	ctx := context.Background()

	// 计算要删除的分区
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep).Format("2006-01")

	log.Printf("🗑️ 清理 %s 之前的分区...", cutoffDate)

	sql := fmt.Sprintf("ALTER TABLE dns_query_log DROP PARTITION '%s'", cutoffDate)
	if err := CHConn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("清理分区失败: %w", err)
	}

	log.Println("✅ 旧分区清理完成")
	return nil
}

func CloseClickHouse() {
	if CHConn != nil {
		CHConn.Close()
		log.Println("✅ ClickHouse 连接已关闭")
	}
}

func CheckClickHouseHealth() error {
	if CHConn == nil {
		return fmt.Errorf("ClickHouse 连接未初始化")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := CHConn.Ping(ctx); err != nil {
		return fmt.Errorf("ClickHouse 健康检查失败: %w", err)
	}

	return nil
}

func GetClickHouseVersion() (string, error) {
	if CHConn == nil {
		return "", fmt.Errorf("ClickHouse 连接未初始化")
	}

	ctx := context.Background()
	var version string
	err := CHConn.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", err
	}

	return version, nil
}
