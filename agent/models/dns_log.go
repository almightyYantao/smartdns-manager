package models

import (
	"time"
)

// DNSLogRecord DNS查询日志记录
type DNSLogRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	Date        time.Time `json:"date"`
	NodeID      uint32    `json:"node_id"`
	ClientIP    string    `json:"client_ip"`
	Domain      string    `json:"domain"`
	QueryType   uint16    `json:"query_type"`
	Group       string    `json:"group"`
	TimeMs      uint32    `json:"time_ms"`
	SpeedMs     float32   `json:"speed_ms"`
	ResultCount uint8     `json:"result_count"`
	ResultIPs   []string  `json:"result_ips"`
	RawLog      string    `json:"raw_log"`
}
