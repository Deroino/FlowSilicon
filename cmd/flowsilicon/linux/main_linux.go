/**
  @author: Hanhai
  @desc: Linux平台主程序入口，包含配置初始化和服务启动功能
**/

package main

import (
	"flowsilicon/internal/config"
	"flowsilicon/internal/key"
	"flowsilicon/internal/logger"
	"flowsilicon/internal/model"
	"flowsilicon/internal/proxy"
	"flowsilicon/internal/web"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	// 全局变量，用于存储服务器端口
	serverPort int
	// 版本号
	Version = "1.3.9"
	// 程序所在目录
	executableDir string
)

func main() {
	// 获取可执行文件所在目录
	var err error
	executableDir, err = getExecutableDir()
	if err != nil {
		fmt.Printf("无法获取可执行文件目录: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	err = logger.InitLogger()
	if err != nil {
		fmt.Printf("初始化日志系统失败: %v\n", err)
		os.Exit(1)
	}

	logger.Info("程序以控制台模式启动，日志同时写入控制台和文件")
	logger.Info("程序运行目录: %s", executableDir)
	// 添加更多路径信息用于调试
	logger.Info("日志目录绝对路径: %s", getAbsolutePath("logs"))
	logger.Info("当前工作目录: %s", getCurrentDir())

	// 确保必要的目录结构存在
	if err := ensureDirectoriesExist(); err != nil {
		logger.Error("确保目录结构存在时出错: %v", err)
	} else {
		logger.Info("已确保必要的目录结构存在")
	}

	// 初始化配置数据库
	dbPath := getAbsolutePath("data/config.db")
	err = config.InitConfigDB(dbPath)
	if err != nil {
		logger.Error("初始化配置数据库失败: %v", err)
		os.Exit(1)
	}
	logger.Info("配置数据库初始化成功: %s", dbPath)

	// 初始化模型数据库
	err = model.InitModelDB(dbPath)
	if err != nil {
		logger.Error("初始化模型数据库失败: %v", err)
		// 不退出程序，因为这不是致命错误
	} else {
		logger.Info("模型数据库初始化成功: %s", dbPath)
	}

	// 将当前版本号保存到数据库中
	// 确保版本号格式一致 (添加v前缀如果不存在)
	versionToSave := Version
	if !strings.HasPrefix(versionToSave, "v") {
		versionToSave = "v" + versionToSave
	}
	err = config.SaveVersion(versionToSave)
	if err != nil {
		logger.Error("保存版本号到数据库失败: %v", err)
	} else {
		logger.Info("版本号 '%s' 已保存到数据库", versionToSave)
	}

	// 检查并插入默认配置
	err = config.EnsureDefaultConfig(dbPath)
	if err != nil {
		logger.Error("确保默认配置失败: %v", err)
		return
	}

	// 确保apikeys表存在
	err = config.EnsureApikeys(dbPath)
	if err != nil {
		logger.Error("创建apikeys表失败: %v", err)
		// 继续执行，因为这不是致命错误
	} else {
		logger.Info("确保API密钥表存在成功")
	}

	// 设置数据文件路径
	config.SetDailyFilePath(getAbsolutePath("data/daily.json"))

	// 初始化每日统计数据
	if err := config.InitDailyStats(); err != nil {
		logger.Error("初始化每日统计数据失败: %v", err)
		// 继续执行，因为这不是致命错误
	} else {
		logger.Info("每日统计数据初始化成功")
	}

	// 加载配置
	cfg, err := config.LoadConfigFromDB()
	if err != nil {
		logger.Error("从数据库加载配置失败: %v", err)
		return
	}

	// 确保cfg不为nil后再使用
	if cfg == nil {
		logger.Error("配置加载后为空")
		return
	}

	// 获取数据库中的版本号，并更新应用标题
	dbVersion := config.GetVersion()
	if dbVersion != "" {
		// 确保版本号格式一致
		if !strings.HasPrefix(dbVersion, "v") {
			dbVersion = "v" + dbVersion
		}
		// 更新App.Title中的版本号
		cfg.App.Title = fmt.Sprintf("流动硅基 FlowSilicon %s", dbVersion)
		// 保存回数据库
		config.UpdateConfig(cfg)
		config.SaveConfigToDB()
		logger.Info("已从数据库更新应用标题为: %s", cfg.App.Title)
	}

	// 在配置加载完成后更新数据库连接参数
	config.UpdateDBConnectionParams()
	model.UpdateModelDBConnectionParams()

	// 添加调试信息
	logger.Info("配置值 - AutoUpdateInterval: %d, StatsRefreshInterval: %d, RateRefreshInterval: %d",
		cfg.App.AutoUpdateInterval, cfg.App.StatsRefreshInterval, cfg.App.RateRefreshInterval)

	// 设置日志文件大小，使用goroutine避免阻塞
	go func() {
		// 设置日志文件最大大小
		logMaxSize := cfg.Log.MaxSizeMB
		if logMaxSize <= 0 {
			logMaxSize = 1 // 默认10MB
		}
		logger.SetMaxLogSize(logMaxSize)

		// 设置日志等级
		logLevel := cfg.Log.Level
		if logLevel == "" {
			logLevel = "warn" // 默认warn级别
		}
		logger.SetLogLevel(logLevel)

		// 手动触发一次日志清理，使用更长的延时确保系统完全初始化
		// 避免在启动流程中太早清理日志造成问题
		time.Sleep(5 * time.Second)
		logger.CleanLogsNow()

		// 输出确认信息
		logger.Info("日志清理任务已在后台启动，日志等级设置为：%s", logLevel)
	}()

	// 加载API密钥
	if err := config.LoadApiKeysFromDB(); err != nil {
		logger.Error("加载API密钥失败: %v", err)
		// 继续执行，因为这不是致命错误
	} else {
		logger.Info("API密钥加载成功")

		// 强制刷新所有API密钥的余额
		if refreshErr := key.ForceRefreshAllKeysBalance(); refreshErr != nil {
			logger.Error("刷新API密钥余额失败: %v", refreshErr)
		} else {
			logger.Info("已完成API密钥余额的强制刷新")
		}
	}

	// 启动API密钥管理器
	key.StartKeyManager()
	logger.Info("API密钥管理器已启动")

	// 启动性能报告器
	proxy.StartPerformanceReporter()
	logger.Info("性能监控报告器已启动")

	// 输出模型策略配置
	logModelStrategies()

	// 创建Gin路由
	router := gin.Default()
	// 设置受信任的代理
	router.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	// 设置API代理
	web.SetupApiProxy(router)

	// 设置API密钥管理
	web.SetupKeysAPI(router)

	// 设置Web界面
	web.SetupWebServer(router)

	// 保存端口到全局变量
	serverPort = cfg.Server.Port

	// 创建一个通道来接收信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 在goroutine中启动服务器
	go func() {
		logger.Info("服务器启动在 :%d", serverPort)
		if err := router.Run(fmt.Sprintf(":%d", serverPort)); err != nil {
			logger.Error("服务器启动失败: %v", err)
			os.Exit(1)
		}
	}()

	// 等待服务器启动
	time.Sleep(500 * time.Millisecond)

	// 打印访问信息
	logger.Info("流动硅基服务已启动，请访问 http://localhost:%d", serverPort)

	// 等待信号
	<-sigChan
	logger.Info("接收到关闭信号，正在关闭服务器...")

	// 确保所有资源被正确关闭
	logger.Info("正在关闭所有资源...")

	// 停止API密钥管理器定时任务
	key.StopKeyManager()
	logger.Info("API密钥管理器已停止")

	// 保存API密钥
	if err := config.SaveApiKeys(); err != nil {
		logger.Error("保存API密钥失败: %v", err)
	} else {
		logger.Info("API密钥已保存")
	}

	// 关闭配置数据库连接
	if err := config.CloseConfigDB(); err != nil {
		logger.Error("关闭配置数据库连接失败: %v", err)
	} else {
		logger.Info("配置数据库已关闭")
	}

	// 关闭模型数据库连接
	if err := model.CloseModelDB(); err != nil {
		logger.Error("关闭模型数据库连接失败: %v", err)
	} else {
		logger.Info("模型数据库已关闭")
	}

	// 关闭日志系统
	logger.CloseLogger()

	logger.Info("服务器已关闭")

	// 确保程序完全退出
	os.Exit(0)
}

// getExecutableDir 获取可执行文件所在目录
func getExecutableDir() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法获取可执行文件路径: %v", err)
	}
	return filepath.Dir(execPath), nil
}

