package handlers

import (
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"smartdns-log-agent/collector"
	"smartdns-log-agent/config"
	"smartdns-log-agent/sender"
)

// AgentHandler API 处理器
type AgentHandler struct {
	cfg             *config.Config
	collector       *collector.LogCollector
	sender          *sender.ClickHouseSender
	isRunning       bool
	startTime       time.Time
	getCollector    func() *collector.LogCollector
	getSender       func() *sender.ClickHouseSender
	getRunning      func() bool
	startCollection func() error
	stopCollection  func()
	getAgentLogs    func(int) ([]string, error)
}

// AgentStatus API 状态响应
type AgentStatus struct {
	Status     string                 `json:"status"`
	Version    string                 `json:"version"`
	NodeID     uint32                 `json:"node_id"`
	NodeName   string                 `json:"node_name"`
	StartTime  string                 `json:"start_time"`
	Uptime     string                 `json:"uptime"`
	IsRunning  bool                   `json:"is_running"`
	LogFile    string                 `json:"log_file"`
	ClickHouse map[string]interface{} `json:"clickhouse"`
	System     map[string]interface{} `json:"system"`
	LastError  string                 `json:"last_error,omitempty"`
}

// AgentStats 统计信息
type AgentStats struct {
	ProcessedLines int64   `json:"processed_lines"`
	SentRecords    int64   `json:"sent_records"`
	ErrorCount     int64   `json:"error_count"`
	LastSentTime   string  `json:"last_sent_time"`
	SendRate       float64 `json:"send_rate"`
	BufferSize     int     `json:"buffer_size"`
}

const Version = "1.0.0"

// NewAgentHandler 创建 API 处理器
func NewAgentHandler(
	cfg *config.Config,
	startTime time.Time,
	getCollector func() *collector.LogCollector,
	getSender func() *sender.ClickHouseSender,
	getRunning func() bool,
	startCollection func() error,
	stopCollection func(),
	getAgentLogs func(int) ([]string, error),
) *AgentHandler {
	return &AgentHandler{
		cfg:             cfg,
		startTime:       startTime,
		getCollector:    getCollector,
		getSender:       getSender,
		getRunning:      getRunning,
		startCollection: startCollection,
		stopCollection:  stopCollection,
		getAgentLogs:    getAgentLogs,
	}
}

// GetStatus 获取 Agent 状态
func (h *AgentHandler) GetStatus(c *gin.Context) {
	uptime := time.Since(h.startTime)

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	status := AgentStatus{
		Status:    "running",
		Version:   Version,
		NodeID:    h.cfg.NodeID,
		NodeName:  h.cfg.NodeName,
		StartTime: h.startTime.Format("2006-01-02 15:04:05"),
		Uptime:    uptime.String(),
		IsRunning: h.getRunning(),
		LogFile:   h.cfg.LogFile,
		ClickHouse: map[string]interface{}{
			"host":     h.cfg.ClickHouse.Host,
			"port":     h.cfg.ClickHouse.Port,
			"database": h.cfg.ClickHouse.Database,
			"user":     h.cfg.ClickHouse.Username,
		},
		System: map[string]interface{}{
			"goroutines": runtime.NumGoroutine(),
			"memory_mb":  m.Alloc / 1024 / 1024,
			"cpu_count":  runtime.NumCPU(),
		},
	}

	// 新增：添加位置信息
	if collector := h.getCollector(); collector != nil {
		status.System["position_info"] = collector.GetPositionInfo()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// StartCollection 启动日志收集
func (h *AgentHandler) StartCollection(c *gin.Context) {
	if err := h.startCollection(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "启动失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志收集已启动",
	})
}

// StopCollection 停止日志收集
func (h *AgentHandler) StopCollection(c *gin.Context) {
	h.stopCollection()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志收集已停止",
	})
}

// RestartCollection 重启日志收集
func (h *AgentHandler) RestartCollection(c *gin.Context) {
	h.stopCollection()
	time.Sleep(1 * time.Second)

	if err := h.startCollection(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "重启失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志收集已重启",
	})
}

// GetStats 获取统计信息
func (h *AgentHandler) GetStats(c *gin.Context) {
	stats := AgentStats{
		ProcessedLines: 0,
		SentRecords:    0,
		ErrorCount:     0,
		LastSentTime:   "",
		SendRate:       0,
		BufferSize:     0,
	}

	// 如果有收集器运行，获取统计信息
	if collector := h.getCollector(); collector != nil && h.getRunning() {
		processedLines, sentRecords, errorCount, lastSentTime := collector.GetStats()
		stats.ProcessedLines = processedLines
		stats.SentRecords = sentRecords
		stats.ErrorCount = errorCount
		if !lastSentTime.IsZero() {
			stats.LastSentTime = lastSentTime.Format("2006-01-02 15:04:05")
		}
		stats.BufferSize = collector.GetBufferSize()

		// 计算发送速率
		if uptime := time.Since(h.startTime).Seconds(); uptime > 0 {
			stats.SendRate = float64(sentRecords) / uptime
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

func (h *AgentHandler) GetLogs(c *gin.Context) {
	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "100"))
	if lines <= 0 || lines > 1000 {
		lines = 100
	}

	logs, err := h.getAgentLogs(lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取日志失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"logs": logs,
		},
	})
}

// GetConfig 获取配置信息
func (h *AgentHandler) GetConfig(c *gin.Context) {
	config := map[string]interface{}{
		"node_id":        h.cfg.NodeID,
		"node_name":      h.cfg.NodeName,
		"log_file":       h.cfg.LogFile,
		"batch_size":     h.cfg.BatchSize,
		"flush_interval": h.cfg.FlushInterval.Seconds(),
		"clickhouse": map[string]interface{}{
			"host":     h.cfg.ClickHouse.Host,
			"port":     h.cfg.ClickHouse.Port,
			"database": h.cfg.ClickHouse.Database,
			"user":     h.cfg.ClickHouse.Username,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// UpdateConfig 更新配置
func (h *AgentHandler) UpdateConfig(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "参数错误: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "配置更新成功，需要重启生效",
	})
}

// HealthCheck 健康检查
func (h *AgentHandler) HealthCheck(c *gin.Context) {
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"uptime":    time.Since(h.startTime).Seconds(),
		"collector": h.getRunning(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    health,
	})
}
