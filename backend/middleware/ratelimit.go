package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type visitor struct {
	limiter  *time.Ticker
	lastSeen time.Time
}

var visitors = make(map[string]*visitor)
var mu sync.Mutex

// RateLimit 速率限制中间件
func RateLimit(requestsPerMinute int) gin.HandlerFunc {
	// 清理过期访问者
	go cleanupVisitors()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		mu.Lock()

		v, exists := visitors[ip]
		if !exists {
			ticker := time.NewTicker(time.Minute / time.Duration(requestsPerMinute))
			visitors[ip] = &visitor{ticker, time.Now()}
			mu.Unlock()
			c.Next()
			return
		}

		v.lastSeen = time.Now()
		mu.Unlock()

		select {
		case <-v.limiter.C:
			c.Next()
		default:
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "请求过于频繁，请稍后再试",
			})
			c.Abort()
		}
	}
}

func cleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		mu.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				v.limiter.Stop()
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}
