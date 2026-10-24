package services

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"smartdns-manager/config"
	"smartdns-manager/models"

	"gorm.io/gorm"
)

// CustomScriptService 自定义脚本执行服务
type CustomScriptService struct {
	db     *gorm.DB
	config *config.Config
}

// NewCustomScriptService 创建自定义脚本服务
func NewCustomScriptService(db *gorm.DB, config *config.Config) (*CustomScriptService, error) {
	return &CustomScriptService{
		db:     db,
		config: config,
	}, nil
}

// ExecuteScript 执行自定义脚本
func (s *CustomScriptService) ExecuteScript(ctx context.Context, scriptConfig models.CustomScriptConfig) (string, error) {
	var nodes []models.Node

	// 获取要执行脚本的节点列表
	query := s.db
	if len(scriptConfig.NodeIDs) > 0 {
		query = query.Where("id IN ?", scriptConfig.NodeIDs)
	}

	if err := query.Find(&nodes).Error; err != nil {
		return "", fmt.Errorf("查询节点失败: %w", err)
	}

	if len(nodes) == 0 {
		return "", fmt.Errorf("没有找到可执行的节点")
	}

	log.Printf("🎯 自定义脚本将在 %d 个节点上执行", len(nodes))

	var results []string
	var successCount, failCount int

	for _, node := range nodes {
		result, err := s.executeScriptOnNode(ctx, node, scriptConfig)
		if err != nil {
			failCount++
			results = append(results, fmt.Sprintf("节点 %s: 执行失败 - %v", node.Name, err))
			log.Printf("❌ 节点 %s 脚本执行失败: %v", node.Name, err)
		} else {
			successCount++
			results = append(results, fmt.Sprintf("节点 %s: 执行成功\n%s", node.Name, result))
			log.Printf("✅ 节点 %s 脚本执行成功", node.Name)
		}
	}

	summary := fmt.Sprintf("脚本执行完成: 成功 %d/%d 个节点\n\n", successCount, len(nodes))
	summary += strings.Join(results, "\n"+strings.Repeat("=", 50)+"\n")

	return summary, nil
}

// executeScriptOnNode 在指定节点上执行脚本
func (s *CustomScriptService) executeScriptOnNode(ctx context.Context, node models.Node, scriptConfig models.CustomScriptConfig) (string, error) {
	// 设置超时
	timeout := time.Duration(scriptConfig.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 300 * time.Second // 默认5分钟超时
	}

	scriptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 构建SSH命令
	sshCmd := s.buildSSHCommand(node, scriptConfig)

	log.Printf("🔧 在节点 %s 执行脚本命令: %s", node.Name, strings.Join(sshCmd, " "))

	// 执行命令
	cmd := exec.CommandContext(scriptCtx, sshCmd[0], sshCmd[1:]...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		return "", fmt.Errorf("命令执行失败: %w, 输出: %s", err, string(output))
	}

	return string(output), nil
}

// buildSSHCommand 构建SSH执行命令
func (s *CustomScriptService) buildSSHCommand(node models.Node, scriptConfig models.CustomScriptConfig) []string {
	// 基础SSH命令
	sshCmd := []string{
		"ssh",
		"-o", "ConnectTimeout=10",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}

	// 添加端口参数
	if node.Port != 22 {
		sshCmd = append(sshCmd, "-p", fmt.Sprintf("%d", node.Port))
	}

	// 添加用户和主机
	user := scriptConfig.RunAsUser
	if user == "" {
		user = "root"
	}
	sshCmd = append(sshCmd, fmt.Sprintf("%s@%s", user, node.Host))

	// 构建远程执行的脚本命令
	remoteCmd := s.buildRemoteCommand(scriptConfig)
	sshCmd = append(sshCmd, remoteCmd)

	return sshCmd
}

// buildRemoteCommand 构建远程执行命令
func (s *CustomScriptService) buildRemoteCommand(scriptConfig models.CustomScriptConfig) string {
	var cmdParts []string

	// 设置工作目录
	workingDir := scriptConfig.WorkingDir
	if workingDir == "" {
		workingDir = "/tmp"
	}
	cmdParts = append(cmdParts, fmt.Sprintf("cd %s", workingDir))

	// 设置环境变量
	for key, value := range scriptConfig.EnvVars {
		cmdParts = append(cmdParts, fmt.Sprintf("export %s='%s'", key, value))
	}

	// 添加默认环境变量
	if _, exists := scriptConfig.EnvVars["PATH"]; !exists {
		cmdParts = append(cmdParts, "export PATH='/usr/local/bin:/usr/bin:/bin'")
	}

	// 创建临时脚本文件并执行
	scriptContent := strings.ReplaceAll(scriptConfig.Script, "'", "'\"'\"'") // 转义单引号
	cmdParts = append(cmdParts, fmt.Sprintf("echo '%s' > /tmp/custom_script_$$.sh", scriptContent))
	cmdParts = append(cmdParts, "chmod +x /tmp/custom_script_$$.sh")
	cmdParts = append(cmdParts, "/tmp/custom_script_$$.sh")
	cmdParts = append(cmdParts, "rm -f /tmp/custom_script_$$.sh") // 清理临时文件

	return strings.Join(cmdParts, " && ")
}

