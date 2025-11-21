package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"smartdns-manager/config"
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
		MaxOpenConns:    10,
		MaxIdleConns:    5,
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
		MaxOpenConns:    10,
		MaxIdleConns:    5,
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

	// 2. 创建物化视图（可选，用于加速查询）
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
    ORDER BY (date, node_id, timestamp)
    TTL date + INTERVAL 30 DAY
    SETTINGS index_granularity = 8192
    COMMENT 'DNS查询日志表'
    `

	if err := conn.Exec(ctx, createTableSQL); err != nil {
		return fmt.Errorf("创建主表失败: %w", err)
	}

	log.Println("✅ dns_query_log 表创建成功")
	return nil
}

// createMaterializedViews 创建物化视图（用于加速统计查询）
func createMaterializedViews(ctx context.Context, conn driver.Conn) error {
	log.Println("🔨 创建物化视图...")

	// 1. 按小时统计的物化视图
	hourlyStatsSQL := `
    CREATE MATERIALIZED VIEW IF NOT EXISTS dns_stats_hourly
    ENGINE = SummingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, hour, node_id, domain)
    AS SELECT
        toDate(timestamp) as date,
        toHour(timestamp) as hour,
        node_id,
        domain,
        count() as query_count,
        avg(time_ms) as avg_time_ms,
        max(time_ms) as max_time_ms,
        uniqExact(client_ip) as unique_clients
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
    ENGINE = SummingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, node_id, domain)
    AS SELECT
        toDate(timestamp) as date,
        node_id,
        domain,
        count() as query_count
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
    ENGINE = SummingMergeTree()
    PARTITION BY toYYYYMM(date)
    ORDER BY (date, node_id, client_ip)
    AS SELECT
        toDate(timestamp) as date,
        node_id,
        client_ip,
        count() as query_count,
        uniqExact(domain) as unique_domains
    FROM dns_query_log
    GROUP BY date, node_id, client_ip
    `

	if err := conn.Exec(ctx, clientStatsSQL); err != nil {
		log.Printf("⚠️ 创建 dns_client_stats 视图失败: %v", err)
	} else {
		log.Println("✅ dns_client_stats 视图创建成功")
	}

	return nil
}

// CloseClickHouse 关闭连接
func CloseClickHouse() {
	if CHConn != nil {
		CHConn.Close()
		log.Println("✅ ClickHouse 连接已关闭")
	}
}

// CheckClickHouseHealth 健康检查
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

// GetClickHouseVersion 获取 ClickHouse 版本
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
