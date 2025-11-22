package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"smartdns-manager/models"
)

type AgentDeployService struct {
	sshTimeout time.Duration
}

type AgentStatus struct {
	Installed    bool     `json:"installed"`
	Running      bool     `json:"running"`
	Version      string   `json:"version"`
	DeployMode   string   `json:"deploy_mode"`
	AutoStart    bool     `json:"auto_start"`
	LastCheck    string   `json:"last_check"`
	ProcessInfo  string   `json:"process_info"`
	LogTail      []string `json:"log_tail"`
	ErrorMessage string   `json:"error_message,omitempty"`
}

type DeployResponse struct {
	Success     bool     `json:"success"`
	Message     string   `json:"message"`
	Output      []string `json:"output"`
	InstallPath string   `json:"install_path"`
	ServiceName string   `json:"service_name"`
	ConfigPath  string   `json:"config_path"`
}

func NewAgentDeployService() *AgentDeployService {
	return &AgentDeployService{
		sshTimeout: 300 * time.Second, // 5分钟超时
	}
}

func (s *AgentDeployService) DeployAgent(node *models.Node, req *models.DeployAgentRequest) (*DeployResponse, error) {
	log.Printf("开始部署 Agent 到节点 %s (%s)", node.Name, node.Host)

	// 创建 SSH 客户端
	sshClient, err := NewSSHClient(node)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %w", err)
	}
	defer sshClient.Close()

	// 生成安装命令
	installCmd := s.generateInstallCommand(node, req)

	log.Printf("执行安装命令: %s", installCmd)

	// 执行安装
	output, err := sshClient.ExecuteCommandWithTimeout(installCmd, s.sshTimeout)
	if err != nil {
		return nil, fmt.Errorf("安装失败: %w", err)
	}

	// 解析输出
	outputLines := strings.Split(output, "\n")

	// 检查安装结果
	success := strings.Contains(output, "安装成功") || strings.Contains(output, "installation successful")

	response := &DeployResponse{
		Success:     success,
		Message:     "Agent 部署完成",
		Output:      outputLines,
		InstallPath: "/opt/smartdns-log-agent",
		ServiceName: "smartdns-log-agent",
		ConfigPath:  "/etc/smartdns-log-agent/config",
	}

	if req.DeployMode == "docker" {
		response.InstallPath = "/opt/smartdns-log-agent"
		response.ConfigPath = "/opt/smartdns-log-agent/.env"
	}

	// 等待服务启动
	time.Sleep(5 * time.Second)

	// 验证安装
	status, err := s.CheckAgentStatus(node)
	if err != nil {
		log.Printf("验证安装状态失败: %v", err)
	} else if !status.Running {
		response.Success = false
		response.Message = "Agent 安装完成但未正常运行"
	}

	return response, nil
}

func (s *AgentDeployService) generateInstallCommand(node *models.Node, req *models.DeployAgentRequest) string {
	// GitHub 仓库地址（需要替换为实际地址）
	repoURL := "https://raw.githubusercontent.com/almightyyantao/smartdns-manager/main/agent/install.sh"

	// 构建安装参数
	params := []string{
		fmt.Sprintf("-n %d", req.NodeID),
		fmt.Sprintf("-N \"%s\"", node.Name),
		fmt.Sprintf("-H %s", req.ClickHouseHost),
		fmt.Sprintf("-P %d", req.ClickHousePort),
		fmt.Sprintf("-d %s", req.ClickHouseDB),
		fmt.Sprintf("-u %s", req.ClickHouseUser),
	}

	if req.ClickHousePassword != "" {
		params = append(params, fmt.Sprintf("-p \"%s\"", req.ClickHousePassword))
	}

	if req.LogFilePath != "" {
		params = append(params, fmt.Sprintf("-l \"%s\"", req.LogFilePath))
	}

	if req.DeployMode != "" {
		params = append(params, fmt.Sprintf("--mode %s", req.DeployMode))
	}

	paramStr := strings.Join(params, " ")

	// 生成安装命令
	return fmt.Sprintf("curl -sSL %s | sudo bash -s -- %s", repoURL, paramStr)
}

