package services

import (
	"encoding/json"
	"fmt"
	"log"
	"smartdns-manager/database"
	"smartdns-manager/models"
)

type ConfigSyncService struct {
	notificationService *NotificationService
}

func NewConfigSyncService() *ConfigSyncService {
	return &ConfigSyncService{
		notificationService: NewNotificationService(),
	}
}

// SyncAddressToNodes 同步地址映射到节点
func (s *ConfigSyncService) SyncAddressToNodes(address *models.AddressMap) error {
	if !address.Enabled {
		return nil // 未启用的不同步
	}

	// 获取目标节点
	nodes, err := s.getTargetNodes(address.NodeIDs)
	if err != nil {
		return err
	}

	// 并发同步到各个节点
	errChan := make(chan error, len(nodes))

	for _, node := range nodes {
		go func(n models.Node) {
			err := s.syncAddressToNode(address, &n)
			errChan <- err
		}(node)
	}

	// 收集错误
	var errors []error
	for i := 0; i < len(nodes); i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("同步失败: %v", errors)
	}

	return nil
}

// syncAddressToNode 同步地址映射到单个节点
func (s *ConfigSyncService) syncAddressToNode(address *models.AddressMap, node *models.Node) error {
	// 生成显示文本
	var displayText string
	if address.Type == "cname" {
		displayText = fmt.Sprintf("%s -> %s (CNAME)", address.Domain, address.CNAME)
	} else {
		displayText = fmt.Sprintf("%s -> %s", address.Domain, address.IP)
	}

	log.Printf("开始同步到节点: %s (%s)", node.Name, displayText)

	syncLog := &models.ConfigSyncLog{
		NodeID:  node.ID,
		Action:  "add",
		Type:    address.Type,
		Content: displayText,
		Status:  "pending",
	}
	database.DB.Create(syncLog)

	// 连接节点
	client, err := NewSSHClient(node)
	if err != nil {
		syncLog.Status = "failed"
		syncLog.Error = err.Error()
		database.DB.Save(syncLog)
		s.notificationService.SendNotification(node.ID, "sync_failed", "❌ 配置同步失败",
			fmt.Sprintf("%s 同步失败\n\n错误: %s", displayText, err.Error()))
		return err
	}
	defer client.Close()

	// 读取当前配置
	currentConfig, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		syncLog.Status = "failed"
		syncLog.Error = err.Error()
		database.DB.Save(syncLog)
		return err
	}

	// 解析配置
	parser := NewConfigParser()
	config, err := parser.Parse(currentConfig)
	if err != nil {
		syncLog.Status = "failed"
		syncLog.Error = err.Error()
		database.DB.Save(syncLog)
		return err
	}

	// 检查是否已存在并更新
	found := false
	for i, addr := range config.Addresses {
		if addr.Domain == address.Domain {
			// 更新现有记录
			config.Addresses[i].IP = address.IP
			config.Addresses[i].CNAME = address.CNAME
			config.Addresses[i].Type = address.Type
			found = true
			break
		}
	}

	// 如果不存在，添加新记录
	if !found {
		config.Addresses = append(config.Addresses, *address)
	}

	// 生成新配置
	newConfig := parser.Generate(config)

	// 创建备份
	_, err = client.CreateBackup(node.ConfigPath)
	if err != nil {
		log.Printf("警告: 创建备份失败: %v", err)
	}

	// 写入新配置
	err = client.WriteFile(node.ConfigPath, newConfig)
	if err != nil {
		syncLog.Status = "failed"
		syncLog.Error = err.Error()
		database.DB.Save(syncLog)
		return err
	}

	// 重载服务
	// err = client.ReloadService("smartdns")
	// if err != nil {
	// 	log.Printf("警告: 重载服务失败: %v", err)
	// }

	syncLog.Status = "success"
	database.DB.Save(syncLog)

	s.notificationService.SendNotification(node.ID, "sync_success", " 配置同步成功",
		fmt.Sprintf("%s 已成功同步到节点", displayText))

	log.Printf(" 成功同步到节点: %s", node.Name)
	return nil
}

