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
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// 日志等级常量
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelFatal = "fatal"
)

// 日志等级权重映射，用于比较等级高低
var logLevelWeights = map[string]int{
	LevelDebug: 0,
	LevelInfo:  1,
	LevelWarn:  2,
	LevelError: 3,
	LevelFatal: 4,
}

var (
	logFile       *os.File
	logger        *log.Logger
	loggerMu      sync.Mutex
	initialized   bool
	cronScheduler *cron.Cron
	maxLogSizeMB  int    = 10     // 默认日志文件最大大小为10MB
	logLevel      string = "warn" // 默认日志等级为warn
	isGuiMode     bool            // 是否是GUI模式
)

// SetGuiMode 设置是否为GUI模式
func SetGuiMode(mode bool) {
	isGuiMode = mode
}

// SetLogLevel 设置日志等级
func SetLogLevel(level string) {
	// 将输入的日志等级转换为小写，并验证有效性
	level = strings.ToLower(level)
	if _, ok := logLevelWeights[level]; !ok {
		// 如果是无效的日志等级，则使用默认等级
		level = LevelWarn
		log.Printf("无效的日志等级: %s，已设置为默认值: %s", level, LevelWarn)
	}

	// 使用锁保护更新全局变量
	loggerMu.Lock()
	defer loggerMu.Unlock()

	logLevel = level
	log.Printf("日志等级已设置为: %s", level)
}

// shouldLog 判断给定的日志等级是否应该被记录
func shouldLog(level string) bool {
	// 将给定的日志等级转换为小写并获取其权重
	level = strings.ToLower(level)
	levelWeight, ok := logLevelWeights[level]
	if !ok {
		// 如果是未知的日志等级，则默认记录
		return true
	}

	// 获取当前设置的日志等级权重
	currentLevelWeight, ok := logLevelWeights[logLevel]
	if !ok {
		// 如果当前设置了未知的日志等级，则使用 warn 的权重
		currentLevelWeight = logLevelWeights[LevelWarn]
	}

	// 如果给定日志等级的权重 >= 当前设置的权重，则记录日志
	return levelWeight >= currentLevelWeight
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

	// 创建一个新的cron调度器
	cronScheduler = cron.New(cron.WithSeconds()) // 使用WithSeconds允许更精确的控制

	// 每60秒检查一次是否需要清理日志 (降低频率以减少潜在的问题)
	_, err := cronScheduler.AddFunc("0 */1 * * * *", func() {
		// 在独立的goroutine中执行清理任务，避免阻塞cron调度器
		go safeCleanLogs()
	})

	if err != nil {
		log.Printf("添加日志清理定时任务失败: %v", err)
		return
	}

	// 启动定时任务
	cronScheduler.Start()

	// 避免在初始化时就记录日志，防止递归调用
	if initialized {
		log.Printf("日志清理定时任务已启动，每分钟检查一次，日志文件大小超过 %d MB 时将自动清理", maxLogSizeMB)
	}
}

// stopLogCleaner 停止日志清理定时任务
func stopLogCleaner() {
	if cronScheduler != nil {
		cronScheduler.Stop()
		cronScheduler = nil
	}
}

// CleanLogsNow 立即清理日志文件
func CleanLogsNow() {
	if !initialized {
		return
	}

	// 使用标准库日志，它不会触发我们的锁机制
	log.Printf("手动触发日志清理，将检查文件大小是否超过限制...")

	// 在单独的goroutine中清理日志，避免阻塞主线程
	go func() {
		// 给其他可能的日志操作一点时间完成
		time.Sleep(500 * time.Millisecond)

		// 使用更加安全的清理方式
		safeCleanLogs()
	}()
}

// safeCleanLogs 以更安全的方式清理日志，避免死锁
func safeCleanLogs() {
	// 1. 首先在锁保护下获取必要的信息
	var needCleanup bool
	var oldFileSize int64
	var logMaxSize int64
	var logFilePath string

	func() {
		// 在子函数内使用锁，确保锁的作用范围最小
		loggerMu.Lock()
		defer loggerMu.Unlock()

		if !initialized || logFile == nil {
			return
		}

		// 获取当前日志文件信息
		fileInfo, err := logFile.Stat()
		if err != nil {
			log.Printf("获取日志文件信息失败: %v", err)
			return
		}

		// 使用全局变量中的日志文件大小限制
		logMaxSize = int64(maxLogSizeMB) * 1024 * 1024
		oldFileSize = fileInfo.Size()
		needCleanup = oldFileSize > logMaxSize
		logFilePath = filepath.Join("logs", "app.log")
	}()

	// 如果不需要清理，直接返回
	if !needCleanup {
		return
	}

	// 2. 记录要清理的信息
	log.Printf("日志大小(%d字节)超过限制(%d字节)，准备清理(使用日志轮转方式)...", oldFileSize, logMaxSize)

	// 使用新的基于日志轮转的清理方式
	rotateAndCreateNewLog(logFilePath)
}