func (s *AgentDeployService) CheckAgentStatus(node *models.Node) (*AgentStatus, error) {
	sshClient, err := NewSSHClient(node)
	if err != nil {
		return &AgentStatus{
			Installed:    false,
			Running:      false,
			LastCheck:    time.Now().Format("2006-01-02 15:04:05"),
			ErrorMessage: "SSH连接失败: " + err.Error(),
		}, nil
	}
	defer sshClient.Close()

	status := &AgentStatus{
		LastCheck: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 方法1：检查服务文件是否存在
	serviceFileCheck, _ := sshClient.ExecuteCommand("test -f /etc/systemd/system/smartdns-log-agent.service && echo 'exists'")
	if strings.TrimSpace(serviceFileCheck) == "exists" {
		status.Installed = true
		status.DeployMode = "systemd"

		// 检查服务状态
		serviceStatus, _ := sshClient.ExecuteCommand("systemctl is-active smartdns-log-agent 2>/dev/null")
		status.Running = strings.TrimSpace(serviceStatus) == "active"

		// 如果服务运行中，获取详细信息
		if status.Running {
			// 获取进程信息
			processInfo, _ := sshClient.ExecuteCommand("systemctl status smartdns-log-agent --no-pager -l 2>/dev/null")
			status.ProcessInfo = processInfo

			// 检查是否开机自启
			enableStatus, _ := sshClient.ExecuteCommand("systemctl is-enabled smartdns-log-agent 2>/dev/null")
			if strings.TrimSpace(enableStatus) == "enabled" {
				status.AutoStart = true
			}
		}

		// 获取版本信息（检查二进制文件）
		version, _ := sshClient.ExecuteCommand("/usr/local/bin/smartdns-log-agent --version 2>/dev/null")
		if version != "" {
			status.Version = strings.TrimSpace(version)
		} else {
			// 如果没有 --version 参数，检查文件是否存在
			binaryCheck, _ := sshClient.ExecuteCommand("test -f /usr/local/bin/smartdns-log-agent && echo 'exists'")
			if strings.TrimSpace(binaryCheck) == "exists" {
				status.Version = "unknown"
			}
		}

		return status, nil
	}

	// 方法2：检查 Docker 部署
	dockerCheck, _ := sshClient.ExecuteCommand("test -f /opt/smartdns-log-agent/docker-compose.yml && echo 'exists'")
	if strings.TrimSpace(dockerCheck) == "exists" {
		status.Installed = true
		status.DeployMode = "docker"

		// 检查 Docker 容器状态
		containerStatus, _ := sshClient.ExecuteCommand("cd /opt/smartdns-log-agent && docker-compose ps 2>/dev/null")
		status.Running = strings.Contains(containerStatus, "Up")

		if status.Running {
			status.ProcessInfo = containerStatus
		}

		return status, nil
	}

	// 方法3：检查进程是否运行（兜底检查）
	processCheck, _ := sshClient.ExecuteCommand("pgrep -f smartdns-log-agent")
	if strings.TrimSpace(processCheck) != "" {
		status.Installed = true
		status.Running = true
		status.DeployMode = "manual"

		// 获取进程信息
		processInfo, _ := sshClient.ExecuteCommand("ps aux | grep smartdns-log-agent | grep -v grep")
		status.ProcessInfo = processInfo

		return status, nil
	}

	// 未检测到任何安装
	status.Installed = false
	status.Running = false
	return status, nil
}
func (s *AgentDeployService) UninstallAgent(node *models.Node) error {
	sshClient, err := NewSSHClient(node)
	if err != nil {
		return fmt.Errorf("SSH连接失败: %w", err)
	}
	defer sshClient.Close()

	// 生成卸载命令
	uninstallCmd := "curl -sSL https://raw.githubusercontent.com/almightyyantao/smartdns-manager/main/agent/install.sh | sudo bash -s -- --uninstall"

	output, err := sshClient.ExecuteCommandWithTimeout(uninstallCmd, s.sshTimeout)
	if err != nil {
		return fmt.Errorf("卸载失败: %w", err)
	}

	log.Printf("卸载输出: %s", output)
	return nil
}

func (s *AgentDeployService) GetAgentLogs(node *models.Node, lines string) ([]string, error) {
	sshClient, err := NewSSHClient(node)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %w", err)
	}
	defer sshClient.Close()

	// 检查部署模式
	status, err := s.CheckAgentStatus(node)
	if err != nil {
		return nil, err
	}

	var cmd string
	if status.DeployMode == "systemd" {
		cmd = fmt.Sprintf("journalctl -u smartdns-log-agent -n %s --no-pager", lines)
	} else if status.DeployMode == "docker" {
		cmd = fmt.Sprintf("cd /opt/smartdns-log-agent && docker-compose logs --tail=%s", lines)
	} else {
		return nil, fmt.Errorf("未知的部署模式")
	}

	output, err := sshClient.ExecuteCommand(cmd)
	if err != nil {
		return nil, fmt.Errorf("获取日志失败: %w", err)
	}

	return strings.Split(output, "\n"), nil
}

func (s *AgentDeployService) GetLatestVersion() string {
	// 这里可以调用 GitHub API 获取最新版本
	// 暂时返回固定版本
	return "v1.0.0"
}
