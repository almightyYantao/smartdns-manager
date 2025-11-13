package database

import (
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"smartdns-manager/models"
)

var DB *gorm.DB

// InitDB 初始化数据库
func InitDB() {
	var err error
	DB, err = gorm.Open(sqlite.Open("smartdns.db"), &gorm.Config{
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
		&models.DomainRule{},
		&models.Nameserver{},
		&models.ConfigSyncLog{},
		&models.NotificationChannel{},
		&models.NotificationLog{},
		&models.InitLog{},
	)

	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	// 为现有记录添加默认值
	DB.Model(&models.AddressMap{}).Where("node_ids IS NULL").Update("node_ids", "[]")
	DB.Model(&models.AddressMap{}).Where("enabled IS NULL").Update("enabled", true)
	DB.Model(&models.DNSServer{}).Where("node_ids IS NULL").Update("node_ids", "[]")
	DB.Model(&models.DNSServer{}).Where("enabled IS NULL").Update("enabled", true)

	log.Println("Database initialized successfully")
}
