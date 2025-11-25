package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"smartdns-log-agent/collector"
	"smartdns-log-agent/config"
	"smartdns-log-agent/handlers"
	"smartdns-log-agent/logger"
	"smartdns-log-agent/sender"

	"github.com/gin-gonic/gin"
)

const Version = "0.0.3"

type AgentServer struct {
	cfg        *config.Config
	collector  *collector.LogCollector
	sender     *sender.ClickHouseSender
	httpServer *http.Server
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	isRunning  bool
	startTime  time.Time
	handler    *handlers.AgentHandler
	logger     *logger.Logger // 新增日志管理器
}

func main() {
	// 命令行参数处理
	var showVersion = flag.Bool("version", false, "显示版本信息")
	var showHelp = flag.Bool("help", false, "显示帮助信息")
	flag.Parse()

	if *showVersion {
		fmt.Printf("SmartDNS Log Agent v%s\n", Version)
		os.Exit(0)
	}

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	log.Printf("🚀 SmartDNS Log Agent v%s 启动中...", Version)

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("❌ 加载配置失败:", err)
	}

	// 初始化日志管理器
	var loggerInstance *logger.Logger
	if cfg.LogConfig.EnableFile {
		loggerInstance, err = logger.NewLogger(cfg.LogConfig.LogDir, cfg.LogConfig.MaxDays)
		if err != nil {
			log.Printf("⚠️ 初始化文件日志失败: %v，将只使用控制台输出", err)
		} else {
			log.Printf("📁 文件日志已启用: %s (保留%d天)", cfg.LogConfig.LogDir, cfg.LogConfig.MaxDays)
		}
	}

	// 创建 Agent 服务器
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent := &AgentServer{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
		logger:    loggerInstance,
	}

	// 创建 API 处理器
	agent.handler = handlers.NewAgentHandler(
		cfg,
		agent.startTime,
		agent.getCollector,
		agent.getSender,
		agent.getRunning,
		agent.startLogCollection,
		agent.stopLogCollection,
		agent.getAgentLogs, // 新增获取日志方法
	)

	// 启动 HTTP API 服务器
	go agent.startHTTPServer()

	// 启动日志收集
	//if err := agent.startLogCollection(); err != nil {
	//	log.Printf("❌ 启动日志收集失败: %v", err)
	//}

	log.Println("✅ Agent 启动成功")

	// 等待退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 正在优雅关闭...")
	agent.shutdown()
}

// getAgentLogs 获取 Agent 日志
func (a *AgentServer) getAgentLogs(lines int) ([]string, error) {
	if a.logger == nil {
		// 如果没有启用文件日志，返回空
		return []string{"文件日志未启用"}, nil
	}
	return a.logger.GetRecentLogs(lines)
}

func printHelp() {
	fmt.Printf("SmartDNS Log Agent v%s\n\n", Version)
	fmt.Println("用法:")
	fmt.Println("  smartdns-log-agent [选项]")
	fmt.Println("\n选项:")
	fmt.Println("  --version    显示版本信息")
	fmt.Println("  --help       显示帮助信息")
	fmt.Println("\n环境变量配置:")
	fmt.Println("  NODE_ID                  节点ID")
	fmt.Println("  NODE_NAME                节点名称")
	fmt.Println("  LOG_FILE                 SmartDNS日志文件路径")
	fmt.Println("  CLICKHOUSE_HOST          ClickHouse 主机")
	fmt.Println("  CLICKHOUSE_PORT          ClickHouse 端口")
	fmt.Println("  CLICKHOUSE_DB            ClickHouse 数据库")
	fmt.Println("  CLICKHOUSE_USER          ClickHouse 用户")
	fmt.Println("  CLICKHOUSE_PASSWORD      ClickHouse 密码")
	fmt.Println("  AGENT_API_PORT           API 端口 (默认: 8888)")
	fmt.Println("  AGENT_LOG_DIR            Agent日志目录 (默认: /var/log/smartdns-agent)")
	fmt.Println("  AGENT_LOG_MAX_DAYS       日志保留天数 (默认: 7)")
	fmt.Println("  AGENT_LOG_ENABLE_FILE    是否启用文件日志 (默认: true)")
}

func (a *AgentServer) shutdown() {
	// 停止日志收集
	a.cancel()
	a.stopLogCollection()

	// 关闭 HTTP 服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if a.httpServer != nil {
		a.httpServer.Shutdown(ctx)
	}

	// 关闭日志管理器
	if a.logger != nil {
		a.logger.Close()
	}

	time.Sleep(1 * time.Second)
	log.Println("✅ Agent 已关闭")
}

// 其他方法保持不变...
func (a *AgentServer) getCollector() *collector.LogCollector {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.collector
}

func (a *AgentServer) getSender() *sender.ClickHouseSender {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.sender
}

func (a *AgentServer) getRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.isRunning
}

func (a *AgentServer) startHTTPServer() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// API 路由
	api := router.Group("/api/v1")
	{
		api.GET("/status", a.handler.GetStatus)
		api.POST("/start", a.handler.StartCollection)
		api.POST("/stop", a.handler.StopCollection)
		api.POST("/restart", a.handler.RestartCollection)
		api.GET("/stats", a.handler.GetStats)
		api.GET("/logs", a.handler.GetLogs)
		api.GET("/config", a.handler.GetConfig)
		api.PUT("/config", a.handler.UpdateConfig)
		api.GET("/health", a.handler.HealthCheck)
	}

	// 获取监听端口
	port := os.Getenv("AGENT_API_PORT")
	if port == "" {
		port = "8888"
	}

	a.httpServer = &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	log.Printf("🌐 HTTP API 服务器启动在端口: %s", port)
	if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("❌ HTTP 服务器启动失败: %v", err)
	}
}

func (a *AgentServer) startLogCollection() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return nil
	}

	// 创建 ClickHouse 发送器
	chSender, err := sender.NewClickHouseSender(a.cfg.ClickHouse)
	if err != nil {
		return err
	}
	a.sender = chSender

	// 创建日志收集器
	logCollector, err := collector.NewLogCollector(a.cfg, chSender)
	if err != nil {
		chSender.Close()
		return err
	}
	a.collector = logCollector

	// 启动收集器
	go a.collector.Start(a.ctx)

	a.isRunning = true
	log.Println("✅ 日志收集已启动")
	return nil
}

func (a *AgentServer) stopLogCollection() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return
	}

	if a.sender != nil {
		a.sender.Close()
		a.sender = nil
	}

	a.collector = nil
	a.isRunning = false
	log.Println("⏹️ 日志收集已停止")
}
