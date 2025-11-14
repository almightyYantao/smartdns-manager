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

// DomainSet 域名集
type DomainSet struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	Name        string    `json:"name" gorm:"not null;uniqueIndex"`
	FilePath    string    `json:"file_path" gorm:"not null"`
	Description string    `json:"description"`
	DomainCount int       `json:"domain_count" gorm:"default:0"`
	NodeIDs     string    `json:"node_ids"` // JSON 数组，应用到哪些节点
	Enabled     bool      `json:"enabled" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DomainSetItem 域名集条目（域名列表）
type DomainSetItem struct {
	ID          uint      `json:"id" gorm:"primarykey"`
	DomainSetID uint      `json:"domain_set_id" gorm:"not null;index"`
	Domain      string    `json:"domain" gorm:"not null"`
	Comment     string    `json:"comment"`
	CreatedAt   time.Time `json:"created_at"`
}

// DomainRule 域名规则
type DomainRule struct {
	ID             uint      `json:"id" gorm:"primarykey"`
	Domain         string    `json:"domain" gorm:"not null;index"`       // 域名或 domain-set:name
	IsDomainSet    bool      `json:"is_domain_set" gorm:"default:false"` // 是否引用域名集
	DomainSetName  string    `json:"domain_set_name"`                    // 域名集名称
	Address        string    `json:"address"`                            // -address 参数
	Nameserver     string    `json:"nameserver"`                         // -nameserver 参数
	SpeedCheckMode string    `json:"speed_check_mode"`                   // -speed-check-mode 参数
	OtherOptions   string    `json:"other_options"`                      // 其他选项
	NodeIDs        string    `json:"node_ids"`                           // JSON 数组
	Enabled        bool      `json:"enabled" gorm:"default:true"`
	Priority       int       `json:"priority" gorm:"default:0"` // 优先级，数字越大越优先
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Nameserver 命名服务器规则
type Nameserver struct {
	ID            uint      `json:"id" gorm:"primarykey"`
	Domain        string    `json:"domain" gorm:"not null;index"`
	IsDomainSet   bool      `json:"is_domain_set" gorm:"default:false"`
	DomainSetName string    `json:"domain_set_name"`
	Group         string    `json:"group" gorm:"not null"`
	NodeIDs       string    `json:"node_ids"`
	Enabled       bool      `json:"enabled" gorm:"default:true"`
	Priority      int       `json:"priority" gorm:"default:0"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
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
