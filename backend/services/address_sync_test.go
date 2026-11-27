package services

import (
	"errors"
	"reflect"
	"testing"

	"smartdns-manager/models"
)

func TestRemoveAddressDirective(t *testing.T) {
	const shared = "# 另一条映射及同域名的其他指令保持原样\r\n" +
		"address /example.test/192.0.2.2 # 第二个地址\r\n" +
		"cname /example.test/another.test\r\n" +
		"group-begin test\r\n" +
		"  address /example.test/192.0.2.1\r\n" +
		"group-end # 保持规则组\r\n" +
		"# address /example.test/192.0.2.1\r\n" +
		"server 192.0.2.53"
	address := models.AddressMap{Type: "address", Domain: "example.test", IP: "192.0.2.1"}
	input := "  address\t/Example.Test/192.0.2.1 # 已下发\r\n" + shared
	if got := removeAddressDirective(input, &address); got != shared {
		t.Fatalf("撤回 IP 映射后配置为 %q，预期 %q", got, shared)
	}
	if got := removeAddressDirective(shared, &address); got != shared {
		t.Fatalf("重复撤回改变了配置：%q", got)
	}
	cname := models.AddressMap{Type: "cname", Domain: "example.test", CNAME: "alias.test"}
	input = "cname /example.test/alias.test\n" + shared
	if got := removeAddressDirective(input, &cname); got != shared {
		t.Fatalf("撤回 CNAME 后配置为 %q，预期 %q", got, shared)
	}
}

func TestNodeAddressMappingComparisonAndRemoval(t *testing.T) {
	const config = "address /matched.test/192.0.2.1\n" +
		"address /other.test/192.0.2.2 # 此节点未分配\n" +
		"cname /local.test/local-name.test\n" +
		"address /domain-set:blocklist/#\n" +
		"address /blocked.test/#\n" +
		"conf-file /etc/smartdns/custom.conf\n"
	mappings := []models.AddressMap{
		{Domain: "matched.test", IP: "192.0.2.1", Enabled: true, NodeIDs: "[]"},
		{Domain: "other.test", IP: "192.0.2.2", Enabled: true, NodeIDs: "[2]"},
		{Domain: "pending.test", IP: "192.0.2.3", Enabled: true, NodeIDs: "[1]"},
	}
	rows, partial, err := CompareNodeAddressMappings(config, mappings, 1)
	expected := []NodeAddressMapping{
		{Domain: "matched.test", Type: "address", Target: "192.0.2.1", Line: 1, Status: "matched"},
		{Domain: "other.test", Type: "address", Target: "192.0.2.2", Line: 2, Status: "unassigned"},
		{Domain: "local.test", Type: "cname", Target: "local-name.test", Line: 3, Status: "node_only"},
		{Domain: "pending.test", Type: "address", Target: "192.0.2.3", Status: "pending"},
	}
	if err != nil || !partial || !reflect.DeepEqual(rows, expected) {
		t.Fatalf("比较返回 %+v，partial=%v，err=%v", rows, partial, err)
	}
	checksum := NodeAddressConfigChecksum(config)
	updated, err := planNodeAddressRemoval(config, checksum, 2, mappings, 1)
	want := "address /matched.test/192.0.2.1\n" +
		"cname /local.test/local-name.test\n" +
		"address /domain-set:blocklist/#\n" +
		"address /blocked.test/#\n" +
		"conf-file /etc/smartdns/custom.conf\n"
	if err != nil || updated != want {
		t.Fatalf("删除修改了其他行：%q，err=%v", updated, err)
	}
	for _, test := range []struct {
		config string
		line   int
		nodeID uint
		err    error
	}{
		{config, 1, 1, ErrNodeAddressAssigned},
		{config, 2, 2, ErrNodeAddressAssigned},
		{config + "# 已变化\n", 2, 1, ErrNodeAddressChanged},
		{config, 4, 1, ErrNodeAddressChanged},
		{config, 0, 1, ErrNodeAddressChanged},
	} {
		if _, err := planNodeAddressRemoval(test.config, checksum, test.line, mappings, test.nodeID); !errors.Is(err, test.err) {
			t.Fatalf("节点 %d 第 %d 行错误为 %v，预期 %v", test.nodeID, test.line, err, test.err)
		}
	}
}
