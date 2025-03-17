package main

import (
	"flowsilicon/internal/config"
	"flowsilicon/internal/logger"
	"flowsilicon/web"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"flowsilicon/internal/key"

	"github.com/getlantern/systray"
	"github.com/gin-gonic/gin"
)

var (
	// 全局变量，用于存储服务器端口
	serverPort int
	// 版本号
	Version = "1.3.5"
)

// openBrowser 打开默认浏览器访问指定URL
func openBrowser(url string) {
	var err error

	logger.Info("正在打开浏览器访问: %s", url)

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
	}

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

	// 创建默认配置文件
	defaultConfig := `# API代理配置
api_proxy:
    # API基础URL, 用于转发请求
    base_url: https://api.siliconflow.cn

# 服务器配置
server:
    # 服务器监听端口
    port: 3201

# 日志配置
log:
    # 日志文件最大大小（MB），超过此大小的日志将被清理
    max_size_mb: 1

# 应用程序配置
app:
    # 应用程序标题，显示在Web界面上
    title: "流动硅基 FlowSilicon"
    # 最低余额阈值，低于此值的API密钥将被自动禁用
    min_balance_threshold: 0.8
    # 余额显示的最大值，用于前端显示进度条
    max_balance_display: 14
    # 每页显示的密钥数量
    items_per_page: 5
    # 最大统计条目数，用于限制请求统计的历史记录数量
    max_stats_entries: 60
    # 恢复检查间隔（分钟），系统会每隔此时间尝试恢复被禁用的密钥
    recovery_interval: 10
    # 最大连续失败次数，超过此值的密钥将被自动禁用
    max_consecutive_failures: 5
    # 权重配置
    # 余额评分权重（默认0.4，即40%）
    balance_weight: 0.4
    # 成功率评分权重（默认0.3，即30%）
    success_rate_weight: 0.3
    # RPM评分权重（默认0.15，即15%）
    rpm_weight: 0.15
    # TPM评分权重（默认0.15，即15%）
    tpm_weight: 0.15
    # 自动更新配置
    stats_refresh_interval: 100  # 统计信息自动刷新间隔（秒）
    rate_refresh_interval: 150   # 速率监控自动刷新间隔（秒）
    auto_update_interval: 100   # API密钥状态自动更新间隔（秒）
    # 模型特定的密钥选择策略
    # 策略ID: 1=高成功率, 2=高分数, 3=低RPM, 4=低TPM, 5=高余额
    model_key_strategies:
        "deepseek-ai/DeepSeek-V3": 2  # 使用高成功率策略`

	// 写入默认配置文件
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("创建默认配置文件失败: %v", err)
	}

	return nil
}

// ensureDirectoriesExist 确保必要的目录结构存在
func ensureDirectoriesExist() error {
	// 需要确保存在的目录列表
	directories := []string{
		"./config",
		"./data",
		"./logs",
	}

	for _, dir := range directories {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %v", dir, err)
		}
	}

	return nil
}

// 系统托盘初始化
func onReady() {
	// 设置托盘图标和标题
	iconPath := "web/static/favicon_16.ico"
	if _, err := os.Stat(iconPath); err == nil {
		// 图标文件存在，读取图标
		icon, err := os.ReadFile(iconPath)
		if err == nil {
			systray.SetIcon(icon)
		} else {
			logger.Error("读取图标文件失败: %v", err)
		}
	}

	systray.SetTitle("流动硅基")
	systray.SetTooltip("流动硅基 FlowSilicon v" + Version)

	// 添加菜单项
	mOpen := systray.AddMenuItem("打开界面", "打开Web界面")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出程序", "退出程序")

	// 处理菜单点击事件
	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				// 打开Web界面
				openBrowser(fmt.Sprintf("http://localhost:%d", serverPort))
			case <-mQuit.ClickedCh:
				// 退出程序
				logger.Info("用户通过托盘菜单退出程序")
				systray.Quit()
				return
			}
		}
	}()
}

// 系统托盘退出
func onExit() {
	// 保存API密钥
	config.SaveApiKeys()
	logger.Info("程序已退出")
	os.Exit(0)
}

func main() {
	// 先初始化日志
	logger.InitLogger()

	// 确保必要的目录结构存在
	if err := ensureDirectoriesExist(); err != nil {
		logger.Error("确保目录结构存在时出错: %v", err)
	} else {
		logger.Info("已确保必要的目录结构存在")
	}

	// 加载配置文件
	configPath := "./config/config.yaml"
	if len(os.Args) > 1 && os.Args[1] == "--config" && len(os.Args) > 2 {
		configPath = os.Args[2]
	}

	// 确保配置文件存在
	if err := ensureConfigExists(configPath); err != nil {
		logger.Error("确保配置文件存在时出错: %v", err)
	} else {
		logger.Info("已确保配置文件存在: %s", configPath)
	}

	// 尝试加载配置文件
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Error("加载配置文件失败: %v，将使用默认配置", err)
		// 使用默认配置
		cfg = config.GetConfig()
	} else {
		logger.Info("成功加载配置文件: %s", configPath)
		// 添加调试信息
		logger.Info("配置值 - AutoUpdateInterval: %d, StatsRefreshInterval: %d, RateRefreshInterval: %d",
			cfg.App.AutoUpdateInterval, cfg.App.StatsRefreshInterval, cfg.App.RateRefreshInterval)
	}

	// 设置日志文件大小，使用goroutine避免阻塞
	go func() {
		// 设置日志文件最大大小
		logMaxSize := cfg.Log.MaxSizeMB
		if logMaxSize <= 0 {
			logMaxSize = 10 // 默认10MB
		}
		logger.SetMaxLogSize(logMaxSize)

		// 手动触发一次日志清理
		time.Sleep(2 * time.Second) // 等待日志系统完全初始化
		logger.CleanLogsNow()
	}()

	// 初始化每日统计数据
	if err := config.InitDailyStats(); err != nil {
		logger.Error("初始化每日统计数据失败: %v", err)
	} else {
		logger.Info("每日统计数据初始化成功")
	}

	// 启动API密钥管理器
	key.StartKeyManager()
	logger.Info("API密钥管理器已启动")

	// 创建Gin路由
	router := gin.Default()

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

	// 自动打开浏览器
	openBrowser(fmt.Sprintf("http://localhost:%d", serverPort))

	// 启动系统托盘
	go systray.Run(onReady, onExit)

	// 等待信号
	<-sigChan
	logger.Info("接收到关闭信号，正在关闭服务器...")

	// 只保存API密钥，不保存配置
	// config.SaveConfig() - 已移除，避免覆盖用户修改的配置文件
	config.SaveApiKeys()

	// 退出系统托盘
	systray.Quit()

	logger.Info("服务器已关闭")
}
