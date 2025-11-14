package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

var domainSetService = services.NewDomainSetService()

// GetDomainSets 获取域名集列表
func GetDomainSets(c *gin.Context) {
	var domainSets []models.DomainSet

	query := database.DB

	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	query.Order("created_at desc").Find(&domainSets)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    domainSets,
		"total":   len(domainSets),
	})
}

// GetDomainSet 获取单个域名集详情
func GetDomainSet(c *gin.Context) {
	id := c.Param("id")
	domainSetID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的域名集ID",
		})
		return
	}

	var domainSet models.DomainSet
	if err := database.DB.First(&domainSet, domainSetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "域名集不存在",
		})
		return
	}

	// 获取域名列表
	var items []models.DomainSetItem
	database.DB.Where("domain_set_id = ?", domainSetID).Order("domain").Find(&items)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"domain_set": domainSet,
			"items":      items,
		},
	})
}

// AddDomainSet 添加域名集
func AddDomainSet(c *gin.Context) {
	var request struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		Domains     []string `json:"domains"` // 域名列表
		NodeIDs     []uint   `json:"node_ids"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 检查名称是否已存在
	var existing models.DomainSet
	if err := database.DB.Where("name = ?", request.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "域名集名称已存在",
		})
		return
	}

	// 转换 NodeIDs
	nodeIDsJSON := "[]"
	if len(request.NodeIDs) > 0 {
		nodeIDsBytes, _ := json.Marshal(request.NodeIDs)
		nodeIDsJSON = string(nodeIDsBytes)
	}

	// 创建域名集
	domainSet := models.DomainSet{
		Name:        request.Name,
		FilePath:    fmt.Sprintf("/etc/smartdns/%s.conf", request.Name),
		Description: request.Description,
		NodeIDs:     nodeIDsJSON,
		DomainCount: len(request.Domains),
		Enabled:     true,
	}

	if err := database.DB.Create(&domainSet).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "创建域名集失败",
			"error":   err.Error(),
		})
		return
	}

	// 添加域名条目
	for _, domain := range request.Domains {
		item := models.DomainSetItem{
			DomainSetID: domainSet.ID,
			Domain:      strings.TrimSpace(domain),
		}
		database.DB.Create(&item)
	}

	// 同步到节点
	go domainSetService.SyncDomainSetToNodes(&domainSet)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "域名集创建成功，正在同步到节点...",
		"data":    domainSet,
	})
}

// UpdateDomainSet 更新域名集
func UpdateDomainSet(c *gin.Context) {
	id := c.Param("id")
	domainSetID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的域名集ID",
		})
		return
	}

	var domainSet models.DomainSet
	if err := database.DB.First(&domainSet, domainSetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "域名集不存在",
		})
		return
	}

	var request struct {
		Description string   `json:"description"`
		Domains     []string `json:"domains"`
		NodeIDs     []uint   `json:"node_ids"`
		Enabled     bool     `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
		})
		return
	}

	// 更新基本信息
	domainSet.Description = request.Description
	domainSet.Enabled = request.Enabled
	domainSet.DomainCount = len(request.Domains)

	if len(request.NodeIDs) > 0 {
		nodeIDsBytes, _ := json.Marshal(request.NodeIDs)
		domainSet.NodeIDs = string(nodeIDsBytes)
	}

	// 删除旧的域名条目
	database.DB.Where("domain_set_id = ?", domainSetID).Delete(&models.DomainSetItem{})

	// 添加新的域名条目
	for _, domain := range request.Domains {
		item := models.DomainSetItem{
			DomainSetID: domainSet.ID,
			Domain:      strings.TrimSpace(domain),
		}
		database.DB.Create(&item)
	}

	database.DB.Save(&domainSet)

	// 同步到节点
	go domainSetService.SyncDomainSetToNodes(&domainSet)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "域名集更新成功，正在同步到节点...",
		"data":    domainSet,
	})
}

// DeleteDomainSet 删除域名集
func DeleteDomainSet(c *gin.Context) {
	id := c.Param("id")
	domainSetID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的域名集ID",
		})
		return
	}

	var domainSet models.DomainSet
	if err := database.DB.First(&domainSet, domainSetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "域名集不存在",
		})
		return
	}

	// 删除域名条目
	database.DB.Where("domain_set_id = ?", domainSetID).Delete(&models.DomainSetItem{})

	// 删除域名集
	database.DB.Delete(&domainSet)

	// 从节点删除
	go domainSetService.DeleteDomainSetFromNodes(&domainSet)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "域名集删除成功",
	})
}

// ImportDomainSetFile 导入域名列表文件
func ImportDomainSetFile(c *gin.Context) {
	id := c.Param("id")
	domainSetID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的域名集ID",
		})
		return
	}

	var domainSet models.DomainSet
	if err := database.DB.First(&domainSet, domainSetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "域名集不存在",
		})
		return
	}

	var request struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
		})
		return
	}

	// 解析域名列表
	lines := strings.Split(request.Content, "\n")
	domains := []string{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domains = append(domains, line)
	}

	// 删除旧条目
	database.DB.Where("domain_set_id = ?", domainSetID).Delete(&models.DomainSetItem{})

	// 批量插入新域名
	successCount := 0
	for _, domain := range domains {
		item := models.DomainSetItem{
			DomainSetID: domainSet.ID,
			Domain:      domain,
		}
		if err := database.DB.Create(&item).Error; err == nil {
			successCount++
		}
	}

	// 更新计数
	domainSet.DomainCount = successCount
	database.DB.Save(&domainSet)

	// 同步到节点
	go domainSetService.SyncDomainSetToNodes(&domainSet)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "导入成功",
		"data": gin.H{
			"total":   len(domains),
			"success": successCount,
		},
	})
}

// ExportDomainSet 导出域名列表
func ExportDomainSet(c *gin.Context) {
	id := c.Param("id")
	domainSetID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的域名集ID",
		})
		return
	}

	var domainSet models.DomainSet
	if err := database.DB.First(&domainSet, domainSetID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "域名集不存在",
		})
		return
	}

	var items []models.DomainSetItem
	database.DB.Where("domain_set_id = ?", domainSetID).Order("domain").Find(&items)

	// 生成文件内容
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Domain Set: %s\n", domainSet.Name))
	content.WriteString(fmt.Sprintf("# Description: %s\n", domainSet.Description))
	content.WriteString(fmt.Sprintf("# Total: %d domains\n", len(items)))
	content.WriteString("\n")

	for _, item := range items {
		if item.Comment != "" {
			content.WriteString(fmt.Sprintf("# %s\n", item.Comment))
		}
		content.WriteString(fmt.Sprintf("%s\n", item.Domain))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    content.String(),
	})
}
