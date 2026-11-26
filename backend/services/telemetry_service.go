package services

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"smartdns-manager/config"
	"smartdns-manager/models"
)

// TelemetryService 遥测服务
type TelemetryService struct {
	db          *gorm.DB
	config      *config.Config
	client      *http.Client
	scheduledMu sync.Mutex
}

// NewTelemetryService 创建遥测服务
func NewTelemetryService(db *gorm.DB, config *config.Config) (*TelemetryService, error) {
	// 创建自定义的HTTP客户端，优化超时和连接设置
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second, // 连接超时
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       60 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // 总体请求超时
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 { // 最多允许3次重定向
				return fmt.Errorf("stopped after 3 redirects")
			}
			return nil
		},
	}

	return &TelemetryService{
		db:     db,
		config: config,
		client: client,
	}, nil
}

// CheckTargets 检查遥测目标
func (s *TelemetryService) CheckTargets(ctx context.Context, config models.TelemetryConfig) (string, error) {
	s.scheduledMu.Lock()
	defer s.scheduledMu.Unlock()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	// 获取要检查的目标
	var targets []models.TelemetryTarget
	query := s.db.Where("enabled = ?", true)
	
	if len(config.Targets) > 0 {
		query = query.Where("id IN ?", config.Targets)
	} else if !config.IncludeDNS {
		query = query.Where("type <> ?", "dns")
	}
	
	if err := query.Find(&targets).Error; err != nil {
		return "", fmt.Errorf("查询遥测目标失败: %w", err)
	}
	
	if len(targets) == 0 {
		return "没有找到启用的遥测目标", nil
	}
	
	successCount := 0
	var results []string
	
	for _, target := range targets {
		log.Printf("🎯 开始检查遥测目标: %s (类型: %s, 地址: %s)",
			target.Name, target.Type, target.Target)
		
		result, err := s.CheckSingleTarget(ctx, target)
		
		// 构建结果描述
		if err != nil {
			errorMsg := err.Error()
			if len(errorMsg) > 100 {
				errorMsg = errorMsg[:100] + "..."
			}
			results = append(results, fmt.Sprintf("%s: ❌失败 (%s)", target.Name, errorMsg))
			log.Printf("❌ 遥测检查失败 [%s]: %v", target.Name, err)
		} else {
			successCount++
			statusIcon := "✅"
			if result.Latency > 1000 {
				statusIcon = "⚠️" // 延迟超过1秒用警告图标
			}
			results = append(results, fmt.Sprintf("%s: %s成功 (延迟: %dms)",
				target.Name, statusIcon, result.Latency))
			log.Printf("✅ 遥测检查成功 [%s]: 延迟 %dms", target.Name, result.Latency)
		}

		if ctx.Err() != nil {
			return strings.Join(results, "; "), ctx.Err()
		}
		if target.Type == "dns" {
			if err := s.saveScheduledDNSResult(target, result); err != nil {
				return strings.Join(results, "; "), err
			}
		} else {
			if saveErr := s.saveResult(target, result, err); saveErr != nil {
				return strings.Join(results, "; "), saveErr
			}
			if statsErr := s.updateTargetStats(target, result, err); statsErr != nil {
				return strings.Join(results, "; "), statsErr
			}
		}

		// 检查上下文是否取消
		select {
		case <-ctx.Done():
			log.Printf("⚠️ 遥测检查被取消，已完成 %d/%d 个目标", len(results), len(targets))
			return fmt.Sprintf("遥测检查被取消: 已完成 %d/%d, 成功 %d",
				len(results), len(targets), successCount), ctx.Err()
		default:
		}
	}
	
	// 清理过期结果
	if config.ResultRetention > 0 {
		if err := s.cleanupResults(config.ResultRetention); err != nil {
			log.Printf("❌ 清理遥测结果失败: %v", err)
		}
	}
	
	summary := fmt.Sprintf("遥测检查完成: 成功 %d/%d", successCount, len(targets))
	if len(results) > 0 {
		summary += "; 详情: " + strings.Join(results, "; ")
	}

	if successCount < len(targets) {
		return summary, fmt.Errorf("%d 个遥测目标检测失败", len(targets)-successCount)
	}
	return summary, nil
}

