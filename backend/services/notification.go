package services

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"smartdns-manager/database"
	"smartdns-manager/models"
)

type NotificationService struct{}

func NewNotificationService() *NotificationService {
	return &NotificationService{}
}

// SendNotification 发送通知
func (s *NotificationService) SendNotification(nodeID uint, eventType, title, content string) error {
	// 获取节点信息（如果 nodeID > 0）
	var node models.Node
	if nodeID > 0 {
		if err := database.DB.First(&node, nodeID).Error; err != nil {
			log.Printf("获取节点信息失败: %v", err)
			// 即使节点不存在，也继续发送通知（使用默认信息）
			node.ID = nodeID
			node.Name = "未知节点"
			node.Host = "N/A"
		}
	} else {
		// nodeID = 0 表示全局通知，使用默认值
		node.ID = 0
		node.Name = "系统全局"
		node.Host = "N/A"
	}

	// 获取所有启用的通知渠道（包括节点专属和全局渠道）
	var channels []models.NotificationChannel
	if nodeID > 0 {
		// 查询该节点的专属渠道 + 全局渠道
		database.DB.Where("(node_id = ? OR node_id = 0) AND enabled = ?", nodeID, true).Find(&channels)
	} else {
		// 只查询全局渠道
		database.DB.Where("node_id = 0 AND enabled = ?", true).Find(&channels)
	}

	if len(channels) == 0 {
		log.Printf("没有可用的通知渠道 (nodeID: %d)", nodeID)
		return nil
	}

	// 过滤订阅了该事件的渠道
	subscribedChannels := s.filterChannelsByEvent(channels, eventType)

	if len(subscribedChannels) == 0 {
		log.Printf("没有订阅该事件的渠道 (event: %s, nodeID: %d)", eventType, nodeID)
		return nil
	}

	// 发送到所有订阅的渠道
	for _, channel := range subscribedChannels {
		go s.sendToChannel(&channel, &node, eventType, title, content)
	}

	return nil
}

// sendToChannel 发送到指定渠道
func (s *NotificationService) sendToChannel(channel *models.NotificationChannel, node *models.Node, eventType, title, content string) {
	log.Printf("发送通知到 %s (%s): %s", channel.Name, channel.Type, title)

	var err error
	var payload interface{}

	switch channel.Type {
	case "wechat":
		payload = s.buildWeChatPayload(node, title, content)
	case "dingtalk":
		payload = s.buildDingTalkPayload(node, title, content, channel.Secret)
	case "feishu":
		payload = s.buildFeishuPayload(node, title, content, channel.Secret)
	case "slack":
		payload = s.buildSlackPayload(node, title, content)
	default:
		log.Printf("不支持的通知类型: %s", channel.Type)
		return
	}

	// 发送 HTTP 请求
	err = s.sendWebhook(channel.WebhookURL, payload)

	// 记录日志
	notifLog := models.NotificationLog{
		ChannelID: channel.ID,
		NodeID:    node.ID,
		EventType: eventType,
		Title:     title,
		Content:   content,
		SentAt:    time.Now(),
	}

	if err != nil {
		notifLog.Status = "failed"
		notifLog.Error = err.Error()
		log.Printf("发送通知失败: %v", err)
	} else {
		notifLog.Status = "success"
		log.Printf("通知发送成功")
	}

	database.DB.Create(&notifLog)
}

// buildWeChatPayload 构建企业微信消息
func (s *NotificationService) buildWeChatPayload(node *models.Node, title, content string) interface{} {
	return map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fmt.Sprintf("### %s\n\n"+
				"**节点**: %s\n"+
				"**主机**: %s\n"+
				"**时间**: %s\n\n"+
				"%s",
				title,
				node.Name,
				node.Host,
				time.Now().Format("2006-01-02 15:04:05"),
				content,
			),
		},
	}
}

