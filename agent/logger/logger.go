package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Logger struct {
	logDir      string
	maxDays     int
	file        *os.File
	currentDate string
}

func NewLogger(logDir string, maxDays int) (*Logger, error) {
	// 创建日志目录
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	logger := &Logger{
		logDir:  logDir,
		maxDays: maxDays,
	}

	// 初始化日志文件
	if err := logger.rotateLog(); err != nil {
		return nil, err
	}

	// 设置标准日志输出
	log.SetOutput(io.MultiWriter(os.Stdout, logger.file))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	// 启动清理协程
	go logger.cleanupLoop()

	return logger, nil
}

func (l *Logger) rotateLog() error {
	today := time.Now().Format("2006-01-02")

	// 如果日期没变，不需要轮转
	if l.currentDate == today && l.file != nil {
		return nil
	}

	// 关闭旧文件
	if l.file != nil {
		l.file.Close()
	}

	// 创建新的日志文件
	logFile := filepath.Join(l.logDir, fmt.Sprintf("smartdns-agent-%s.log", today))
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("创建日志文件失败: %w", err)
	}

	l.file = file
	l.currentDate = today

	log.Printf("📁 日志文件轮转: %s", logFile)
	return nil
}

func (l *Logger) cleanupLoop() {
	// 每小时检查一次
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanup()
			// 每天0点轮转日志
			if time.Now().Hour() == 0 {
				l.rotateLog()
			}
		}
	}
}

func (l *Logger) cleanup() {
	files, err := filepath.Glob(filepath.Join(l.logDir, "smartdns-agent-*.log"))
	if err != nil {
		log.Printf("❌ 扫描日志文件失败: %v", err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -l.maxDays)
	deletedCount := 0

	for _, file := range files {
		// 从文件名提取日期
		basename := filepath.Base(file)
		if !strings.HasPrefix(basename, "smartdns-agent-") || !strings.HasSuffix(basename, ".log") {
			continue
		}

		dateStr := strings.TrimPrefix(basename, "smartdns-agent-")
		dateStr = strings.TrimSuffix(dateStr, ".log")

		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			log.Printf("⚠️ 无法解析日志文件日期: %s", basename)
			continue
		}

		if fileDate.Before(cutoff) {
			if err := os.Remove(file); err != nil {
				log.Printf("❌ 删除日志文件失败: %s, %v", file, err)
			} else {
				deletedCount++
				log.Printf("🗑️ 删除过期日志文件: %s", basename)
			}
		}
	}

	if deletedCount > 0 {
		log.Printf("✅ 清理完成，删除了 %d 个过期日志文件", deletedCount)
	}
}

func (l *Logger) GetLogFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(l.logDir, "smartdns-agent-*.log"))
	if err != nil {
		return nil, err
	}

	// 按日期倒序排列
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})

	return files, nil
}

func (l *Logger) GetRecentLogs(lines int) ([]string, error) {
	files, err := l.GetLogFiles()
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return []string{}, nil
	}

	// 读取最新的日志文件
	latestFile := files[0]
	return readLastLines(latestFile, lines)
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// readLastLines 读取文件的最后N行
func readLastLines(filename string, lines int) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// 简单实现：读取整个文件然后取最后N行
	// 对于大文件，可以优化为从文件末尾向前读取
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	allLines := strings.Split(string(content), "\n")

	// 移除空行
	var validLines []string
	for _, line := range allLines {
		if strings.TrimSpace(line) != "" {
			validLines = append(validLines, line)
		}
	}

	// 返回最后N行
	if len(validLines) <= lines {
		return validLines, nil
	}

	return validLines[len(validLines)-lines:], nil
}
