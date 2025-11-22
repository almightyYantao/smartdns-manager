package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	NodeID        uint32           `json:"node_id"`
	NodeName      string           `json:"node_name"`
	LogFile       string           `json:"log_file"`
	BatchSize     int              `json:"batch_size"`
	FlushInterval time.Duration    `json:"flush_interval"`
	ClickHouse    ClickHouseConfig `json:"clickhouse"`
	LogConfig     LogConfig        `json:"log_config"`
}

type LogConfig struct {
	LogDir     string `json:"log_dir"`     // 日志目录
	MaxDays    int    `json:"max_days"`    // 保留天数
	EnableFile bool   `json:"enable_file"` // 是否启用文件日志
}

type ClickHouseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func Load() (*Config, error) {
	nodeIDStr := getEnv("NODE_ID", "")
	if nodeIDStr == "" {
		return nil, fmt.Errorf("NODE_ID 环境变量必须设置")
	}

	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("NODE_ID 必须是数字")
	}

	return &Config{
		NodeID:        uint32(nodeID),
		NodeName:      getEnv("NODE_NAME", fmt.Sprintf("node-%d", nodeID)),
		LogFile:       getEnv("LOG_FILE", "/var/log/smartdns/audit.log"),
		BatchSize:     getEnvInt("BATCH_SIZE", 1000),
		FlushInterval: time.Duration(getEnvInt("FLUSH_INTERVAL_SEC", 2)) * time.Second,
		ClickHouse: ClickHouseConfig{
			Host:     getEnv("CLICKHOUSE_HOST", "localhost"),
			Port:     getEnvInt("CLICKHOUSE_PORT", 9000),
			Database: getEnv("CLICKHOUSE_DB", "smartdns_logs"),
			Username: getEnv("CLICKHOUSE_USER", "default"),
			Password: getEnv("CLICKHOUSE_PASSWORD", ""),
		},
		LogConfig: LogConfig{
			LogDir:     getEnv("AGENT_LOG_DIR", "/var/log/smartdns-agent"),
			MaxDays:    getEnvInt("AGENT_LOG_MAX_DAYS", 7),
			EnableFile: getEnvBool("AGENT_LOG_ENABLE_FILE", true),
		},
	}, nil
}

func getEnvBool(key string, defaultValue bool) bool {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.ParseBool(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if valueStr := os.Getenv(key); valueStr != "" {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}
