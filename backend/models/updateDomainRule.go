package models

type UpdateDomainRuleRequest struct {
	Domain         string `json:"domain"`
	IsDomainSet    bool   `json:"is_domain_set"`
	DomainSetName  string `json:"domain_set_name"`
	Address        string `json:"address"`
	Nameserver     string `json:"nameserver"`
	SpeedCheckMode string `json:"speed_check_mode"`
	OtherOptions   string `json:"other_options"`
	NodeIDs        []int  `json:"node_ids"` // 接收数组
	Enabled        *bool  `json:"enabled"`  // 使用指针，允许区分零值和未设置
	Priority       int    `json:"priority"`
	Description    string `json:"description"`
}
