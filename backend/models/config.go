package models

import "time"

type SmartDNSConfig struct {
	Servers       []DNSServer       `json:"servers"`
	Addresses     []AddressMap      `json:"addresses"`
	DomainSets    []DomainSet       `json:"domain_sets"`
	DomainRules   []DomainRule      `json:"domain_rules"`
	Nameservers   []Nameserver      `json:"nameservers"`
	BasicSettings map[string]string `json:"basic_settings"`
}

// DNSServer DNS服务器
type DNSServer struct {
	ID             uint      `json:"id" gorm:"primarykey"`
	Address        string    `json:"address" gorm:"not null"`
	Type           string    `json:"type"` // udp, tcp, tls, https
	Groups         []string  `json:"groups" gorm:"-"`
	GroupsStr      string    `json:"-" gorm:"column:groups"` // 存储 JSON
	ExcludeDefault bool      `json:"exclude_default"`
	Options        string    `json:"options"`
	NodeIDs        string    `json:"node_ids"` // JSON 数组，应用到哪些节点
	Enabled        bool      `json:"enabled" gorm:"default:true"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AddressMap 地址映射
type AddressMap struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Domain    string    `json:"domain" gorm:"not null;index"`
	IP        string    `json:"ip" gorm:"not null"`
	Tags      string    `json:"tags"` // JSON 数组
	Comment   string    `json:"comment"`
	NodeIDs   string    `json:"node_ids"`                    // JSON 数组，应用到哪些节点，空表示全部
	Enabled   bool      `json:"enabled" gorm:"default:true"` // 是否启用
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DomainSet struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Name      string    `json:"name" gorm:"not null;unique"`
	FilePath  string    `json:"file_path"`
	Domains   []string  `json:"domains" gorm:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type DomainRule struct {
	ID         uint      `json:"id" gorm:"primarykey"`
	Domain     string    `json:"domain" gorm:"not null"`
	Nameserver string    `json:"nameserver"`
	Options    string    `json:"options"`
	CreatedAt  time.Time `json:"created_at"`
}

type Nameserver struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	Domain    string    `json:"domain" gorm:"not null"`
	Group     string    `json:"group" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
}

// ConfigSyncLog 配置同步日志
type ConfigSyncLog struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	NodeID    uint      `json:"node_id"`
	Action    string    `json:"action"` // add, update, delete
	Type      string    `json:"type"`   // address, server, full_sync
	Content   string    `json:"content"`
	Status    string    `json:"status"` // pending, success, failed
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
}