// CheckSingleTarget 检查单个目标（公开方法，供外部调用）
func (s *TelemetryService) CheckSingleTarget(ctx context.Context, target models.TelemetryTarget) (*models.TelemetryResult, error) {
	result := &models.TelemetryResult{
		TargetID:  target.ID,
		CheckedAt: time.Now(),
	}
	
	if target.Type == "dns" {
		if err := ValidateDNSProbe(target); err != nil {
			result.Error = err.Error()
			return result, err
		}
		timeout := time.Duration(target.Timeout) * time.Millisecond
		if timeout <= 0 || timeout > 5*time.Second {
			timeout = 2*time.Second
		}
		probeCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		start := time.Now()
		response, err := probeDNS(probeCtx, net.JoinHostPort(strings.TrimSpace(target.Target), "53"), target)
		result.Response = response
		result.Latency = time.Since(start).Milliseconds()
		result.Success = err == nil
		if err != nil {
			result.Error = err.Error()
		}
		return result, err
	}

	// 优化超时设置，最小3秒，最大30秒
	timeout := time.Duration(target.Timeout) * time.Millisecond
	if timeout == 0 || timeout < 3*time.Second {
		timeout = 15 * time.Second // 默认15秒
	}
	if timeout > 30*time.Second {
		timeout = 30 * time.Second // 最大30秒
	}
	
	log.Printf("🔍 开始检测遥测目标 [%s] 类型: %s, 地址: %s, 超时: %v",
		target.Name, target.Type, target.Target, timeout)
	
	// 创建带超时的上下文
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	startTime := time.Now()
	var err error
	
	switch strings.ToLower(target.Type) {
	case "ping":
		err = s.pingCheck(checkCtx, target.Target)
		result.Latency = time.Since(startTime).Milliseconds()
		result.Success = err == nil
		if err != nil {
			result.Error = err.Error()
			log.Printf("❌ PING检测失败 [%s]: %v (耗时: %dms)", target.Name, err, result.Latency)
			return result, err
		}
		log.Printf("✅ PING检测成功 [%s]: 延迟 %dms", target.Name, result.Latency)
		
	case "http", "https":
		resp, err := s.httpCheck(checkCtx, target.Target)
		result.Latency = time.Since(startTime).Milliseconds()
		result.Success = err == nil
		if err != nil {
			result.Error = err.Error()
			log.Printf("❌ HTTP检测失败 [%s]: %v (耗时: %dms)", target.Name, err, result.Latency)
			return result, err
		}
		result.Response = fmt.Sprintf("HTTP %d", resp.StatusCode)
		log.Printf("✅ HTTP检测成功 [%s]: %s (延迟: %dms)", target.Name, result.Response, result.Latency)
		
	case "tcp":
		err = s.tcpCheck(checkCtx, target.Target)
		result.Latency = time.Since(startTime).Milliseconds()
		result.Success = err == nil
		if err != nil {
			result.Error = err.Error()
			log.Printf("❌ TCP检测失败 [%s]: %v (耗时: %dms)", target.Name, err, result.Latency)
			return result, err
		}
		log.Printf("✅ TCP检测成功 [%s]: 延迟 %dms", target.Name, result.Latency)
		
	default:
		result.Success = false
		result.Error = fmt.Sprintf("不支持的检查类型: %s", target.Type)
		log.Printf("❌ 不支持的检查类型 [%s]: %s", target.Name, target.Type)
		return result, fmt.Errorf("不支持的检查类型: %s", target.Type)
	}
	
	return result, nil
}

