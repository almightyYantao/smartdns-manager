package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smartdns-manager/database"
	"smartdns-manager/models"
)

// AddServer 添加DNS服务器
func AddServer(c *gin.Context) {
	var server models.DNSServer
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证地址
	if server.Address == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "服务器地址不能为空",
		})
		return
	}

	// 确定服务器类型
	if server.Type == "" {
		if strings.HasPrefix(server.Address, "https://") {
			server.Type = "https"
		} else if strings.HasPrefix(server.Address, "tls://") {
			server.Type = "tls"
		} else if strings.Contains(server.Address, ":") && !strings.Contains(server.Address, "://") {
			server.Type = "tcp"
		} else {
			server.Type = "udp"
		}
	}

	// 序列化 Groups
	if len(server.Groups) > 0 {
		groupsJSON, _ := json.Marshal(server.Groups)
		server.GroupsStr = string(groupsJSON)
	}

	// 生成 Options 字符串
	if server.Options == "" {
		server.Options = generateServerOptions(&server)
	}

	// 检查是否已存在
	var existing models.DNSServer
	if err := database.DB.Where("address = ?", server.Address).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "该DNS服务器已存在",
		})
		return
	}

	if err := database.DB.Create(&server).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "添加DNS服务器失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "DNS服务器添加成功",
		"data":    server,
	})
}

// UpdateServer 更新DNS服务器
func UpdateServer(c *gin.Context) {
	id := c.Param("id")
	serverID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的服务器ID",
		})
		return
	}

	var server models.DNSServer
	if err := database.DB.First(&server, serverID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "DNS服务器不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询DNS服务器失败",
		})
		return
	}

	var updateData models.DNSServer
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 更新字段
	if updateData.Address != "" {
		server.Address = updateData.Address
	}
	if updateData.Type != "" {
		server.Type = updateData.Type
	}
	if len(updateData.Groups) > 0 {
		server.Groups = updateData.Groups
		groupsJSON, _ := json.Marshal(server.Groups)
		server.GroupsStr = string(groupsJSON)
	}
	server.ExcludeDefault = updateData.ExcludeDefault

	// 重新生成 Options
	server.Options = generateServerOptions(&server)

	if err := database.DB.Save(&server).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "更新DNS服务器失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DNS服务器更新成功",
		"data":    server,
	})
}

// DeleteServer 删除DNS服务器
func DeleteServer(c *gin.Context) {
	id := c.Param("id")
	serverID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的服务器ID",
		})
		return
	}

	var server models.DNSServer
	if err := database.DB.First(&server, serverID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "DNS服务器不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询DNS服务器失败",
		})
		return
	}

	if err := database.DB.Delete(&server).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除DNS服务器失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DNS服务器删除成功",
	})
}

// GetServers 获取DNS服务器列表
func GetServers(c *gin.Context) {
	var servers []models.DNSServer

	query := database.DB

	// 支持类型筛选
	if serverType := c.Query("type"); serverType != "" {
		query = query.Where("type = ?", serverType)
	}

	// 支持分组筛选
	if group := c.Query("group"); group != "" {
		query = query.Where("groups LIKE ?", "%"+group+"%")
	}

	if err := query.Find(&servers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取DNS服务器列表失败",
			"error":   err.Error(),
		})
		return
	}

	// 反序列化 Groups
	for i := range servers {
		if servers[i].GroupsStr != "" {
			json.Unmarshal([]byte(servers[i].GroupsStr), &servers[i].Groups)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    servers,
		"total":   len(servers),
	})
}

// 辅助函数：生成服务器选项字符串
func generateServerOptions(server *models.DNSServer) string {
	options := []string{}

	for _, group := range server.Groups {
		options = append(options, "-group "+group)
	}

	if server.ExcludeDefault {
		options = append(options, "-exclude-default-group")
	}

	return strings.Join(options, " ")
}
