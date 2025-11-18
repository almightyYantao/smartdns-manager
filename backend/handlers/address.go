package handlers

import (
	"encoding/json"
	_ "encoding/json"
	"fmt"
	"log"
	"net/http"
	"smartdns-manager/services"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"smartdns-manager/database"
	"smartdns-manager/models"
)

var syncService = services.NewConfigSyncService()

// AddAddress 添加地址映射
func AddAddress(c *gin.Context) {
	var address models.AddressMap
	if err := c.ShouldBindJSON(&address); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 验证域名
	if address.Domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "域名不能为空",
		})
		return
	}

	// 验证类型和对应的值
	if address.Type == "" {
		address.Type = "address" // 默认类型
	}

	if address.Type == "address" {
		if address.IP == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "IP地址不能为空",
			})
			return
		}
		address.CNAME = "" // 清空 CNAME
	} else if address.Type == "cname" {
		if address.CNAME == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "CNAME别名不能为空",
			})
			return
		}
		address.IP = "" // 清空 IP
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "类型必须是 address 或 cname",
		})
		return
	}

	// 检查是否已存在
	var existing models.AddressMap
	query := database.DB.Where("domain = ?", address.Domain)
	if address.Type == "address" {
		query = query.Where("ip = ?", address.IP)
	} else {
		query = query.Where("cname = ?", address.CNAME)
	}

	if err := query.First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "该映射已存在",
		})
		return
	}

	// 默认启用
	address.Enabled = true

	// 保存到数据库
	if err := database.DB.Create(&address).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "添加失败",
			"error":   err.Error(),
		})
		return
	}

	// 自动同步到节点
	go func() {
		if err := syncService.SyncAddressToNodes(&address); err != nil {
			log.Printf("同步到节点失败: %v", err)
		}
	}()

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "添加成功，正在同步到节点...",
		"data":    address,
	})
}

// UpdateAddress 更新地址映射
func UpdateAddress(c *gin.Context) {
	id := c.Param("id")
	addressID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的地址映射ID",
		})
		return
	}

	var address models.AddressMap
	if err := database.DB.First(&address, addressID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "地址映射不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询地址映射失败",
		})
		return
	}

	var updateData models.AddressMap
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	// 更新字段
	if updateData.Domain != "" {
		address.Domain = updateData.Domain
	}
	if updateData.IP != "" {
		address.IP = updateData.IP
	}
	address.Tags = updateData.Tags
	address.Comment = updateData.Comment
	address.NodeIDs = updateData.NodeIDs
	address.Enabled = updateData.Enabled

	if err := database.DB.Save(&address).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "更新地址映射失败",
			"error":   err.Error(),
		})
		return
	}

	// ========== 自动同步到节点 ==========
	go func() {
		if err := syncService.SyncAddressToNodes(&address); err != nil {
			log.Printf("同步地址映射到节点失败: %v", err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "地址映射更新成功，正在同步到节点...",
		"data":    address,
	})
}

// DeleteAddress 删除地址映射
func DeleteAddress(c *gin.Context) {
	id := c.Param("id")
	addressID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的地址映射ID",
		})
		return
	}

	var address models.AddressMap
	if err := database.DB.First(&address, addressID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "地址映射不存在",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "查询地址映射失败",
		})
		return
	}

	// ========== 先从节点删除 ==========
	go func() {
		if err := syncService.DeleteAddressFromNodes(&address); err != nil {
			log.Printf("从节点删除地址映射失败: %v", err)
		}
	}()

	// 从数据库删除
	if err := database.DB.Delete(&address).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除地址映射失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "地址映射删除成功，正在从节点移除...",
	})
}

