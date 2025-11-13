package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

var configSyncService = services.NewConfigSyncService()

// TriggerFullSync 手动触发完整同步
func TriggerFullSync(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	// 异步执行完整同步
	go func() {
		if err := configSyncService.FullSyncToNode(uint(nodeID)); err != nil {
			log.Printf("完整同步失败: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已开始完整同步，请稍后查看同步日志",
	})
}

// BatchFullSync 批量完整同步
func BatchFullSync(c *gin.Context) {
	var request struct {
		NodeIDs []uint `json:"node_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
		})
		return
	}

	go func() {
		for _, nodeID := range request.NodeIDs {
			if err := configSyncService.FullSyncToNode(nodeID); err != nil {
				log.Printf("节点 %d 完整同步失败: %v", nodeID, err)
			}
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("已开始同步 %d 个节点", len(request.NodeIDs)),
	})
}

// GetSyncLogs 获取同步日志
func GetSyncLogs(c *gin.Context) {
	nodeID := c.Query("node_id")
	syncType := c.Query("type")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	query := database.DB.Model(&models.ConfigSyncLog{})

	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	if syncType != "" {
		query = query.Where("type = ?", syncType)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var logs []models.ConfigSyncLog
	offset := (page - 1) * pageSize
	query.Order("created_at desc").Offset(offset).Limit(pageSize).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetSyncStats 获取同步统计
func GetSyncStats(c *gin.Context) {
	var stats struct {
		Total   int64 `json:"total"`
		Success int64 `json:"success"`
		Failed  int64 `json:"failed"`
		Pending int64 `json:"pending"`
	}

	database.DB.Model(&models.ConfigSyncLog{}).Count(&stats.Total)
	database.DB.Model(&models.ConfigSyncLog{}).Where("status = ?", "success").Count(&stats.Success)
	database.DB.Model(&models.ConfigSyncLog{}).Where("status = ?", "failed").Count(&stats.Failed)
	database.DB.Model(&models.ConfigSyncLog{}).Where("status = ?", "pending").Count(&stats.Pending)

	// 最近的同步记录
	var recentLogs []models.ConfigSyncLog
	database.DB.Order("created_at desc").Limit(10).Find(&recentLogs)

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"stats":       stats,
		"recent_logs": recentLogs,
	})
}

// RetrySyncLog 重试失败的同步
func RetrySyncLog(c *gin.Context) {
	id := c.Param("id")
	logID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的日志ID",
		})
		return
	}

	var syncLog models.ConfigSyncLog
	if err := database.DB.First(&syncLog, logID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "同步日志不存在",
		})
		return
	}

	// 只能重试失败的记录
	if syncLog.Status != "failed" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "只能重试失败的同步记录",
		})
		return
	}

	// 异步重试
	go func() {
		syncLog.Status = "pending"
		syncLog.Error = ""
		database.DB.Save(&syncLog)

		var node models.Node
		database.DB.First(&node, syncLog.NodeID)

		switch syncLog.Type {
		case "address":
			// 解析内容，重新同步地址
			// 这里需要根据 content 字段解析出 domain 和 ip
			// 简化处理：触发完整同步
			configSyncService.FullSyncToNode(syncLog.NodeID)
		case "server":
			configSyncService.FullSyncToNode(syncLog.NodeID)
		case "full_sync":
			configSyncService.FullSyncToNode(syncLog.NodeID)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "已开始重试同步",
	})
}

// ClearSyncLogs 清理同步日志
func ClearSyncLogs(c *gin.Context) {
	var request struct {
		Days   int    `json:"days"`   // 清理多少天前的日志
		Status string `json:"status"` // 只清理指定状态的日志
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		request.Days = 30 // 默认30天
	}

	query := database.DB.Model(&models.ConfigSyncLog{})

	if request.Days > 0 {
		cutoffTime := time.Now().AddDate(0, 0, -request.Days)
		query = query.Where("created_at < ?", cutoffTime)
	}

	if request.Status != "" {
		query = query.Where("status = ?", request.Status)
	}

	result := query.Delete(&models.ConfigSyncLog{})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("已清理 %d 条日志", result.RowsAffected),
		"deleted": result.RowsAffected,
	})
}
