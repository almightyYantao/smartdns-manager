package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

// 初始化日志监控服务
var logMonitorService services.LogMonitorInterface

// InitLogMonitorHandler 初始化处理器
func InitLogMonitorHandler(service services.LogMonitorInterface) {
	logMonitorService = service
}

// ========== Agent 控制相关（通过 Agent API）==========

// StartNodeLogMonitor 启动节点日志监控（调用 Agent API）
func StartNodeLogMonitor(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	// 获取节点信息
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "节点不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询节点失败",
		})
		return
	}

	// 检查 Agent 是否已安装
	if !node.AgentInstalled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "节点上未安装 Agent，请先部署 Agent",
		})
		return
	}

	// 调用 Agent API 启动日志收集
	agentPort := services.GetAgentPort(&node)
	agentURL := fmt.Sprintf("http://%s:%d/api/v1/start", node.Host, agentPort)

	err = services.CallAgentAPI("POST", agentURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "启动监控失败: " + err.Error(),
		})
		return
	}

	// 更新数据库状态
	database.DB.Model(&node).Updates(map[string]interface{}{
		"log_monitor_enabled": true,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志监控已启动",
	})
}

// StopNodeLogMonitor 停止节点日志监控（调用 Agent API）
func StopNodeLogMonitor(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "节点不存在",
		})
		return
	}

	// 调用 Agent API 停止日志收集
	agentPort := services.GetAgentPort(&node)
	agentURL := fmt.Sprintf("http://%s:%d/api/v1/stop", node.Host, agentPort)

	err = services.CallAgentAPI("POST", agentURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "停止监控失败: " + err.Error(),
		})
		return
	}

	// 更新数据库状态
	database.DB.Model(&node).Updates(map[string]interface{}{
		"log_monitor_enabled": false,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志监控已停止",
	})
}

// GetNodeLogMonitorStatus 获取节点监控状态（调用 Agent API）
func GetNodeLogMonitorStatus(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "节点不存在",
		})
		return
	}

	// 如果 Agent 未安装，返回未安装状态
	if !node.AgentInstalled {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"is_running":      false,
				"agent_installed": false,
				"message":         "Agent 未安装",
			},
		})
		return
	}

	// 调用 Agent API 获取状态
	agentPort := services.GetAgentPort(&node)
	agentURL := fmt.Sprintf("http://%s:%d/api/v1/status", node.Host, agentPort)

	response, err := services.CallAgentAPIWithResponse("GET", agentURL, nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"is_running":      false,
				"agent_installed": true,
				"error":           err.Error(),
				"message":         "无法连接到 Agent",
			},
		})
		return
	}

	// 更新节点 Agent 状态
	database.DB.Model(&node).Updates(map[string]interface{}{
		"agent_installed": true,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"is_running":      response["data"].(map[string]interface{})["is_running"],
			"agent_installed": true,
			"agent_status":    response["data"],
		},
	})
}

// RestartNodeLogMonitor 重启节点日志监控（调用 Agent API）
func RestartNodeLogMonitor(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "节点不存在",
		})
		return
	}

	// 调用 Agent API 重启日志收集
	agentPort := services.GetAgentPort(&node)
	agentURL := fmt.Sprintf("http://%s:%d/api/v1/restart", node.Host, agentPort)

	err = services.CallAgentAPI("POST", agentURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "重启监控失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "日志监控已重启",
	})
}

// GetAgentStats 获取 Agent 统计信息（调用 Agent API）
func GetAgentStats(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "节点不存在",
		})
		return
	}

	// 调用 Agent API 获取统计信息
	agentPort := services.GetAgentPort(&node)
	agentURL := fmt.Sprintf("http://%s:%d/api/v1/stats", node.Host, agentPort)

	response, err := services.CallAgentAPIWithResponse("GET", agentURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取统计失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response["data"],
	})
}

func GetAgentLogs(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "节点不存在",
		})
		return
	}

	// 调用 Agent API 获取统计信息
	agentPort := services.GetAgentPort(&node)
	agentURL := fmt.Sprintf("http://%s:%d/api/v1/logs", node.Host, agentPort)

	response, err := services.CallAgentAPIWithResponse("GET", agentURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取日志失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response["data"],
	})
}

// ========== DNS 日志查询相关（直接查询 ClickHouse）==========

