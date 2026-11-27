package services

import (
	"encoding/json"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"smartdns-manager/models"
)

var taskScheduleParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

const defaultDNSProbeTask = "DNS 自动拨测"

func (s *SchedulerService) migrateDNSProbeSchedule() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 已删除的迁移任务也保留标记，避免重启时重新启用管理员关掉的任务。
		var count int64
		if err := tx.Unscoped().Model(&models.ScheduledTask{}).Where("type = ? AND (description = ? OR name = ?)", models.TaskTypeTelemetry, "由内置 DNS 拨测迁移", defaultDNSProbeTask).Count(&count).Error; err != nil || count > 0 {
			return err
		}
		var ids []uint
		if err := tx.Model(&models.TelemetryTarget{}).Where("type = ?", "dns").Pluck("id", &ids).Error; err != nil || len(ids) == 0 {
			return err
		}
		data, err := json.Marshal(models.TelemetryConfig{Targets: ids})
		if err != nil {
			return err
		}
		return tx.Create(&models.ScheduledTask{Name: defaultDNSProbeTask, Type: models.TaskTypeTelemetry, Description: "由内置 DNS 拨测迁移", CronExpr: "*/30 * * * * *", Config: string(data), Enabled: true}).Error
	})
}

type TelemetrySchedule struct {
	ID        uint       `json:"id"`
	Name      string     `json:"name"`
	Enabled   bool       `json:"enabled"`
	NextRunAt *time.Time `json:"next_run_at"`
}

func (s *SchedulerService) TelemetrySchedules(targets []models.TelemetryTarget) (map[uint][]TelemetrySchedule, error) {
	var tasks []models.ScheduledTask
	if err := s.db.Select("id", "name", "enabled", "config", "next_run_at").Where("type = ?", models.TaskTypeTelemetry).Find(&tasks).Error; err != nil {
		return nil, err
	}
	result := make(map[uint][]TelemetrySchedule)
	for _, task := range tasks {
		var config models.TelemetryConfig
		if json.Unmarshal([]byte(task.Config), &config) != nil {
			continue
		}
		for _, target := range targets {
			match := len(config.Targets) == 0 && (target.Type != "dns" || config.IncludeDNS)
			for _, id := range config.Targets {
				if id == target.ID {
					match = true
					break
				}
			}
			if match {
				result[target.ID] = append(result[target.ID], TelemetrySchedule{task.ID, task.Name, task.Enabled, task.NextRunAt})
			}
		}
	}
	return result, nil
}
