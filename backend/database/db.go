package database

import (
	"log"

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

	// 使用配置中的数据库路径，而不是硬编码
	dbPath := config.GetConfig().DBPath

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

	log.Printf("Database initialized successfully at: %s", dbPath)
}
