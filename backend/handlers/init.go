package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

var initService = services.NewInitService()

// InitNode 初始化节点
func InitNode(c *gin.Context) {
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

	// 检查是否正在初始化
	if node.InitStatus == "initializing" {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "节点正在初始化中",
		})
		return
	}

	// 异步执行初始化
	go func() {
		if err := initService.InitNode(uint(nodeID)); err != nil {
			log.Printf("节点初始化失败: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点初始化已开始，请查看初始化日志",
	})
}

// CheckNodeInit 检查节点初始化状态
func CheckNodeInit(c *gin.Context) {
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

	// 如果状态是 unknown，尝试检测
	if node.InitStatus == "unknown" {
		go func() {
			if err := initService.CheckAndUpdateNodeStatus(&node); err != nil {
				log.Printf("检测节点状态失败: %v", err)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"init_status":      node.InitStatus,
			"smartdns_version": node.SmartDNSVersion,
			"os_type":          node.OSType,
			"os_version":       node.OSVersion,
			"architecture":     node.Architecture,
		},
	})
}

// GetInitLogs 获取初始化日志
func GetInitLogs(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	var logs []models.InitLog
	database.DB.Where("node_id = ?", nodeID).Order("created_at desc").Limit(50).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
	})
}

// UninstallSmartDNS 卸载 SmartDNS
func UninstallSmartDNS(c *gin.Context) {
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

	// 异步执行卸载
	go func() {
		if err := initService.UninstallSmartDNS(uint(nodeID)); err != nil {
			log.Printf("卸载失败: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "卸载任务已开始",
	})
}

// ReinstallSmartDNS 重新安装 SmartDNS
func ReinstallSmartDNS(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	// 先卸载再安装
	go func() {
		if err := initService.UninstallSmartDNS(uint(nodeID)); err != nil {
			log.Printf("卸载失败: %v", err)
			return
		}

		time.Sleep(2 * time.Second)

		if err := initService.InitNode(uint(nodeID)); err != nil {
			log.Printf("重新安装失败: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "重新安装任务已开始",
	})
}
