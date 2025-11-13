package config

import (
	"os"
)

type Config struct {
	ServerPort string
	JWTSecret  string
	DBPath     string
}

var config *Config

// GetConfig 获取配置
func GetConfig() *Config {
	if config == nil {
		config = &Config{
			ServerPort: getEnv("SERVER_PORT", "3001"),
			JWTSecret:  getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			DBPath:     getEnv("DB_PATH", "smartdns.db"),
		}
	}
	return config
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