// getCurrentDir 获取当前工作目录
func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "未知（获取失败）"
	}
	return dir
}

// getAbsolutePath 获取相对于可执行文件目录的绝对路径
func getAbsolutePath(relativePath string) string {
	return filepath.Join(executableDir, relativePath)
}

// openBrowser 打开默认浏览器访问指定URL
func openBrowser(url string) {
	var err error

	logger.Info("正在打开浏览器访问: %s", url)

	// Linux下使用xdg-open打开浏览器
	err = exec.Command("xdg-open", url).Start()

	if err != nil {
		logger.Error("打开浏览器失败: %v", err)
	}
}

// ensureConfigExists 确保配置文件存在，如果不存在则创建
func ensureConfigExists(configPath string) error {
	// 检查配置文件是否存在
	_, err := os.Stat(configPath)
	if err == nil {
		// 配置文件已存在
		return nil
	}

	if !os.IsNotExist(err) {
		// 发生了其他错误
		return fmt.Errorf("检查配置文件时出错: %v", err)
	}

	// 确保配置目录存在
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %v", err)
	}

	return nil
}

// ensureDirectoriesExist 确保必要的目录结构存在
func ensureDirectoriesExist() error {
	// 需要确保存在的目录列表
	directories := []string{
		getAbsolutePath("data"),
		getAbsolutePath("logs"),
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %v", dir, err)
		}
	}

	return nil
}

