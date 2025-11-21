package services

import (
	"smartdns-manager/models"
	"time"
)

// LogMonitorServiceInterface 日志监控服务接口
type LogMonitorServiceInterface interface {
	// 启动节点监控
	StartNodeMonitor(nodeID uint) error

	// 停止节点监控
	StopNodeMonitor(nodeID uint) error

	// 获取节点监控状态
	GetNodeMonitorStatus(nodeID uint) (bool, error)

	// 查询日志列表
	GetLogs(page, pageSize int, filters map[string]interface{}) ([]models.DNSLog, int64, error)

	// 获取节点统计信息
	GetNodeStats(nodeID uint, startTime, endTime time.Time) (*models.DNSLogStats, error)

	// 清理节点旧日志
	CleanNodeLogs(nodeID uint, days int) error

	// 停止所有监控
	StopAll()
}
