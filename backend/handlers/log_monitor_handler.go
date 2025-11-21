package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"smartdns-manager/services"

	"github.com/gin-gonic/gin"
)

var logMonitorService services.LogMonitorServiceInterface

// InitLogMonitorHandler 初始化日志监控处理器
func InitLogMonitorHandler(service services.LogMonitorServiceInterface) {
	if service == nil {
		log.Fatal("❌ 尝试使用 nil service 初始化日志监控处理器")
	}
	logMonitorService = service
	log.Println("✅ 日志监控处理器初始化成功")
}

// StartNodeLogMonitor 启动节点日志监控
func StartNodeLogMonitor(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
		})
		return
	}

	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	if err := logMonitorService.StartNodeMonitor(uint(nodeID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "启动监控失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志监控已启动",
	})
}

// StopNodeLogMonitor 停止节点日志监控
func StopNodeLogMonitor(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
		})
		return
	}

	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	if err := logMonitorService.StopNodeMonitor(uint(nodeID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "停止监控失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志监控已停止",
	})
}

// GetNodeLogMonitorStatus 获取节点监控状态
func GetNodeLogMonitorStatus(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
		})
		return
	}

	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	isRunning, err := logMonitorService.GetNodeMonitorStatus(uint(nodeID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"is_running": isRunning,
		},
	})
}

// GetDNSLogs 获取DNS日志列表
func GetDNSLogs(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filters := make(map[string]interface{})

	if nodeIDStr := c.Query("node_id"); nodeIDStr != "" {
		nodeID, _ := strconv.ParseUint(nodeIDStr, 10, 32)
		filters["node_id"] = uint(nodeID)
	}
	if clientIP := c.Query("client_ip"); clientIP != "" {
		filters["client_ip"] = clientIP
	}
	if domain := c.Query("domain"); domain != "" {
		filters["domain"] = domain
	}
	if queryType := c.Query("query_type"); queryType != "" {
		qt, _ := strconv.Atoi(queryType)
		filters["query_type"] = qt
	}
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filters["start_time"] = t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filters["end_time"] = t
		}
	}

	logs, total, err := logMonitorService.GetLogs(page, pageSize, filters)
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
			"logs":      logs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetNodeLogStats 获取节点日志统计
func GetNodeLogStats(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
		})
		return
	}

	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	// 默认统计最近24小时
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	if st := c.Query("start_time"); st != "" {
		if t, err := time.Parse(time.RFC3339, st); err == nil {
			startTime = t
		}
	}
	if et := c.Query("end_time"); et != "" {
		if t, err := time.Parse(time.RFC3339, et); err == nil {
			endTime = t
		}
	}

	stats, err := logMonitorService.GetNodeStats(uint(nodeID), startTime, endTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取统计失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// CleanNodeLogs 清理节点旧日志
func CleanNodeLogs(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
		})
		return
	}

	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	if err := logMonitorService.CleanNodeLogs(uint(nodeID), days); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "清理失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "清理完成",
	})
}