// SyncServerToNodes 同步DNS服务器到节点
func (s *ConfigSyncService) SyncServerToNodes(server *models.DNSServer) error {
	if !server.Enabled {
		return nil
	}

	nodes, err := s.getTargetNodes(server.NodeIDs)
	if err != nil {
		return err
	}

	errChan := make(chan error, len(nodes))

	for _, node := range nodes {
		go func(n models.Node) {
			err := s.syncServerToNode(server, &n)
			errChan <- err
		}(node)
	}

	var errors []error
	for i := 0; i < len(nodes); i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("同步失败: %v", errors)
	}

	return nil
}

// syncServerToNode 同步DNS服务器到单个节点
func (s *ConfigSyncService) syncServerToNode(server *models.DNSServer, node *models.Node) error {
	log.Printf("开始同步DNS服务器到节点: %s (%s)", node.Name, server.Address)

	syncLog := &models.ConfigSyncLog{
		NodeID:  node.ID,
		Action:  "add",
		Type:    "server",
		Content: server.Address,
		Status:  "pending",
	}
	database.DB.Create(syncLog)

	client, err := NewSSHClient(node)
	if err != nil {
		syncLog.Status = "failed"
		syncLog.Error = err.Error()
		database.DB.Save(syncLog)
		return err
	}
	defer client.Close()

	currentConfig, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		syncLog.Status = "failed"
		syncLog.Error = err.Error()
		database.DB.Save(syncLog)
		return err
	}

	parser := NewConfigParser()
	config, err := parser.Parse(currentConfig)
	if err != nil {
		syncLog.Status = "failed"
		syncLog.Error = err.Error()
		database.DB.Save(syncLog)
		return err
	}

	// 检查是否已存在
	serverExists := false
	for i, srv := range config.Servers {
		if srv.Address == server.Address {
			config.Servers[i] = *server
			serverExists = true
			break
		}
	}

	if !serverExists {
		config.Servers = append(config.Servers, *server)
	}

	newConfig := parser.Generate(config)

	_, err = client.CreateBackup(node.ConfigPath)
	if err != nil {
		log.Printf("警告: 创建备份失败: %v", err)
	}

	err = client.WriteFile(node.ConfigPath, newConfig)
	if err != nil {
		syncLog.Status = "failed"
		syncLog.Error = err.Error()
		database.DB.Save(syncLog)
		return err
	}

	syncLog.Status = "success"
	database.DB.Save(syncLog)

	log.Printf(" 成功同步DNS服务器到节点: %s", node.Name)
	return nil
}

// DeleteAddressFromNodes 从节点删除地址映射
func (s *ConfigSyncService) DeleteAddressFromNodes(address *models.AddressMap) error {
	nodes, err := s.getTargetNodes(address.NodeIDs)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if err := s.deleteAddressFromNode(address, &node); err != nil {
			log.Printf("删除地址映射失败 (节点: %s): %v", node.Name, err)
		}
	}

	return nil
}

// deleteAddressFromNode 从单个节点删除地址映射
func (s *ConfigSyncService) deleteAddressFromNode(address *models.AddressMap, node *models.Node) error {
	client, err := NewSSHClient(node)
	if err != nil {
		return err
	}
	defer client.Close()

	currentConfig, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		return err
	}

	parser := NewConfigParser()
	config, err := parser.Parse(currentConfig)
	if err != nil {
		return err
	}

	// 过滤掉要删除的地址
	newAddresses := []models.AddressMap{}
	for _, addr := range config.Addresses {
		if addr.Domain != address.Domain || addr.IP != address.IP {
			newAddresses = append(newAddresses, addr)
		}
	}
	config.Addresses = newAddresses

	newConfig := parser.Generate(config)

	client.CreateBackup(node.ConfigPath)
	return client.WriteFile(node.ConfigPath, newConfig)
}