// pingCheck PING检查（基于网络连通性的检查，不是真正的ICMP ping）
func (s *TelemetryService) pingCheck(ctx context.Context, target string) error {
	host := target
	
	// 如果目标包含端口，提取主机部分（但ping不应该有端口）
	if strings.Contains(target, ":") {
		parts := strings.Split(target, ":")
		if len(parts) >= 2 {
			host = parts[0]
			log.Printf("⚠️ PING目标不应包含端口，已提取主机部分: %s", host)
		}
	}
	
	log.Printf("🏓 开始PING检查（网络连通性测试）: %s", host)
	
	// 由于Go程序通常无法发送ICMP包（需要特殊权限），我们使用多种方式测试连通性：
	// 1. 首先尝试DNS解析
	// 2. 然后尝试常用端口的TCP连接
	
	// 1. DNS解析测试
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 5 * time.Second,
			}
			return d.DialContext(ctx, network, address)
		},
	}
	
	ips, err := resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return fmt.Errorf("DNS解析失败: %w", err)
	}
	
	if len(ips) == 0 {
		return fmt.Errorf("DNS解析未返回IP地址")
	}
	
	log.Printf("✅ DNS解析成功: %s -> %v", host, ips[0].IP)
	
	// 2. 尝试多个常用端口的TCP连接来测试网络连通性
	commonPorts := []string{"80", "443", "22", "53", "8080", "8443", "21", "23", "25", "110", "143", "993", "995"}
	
	dialer := &net.Dialer{
		Timeout: 3 * time.Second, // 每个端口3秒超时
	}
	
	var lastErr error
	for _, port := range commonPorts {
		select {
		case <-ctx.Done():
			return fmt.Errorf("检查超时: %w", ctx.Err())
		default:
		}
		
		addr := net.JoinHostPort(host, port)
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err == nil {
			conn.Close()
			log.Printf("✅ 网络连通性确认 %s (通过端口 %s)", host, port)
			return nil
		}
		lastErr = err
		
		// 如果是明确的连接拒绝错误，说明主机是可达的
		if strings.Contains(err.Error(), "connection refused") ||
		   strings.Contains(err.Error(), "refused") {
			log.Printf("✅ 网络连通性确认 %s (端口 %s 拒绝连接，但主机可达)", host, port)
			return nil
		}
	}
	
	// 如果所有端口都失败，返回最后的错误
	return fmt.Errorf("网络连通性测试失败，所有常用端口均不可达: %w", lastErr)
}

// httpCheck HTTP检查（带重试机制和动态超时）
func (s *TelemetryService) httpCheck(ctx context.Context, target string) (*http.Response, error) {
	// 确保URL有协议前缀
	url := target
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		url = "http://" + target
	}
	
	log.Printf("🌐 发起HTTP请求: %s", url)
	
	// 从上下文获取超时时间并创建专用的HTTP客户端
	deadline, hasDeadline := ctx.Deadline()
	var timeout time.Duration = 30 * time.Second // 默认超时
	
	if hasDeadline {
		timeout = time.Until(deadline)
		if timeout < time.Second {
			timeout = time.Second // 最小1秒
		}
	}
	
	// 创建专用的HTTP客户端，使用上下文超时
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   timeout / 2, // 连接超时设为总超时的一半
			KeepAlive: -1,          // 禁用keep-alive
		}).DialContext,
		TLSHandshakeTimeout:   timeout / 3,    // TLS握手超时
		ResponseHeaderTimeout: timeout / 2,    // 响应头超时
		ExpectContinueTimeout: time.Second,
		DisableKeepAlives:     true,           // 禁用连接复用
		MaxIdleConns:          0,
	}
	
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 2 { // 最多允许2次重定向
				return fmt.Errorf("stopped after 2 redirects")
			}
			return nil
		},
	}
	
	log.Printf("🕒 HTTP客户端超时设置: %v", timeout)
	
	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil) // 使用HEAD请求减少数据传输
	if err != nil {
		return nil, fmt.Errorf("创建HTTP请求失败: %w", err)
	}
	
	// 设置请求头
	req.Header.Set("User-Agent", "SmartDNS-Manager-Telemetry/1.0")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Connection", "close") // 避免连接复用
	
	// 执行请求
	resp, err := client.Do(req)
	if err != nil {
		// 如果HEAD请求失败，尝试GET请求
		if strings.Contains(err.Error(), "Method Not Allowed") ||
		   strings.Contains(err.Error(), "405") ||
		   strings.Contains(err.Error(), "method not allowed") {
			log.Printf("⚠️ HEAD请求失败，尝试GET请求: %s", url)
			req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				return nil, fmt.Errorf("创建GET请求失败: %w", err)
			}
			req.Header.Set("User-Agent", "SmartDNS-Manager-Telemetry/1.0")
			req.Header.Set("Accept", "*/*")
			req.Header.Set("Connection", "close")
			
			resp, err = client.Do(req)
		}
		
		if err != nil {
			return nil, fmt.Errorf("HTTP请求失败: %w", err)
		}
	}
	
	log.Printf("✅ HTTP响应: %d %s", resp.StatusCode, resp.Status)
	return resp, nil
}

