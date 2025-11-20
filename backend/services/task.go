package services

import (
	"fmt"
	"log"
	"smartdns-manager/database"
	"smartdns-manager/models"
	"strings"
	"sync"
	"time"
)

// NodeHealthChecker 节点健康检查器
type NodeHealthChecker struct {
	ticker              *time.Ticker
	stopChan            chan bool
	notificationService *NotificationService
	lastErrorStatus     map[uint]string        // 记录节点上次的错误状态
	nodeStatusCache     map[uint]string        // 节点状态缓存
	mu                  sync.RWMutex           // 保护并发访问
	batchUpdateChan     chan *nodeStatusUpdate // 批量更新通道
}

type nodeStatusUpdate struct {
	nodeID    uint
	status    string
	lastCheck time.Time
}

// NewNodeHealthChecker 创建健康检查器
func NewNodeHealthChecker(interval time.Duration) *NodeHealthChecker {
	checker := &NodeHealthChecker{
		ticker:              time.NewTicker(interval),
		stopChan:            make(chan bool),
		notificationService: NewNotificationService(),
		lastErrorStatus:     make(map[uint]string),
		nodeStatusCache:     make(map[uint]string),
		batchUpdateChan:     make(chan *nodeStatusUpdate, 100),
	}

	// 启动批量更新协程
	go checker.batchUpdateWorker()

	return checker
}

// Start 启动定时检查
func (checker *NodeHealthChecker) Start() {
	log.Println("节点健康检查任务已启动")

	// 初始化缓存
	checker.initCache()

	// 立即执行一次
	checker.checkAllNodes()

	go func() {
		for {
			select {
			case <-checker.ticker.C:
				checker.checkAllNodes()
			case <-checker.stopChan:
				log.Println("节点健康检查任务已停止")
				return
			}
		}
	}()
}

// Stop 停止定时检查
func (checker *NodeHealthChecker) Stop() {
	checker.ticker.Stop()
	checker.stopChan <- true
	close(checker.batchUpdateChan)
}

// initCache 初始化状态缓存
func (checker *NodeHealthChecker) initCache() {
	var nodes []models.Node
	if err := database.DB.Select("id, status").Find(&nodes).Error; err != nil {
		log.Printf("初始化状态缓存失败: %v", err)
		return
	}

	checker.mu.Lock()
	defer checker.mu.Unlock()

	for _, node := range nodes {
		checker.nodeStatusCache[node.ID] = node.Status
	}
}

// batchUpdateWorker 批量更新数据库
func (checker *NodeHealthChecker) batchUpdateWorker() {
	ticker := time.NewTicker(5 * time.Second) // 每5秒批量更新一次
	defer ticker.Stop()

	updates := make([]*nodeStatusUpdate, 0, 50)

	for {
		select {
		case update, ok := <-checker.batchUpdateChan:
			if !ok {
				// 通道关闭，执行最后一次批量更新
				if len(updates) > 0 {
					checker.executeBatchUpdate(updates)
				}
				return
			}
			updates = append(updates, update)

			// 如果累积到一定数量，立即执行
			if len(updates) >= 50 {
				checker.executeBatchUpdate(updates)
				updates = make([]*nodeStatusUpdate, 0, 50)
			}

		case <-ticker.C:
			// 定时批量更新
			if len(updates) > 0 {
				checker.executeBatchUpdate(updates)
				updates = make([]*nodeStatusUpdate, 0, 50)
			}
		}
	}
}

