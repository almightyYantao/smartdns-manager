package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"smartdns-manager/database"
	"smartdns-manager/models"
)

func TestUpdateAddressRetainsScopeWhenRemovalFails(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/address.db"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Node{}, &models.AddressMap{}, &models.ConfigSyncLog{}); err != nil {
		t.Fatal(err)
	}
	priorDB := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = priorDB })
	if err := db.Create(&models.Node{ID: 1, Name: "unreachable", Host: "127.0.0.1", Port: 1}).Error; err != nil {
		t.Fatal(err)
	}
	address := models.AddressMap{ID: 1, Domain: "example.test", Type: "address", IP: "192.0.2.1", NodeIDs: "[1,2]", Enabled: true}
	if err := db.Create(&address).Error; err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest("PUT", "/addresses/1", strings.NewReader(`{"node_ids":"[2]","enabled":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateAddress(c)
	if response.Code != 502 {
		t.Fatalf("无法清除节点配置时应返回 502，实际 %d", response.Code)
	}
	if err := db.First(&address, 1).Error; err != nil || address.NodeIDs != "[1,2]" {
		t.Fatalf("保存失败后应保留原范围，实际 %q，错误 %v", address.NodeIDs, err)
	}
}
