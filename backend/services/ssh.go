package services

import (
	"bytes"
	"context"
	"fmt"
	_ "io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"smartdns-manager/models"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/proxy"
)

type SSHClient struct {
	client     *ssh.Client
	jumpClient *ssh.Client
}

func NewSSHClient(node *models.Node) (*SSHClient, error) {
	var auth []ssh.AuthMethod

	if node.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(node.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	} else if node.Password != "" {
		auth = append(auth, ssh.Password(node.Password))
	} else {
		return nil, fmt.Errorf("no authentication method provided")
	}

	config := &ssh.ClientConfig{
		User:            node.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	//if node.ProxyConfig != nil && node.ProxyConfig.Enabled {
	//	return NewSSHClientWithProxy(node, config)
	//}

	addr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &SSHClient{client: client}, nil
}

func NewSSHClientWithProxy(node *models.Node, config *ssh.ClientConfig) (*SSHClient, error) {
	proxyConfig := node.ProxyConfig

	switch proxyConfig.ProxyType {
	case "socks5":
		return newSSHClientWithSOCKS5(node, config, proxyConfig)
	case "http":
		return newSSHClientWithHTTP(node, config, proxyConfig)
	case "ssh":
		return newSSHClientWithJumpHost(node, config, proxyConfig)
	default:
		return nil, fmt.Errorf("不支持的代理类型: %s", proxyConfig.ProxyType)
	}
}

// SOCKS5代理连接
func newSSHClientWithSOCKS5(node *models.Node, config *ssh.ClientConfig, proxyConfig *models.ProxyConfig) (*SSHClient, error) {
	// 创建SOCKS5代理地址
	proxyAddr := fmt.Sprintf("%s:%d", proxyConfig.ProxyHost, proxyConfig.ProxyPort)

	var auth *proxy.Auth
	if proxyConfig.ProxyUser != "" {
		auth = &proxy.Auth{
			User:     proxyConfig.ProxyUser,
			Password: proxyConfig.ProxyPass,
		}
	}

	// 创建SOCKS5拨号器
	dialer, err := proxy.SOCKS5("tcp", proxyAddr, auth, proxy.Direct)
	if err != nil {
		return nil, fmt.Errorf("创建SOCKS5代理失败: %w", err)
	}

	// 通过代理连接目标主机
	targetAddr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	conn, err := dialer.Dial("tcp", targetAddr)
	if err != nil {
		return nil, fmt.Errorf("通过SOCKS5代理连接失败: %w", err)
	}

	// 建立SSH连接
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, targetAddr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SSH握手失败: %w", err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	return &SSHClient{client: client}, nil
}

func newSSHClientWithHTTP(node *models.Node, config *ssh.ClientConfig, proxy *models.ProxyConfig) (*SSHClient, error) {
	// 创建HTTP代理连接
	proxyURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", proxy.ProxyHost, proxy.ProxyPort),
	}

	if proxy.ProxyUser != "" {
		proxyURL.User = url.UserPassword(proxy.ProxyUser, proxy.ProxyPass)
	}

	// 使用HTTP CONNECT方法
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		Dial:  dialer.Dial,
	}

	client := &http.Client{Transport: transport}

	// 建立CONNECT隧道
	targetAddr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	req, err := http.NewRequest("CONNECT", targetAddr, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP代理连接失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP代理返回错误状态码: %d", resp.StatusCode)
	}

	// 获取底层连接
	conn := resp.Body.(net.Conn) // 需要类型断言，实际实现可能需要更复杂的处理

	// 建立SSH连接
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, targetAddr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SSH握手失败: %w", err)
	}

	sshClient := ssh.NewClient(sshConn, chans, reqs)
	return &SSHClient{client: sshClient}, nil
}

