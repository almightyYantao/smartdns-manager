package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

var notificationService = services.NewNotificationService()

// GetNotificationChannels 获取通知渠道列表
func GetNotificationChannels(c *gin.Context) {
	nodeID := c.Query("node_id")

	query := database.DB.Model(&models.NotificationChannel{})

	if nodeID != "" {
		query = query.Where("node_id = ? OR node_id = 0", nodeID)
	}

	var channels []models.NotificationChannel
	query.Order("node_id, created_at desc").Find(&channels)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    channels,
	})
}

// AddNotificationChannel 添加通知渠道
func AddNotificationChannel(c *gin.Context) {
	var channel models.NotificationChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	if err := database.DB.Create(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "添加通知渠道失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "通知渠道添加成功",
		"data":    channel,
	})
}

// UpdateNotificationChannel 更新通知渠道
func UpdateNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	channelID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的渠道ID",
		})
		return
	}

	var channel models.NotificationChannel
	if err := database.DB.First(&channel, channelID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "通知渠道不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询失败",
		})
		return
	}

	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
		})
		return
	}

	if err := database.DB.Save(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "更新失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
		"data":    channel,
	})
}

// DeleteNotificationChannel 删除通知渠道
func DeleteNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	channelID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的渠道ID",
		})
		return
	}

	if err := database.DB.Delete(&models.NotificationChannel{}, channelID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// TestNotificationChannel 测试通知渠道
func TestNotificationChannel(c *gin.Context) {
	id := c.Param("id")
	channelID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的渠道ID",
		})
		return
	}

	var channel models.NotificationChannel
	if err := database.DB.First(&channel, channelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "通知渠道不存在",
		})
		return
	}

	// 构造测试节点信息
	var node models.Node
	if channel.NodeID > 0 {
		// 如果是节点专属渠道，获取节点信息
		if err := database.DB.First(&node, channel.NodeID).Error; err != nil {
			// 节点不存在，使用默认值
			node.ID = channel.NodeID
			node.Name = "未知节点"
			node.Host = "N/A"
		}
	} else {
		// 全局渠道，使用默认值
		node.ID = 0
		node.Name = "系统全局"
		node.Host = "N/A"
	}

	// 直接调用 sendToChannel 方法
	go notificationService.SendNotification(
		channel.NodeID,
		"test",
		"🔔 测试通知",
		"这是一条测试消息，如果您收到此消息，说明通知渠道配置正确。",
	)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "测试消息已发送，请检查通知渠道",
	})
}

// GetNotificationLogs 获取通知日志
func GetNotificationLogs(c *gin.Context) {
	nodeID := c.Query("node_id")
	channelID := c.Query("channel_id")
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	query := database.DB.Model(&models.NotificationLog{})

	if nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	if channelID != "" {
		query = query.Where("channel_id = ?", channelID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var logs []models.NotificationLog
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
