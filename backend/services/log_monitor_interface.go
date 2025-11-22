package services

import (
	"smartdns-manager/database"
	"smartdns-manager/models"
	"time"
)

// LogMonitorInterface 日志监控服务接口
type LogMonitorInterface interface {
	GetLogs(page, pageSize int, filters map[string]interface{}) ([]models.DNSLog, int64, error)
	GetStats(nodeID uint, startTime, endTime time.Time) (*models.DNSLogStats, error)
	SearchDomains(keyword string, limit int) ([]string, error)
	CleanOldLogs(nodeID uint, days int) error
	CheckHealth() error
	GetStorageType() string
	GetStorageInfo() map[string]interface{}
	EnsureTables() error
	GetTableStats() (map[string]interface{}, error)
}

// NewLogMonitorService 创建日志监控服务工厂函数
func NewLogMonitorService() LogMonitorInterface {
	return NewLogMonitorServiceCH(database.CHConn)
}
