package services

import (
	"fmt"
	"regexp"
	"smartdns-manager/models"
	"strings"
	"time"
)

type ConfigParser struct{}

func NewConfigParser() *ConfigParser {
	return &ConfigParser{}
}

func (p *ConfigParser) Parse(content string) (*models.SmartDNSConfig, error) {
	config := &models.SmartDNSConfig{
		Servers:       []models.DNSServer{},
		Addresses:     []models.AddressMap{},
		DomainSets:    []models.DomainSet{},
		DomainRules:   []models.DomainRule{},
		Nameservers:   []models.Nameserver{},
		BasicSettings: make(map[string]string),
	}

	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行和注释
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 server
		if strings.HasPrefix(line, "server ") {
			server := p.parseServer(line)
			if server != nil {
				config.Servers = append(config.Servers, *server)
			}
		}

		// 解析 address
		if strings.HasPrefix(line, "address /") {
			address := p.parseAddress(line)
			if address != nil {
				config.Addresses = append(config.Addresses, *address)
			}
		}

		// 解析 domain-set
		if strings.HasPrefix(line, "domain-set ") {
			domainSet := p.parseDomainSet(line)
			if domainSet != nil {
				config.DomainSets = append(config.DomainSets, *domainSet)
			}
		}

		// 解析 domain-rules
		if strings.HasPrefix(line, "domain-rules /") {
			rule := p.parseDomainRule(line)
			if rule != nil {
				config.DomainRules = append(config.DomainRules, *rule)
			}
		}

		// 解析 nameserver
		if strings.HasPrefix(line, "nameserver /") {
			ns := p.parseNameserver(line)
			if ns != nil {
				config.Nameservers = append(config.Nameservers, *ns)
			}
		}

		// 解析基础设置
		p.parseBasicSetting(line, config.BasicSettings)
	}

	return config, nil
}

