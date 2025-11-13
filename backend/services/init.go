package services

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"smartdns-manager/database"
	"smartdns-manager/models"
)

type InitService struct {
	notificationService *NotificationService
}

func NewInitService() *InitService {
	return &InitService{
		notificationService: NewNotificationService(),
	}
}

// SmartDNSRelease SmartDNS 发行版信息
type SmartDNSRelease struct {
	Version      string
	DownloadURL  string
	Architecture string
	OSType       string
}

// GetLatestReleases 获取最新版本的下载链接
func (s *InitService) GetLatestReleases() map[string]SmartDNSRelease {
	// 使用固定版本，也可以从 GitHub API 动态获取
	version := "1.2024.11.10-2328"
	baseURL := "https://github.com/pymumu/smartdns/releases/download/Release46"

	return map[string]SmartDNSRelease{
		"x86_64-linux": {
			Version:      version,
			DownloadURL:  fmt.Sprintf("%s/smartdns.%s.x86_64-linux-all.tar.gz", baseURL, version),
			Architecture: "x86_64",
			OSType:       "linux",
		},
		"aarch64-linux": {
			Version:      version,
			DownloadURL:  fmt.Sprintf("%s/smartdns.%s.aarch64-linux-all.tar.gz", baseURL, version),
			Architecture: "aarch64",
			OSType:       "linux",
		},
		"arm-linux": {
			Version:      version,
			DownloadURL:  fmt.Sprintf("%s/smartdns.%s.arm-linux-all.tar.gz", baseURL, version),
			Architecture: "arm",
			OSType:       "linux",
		},
	}
}

// InitNode 初始化节点
func (s *InitService) InitNode(nodeID uint) error {
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		return fmt.Errorf("节点不存在: %w", err)
	}

	log.Printf("🚀 开始初始化节点: %s (%s)", node.Name, node.Host)

	// 更新初始化状态
	node.InitStatus = "initializing"
	database.DB.Save(&node)

	// 发送通知
	s.notificationService.SendNotification(
		node.ID,
		"node_init_start",
		"🚀 节点初始化开始",
		fmt.Sprintf("节点 `%s` 开始初始化 SmartDNS", node.Name),
	)

	// 步骤1: 检测系统环境
	if err := s.detectSystem(&node); err != nil {
		return s.handleInitError(&node, "detect", err)
	}

	// 步骤2: 检查 SmartDNS 是否已安装
	installed, version := s.checkSmartDNSInstalled(&node)
	if installed {
		log.Printf("✅ SmartDNS 已安装，版本: %s", version)
		node.InitStatus = "installed"
		node.SmartDNSVersion = version
		database.DB.Save(&node)

		s.notificationService.SendNotification(
			node.ID,
			"node_init_success",
			"✅ 节点已安装 SmartDNS",
			fmt.Sprintf("节点 `%s` 已安装 SmartDNS %s", node.Name, version),
		)
		return nil
	}

	// 步骤3: 下载 SmartDNS
	if err := s.downloadSmartDNS(&node); err != nil {
		return s.handleInitError(&node, "download", err)
	}

	// 步骤4: 安装 SmartDNS
	if err := s.installSmartDNS(&node); err != nil {
		return s.handleInitError(&node, "install", err)
	}

	// 步骤5: 初始化配置
	if err := s.initConfig(&node); err != nil {
		return s.handleInitError(&node, "configure", err)
	}

	// 步骤6: 启动服务
	if err := s.startService(&node); err != nil {
		return s.handleInitError(&node, "start", err)
	}

	// 更新状态
	node.InitStatus = "installed"
	database.DB.Save(&node)

	log.Printf("✅ 节点初始化完成: %s", node.Name)

	// 发送成功通知
	s.notificationService.SendNotification(
		node.ID,
		"node_init_success",
		"✅ 节点初始化成功",
		fmt.Sprintf("节点 `%s` SmartDNS 安装完成\n版本: %s", node.Name, node.SmartDNSVersion),
	)

	return nil
}

