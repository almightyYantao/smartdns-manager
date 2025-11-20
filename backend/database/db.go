package database

import (
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"smartdns-manager/config"
	"smartdns-manager/models"
)

var DB *gorm.DB

// InitDB 初始化数据库
func InitDB() {
	var err error

	// 获取数据库路径
	dbPath := config.GetConfig().DBPath

	// 确保数据库目录存在
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatal("Failed to create database directory:", err)
	}

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 自动迁移数据库结构
	err = DB.AutoMigrate(
		&models.User{},
		&models.Node{},
		&models.DNSServer{},
		&models.AddressMap{},
		&models.DomainSet{},
		&models.DNSGroup{},
		&models.DomainSetItem{},
		&models.DomainRule{},
		&models.Nameserver{},
		&models.ConfigSyncLog{},
		&models.NotificationChannel{},
		&models.NotificationLog{},
		&models.InitLog{},
		&models.Backup{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// 为现有记录添加默认值
	DB.Model(&models.AddressMap{}).Where("node_ids IS NULL").Update("node_ids", "[]")
	DB.Model(&models.AddressMap{}).Where("enabled IS NULL").Update("enabled", true)
	DB.Model(&models.DNSServer{}).Where("node_ids IS NULL").Update("node_ids", "[]")
	DB.Model(&models.DNSServer{}).Where("enabled IS NULL").Update("enabled", true)

	log.Printf("Database initialized successfully at: %s", dbPath)

	// 初始化系统分组
	initSystemGroups()
}

func initSystemGroups() {
	systemGroups := []models.DNSGroup{
		{Name: "cn", Description: "国内DNS", Color: "blue", IsSystem: true},
		{Name: "oversea", Description: "国外DNS", Color: "green", IsSystem: true},
		{Name: "local", Description: "本地DNS", Color: "orange", IsSystem: true},
		{Name: "ad", Description: "广告过滤", Color: "red", IsSystem: true},
		{Name: "bootstrap", Description: "启动DNS", Color: "purple", IsSystem: true},
	}

	for _, group := range systemGroups {
		var existing models.DNSGroup
		if err := DB.Where("name = ?", group.Name).First(&existing).Error; err != nil {
			// 不存在则创建
			DB.Create(&group)
			log.Printf("Created system group: %s", group.Name)
		}
	}
}