// rotateAndCreateNewLog 使用日志轮转方式管理日志文件
func rotateAndCreateNewLog(logFilePath string) {
	loggerMu.Lock()
	defer loggerMu.Unlock()

	if !initialized || logFile == nil {
		return
	}

	// 关闭当前日志文件
	oldFile := logFile
	logFile = nil // 先置空，避免其他goroutine使用已关闭的文件

	if err := oldFile.Close(); err != nil {
		log.Printf("关闭旧日志文件失败: %v", err)
		logFile = oldFile // 恢复指针
		return
	}

	// 获取日志目录
	logDir := filepath.Dir(logFilePath)
	logFileName := filepath.Base(logFilePath)
	fileNameWithoutExt := strings.TrimSuffix(logFileName, filepath.Ext(logFileName))
	fileExt := filepath.Ext(logFileName)

	// 生成包含时间戳的新文件名（轮转后的归档文件名）
	timestamp := time.Now().Format("20060102_150405")
	archiveFileName := fmt.Sprintf("%s_%s%s", fileNameWithoutExt, timestamp, fileExt)
	archiveFilePath := filepath.Join(logDir, archiveFileName)

	// 重命名当前日志文件为带时间戳的归档文件
	err := os.Rename(logFilePath, archiveFilePath)
	if err != nil {
		log.Printf("重命名日志文件失败: %v", err)

		// 尝试重新打开旧文件
		logFile, _ = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		return
	}

	// 创建新的日志文件
	newFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("创建新日志文件失败: %v", err)

		// 尝试将归档文件改回原名
		os.Rename(archiveFilePath, logFilePath)

		// 尝试重新打开旧文件
		logFile, _ = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		return
	}

	// 写入日志初始记录
	timestamp = time.Now().Format("2006/01/02 15:04:05")
	initialLog := fmt.Sprintf("%s - - 日志文件已轮转 (旧日志已归档为: %s)\n", timestamp, archiveFileName)
	if _, err := newFile.WriteString(initialLog); err != nil {
		log.Printf("写入新日志文件失败: %v", err)
	}

	// 更新全局日志文件指针
	logFile = newFile

	// 重新设置日志输出
	var writer io.Writer
	if isGuiMode {
		// GUI模式下只写入文件
		writer = newFile
	} else {
		// 控制台模式下同时写入文件和控制台
		writer = io.MultiWriter(os.Stdout, newFile)
	}

	logger = log.New(writer, "", 0)
	log.SetOutput(writer)

	// 清理旧日志文件
	go cleanOldLogFiles(logDir, fileNameWithoutExt, fileExt)

	// 使用标准日志库记录清理信息
	log.Printf("日志已轮转完成，新日志文件已创建，旧日志已归档为 %s", archiveFileName)
}

// cleanOldLogFiles 清理过老的日志文件，保留最近的几个
func cleanOldLogFiles(logDir, fileNamePrefix, fileExt string) {
	// 保留的日志文件数量
	const maxLogFiles = 5

	// 查找所有匹配的日志文件
	pattern := filepath.Join(logDir, fileNamePrefix+"_*"+fileExt)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		log.Printf("查找旧日志文件失败: %v", err)
		return
	}

	// 如果日志文件数量未超过限制，不需要清理
	if len(matches) <= maxLogFiles {
		return
	}

	// 按文件名排序（时间戳在文件名中，所以这会按时间排序）
	sort.Strings(matches)

	// 删除多余的最旧的日志文件
	for i := 0; i < len(matches)-maxLogFiles; i++ {
		if err := os.Remove(matches[i]); err != nil {
			log.Printf("删除旧日志文件 %s 失败: %v", matches[i], err)
		} else {
			log.Printf("已删除旧日志文件: %s", matches[i])
		}
	}
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

	// 检查当前日志等级是否允许记录info级别的日志
	if !shouldLog(LevelInfo) {
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

	// 检查当前日志等级是否允许记录info级别的日志
	if !shouldLog(LevelInfo) {
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

	// 检查当前日志等级是否允许记录warn级别的日志
	if !shouldLog(LevelWarn) {
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

// Error 记录错误日志
func Error(format string, args ...interface{}) {
	// 如果格式字符串为空且没有参数，不记录日志
	if format == "" && len(args) == 0 {
		return
	}

	// 检查当前日志等级是否允许记录error级别的日志
	if !shouldLog(LevelError) {
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

// Fatal 记录致命错误日志并退出程序
func Fatal(format string, args ...interface{}) {
	// 检查当前日志等级是否允许记录fatal级别的日志 (通常fatal级别总是会被记录的)
	if !shouldLog(LevelFatal) {
		// 即使配置不记录，也至少在标准输出上显示
		log.Printf("FATAL: "+format, args...)
		os.Exit(1)
		return
	}

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

// CloseLogger 关闭日志系统，停止所有定时任务
func CloseLogger() {
	// 停止日志清理任务
	stopLogCleaner()

	// 等待所有日志写入完成
	loggerMu.Lock()
	defer loggerMu.Unlock()

	// 关闭文件句柄
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}

	log.Println("日志系统已关闭")
}