// tcpCheck TCP连接检查（增强版，支持动态超时）
func (s *TelemetryService) tcpCheck(ctx context.Context, target string) error {
	log.Printf("🔗 进行TCP连接检查: %s", target)
	
	// 从上下文获取超时时间
	deadline, hasDeadline := ctx.Deadline()
	var timeout time.Duration = 10 * time.Second // 默认超时
	
	if hasDeadline {
		timeout = time.Until(deadline)
		if timeout < time.Second {
			timeout = time.Second // 最小1秒
		}
	}
	
	log.Printf("🕒 TCP连接超时设置: %v", timeout)
	
	// 创建连接器，使用动态超时
	dialer := &net.Dialer{
		Timeout:   timeout,  // 使用从上下文获取的超时时间
		KeepAlive: -1,       // 禁用keep-alive
	}
	
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return fmt.Errorf("TCP连接失败 %s: %w", target, err)
	}
	defer conn.Close()
	
	// 尝试写入一些数据来验证连接质量（但要考虑剩余时间）
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		// 设置写超时，不超过剩余时间的一半
		writeTimeout := 2 * time.Second
		if hasDeadline {
			remaining := time.Until(deadline)
			if remaining > 0 && remaining < writeTimeout {
				writeTimeout = remaining / 2
			}
		}
		
		if writeTimeout > 100*time.Millisecond {
			tcpConn.SetWriteDeadline(time.Now().Add(writeTimeout))
			
			// 发送简单的数据包测试连接
			_, writeErr := tcpConn.Write([]byte("test"))
			if writeErr != nil {
				log.Printf("⚠️ TCP写入测试失败（但连接成功）: %v", writeErr)
			}
		}
	}
	
	log.Printf("✅ TCP连接成功: %s", target)
	return nil
}

// saveResult 保存检查结果（增强版错误处理）
func (s *TelemetryService) saveResult(target models.TelemetryTarget, result *models.TelemetryResult, checkErr error) error {
	if result == nil {
		result = &models.TelemetryResult{
			TargetID:  target.ID,
			Success:   false,
			CheckedAt: time.Now(),
			Latency:   0,
		}
		if checkErr != nil {
			// 限制错误信息长度，避免数据库字段溢出
			errorMsg := checkErr.Error()
			if len(errorMsg) > 1000 {
				errorMsg = errorMsg[:1000] + "... (截断)"
			}
			result.Error = errorMsg
		}
	}
	
	// 验证结果数据
	if result.TargetID == 0 {
		result.TargetID = target.ID
	}
	
	// 确保延迟不为负数
	if result.Latency < 0 {
		result.Latency = 0
	}
	
	// 保存到数据库
	if err := s.db.Create(result).Error; err != nil {
		return fmt.Errorf("数据库保存失败: %w", err)
	}
	
	log.Printf("💾 遥测结果已保存 - 目标: %s, 成功: %t, 延迟: %dms",
		target.Name, result.Success, result.Latency)
	
	return nil
}