// detectSystem 检测系统环境
func (s *InitService) detectSystem(node *models.Node) error {
	log.Printf("📋 步骤1: 检测系统环境...")

	initLog := s.createInitLog(node.ID, "detect", "running", "检测系统环境")

	client, err := NewSSHClient(node)
	if err != nil {
		s.updateInitLog(initLog, "failed", "", err.Error())
		return fmt.Errorf("SSH连接失败: %w", err)
	}
	defer client.Close()

	// 检测操作系统
	osInfo, err := client.ExecuteCommand("cat /etc/os-release 2>/dev/null || cat /etc/redhat-release 2>/dev/null || echo 'Unknown'")
	if err != nil {
		s.updateInitLog(initLog, "failed", "", "无法获取系统信息")
		return fmt.Errorf("获取系统信息失败: %w", err)
	}

	// 解析系统类型
	osInfoLower := strings.ToLower(osInfo)
	if strings.Contains(osInfoLower, "ubuntu") {
		node.OSType = "ubuntu"
	} else if strings.Contains(osInfoLower, "debian") {
		node.OSType = "debian"
	} else if strings.Contains(osInfoLower, "centos") || strings.Contains(osInfoLower, "red hat") {
		node.OSType = "centos"
	} else if strings.Contains(osInfoLower, "alpine") {
		node.OSType = "alpine"
	} else {
		node.OSType = "linux"
	}

	// 提取版本号
	versionRe := regexp.MustCompile(`VERSION_ID="([^"]+)"`)
	if match := versionRe.FindStringSubmatch(osInfo); len(match) > 1 {
		node.OSVersion = match[1]
	}

	// 检测架构
	arch, err := client.ExecuteCommand("uname -m")
	if err != nil {
		s.updateInitLog(initLog, "failed", "", "无法获取系统架构")
		return fmt.Errorf("获取系统架构失败: %w", err)
	}
	node.Architecture = strings.TrimSpace(arch)

	// 检查并安装依赖
	dependencies := []string{"wget", "tar"}
	for _, dep := range dependencies {
		if _, err := client.ExecuteCommand(fmt.Sprintf("which %s", dep)); err != nil {
			log.Printf("⚠️  缺少依赖: %s，尝试安装...", dep)
			if err := s.installDependency(client, node.OSType, dep); err != nil {
				log.Printf("⚠️  安装依赖 %s 失败: %v", dep, err)
			}
		}
	}

	database.DB.Save(node)

	detail := fmt.Sprintf("OS: %s %s\nArchitecture: %s", node.OSType, node.OSVersion, node.Architecture)
	s.updateInitLog(initLog, "success", detail, "")

	log.Printf("✅ 系统检测完成: %s %s (%s)", node.OSType, node.OSVersion, node.Architecture)
	return nil
}

// checkSmartDNSInstalled 检查 SmartDNS 是否已安装
func (s *InitService) checkSmartDNSInstalled(node *models.Node) (bool, string) {
	client, err := NewSSHClient(node)
	if err != nil {
		return false, ""
	}
	defer client.Close()

	// 检查 smartdns 命令是否存在
	output, err := client.ExecuteCommand("smartdns -v 2>&1 || /usr/sbin/smartdns -v 2>&1")
	if err != nil {
		return false, ""
	}

	// 提取版本号
	versionRe := regexp.MustCompile(`SmartDNS\s+([^\s,]+)`)
	if match := versionRe.FindStringSubmatch(output); len(match) > 1 {
		return true, match[1]
	}

	return false, ""
}

// downloadSmartDNS 下载 SmartDNS
func (s *InitService) downloadSmartDNS(node *models.Node) error {
	log.Printf("📥 步骤2: 下载 SmartDNS...")

	initLog := s.createInitLog(node.ID, "download", "running", "下载 SmartDNS")

	client, err := NewSSHClient(node)
	if err != nil {
		s.updateInitLog(initLog, "failed", "", err.Error())
		return err
	}
	defer client.Close()

	// 获取下载链接
	releases := s.GetLatestReleases()
	var release SmartDNSRelease
	found := false

	// 根据架构选择合适的版本
	archKey := fmt.Sprintf("%s-linux", node.Architecture)
	if r, ok := releases[archKey]; ok {
		release = r
		found = true
	} else {
		// 尝试兼容性匹配
		for key, r := range releases {
			if strings.Contains(key, node.Architecture) {
				release = r
				found = true
				break
			}
		}
	}

	if !found {
		s.updateInitLog(initLog, "failed", "", "不支持的系统架构: "+node.Architecture)
		return fmt.Errorf("不支持的系统架构: %s", node.Architecture)
	}

	// 创建临时目录
	tmpDir := "/tmp/smartdns-install"
	client.ExecuteCommand(fmt.Sprintf("mkdir -p %s", tmpDir))

	// 下载文件
	fileName := fmt.Sprintf("smartdns.%s.%s-linux-all.tar.gz", release.Version, node.Architecture)
	//downloadPath := fmt.Sprintf("%s/%s", tmpDir, fileName)

	log.Printf("下载地址: %s", release.DownloadURL)

	// 使用 wget 下载，添加重试和超时
	downloadCmd := fmt.Sprintf("cd %s && wget --tries=3 --timeout=30 -q --show-progress '%s' -O %s",
		tmpDir, release.DownloadURL, fileName)

	output, err := client.ExecuteCommand(downloadCmd)
	if err != nil {
		s.updateInitLog(initLog, "failed", output, "下载失败: "+err.Error())
		return fmt.Errorf("下载失败: %w", err)
	}

	// 解压
	extractCmd := fmt.Sprintf("cd %s && tar zxf %s", tmpDir, fileName)
	if output, err := client.ExecuteCommand(extractCmd); err != nil {
		s.updateInitLog(initLog, "failed", output, "解压失败: "+err.Error())
		return fmt.Errorf("解压失败: %w", err)
	}

	node.SmartDNSVersion = release.Version
	database.DB.Save(node)

	s.updateInitLog(initLog, "success", fmt.Sprintf("版本: %s", release.Version), "")

	log.Printf("✅ SmartDNS 下载完成: %s", release.Version)
	return nil
}