// SSH跳板机连接
func newSSHClientWithJumpHost(node *models.Node, config *ssh.ClientConfig, proxy *models.ProxyConfig) (*SSHClient, error) {
	// 连接跳板机
	jumpConfig := &ssh.ClientConfig{
		User: proxy.JumpUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(proxy.JumpPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	jumpAddr := fmt.Sprintf("%s:%d", proxy.JumpHost, proxy.JumpPort)
	jumpClient, err := ssh.Dial("tcp", jumpAddr, jumpConfig)
	if err != nil {
		return nil, fmt.Errorf("连接跳板机失败: %w", err)
	}

	// 通过跳板机连接目标主机
	targetAddr := fmt.Sprintf("%s:%d", node.Host, node.Port)
	conn, err := jumpClient.Dial("tcp", targetAddr)
	if err != nil {
		jumpClient.Close()
		return nil, fmt.Errorf("通过跳板机连接目标主机失败: %w", err)
	}

	// 建立SSH连接
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, targetAddr, config)
	if err != nil {
		conn.Close()
		jumpClient.Close()
		return nil, fmt.Errorf("SSH握手失败: %w", err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	return &SSHClient{
		client:     client,
		jumpClient: jumpClient, // 保存跳板机连接，用于后续关闭
	}, nil
}

func (c *SSHClient) Close() error {
	var err error
	if c.client != nil {
		err = c.client.Close()
	}
	if c.jumpClient != nil {
		if jumpErr := c.jumpClient.Close(); jumpErr != nil && err == nil {
			err = jumpErr
		}
	}
	return err
}

func (c *SSHClient) ExecuteCommand(cmd string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	if err := session.Run(cmd); err != nil {
		return "", fmt.Errorf("command failed: %s, error: %w", stderr.String(), err)
	}

	return stdout.String(), nil
}

func (c *SSHClient) ReadFile(path string) (string, error) {
	cmd := fmt.Sprintf("cat %s", path)
	return c.ExecuteCommand(cmd)
}

func (c *SSHClient) WriteFile(path, content string) error {
	// 创建临时文件
	tmpFile := fmt.Sprintf("/tmp/smartdns-config-%d", time.Now().Unix())

	// 写入临时文件
	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdin = bytes.NewBufferString(content)
	if err := session.Run(fmt.Sprintf("cat > %s", tmpFile)); err != nil {
		return err
	}

	// 移动到目标位置（需要 sudo）
	_, err = c.ExecuteCommand(fmt.Sprintf("sudo mv %s %s", tmpFile, path))
	return err
}

func (c *SSHClient) RestartService(serviceName string) error {
	_, err := c.ExecuteCommand(fmt.Sprintf("sudo systemctl restart %s", serviceName))
	return err
}

func (c *SSHClient) GetServiceStatus(serviceName string) (bool, error) {
	output, err := c.ExecuteCommand(fmt.Sprintf("systemctl is-active %s", serviceName))
	if err != nil {
		return false, nil
	}
	return output == "active\n", nil
}

func (c *SSHClient) GetSystemInfo() (*models.NodeStatus, error) {
	status := &models.NodeStatus{
		LastChecked: time.Now(),
		IsOnline:    true,
	}

	// 获取服务状态
	serviceUp, _ := c.GetServiceStatus("smartdns")
	status.ServiceUp = serviceUp

	// 获取 CPU 使用率
	cpuOutput, _ := c.ExecuteCommand("top -bn1 | grep 'Cpu(s)' | awk '{print $2}'")
	fmt.Sscanf(cpuOutput, "%f", &status.CPUUsage)

	// 获取内存使用率
	memOutput, _ := c.ExecuteCommand("free | grep Mem | awk '{print ($3/$2) * 100.0}'")
	fmt.Sscanf(memOutput, "%f", &status.MemoryUsage)

	// 获取磁盘使用率
	diskOutput, _ := c.ExecuteCommand("df -h / | tail -1 | awk '{print $5}' | sed 's/%//'")
	fmt.Sscanf(diskOutput, "%f", &status.DiskUsage)

	// 获取版本
	versionOutput, _ := c.ExecuteCommand("smartdns -v 2>&1 | head -1")
	status.Version = versionOutput

	return status, nil
}

func (c *SSHClient) GetLogs(lines int) (string, error) {
	cmd := fmt.Sprintf("sudo tail -n %d /var/log/smartdns.log", lines)
	return c.ExecuteCommand(cmd)
}

func (c *SSHClient) CreateBackup(configPath string) (string, error) {
	backupPath := fmt.Sprintf("%s.backup-%d", configPath, time.Now().Unix())
	cmd := fmt.Sprintf("sudo cp %s %s", configPath, backupPath)
	_, err := c.ExecuteCommand(cmd)
	if err != nil {
		return "", err
	}
	return backupPath, nil
}

func (c *SSHClient) ListBackups(configPath string) ([]string, error) {
	cmd := fmt.Sprintf("ls -t %s.backup-* 2>/dev/null", configPath)
	output, err := c.ExecuteCommand(cmd)
	if err != nil {
		return []string{}, nil
	}

	var backups []string
	for _, line := range bytes.Split([]byte(output), []byte("\n")) {
		if len(line) > 0 {
			backups = append(backups, string(line))
		}
	}
	return backups, nil
}

func (client *SSHClient) ExecuteCommandWithTimeout(command string, timeout time.Duration) (string, error) {
	session, err := client.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 创建结果通道
	type result struct {
		output string
		err    error
	}
	resultChan := make(chan result, 1)

	// 在 goroutine 中执行命令
	go func() {
		output, err := session.CombinedOutput(command)
		resultChan <- result{string(output), err}
	}()

	// 等待结果或超时
	select {
	case res := <-resultChan:
		// 即使有错误也返回输出，这样可以看到错误信息
		if res.err != nil {
			log.Printf("SSH命令执行出错: %v", res.err)
			log.Printf("命令输出: %s", res.output)
		}
		return res.output, res.err
	case <-ctx.Done():
		session.Signal(ssh.SIGTERM)
		return "", fmt.Errorf("命令执行超时")
	}
}
