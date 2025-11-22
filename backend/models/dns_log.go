package models

import (
	"time"
)

// DNSLog SQLite 日志模型（用于 GORM）
type DNSLog struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	NodeID    uint      `json:"node_id" gorm:"index"`
	Timestamp time.Time `json:"timestamp" gorm:"index"`
	ClientIP  string    `json:"client_ip" gorm:"index"`
	Domain    string    `json:"domain" gorm:"index"`
	QueryType int       `json:"query_type"`
	TimeMs    int       `json:"time_ms"`
	SpeedMs   float64   `json:"speed_ms"`
	Result    string    `json:"result"`
	ResultIPs string    `json:"result_ips" gorm:"type:text"`
	IPCount   int       `json:"ip_count"`
	RawLog    string    `json:"raw_log" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}

func (DNSLog) TableName() string {
	return "dns_logs"
}

// DNSLogCK ClickHouse 日志模型（用于 ClickHouse 驱动）
type DNSLogCK struct {
	Timestamp   time.Time `json:"timestamp"`
	Date        time.Time `json:"date"`
	NodeID      uint32    `json:"node_id"`
	ClientIP    string    `json:"client_ip"`
	Domain      string    `json:"domain"`
	QueryType   uint16    `json:"query_type"`
	TimeMs      uint32    `json:"time_ms"`
	SpeedMs     float32   `json:"speed_ms"`
	ResultCount uint8     `json:"result_count"`
	ResultIPs   []string  `json:"result_ips"`
	RawLog      string    `json:"raw_log"`
}

// DNSLogStats 统计信息（通用）
type DNSLogStats struct {
	TotalQueries  int64        `json:"total_queries"`
	UniqueClients int64        `json:"unique_clients"`
	UniqueDomains int64        `json:"unique_domains"`
	AvgQueryTime  float64      `json:"avg_query_time"`
	TopDomains    []DomainStat `json:"top_domains"`
	TopClients    []ClientStat `json:"top_clients"`
	HourlyStats   []HourlyStat `json:"hourly_stats"`
}

type DomainStat struct {
	Domain string `json:"domain"`
	Count  int64  `json:"count"`
}

type ClientStat struct {
	ClientIP string `json:"client_ip"`
	Count    int64  `json:"count"`
}

type HourlyStat struct {
	Hour  int   `json:"hour"`
	Count int64 `json:"count"`
}

// DeployAgentRequest 部署请求结构
type DeployAgentRequest struct {
	NodeID             uint   `json:"node_id" binding:"required"`
	DeployMode         string `json:"deploy_mode" binding:"required"` // systemd 或 docker
	ClickHouseHost     string `json:"clickhouse_host" binding:"required"`
	ClickHousePort     int    `json:"clickhouse_port"`
	ClickHouseDB       string `json:"clickhouse_db"`
	ClickHouseUser     string `json:"clickhouse_user"`
	ClickHousePassword string `json:"clickhouse_password"`
	LogFilePath        string `json:"log_file_path"`
	BatchSize          int    `json:"batch_size"`
	FlushInterval      int    `json:"flush_interval"`
}
