package services

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"smartdns-manager/database"
	"smartdns-manager/models"
)

type DomainRuleService struct{}

func NewDomainRuleService() *DomainRuleService {
	return &DomainRuleService{}
}

// SyncDomainRuleToNodes 同步域名规则到节点
func (s *DomainRuleService) SyncDomainRuleToNodes(rule *models.DomainRule) error {
	if !rule.Enabled {
		return nil
	}

	nodes, err := s.getTargetNodes(rule.NodeIDs)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		go s.syncDomainRuleToNode(rule, &node)
	}

	return nil
}

// syncDomainRuleToNode 同步域名规则到单个节点
func (s *DomainRuleService) syncDomainRuleToNode(rule *models.DomainRule, node *models.Node) {
	log.Printf("同步域名规则到节点: %s", node.Name)

	client, err := NewSSHClient(node)
	if err != nil {
		return
	}
	defer client.Close()

	// 读取配置
	configContent, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		return
	}

	// 生成规则行
	ruleLine := s.generateDomainRuleLine(rule)

	// 更新配置
	newContent := s.updateDomainRulesInConfig(configContent, ruleLine, rule.Domain)

	// 写回配置
	client.WriteFile(node.ConfigPath, newContent)
}

// generateDomainRuleLine 生成域名规则行
func (s *DomainRuleService) generateDomainRuleLine(rule *models.DomainRule) string {
	var domain string
	if rule.IsDomainSet {
		domain = fmt.Sprintf("/domain-set:%s/", rule.DomainSetName)
	} else {
		domain = fmt.Sprintf("/%s/", rule.Domain)
	}

	var options []string

	if rule.Address != "" {
		options = append(options, fmt.Sprintf("-address %s", rule.Address))
	}
	if rule.Nameserver != "" {
		options = append(options, fmt.Sprintf("-nameserver %s", rule.Nameserver))
	}
	if rule.SpeedCheckMode != "" {
		options = append(options, fmt.Sprintf("-speed-check-mode %s", rule.SpeedCheckMode))
	}
	if rule.OtherOptions != "" {
		options = append(options, rule.OtherOptions)
	}

	optionsStr := ""
	if len(options) > 0 {
		optionsStr = " " + strings.Join(options, " ")
	}

	return fmt.Sprintf("domain-rules %s%s", domain, optionsStr)
}

// updateDomainRulesInConfig 在配置中更新域名规则
func (s *DomainRuleService) updateDomainRulesInConfig(configContent, newRuleLine, domain string) string {
	lines := strings.Split(configContent, "\n")
	var newLines []string
	ruleFound := false
	inDomainRulesSection := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# Domain Rules") {
			inDomainRulesSection = true
		}

		if inDomainRulesSection && strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "# Domain Rules") {
			inDomainRulesSection = false
		}

		// 检查是否是要更新的规则
		if strings.Contains(line, fmt.Sprintf("domain-rules /%s/", domain)) ||
			strings.Contains(line, fmt.Sprintf("domain-rules /domain-set:%s/", domain)) {
			newLines = append(newLines, newRuleLine)
			ruleFound = true
		} else {
			newLines = append(newLines, line)
		}
	}

	// 如果规则不存在，添加到 Domain Rules 部分
	if !ruleFound {
		insertIndex := -1
		for i, line := range newLines {
			if strings.Contains(line, "# Domain Rules") {
				insertIndex = i + 1
				break
			}
		}

		if insertIndex == -1 {
			// 没有 Domain Rules 部分，创建一个
			newLines = append(newLines, "\n# Domain Rules", newRuleLine)
		} else {
			// 在 Domain Rules 部分后插入
			newLines = append(newLines[:insertIndex], append([]string{newRuleLine}, newLines[insertIndex:]...)...)
		}
	}

	return strings.Join(newLines, "\n")
}

// DeleteDomainRuleFromNodes 从节点删除域名规则
func (s *DomainRuleService) DeleteDomainRuleFromNodes(rule *models.DomainRule) error {
	nodes, err := s.getTargetNodes(rule.NodeIDs)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		go s.deleteDomainRuleFromNode(rule, &node)
	}

	return nil
}

// deleteDomainRuleFromNode 从单个节点删除域名规则
func (s *DomainRuleService) deleteDomainRuleFromNode(rule *models.DomainRule, node *models.Node) {
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

	searchPattern := fmt.Sprintf("domain-rules /%s/", rule.Domain)
	if rule.IsDomainSet {
		searchPattern = fmt.Sprintf("domain-rules /domain-set:%s/", rule.DomainSetName)
	}

	for _, line := range lines {
		if !strings.Contains(line, searchPattern) {
			newLines = append(newLines, line)
		}
	}

	client.WriteFile(node.ConfigPath, strings.Join(newLines, "\n"))
}

func (s *DomainRuleService) getTargetNodes(nodeIDsJSON string) ([]models.Node, error) {
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
