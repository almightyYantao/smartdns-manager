package models

import (
	"time"
	"gorm.io/gorm"
)

// TaskType 任务类型枚举
type TaskType string

const (
	TaskTypeDBBackup      TaskType = "db_backup"      // 数据库备份
	TaskTypeNodeBackup    TaskType = "node_backup"    // 节点配置备份
	TaskTypeLogCleanup    TaskType = "log_cleanup"    // 日志清理
	TaskTypeTelemetry     TaskType = "telemetry"      // 网络遥测
	TaskTypeCustomScript  TaskType = "custom_script"  // 自定义脚本执行
)

// TaskStatus 任务状态枚举
type TaskStatus string

const (
	TaskStatusPending  TaskStatus = "pending"  // 等待执行
	TaskStatusRunning  TaskStatus = "running"  // 正在执行
	TaskStatusSuccess  TaskStatus = "success"  // 执行成功
	TaskStatusFailed   TaskStatus = "failed"   // 执行失败
	TaskStatusSkipped  TaskStatus = "skipped"  // 跳过执行
)

// ScheduledTask 定时任务配置
type ScheduledTask struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"not null;size:100;comment:任务名称"`
	Type        TaskType  `json:"type" gorm:"not null;comment:任务类型"`
	Description string    `json:"description" gorm:"size:500;comment:任务描述"`
	CronExpr    string    `json:"cron_expr" gorm:"not null;size:100;comment:Cron表达式"`
	Config      string    `json:"config" gorm:"type:text;comment:任务配置JSON"`
	Enabled     bool      `json:"enabled" gorm:"default:true;comment:是否启用"`
	
	// 执行状态
	LastRunAt    *time.Time `json:"last_run_at" gorm:"comment:上次执行时间"`
	NextRunAt    *time.Time `json:"next_run_at" gorm:"comment:下次执行时间"`
	LastStatus   TaskStatus `json:"last_status" gorm:"default:pending;comment:上次执行状态"`
	LastError    string     `json:"last_error" gorm:"type:text;comment:上次执行错误"`
	RunCount     int        `json:"run_count" gorm:"default:0;comment:执行次数"`
	SuccessCount int        `json:"success_count" gorm:"default:0;comment:成功次数"`
	
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TaskExecution 任务执行历史
type TaskExecution struct {
	ID       uint       `json:"id" gorm:"primaryKey"`
	TaskID   uint       `json:"task_id" gorm:"not null;index;comment:任务ID"`
	Task     ScheduledTask `json:"task" gorm:"foreignKey:TaskID"`
	
	Status    TaskStatus `json:"status" gorm:"not null;comment:执行状态"`
	StartedAt time.Time  `json:"started_at" gorm:"not null;comment:开始时间"`
	EndedAt   *time.Time `json:"ended_at" gorm:"comment:结束时间"`
	Duration  int64      `json:"duration" gorm:"comment:执行时长(毫秒)"`
	
	Output    string `json:"output" gorm:"type:text;comment:执行输出"`
	Error     string `json:"error" gorm:"type:text;comment:错误信息"`
	Metadata  string `json:"metadata" gorm:"type:text;comment:元数据JSON"`
	
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TelemetryTarget 遥测目标
type TelemetryTarget struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"not null;size:100;comment:目标名称"`
	Type        string `json:"type" gorm:"not null;size:20;comment:检测类型(ping/http/tcp)"`
	Target      string `json:"target" gorm:"not null;size:255;comment:目标地址"`
	Timeout     int    `json:"timeout" gorm:"default:5000;comment:超时时间(毫秒)"`
	Enabled     bool   `json:"enabled" gorm:"default:true;comment:是否启用"`
	Description string `json:"description" gorm:"size:500;comment:描述"`
	
	// 统计信息
	LastCheckAt    *time.Time `json:"last_check_at" gorm:"comment:上次检测时间"`
	LastLatency    int64      `json:"last_latency" gorm:"comment:上次延迟(毫秒)"`
	LastStatus     bool       `json:"last_status" gorm:"comment:上次检测状态"`
	CheckCount     int        `json:"check_count" gorm:"default:0;comment:检测次数"`
	SuccessCount   int        `json:"success_count" gorm:"default:0;comment:成功次数"`
	AvgLatency     float64    `json:"avg_latency" gorm:"comment:平均延迟(毫秒)"`
	
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TelemetryResult 遥测结果
type TelemetryResult struct {
	ID       uint            `json:"id" gorm:"primaryKey"`
	TargetID uint            `json:"target_id" gorm:"not null;index;comment:目标ID"`
	Target   TelemetryTarget `json:"target" gorm:"foreignKey:TargetID"`
	
	Success   bool    `json:"success" gorm:"not null;comment:是否成功"`
	Latency   int64   `json:"latency" gorm:"comment:延迟(毫秒)"`
	Error     string  `json:"error" gorm:"type:text;comment:错误信息"`
	Response  string  `json:"response" gorm:"type:text;comment:响应内容"`
	CheckedAt time.Time `json:"checked_at" gorm:"not null;comment:检测时间"`
	
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// TableName 设置表名
func (ScheduledTask) TableName() string {
	return "scheduled_tasks"
}

func (TaskExecution) TableName() string {
	return "task_executions"
}

func (TelemetryTarget) TableName() string {
	return "telemetry_targets"
}

func (TelemetryResult) TableName() string {
	return "telemetry_results"
}

// 任务配置结构体

// S3Config S3配置
type S3Config struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	Endpoint  string `json:"endpoint,omitempty"` // 自定义端点(如MinIO)
	Prefix    string `json:"prefix,omitempty"`   // 存储前缀
}

