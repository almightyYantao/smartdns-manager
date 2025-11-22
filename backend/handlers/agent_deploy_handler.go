package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

// DeployAgentRequest 部署请求结构
type DeployAgentRequest struct {
	NodeID             uint   `json:"node_id" binding:"required"`
	DeployMode         string `json:"deploy_mode" binding:"required"` // systemd 或 docker
	ClickHouseHost     string `json:"clickhouse_host" binding:"required"`
	ClickHousePort     int    `json:"clickhouse_port"`
	ClickHouseDB       string `json:"clickhouse_db"`
	ClickHouseUser     string `json:"clickhouse_user"`
	ClickHousePassword string `json:"clickhouse_password"`
	LogFilePath        string `json:"log_file_path"`
	BatchSize          int    `json:"batch_size"`
	FlushInterval      int    `json:"flush_interval"`
}

// DeployAgentResponse 部署响应结构
type DeployAgentResponse struct {
	Success     bool     `json:"success"`
	Message     string   `json:"message"`
	Output      []string `json:"output"`
	AgentStatus string   `json:"agent_status"`
}

// DeployAgent 一键部署 Agent
func DeployAgent(c *gin.Context) {
	var req models.DeployAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 获取节点信息
	var node models.Node
	if err := database.DB.First(&node, req.NodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "节点不存在",
		})
		return
	}

	// 设置默认值
	if req.ClickHousePort == 0 {
		req.ClickHousePort = 9000
	}
	if req.ClickHouseDB == "" {
		req.ClickHouseDB = "smartdns_logs"
	}
	if req.ClickHouseUser == "" {
		req.ClickHouseUser = "default"
	}
	if req.LogFilePath == "" {
		req.LogFilePath = "/var/log/audit/audit.log"
	}
	if req.BatchSize == 0 {
		req.BatchSize = 1000
	}
	if req.FlushInterval == 0 {
		req.FlushInterval = 2
	}

	// 创建部署服务
	deployService := services.NewAgentDeployService()

	// 执行部署
	response, err := deployService.DeployAgent(&node, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "部署失败: " + err.Error(),
		})
		return
	}

	// 更新节点 Agent 状态
	database.DB.Model(&node).Updates(map[string]interface{}{
		"agent_installed": true,
		"agent_version":   deployService.GetLatestVersion(),
		"deploy_mode":     req.DeployMode,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    response,
	})
}

// CheckAgentStatus 检查 Agent 状态
func CheckAgentStatus(c *gin.Context) {
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

	deployService := services.NewAgentDeployService()
	status, err := deployService.CheckAgentStatus(&node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "检查状态失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// UninstallAgent 卸载 Agent
func UninstallAgent(c *gin.Context) {
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

	deployService := services.NewAgentDeployService()
	err = deployService.UninstallAgent(&node)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "卸载失败: " + err.Error(),
		})
		return
	}

	// 更新节点状态
	database.DB.Model(&node).Updates(map[string]interface{}{
		"agent_installed": false,
		"agent_version":   "",
		"deploy_mode":     "",
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Agent 卸载成功",
	})
}
