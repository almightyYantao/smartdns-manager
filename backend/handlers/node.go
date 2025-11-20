package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

// GetNodes 获取所有节点
func GetNodes(c *gin.Context) {
	var nodes []models.Node

	query := database.DB

	// 支持标签筛选
	if tags := c.Query("tags"); tags != "" {
		query = query.Where("tags LIKE ?", "%"+tags+"%")
	}

	// 支持状态筛选
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取节点列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nodes,
		"total":   len(nodes),
	})
}

// AddNode 添加新节点
func AddNode(c *gin.Context) {
	var node models.Node
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 设置默认值
	if node.Port == 0 {
		node.Port = 22
	}
	if node.ConfigPath == "" {
		node.ConfigPath = "/etc/smartdns/smartdns.conf"
	}
	node.Status = "unknown"
	node.LastCheck = time.Now()

	// 保存到数据库
	if err := database.DB.Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "添加节点失败",
			"error":   err.Error(),
		})
		return
	}

	// 异步测试连接
	go testAndUpdateNodeStatus(&node)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "节点添加成功",
		"data":    node,
	})
}

// UpdateNode 更新节点信息
func UpdateNode(c *gin.Context) {
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

	var updateData models.Node
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 更新字段
	node.Name = updateData.Name
	node.Host = updateData.Host
	node.Port = updateData.Port
	node.Username = updateData.Username
	if updateData.Password != "" {
		node.Password = updateData.Password
	}
	if updateData.PrivateKey != "" {
		node.PrivateKey = updateData.PrivateKey
	}
	node.ConfigPath = updateData.ConfigPath
	node.Tags = updateData.Tags
	node.Description = updateData.Description

	if err := database.DB.Save(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "更新节点失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点更新成功",
		"data":    node,
	})
}

// DeleteNode 删除节点
func DeleteNode(c *gin.Context) {
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的节点ID",
		})
		return
	}

	if err := database.DB.Delete(&models.Node{}, nodeID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除节点失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "节点删除成功",
	})
}

// TestNodeConnection 测试节点连接
func TestNodeConnection(c *gin.Context) {
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

	// 测试 SSH 连接
	client, err := services.NewSSHClient(&node)
	if err != nil {
		node.Status = "offline"
		database.DB.Save(&node)

		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "连接失败",
			"error":   err.Error(),
		})
		return
	}
	defer client.Close()

	// 测试配置文件是否存在
	_, err = client.ReadFile(node.ConfigPath)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "配置文件不存在或无法读取",
			"error":   err.Error(),
		})
		return
	}

	// 获取系统信息
	status, err := client.GetSystemInfo()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "获取系统信息失败",
			"error":   err.Error(),
		})
		return
	}

	node.Status = "online"
	node.LastCheck = time.Now()
	database.DB.Save(&node)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "连接成功",
		"data":    status,
	})
}

// 辅助函数：测试并更新节点状态
func testAndUpdateNodeStatus(node *models.Node) {
	client, err := services.NewSSHClient(node)
	if err != nil {
		node.Status = "offline"
		node.LastCheck = time.Now()
		database.DB.Save(node)
		return
	}
	defer client.Close()

	// 检查配置文件是否存在
	_, err = client.ReadFile(node.ConfigPath)
	if err != nil {
		node.Status = "error"
		node.LastCheck = time.Now()
		database.DB.Save(node)
		return
	}

	// 检查 SmartDNS 服务状态
	output, err := client.ExecuteCommand("systemctl is-active smartdns 2>&1")
	if err != nil || strings.TrimSpace(output) != "active" {
		node.Status = "stopped" // 或 "error"
		node.LastCheck = time.Now()
		database.DB.Save(node)
		return
	}

	// 所有检查都通过
	node.Status = "online"
	node.LastCheck = time.Now()
	database.DB.Save(node)
}
