/**
  @author: Hanhai
  @desc: 代理中间件，包含日志记录和性能监控
**/

package proxy

import (
	"flowsilicon/internal/logger"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestLoggingMiddleware 请求日志中间件
func RequestLoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 生成请求ID
		requestID := uuid.New().String()[:8] // 使用短ID
		c.Set("request_id", requestID)
		
		// 创建请求日志记录器
		rl := logger.NewRequestLogger(requestID, "", "proxy").
			SetMethod(c.Request.Method).
			SetPath(c.Request.URL.Path)
		
		// 将日志记录器存入context
		c.Set("request_logger", rl)
		
		// 记录请求开始
		rl.Info("Request started")
		
		// 时间追踪器
		tracker := logger.NewTimeTracker(requestID)
		c.Set("time_tracker", tracker)
		
		// 记录请求开始时间
		startTime := time.Now()
		
		// 处理请求
		c.Next()
		
		// 计算请求耗时
		duration := time.Since(startTime)
		
		// 获取响应状态
		statusCode := c.Writer.Status()
		success := statusCode >= 200 && statusCode < 300
		
		// 记录请求完成
		rl.LogRequestComplete(success, statusCode)
		
		// 记录性能指标
		logger.RecordRequestMetrics(duration, success)
		
		// 如果是慢请求，记录详情
		if duration > 5*time.Second {
			details := map[string]interface{}{
				"status_code": statusCode,
				"method":      c.Request.Method,
				"user_agent":  c.Request.UserAgent(),
			}
			logger.LogSlowRequest(requestID, c.Request.URL.Path, duration, details)
		}
	}
}

// GetRequestLogger 从gin context获取请求日志记录器
func GetRequestLogger(c *gin.Context) *logger.RequestLogger {
	if rl, exists := c.Get("request_logger"); exists {
		if requestLogger, ok := rl.(*logger.RequestLogger); ok {
			return requestLogger
		}
	}
	
	// 如果没有，创建一个默认的
	requestID := "unknown"
	if id, exists := c.Get("request_id"); exists {
		requestID = id.(string)
	}
	
	return logger.NewRequestLogger(requestID, "", "proxy")
}

// GetTimeTracker 从gin context获取时间追踪器
func GetTimeTracker(c *gin.Context) *logger.TimeTracker {
	if tt, exists := c.Get("time_tracker"); exists {
		if tracker, ok := tt.(*logger.TimeTracker); ok {
			return tracker
		}
	}
	return nil
}