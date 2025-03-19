/**
  @author: Hanhai
  @since: 2025/3/16 20:42:10
  @desc:
**/

package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var (
	logFile       *os.File
	logger        *log.Logger
	loggerMu      sync.Mutex
	initialized   bool
	cronScheduler *cron.Cron
	maxLogSizeMB  int  = 10 // 默认日志文件最大大小为10MB
	isGuiMode     bool      // 是否是GUI模式
)

// SetGuiMode 设置是否为GUI模式
func SetGuiMode(mode bool) {
	isGuiMode = mode
}

// Init 初始化日志系统
func Init() error {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if initialized {
		return nil
	}

	// 创建logs目录
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 创建日志文件
	logFilePath := filepath.Join(logsDir, "app.log")
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %v", err)
	}

	// 设置日志输出
	logFile = file

	var writer io.Writer
	if isGuiMode {
		// GUI模式下只写入文件
		writer = file
	} else {
		// 控制台模式下同时写入文件和控制台
		writer = io.MultiWriter(os.Stdout, file)
	}

	logger = log.New(writer, "", 0) // 不添加前缀，我们将在自定义格式中添加

	// 设置标准日志库的输出
	log.SetOutput(writer)
	log.SetFlags(0) // 清除默认标志，我们将使用自定义格式

	// 先标记为已初始化，然后再启动清理任务
	initialized = true

	// 启动定时清理任务，放在最后执行
	go func() {
		// 延迟一秒启动清理任务，确保日志系统完全初始化
		time.Sleep(1 * time.Second)
		startLogCleaner()
	}()

	return nil
}

// SetMaxLogSize 设置日志文件最大大小
func SetMaxLogSize(sizeMB int) {
	if sizeMB <= 0 {
		sizeMB = 10 // 如果设置为0或负数，使用默认值10MB
	}

	// 更新最大大小
	maxLogSizeMB = sizeMB

	// 使用标准日志库记录，避免递归调用
	log.Printf("日志文件最大大小已设置为 %d MB", sizeMB)
}

// startLogCleaner 启动日志清理定时任务
func startLogCleaner() {
	if cronScheduler != nil {
		stopLogCleaner()
	}

	cronScheduler = cron.New()
	// 每30秒检查一次是否需要清理日志
	cronScheduler.AddFunc("@every 30s", cleanLogs)
	cronScheduler.Start()

	// 避免在初始化时就记录日志，防止递归调用
	if initialized {
		log.Printf("日志清理任务已启动，日志文件大小超过 %d MB 时将自动清理", maxLogSizeMB)
	}
}

// stopLogCleaner 停止日志清理定时任务
func stopLogCleaner() {
	if cronScheduler != nil {
		cronScheduler.Stop()
		cronScheduler = nil
	}
}

// cleanLogs 清理过期的日志
func cleanLogs() {
	// 避免在未初始化时调用
	if !initialized {
		return
	}

	loggerMu.Lock()
	defer loggerMu.Unlock()

	if logFile == nil {
		return
	}

	// 获取当前日志文件信息
	fileInfo, err := logFile.Stat()
	if err != nil {
		log.Printf("获取日志文件信息失败: %v", err)
		return
	}

	// 使用全局变量中的日志文件大小限制
	maxLogSize := int64(maxLogSizeMB) * 1024 * 1024

	// 检查日志文件的大小，如果超过限制则清理
	if fileInfo.Size() > maxLogSize {
		// 关闭当前日志文件
		oldFile := logFile
		oldFile.Close()

		// 创建新的日志文件（清空内容）
		logFilePath := filepath.Join("logs", "app.log")
		file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			log.Printf("清空日志文件失败: %v", err)

			// 尝试重新打开原文件
			logFile, _ = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			return
		}

		// 更新日志文件和写入器
		logFile = file

		var writer io.Writer
		if isGuiMode {
			// GUI模式下只写入文件
			writer = file
		} else {
			// 控制台模式下同时写入文件和控制台
			writer = io.MultiWriter(os.Stdout, file)
		}

		logger = log.New(writer, "", 0)
		log.SetOutput(writer)

		// 使用标准日志库记录清理信息，避免递归调用
		log.Printf("日志已自动清理，文件大小超过 %d MB", maxLogSizeMB)
	}
}

// Close 关闭日志文件
func Close() {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	// 停止清理任务
	stopLogCleaner()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	initialized = false
}

