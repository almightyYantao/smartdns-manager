package services

import (
	"context"
	"strings"
	"testing"

	"smartdns-manager/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCustomScriptValidatesExistingNodeWithoutEnabledColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Node{}); err != nil {
		t.Fatal(err)
	}
	node := models.Node{Name: "test", Host: "192.0.2.1", Username: "test"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	service, err := NewCustomScriptService(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ValidateScript(models.CustomScriptConfig{NodeIDs: []uint{node.ID}, Script: "true"}); err != nil {
		t.Fatalf("存在的节点应通过校验: %v", err)
	}
	if err := service.ValidateScript(models.CustomScriptConfig{NodeIDs: []uint{node.ID + 1}, Script: "true"}); err == nil {
		t.Fatal("不存在的节点应被拒绝")
	}
}

func TestCustomScriptExecutesLocallyWithoutDatabase(t *testing.T) {
	service, err := NewCustomScriptService(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	config := models.CustomScriptConfig{ExecutionMode: "local", Script: "printf local-ok", Timeout: 1}
	if err := service.ValidateScript(config); err != nil {
		t.Fatal(err)
	}
	output, err := service.ExecuteScript(context.Background(), config)
	if err != nil || !strings.Contains(output, "local-ok") {
		t.Fatalf("本机任务未正确执行: %q %v", output, err)
	}
	config.NodeIDs = []uint{1}
	if err := service.ValidateScript(config); err == nil {
		t.Fatal("本机任务不应接受远程节点")
	}
	if _, err := service.ExecuteScript(context.Background(), config); err == nil {
		t.Fatal("执行入口未拒绝混合模式")
	}
}
