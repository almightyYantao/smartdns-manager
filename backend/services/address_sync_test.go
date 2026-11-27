package services

import (
	"errors"
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"smartdns-manager/models"
)

func TestUnassignNodeAddress(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/addresses.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Node{}, &models.AddressMap{}); err != nil {
		t.Fatal(err)
	}
	nodes := []models.Node{
		{ID: 1, Name: "当前节点", Host: "192.0.2.11"},
		{ID: 2, Name: "节点二", Host: "192.0.2.12"},
		{ID: 3, Name: "节点三", Host: "192.0.2.13"},
	}
	if err := db.Create(&nodes).Error; err != nil {
		t.Fatal(err)
	}
	mappings := []models.AddressMap{
		{Domain: "example.test", IP: "192.0.2.1", NodeIDs: "[]", Enabled: true},
		{Domain: "example.test", IP: "192.0.2.1", NodeIDs: "[1,3]", Enabled: true},
		{Domain: "example.test", IP: "192.0.2.1", NodeIDs: "[1]", Enabled: true},
		{Domain: "example.test", IP: "192.0.2.2", NodeIDs: "[1]", Enabled: true},
		{Domain: "example.test", IP: "192.0.2.1", NodeIDs: "[2]", Enabled: true},
	}
	if err := db.Create(&mappings).Error; err != nil {
		t.Fatal(err)
	}
	config := "address /example.test/192.0.2.1\naddress /example.test/192.0.2.2\n"
	mapping, err := nodeAddressAt(config, NodeAddressConfigChecksum(config), 1)
	if err != nil {
		t.Fatal(err)
	}
	writeErr := errors.New("节点写入失败")
	if err := unassignNodeAddress(db, 1, mapping, func() error { return writeErr }); !errors.Is(err, writeErr) {
		t.Fatal(err)
	}
	var unchanged []models.AddressMap
	if err := db.Order("id").Find(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if len(unchanged) != len(mappings) || unchanged[0].NodeIDs != "[]" || unchanged[1].NodeIDs != "[1,3]" {
		t.Fatalf("写入失败后保留原分配: %+v", unchanged)
	}

	if err := unassignNodeAddress(db, 1, mapping, func() error {
		config = removeAddressDirective(config, &mapping)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var updated []models.AddressMap
	if err := db.Order("id").Find(&updated).Error; err != nil {
		t.Fatal(err)
	}
	if len(updated) != 4 || updated[0].NodeIDs != "[2,3]" || updated[1].NodeIDs != "[3]" ||
		updated[2].IP != "192.0.2.2" || updated[2].NodeIDs != "[1]" || updated[3].NodeIDs != "[2]" ||
		config != "address /example.test/192.0.2.2\n" {
		t.Fatalf("移除后出现错误的节点分配或文件内容：%+v %q", updated, config)
	}
	if err := unassignNodeAddress(db, 1, mapping, func() error {
		t.Fatal("分配已取消，不能再次写节点配置")
		return nil
	}); !errors.Is(err, ErrNodeAddressChanged) {
		t.Fatal(err)
	}
}

func TestAdoptNodeAddressMapping(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/addresses.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.AddressMap{}); err != nil {
		t.Fatal(err)
	}
	original := []models.AddressMap{
		{Domain: "existing.test", IP: "192.0.2.1", NodeIDs: "[2,3]", Comment: "保留备注", Enabled: true},
		{Domain: "different.test", IP: "192.0.2.2", NodeIDs: "[2]", Enabled: true},
		{Domain: "conflict.test", IP: "192.0.2.2", NodeIDs: "[1]", Enabled: true},
		{Domain: "disabled.test", IP: "192.0.2.1", NodeIDs: "[]", Enabled: true},
	}
	if err := db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&original[3]).Update("enabled", false).Error; err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		config string
		action string
		err    error
	}{
		{"address /Existing.Test/192.0.2.1", "assigned", nil},
		{"address /existing.test/192.0.2.1", "already_assigned", nil},
		{"cname /new.test/alias.test", "created", nil},
		{"address /different.test/192.0.2.1", "created", nil},
		{"address /conflict.test/192.0.2.1", "", ErrNodeAddressConflict},
		{"address /disabled.test/192.0.2.1", "", ErrNodeAddressDisabled},
		{"address /policy.test/#", "", ErrNodeAddressChanged},
	} {
		action, err := AdoptNodeAddressMapping(db, 1, tc.config, NodeAddressConfigChecksum(tc.config), 1)
		if action != tc.action || !errors.Is(err, tc.err) {
			t.Errorf("%s: action=%q err=%v，预期 %q %v", tc.config, action, err, tc.action, tc.err)
		}
	}
	const change = "address /stale.test/192.0.2.1"
	if _, err := AdoptNodeAddressMapping(db, 1, change, NodeAddressConfigChecksum("原配置"), 1); !errors.Is(err, ErrNodeAddressChanged) {
		t.Fatal(err)
	}
	var all []models.AddressMap
	if err := db.Order("id").Find(&all).Error; err != nil {
		t.Fatal(err)
	}
	if len(all) != 6 || all[0].NodeIDs != "[2,3,1]" || all[0].Comment != "保留备注" ||
		all[1].NodeIDs != "[2]" || all[3].Enabled || all[4].NodeIDs != "[1]" || all[4].Type != "cname" ||
		all[5].NodeIDs != "[1]" {
		t.Fatalf("纳入操作改变了其他配置: %+v", all)
	}
}

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
	const config = "address /matched.test/192.0.2.1 # 节点原注释\n" +
		"address /other.test/192.0.2.2 # 此节点未分配\n" +
		"cname /local.test/local-name.test # 节点本地别名\n" +
		"address /domain-set:blocklist/#\n" +
		"address /blocked.test/#\n" +
		"conf-file /etc/smartdns/custom.conf\n"
	mappings := []models.AddressMap{
		{Domain: "matched.test", IP: "192.0.2.1", Enabled: true, NodeIDs: "[]", Comment: "全局用途"},
		{Domain: "other.test", IP: "192.0.2.2", Enabled: true, NodeIDs: "[2]"},
		{Domain: "pending.test", IP: "192.0.2.3", Enabled: true, NodeIDs: "[1]", Comment: "待下发用途"},
	}
	rows, partial, err := CompareNodeAddressMappings(config, mappings, 1)
	expected := []NodeAddressMapping{
		{Domain: "matched.test", Type: "address", Target: "192.0.2.1", Comment: "全局用途", Line: 1, Status: "matched"},
		{Domain: "other.test", Type: "address", Target: "192.0.2.2", Comment: "此节点未分配", Line: 2, Status: "unassigned"},
		{Domain: "local.test", Type: "cname", Target: "local-name.test", Comment: "节点本地别名", Line: 3, Status: "node_only"},
		{Domain: "pending.test", Type: "address", Target: "192.0.2.3", Comment: "待下发用途", Status: "pending"},
	}
	if err != nil || !partial || !reflect.DeepEqual(rows, expected) {
		t.Fatalf("比较返回 %+v，partial=%v，err=%v", rows, partial, err)
	}
	checksum := NodeAddressConfigChecksum(config)
	updated, err := planNodeAddressRemoval(config, checksum, 2, mappings, 1)
	want := "address /matched.test/192.0.2.1 # 节点原注释\n" +
		"cname /local.test/local-name.test # 节点本地别名\n" +
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
