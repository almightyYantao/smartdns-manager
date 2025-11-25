package utils

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"smartdns-log-agent/models"
)

type LogParser struct {
	regex          *regexp.Regexp
	regexWithGroup *regexp.Regexp // 新增：支持带 group 字段的格式
}

func NewLogParser() *LogParser {
	// 原始格式（不带 group）
	regex := regexp.MustCompile(`\[([^\]]+)\]\s+(\S+)\s+query\s+(\S+),\s+type\s+(\d+),\s+time\s+(\d+)ms,\s+speed:\s+([-\d.]+)ms,\s+result\s*(.*)`)

	// 新格式（带 group）
	regexWithGroup := regexp.MustCompile(`\[([^\]]+)\]\s+(\S+)\s+query\s+(\S+),\s+type\s+(\d+),\s+time\s+(\d+)ms,\s+speed:\s+([-\d.]+)ms,\s+group\s+(\S+),\s+result\s*(.*)`)

	return &LogParser{
		regex:          regex,
		regexWithGroup: regexWithGroup,
	}
}

func (p *LogParser) Parse(line string, nodeID uint32) *models.DNSLogRecord {
	if line == "" {
		return nil
	}

	// 先尝试匹配带 group 的格式
	matches := p.regexWithGroup.FindStringSubmatch(line)
	if matches != nil && len(matches) >= 9 {
		return p.parseWithGroup(matches, nodeID, line)
	}

	// 再尝试匹配不带 group 的格式
	matches = p.regex.FindStringSubmatch(line)
	if matches != nil && len(matches) >= 8 {
		return p.parseWithoutGroup(matches, nodeID, line)
	}

	return nil
}

// parseWithGroup 解析带 group 字段的日志
func (p *LogParser) parseWithGroup(matches []string, nodeID uint32, line string) *models.DNSLogRecord {
	// 解析时间戳
	timestamp, err := time.ParseInLocation("2006-01-02 15:04:05,000", matches[1], time.Local)
	if err != nil {
		timestamp, _ = time.ParseInLocation("2006-01-02 15:04:05", matches[1][:19], time.Local)
	}

	queryType, _ := strconv.Atoi(matches[4])
	timeMs, _ := strconv.Atoi(matches[5])
	speedMs, _ := strconv.ParseFloat(matches[6], 32)

	// 解析结果 IP（matches[8] 是 result 部分）
	resultStr := strings.TrimSpace(matches[8])
	var resultIPs []string
	if resultStr != "" {
		resultIPs = strings.Split(resultStr, ",")
		for i := range resultIPs {
			resultIPs[i] = strings.TrimSpace(resultIPs[i])
		}
	}

	return &models.DNSLogRecord{
		Timestamp:   timestamp,
		Date:        time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location()),
		NodeID:      nodeID,
		ClientIP:    matches[2],
		Domain:      matches[3],
		QueryType:   uint16(queryType),
		TimeMs:      uint32(timeMs),
		SpeedMs:     float32(speedMs),
		ResultCount: uint8(len(resultIPs)),
		ResultIPs:   resultIPs,
		RawLog:      line,
		Group:       strings.TrimSpace(matches[7]),
	}
}

// parseWithoutGroup 解析不带 group 字段的日志
func (p *LogParser) parseWithoutGroup(matches []string, nodeID uint32, line string) *models.DNSLogRecord {
	// 解析时间戳
	timestamp, err := time.ParseInLocation("2006-01-02 15:04:05,000", matches[1], time.Local)
	if err != nil {
		timestamp, _ = time.ParseInLocation("2006-01-02 15:04:05", matches[1][:19], time.Local)
	}

	queryType, _ := strconv.Atoi(matches[4])
	timeMs, _ := strconv.Atoi(matches[5])
	speedMs, _ := strconv.ParseFloat(matches[6], 32)

	// 解析结果 IP（matches[7] 是 result 部分）
	resultStr := strings.TrimSpace(matches[7])
	var resultIPs []string
	if resultStr != "" {
		resultIPs = strings.Split(resultStr, ",")
		for i := range resultIPs {
			resultIPs[i] = strings.TrimSpace(resultIPs[i])
		}
	}

	return &models.DNSLogRecord{
		Timestamp:   timestamp,
		Date:        time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location()),
		NodeID:      nodeID,
		ClientIP:    matches[2],
		Domain:      matches[3],
		QueryType:   uint16(queryType),
		TimeMs:      uint32(timeMs),
		SpeedMs:     float32(speedMs),
		ResultCount: uint8(len(resultIPs)),
		ResultIPs:   resultIPs,
		RawLog:      line,
	}
}
