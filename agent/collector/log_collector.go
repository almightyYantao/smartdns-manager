package collector

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"smartdns-log-agent/config"
	"smartdns-log-agent/models"
	"smartdns-log-agent/sender"
	"smartdns-log-agent/utils"
)

// PositionInfo 位置信息
type PositionInfo struct {
	FilePath     string    `json:"file_path"`
	LastPosition int64     `json:"last_position"`
	LastModTime  time.Time `json:"last_mod_time"`
	FileSize     int64     `json:"file_size"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type LogCollector struct {
	cfg      *config.Config
	sender   *sender.ClickHouseSender
	parser   *utils.LogParser
	buffer   []models.DNSLogRecord
	lastSize int64

	// 统计字段
	processedLines int64
	sentRecords    int64
	errorCount     int64
	lastSentTime   time.Time
	mu             sync.RWMutex

	// 新增：位置记录
	positionFile      string
	positionInfo      *PositionInfo
	lastSavedPosition int64     // 上次保存的位置
	positionDirty     bool      // 位置是否需要保存
	lastPositionSave  time.Time // 上次保存位置的时间
}

func NewLogCollector(cfg *config.Config, sender *sender.ClickHouseSender) (*LogCollector, error) {
	parser := utils.NewLogParser()

	// 创建位置文件路径
	positionDir := "/var/lib/smartdns-agent"
	if err := os.MkdirAll(positionDir, 0755); err != nil {
		log.Printf("⚠️ 创建位置文件目录失败: %v", err)
		positionDir = "/tmp"
	}

	positionFile := filepath.Join(positionDir, fmt.Sprintf("position-node-%d.json", cfg.NodeID))

	collector := &LogCollector{
		cfg:          cfg,
		sender:       sender,
		parser:       parser,
		buffer:       make([]models.DNSLogRecord, 0, cfg.BatchSize),
		positionFile: positionFile,
	}

	// 加载位置信息
	collector.loadPosition()

	return collector, nil
}

// loadPosition 加载位置信息
func (c *LogCollector) loadPosition() {
	data, err := os.ReadFile(c.positionFile)
	if err != nil {
		log.Printf("📍 位置文件不存在或读取失败，从文件末尾开始: %v", err)
		// 设置从文件末尾开始读取
		if stat, err := os.Stat(c.cfg.LogFile); err == nil {
			c.lastSize = stat.Size()
			log.Printf("📍 从文件末尾开始读取，位置: %d", c.lastSize)
		}
		return
	}

	var pos PositionInfo
	if err := json.Unmarshal(data, &pos); err != nil {
		log.Printf("⚠️ 解析位置文件失败: %v", err)
		return
	}

	// 检查文件是否变化
	stat, err := os.Stat(c.cfg.LogFile)
	if err != nil {
		log.Printf("⚠️ 检查日志文件失败: %v", err)
		return
	}

	// 如果文件路径不匹配，重新开始
	if pos.FilePath != c.cfg.LogFile {
		log.Printf("📍 日志文件路径变化，重新开始: %s -> %s", pos.FilePath, c.cfg.LogFile)
		c.lastSize = stat.Size() // 从末尾开始
		return
	}

	// 如果文件被重新创建（修改时间更新且大小变小）
	if stat.ModTime().After(pos.LastModTime) && stat.Size() < pos.LastPosition {
		log.Printf("📍 检测到日志文件重新创建，从头开始")
		c.lastSize = 0
		return
	}

	// 如果文件大小小于记录的位置，说明文件被截断
	if stat.Size() < pos.LastPosition {
		log.Printf("📍 文件被截断，从头开始: 当前大小=%d, 记录位置=%d", stat.Size(), pos.LastPosition)
		c.lastSize = 0
		return
	}

	// 恢复位置
	c.lastSize = pos.LastPosition
	c.positionInfo = &pos
	log.Printf("📍 恢复读取位置: %d (文件: %s)", c.lastSize, c.cfg.LogFile)
}

// savePosition 保存位置信息
func (c *LogCollector) savePosition() {
	stat, err := os.Stat(c.cfg.LogFile)
	if err != nil {
		return
	}

	pos := PositionInfo{
		FilePath:     c.cfg.LogFile,
		LastPosition: c.lastSize,
		LastModTime:  stat.ModTime(),
		FileSize:     stat.Size(),
		UpdatedAt:    time.Now(),
	}

	data, err := json.Marshal(pos)
	if err != nil {
		log.Printf("⚠️ 序列化位置信息失败: %v", err)
		return
	}

	if err := os.WriteFile(c.positionFile, data, 0644); err != nil {
		log.Printf("⚠️ 保存位置文件失败: %v", err)
	}
}

func (c *LogCollector) Start(ctx context.Context) {
	log.Printf("📖 开始监控日志文件: %s (从位置: %d)", c.cfg.LogFile, c.lastSize)

	// 启动定时刷新
	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()

	// 启动位置保存定时器
	positionTicker := time.NewTicker(30 * time.Second) // 每10秒保存一次位置
	defer positionTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				c.savePosition()
				return
			case <-ticker.C:
				c.flushBuffer()
			case <-positionTicker.C:
				c.savePositionIfNeeded()
			}
		}
	}()

	// 监控日志文件
	for {
		select {
		case <-ctx.Done():
			c.flushBuffer()
			c.savePosition() // 退出前保存位置
			return
		default:
			if err := c.readNewLines(ctx); err != nil {
				log.Printf("❌ 读取日志文件失败: %v, 2秒后重试", err)

				c.mu.Lock()
				c.errorCount++
				c.mu.Unlock()

				time.Sleep(2 * time.Second)
			} else {
				// 读取成功后稍微休息一下
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}

func (c *LogCollector) readNewLines(ctx context.Context) error {
	file, err := os.Open(c.cfg.LogFile)
	if err != nil {
		return err
	}
	defer file.Close()

	// 获取文件信息
	stat, err := file.Stat()
	if err != nil {
		return err
	}

	currentSize := stat.Size()

	// 如果文件被重新创建或截断
	if currentSize < c.lastSize {
		log.Println("📝 检测到日志文件轮转或截断")
		c.lastSize = 0
	}

	// 如果文件没有增长，直接返回
	if currentSize == c.lastSize {
		return nil
	}

	// 跳到上次读取的位置
	if c.lastSize > 0 {
		_, err = file.Seek(c.lastSize, 0)
		if err != nil {
			return err
		}
	}

	scanner := bufio.NewScanner(file)
	lineCount := 0
	parsedCount := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			lineCount++

			// 更新处理行数统计
			c.mu.Lock()
			c.processedLines++
			c.mu.Unlock()

			// 解析日志行
			if record := c.parser.Parse(line, c.cfg.NodeID); record != nil {
				parsedCount++

				c.mu.Lock()
				c.buffer = append(c.buffer, *record)
				bufferLen := len(c.buffer)
				c.mu.Unlock()

				// 缓冲区满了就刷新
				if bufferLen >= c.cfg.BatchSize {
					c.flushBuffer()
				}
			}
		}
	}

	// 更新文件位置
	newPos, _ := file.Seek(0, 1)
	c.lastSize = newPos

	if lineCount > 0 {
		log.Printf("📊 处理了 %d 行新日志, 成功解析 %d 行, 位置: %d", lineCount, parsedCount, c.lastSize)
	}

	return scanner.Err()
}

func (c *LogCollector) flushBuffer() {
	c.mu.Lock()
	if len(c.buffer) == 0 {
		c.mu.Unlock()
		return
	}

	// 复制缓冲区数据
	bufferCopy := make([]models.DNSLogRecord, len(c.buffer))
	copy(bufferCopy, c.buffer)
	c.buffer = c.buffer[:0] // 清空缓冲区
	c.mu.Unlock()

	start := time.Now()
	err := c.sender.SendBatch(bufferCopy)
	duration := time.Since(start)

	c.mu.Lock()
	if err != nil {
		c.errorCount++
		c.mu.Unlock()
		return
	}
	c.sentRecords += int64(len(bufferCopy))
	c.lastSentTime = time.Now()
	c.positionDirty = true // 标记需要保存位置
	c.mu.Unlock()

	log.Printf("✅ 发送 %d 条日志到 ClickHouse, 耗时: %v", len(bufferCopy), duration)

	// 发送成功后保存位置
	c.savePosition()
}

func (c *LogCollector) savePositionIfNeeded() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.positionDirty {
		return
	}

	// 如果位置没有显著变化，跳过保存
	if c.lastSize == c.lastSavedPosition {
		return
	}

	c.positionDirty = false
	c.lastSavedPosition = c.lastSize
	c.mu.Unlock()

	c.savePosition()

	c.mu.Lock()
	c.lastPositionSave = time.Now()
}

// GetStats 获取统计信息
func (c *LogCollector) GetStats() (int64, int64, int64, time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.processedLines, c.sentRecords, c.errorCount, c.lastSentTime
}

// GetBufferSize 获取缓冲区大小
func (c *LogCollector) GetBufferSize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.buffer)
}

// GetPositionInfo 获取位置信息（用于调试）
func (c *LogCollector) GetPositionInfo() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"position_file": c.positionFile,
		"last_size":     c.lastSize,
		"position_info": c.positionInfo,
	}
}
