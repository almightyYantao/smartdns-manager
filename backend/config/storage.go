// config/storage.go
package config

import (
	"fmt"
	"os"
)

// StorageConfig 存储配置
type StorageConfig struct {
	Type        string // local 或 s3
	S3AccessKey string
	S3SecretKey string
	S3Region    string
	S3Bucket    string
	S3Endpoint  string // 可选，支持MinIO
}

// LoadStorageConfig 从环境变量加载存储配置
func LoadStorageConfig() *StorageConfig {
	storageType := os.Getenv("BACKUP_STORAGE_TYPE")
	if storageType == "" {
		storageType = "local" // 默认使用本地存储
	}

	return &StorageConfig{
		Type:        storageType,
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Region:    getEnvOrDefault("S3_REGION", "us-east-1"),
		S3Bucket:    os.Getenv("S3_BUCKET"),
		S3Endpoint:  os.Getenv("S3_ENDPOINT"), // 支持MinIO等
	}
}

// Validate 验证配置
func (c *StorageConfig) Validate() error {
	if c.Type == "s3" {
		if c.S3AccessKey == "" {
			return fmt.Errorf("S3_ACCESS_KEY 未设置")
		}
		if c.S3SecretKey == "" {
			return fmt.Errorf("S3_SECRET_KEY 未设置")
		}
		if c.S3Bucket == "" {
			return fmt.Errorf("S3_BUCKET 未设置")
		}
	}
	return nil
}

// IsS3Enabled 是否启用S3存储
func (c *StorageConfig) IsS3Enabled() bool {
	return c.Type == "s3"
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
