package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"smartdns-manager/models"
)

// GetAgentPort 获取 Agent API 端口
func GetAgentPort(node *models.Node) int {
	// 可以从节点配置中获取，或使用默认端口
	if node.AgentAPIPort > 0 {
		return node.AgentAPIPort
	}
	return 8888 // 默认端口
}

// CallAgentAPI 调用 Agent API（无返回数据）
func CallAgentAPI(method, url string, data interface{}) error {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("序列化数据失败: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Agent 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Agent 返回错误状态 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// 解析响应检查是否成功
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		message := "未知错误"
		if msg, ok := result["message"].(string); ok {
			message = msg
		}
		return fmt.Errorf("Agent 操作失败: %s", message)
	}

	return nil
}

// CallAgentAPIWithResponse 调用 Agent API（返回响应数据）
func CallAgentAPIWithResponse(method, url string, data interface{}) (map[string]interface{}, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("序列化数据失败: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	if data != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Agent 失败: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		message := "未知错误"
		if msg, ok := result["message"].(string); ok {
			message = msg
		}
		return nil, fmt.Errorf("Agent 返回错误状态 %d: %s", resp.StatusCode, message)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		message := "未知错误"
		if msg, ok := result["message"].(string); ok {
			message = msg
		}
		return nil, fmt.Errorf("Agent 操作失败: %s", message)
	}

	return result, nil
}