func (s *ConfigSyncService) FullSyncToNode(nodeID uint) error {
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		return err
	}
	log.Printf("开始完整同步配置到节点: %s", node.Name)

	// 连接节点
	client, err := NewSSHClient(&node)
	if err != nil {
		return err
	}
	defer client.Close()

	// 读取现有配置
	currentConfig, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		return err
	}

	// 解析现有配置
	parser := NewConfigParser()
	config, err := parser.Parse(currentConfig)
	if err != nil {
		return err
	}

	// 获取数据库中的配置
	var dbAddresses []models.AddressMap
	database.DB.Where("enabled = ?", true).Find(&dbAddresses)
	targetAddresses := s.filterConfigForNode(dbAddresses, nodeID)

	var dbServers []models.DNSServer
	database.DB.Where("enabled = ?", true).Find(&dbServers)
	targetServers := s.filterServersForNode(dbServers, nodeID)

	// 合并配置：保留现有的，添加数据库中的
	config = s.mergeConfigs(config, targetServers, targetAddresses)

	// 创建备份
	backupPath, err := client.CreateBackup(node.ConfigPath)
	if err != nil {
		log.Printf("警告: 创建备份失败: %v", err)
	} else {
		log.Printf("配置已备份到: %s", backupPath)
	}

	// 生成新配置
	newConfig := parser.Generate(config)

	// 写入配置
	err = client.WriteFile(node.ConfigPath, newConfig)
	if err != nil {
		return err
	}

	log.Printf(" 完整同步成功: %s", node.Name)
	return nil
}

// 合并配置的辅助方法
func (s *ConfigSyncService) mergeConfigs(existingConfig *models.SmartDNSConfig, dbServers []models.DNSServer, dbAddresses []models.AddressMap) *models.SmartDNSConfig {
	// 合并服务器配置
	for _, dbServer := range dbServers {
		found := false
		// 检查是否已存在（按地址匹配）
		for i, existing := range existingConfig.Servers {
			if existing.Address == dbServer.Address {
				// 更新现有的
				existingConfig.Servers[i] = dbServer
				found = true
				break
			}
		}
		// 如果不存在，添加新的
		if !found {
			existingConfig.Servers = append(existingConfig.Servers, dbServer)
		}
	}

	// 合并地址映射配置
	for _, dbAddr := range dbAddresses {
		found := false
		// 检查是否已存在（按域名匹配）
		for i, existing := range existingConfig.Addresses {
			if existing.Domain == dbAddr.Domain {
				// 更新现有的
				existingConfig.Addresses[i] = dbAddr
				found = true
				break
			}
		}
		// 如果不存在，添加新的
		if !found {
			existingConfig.Addresses = append(existingConfig.Addresses, dbAddr)
		}
	}

	return existingConfig
}

// getTargetNodes 获取目标节点列表
func (s *ConfigSyncService) getTargetNodes(nodeIDsJSON string) ([]models.Node, error) {
	var nodes []models.Node

	if nodeIDsJSON == "" || nodeIDsJSON == "[]" {
		// 空表示所有节点
		database.DB.Find(&nodes)
	} else {
		// 解析节点ID列表
		var nodeIDs []uint
		if err := json.Unmarshal([]byte(nodeIDsJSON), &nodeIDs); err != nil {
			return nil, err
		}
		database.DB.Where("id IN ?", nodeIDs).Find(&nodes)
	}

	return nodes, nil
}

// filterConfigForNode 过滤出应用到指定节点的配置
func (s *ConfigSyncService) filterConfigForNode(addresses []models.AddressMap, nodeID uint) []models.AddressMap {
	result := []models.AddressMap{}

	for _, addr := range addresses {
		if addr.NodeIDs == "" || addr.NodeIDs == "[]" {
			// 空表示应用到所有节点
			result = append(result, addr)
		} else {
			var nodeIDs []uint
			json.Unmarshal([]byte(addr.NodeIDs), &nodeIDs)
			for _, id := range nodeIDs {
				if id == nodeID {
					result = append(result, addr)
					break
				}
			}
		}
	}

	return result
}

func (s *ConfigSyncService) filterServersForNode(servers []models.DNSServer, nodeID uint) []models.DNSServer {
	result := []models.DNSServer{}

	for _, srv := range servers {
		if srv.NodeIDs == "" || srv.NodeIDs == "[]" {
			result = append(result, srv)
		} else {
			var nodeIDs []uint
			json.Unmarshal([]byte(srv.NodeIDs), &nodeIDs)
			for _, id := range nodeIDs {
				if id == nodeID {
					result = append(result, srv)
					break
				}
			}
		}
	}

	return result
}