// DBBackupConfig 数据库备份任务配置
type DBBackupConfig struct {
	S3Config     S3Config `json:"s3_config"`
	Compression  bool     `json:"compression"`
	Encryption   bool     `json:"encryption"`
	RetentionDays int     `json:"retention_days"`
}

// NodeBackupConfig 节点配置备份任务配置
type NodeBackupConfig struct {
	// 存储配置
	StorageType   string   `json:"storage_type"`   // 存储类型: "local" 或 "s3"
	LocalPath     string   `json:"local_path"`     // 本地存储路径
	S3Config      S3Config `json:"s3_config"`      // S3存储配置
	
	// 备份内容配置
	NodeIDs       []uint   `json:"node_ids"`       // 空表示所有节点
	BackupConfigs bool     `json:"backup_configs"` // 是否备份配置文件
	BackupLogs    bool     `json:"backup_logs"`    // 是否备份日志
	
	// 通用配置
	Compression   bool     `json:"compression"`    // 是否压缩备份文件
	RetentionDays int      `json:"retention_days"` // 备份保留天数
}

// LogCleanupConfig 日志清理任务配置
type LogCleanupConfig struct {
	AgentLogDays    int      `json:"agent_log_days"`    // agent日志保留天数
	BackendLogDays  int      `json:"backend_log_days"`  // backend日志保留天数
	SmartDNSLogDays int      `json:"smartdns_log_days"` // SmartDNS日志保留天数
	LogPaths        []string `json:"log_paths"`         // 自定义日志路径
}

// TelemetryConfig 遥测任务配置
type TelemetryConfig struct {
	Targets         []uint `json:"targets"`          // 遥测目标ID列表，空表示所有启用的目标
	ResultRetention int    `json:"result_retention"` // 结果保留天数
	AlertThreshold  int    `json:"alert_threshold"`  // 连续失败告警阈值
}

// CustomScriptConfig 自定义脚本任务配置
type CustomScriptConfig struct {
	NodeIDs     []uint            `json:"node_ids"`     // 要执行脚本的节点ID列表，空表示所有节点
	Script      string            `json:"script"`       // Shell脚本内容
	Timeout     int               `json:"timeout"`      // 脚本执行超时时间（秒），默认300秒
	WorkingDir  string            `json:"working_dir"`  // 脚本执行的工作目录，默认/tmp
	EnvVars     map[string]string `json:"env_vars"`     // 环境变量设置
	RunAsUser   string            `json:"run_as_user"`  // 执行脚本的用户，默认root
}

// TaskStats 任务统计信息
type TaskStats struct {
	TotalTasks        int64      `json:"total_tasks"`
	EnabledTasks      int64      `json:"enabled_tasks"`
	RunningTasks      int64      `json:"running_tasks"`
	TotalExecutions   int64      `json:"total_executions"`
	SuccessExecutions int64      `json:"success_executions"`
	FailedExecutions  int64      `json:"failed_executions"`
	SuccessRate       float64    `json:"success_rate"`
	LastExecutionAt   *time.Time `json:"last_execution_at"`
	NextExecutionAt   *time.Time `json:"next_execution_at"`
}

// TelemetryStats 遥测统计信息
type TelemetryStats struct {
	TargetID      uint       `json:"target_id"`
	TargetName    string     `json:"target_name"`
	CheckCount    int        `json:"check_count"`
	SuccessCount  int        `json:"success_count"`
	SuccessRate   float64    `json:"success_rate"`
	LastCheckAt   *time.Time `json:"last_check_at"`
	LastLatency   int64      `json:"last_latency"`
	AvgLatency    float64    `json:"avg_latency"`
	LastStatus    bool       `json:"last_status"`
}