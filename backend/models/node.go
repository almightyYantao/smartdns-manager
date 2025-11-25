package models

import (
	"time"

	"gorm.io/gorm"
)

type Node struct {
	ID                   uint                  `json:"id" gorm:"primarykey"`
	Name                 string                `json:"name" gorm:"not null"`
	Host                 string                `json:"host" gorm:"not null"`
	Port                 int                   `json:"port" gorm:"default:22"`
	Username             string                `json:"username" gorm:"not null"`
	Password             string                `json:"password,omitempty"`
	PrivateKey           string                `json:"private_key,omitempty"`
	ConfigPath           string                `json:"config_path" gorm:"default:/etc/smartdns/smartdns.conf"`
	LogPath              string                `json:"log_path" gorm:"default:/var/log/audit/audit.log"`
	LogMonitorEnabled    bool                  `json:"log_monitor_enabled" gorm:"default:false"`
	Status               string                `json:"status" gorm:"default:unknown"`      // online, offline, error
	InitStatus           string                `json:"init_status" gorm:"default:unknown"` // unknown, not_installed, installed, initializing, failed
	SmartDNSVersion      string                `json:"smartdns_version"`
	OSType               string                `json:"os_type"` // ubuntu, debian, centos, alpine
	OSVersion            string                `json:"os_version"`
	Architecture         string                `json:"architecture"` // x86_64, aarch64, arm
	LastCheck            time.Time             `json:"last_check"`
	Tags                 string                `json:"tags"`
	Description          string                `json:"description"`
	EnableNotification   bool                  `json:"enable_notification" gorm:"default:true"`
	NotificationChannels []NotificationChannel `json:"notification_channels" gorm:"foreignKey:NodeID"`
	CreatedAt            time.Time             `json:"created_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
	DeletedAt            gorm.DeletedAt        `json:"-" gorm:"index"`
	AgentAPIPort         int                   `json:"agent_api_port" gorm:"default:8888"`

	AgentInstalled bool   `json:"agent_installed" gorm:"default:false"`
	AgentVersion   string `json:"agent_version"`
	DeployMode     string `json:"deploy_mode"`
	AgentConfig    string `json:"agent_config" gorm:"type:text"`

	ProxyConfig *ProxyConfig `json:"proxy_config" gorm:"type:json"`
}

type ProxyConfig struct {
	Enabled   bool   `json:"enabled"`
	ProxyType string `json:"proxy_type"` // "socks5", "http", "ssh"
	ProxyHost string `json:"proxy_host"`
	ProxyPort int    `json:"proxy_port"`
	ProxyUser string `json:"proxy_user,omitempty"`
	ProxyPass string `json:"proxy_pass,omitempty"`
	// SSH跳板机配置
	JumpHost     string `json:"jump_host,omitempty"`
	JumpPort     int    `json:"jump_port,omitempty"`
	JumpUser     string `json:"jump_user,omitempty"`
	JumpPassword string `json:"jump_password,omitempty"`
}

// InitLog 初始化日志
type InitLog struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	NodeID    uint      `json:"node_id"`
	Step      string    `json:"step"`   // detect, download, install, configure, start
	Status    string    `json:"status"` // pending, running, success, failed
	Message   string    `json:"message"`
	Detail    string    `json:"detail" gorm:"type:text"`
	Error     string    `json:"error"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
	CreatedAt time.Time `json:"created_at"`
}

type NodeConfig struct {
	NodeID    uint      `json:"node_id"`
	Content   string    `json:"content"`
	Checksum  string    `json:"checksum"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NodeStatus struct {
	NodeID      uint      `json:"node_id"`
	IsOnline    bool      `json:"is_online"`
	ServiceUp   bool      `json:"service_up"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	DiskUsage   float64   `json:"disk_usage"`
	Uptime      int64     `json:"uptime"`
	Version     string    `json:"version"`
	LastChecked time.Time `json:"last_checked"`
}
