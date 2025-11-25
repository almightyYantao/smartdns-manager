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
	// 执行迁移替代原来的 createTablesIfNotExists
	if err := runMigrations(ctx, CHConn); err != nil {
		CHConn.Close()
		log.Fatal("❌ 数据库迁移失败:", err)
	}

	// 创建索引和视图
	if err := createIndexes(ctx, CHConn); err != nil {
		log.Printf("⚠️ 创建索引失败: %v", err)
	}

	if err := createMaterializedViews(ctx, CHConn); err != nil {
		log.Printf("⚠️ 创建物化视图失败（可忽略）: %v", err)
	}

	log.Printf("✅ ClickHouse 初始化完成 - 数据库: %s", cfg.Database)
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
