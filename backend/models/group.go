package models

import "time"

type DNSGroup struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex;not null"`
	Description string    `json:"description"`
	Color       string    `json:"color"`                          // 用于前端显示
	IsSystem    bool      `json:"is_system" gorm:"default:false"` // 系统预设分组
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
