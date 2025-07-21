/**
  @author: Hanhai
  @desc: 结构化日志扩展，提供更规范的日志输出
**/

package logger

import (
	"context"
	"fmt"
	"time"
)

// LogContext 日志上下文
type LogContext struct {
	RequestID   string                 // 请求ID
	APIKey      string                 // API密钥（掩码后）
	Module      string                 // 模块名称
	Method      string                 // HTTP方法
	Path        string                 // 请求路径
	ModelName   string                 // 模型名称
	StartTime   time.Time              // 开始时间
	Extra       map[string]interface{} // 额外信息
}

// RequestLogger 请求日志记录器
type RequestLogger struct {
	ctx       *LogContext
	startTime time.Time
}

// NewRequestLogger 创建新的请求日志记录器
func NewRequestLogger(requestID, apiKey, module string) *RequestLogger {
	return &RequestLogger{
		ctx: &LogContext{
			RequestID: requestID,
			APIKey:    apiKey,
			Module:    module,
			StartTime: time.Now(),
			Extra:     make(map[string]interface{}),
		},
		startTime: time.Now(),
	}
}

// SetMethod 设置HTTP方法
func (rl *RequestLogger) SetMethod(method string) *RequestLogger {
	rl.ctx.Method = method
	return rl
}

// SetPath 设置请求路径
func (rl *RequestLogger) SetPath(path string) *RequestLogger {
	rl.ctx.Path = path
	return rl
}

// SetModel 设置模型名称
func (rl *RequestLogger) SetModel(model string) *RequestLogger {
	rl.ctx.ModelName = model
	return rl
}

// SetExtra 设置额外信息
func (rl *RequestLogger) SetExtra(key string, value interface{}) *RequestLogger {
	rl.ctx.Extra[key] = value
	return rl
}

// formatStructuredLog 格式化结构化日志
func (rl *RequestLogger) formatStructuredLog(level, message string, duration ...time.Duration) string {
	// 时间戳
	timeStr := time.Now().Format("2006/01/02 15:04:05")
	
	// 基础格式：时间 - RequestID - APIKey - 级别 - 模块
	logStr := fmt.Sprintf("%s - %s - %s - %s - %s", 
		timeStr, 
		rl.ctx.RequestID, 
		formatAPIKey(rl.ctx.APIKey),
		level,
		rl.ctx.Module,
	)
	
	// 添加方法和路径
	if rl.ctx.Method != "" && rl.ctx.Path != "" {
		logStr += fmt.Sprintf(" - %s %s", rl.ctx.Method, rl.ctx.Path)
	}
	
	// 添加模型名称
	if rl.ctx.ModelName != "" {
		logStr += fmt.Sprintf(" - Model: %s", rl.ctx.ModelName)
	}
	
	// 添加耗时
	if len(duration) > 0 {
		logStr += fmt.Sprintf(" - Duration: %v", duration[0])
	}
	
	// 添加消息
	logStr += fmt.Sprintf(" - %s", message)
	
	// 添加额外信息
	if len(rl.ctx.Extra) > 0 {
		logStr += " - Extra: {"
		first := true
		for k, v := range rl.ctx.Extra {
			if !first {
				logStr += ", "
			}
			logStr += fmt.Sprintf("%s: %v", k, v)
			first = false
		}
		logStr += "}"
	}
	
	return logStr
}

// Info 记录信息日志
func (rl *RequestLogger) Info(format string, args ...interface{}) {
	if !shouldLog(LevelInfo) {
		return
	}
	
	message := fmt.Sprintf(format, args...)
	logStr := rl.formatStructuredLog("INFO", message)
	
	loggerMu.Lock()
	defer loggerMu.Unlock()
	
	if initialized && logger != nil {
		logger.Println(logStr)
	}
}

// InfoWithDuration 记录带耗时的信息日志
func (rl *RequestLogger) InfoWithDuration(format string, args ...interface{}) {
	if !shouldLog(LevelInfo) {
		return
	}
	
	duration := time.Since(rl.startTime)
	message := fmt.Sprintf(format, args...)
	logStr := rl.formatStructuredLog("INFO", message, duration)
	
	loggerMu.Lock()
	defer loggerMu.Unlock()
	
	if initialized && logger != nil {
		logger.Println(logStr)
	}
}

// Warn 记录警告日志
func (rl *RequestLogger) Warn(format string, args ...interface{}) {
	if !shouldLog(LevelWarn) {
		return
	}
	
	message := fmt.Sprintf(format, args...)
	logStr := rl.formatStructuredLog("WARN", message)
	
	loggerMu.Lock()
	defer loggerMu.Unlock()
	
	if initialized && logger != nil {
		logger.Println(logStr)
	}
}

// WarnWithDuration 记录带耗时的警告日志
func (rl *RequestLogger) WarnWithDuration(format string, args ...interface{}) {
	if !shouldLog(LevelWarn) {
		return
	}
	
	duration := time.Since(rl.startTime)
	message := fmt.Sprintf(format, args...)
	logStr := rl.formatStructuredLog("WARN", message, duration)
	
	loggerMu.Lock()
	defer loggerMu.Unlock()
	
	if initialized && logger != nil {
		logger.Println(logStr)
	}
}

// Error 记录错误日志
func (rl *RequestLogger) Error(format string, args ...interface{}) {
	if !shouldLog(LevelError) {
		return
	}
	
	message := fmt.Sprintf(format, args...)
	logStr := rl.formatStructuredLog("ERROR", message)
	
	loggerMu.Lock()
	defer loggerMu.Unlock()
	
	if initialized && logger != nil {
		logger.Println(logStr)
	}
}

// ErrorWithDuration 记录带耗时的错误日志
func (rl *RequestLogger) ErrorWithDuration(format string, args ...interface{}) {
	if !shouldLog(LevelError) {
		return
	}
	
	duration := time.Since(rl.startTime)
	message := fmt.Sprintf(format, args...)
	logStr := rl.formatStructuredLog("ERROR", message, duration)
	
	loggerMu.Lock()
	defer loggerMu.Unlock()
	
	if initialized && logger != nil {
		logger.Println(logStr)
	}
}

// LogRequestComplete 记录请求完成日志（自动计算耗时）
func (rl *RequestLogger) LogRequestComplete(success bool, statusCode int) {
	if !shouldLog(LevelInfo) {
		return
	}
	
	duration := time.Since(rl.startTime)
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	
	message := fmt.Sprintf("Request completed - Status: %s, Code: %d", status, statusCode)
	
	// 根据耗时选择日志级别
	level := "INFO"
	if duration > 30*time.Second {
		level = "WARN"
	} else if duration > 60*time.Second {
		level = "ERROR"
	}
	
	logStr := rl.formatStructuredLog(level, message, duration)
	
	loggerMu.Lock()
	defer loggerMu.Unlock()
	
	if initialized && logger != nil {
		logger.Println(logStr)
	}
}

// formatAPIKey 格式化API密钥
func formatAPIKey(apiKey string) string {
	if apiKey == "" {
		return "-"
	}
	if len(apiKey) > 6 {
		return apiKey[:6]
	}
	return apiKey
}

// WithRequestLogger 为context添加请求日志记录器
func WithRequestLogger(ctx context.Context, rl *RequestLogger) context.Context {
	return context.WithValue(ctx, "request_logger", rl)
}

// GetRequestLogger 从context获取请求日志记录器
func GetRequestLogger(ctx context.Context) *RequestLogger {
	if rl, ok := ctx.Value("request_logger").(*RequestLogger); ok {
		return rl
	}
	return nil
}

// TruncateContent 截断内容到指定长度
func TruncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}