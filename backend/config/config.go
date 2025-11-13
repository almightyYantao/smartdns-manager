package config

import (
	"log"
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
			// 使用绝对路径，方便 Docker 挂载
			DBPath: getEnv("DB_PATH", "/app/data/smartdns.db"),
		}

		// 打印配置信息（生产环境可以去掉敏感信息）
		log.Printf("Config loaded - ServerPort: %s, DBPath: %s", config.ServerPort, config.DBPath)
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
