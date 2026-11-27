package services

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strings"

	"gorm.io/gorm"

	"smartdns-manager/models"
)

var ErrNodeAddressChanged = errors.New("节点配置已变化，请重新读取后重试")
var ErrNodeAddressAssigned = errors.New("该映射在此节点已由控制台分配，不能在这里删除")
var ErrNodeAddressDisabled = errors.New("相同的全局映射已停用，请先在全局地址映射中确认其启用范围")
var ErrNodeAddressConflict = errors.New("当前节点已分配同域名、不同类型或目标的映射，请先处理冲突")
var ordinaryAddressDomain = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)*\.?$`)

func ordinaryAddressMapping(mapping models.AddressMap) bool {
	if !ordinaryAddressDomain.MatchString(mapping.Domain) {
		return false
	}
	if mapping.Type == "cname" {
		return ordinaryAddressDomain.MatchString(mapping.CNAME)
	}
	for _, value := range strings.Split(mapping.IP, ",") {
		if ip, err := netip.ParseAddr(value); err != nil || ip.Zone() != "" {
			return false
		}
	}
	return true
}

func addressDirective(address *models.AddressMap) string {
	kind, value := "address", address.IP
	if address.Type == "cname" {
		kind, value = "cname", strings.ToLower(address.CNAME)
	}
	return fmt.Sprintf("%s /%s/%s", kind, strings.ToLower(address.Domain), value)
}

type parsedAddressDirective struct {
	mapping models.AddressMap
	line    int
}

// 只比较普通顶层映射，规则组内或引用文件中的指令有独立的作用范围。
func parseAddressDirectives(lines []string) (entries []parsedAddressDirective, partial bool) {
	depth := 0
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "conf-file", "conf-dir", "group-begin":
			partial = true
			if fields[0] == "group-begin" {
				depth++
			}
		case "group-end":
			if depth > 0 {
				depth--
			}
		case "address", "cname":
			if depth != 0 {
				continue
			}
			if len(fields) < 2 || (len(fields) > 2 && !strings.HasPrefix(fields[2], "#")) {
				partial = true
				continue
			}
			parts := strings.SplitN(fields[1], "/", 3)
			if len(parts) != 3 || parts[0] != "" || parts[1] == "" || parts[2] == "" {
				partial = true
				continue
			}
			mapping := models.AddressMap{Type: fields[0], Domain: parts[1]}
			if mapping.Type == "cname" {
				mapping.CNAME = parts[2]
			} else {
				mapping.IP = parts[2]
			}
			entries = append(entries, parsedAddressDirective{mapping: mapping, line: i})
		}
	}
	return entries, partial
}

func removeAddressDirective(config string, address *models.AddressMap) string {
	want := addressDirective(address)
	lines := strings.SplitAfter(config, "\n")
	entries, _ := parseAddressDirectives(lines)
	for _, entry := range entries {
		if addressDirective(&entry.mapping) == want {
			lines[entry.line] = ""
		}
	}
	return strings.Join(lines, "")
}

type NodeAddressMapping struct {
	Domain string `json:"domain"`
	Type   string `json:"type"`
	Target string `json:"target"`
	Line   int    `json:"line,omitempty"`
	Status string `json:"status"`
}

func nodeAddressMapping(mapping models.AddressMap, line int, status string) NodeAddressMapping {
	kind, target := "address", mapping.IP
	if mapping.Type == "cname" {
		kind, target = "cname", mapping.CNAME
	}
	return NodeAddressMapping{Domain: mapping.Domain, Type: kind, Target: target, Line: line, Status: status}
}

func CompareNodeAddressMappings(config string, mappings []models.AddressMap, nodeID uint) ([]NodeAddressMapping, bool, error) {
	entries, partial := parseAddressDirectives(strings.Split(config, "\n"))
	known := map[string]bool{}
	assigned := map[string]models.AddressMap{}
	for _, mapping := range mappings {
		if !ordinaryAddressMapping(mapping) {
			continue
		}
		key := addressDirective(&mapping)
		known[key] = true
		applies, err := addressAssignedToNode(mapping, nodeID)
		if err != nil {
			return nil, false, err
		}
		if applies {
			assigned[key] = mapping
		}
	}
	result := make([]NodeAddressMapping, 0, len(entries))
	present := map[string]bool{}
	for _, entry := range entries {
		if !ordinaryAddressMapping(entry.mapping) {
			partial = true
			continue
		}
		key := addressDirective(&entry.mapping)
		present[key] = true
		status := "node_only"
		if _, ok := assigned[key]; ok {
			status = "matched"
		} else if known[key] {
			status = "unassigned"
		}
		result = append(result, nodeAddressMapping(entry.mapping, entry.line+1, status))
	}
	for _, mapping := range mappings {
		key := addressDirective(&mapping)
		if expected, ok := assigned[key]; ok && !present[key] {
			result = append(result, nodeAddressMapping(expected, 0, "pending"))
			present[key] = true
		}
	}
	return result, partial, nil
}

func addressAssignedToNode(mapping models.AddressMap, nodeID uint) (bool, error) {
	if !mapping.Enabled {
		return false, nil
	}
	if mapping.NodeIDs == "" || mapping.NodeIDs == "[]" {
		return true, nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(mapping.NodeIDs), &ids); err != nil {
		return false, fmt.Errorf("地址映射 %d 的节点范围无法读取: %w", mapping.ID, err)
	}
	return slices.Contains(ids, nodeID), nil
}

func NodeAddressConfigChecksum(config string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(config)))
}

// AdoptNodeAddressMapping 将节点上已有的指令登记到控制台，保留现有记录和其他节点的范围。
func AdoptNodeAddressMapping(db *gorm.DB, nodeID uint, config, checksum string, line int) (string, error) {
	mapping, err := nodeAddressAt(config, checksum, line)
	if err != nil {
		return "", err
	}
	return assignNodeAddressMapping(db, nodeID, mapping)
}

func nodeAddressAt(config, checksum string, line int) (models.AddressMap, error) {
	if NodeAddressConfigChecksum(config) != checksum {
		return models.AddressMap{}, ErrNodeAddressChanged
	}
	entries, _ := parseAddressDirectives(strings.Split(config, "\n"))
	for _, entry := range entries {
		if entry.line+1 == line && ordinaryAddressMapping(entry.mapping) {
			return entry.mapping, nil
		}
	}
	return models.AddressMap{}, ErrNodeAddressChanged
}

func assignNodeAddressMapping(db *gorm.DB, nodeID uint, mapping models.AddressMap) (string, error) {
	var action string
	err := db.Transaction(func(tx *gorm.DB) error {
		var existing []models.AddressMap
		if err := tx.Where("domain = ? COLLATE NOCASE", mapping.Domain).Order("id").Find(&existing).Error; err != nil {
			return err
		}
		var reusable *models.AddressMap
		var disabledMatch, conflict bool
		key := addressDirective(&mapping)
		for _, candidate := range existing {
			same := addressDirective(&candidate) == key
			if !candidate.Enabled {
				disabledMatch = disabledMatch || same
				continue
			}
			assigned, err := addressAssignedToNode(candidate, nodeID)
			if err != nil {
				return err
			}
			switch {
			case same && assigned:
				action = "already_assigned"
				return nil
			case same && reusable == nil:
				reusable = &candidate
			case !same && assigned:
				conflict = true
			}
		}
		if conflict {
			return ErrNodeAddressConflict
		}
		if reusable != nil {
			var ids []uint
			if err := json.Unmarshal([]byte(reusable.NodeIDs), &ids); err != nil {
				return err
			}
			newScope, err := json.Marshal(append(ids, nodeID))
			if err != nil {
				return err
			}
			action = "assigned"
			return tx.Model(reusable).Update("node_ids", string(newScope)).Error
		}
		if disabledMatch {
			return ErrNodeAddressDisabled
		}
		mapping.Enabled = true
		mapping.NodeIDs = fmt.Sprintf("[%d]", nodeID)
		action = "created"
		return tx.Create(&mapping).Error
	})
	return action, err
}

func planNodeAddressRemoval(config, checksum string, line int, mappings []models.AddressMap, nodeID uint) (string, error) {
	if NodeAddressConfigChecksum(config) != checksum {
		return "", ErrNodeAddressChanged
	}
	entries, _, err := CompareNodeAddressMappings(config, mappings, nodeID)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if line > 0 && entry.Line == line {
			if entry.Status == "matched" {
				return "", ErrNodeAddressAssigned
			}
			lines := strings.SplitAfter(config, "\n")
			lines[line-1] = ""
			return strings.Join(lines, ""), nil
		}
	}
	return "", ErrNodeAddressChanged
}

func RemoveNodeAddressMapping(node *models.Node, checksum string, line int, mappings []models.AddressMap) (string, error) {
	client, err := NewSSHClient(node)
	if err != nil {
		return "", err
	}
	defer client.Close()
	current, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		return "", err
	}
	updated, err := planNodeAddressRemoval(current, checksum, line, mappings, node.ID)
	if err != nil {
		return "", err
	}
	return writeNodeAddressConfig(client, node.ConfigPath, current, updated)
}

// 取消全部匹配的分配，避免重复登记的记录或「全部节点」在下次同步时加回映射。
func unassignNodeAddress(db *gorm.DB, nodeID uint, mapping models.AddressMap, write func() error) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var existing []models.AddressMap
		if err := tx.Where("domain = ? COLLATE NOCASE AND enabled = ?", mapping.Domain, true).Find(&existing).Error; err != nil {
			return err
		}
		changed := false
		for _, candidate := range existing {
			if addressDirective(&candidate) != addressDirective(&mapping) {
				continue
			}
			applies, err := addressAssignedToNode(candidate, nodeID)
			if err != nil {
				return err
			}
			if !applies {
				continue
			}
			changed = true
			var remaining []uint
			if candidate.NodeIDs == "" || candidate.NodeIDs == "[]" {
				if err := tx.Model(&models.Node{}).Where("id <> ?", nodeID).Order("id").Pluck("id", &remaining).Error; err != nil {
					return err
				}
			} else {
				if err := json.Unmarshal([]byte(candidate.NodeIDs), &remaining); err != nil {
					return err
				}
				remaining = slices.DeleteFunc(remaining, func(id uint) bool { return id == nodeID })
			}
			if len(remaining) == 0 {
				if err := tx.Delete(&candidate).Error; err != nil {
					return err
				}
			} else {
				scope, err := json.Marshal(remaining)
				if err != nil {
					return err
				}
				if err := tx.Model(&candidate).Update("node_ids", string(scope)).Error; err != nil {
					return err
				}
			}
		}
		if !changed {
			return ErrNodeAddressChanged
		}
		return write()
	})
}

func RemoveAssignedNodeAddressMapping(db *gorm.DB, node *models.Node, checksum string, line int) (string, error) {
	client, err := NewSSHClient(node)
	if err != nil {
		return "", err
	}
	defer client.Close()
	current, err := client.ReadFile(node.ConfigPath)
	if err != nil {
		return "", err
	}
	mapping, err := nodeAddressAt(current, checksum, line)
	if err != nil {
		return "", err
	}
	updated := removeAddressDirective(current, &mapping)
	var backup string
	var attempted bool
	err = unassignNodeAddress(db, node.ID, mapping, func() error {
		attempted = true
		var writeErr error
		backup, writeErr = writeNodeAddressConfig(client, node.ConfigPath, current, updated)
		return writeErr
	})
	if err != nil && attempted && !errors.Is(err, ErrNodeAddressChanged) {
		// 如果数据库未提交而节点已经写入，只在确认文件仍是本次版本时补偿。
		latest, readErr := client.ReadFile(node.ConfigPath)
		switch {
		case readErr != nil:
			return backup, errors.Join(err, fmt.Errorf("无法确认节点配置是否已写入: %w", readErr))
		case latest == updated:
			if restoreErr := client.WriteFile(node.ConfigPath, current); restoreErr != nil {
				return backup, errors.Join(err, fmt.Errorf("无法恢复节点原配置: %w", restoreErr))
			}
		case latest != current:
			return backup, errors.Join(err, fmt.Errorf("节点配置已被其他操作修改，请从备份核对"))
		}
	}
	return backup, err
}
