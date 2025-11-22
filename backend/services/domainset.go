package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"smartdns-manager/database"
	"smartdns-manager/models"
)

type DomainSetService struct {
	notificationService *NotificationService
}

func NewDomainSetService() *DomainSetService {
	return &DomainSetService{
		notificationService: NewNotificationService(),
	}
}

// SyncDomainSetToNodes 同步域名集到节点
func (s *DomainSetService) SyncDomainSetToNodes(domainSet *models.DomainSet) error {
	if !domainSet.Enabled {
		return nil
	}

	// 获取目标节点
	nodes, err := s.getTargetNodes(domainSet.NodeIDs)
	if err != nil {
		return err
	}

	// 获取域名列表
	var items []models.DomainSetItem
	database.DB.Where("domain_set_id = ?", domainSet.ID).Find(&items)

	// 生成文件内容
	content := s.generateDomainSetFile(domainSet, items)

	// 同步到各个节点
	for _, node := range nodes {
		go s.syncDomainSetToNode(domainSet, &node, content)
	}

	return nil
}

// syncDomainSetToNode 同步域名集到单个节点
func (s *DomainSetService) syncDomainSetToNode(domainSet *models.DomainSet, node *models.Node, content string) {
	log.Printf("同步域名集 %s 到节点: %s", domainSet.Name, node.Name)

	client, err := NewSSHClient(node)
	if err != nil {
		log.Printf("连接节点失败: %v", err)
		return
	}
	defer client.Close()

	// 确保目录存在
	dirPath := "/etc/smartdns"
	client.ExecuteCommand(fmt.Sprintf("sudo mkdir -p %s", dirPath))

	// 写入域名集文件
	if err := client.WriteFile(domainSet.FilePath, content); err != nil {
		log.Printf("写入域名集文件失败: %v", err)
		return
	}

	// 更新主配置文件，确保引用了这个域名集
	s.ensureDomainSetInConfig(client, node, domainSet)
	s.notificationService.SendNotification(node.ID, "domain_set_sync", "域名集同步", fmt.Sprintf("域名集 %s 已完成同步 %s", domainSet.Name, node.Name))
	log.Printf(" 域名集 %s 同步成功: %s", domainSet.Name, node.Name)
}

// generateDomainSetFile 生成域名集文件内容
func (s *DomainSetService) generateDomainSetFile(domainSet *models.DomainSet, items []models.DomainSetItem) string {
	var builder strings.Builder

	builder.WriteString(fmt.Sprintf("# Domain Set: %s\n", domainSet.Name))
	if domainSet.Description != "" {
		builder.WriteString(fmt.Sprintf("# Description: %s\n", domainSet.Description))
	}
	builder.WriteString(fmt.Sprintf("# Total: %d domains\n", len(items)))
	builder.WriteString(fmt.Sprintf("# Generated at: %s\n\n", domainSet.UpdatedAt.Format("2006-01-02 15:04:05")))

	for _, item := range items {
		if item.Comment != "" {
			builder.WriteString(fmt.Sprintf("# %s\n", item.Comment))
		}
		builder.WriteString(fmt.Sprintf("%s\n", item.Domain))
	}

	return builder.String()
}

// ensureDomainSetInConfig 确保主配置文件中引用了域名集
func (s *DomainSetService) ensureDomainSetInConfig(client *SSHClient, node *models.Node, domainSet *models.DomainSet) error {
	// 读取当前配置
	configContent, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		return err
	}

	// 检查是否已经存在域名集定义
	domainSetLine := fmt.Sprintf("domain-set -name %s -file %s", domainSet.Name, domainSet.FilePath)

	if strings.Contains(configContent, domainSetLine) {
		return nil // 已存在
	}

	// 添加域名集定义
	lines := strings.Split(configContent, "\n")

	// 找到合适的位置插入（在 Domain Sets 部分）
	insertIndex := -1
	inDomainSetSection := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# Domain Sets") {
			inDomainSetSection = true
		}
		if inDomainSetSection && (trimmed == "" || strings.HasPrefix(trimmed, "# Domain Rules")) {
			insertIndex = i
			break
		}
	}

	// 如果没找到 Domain Sets 部分，创建一个
	if insertIndex == -1 {
		// 在文件末尾添加
		configContent += "\n# Domain Sets\n" + domainSetLine + "\n"
	} else {
		// 在找到的位置插入
		lines = append(lines[:insertIndex], append([]string{domainSetLine}, lines[insertIndex:]...)...)
		configContent = strings.Join(lines, "\n")
	}

	// 写回配置文件
	return client.WriteFile(node.ConfigPath, configContent)
}

// DeleteDomainSetFromNodes 从节点删除域名集
func (s *DomainSetService) DeleteDomainSetFromNodes(domainSet *models.DomainSet) error {
	nodes, err := s.getTargetNodes(domainSet.NodeIDs)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		go s.deleteDomainSetFromNode(domainSet, &node)
	}

	return nil
}

// deleteDomainSetFromNode 从单个节点删除域名集
func (s *DomainSetService) deleteDomainSetFromNode(domainSet *models.DomainSet, node *models.Node) {
	client, err := NewSSHClient(node)
	if err != nil {
		return
	}
	defer client.Close()

	// 删除域名集文件
	client.ExecuteCommand(fmt.Sprintf("sudo rm -f %s", domainSet.FilePath))

	// 从主配置文件中删除引用
	configContent, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		return
	}

	lines := strings.Split(configContent, "\n")
	var newLines []string

	domainSetLine := fmt.Sprintf("domain-set -name %s", domainSet.Name)

	for _, line := range lines {
		if !strings.Contains(line, domainSetLine) {
			newLines = append(newLines, line)
		}
	}

	s.notificationService.SendNotification(node.ID, "domain_set_sync", "域名集删除同步", fmt.Sprintf("域名集 %s 已删除 %s", domainSet.Name, node.Name))

	client.WriteFile(node.ConfigPath, strings.Join(newLines, "\n"))
}

// getTargetNodes 获取目标节点列表
func (s *DomainSetService) getTargetNodes(nodeIDsJSON string) ([]models.Node, error) {
	var nodes []models.Node

	if nodeIDsJSON == "" || nodeIDsJSON == "[]" {
		database.DB.Find(&nodes)
	} else {
		var nodeIDs []uint
		if err := json.Unmarshal([]byte(nodeIDsJSON), &nodeIDs); err != nil {
			return nil, err
		}
		database.DB.Where("id IN ?", nodeIDs).Find(&nodes)
	}

	return nodes, nil
}
