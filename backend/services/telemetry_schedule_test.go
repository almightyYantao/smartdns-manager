package services

import (
	"encoding/json"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"smartdns-manager/models"
)

func TestMigrateDNSProbeSchedule(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ScheduledTask{}, &models.TelemetryTarget{}); err != nil {
		t.Fatal(err)
	}
	targets := []models.TelemetryTarget{{Type: "dns", Name: "dns", Target: "192.0.2.1", DNSStreak: 2}, {Type: "https", Name: "https", Target: "example.com"}}
	if err := db.Create(&targets).Error; err != nil {
		t.Fatal(err)
	}
	s := &SchedulerService{db: db}
	for range 2 {
		if err := s.migrateDNSProbeSchedule(); err != nil {
			t.Fatal(err)
		}
	}
	var tasks []models.ScheduledTask
	if err := db.Find(&tasks).Error; err != nil || len(tasks) != 1 {
		t.Fatalf("迁移结果 %+v: %v", tasks, err)
	}
	var cfg models.TelemetryConfig
	if err := json.Unmarshal([]byte(tasks[0].Config), &cfg); err != nil || len(cfg.Targets) != 1 || cfg.Targets[0] != targets[0].ID || tasks[0].CronExpr != "*/30 * * * * *" {
		t.Fatalf("迁移配置 %+v: %v", tasks[0], err)
	}
	schedules, err := s.TelemetrySchedules(targets)
	if err != nil || len(schedules[targets[0].ID]) != 1 || len(schedules[targets[1].ID]) != 0 {
		t.Fatalf("任务关联 %+v: %v", schedules, err)
	}
	if err := db.First(&targets[0], targets[0].ID).Error; err != nil || targets[0].DNSStreak != 2 {
		t.Fatalf("告警计数 %+v: %v", targets[0], err)
	}
	if err := db.Delete(&tasks[0]).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.migrateDNSProbeSchedule(); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&models.ScheduledTask{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("任务被重新创建: %d %v", count, err)
	}
}