// installSmartDNS 安装 SmartDNS
func (s *InitService) installSmartDNS(node *models.Node) error {
	log.Printf("📦 步骤3: 安装 SmartDNS...")

	initLog := s.createInitLog(node.ID, "install", "running", "安装 SmartDNS")

	client, err := NewSSHClient(node)
	if err != nil {
		s.updateInitLog(initLog, "failed", "", err.Error())
		return err
	}
	defer client.Close()

	tmpDir := "/tmp/smartdns-install"

	// 进入解压目录并执行安装
	installCmd := fmt.Sprintf("cd %s/smartdns && chmod +x ./install && sudo ./install -i", tmpDir)
	output, err := client.ExecuteCommand(installCmd)

	if err != nil {
		s.updateInitLog(initLog, "failed", output, err.Error())
		return fmt.Errorf("安装失败: %w", err)
	}

	s.updateInitLog(initLog, "success", output, "")

	log.Printf("✅ SmartDNS 安装完成")
	return nil
}

// initConfig 初始化配置
func (s *InitService) initConfig(node *models.Node) error {
	log.Printf("⚙️  步骤4: 初始化配置...")

	initLog := s.createInitLog(node.ID, "configure", "running", "初始化配置文件")

	client, err := NewSSHClient(node)
	if err != nil {
		s.updateInitLog(initLog, "failed", "", err.Error())
		return err
	}
	defer client.Close()

	// 创建默认配置
	defaultConfig := `# SmartDNS 配置文件
# 由 SmartDNS Manager 自动生成

# 绑定端口
bind :53

# 缓存设置
cache-size 4096
prefetch-domain yes
serve-expired yes

# TTL 设置
rr-ttl-min 60
rr-ttl-max 3600

# 日志设置
log-level info
log-file /var/log/smartdns/smartdns.log
log-size 128k

# 审计日志
audit-enable yes
audit-size 16M
audit-file /var/log/smartdns/audit.log

# 强制 AAAA 查询返回 SOA
force-AAAA-SOA yes

# 禁用双栈选择
dualstack-ip-selection no

# 默认上游 DNS 服务器
server 8.8.8.8
server 114.114.114.114
`

	// 设置配置文件路径
	configPath := node.ConfigPath
	if configPath == "" {
		configPath = "/etc/smartdns/smartdns.conf"
		node.ConfigPath = configPath
	}

	// 备份原配置（如果存在）
	backupCmd := fmt.Sprintf("sudo cp %s %s.bak.$(date +%%s) 2>/dev/null || true", configPath, configPath)
	client.ExecuteCommand(backupCmd)

	// 写入新配置
	if err := client.WriteFile(configPath, defaultConfig); err != nil {
		s.updateInitLog(initLog, "failed", "", "写入配置文件失败: "+err.Error())
		return fmt.Errorf("写入配置失败: %w", err)
	}

	database.DB.Save(node)

	s.updateInitLog(initLog, "success", "配置文件: "+configPath, "")

	log.Printf("✅ 配置初始化完成")
	return nil
}

// startService 启动服务
func (s *InitService) startService(node *models.Node) error {
	log.Printf("🚀 步骤5: 启动 SmartDNS 服务...")

	initLog := s.createInitLog(node.ID, "start", "running", "启动 SmartDNS 服务")

	client, err := NewSSHClient(node)
	if err != nil {
		s.updateInitLog(initLog, "failed", "", err.Error())
		return err
	}
	defer client.Close()

	// 重新加载 systemd
	client.ExecuteCommand("sudo systemctl daemon-reload")

	// 启用开机自启
	if _, err := client.ExecuteCommand("sudo systemctl enable smartdns"); err != nil {
		log.Printf("⚠️  启用开机自启失败: %v", err)
	}

	// 启动服务
	if err := client.RestartService("smartdns"); err != nil {
		s.updateInitLog(initLog, "failed", "", "启动服务失败: "+err.Error())
		return fmt.Errorf("启动服务失败: %w", err)
	}

	// 等待服务启动
	time.Sleep(3 * time.Second)

	// 检查服务状态
	isRunning, err := client.GetServiceStatus("smartdns")
	if err != nil || !isRunning {
		s.updateInitLog(initLog, "failed", "", "服务未正常运行")
		return fmt.Errorf("服务未正常运行")
	}

	s.updateInitLog(initLog, "success", "SmartDNS 服务已启动", "")

	log.Printf("✅ SmartDNS 服务启动成功")
	return nil
}

