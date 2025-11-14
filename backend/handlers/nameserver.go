package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

var nameserverService = services.NewNameserverService()

// GetNameservers 获取命名服务器规则列表
func GetNameservers(c *gin.Context) {
	var nameservers []models.Nameserver

	query := database.DB

	if group := c.Query("group"); group != "" {
		query = query.Where("group = ?", group)
	}

	query.Order("priority desc, created_at desc").Find(&nameservers)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    nameservers,
		"total":   len(nameservers),
	})
}

// AddNameserver 添加命名服务器规则
func AddNameserver(c *gin.Context) {
	var request struct {
		Domain        string `json:"domain"`
		IsDomainSet   bool   `json:"is_domain_set"`
		DomainSetName string `json:"domain_set_name"`
		Group         string `json:"group" binding:"required"`
		Priority      int    `json:"priority"`
		Description   string `json:"description"`
		NodeIDs       []uint `json:"node_ids"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	nodeIDsJSON := "[]"
	if len(request.NodeIDs) > 0 {
		nodeIDsBytes, _ := json.Marshal(request.NodeIDs)
		nodeIDsJSON = string(nodeIDsBytes)
	}

	nameserver := models.Nameserver{
		Domain:        request.Domain,
		IsDomainSet:   request.IsDomainSet,
		DomainSetName: request.DomainSetName,
		Group:         request.Group,
		Priority:      request.Priority,
		Description:   request.Description,
		NodeIDs:       nodeIDsJSON,
		Enabled:       true,
	}

	if err := database.DB.Create(&nameserver).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "创建规则失败",
			"error":   err.Error(),
		})
		return
	}

	// 同步到节点
	go nameserverService.SyncNameserverToNodes(&nameserver)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "规则创建成功",
		"data":    nameserver,
	})
}

// UpdateNameserver 更新命名服务器规则
func UpdateNameserver(c *gin.Context) {
	id := c.Param("id")
	nameserverID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的规则ID",
		})
		return
	}

	var nameserver models.Nameserver
	if err := database.DB.First(&nameserver, nameserverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "规则不存在",
		})
		return
	}

	if err := c.ShouldBindJSON(&nameserver); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
		})
		return
	}

	database.DB.Save(&nameserver)

	// 同步到节点
	go nameserverService.SyncNameserverToNodes(&nameserver)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "规则更新成功",
		"data":    nameserver,
	})
}

// DeleteNameserver 删除命名服务器规则
func DeleteNameserver(c *gin.Context) {
	id := c.Param("id")
	nameserverID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的规则ID",
		})
		return
	}

	var nameserver models.Nameserver
	if err := database.DB.First(&nameserver, nameserverID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "规则不存在",
		})
		return
	}

	database.DB.Delete(&nameserver)

	// 从节点删除
	go nameserverService.DeleteNameserverFromNodes(&nameserver)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "规则删除成功",
	})
}
