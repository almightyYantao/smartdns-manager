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

var domainRuleService = services.NewDomainRuleService()

// GetDomainRules 获取域名规则列表
func GetDomainRules(c *gin.Context) {
	var rules []models.DomainRule

	query := database.DB

	if domain := c.Query("domain"); domain != "" {
		query = query.Where("domain LIKE ?", "%"+domain+"%")
	}

	query.Order("priority desc, created_at desc").Find(&rules)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rules,
		"total":   len(rules),
	})
}

// AddDomainRule 添加域名规则
func AddDomainRule(c *gin.Context) {
	var request struct {
		Domain         string `json:"domain"`
		IsDomainSet    bool   `json:"is_domain_set"`
		DomainSetName  string `json:"domain_set_name"`
		Address        string `json:"address"`
		Nameserver     string `json:"nameserver"`
		SpeedCheckMode string `json:"speed_check_mode"`
		OtherOptions   string `json:"other_options"`
		Priority       int    `json:"priority"`
		Description    string `json:"description"`
		NodeIDs        []uint `json:"node_ids"`
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

	rule := models.DomainRule{
		Domain:         request.Domain,
		IsDomainSet:    request.IsDomainSet,
		DomainSetName:  request.DomainSetName,
		Address:        request.Address,
		Nameserver:     request.Nameserver,
		SpeedCheckMode: request.SpeedCheckMode,
		OtherOptions:   request.OtherOptions,
		Priority:       request.Priority,
		Description:    request.Description,
		NodeIDs:        nodeIDsJSON,
		Enabled:        true,
	}

	if err := database.DB.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "创建规则失败",
			"error":   err.Error(),
		})
		return
	}

	// 同步到节点
	go domainRuleService.SyncDomainRuleToNodes(&rule)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "规则创建成功",
		"data":    rule,
	})
}

// UpdateDomainRule 更新域名规则
func UpdateDomainRule(c *gin.Context) {
	id := c.Param("id")
	ruleID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的规则ID",
		})
		return
	}

	// 查询现有规则
	var rule models.DomainRule
	if err := database.DB.First(&rule, ruleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "规则不存在",
		})
		return
	}

	// 使用请求结构体接收数据
	var req models.UpdateDomainRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 更新字段
	rule.Domain = req.Domain
	rule.IsDomainSet = req.IsDomainSet
	rule.DomainSetName = req.DomainSetName
	rule.Address = req.Address
	rule.Nameserver = req.Nameserver
	rule.SpeedCheckMode = req.SpeedCheckMode
	rule.OtherOptions = req.OtherOptions
	rule.Priority = req.Priority
	rule.Description = req.Description

	// 处理 Enabled（如果传了就更新，没传就保持原值）
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}

	// 处理 NodeIDs：将数组转换为 JSON 字符串
	if req.NodeIDs != nil {
		nodeIDsJSON, err := json.Marshal(req.NodeIDs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "节点ID序列化失败",
			})
			return
		}
		rule.NodeIDs = string(nodeIDsJSON)
	} else {
		rule.NodeIDs = "[]"
	}

	// 保存到数据库
	if err := database.DB.Save(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "规则更新失败: " + err.Error(),
		})
		return
	}

	// 同步到节点
	go domainRuleService.SyncDomainRuleToNodes(&rule)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "规则更新成功",
		"data":    rule,
	})
}

// DeleteDomainRule 删除域名规则
func DeleteDomainRule(c *gin.Context) {
	id := c.Param("id")
	ruleID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的规则ID",
		})
		return
	}

	var rule models.DomainRule
	if err := database.DB.First(&rule, ruleID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "规则不存在",
		})
		return
	}

	database.DB.Delete(&rule)

	// 从节点删除
	go domainRuleService.DeleteDomainRuleFromNodes(&rule)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "规则删除成功",
	})
}