// GetDNSLogs 获取DNS日志列表（从 ClickHouse 查询）
func GetDNSLogs(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
		})
		return
	}

	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	// 限制大数据量查询的页面大小
	if pageSize > 50 {
		pageSize = 50
	}

	// 构建过滤条件
	filters := make(map[string]interface{})

	// 节点ID
	if nodeIDStr := c.Query("node_id"); nodeIDStr != "" {
		if nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32); err == nil {
			filters["node_id"] = uint(nodeID)
		}
	}

	// 客户端IP
	if clientIP := c.Query("client_ip"); clientIP != "" {
		filters["client_ip"] = clientIP
	}

	if group := c.Query("group"); group != "" {
		filters["group"] = group
	}

	// 域名
	if domain := c.Query("domain"); domain != "" {
		filters["domain"] = domain
	}

	// 查询类型
	if queryTypeStr := c.Query("query_type"); queryTypeStr != "" {
		if queryType, err := strconv.Atoi(queryTypeStr); err == nil {
			filters["query_type"] = queryType
		}
	}

	// 时间范围 - 改进时间解析
	if startTimeStr := c.Query("start_time"); startTimeStr != "" {
		// 支持多种时间格式
		formats := []string{
			time.RFC3339,
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}

		for _, format := range formats {
			if startTime, err := time.Parse(format, startTimeStr); err == nil {
				filters["start_time"] = startTime
				break
			}
		}
	}

	if endTimeStr := c.Query("end_time"); endTimeStr != "" {
		formats := []string{
			time.RFC3339,
			"2006-01-02T15:04:05Z",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}

		for _, format := range formats {
			if endTime, err := time.Parse(format, endTimeStr); err == nil {
				filters["end_time"] = endTime
				break
			}
		}
	}

	// 解析排序参数
	sortField := c.DefaultQuery("sort_field", "timestamp")
	sortOrder := c.DefaultQuery("sort_order", "desc")
	
	// 验证排序字段
	allowedSortFields := map[string]bool{
		"timestamp": true,
		"time_ms":   true,
		"speed_ms":  true,
		"domain":    true,
		"client_ip": true,
	}
	
	if !allowedSortFields[sortField] {
		sortField = "timestamp"
	}
	
	// 验证排序方向
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	
	filters["sort_field"] = sortField
	filters["sort_order"] = sortOrder

	// 添加请求超时控制
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	// 在新的上下文中查询
	done := make(chan bool, 1)
	var logs []models.DNSLog
	var total int64
	var err error

	go func() {
		logs, total, err = logMonitorService.GetLogs(page, pageSize, filters)
		done <- true
	}()

	select {
	case <-done:
		if err != nil {
			log.Printf("❌ 获取日志失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "获取日志失败: " + err.Error(),
			})
			return
		}

		// 确保 logs 不为 nil
		if logs == nil {
			logs = make([]models.DNSLog, 0)
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

	case <-ctx.Done():
		c.JSON(http.StatusRequestTimeout, gin.H{
			"success": false,
			"message": "查询超时，请缩小时间范围或添加更多过滤条件",
		})
	}
}

// GetLogStats 获取日志统计信息（从 ClickHouse 查询）
func GetLogStats(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
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

	nodeIDStr := c.Param("id")
	nodeIDInt, err := strconv.Atoi(nodeIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点ID"})
		return
	}
	nodeID := uint(nodeIDInt) // 转换为uint类型

	// 从 ClickHouse 获取统计信息
	stats, err := logMonitorService.GetStats(nodeID, startTime, endTime)
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

// SearchDomains 搜索域名（从 ClickHouse 查询）
func SearchDomains(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
		})
		return
	}

	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请提供搜索关键词",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	// 从 ClickHouse 搜索域名
	domains, err := logMonitorService.SearchDomains(keyword, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "搜索失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    domains,
	})
}

// CleanOldLogs 清理旧日志（直接操作 ClickHouse）
func CleanOldLogs(c *gin.Context) {
	if logMonitorService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "日志监控服务未初始化",
		})
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 || days > 365 {
		days = 30
	}

	// 节点ID（可选）
	var nodeID uint
	if nodeIDStr := c.Query("node_id"); nodeIDStr != "" {
		if id, err := strconv.ParseUint(nodeIDStr, 10, 32); err == nil {
			nodeID = uint(id)
		}
	}

	// 直接从 ClickHouse 清理日志
	err := logMonitorService.CleanOldLogs(nodeID, days)
	if err != nil {
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