// updateTargetStats 更新目标统计信息（增强版）
func (s *TelemetryService) updateTargetStats(target models.TelemetryTarget, result *models.TelemetryResult, checkErr error) error {
	now := time.Now()
	updates := map[string]interface{}{
		"last_check_at": &now,
		"check_count":   gorm.Expr("check_count + 1"),
	}
	
	if checkErr == nil && result != nil && result.Success {
		// 成功的情况
		updates["last_latency"] = result.Latency
		updates["last_status"] = true
		updates["success_count"] = gorm.Expr("success_count + 1")
		
		// 计算平均延迟（只计算成功的结果）
		var avgLatency float64
		if err := s.db.Model(&models.TelemetryResult{}).
			Where("target_id = ? AND success = ?", target.ID, true).
			Select("AVG(latency)").Scan(&avgLatency); err != nil {
			log.Printf("⚠️ 计算平均延迟失败 [%s]: %v", target.Name, err)
		} else {
			updates["avg_latency"] = avgLatency
		}
		
		log.Printf("📈 更新成功统计 [%s]: 延迟 %dms, 平均延迟 %.1fms",
			target.Name, result.Latency, avgLatency)
	} else {
		// 失败的情况
		updates["last_status"] = false
		if result != nil {
			updates["last_latency"] = result.Latency
		}
		
		log.Printf("📉 更新失败统计 [%s]: %v", target.Name, checkErr)
	}
	
	// 执行数据库更新
	if err := s.db.Model(&target).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新目标统计失败: %w", err)
	}
	
	return nil
}

// cleanupResults 清理过期结果（增强版）
func (s *TelemetryService) cleanupResults(retentionDays int) error {
	if retentionDays <= 0 {
		log.Printf("⚠️ 跳过结果清理: 保留天数设置无效 (%d)", retentionDays)
		return nil
	}
	
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	log.Printf("🗑️ 开始清理 %s 之前的遥测结果", cutoff.Format("2006-01-02 15:04:05"))
	
	// 先查询将要删除的记录数
	var countToDelete int64
	if err := s.db.Model(&models.TelemetryResult{}).
		Where("created_at < ? AND target_id NOT IN (?)", cutoff, s.db.Unscoped().Model(&models.TelemetryTarget{}).Select("id").Where("type = ?", "dns")).
		Count(&countToDelete).Error; err != nil {
		return fmt.Errorf("查询待删除记录数失败: %w", err)
	}
	
	if countToDelete == 0 {
		log.Printf("✅ 无需清理遥测结果: 没有过期记录")
		return nil
	}
	
	// 执行删除
	result := s.db.Where("created_at < ? AND target_id NOT IN (?)", cutoff, s.db.Unscoped().Model(&models.TelemetryTarget{}).Select("id").Where("type = ?", "dns")).Delete(&models.TelemetryResult{})
	if result.Error != nil {
		return fmt.Errorf("删除过期记录失败: %w", result.Error)
	}
	
	log.Printf("🗑️ 清理过期遥测结果完成: 预计删除 %d 条, 实际删除 %d 条记录",
		countToDelete, result.RowsAffected)
	
	return nil
}

// GetTargetStats 获取目标统计信息
func (s *TelemetryService) GetTargetStats(targetID uint) (*models.TelemetryStats, error) {
	var target models.TelemetryTarget
	if err := s.db.First(&target, targetID).Error; err != nil {
		return nil, fmt.Errorf("查询目标失败: %w", err)
	}
	
	stats := &models.TelemetryStats{
		TargetID:      targetID,
		TargetName:    target.Name,
		CheckCount:    target.CheckCount,
		SuccessCount:  target.SuccessCount,
		LastCheckAt:   target.LastCheckAt,
		LastLatency:   target.LastLatency,
		AvgLatency:    target.AvgLatency,
		LastStatus:    target.LastStatus,
	}
	
	if target.CheckCount > 0 {
		stats.SuccessRate = float64(target.SuccessCount) / float64(target.CheckCount) * 100
	}
	
	return stats, nil
}