// installDependency 安装依赖
func (s *InitService) installDependency(client *SSHClient, osType, packageName string) error {
	var installCmd string

	switch osType {
	case "ubuntu", "debian":
		installCmd = fmt.Sprintf("sudo apt-get update -qq && sudo apt-get install -y %s", packageName)
	case "centos":
		installCmd = fmt.Sprintf("sudo yum install -y %s", packageName)
	case "alpine":
		installCmd = fmt.Sprintf("sudo apk add --no-cache %s", packageName)
	default:
		return fmt.Errorf("不支持的操作系统: %s", osType)
	}

	_, err := client.ExecuteCommand(installCmd)
	return err
}

// UninstallSmartDNS 卸载 SmartDNS
func (s *InitService) UninstallSmartDNS(nodeID uint) error {
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		return fmt.Errorf("节点不存在: %w", err)
	}

	log.Printf("🗑️  开始卸载 SmartDNS: %s", node.Name)

	client, err := NewSSHClient(&node)
	if err != nil {
		return fmt.Errorf("SSH连接失败: %w", err)
	}
	defer client.Close()

	// 停止服务
	log.Printf("停止 SmartDNS 服务...")
	client.ExecuteCommand("sudo systemctl stop smartdns")
	client.ExecuteCommand("sudo systemctl disable smartdns")

	// 执行卸载脚本
	log.Printf("执行卸载...")
	tmpDir := "/tmp/smartdns-install"
	uninstallCmd := fmt.Sprintf("cd %s/smartdns 2>/dev/null && sudo ./install -u || true", tmpDir)
	client.ExecuteCommand(uninstallCmd)

	// 手动清理
	log.Printf("清理文件...")
	client.ExecuteCommand("sudo rm -f /usr/sbin/smartdns")
	client.ExecuteCommand("sudo rm -f /etc/systemd/system/smartdns.service")
	client.ExecuteCommand("sudo rm -rf /etc/smartdns")
	client.ExecuteCommand("sudo rm -rf /var/log/smartdns")
	client.ExecuteCommand(fmt.Sprintf("sudo rm -rf %s", tmpDir))

	// 重新加载 systemd
	client.ExecuteCommand("sudo systemctl daemon-reload")

	// 更新节点状态
	node.InitStatus = "not_installed"
	node.SmartDNSVersion = ""
	database.DB.Save(&node)

	log.Printf("✅ SmartDNS 卸载完成")

	// 发送通知
	s.notificationService.SendNotification(
		node.ID,
		"node_uninstall",
		"🗑️ SmartDNS 已卸载",
		fmt.Sprintf("节点 `%s` 的 SmartDNS 已被卸载", node.Name),
	)

	return nil
}

// CheckAndUpdateNodeStatus 检查并更新节点状态
func (s *InitService) CheckAndUpdateNodeStatus(node *models.Node) error {
	client, err := NewSSHClient(node)
	if err != nil {
		node.InitStatus = "unknown"
		database.DB.Save(node)
		return err
	}
	defer client.Close()

	// 检查是否安装
	installed, version := s.checkSmartDNSInstalled(node)
	if installed {
		node.InitStatus = "installed"
		node.SmartDNSVersion = version

		// 检测系统信息（如果未检测过）
		if node.OSType == "" {
			s.detectSystem(node)
		}
	} else {
		node.InitStatus = "not_installed"
		node.SmartDNSVersion = ""
	}

	database.DB.Save(node)
	return nil
}

// handleInitError 处理初始化错误
func (s *InitService) handleInitError(node *models.Node, step string, err error) error {
	log.Printf("❌ 初始化失败 (%s): %v", step, err)

	node.InitStatus = "failed"
	database.DB.Save(node)

	// 发送失败通知
	s.notificationService.SendNotification(
		node.ID,
		"node_init_failed",
		"❌ 节点初始化失败",
		fmt.Sprintf("节点 `%s` 初始化失败\n\n步骤: %s\n错误: %s", node.Name, step, err.Error()),
	)

	return err
}

// createInitLog 创建初始化日志
func (s *InitService) createInitLog(nodeID uint, step, status, message string) *models.InitLog {
	initLog := &models.InitLog{
		NodeID:    nodeID,
		Step:      step,
		Status:    status,
		Message:   message,
		StartedAt: time.Now(),
	}
	database.DB.Create(initLog)
	return initLog
}

// updateInitLog 更新初始化日志
func (s *InitService) updateInitLog(initLog *models.InitLog, status, detail, errorMsg string) {
	initLog.Status = status
	initLog.Detail = detail
	initLog.Error = errorMsg
	initLog.EndedAt = time.Now()
	database.DB.Save(initLog)
}
