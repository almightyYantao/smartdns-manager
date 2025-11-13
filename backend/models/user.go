package models

import (
	"gorm.io/gorm"
	"time"
)

type User struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	Username  string         `json:"username" gorm:"unique;not null"`
	Password  string         `json:"-" gorm:"not null"` // 哈希后的密码
	Email     string         `json:"email" gorm:"unique"`
	Role      string         `json:"role" gorm:"default:user"` // admin, user
	IsActive  bool           `json:"is_active" gorm:"default:true"`
	LastLogin time.Time      `json:"last_login"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
