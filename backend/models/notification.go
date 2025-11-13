package models

import (
	"time"
)

// NotificationChannel 通知渠道
type NotificationChannel struct {
	ID         uint      `json:"id" gorm:"primarykey"`
	NodeID     uint      `json:"node_id"`                     // 关联的节点ID，0表示全局
	Name       string    `json:"name" gorm:"not null"`        // 渠道名称
	Type       string    `json:"type" gorm:"not null"`        // wechat, dingtalk, feishu, slack
	WebhookURL string    `json:"webhook_url" gorm:"not null"` // Webhook URL
	Secret     string    `json:"secret"`                      // 签名密钥（钉钉、飞书需要）
	Events     string    `json:"events"`                      // JSON数组，订阅的事件类型
	Enabled    bool      `json:"enabled" gorm:"default:true"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NotificationLog 通知日志
type NotificationLog struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	ChannelID uint      `json:"channel_id"`
	NodeID    uint      `json:"node_id"`
	EventType string    `json:"event_type"` // sync_success, sync_failed, node_offline等
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Status    string    `json:"status"` // success, failed
	Error     string    `json:"error"`
	SentAt    time.Time `json:"sent_at"`
	CreatedAt time.Time `json:"created_at"`
}