// buildDingTalkPayload 构建钉钉消息
func (s *NotificationService) buildDingTalkPayload(node *models.Node, title, content string, secret string) interface{} {
	timestamp := time.Now().UnixMilli()
	sign := ""

	if secret != "" {
		sign = s.generateDingTalkSign(timestamp, secret)
	}

	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": title,
			"text": fmt.Sprintf("### %s\n\n"+
				"- **节点**: %s\n"+
				"- **主机**: %s\n"+
				"- **时间**: %s\n\n"+
				"%s",
				title,
				node.Name,
				node.Host,
				time.Now().Format("2006-01-02 15:04:05"),
				content,
			),
		},
	}

	if sign != "" {
		payload["timestamp"] = timestamp
		payload["sign"] = sign
	}

	return payload
}

// buildFeishuPayload 构建飞书消息
func (s *NotificationService) buildFeishuPayload(node *models.Node, title, content string, secret string) interface{} {
	timestamp := time.Now().Unix()
	sign := ""

	if secret != "" {
		sign = s.generateFeishuSign(timestamp, secret)
	}

	payload := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"content": title,
					"tag":     "plain_text",
				},
				"template": "blue",
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"fields": []map[string]interface{}{
						{
							"is_short": true,
							"text": map[string]interface{}{
								"content": fmt.Sprintf("**节点**: %s", node.Name),
								"tag":     "lark_md",
							},
						},
						{
							"is_short": true,
							"text": map[string]interface{}{
								"content": fmt.Sprintf("**主机**: %s", node.Host),
								"tag":     "lark_md",
							},
						},
					},
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"content": content,
						"tag":     "lark_md",
					},
				},
				{
					"tag": "div",
					"text": map[string]interface{}{
						"content": fmt.Sprintf("⏰ %s", time.Now().Format("2006-01-02 15:04:05")),
						"tag":     "lark_md",
					},
				},
			},
		},
	}

	if sign != "" {
		payload["timestamp"] = fmt.Sprintf("%d", timestamp)
		payload["sign"] = sign
	}

	return payload
}

// buildSlackPayload 构建 Slack 消息
func (s *NotificationService) buildSlackPayload(node *models.Node, title, content string) interface{} {
	return map[string]interface{}{
		"text": title,
		"blocks": []map[string]interface{}{
			{
				"type": "header",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": title,
				},
			},
			{
				"type": "section",
				"fields": []map[string]interface{}{
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*节点:*\n%s", node.Name),
					},
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("*主机:*\n%s", node.Host),
					},
				},
			},
			{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": content,
				},
			},
			{
				"type": "context",
				"elements": []map[string]interface{}{
					{
						"type": "mrkdwn",
						"text": fmt.Sprintf("⏰ %s", time.Now().Format("2006-01-02 15:04:05")),
					},
				},
			},
		},
	}
}

// sendWebhook 发送 Webhook 请求
func (s *NotificationService) sendWebhook(url string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return fmt.Errorf("webhook 返回错误: %d, %s", resp.StatusCode, string(body))
	}

	log.Printf("Webhook 响应: %s", string(body))
	return nil
}

// generateDingTalkSign 生成钉钉签名
func (s *NotificationService) generateDingTalkSign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// generateFeishuSign 生成飞书签名
func (s *NotificationService) generateFeishuSign(timestamp int64, secret string) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	h := hmac.New(sha256.New, []byte(stringToSign))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// filterChannelsByEvent 过滤订阅了特定事件的渠道
func (s *NotificationService) filterChannelsByEvent(channels []models.NotificationChannel, eventType string) []models.NotificationChannel {
	result := []models.NotificationChannel{}

	for _, channel := range channels {
		// 如果 events 为空，表示订阅所有事件
		if channel.Events == "" || channel.Events == "[]" {
			result = append(result, channel)
			continue
		}

		// 解析订阅的事件列表
		var events []string
		if err := json.Unmarshal([]byte(channel.Events), &events); err != nil {
			log.Printf("解析事件列表失败: %v", err)
			continue
		}

		// 检查是否订阅了该事件
		for _, event := range events {
			if event == eventType || event == "*" {
				result = append(result, channel)
				break
			}
		}
	}

	return result
}
