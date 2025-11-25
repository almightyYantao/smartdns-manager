// migration.go
package database

import (
	"context"
	"fmt"
	"log"
	"sort"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Migration 迁移结构
type Migration struct {
	Version     int
	Description string
	SQL         string
	Execute     func(ctx context.Context, conn driver.Conn) error // 可选：复杂迁移逻辑
}

// 所有迁移定义
var migrations = []Migration{
	{
		Version:     1,
		Description: "创建初始表结构",
		Execute:     migration001CreateInitialTable,
	},
	{
		Version:     2,
		Description: "添加 group 字段",
		SQL:         `ALTER TABLE dns_query_log ADD COLUMN IF NOT EXISTS group String DEFAULT '' COMMENT '所属组'`,
	},
}

// 创建迁移记录表
func createMigrationTable(ctx context.Context, conn driver.Conn) error {
	sql := `
    CREATE TABLE IF NOT EXISTS schema_migrations (
        version UInt32,
        description String,
        executed_at DateTime DEFAULT now()
    ) ENGINE = MergeTree()
    ORDER BY version
    `
	return conn.Exec(ctx, sql)
}

// 获取已执行的迁移版本
func getExecutedMigrations(ctx context.Context, conn driver.Conn) (map[int]bool, error) {
	executed := make(map[int]bool)

	rows, err := conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return executed, err
	}
	defer rows.Close()

	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			continue
		}
		executed[version] = true
	}

	return executed, nil
}

// 执行迁移
func runMigrations(ctx context.Context, conn driver.Conn) error {
	log.Println("🔄 开始执行数据库迁移...")

	// 创建迁移记录表
	if err := createMigrationTable(ctx, conn); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	// 获取已执行的迁移
	executed, err := getExecutedMigrations(ctx, conn)
	if err != nil {
		return fmt.Errorf("获取迁移记录失败: %w", err)
	}

	// 按版本号排序
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	// 执行未执行的迁移
	for _, migration := range migrations {
		if executed[migration.Version] {
			log.Printf("⏭️  迁移 v%d 已执行，跳过", migration.Version)
			continue
		}

		log.Printf("🚀 执行迁移 v%d: %s", migration.Version, migration.Description)

		if err := executeMigration(ctx, conn, migration); err != nil {
			return fmt.Errorf("迁移 v%d 执行失败: %w", migration.Version, err)
		}

		// 记录迁移执行
		recordSQL := `INSERT INTO schema_migrations (version, description) VALUES (?, ?)`
		if err := conn.Exec(ctx, recordSQL, migration.Version, migration.Description); err != nil {
			return fmt.Errorf("记录迁移失败: %w", err)
		}

		log.Printf("✅ 迁移 v%d 执行成功", migration.Version)
	}

	log.Println("✅ 所有迁移执行完成")
	return nil
}

// 执行单个迁移
func executeMigration(ctx context.Context, conn driver.Conn, migration Migration) error {
	if migration.Execute != nil {
		return migration.Execute(ctx, conn)
	}

	if migration.SQL != "" {
		return conn.Exec(ctx, migration.SQL)
	}

	return fmt.Errorf("迁移 v%d 没有定义执行逻辑", migration.Version)
}

// 迁移 v1：创建初始表
func migration001CreateInitialTable(ctx context.Context, conn driver.Conn) error {
	sql := `
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
    COMMENT 'DNS查询日志表'
    `
	return conn.Exec(ctx, sql)
}
