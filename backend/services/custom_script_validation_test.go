package services

import (
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