func (p *ConfigParser) parseServer(line string) *models.DNSServer {
	re := regexp.MustCompile(`server\s+(\S+)(?:\s+(.*))?`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 2 {
		return nil
	}

	server := &models.DNSServer{
		Address: matches[1],
		Options: "",
	}

	if len(matches) > 2 {
		options := matches[2]
		server.Options = options

		// 提取 groups
		groupRe := regexp.MustCompile(`-group\s+(\S+)`)
		groupMatches := groupRe.FindAllStringSubmatch(options, -1)
		for _, m := range groupMatches {
			server.Groups = append(server.Groups, m[1])
		}

		// 检查 exclude-default-group
		server.ExcludeDefault = strings.Contains(options, "-exclude-default-group")
	}

	// 确定类型
	if strings.HasPrefix(server.Address, "https://") {
		server.Type = "https"
	} else if strings.HasPrefix(server.Address, "tls://") {
		server.Type = "tls"
	} else {
		server.Type = "udp"
	}

	return server
}

func (p *ConfigParser) parseAddress(line string) *models.AddressMap {
	re := regexp.MustCompile(`address\s+/(.*?)/(.*)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) != 3 {
		return nil
	}

	return &models.AddressMap{
		Domain: matches[1],
		IP:     matches[2],
	}
}

func (p *ConfigParser) parseDomainSet(line string) *models.DomainSet {
	nameRe := regexp.MustCompile(`-name\s+(\S+)`)
	fileRe := regexp.MustCompile(`-file\s+(\S+)`)

	nameMatch := nameRe.FindStringSubmatch(line)
	fileMatch := fileRe.FindStringSubmatch(line)

	if len(nameMatch) < 2 {
		return nil
	}

	ds := &models.DomainSet{
		Name: nameMatch[1],
	}

	if len(fileMatch) > 1 {
		ds.FilePath = fileMatch[1]
	}

	return ds
}

func (p *ConfigParser) parseDomainRule(line string) *models.DomainRule {
	// 匹配 domain-rules /domain/ options 或 domain-rules /domain-set:name/ options
	re := regexp.MustCompile(`domain-rules\s+/(.*?)/\s*(.*)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 3 {
		return nil
	}

	domain := strings.TrimSpace(matches[1])
	options := strings.TrimSpace(matches[2])

	rule := &models.DomainRule{
		Enabled: true,
	}

	// 检查是否是域名集引用
	if strings.HasPrefix(domain, "domain-set:") {
		rule.IsDomainSet = true
		rule.DomainSetName = strings.TrimPrefix(domain, "domain-set:")
		rule.Domain = domain // 保存完整的引用
	} else {
		rule.IsDomainSet = false
		rule.Domain = domain
	}

	// 解析选项
	if options != "" {
		// 提取 -address
		addressRe := regexp.MustCompile(`-address\s+(\S+)`)
		if addressMatch := addressRe.FindStringSubmatch(options); len(addressMatch) > 1 {
			rule.Address = addressMatch[1]
		}

		// 提取 -nameserver
		nameserverRe := regexp.MustCompile(`-nameserver\s+(\S+)`)
		if nsMatch := nameserverRe.FindStringSubmatch(options); len(nsMatch) > 1 {
			rule.Nameserver = nsMatch[1]
		}

		// 提取 -speed-check-mode
		speedRe := regexp.MustCompile(`-speed-check-mode\s+(\S+)`)
		if speedMatch := speedRe.FindStringSubmatch(options); len(speedMatch) > 1 {
			rule.SpeedCheckMode = speedMatch[1]
		}

		// 保存其他选项
		rule.OtherOptions = options
	}

	return rule
}

func (p *ConfigParser) parseNameserver(line string) *models.Nameserver {
	// 匹配 nameserver /domain/group 或 nameserver /domain-set:name/group
	re := regexp.MustCompile(`nameserver\s+/(.*?)/(.*)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) != 3 {
		return nil
	}

	domain := strings.TrimSpace(matches[1])
	group := strings.TrimSpace(matches[2])

	ns := &models.Nameserver{
		Group:   group,
		Enabled: true,
	}

	// 检查是否是域名集引用
	if strings.HasPrefix(domain, "domain-set:") {
		ns.IsDomainSet = true
		ns.DomainSetName = strings.TrimPrefix(domain, "domain-set:")
		ns.Domain = domain // 保存完整的引用
	} else {
		ns.IsDomainSet = false
		ns.Domain = domain
	}

	return ns
}

func (p *ConfigParser) parseBasicSetting(line string, settings map[string]string) {
	basicKeys := []string{
		"bind", "cache-size", "prefetch-domain", "serve-expired",
		"force-AAAA-SOA", "dualstack-ip-selection", "rr-ttl-min",
		"rr-ttl-max", "log-level", "log-file", "log-size",
		"audit-enable", "audit-num", "audit-size", "audit-file",
		"speed-check-mode", "expand-ptr-from-address",
	}

	for _, key := range basicKeys {
		if strings.HasPrefix(line, key+" ") || strings.HasPrefix(line, key+":") {
			value := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, key), ":"))
			settings[key] = strings.TrimSpace(value)
			break
		}
	}
}

func (p *ConfigParser) Generate(config *models.SmartDNSConfig) string {
	var builder strings.Builder
	builder.WriteString("# SmartDNS Configuration\n")
	builder.WriteString("# Auto-generated by SmartDNS Manager\n")
	builder.WriteString(fmt.Sprintf("# Generated at: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// 基础设置
	if len(config.BasicSettings) > 0 {
		builder.WriteString("# Basic Settings\n")
		// 按顺序输出重要设置
		orderedKeys := []string{
			"bind", "cache-size", "prefetch-domain", "serve-expired",
			"rr-ttl-min", "rr-ttl-max", "log-level", "log-file", "log-size",
			"audit-enable", "audit-num", "audit-size", "audit-file",
			"force-AAAA-SOA", "dualstack-ip-selection", "speed-check-mode",
			"expand-ptr-from-address",
		}
		for _, key := range orderedKeys {
			if value, ok := config.BasicSettings[key]; ok {
				builder.WriteString(fmt.Sprintf("%s %s\n", key, value))
			}
		}
		// 输出其他设置
		for key, value := range config.BasicSettings {
			found := false
			for _, orderedKey := range orderedKeys {
				if key == orderedKey {
					found = true
					break
				}
			}
			if !found {
				builder.WriteString(fmt.Sprintf("%s %s\n", key, value))
			}
		}
		builder.WriteString("\n")
	}

	// DNS 服务器
	if len(config.Servers) > 0 {
		builder.WriteString("# DNS Servers\n")
		for _, server := range config.Servers {
			builder.WriteString(fmt.Sprintf("server %s", server.Address))
			if server.Options != "" {
				builder.WriteString(fmt.Sprintf(" %s", server.Options))
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	// 地址映射和CNAME
	if len(config.Addresses) > 0 {
		builder.WriteString("# Address Mappings\n")
		for _, addr := range config.Addresses {
			if addr.Type == "cname" {
				// CNAME 格式
				builder.WriteString(fmt.Sprintf("cname /%s/%s", addr.Domain, addr.CNAME))
			} else {
				// Address 格式
				builder.WriteString(fmt.Sprintf("address /%s/%s", addr.Domain, addr.IP))
			}
			// 添加注释
			if addr.Comment != "" {
				builder.WriteString(fmt.Sprintf(" # %s", addr.Comment))
			}
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	// Domain Sets
	if len(config.DomainSets) > 0 {
		builder.WriteString("# Domain Sets\n")
		for _, ds := range config.DomainSets {
			builder.WriteString(fmt.Sprintf("domain-set -name %s -file %s\n", ds.Name, ds.FilePath))
		}
		builder.WriteString("\n")
	}

	// Domain Rules
	if len(config.DomainRules) > 0 {
		builder.WriteString("# Domain Rules\n")
		for _, rule := range config.DomainRules {
			var domain string
			if rule.IsDomainSet {
				domain = fmt.Sprintf("domain-set:%s", rule.DomainSetName)
			} else {
				domain = rule.Domain
			}
			line := fmt.Sprintf("domain-rules /%s/", domain)

			// 构建选项
			var opts []string
			addOptIfNotExists := func(newOpt string) {
				for _, existing := range opts {
					if existing == newOpt {
						return
					}
				}
				opts = append(opts, newOpt)
			}

			if rule.Address != "" {
				addOptIfNotExists(fmt.Sprintf("-address %s", rule.Address))
			}
			if rule.Nameserver != "" {
				addOptIfNotExists(fmt.Sprintf("-nameserver %s", rule.Nameserver))
			}
			if rule.SpeedCheckMode != "" {
				opts = append(opts, fmt.Sprintf("-speed-check-mode %s", rule.SpeedCheckMode))
			}
			if rule.OtherOptions != "" {
				opts = append(opts, rule.OtherOptions)
			}

			if len(opts) > 0 {
				line += " " + strings.Join(opts, " ")
			}
			builder.WriteString(line + "\n")
		}
		builder.WriteString("\n")
	}

	// Nameserver Rules
	if len(config.Nameservers) > 0 {
		builder.WriteString("# Nameserver Rules\n")
		for _, ns := range config.Nameservers {
			var domain string
			if ns.IsDomainSet {
				domain = fmt.Sprintf("domain-set:%s", ns.DomainSetName)
			} else {
				domain = ns.Domain
			}
			builder.WriteString(fmt.Sprintf("nameserver /%s/%s\n", domain, ns.Group))
		}
		builder.WriteString("\n")
	}

	return builder.String()
}