// formatLog 格式化日志消息
func formatLog(apiKey, format string, args ...interface{}) string {
	// 格式化时间
	timeStr := time.Now().Format("2006/01/02 15:04:05")

	// 如果API密钥为空，则使用"-"代替
	if apiKey == "" {
		apiKey = "-"
	} else if len(apiKey) > 6 {
		// 只保留API密钥的前6个字母
		apiKey = apiKey[:6]
	}

	// 格式化消息内容
	var message string
	if len(args) > 0 {
		message = fmt.Sprintf(format, args...)
	} else {
		message = format
	}

	// 返回完整的日志信息
	if message != "" {
		return fmt.Sprintf("%s - %s - %s", timeStr, apiKey, message)
	}

	// 如果没有消息内容，只返回时间和API密钥
	return fmt.Sprintf("%s - %s", timeStr, apiKey)
}

// Info 记录普通信息日志
func Info(format string, args ...interface{}) {
	// 如果格式字符串为空，不记录日志
	if format == "" && len(args) == 0 {
		return
	}

	loggerMu.Lock()
	defer loggerMu.Unlock()

	if !initialized {
		if err := Init(); err != nil {
			log.Printf("初始化日志系统失败: %v", err)
			return
		}
	}

	logger.Println(formatLog("", format, args...))
}

// InfoWithKey 记录带API密钥的普通信息日志
func InfoWithKey(apiKey, format string, args ...interface{}) {
	// 如果格式字符串为空且没有参数，不记录日志
	if format == "" && len(args) == 0 {
		return
	}

	loggerMu.Lock()
	defer loggerMu.Unlock()

	if !initialized {
		if err := Init(); err != nil {
			log.Printf("初始化日志系统失败: %v", err)
			return
		}
	}

	logger.Println(formatLog(apiKey, format, args...))
}

// Warn 记录警告日志
func Warn(format string, args ...interface{}) {
	// 如果格式字符串为空且没有参数，不记录日志
	if format == "" && len(args) == 0 {
		return
	}

	loggerMu.Lock()
	defer loggerMu.Unlock()

	if !initialized {
		if err := Init(); err != nil {
			log.Printf("初始化日志系统失败: %v", err)
			return
		}
	}

	logger.Println(formatLog("", "WARN: "+format, args...))
}

// WarnWithKey 记录带API密钥的警告日志
func WarnWithKey(apiKey, format string, args ...interface{}) {
	// 如果格式字符串为空且没有参数，不记录日志
	if format == "" && len(args) == 0 {
		return
	}

	loggerMu.Lock()
	defer loggerMu.Unlock()

	if !initialized {
		if err := Init(); err != nil {
			log.Printf("初始化日志系统失败: %v", err)
			return
		}
	}

	logger.Println(formatLog(apiKey, "WARN: "+format, args...))
}

// Error 记录错误日志
func Error(format string, args ...interface{}) {
	// 如果格式字符串为空且没有参数，不记录日志
	if format == "" && len(args) == 0 {
		return
	}

	loggerMu.Lock()
	defer loggerMu.Unlock()

	if !initialized {
		if err := Init(); err != nil {
			log.Printf("初始化日志系统失败: %v", err)
			return
		}
	}

	logger.Println(formatLog("", "ERROR: "+format, args...))
}

// ErrorWithKey 记录带API密钥的错误日志
func ErrorWithKey(apiKey, format string, args ...interface{}) {
	// 如果格式字符串为空且没有参数，不记录日志
	if format == "" && len(args) == 0 {
		return
	}

	loggerMu.Lock()
	defer loggerMu.Unlock()

	if !initialized {
		if err := Init(); err != nil {
			log.Printf("初始化日志系统失败: %v", err)
			return
		}
	}

	logger.Println(formatLog(apiKey, "ERROR: "+format, args...))
}

// Fatal 记录致命错误日志并退出程序
func Fatal(format string, args ...interface{}) {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if !initialized {
		if err := Init(); err != nil {
			log.Fatalf("初始化日志系统失败: %v", err)
			return
		}
	}

	logger.Println(formatLog("", "FATAL: "+format, args...))
	os.Exit(1)
}

// InitLogger 初始化日志系统（Init的别名，用于兼容性）
func InitLogger() error {
	return Init()
}

// CleanLogsNow 立即清理日志文件
func CleanLogsNow() {
	if !initialized {
		return
	}

	log.Printf("手动触发日志清理，将检查文件大小是否超过限制...")
	cleanLogs()
}
