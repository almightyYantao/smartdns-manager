package utils

import (
	"bufio"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"time"

	"smartdns-log-agent/models"
)

// StaticAddressGroup 只表示采集时匹配静态映射，不表示已证明响应来自本地。
const StaticAddressGroup = "__static_match__"

type staticAddressRule struct {
	domain string
	ips    map[netip.Addr]bool
}

// StaticAddressTagger 读取本节点配置，仅标记启用时间之后的 A/AAAA 日志。
// 复杂域名规则不做猜测，配置读取失败时保留原日志分组。
type StaticAddressTagger struct {
	path      string
	from      time.Time
	checkedAt time.Time
	modified  time.Time
	rules     []staticAddressRule
}

func NewStaticAddressTagger(path string, from time.Time) *StaticAddressTagger {
	return &StaticAddressTagger{path: path, from: from}
}

// Refresh 每秒至多读取一次配置，避免每条查询都访问磁盘。
func (t *StaticAddressTagger) Refresh(now time.Time) error {
	if !t.checkedAt.IsZero() && now.Sub(t.checkedAt) < time.Second {
		return nil
	}
	t.checkedAt = now
	t.rules = nil
	file, err := os.Open(t.path)
	if err != nil {
		return err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		return err
	}
	if before.Size() > 4*1024*1024 {
		return fmt.Errorf("静态映射配置文件过大")
	}
	rules, err := parseStaticAddresses(file)
	if err != nil {
		return err
	}
	after, err := file.Stat()
	if err != nil {
		return err
	}
	if before.ModTime() != after.ModTime() || before.Size() != after.Size() {
		return fmt.Errorf("静态映射配置正在更新")
	}
	t.rules = rules
	t.modified = after.ModTime()
	return nil
}

func parseStaticAddresses(file *os.File) ([]staticAddressRule, error) {
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var rules []staticAddressRule
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		// 这些配置可能覆盖或改变地址规则作用范围，暂不推断。
		if fields[0] == "conf-file" || fields[0] == "group-begin" || fields[0] == "group-match" ||
			strings.Contains(line, "-no-rule-addr") {
			return nil, fmt.Errorf("配置包含未支持的地址规则作用范围")
		}
		if fields[0] != "address" || len(fields) < 2 {
			continue
		}
		parts := strings.Split(fields[1], "/")
		if len(parts) != 3 || parts[0] != "" {
			continue
		}
		domain := strings.ToLower(strings.TrimSuffix(parts[1], "."))
		if !isPlainDomain(domain) {
			continue
		}
		rule := staticAddressRule{domain: domain, ips: map[netip.Addr]bool{}}
		for _, value := range strings.Split(parts[2], ",") {
			if ip, err := netip.ParseAddr(value); err == nil {
				rule.ips[ip] = true
			}
		}
		// 空映射和特殊指令也保留，防止误用更宽泛的父域映射。
		rules = append(rules, rule)
	}
	return rules, scanner.Err()
}

func isPlainDomain(domain string) bool {
	if domain == "" || strings.HasPrefix(domain, "-") {
		return false
	}
	for _, ch := range domain {
		if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.') {
			return false
		}
	}
	return true
}

// Tag 仅修改结构化记录的 Group，RawLog、磁盘日志和历史数据库记录均不变。
func (t *StaticAddressTagger) Tag(record *models.DNSLogRecord) bool {
	if record.Timestamp.Before(t.from) || record.Timestamp.Before(t.modified) ||
		(record.QueryType != 1 && record.QueryType != 28) || len(record.ResultIPs) == 0 {
		return false
	}
	domain := strings.ToLower(strings.TrimSuffix(record.Domain, "."))
	best := -1
	ips := map[netip.Addr]bool{}
	for _, rule := range t.rules {
		if domain != rule.domain && !strings.HasSuffix(domain, "."+rule.domain) {
			continue
		}
		if len(rule.domain) > best {
			best = len(rule.domain)
			ips = map[netip.Addr]bool{}
		}
		if len(rule.domain) == best {
			for ip := range rule.ips {
				ips[ip] = true
			}
		}
	}
	for _, result := range record.ResultIPs {
		ip, err := netip.ParseAddr(strings.TrimSpace(result))
		if err != nil || !ips[ip] || (record.QueryType == 1 && !ip.Is4()) || (record.QueryType == 28 && !ip.Is6()) {
			return false
		}
	}
	record.Group = StaticAddressGroup
	return true
}
