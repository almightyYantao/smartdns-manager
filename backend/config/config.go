package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort  string
	JWTSecret   string
	DBPath      string
	BackupPath  string
	InitVersion string
	InitBaseURL string
	StatusTime  string
}

var config *Config

// GetConfig 获取配置
func GetConfig() *Config {
	if config == nil {

		if err := godotenv.Load(); err != nil {
			log.Println("Warning: .env file not found, using defaults")
		}

		config = &Config{
			ServerPort: getEnv("SERVER_PORT", "3001"),
			JWTSecret:  getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			// 使用绝对路径，方便 Docker 挂载
			DBPath:      getEnv("DB_PATH", "/app/data/smartdns.db"),
			BackupPath:  getEnv("BACKUP_PATH", "/etc/smartdns/backups"),
			InitVersion: getEnv("INIT_VERSION", "1.2024.06.12-2222"),
			StatusTime:  getEnv("STATUS_CHECK_TIME", "10"),
			InitBaseURL: getEnv("INIT_BASE_URL", "https://github.com/pymumu/smartdns/releases/download/Release46"),
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