// logModelStrategies 输出所有已配置的模型策略
func logModelStrategies() {
	cfg := config.GetConfig()

	if len(cfg.App.ModelKeyStrategies) == 0 {
		logger.Info("未配置任何模型特定策略")
		return
	}

	logger.Info("===== 模型特定策略配置 =====")
	for model, strategyID := range cfg.App.ModelKeyStrategies {
		var strategyName string
		switch strategyID {
		case 1:
			strategyName = "高成功率"
		case 2:
			strategyName = "高分数"
		case 3:
			strategyName = "低RPM"
		case 4:
			strategyName = "低TPM"
		case 5:
			strategyName = "高余额"
		case 6:
			strategyName = "普通"
		case 7:
			strategyName = "低余额"
		case 8:
			strategyName = "免费"
		default:
			strategyName = "普通"
		}
		logger.Info("模型: %s, 策略: %s (%d)", model, strategyName, strategyID)
	}
	logger.Info("==========================")
}

// reloadConfig 重新加载配置文件
func reloadConfig() {
	logger.Info("正在重新加载配置")

	// 加载配置
	cfg, err := config.LoadConfigFromDB()
	if err != nil {
		logger.Error("从数据库重新加载配置失败: %v", err)
		return
	}

	// 确保cfg不为nil后再使用
	if cfg == nil {
		logger.Error("配置加载后为空")
		return
	}

	// 获取数据库中的版本号，并更新应用标题
	dbVersion := config.GetVersion()
	if dbVersion != "" {
		// 确保版本号格式一致
		if !strings.HasPrefix(dbVersion, "v") {
			dbVersion = "v" + dbVersion
		}
		// 更新App.Title中的版本号
		cfg.App.Title = fmt.Sprintf("流动硅基 FlowSilicon %s", dbVersion)
		logger.Info("已从数据库更新应用标题为: %s", cfg.App.Title)
	}

	// 更新全局配置
	config.UpdateConfig(cfg)

	// 更新服务器端口
	serverPort = cfg.Server.Port

	// 将更新后的配置保存回数据库
	err = config.SaveConfigToDB()
	if err != nil {
		logger.Error("保存配置到数据库失败: %v", err)
	}

	// 输出模型策略配置
	logModelStrategies()

	logger.Info("配置重新加载成功")
}

// restartProgram Linux版本的重启程序功能
// 使用exec.Command启动新进程，然后退出当前进程
func restartProgram() {
	logger.Info("正在重启程序...")

	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		logger.Error("获取当前程序路径失败: %v", err)
		return
	}

	// 获取当前工作目录
	workDir, err := os.Getwd()
	if err != nil {
		logger.Error("获取当前工作目录失败: %v", err)
		workDir = executableDir // 如果获取失败，使用程序所在目录
	}

	// 获取当前命令行参数，排除第一个(程序路径)
	args := os.Args
	if len(args) > 1 {
		args = args[1:]
	} else {
		args = []string{} // 确保args不为nil
	}

	logger.Info("准备重启程序: 路径=%s, 工作目录=%s, 参数=%v", execPath, workDir, args)

	// 创建新进程
	cmd := exec.Command(execPath, args...)
	cmd.Dir = workDir
	cmd.Env = os.Environ() // 传递所有当前环境变量

	// 分离新进程与当前进程
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// 启动新进程
	err = cmd.Start()
	if err != nil {
		logger.Error("启动新进程失败: %v", err)
		return
	}

	// 从父进程中分离子进程
	err = cmd.Process.Release()
	if err != nil {
		logger.Error("分离进程失败: %v", err)
	}

	logger.Info("新进程已启动(PID: %d)，当前进程将退出", cmd.Process.Pid)

	// 保存必要的数据
	logger.Info("正在保存重要数据...")
	config.SaveApiKeys()
	config.CloseConfigDB()

	// 需要延迟一小段时间确保数据保存和日志写入完成
	time.Sleep(500 * time.Millisecond)

	// 退出当前进程
	os.Exit(0)
}
