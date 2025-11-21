package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type ClickHouseConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Database string
	Username string
	Password string
}

func GetClickHouseConfig() *ClickHouseConfig {
	logStorageType := strings.ToLower(getEnv("LOG_STORAGE_TYPE", "sqlite"))
	clickhouseEnabled := logStorageType == "clickhouse"
	return &ClickHouseConfig{
		Enabled:  clickhouseEnabled,
		Host:     getEnv("CLICKHOUSE_HOST", "localhost"),
		Port:     getEnvAsInt("CLICKHOUSE_PORT", 9000),
		Database: getEnv("CLICKHOUSE_DB", "smartdns_logs"),
		Username: getEnv("CLICKHOUSE_USER", "smartdns"),
		Password: getEnv("CLICKHOUSE_PASSWORD", "smartdns"),
	}
}

// IsClickHouseEnabled 是否启用 ClickHouse
func IsClickHouseEnabled() bool {
	return GetClickHouseConfig().Enabled
}

// getEnvAsInt 获取环境变量并转换为整数
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Warning: Invalid integer value for %s: %s, using default %d", key, valueStr, defaultValue)
		return defaultValue
	}

	return value
}
