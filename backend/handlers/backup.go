package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

// GetNodeBackups 获取节点备份列表
func GetNodeBackups(c *gin.Context) {
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

	client, err := services.NewSSHClient(&node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "连接节点失败",
			"error":   err.Error(),
		})
		return
	}
	defer client.Close()

	backups, err := client.ListBackups(node.ConfigPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取备份列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    backups,
		"total":   len(backups),
	})
}

// CreateNodeBackup 创建节点备份
func CreateNodeBackup(c *gin.Context) {
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

	client, err := services.NewSSHClient(&node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "连接节点失败",
			"error":   err.Error(),
		})
		return
	}
	defer client.Close()

	backupPath, err := client.CreateBackup(node.ConfigPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "创建备份失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "备份创建成功",
		"data": map[string]string{
			"path": backupPath,
		},
	})
}

// RestoreNodeBackup 恢复节点备份
func RestoreNodeBackup(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	var request struct {
		BackupPath string `json:"backup_path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
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

	client, err := services.NewSSHClient(&node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "连接节点失败",
			"error":   err.Error(),
		})
		return
	}
	defer client.Close()

	// 先备份当前配置
	currentBackup, err := client.CreateBackup(node.ConfigPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "备份当前配置失败",
			"error":   err.Error(),
		})
		return
	}

	// 恢复备份
	cmd := fmt.Sprintf("sudo cp %s %s", request.BackupPath, node.ConfigPath)
	if _, err := client.ExecuteCommand(cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "恢复备份失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "配置恢复成功",
		"currentBackup": currentBackup,
	})
}
