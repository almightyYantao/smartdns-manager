package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 自定义日志中间件
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		endTime := time.Now()
		latency := endTime.Sub(startTime)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()
		path := c.Request.URL.Path

		log.Printf("[%s] %s %s %d %v",
			method,
			clientIP,
			path,
			statusCode,
			latency,
		)
	}
}