// BatchAddAddresses 批量添加地址映射
func BatchAddAddresses(c *gin.Context) {
	var request struct {
		Addresses []models.AddressMap `json:"addresses" binding:"required"`
		NodeIDs   []uint              `json:"node_ids"` // 可选，指定要应用到的节点
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	successCount := 0
	failCount := 0
	results := make([]map[string]interface{}, 0)
	addedAddresses := []models.AddressMap{} // 用于批量同步

	// 将 NodeIDs 转为 JSON 字符串
	nodeIDsJSON := "[]"
	if len(request.NodeIDs) > 0 {
		nodeIDsBytes, _ := json.Marshal(request.NodeIDs)
		nodeIDsJSON = string(nodeIDsBytes)
	}

	for _, addr := range request.Addresses {
		result := map[string]interface{}{
			"domain":  addr.Domain,
			"ip":      addr.IP,
			"success": false,
		}

		// 检查是否已存在
		var existing models.AddressMap
		if err := database.DB.Where("domain = ? AND ip = ?", addr.Domain, addr.IP).First(&existing).Error; err == nil {
			result["error"] = "已存在"
			failCount++
			results = append(results, result)
			continue
		}

		// 设置节点ID和启用状态
		addr.NodeIDs = nodeIDsJSON
		addr.Enabled = true

		if err := database.DB.Create(&addr).Error; err != nil {
			result["error"] = err.Error()
			failCount++
		} else {
			result["success"] = true
			result["id"] = addr.ID
			successCount++
			addedAddresses = append(addedAddresses, addr) // 收集成功添加的
		}
		results = append(results, result)
	}

	// ========== 批量同步到节点 ==========
	if len(addedAddresses) > 0 {
		go func() {
			log.Printf("开始批量同步 %d 个地址映射到节点", len(addedAddresses))
			for _, addr := range addedAddresses {
				if err := syncService.SyncAddressToNodes(&addr); err != nil {
					log.Printf("同步地址映射失败 (%s -> %s): %v", addr.Domain, addr.IP, err)
				}
			}
			log.Printf("批量同步完成")
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       fmt.Sprintf("批量添加完成，正在同步到节点..."),
		"success_count": successCount,
		"fail_count":    failCount,
		"results":       results,
	})
}

// ImportAddresses 从文件导入地址映射
func ImportAddresses(c *gin.Context) {
	var request struct {
		Content string `json:"content" binding:"required"` // 配置文件内容
		Format  string `json:"format"`                     // 格式：smartdns, hosts
		NodeIDs []uint `json:"node_ids"`                   // 应用到的节点
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误",
			"error":   err.Error(),
		})
		return
	}

	addresses, err := parseAddressesFromContent(request.Content, request.Format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "解析内容失败",
			"error":   err.Error(),
		})
		return
	}

	// 将 NodeIDs 转为 JSON
	nodeIDsJSON := "[]"
	if len(request.NodeIDs) > 0 {
		nodeIDsBytes, _ := json.Marshal(request.NodeIDs)
		nodeIDsJSON = string(nodeIDsBytes)
	}

	successCount := 0
	importedAddresses := []models.AddressMap{}

	for _, addr := range addresses {
		addr.NodeIDs = nodeIDsJSON
		addr.Enabled = true

		if err := database.DB.Create(&addr).Error; err == nil {
			successCount++
			importedAddresses = append(importedAddresses, addr)
		}
	}

	// ========== 批量同步到节点 ==========
	if len(importedAddresses) > 0 {
		go func() {
			log.Printf("开始同步导入的 %d 个地址映射", len(importedAddresses))
			for _, addr := range importedAddresses {
				if err := syncService.SyncAddressToNodes(&addr); err != nil {
					log.Printf("同步导入的地址映射失败: %v", err)
				}
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"message":      "导入完成，正在同步到节点...",
		"total":        len(addresses),
		"successCount": successCount,
	})
}

// GetAddresses 获取地址映射列表
func GetAddresses(c *gin.Context) {
	var addresses []models.AddressMap

	query := database.DB

	// 支持标签筛选
	if tags := c.Query("tags"); tags != "" {
		query = query.Where("tags LIKE ?", "%"+tags+"%")
	}

	// 支持域名搜索
	if domain := c.Query("domain"); domain != "" {
		query = query.Where("domain LIKE ?", "%"+domain+"%")
	}

	// 分页
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	offset := (page - 1) * pageSize

	var total int64
	query.Model(&models.AddressMap{}).Count(&total)

	if err := query.Offset(offset).Limit(pageSize).Find(&addresses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取地址映射列表失败",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      addresses,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// 辅助函数：从内容解析地址映射
func parseAddressesFromContent(content, format string) ([]models.AddressMap, error) {
	addresses := make([]models.AddressMap, 0)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		// 检查是否是 CNAME 格式
		if len(parts) >= 3 && strings.ToLower(parts[1]) == "cname" {
			addresses = append(addresses, models.AddressMap{
				Domain: parts[0],
				CNAME:  parts[2],
				Type:   "cname",
			})
		} else {
			// Address 格式
			addresses = append(addresses, models.AddressMap{
				Domain: parts[0],
				IP:     parts[1],
				Type:   "address",
			})
		}
	}

	return addresses, nil
}
