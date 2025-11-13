package config

// NotificationEvent 通知事件类型
type NotificationEvent struct {
	Key         string
	Name        string
	Description string
}

var NotificationEvents = []NotificationEvent{
	{
		Key:         "*",
		Name:        "全部事件",
		Description: "订阅所有类型的通知",
	},
	{
		Key:         "sync_success",
		Name:        "同步成功",
		Description: "配置同步成功时触发",
	},
	{
		Key:         "sync_failed",
		Name:        "同步失败",
		Description: "配置同步失败时触发",
	},
	{
		Key:         "node_online",
		Name:        "节点上线",
		Description: "节点状态变为在线时触发",
	},
	{
		Key:         "node_offline",
		Name:        "节点离线",
		Description: "节点状态变为离线时触发",
	},
	{
		Key:         "service_restart",
		Name:        "服务重启",
		Description: "SmartDNS 服务重启时触发",
	},
	{
		Key:         "config_backup",
		Name:        "配置备份",
		Description: "创建配置备份时触发",
	},
	{
		Key:         "config_restore",
		Name:        "配置恢复",
		Description: "恢复配置备份时触发",
	},
	{
		Key:         "test",
		Name:        "测试消息",
		Description: "测试通知渠道时触发",
	},
}