// executeBatchUpdate 执行批量更新
func (checker *NodeHealthChecker) executeBatchUpdate(updates []*nodeStatusUpdate) {
	if len(updates) == 0 {
		return
	}

	// 使用事务批量更新
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	for _, update := range updates {
		if err := tx.Model(&models.Node{}).
			Where("id = ?", update.nodeID).
			Updates(map[string]interface{}{
				"status":     update.status,
				"last_check": update.lastCheck,
			}).Error; err != nil {
			log.Printf("批量更新节点状态失败: %v", err)
			tx.Rollback()
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("提交批量更新失败: %v", err)
	} else {
		log.Printf("批量更新了 %d 个节点状态", len(updates))
	}
}

// checkAllNodes 检查所有节点
func (checker *NodeHealthChecker) checkAllNodes() {
	var nodes []models.Node
	// 只查询必要的字段
	if err := database.DB.Select("id, name, host, port, username, password, config_path, status").Find(&nodes).Error; err != nil {
		log.Printf("获取节点列表失败: %v", err)
		return
	}

	// 使用 WaitGroup 等待所有检查完成
	var wg sync.WaitGroup
	// 限制并发数，避免同时发起太多SSH连接
	semaphore := make(chan struct{}, 10) // 最多10个并发

	for i := range nodes {
		wg.Add(1)
		go func(node *models.Node) {
			defer wg.Done()
			semaphore <- struct{}{}        // 获取信号量
			defer func() { <-semaphore }() // 释放信号量

			checker.checkNode(node)
		}(&nodes[i])
	}

	wg.Wait()
}

// checkNode 检查单个节点
func (checker *NodeHealthChecker) checkNode(node *models.Node) {
	// 从缓存获取旧状态
	checker.mu.RLock()
	oldStatus := checker.nodeStatusCache[node.ID]
	checker.mu.RUnlock()

	client, err := NewSSHClient(node)
	if err != nil {
		checker.updateNodeStatusAsync(node, oldStatus, "offline")
		log.Printf("节点 %s SSH连接失败: %v", node.Name, err)
		checker.sendNotificationIfNeeded(node, oldStatus, "offline",
			"⚠️ 节点连接失败",
			fmt.Sprintf("节点：%s\n状态：SSH连接失败\n时间：%s\n原因：%v",
				node.Name, time.Now().Format("2006-01-02 15:04:05"), err))
		return
	}
	defer client.Close()

	// 检查配置文件
	_, err = client.ReadFile(node.ConfigPath)
	if err != nil {
		checker.updateNodeStatusAsync(node, oldStatus, "error")
		log.Printf("节点 %s 配置文件不存在: %v", node.Name, err)
		checker.sendNotificationIfNeeded(node, oldStatus, "error",
			"❌ 节点配置异常",
			fmt.Sprintf("节点：%s\n状态：配置文件缺失\n时间：%s\n路径：%s",
				node.Name, time.Now().Format("2006-01-02 15:04:05"), node.ConfigPath))
		return
	}

	// 检查 SmartDNS 服务状态
	output, err := client.ExecuteCommand("systemctl is-active smartdns 2>&1")
	if err != nil || strings.TrimSpace(output) != "active" {
		checker.updateNodeStatusAsync(node, oldStatus, "stopped")
		log.Printf("节点 %s SmartDNS服务未运行: %s", node.Name, output)
		checker.sendNotificationIfNeeded(node, oldStatus, "stopped",
			"🛑 SmartDNS服务已停止",
			fmt.Sprintf("节点：%s\n状态：服务未运行\n时间：%s\n详情：%s",
				node.Name, time.Now().Format("2006-01-02 15:04:05"), strings.TrimSpace(output)))
		return
	}

	// 检查服务运行状态（简化检查，避免额外的SSH命令）
	statusOutput, err := client.ExecuteCommand("systemctl status smartdns 2>&1")
	if err != nil || !strings.Contains(statusOutput, "active (running)") {
		checker.updateNodeStatusAsync(node, oldStatus, "error")
		log.Printf("节点 %s SmartDNS服务状态异常", node.Name)
		checker.sendNotificationIfNeeded(node, oldStatus, "error",
			"⚠️ SmartDNS服务异常",
			fmt.Sprintf("节点：%s\n状态：服务状态异常\n时间：%s",
				node.Name, time.Now().Format("2006-01-02 15:04:05")))
		return
	}

	// 所有检查通过
	checker.updateNodeStatusAsync(node, oldStatus, "online")

	// 如果之前是错误状态，现在恢复了，发送恢复通知
	if oldStatus != "online" && oldStatus != "" {
		checker.sendRecoveryNotification(node, oldStatus)
	}
}

// updateNodeStatusAsync 异步更新节点状态（通过批量更新通道）
func (checker *NodeHealthChecker) updateNodeStatusAsync(node *models.Node, oldStatus, newStatus string) {
	// 更新缓存
	checker.mu.Lock()
	checker.nodeStatusCache[node.ID] = newStatus
	checker.mu.Unlock()

	// 只有状态真正改变时才推送到更新队列
	if oldStatus != newStatus {
		checker.batchUpdateChan <- &nodeStatusUpdate{
			nodeID:    node.ID,
			status:    newStatus,
			lastCheck: time.Now(),
		}
	}
}

// sendNotificationIfNeeded 仅在状态改变时发送通知
func (checker *NodeHealthChecker) sendNotificationIfNeeded(node *models.Node, oldStatus, newStatus, title, message string) {
	// 如果状态没有变化，不发送通知
	if oldStatus == newStatus {
		return
	}

	checker.mu.Lock()
	lastError, exists := checker.lastErrorStatus[node.ID]
	if exists && lastError == newStatus {
		checker.mu.Unlock()
		return
	}
	checker.lastErrorStatus[node.ID] = newStatus
	checker.mu.Unlock()

	// 异步发送通知，不阻塞检查流程
	go checker.notificationService.SendNotification(
		node.ID,
		"node_health_check",
		title,
		message,
	)
}

// sendRecoveryNotification 发送恢复通知
func (checker *NodeHealthChecker) sendRecoveryNotification(node *models.Node, oldStatus string) {
	statusText := map[string]string{
		"offline": "连接失败",
		"stopped": "服务停止",
		"error":   "状态异常",
	}

	message := fmt.Sprintf("节点：%s\n状态：已恢复正常 ✅\n时间：%s\n之前状态：%s",
		node.Name,
		time.Now().Format("2006-01-02 15:04:05"),
		statusText[oldStatus])

	// 异步发送通知
	go checker.notificationService.SendNotification(
		node.ID,
		"node_health_check",
		"✅ 节点已恢复",
		message,
	)

	// 清除错误状态记录
	checker.mu.Lock()
	delete(checker.lastErrorStatus, node.ID)
	checker.mu.Unlock()
}
