package handlers

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"smartdns-manager/database"
	"smartdns-manager/models"
	"smartdns-manager/services"
)

// GetDashboardStats 获取仪表板统计信息
func GetDashboardStats(c *gin.Context) {
	stats := make(map[string]interface{})

	// 节点统计
	var nodeCount int64
	database.DB.Model(&models.Node{}).Count(&nodeCount)
	stats["total_nodes"] = nodeCount

	var onlineNodes int64
	database.DB.Model(&models.Node{}).Where("status = ?", "online").Count(&onlineNodes)
	stats["online_nodes"] = onlineNodes

	var offlineNodes int64
	database.DB.Model(&models.Node{}).Where("status = ?", "offline").Count(&offlineNodes)
	stats["offline_nodes"] = offlineNodes

	// DNS服务器统计
	var serverCount int64
	database.DB.Model(&models.DNSServer{}).Count(&serverCount)
	stats["total_servers"] = serverCount

	// 按类型统计
	var serversByType []struct {
		Type  string
		Count int64
	}
	database.DB.Model(&models.DNSServer{}).
		Select("type, count(*) as count").
		Group("type").
		Scan(&serversByType)
	stats["servers_by_type"] = serversByType

	// 地址映射统计
	var addressCount int64
	database.DB.Model(&models.AddressMap{}).Count(&addressCount)
	stats["total_addresses"] = addressCount

	// 域名集统计
	var domainSetCount int64
	database.DB.Model(&models.DomainSet{}).Count(&domainSetCount)
	stats["total_domain_sets"] = domainSetCount

	// 域名规则统计
	var domainRuleCount int64
	database.DB.Model(&models.DomainRule{}).Count(&domainRuleCount)
	stats["total_domain_rules"] = domainRuleCount

	// 最近添加的地址映射
	var recentAddresses []models.AddressMap
	database.DB.Order("created_at desc").Limit(10).Find(&recentAddresses)
	stats["recent_addresses"] = recentAddresses

	// 最近添加的节点
	var recentNodes []models.Node
	database.DB.Order("created_at desc").Limit(5).Find(&recentNodes)
	stats["recent_nodes"] = recentNodes

	// 系统信息
	stats["system"] = map[string]interface{}{
		"version":    "1.0.0",
		"updated_at": time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// GetNodesHealth 获取所有节点健康状态
func GetNodesHealth(c *gin.Context) {
	var nodes []models.Node
	if err := database.DB.Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取节点列表失败",
			"error":   err.Error(),
		})
		return
	}

	type NodeHealth struct {
		NodeID     uint               `json:"node_id"`
		NodeName   string             `json:"node_name"`
		Status     string             `json:"status"`
		HealthData *models.NodeStatus `json:"health_data,omitempty"`
		Error      string             `json:"error,omitempty"`
		CheckedAt  time.Time          `json:"checked_at"`
	}

	healthResults := make([]NodeHealth, 0, len(nodes))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 并发检查所有节点
	for _, node := range nodes {
		wg.Add(1)
		go func(n models.Node) {
			defer wg.Done()

			health := NodeHealth{
				NodeID:    n.ID,
				NodeName:  n.Name,
				Status:    n.Status,
				CheckedAt: time.Now(),
			}

			// 尝试连接节点
			client, err := services.NewSSHClient(&n)
			if err != nil {
				health.Status = "offline"
				health.Error = err.Error()
			} else {
				defer client.Close()

				// 获取详细健康信息
				status, err := client.GetSystemInfo()
				if err != nil {
					health.Status = "error"
					health.Error = err.Error()
				} else {
					health.Status = "online"
					health.HealthData = status
				}

				// 更新数据库中的状态
				n.Status = health.Status
				n.LastCheck = time.Now()
				database.DB.Save(&n)
			}

			mu.Lock()
			healthResults = append(healthResults, health)
			mu.Unlock()
		}(node)
	}

	wg.Wait()

	// 统计信息
	summary := map[string]int{
		"total":   len(nodes),
		"online":  0,
		"offline": 0,
		"error":   0,
	}

	for _, h := range healthResults {
		switch h.Status {
		case "online":
			summary["online"]++
		case "offline":
			summary["offline"]++
		case "error":
			summary["error"]++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    healthResults,
		"summary": summary,
	})
}

// GetNodeMetrics 获取节点性能指标
func GetNodeMetrics(c *gin.Context) {
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

	// 收集多个时间点的数据
	metrics := make([]models.NodeStatus, 0)
	for i := 0; i < 5; i++ {
		status, err := client.GetSystemInfo()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "获取系统信息失败",
				"error":   err.Error(),
			})
			return
		}
		metrics = append(metrics, *status)
		if i < 4 {
			time.Sleep(2 * time.Second)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}

// GetSystemOverview 获取系统总览
func GetSystemOverview(c *gin.Context) {
	overview := make(map[string]interface{})

	// 节点概览
	var nodes []models.Node
	database.DB.Find(&nodes)

	nodeStats := map[string]int{
		"total":   len(nodes),
		"online":  0,
		"offline": 0,
		"error":   0,
	}

	for _, node := range nodes {
		switch node.Status {
		case "online":
			nodeStats["online"]++
		case "offline":
			nodeStats["offline"]++
		default:
			nodeStats["error"]++
		}
	}
	overview["nodes"] = nodeStats

	// 配置统计
	var serverCount, addressCount, domainSetCount, domainRuleCount int64
	database.DB.Model(&models.DNSServer{}).Count(&serverCount)
	database.DB.Model(&models.AddressMap{}).Count(&addressCount)
	database.DB.Model(&models.DomainSet{}).Count(&domainSetCount)
	database.DB.Model(&models.DomainRule{}).Count(&domainRuleCount)

	overview["config"] = map[string]int64{
		"servers":      serverCount,
		"addresses":    addressCount,
		"domain_sets":  domainSetCount,
		"domain_rules": domainRuleCount,
	}

	// 最近活动
	var recentActivities []map[string]interface{}

	// 最近添加的地址
	var recentAddresses []models.AddressMap
	database.DB.Order("created_at desc").Limit(5).Find(&recentAddresses)
	for _, addr := range recentAddresses {
		recentActivities = append(recentActivities, map[string]interface{}{
			"type":       "address_added",
			"content":    addr.Domain + " -> " + addr.IP,
			"created_at": addr.CreatedAt,
		})
	}

	// 最近更新的节点
	var recentNodes []models.Node
	database.DB.Order("updated_at desc").Limit(5).Find(&recentNodes)
	for _, node := range recentNodes {
		recentActivities = append(recentActivities, map[string]interface{}{
			"type":       "node_updated",
			"content":    node.Name,
			"created_at": node.UpdatedAt,
		})
	}

	overview["recent_activities"] = recentActivities

	// 系统健康评分（简单算法）
	healthScore := float64(nodeStats["online"]) / float64(nodeStats["total"]) * 100
	overview["health_score"] = healthScore

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    overview,
	})
}
