/**
  @author: Hanhai
  @since: 2025/3/23 22:30:10
  @desc:
**/

package main

import (
	"flowsilicon/internal/config"
	"flowsilicon/internal/key"
	"flowsilicon/internal/logger"
	"flowsilicon/internal/model"
	"flowsilicon/web"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/gin-gonic/gin"
)

var (
	// 全局变量，用于存储服务器端口
	serverPort int
	// 版本号
	Version = "1.3.8"
	// 控制程序退出的通道
	quitChan chan struct{} = make(chan struct{})
	// 控制是否真正退出程序
	realQuit bool = false
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

	// 检测是否是GUI模式（使用-H windowsgui参数打包）
	// 通过检测是否有控制台窗口来判断
	isGui := !isConsolePresent()
	logger.SetGuiMode(isGui)

	// 初始化日志
	logger.InitLogger()

	// 记录启动模式
	if isGui {
		logger.Info("程序以GUI模式启动，日志仅写入文件")
	} else {
		logger.Info("程序以控制台模式启动，日志同时写入控制台和文件")
	}

	logger.Info("程序运行目录: %s", executableDir)
	// 添加更多路径信息用于调试
	logger.Info("日志目录绝对路径: %s", getAbsolutePath("logs"))
	logger.Info("当前工作目录: %s", getCurrentDir())

	// 确保必要的目录结构存在
	if err = ensureDirectoriesExist(); err != nil {
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

	// 检查配置是否存在，如果不存在则插入默认配置
	err = config.EnsureDefaultConfig(dbPath)
	if err != nil {
		logger.Error("确保默认配置失败: %v", err)
		return
	}

	// 确保API密钥表存在
	err = config.EnsureApikeys(dbPath)
	if err != nil {
		logger.Error("确保API密钥表存在失败: %v", err)
		// 继续执行，因为这不是致命错误
	} else {
		logger.Info("确保API密钥表存在成功")
	}

	// 设置数据文件路径
	config.SetDailyFilePath(getAbsolutePath("data/daily.json"))

	// 确保初始化每日统计数据
	err = config.InitDailyStats()
	if err != nil {
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

	// 确保appConfig不为nil后再使用
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

	// 加载API密钥
	err = config.LoadApiKeys()
	if err != nil {
		logger.Error("从数据库加载API密钥失败: %v", err)
		// 继续执行，因为这不是致命错误
	} else {
		logger.Info("已成功从数据库加载API密钥")

		// 强制刷新所有API密钥的余额
		if refreshErr := key.ForceRefreshAllKeysBalance(); refreshErr != nil {
			logger.Error("强制刷新API密钥余额失败: %v", refreshErr)
		} else {
			logger.Info("已完成API密钥余额的强制刷新")
		}
	}

	// 添加调试信息
	logger.Info("配置值 - AutoUpdateInterval: %d, StatsRefreshInterval: %d, RateRefreshInterval: %d",
		cfg.App.AutoUpdateInterval, cfg.App.StatsRefreshInterval, cfg.App.RateRefreshInterval)

	// 设置日志文件大小，使用goroutine避免阻塞
	go func() {
		// 设置日志文件最大大小
		logMaxSize := cfg.Log.MaxSizeMB
		if logMaxSize <= 0 {
			logMaxSize = 10 // 默认10MB
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

	// 启动API密钥管理器
	key.StartKeyManager()
	logger.Info("API密钥管理器已启动")

	// 输出模型策略配置
	logModelStrategies()

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

	// 等待信号或退出通道
	select {
	case <-sigChan:
		logger.Info("接收到关闭信号，正在关闭服务器...")
	case <-quitChan:
		logger.Info("接收到退出请求，正在关闭服务器...")
	}

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

// 系统托盘初始化
func onReady() {
	// 设置托盘图标和标题
	iconPath := getAbsolutePath("web/static/img/favicon_32.ico")
	if _, err := os.Stat(iconPath); err == nil {
		// 图标文件存在，读取图标
		icon, err := os.ReadFile(iconPath)
		if err == nil {
			systray.SetIcon(icon)
		} else {
			logger.Error("读取图标文件失败: %v", err)
		}
	}

	// 获取版本号
	dbVersion := config.GetVersion()
	if dbVersion == "" {
		// 如果数据库中没有版本号，使用硬编码的版本号
		dbVersion = Version
		// 确保版本号格式一致
		if !strings.HasPrefix(dbVersion, "v") {
			dbVersion = "v" + dbVersion
		}
	}

	// 正常显示图标和标题
	systray.SetTitle("流动硅基")
	systray.SetTooltip("流动硅基 FlowSilicon " + dbVersion)

	// 添加菜单项
	mOpen := systray.AddMenuItem("打开界面", "打开Web界面")
	systray.AddSeparator()

	// 新增重启程序菜单项
	mRestart := systray.AddMenuItem("重启程序", "重新启动程序")

	// 新增开机自启菜单项
	mAutoStart := systray.AddMenuItem("开机自启", "设置或取消开机自启")
	// 检查当前开机自启状态并设置选中状态
	if isAutoStartEnabled() {
		mAutoStart.Check()
	}

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出程序", "退出程序")

	// 处理菜单点击事件
	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				// 打开Web界面
				openBrowser(fmt.Sprintf("http://localhost:%d", serverPort))
			case <-mRestart.ClickedCh:
				// 重启程序
				logger.Info("用户通过托盘菜单请求重启程序")
				restartProgram()
			case <-mAutoStart.ClickedCh:
				// 切换开机自启状态
				if isAutoStartEnabled() {
					disableAutoStart()
					mAutoStart.Uncheck()
					logger.Info("已禁用开机自启")
				} else {
					enableAutoStart()
					mAutoStart.Check()
					logger.Info("已启用开机自启")
				}
			case <-mQuit.ClickedCh:
				// 退出程序
				logger.Info("用户通过托盘菜单退出程序")
				realQuit = true // 设置真正退出标志
				systray.Quit()
				return
			}
		}
	}()
}

// 系统托盘退出
func onExit() {
	// 如果是真正的退出请求，则退出程序
	if realQuit {
		// 保存API密钥
		config.SaveApiKeys()
		// 关闭数据库连接
		config.CloseConfigDB()
		// 关闭模型数据库
		model.CloseModelDB()
		logger.Info("程序已退出")
		// 关闭退出通道，通知主程序退出
		close(quitChan)
	} else {
		// 如果不是真正退出，只是重启systray（比如在隐藏/显示图标时）
		logger.Info("系统托盘重启中...")
	}
}

// isConsolePresent 检测当前程序是否有控制台窗口
func isConsolePresent() bool {
	// Windows特定的检测逻辑
	if runtime.GOOS == "windows" {
		// 尝试获取控制台窗口的句柄
		// 如果返回值不为0，则说明有控制台窗口
		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
		hwnd, _, _ := getConsoleWindow.Call()
		return hwnd != 0
	}

	// 其他平台默认认为有控制台
	return true
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

// isAutoStartEnabled 检查是否已启用开机自启
func isAutoStartEnabled() bool {
	if runtime.GOOS != "windows" {
		logger.Error("开机自启功能仅支持Windows系统")
		return false
	}

	// 不需要获取可执行文件路径，只需要查询注册表项是否存在
	// 使用reg query命令检查注册表项是否存在
	cmd := exec.Command("reg", "query", "HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "FlowSilicon")
	err := cmd.Run()

	// 如果命令执行成功，说明注册表项存在（已启用开机自启）
	return err == nil
}

// enableAutoStart 启用开机自启
func enableAutoStart() {
	if runtime.GOOS != "windows" {
		logger.Error("开机自启功能仅支持Windows系统")
		return
	}

	// 获取当前可执行文件的路径
	exePath, err := os.Executable()
	if err != nil {
		logger.Error("获取可执行文件路径失败: %v", err)
		return
	}

	// 使用reg add命令添加注册表项
	cmd := exec.Command("reg", "add", "HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "FlowSilicon", "/t", "REG_SZ", "/d", fmt.Sprintf("\"%s\"", exePath), "/f")
	if err := cmd.Run(); err != nil {
		logger.Error("设置开机自启失败: %v", err)
		return
	}

	logger.Info("已成功设置开机自启")
}

// disableAutoStart 禁用开机自启
func disableAutoStart() {
	if runtime.GOOS != "windows" {
		logger.Error("开机自启功能仅支持Windows系统")
		return
	}

	// 使用reg delete命令删除注册表项
	cmd := exec.Command("reg", "delete", "HKEY_CURRENT_USER\\Software\\Microsoft\\Windows\\CurrentVersion\\Run", "/v", "FlowSilicon", "/f")
	if err := cmd.Run(); err != nil {
		logger.Error("禁用开机自启失败: %v", err)
		return
	}

	logger.Info("已成功禁用开机自启")
}

// restartProgram 重新启动程序，保留原始命令行参数
func restartProgram() {
	execPath, err := os.Executable()
	if err != nil {
		logger.Error("获取当前程序路径失败: %v", err)
		return
	}

	// 获取当前命令行参数，排除第一个(程序路径)
	args := []string{}
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}

	// 记录重启前的命令行参数
	logger.Info("重启程序，当前命令行参数: %v", args)

	// 创建新的进程
	cmd := exec.Command(execPath, args...)

	// 设置工作目录
	cmd.Dir = executableDir

	// 传递所有当前环境变量
	cmd.Env = os.Environ()

	// 检查是否为GUI模式
	isGui := !isConsolePresent()
	if isGui {
		logger.Info("以GUI模式重启程序")
		// 在Windows上，使用特定的启动标志使窗口隐藏
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
	} else {
		logger.Info("以控制台模式重启程序")
	}

	// 启动新进程
	err = cmd.Start()
	if err != nil {
		logger.Error("重启程序失败: %v", err)
		return
	}

	logger.Info("新进程已启动，进程ID: %d，命令行参数: %v，工作目录: %s",
		cmd.Process.Pid, args, executableDir)

	// 设置退出标志并请求程序退出
	logger.Info("当前程序将在重启成功后退出")
	realQuit = true
	systray.Quit()
}
