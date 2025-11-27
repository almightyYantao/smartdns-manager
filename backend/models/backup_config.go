package models

import (
	"time"
)

// BackupConfig 数据库备份配置表
type BackupConfig struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	Name              string    `gorm:"type:varchar(255);not null" json:"name"`                // 配置名称
	Enabled           bool      `gorm:"default:false" json:"enabled"`                         // 是否启用
	BackupType        string    `gorm:"type:varchar(20);default:'database'" json:"backup_type"` // 备份类型：database, files
	
	// 备份周期配置
	Schedule          string    `gorm:"type:varchar(50);not null" json:"schedule"`            // cron表达式
	RetentionDays     int       `gorm:"default:30" json:"retention_days"`                     // 保留天数
	
	// S3配置
	S3Enabled         bool      `gorm:"default:false" json:"s3_enabled"`                      // 是否启用S3
	S3AccessKey       string    `gorm:"type:varchar(255)" json:"s3_access_key,omitempty"`
	S3SecretKey       string    `gorm:"type:varchar(255)" json:"s3_secret_key,omitempty"`
	S3Region          string    `gorm:"type:varchar(100)" json:"s3_region,omitempty"`
	S3Bucket          string    `gorm:"type:varchar(255)" json:"s3_bucket,omitempty"`
	S3Endpoint        string    `gorm:"type:varchar(500)" json:"s3_endpoint,omitempty"`       // 自定义S3端点(如MinIO)
	S3Prefix          string    `gorm:"type:varchar(255);default:'smartdns-backups'" json:"s3_prefix"` // S3存储前缀
	
	// 本地存储配置
	LocalPath         string    `gorm:"type:varchar(500)" json:"local_path,omitempty"`        // 本地备份路径
	
	// 压缩配置
	CompressionEnabled bool     `gorm:"default:true" json:"compression_enabled"`              // 是否压缩
	CompressionLevel   int      `gorm:"default:6" json:"compression_level"`                   // 压缩级别(0-9)
	
	// 加密配置
	EncryptionEnabled bool      `gorm:"default:false" json:"encryption_enabled"`             // 是否加密
	EncryptionKey     string    `gorm:"type:varchar(255)" json:"encryption_key,omitempty"`   // 加密密钥
	
	// 通知配置
	NotifyOnSuccess   bool      `gorm:"default:false" json:"notify_on_success"`              // 成功时通知
	NotifyOnFailure   bool      `gorm:"default:true" json:"notify_on_failure"`               // 失败时通知
	NotificationChannels string `gorm:"type:text" json:"notification_channels,omitempty"`   // 通知渠道JSON数组
	
	// 状态信息
	LastBackupAt      *time.Time `json:"last_backup_at,omitempty"`                           // 最后备份时间
	LastBackupStatus  string     `gorm:"type:varchar(20)" json:"last_backup_status,omitempty"` // 最后备份状态
	LastBackupSize    int64      `json:"last_backup_size,omitempty"`                         // 最后备份大小
	LastBackupError   string     `gorm:"type:text" json:"last_backup_error,omitempty"`       // 最后备份错误
	NextBackupAt      *time.Time `json:"next_backup_at,omitempty"`                           // 下次备份时间
	
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// BackupHistory 备份历史记录表
type BackupHistory struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	ConfigID         uint      `gorm:"not null;index" json:"config_id"`                    // 关联的配置ID
	BackupType       string    `gorm:"type:varchar(20)" json:"backup_type"`               // 备份类型
	Status           string    `gorm:"type:varchar(20)" json:"status"`                    // 状态：running, success, failed
	
	// 文件信息
	FileName         string    `gorm:"type:varchar(255)" json:"file_name"`                // 备份文件名
	FileSize         int64     `json:"file_size"`                                         // 文件大小
	FilePath         string    `gorm:"type:varchar(500)" json:"file_path,omitempty"`     // 本地文件路径
	
	// S3信息
	S3Key            string    `gorm:"type:varchar(500)" json:"s3_key,omitempty"`        // S3对象键
	S3Bucket         string    `gorm:"type:varchar(255)" json:"s3_bucket,omitempty"`     // S3存储桶
	S3Region         string    `gorm:"type:varchar(100)" json:"s3_region,omitempty"`     // S3区域
	
	// 执行信息
	StartedAt        time.Time `json:"started_at"`                                        // 开始时间
	CompletedAt      *time.Time `json:"completed_at,omitempty"`                          // 完成时间
	Duration         int64     `json:"duration,omitempty"`                               // 执行时长(秒)
	ErrorMessage     string    `gorm:"type:text" json:"error_message,omitempty"`         // 错误消息
	
	// 备份内容摘要
	DatabaseSize     int64     `json:"database_size,omitempty"`                          // 原始数据库大小
	CompressionRatio float64   `json:"compression_ratio,omitempty"`                      // 压缩比
	
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// 关联
	Config           BackupConfig `gorm:"foreignKey:ConfigID" json:"config,omitempty"`
}

// BackupRestoreRequest 备份恢复请求
type BackupRestoreRequest struct {
	BackupHistoryID uint   `json:"backup_history_id" binding:"required"`
	RestoreType     string `json:"restore_type" binding:"required"` // full, selective
	BackupPassword  string `json:"backup_password,omitempty"`       // 备份密码（如果加密）
}

// BackupConfigRequest 备份配置请求
type BackupConfigRequest struct {
	Name                 string   `json:"name" binding:"required"`
	Enabled              bool     `json:"enabled"`
	BackupType           string   `json:"backup_type" binding:"required"`
	Schedule             string   `json:"schedule" binding:"required"`
	RetentionDays        int      `json:"retention_days"`
	S3Enabled            bool     `json:"s3_enabled"`
	S3AccessKey          string   `json:"s3_access_key,omitempty"`
	S3SecretKey          string   `json:"s3_secret_key,omitempty"`
	S3Region             string   `json:"s3_region,omitempty"`
	S3Bucket             string   `json:"s3_bucket,omitempty"`
	S3Endpoint           string   `json:"s3_endpoint,omitempty"`
	S3Prefix             string   `json:"s3_prefix,omitempty"`
	LocalPath            string   `json:"local_path,omitempty"`
	CompressionEnabled   bool     `json:"compression_enabled"`
	CompressionLevel     int      `json:"compression_level"`
	EncryptionEnabled    bool     `json:"encryption_enabled"`
	EncryptionKey        string   `json:"encryption_key,omitempty"`
	NotifyOnSuccess      bool     `json:"notify_on_success"`
	NotifyOnFailure      bool     `json:"notify_on_failure"`
	NotificationChannels []uint   `json:"notification_channels,omitempty"`
}

// BackupStats 备份统计信息
type BackupStats struct {
	TotalConfigs       int     `json:"total_configs"`
	ActiveConfigs      int     `json:"active_configs"`
	TotalBackups       int     `json:"total_backups"`
	SuccessfulBackups  int     `json:"successful_backups"`
	FailedBackups      int     `json:"failed_backups"`
	TotalSize          int64   `json:"total_size"`
	LastBackupAt       *time.Time `json:"last_backup_at"`
	NextBackupAt       *time.Time `json:"next_backup_at"`
	SuccessRate        float64 `json:"success_rate"`
}