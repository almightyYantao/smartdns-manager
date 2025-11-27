package main

import (
	"log"
	"smartdns-manager/config"
	"smartdns-manager/database"
	"smartdns-manager/handlers"
	"smartdns-manager/middleware"
	"smartdns-manager/services"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化数据库
	database.InitDB()
	database.InitClickHouse()

	// 创建 Gin 路由
	r := gin.Default()

	// CORS 配置
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true, // 允许所有来源（仅开发环境）
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * 3600,
	}))

	statusTime, err := strconv.Atoi(config.GetConfig().StatusTime)
	if err != nil {
		log.Printf("StatusTime配置错误，使用默认值10秒: %v", err)
		statusTime = 10
	}
	healthChecker := services.NewNodeHealthChecker(time.Duration(statusTime) * time.Second)
	healthChecker.Start()

	// 创建日志监控服务
	logMonitorService := services.NewLogMonitorService()

	// 创建S3服务
	var s3Service *services.S3Service
	
	// 创建调度服务
	schedulerService, err := services.NewSchedulerService(database.DB, config.GetConfig(), s3Service)
	if err != nil {
		log.Fatalf("创建调度服务失败: %v", err)
	}
	
	// 启动调度服务
	if err := schedulerService.Start(); err != nil {
		log.Printf("启动调度服务失败: %v", err)
	}

	// 创建数据库备份服务（保留兼容性）
	databaseBackupService := services.NewDatabaseBackupService(database.DB, s3Service)

	// 初始化处理器
	handlers.InitLogMonitorHandler(logMonitorService)
	databaseBackupHandler := handlers.NewDatabaseBackupHandler(database.DB, databaseBackupService)
	schedulerHandler := handlers.NewSchedulerHandler(schedulerService)

	defer healthChecker.Stop()
	defer schedulerService.Stop()

	// 公开路由
	public := r.Group("/api")
	{
		public.POST("/login", handlers.Login)
		public.POST("/register", handlers.Register)
	}

	// 注册路由
	logGroup := r.Group("/api/dns-logs")
	logGroup.Use(middleware.AuthMiddleware())
	logGroup.Use(middleware.AdminRequired())
	{
		logGroup.POST("/:id/log-monitor/start", handlers.StartNodeLogMonitor)     // 启动监控
		logGroup.POST("/:id/log-monitor/stop", handlers.StopNodeLogMonitor)       // 停止监控
		logGroup.GET("/:id/log-monitor/status", handlers.GetNodeLogMonitorStatus) // 监控状态
		logGroup.GET("/:id/logs/stats", handlers.GetLogStats)                     // 日志统计
		logGroup.POST("/:id/logs/clean", handlers.CleanOldLogs)                   // 清理日志
		logGroup.GET("", handlers.GetDNSLogs)                                     // 获取日志列表（支持按节点过滤）
	}

	handlers.InitVersionHandler("docker-v0.0.3")
	apiVersion := r.Group("/api")
	apiVersion.Use(middleware.AuthMiddleware())
	apiVersion.Use(middleware.AdminRequired())
	{
		// 版本管理相关路由
		version := apiVersion.Group("/version")
		{
			version.GET("/check", handlers.CheckVersion)
			version.GET("/info", handlers.GetSystemInfo)
			version.GET("/history", handlers.GetVersionHistory)
		}
	}

	// 需要认证的路由
	protected := r.Group("/api")
	protected.Use(middleware.AuthMiddleware())
	protected.Use(middleware.AdminRequired())
	{
		// 节点管理
		protected.GET("/nodes", handlers.GetNodes)
		protected.POST("/nodes", handlers.AddNode)
		protected.PUT("/nodes/:id", handlers.UpdateNode)
		protected.DELETE("/nodes/:id", handlers.DeleteNode)
		protected.POST("/nodes/:id/test", handlers.TestNodeConnection)

		// Agent 部署管理
		protected.POST("/nodes/:id/agent/deploy", handlers.DeployAgent)     // 部署 Agent
		protected.GET("/nodes/:id/agent/status", handlers.CheckAgentStatus) // 检查状态
		protected.DELETE("/nodes/:id/agent", handlers.UninstallAgent)       // 卸载 Agent
		protected.GET("/nodes/:id/agent/logs", handlers.GetAgentLogs)       // 获取日志

		// 配置管理
		protected.GET("/nodes/:id/config", handlers.GetNodeConfig)
		protected.POST("/nodes/:id/config", handlers.SaveNodeConfig)
		protected.POST("/nodes/:id/restart", handlers.RestartNodeService)
		protected.GET("/nodes/:id/status", handlers.GetNodeStatus)
		protected.GET("/nodes/:id/logs", handlers.GetNodeLogs)

		// 批量操作
		protected.POST("/nodes/batch/config", handlers.BatchUpdateConfig)
		protected.POST("/nodes/batch/restart", handlers.BatchRestart)

		// 地址映射管理
		protected.POST("/addresses", handlers.AddAddress)
		protected.PUT("/addresses/:id", handlers.UpdateAddress)
		protected.DELETE("/addresses/:id", handlers.DeleteAddress)
		protected.POST("/addresses/batch", handlers.BatchAddAddresses)
		protected.GET("/addresses", handlers.GetAddresses)

		// ========== 配置同步 ==========
		protected.POST("/sync/node/:id/full", handlers.TriggerFullSync) // 完整同步单个节点
		protected.POST("/sync/batch", handlers.BatchFullSync)           // 批量完整同步
		protected.GET("/sync/logs", handlers.GetSyncLogs)               // 获取同步日志
		protected.GET("/sync/stats", handlers.GetSyncStats)             // 同步统计
		protected.POST("/sync/logs/:id/retry", handlers.RetrySyncLog)   // 重试失败的同步
		protected.DELETE("/sync/logs", handlers.ClearSyncLogs)          // 清理日志

		// ========== 通知管理 ==========
		protected.GET("/notifications/channels", handlers.GetNotificationChannels)
		protected.POST("/notifications/channels", handlers.AddNotificationChannel)
		protected.PUT("/notifications/channels/:id", handlers.UpdateNotificationChannel)
		protected.DELETE("/notifications/channels/:id", handlers.DeleteNotificationChannel)
		protected.POST("/notifications/channels/:id/test", handlers.TestNotificationChannel)
		protected.GET("/notifications/logs", handlers.GetNotificationLogs)

		// ========== 节点初始化 ==========
		protected.POST("/nodes/:id/init", handlers.InitNode)               // 初始化节点
		protected.GET("/nodes/:id/init/status", handlers.CheckNodeInit)    // 检查初始化状态
		protected.GET("/nodes/:id/init/logs", handlers.GetInitLogs)        // 获取初始化日志
		protected.POST("/nodes/:id/uninstall", handlers.UninstallSmartDNS) // 卸载
		protected.POST("/nodes/:id/reinstall", handlers.ReinstallSmartDNS) // 重新安装

		// ========== 备份管理 ==========
		protected.GET("/nodes/:id/backups", handlers.GetNodeBackups)             // 获取备份列表
		protected.POST("/nodes/:id/backups", handlers.CreateNodeBackup)          // 创建备份（改为 /backups）
		protected.POST("/nodes/:id/backups/preview", handlers.PreviewBackup)     // 预览备份
		protected.POST("/nodes/:id/backups/restore", handlers.RestoreNodeBackup) // 还原备份（改为 /backups/restore）
		protected.DELETE("/nodes/:id/backups", handlers.DeleteNodeBackup)        // 删除备份
		protected.GET("/nodes/:id/backups/download", handlers.DownloadBackup)    // 下载备份

		// DNS 服务器管理
		protected.POST("/servers", handlers.AddServer)
		protected.PUT("/servers/:id", handlers.UpdateServer)
		protected.DELETE("/servers/:id", handlers.DeleteServer)
		protected.GET("/servers", handlers.GetServers)

		// 统计信息
		protected.GET("/dashboard/stats", handlers.GetDashboardStats)
		protected.GET("/dashboard/health", handlers.GetNodesHealth)

		// ========== 域名集管理 ==========
		protected.GET("/domain-sets", handlers.GetDomainSets)
		protected.GET("/domain-sets/:id", handlers.GetDomainSet)
		protected.POST("/domain-sets", handlers.AddDomainSet)
		protected.PUT("/domain-sets/:id", handlers.UpdateDomainSet)
		protected.DELETE("/domain-sets/:id", handlers.DeleteDomainSet)
		protected.POST("/domain-sets/:id/import", handlers.ImportDomainSetFile)
		protected.GET("/domain-sets/:id/export", handlers.ExportDomainSet)

		// ========== 域名规则管理 ==========
		protected.GET("/domain-rules", handlers.GetDomainRules)
		protected.POST("/domain-rules", handlers.AddDomainRule)
		protected.PUT("/domain-rules/:id", handlers.UpdateDomainRule)
		protected.DELETE("/domain-rules/:id", handlers.DeleteDomainRule)

		// DNS 分组管理
		protected.GET("/groups", handlers.GetGroups)
		protected.POST("/groups", handlers.AddGroup)
		protected.PUT("/groups/:id", handlers.UpdateGroup)
		protected.DELETE("/groups/:id", handlers.DeleteGroup)

		// ========== 命名服务器规则管理 ==========
		protected.GET("/nameservers", handlers.GetNameservers)
		protected.POST("/nameservers", handlers.AddNameserver)
		protected.PUT("/nameservers/:id", handlers.UpdateNameserver)
		protected.DELETE("/nameservers/:id", handlers.DeleteNameserver)

		// ========== 数据库备份管理 ==========
		// 备份配置管理
		protected.GET("/database-backup/configs", databaseBackupHandler.GetBackupConfigs)
		protected.POST("/database-backup/configs", databaseBackupHandler.CreateBackupConfig)
		protected.GET("/database-backup/configs/:id", databaseBackupHandler.GetBackupConfig)
		protected.PUT("/database-backup/configs/:id", databaseBackupHandler.UpdateBackupConfig)
		protected.DELETE("/database-backup/configs/:id", databaseBackupHandler.DeleteBackupConfig)
		
		// 备份操作
		protected.POST("/database-backup/configs/:id/backup", databaseBackupHandler.ManualBackup)
		protected.GET("/database-backup/history", databaseBackupHandler.GetBackupHistory)
		protected.POST("/database-backup/restore", databaseBackupHandler.RestoreBackup)
		protected.GET("/database-backup/stats", databaseBackupHandler.GetBackupStats)
		protected.POST("/database-backup/test-s3", databaseBackupHandler.TestS3Connection)

		// ========== 定时任务管理 ==========
		// 任务管理
		protected.GET("/scheduler/tasks", schedulerHandler.GetTasks)
		protected.POST("/scheduler/tasks", schedulerHandler.CreateTask)
		protected.GET("/scheduler/tasks/:id", schedulerHandler.GetTask)
		protected.PUT("/scheduler/tasks/:id", schedulerHandler.UpdateTask)
		protected.DELETE("/scheduler/tasks/:id", schedulerHandler.DeleteTask)
		protected.POST("/scheduler/tasks/:id/toggle", schedulerHandler.ToggleTask)
		protected.POST("/scheduler/tasks/:id/execute", schedulerHandler.ExecuteTask)
		
		// 任务执行历史
		protected.GET("/scheduler/tasks/:id/executions", schedulerHandler.GetTaskExecutions)
		protected.GET("/scheduler/running", schedulerHandler.GetRunningTasks)
		protected.GET("/scheduler/stats", schedulerHandler.GetStats)
		
		// 快速任务创建
		protected.POST("/scheduler/quick-task", schedulerHandler.CreateQuickTask)
		
		// 遥测目标管理
		protected.GET("/scheduler/telemetry/targets", schedulerHandler.GetTelemetryTargets)
		protected.POST("/scheduler/telemetry/targets", schedulerHandler.CreateTelemetryTarget)
		protected.PUT("/scheduler/telemetry/targets/:id", schedulerHandler.UpdateTelemetryTarget)
		protected.DELETE("/scheduler/telemetry/targets/:id", schedulerHandler.DeleteTelemetryTarget)
		protected.POST("/scheduler/telemetry/targets/:id/test", schedulerHandler.TestTelemetryTarget)
		
		// 遥测结果和统计
		protected.GET("/scheduler/telemetry/results", schedulerHandler.GetTelemetryResults)
		protected.GET("/scheduler/telemetry/stats", schedulerHandler.GetTelemetryStats)
		
		// 脚本模板管理
		protected.GET("/scheduler/script-templates", schedulerHandler.GetScriptTemplates)
	}

	// 启动服务器
	port := config.GetConfig().ServerPort
	log.Printf("Server starting on port %s", port)
	if err := r.Run("0.0.0.0:" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
