package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"smartdns-log-agent/models"
)

func testTagger(t *testing.T, content string) (*StaticAddressTagger, string, time.Time) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "smartdns.conf")
	now := time.Date(2026, 10, 26, 10, 0, 0, 0, time.Local)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now.Add(-time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	tagger := NewStaticAddressTagger(path, now.Add(-time.Minute))
	if err := tagger.Refresh(now); err != nil {
		t.Fatal(err)
	}
	return tagger, path, now
}

func TestStaticAddressNewRecordsOnly(t *testing.T) {
	tagger, _, now := testTagger(t, "address /api.example.org/192.0.2.10\n")
	raw := "[" + now.Format("2006-01-02 15:04:05,000") + "] 192.0.2.1 query api.example.org, type 1, time 0ms, speed: -0.1ms, group oversea, result 192.0.2.10"
	record := NewLogParser().Parse(raw, 1)
	if record == nil || !tagger.Tag(record) || record.Group != StaticAddressGroup {
		t.Fatalf("未标记新静态映射记录: %+v", record)
	}
	if record.RawLog != raw || !strings.Contains(record.RawLog, "group oversea") {
		t.Fatal("原始日志被修改")
	}
	old := *record
	old.Group = "oversea"
	old.Timestamp = now.Add(-time.Hour)
	if tagger.Tag(&old) || old.Group != "oversea" {
		t.Fatal("修改了启用前的历史日志")
	}
}

func TestStaticAddressMatchBoundaries(t *testing.T) {
	tagger, _, now := testTagger(t, "address /api.example.org/192.0.2.10,192.0.2.11\naddress /api.example.org/2001:db8::1\naddress /blocked.api.example.org/-\n")
	cases := []struct {
		name, domain string
		qtype        uint16
		ips          []string
		want         bool
	}{
		{"A", "api.example.org", 1, []string{"192.0.2.10"}, true},
		{"多个结果", "api.example.org", 1, []string{"192.0.2.10", "192.0.2.11"}, true},
		{"IPv6规范化", "api.example.org", 28, []string{"2001:0db8::1"}, true},
		{"子域和尾点", "SUB.API.EXAMPLE.ORG.", 1, []string{"192.0.2.10"}, true},
		{"HTTPS不标记", "api.example.org", 65, []string{"192.0.2.10"}, false},
		{"AAAA不使用IPv4", "api.example.org", 28, []string{"192.0.2.10"}, false},
		{"空结果", "api.example.org", 1, nil, false},
		{"SOA", "api.example.org", 1, []string{"soa"}, false},
		{"不同IP", "api.example.org", 1, []string{"192.0.2.20"}, false},
		{"部分不同", "api.example.org", 1, []string{"192.0.2.10", "192.0.2.20"}, false},
		{"后缀边界", "notapi.example.org", 1, []string{"192.0.2.10"}, false},
		{"更具体的忽略规则", "blocked.api.example.org", 1, []string{"192.0.2.10"}, false},
		{"其他节点无映射", "other.example.org", 1, []string{"192.0.2.10"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := models.DNSLogRecord{Domain: tc.domain, QueryType: tc.qtype, ResultIPs: tc.ips, Timestamp: now, Group: "oversea"}
			if got := tagger.Tag(&r); got != tc.want {
				t.Fatalf("Tag=%v, want=%v", got, tc.want)
			}
			if !tc.want && r.Group != "oversea" {
				t.Fatal("未匹配的记录被修改")
			}
		})
	}
}

func TestStaticAddressMissingConfigAndReload(t *testing.T) {
	tagger, path, now := testTagger(t, "address /example.org/192.0.2.10\n")
	r := models.DNSLogRecord{Domain: "example.org", QueryType: 1, ResultIPs: []string{"192.0.2.10"}, Timestamp: now, Group: "cn"}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := tagger.Refresh(now.Add(2 * time.Second)); err == nil {
		t.Fatal("预期配置读取失败")
	}
	if tagger.Tag(&r) {
		t.Fatal("配置不可读后继续使用了旧规则")
	}
	if err := os.WriteFile(path, []byte("address /example.org/192.0.2.20\n"), 0600); err != nil {
		t.Fatal(err)
	}
	changed := now.Add(3 * time.Second)
	if err := os.Chtimes(path, changed, changed); err != nil {
		t.Fatal(err)
	}
	if err := tagger.Refresh(now.Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	r.ResultIPs = []string{"192.0.2.20"}
	if tagger.Tag(&r) {
		t.Fatal("用晚于查询的配置解释了旧记录")
	}
	r.Timestamp = changed.Add(time.Second)
	if !tagger.Tag(&r) {
		t.Fatal("未加载更新后的配置")
	}
}

func TestStaticAddressUnsupportedScope(t *testing.T) {
	for _, directive := range []string{"conf-file other.conf", "group-begin other", "group-match -client-ip 192.0.2.0/24", "bind :53 -no-rule-addr"} {
		t.Run(directive, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "smartdns.conf")
			if err := os.WriteFile(path, []byte("address /example.org/192.0.2.10\n"+directive), 0600); err != nil {
				t.Fatal(err)
			}
			tagger := NewStaticAddressTagger(path, time.Now())
			if err := tagger.Refresh(time.Now()); err == nil || len(tagger.rules) != 0 {
				t.Fatal("复杂作用范围下仍进行推断")
			}
		})
	}
}
