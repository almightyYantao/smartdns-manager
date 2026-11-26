package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
	"smartdns-manager/models"
)

func (s *TelemetryService) checkDNSTargets() {
	var targets []models.TelemetryTarget
	if err := s.db.Where("type = ? AND enabled = ?", "dns", true).Find(&targets).Error; err != nil {
		log.Printf("读取 DNS 拨测目标失败: %v", err)
		return
	}
	for _, target := range targets {
		result, _ := s.CheckSingleTarget(context.Background(), target)
		event, err := s.recordDNSProbe(target, result)
		if err != nil {
			log.Printf("保存 DNS 拨测结果失败 [%d]: %v", target.ID, err)
		} else if event != "" {
			title := "DNS 解析异常"
			if event == "dns_probe_recovered" {
				title = "DNS 解析恢复"
			}
			content := fmt.Sprintf("目标：%s\n探测来源：控制台后端\nDNS：%s（UDP/53）\n域名：%s\n预期：%s\n结果：%s %s", target.Name, target.Target, target.DNSName, target.DNSExpectedA, result.Response, result.Error)
			if err := NewNotificationService().SendNotification(0, event, title, content); err != nil {
				log.Printf("DNS 拨测通知失败: %v", err)
			}
		}
	}
}

func (s *TelemetryService) recordDNSProbe(target models.TelemetryTarget, result *models.TelemetryResult) (string, error) {
	event := ""
	err := s.db.Transaction(func(tx *gorm.DB) error {
		alerting, streak, transition := dnsProbeTransition(target.DNSAlerting, target.DNSStreak, result.Success)
		updates := map[string]interface{}{
			"dns_alerting": alerting, "dns_streak": streak, "last_check_at": result.CheckedAt,
			"last_status": result.Success, "last_latency": result.Latency, "check_count": gorm.Expr("check_count + 1"),
			"updated_at": gorm.Expr("updated_at"),
		}
		if result.Success {
			updates["dns_last_success"] = result.CheckedAt
			updates["avg_latency"] = gorm.Expr("(avg_latency * success_count + ?) / (success_count + 1)", result.Latency)
			updates["success_count"] = gorm.Expr("success_count + 1")
		}
		updated := tx.Model(&target).Where("enabled = ? AND updated_at = ? AND dns_streak = ? AND dns_alerting = ?", true, target.UpdatedAt, target.DNSStreak, target.DNSAlerting).Updates(updates)
		if updated.Error != nil || updated.RowsAffected == 0 {
			return updated.Error
		}
		if err := tx.Create(result).Error; err != nil {
			return err
		}
		event = transition
		return nil
	})
	return event, err
}

func dnsProbeTransition(alerting bool, streak int, success bool) (bool, int, string) {
	if success != alerting {
		return alerting, 0, ""
	}
	streak++
	if alerting && streak == 2 {
		return false, 0, "dns_probe_recovered"
	}
	if !alerting && streak == 3 {
		return true, 0, "dns_probe_failed"
	}
	return alerting, streak, ""
}

func (s *TelemetryService) pruneDNSResults() {
	// DNS 结果每 30 秒生成一条，按现有结果模型保留最近七天。
	err := s.db.Unscoped().Where("created_at < ? AND target_id IN (?)", time.Now().AddDate(0, 0, -7), s.db.Unscoped().Model(&models.TelemetryTarget{}).Select("id").Where("type = ?", "dns")).Delete(&models.TelemetryResult{}).Error
	if err != nil {
		log.Printf("清理 DNS 拨测历史失败: %v", err)
	}
}
