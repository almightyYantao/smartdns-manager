package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"smartdns-manager/database"
	"smartdns-manager/models"
)

type NameserverService struct{}

func NewNameserverService() *NameserverService {
	return &NameserverService{}
}

// SyncNameserverToNodes 同步命名服务器规则到节点
func (s *NameserverService) SyncNameserverToNodes(nameserver *models.Nameserver) error {
	if !nameserver.Enabled {
		return nil
	}

	nodes, err := s.getTargetNodes(nameserver.NodeIDs)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		go s.syncNameserverToNode(nameserver, &node)
	}

	return nil
}

// syncNameserverToNode 同步命名服务器规则到单个节点
func (s *NameserverService) syncNameserverToNode(nameserver *models.Nameserver, node *models.Node) {
	log.Printf("同步 nameserver 规则到节点: %s", node.Name)

	client, err := NewSSHClient(node)
	if err != nil {
		return
	}
	defer client.Close()

	configContent, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		return
	}

	// 生成规则行
	ruleLine := s.generateNameserverLine(nameserver)

	// 更新配置
	newContent := s.updateNameserversInConfig(configContent, ruleLine, nameserver.Domain)

	client.WriteFile(node.ConfigPath, newContent)
}

// generateNameserverLine 生成命名服务器规则行
func (s *NameserverService) generateNameserverLine(nameserver *models.Nameserver) string {
	var domain string
	if nameserver.IsDomainSet {
		domain = fmt.Sprintf("/domain-set:%s/", nameserver.DomainSetName)
	} else {
		domain = fmt.Sprintf("/%s/", nameserver.Domain)
	}

	return fmt.Sprintf("nameserver %s%s", domain, nameserver.Group)
}

// updateNameserversInConfig 在配置中更新命名服务器规则
func (s *NameserverService) updateNameserversInConfig(configContent, newRuleLine, domain string) string {
	lines := strings.Split(configContent, "\n")
	var newLines []string
	ruleFound := false

	for _, line := range lines {
		// 检查是否是要更新的规则
		if strings.Contains(line, fmt.Sprintf("nameserver /%s/", domain)) ||
			strings.Contains(line, fmt.Sprintf("nameserver /domain-set:%s/", domain)) {
			newLines = append(newLines, newRuleLine)
			ruleFound = true
		} else {
			newLines = append(newLines, line)
		}
	}

	// 如果规则不存在，添加到 Nameserver Rules 部分
	if !ruleFound {
		insertIndex := -1
		for i, line := range newLines {
			if strings.Contains(line, "# Nameserver Rules") {
				insertIndex = i + 1
				break
			}
		}

		if insertIndex == -1 {
			newLines = append(newLines, "\n# Nameserver Rules", newRuleLine)
		} else {
			newLines = append(newLines[:insertIndex], append([]string{newRuleLine}, newLines[insertIndex:]...)...)
		}
	}

	return strings.Join(newLines, "\n")
}

// DeleteNameserverFromNodes 从节点删除命名服务器规则
func (s *NameserverService) DeleteNameserverFromNodes(nameserver *models.Nameserver) error {
	nodes, err := s.getTargetNodes(nameserver.NodeIDs)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		go s.deleteNameserverFromNode(nameserver, &node)
	}

	return nil
}

func (s *NameserverService) deleteNameserverFromNode(nameserver *models.Nameserver, node *models.Node) {
	client, err := NewSSHClient(node)
	if err != nil {
		return
	}
	defer client.Close()

	configContent, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		return
	}

	lines := strings.Split(configContent, "\n")
	var newLines []string

	searchPattern := fmt.Sprintf("nameserver /%s/", nameserver.Domain)
	if nameserver.IsDomainSet {
		searchPattern = fmt.Sprintf("nameserver /domain-set:%s/", nameserver.DomainSetName)
	}

	for _, line := range lines {
		if !strings.Contains(line, searchPattern) {
			newLines = append(newLines, line)
		}
	}

	client.WriteFile(node.ConfigPath, strings.Join(newLines, "\n"))
}

func (s *NameserverService) getTargetNodes(nodeIDsJSON string) ([]models.Node, error) {
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