// ValidateScript 验证脚本配置
func (s *CustomScriptService) ValidateScript(scriptConfig models.CustomScriptConfig) error {
	if strings.TrimSpace(scriptConfig.Script) == "" {
		return fmt.Errorf("脚本内容不能为空")
	}

	if scriptConfig.Timeout < 0 {
		return fmt.Errorf("超时时间不能为负数")
	}

	if scriptConfig.Timeout > 3600 {
		return fmt.Errorf("超时时间不能超过1小时")
	}

	// 检查脚本中是否包含危险命令
	dangerousPatterns := []string{
		"rm -rf /",
		"dd if=",
		"mkfs",
		"fdisk",
		"format",
	}

	scriptLower := strings.ToLower(scriptConfig.Script)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(scriptLower, pattern) {
			log.Printf("⚠️ 检测到潜在危险命令: %s", pattern)
			// 注意：这里只是警告，不阻止执行，因为可能有合法用途
		}
	}

	// 验证节点ID
	if len(scriptConfig.NodeIDs) > 0 {
		var count int64
		if err := s.db.Model(&models.Node{}).Where("id IN ?", scriptConfig.NodeIDs).Count(&count).Error; err != nil {
			return fmt.Errorf("验证节点ID失败: %w", err)
		}
		if count == 0 {
			return fmt.Errorf("指定的节点ID中没有可用的节点")
		}
	}

	return nil
}

// GetScriptTemplates 获取脚本模板
func (s *CustomScriptService) GetScriptTemplates() []ScriptTemplate {
	return []ScriptTemplate{
		{
			Name:        "系统信息收集",
			Description: "收集系统基本信息，包括硬件、内存、磁盘使用情况",
			Category:    "系统监控",
			Script: `#!/bin/bash
echo "=== 系统信息收集开始 ==="
echo "时间: $(date)"
echo ""

echo "=== 系统版本 ==="
uname -a
echo ""

echo "=== CPU信息 ==="
lscpu | head -20
echo ""

echo "=== 内存使用情况 ==="
free -h
echo ""

echo "=== 磁盘使用情况 ==="
df -h
echo ""

echo "=== 网络接口 ==="
ip addr show
echo ""

echo "=== 系统负载 ==="
uptime
echo ""

echo "=== 进程统计 ==="
ps aux --sort=-%cpu | head -10
echo ""

echo "=== 系统信息收集完成 ==="`,
		},
		{
			Name:        "SmartDNS服务管理",
			Description: "重启SmartDNS服务并检查状态",
			Category:    "服务管理",
			Script: `#!/bin/bash
echo "=== SmartDNS服务管理 ==="

echo "停止SmartDNS服务..."
systemctl stop smartdns

echo "等待服务完全停止..."
sleep 2

echo "启动SmartDNS服务..."
systemctl start smartdns

echo "等待服务启动完成..."
sleep 3

echo "检查服务状态..."
systemctl status smartdns --no-pager

echo "检查服务是否正在监听..."
netstat -tulnp | grep smartdns

echo "=== SmartDNS服务管理完成 ==="`,
		},
		{
			Name:        "日志清理",
			Description: "清理系统和应用程序的旧日志文件",
			Category:    "系统维护",
			Script: `#!/bin/bash
echo "=== 日志清理开始 ==="

# 清理7天前的系统日志
echo "清理系统日志..."
find /var/log -name "*.log" -mtime +7 -type f -exec rm -f {} \;
find /var/log -name "*.log.*" -mtime +7 -type f -exec rm -f {} \;

# 清理journal日志
echo "清理journal日志..."
journalctl --vacuum-time=7d

# 清理SmartDNS日志
if [ -d "/var/log/smartdns" ]; then
    echo "清理SmartDNS日志..."
    find /var/log/smartdns -name "*.log" -mtime +7 -type f -exec rm -f {} \;
fi

# 清理临时文件
echo "清理临时文件..."
find /tmp -type f -mtime +3 -exec rm -f {} \;

echo "=== 日志清理完成 ==="`,
		},
		{
			Name:        "网络连接测试",
			Description: "测试网络连接性和DNS解析",
			Category:    "网络诊断",
			Script: `#!/bin/bash
echo "=== 网络连接测试开始 ==="

# 测试基本网络连通性
echo "测试网络连通性..."
ping -c 3 8.8.8.8

echo ""
echo "测试DNS解析..."
nslookup google.com

echo ""
echo "测试HTTP连接..."
curl -I --connect-timeout 5 http://www.google.com

echo ""
echo "显示路由表..."
ip route show

echo ""
echo "显示DNS配置..."
cat /etc/resolv.conf

echo "=== 网络连接测试完成 ==="`,
		},
		{
			Name:        "系统更新检查",
			Description: "检查系统更新并显示可更新的包",
			Category:    "系统维护",
			Script: `#!/bin/bash
echo "=== 系统更新检查开始 ==="

# 检测系统类型
if command -v apt >/dev/null 2>&1; then
    echo "检测到Ubuntu/Debian系统，使用apt..."
    apt update
    echo ""
    echo "可更新的包列表:"
    apt list --upgradable
elif command -v yum >/dev/null 2>&1; then
    echo "检测到CentOS/RHEL系统，使用yum..."
    yum check-update
elif command -v dnf >/dev/null 2>&1; then
    echo "检测到Fedora系统，使用dnf..."
    dnf check-update
else
    echo "未识别的包管理器"
fi

echo ""
echo "=== 系统更新检查完成 ==="`,
		},
	}
}

// ScriptTemplate 脚本模板
type ScriptTemplate struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Script      string `json:"script"`
}
