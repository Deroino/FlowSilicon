/**
  @author: Hanhai
  @since: 2025/3/16 21:56:00
  @desc:
**/

package web

import (
	"flowsilicon/internal/common"
	"flowsilicon/internal/config"
	"flowsilicon/internal/key"
	"flowsilicon/internal/logger"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// handleListKeys 处理列出所有 API 密钥的请求
func handleListKeys(c *gin.Context) {
	// 获取所有API密钥
	allKeys := config.GetApiKeys()

	// 使用公共函数计算密钥得分
	keysWithScores := key.CalculateKeyScores(allKeys)

	// 创建一个映射，用于存储密钥的得分
	scoreMap := make(map[string]float64)
	for _, ks := range keysWithScores {
		scoreMap[ks.Key.Key] = ks.Score
	}

	// 为每个密钥添加得分
	for i := range allKeys {
		// 如果在scoreMap中找到对应的得分，则添加
		if score, ok := scoreMap[allKeys[i].Key]; ok {
			allKeys[i].Score = score
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"keys": allKeys,
	})
}

// handleAddKey 处理添加 API 密钥的请求
func handleAddKey(c *gin.Context) {
	var req struct {
		Key     string  `json:"key" binding:"required"`
		Balance float64 `json:"balance"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效请求: %v", err),
		})
		return
	}

	// 如果未提供余额，尝试检查余额
	if req.Balance == 0 {
		balance, err := key.CheckKeyBalance(req.Key)
		if err == nil {
			req.Balance = balance
		} else {
			// 继续使用提供的余额（0）
		}
	}

	// 检查余额是否小于或等于0
	if req.Balance <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "无法添加余额小于或等于0的API密钥",
			"balance": req.Balance,
		})
		return
	}

	// 添加 API 密钥
	config.AddApiKey(req.Key, req.Balance)

	// 重新排序 API 密钥
	config.SortApiKeysByBalance()

	// 保存到数据库
	if err := config.SaveApiKeys(); err != nil {
		logger.Error("保存API密钥到数据库失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API密钥添加成功",
		"balance": req.Balance,
	})
}

// handleCheckKey 处理检查 API 密钥余额的请求
func handleCheckKey(c *gin.Context) {
	var req struct {
		Key string `json:"key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// 检查 API 密钥余额
	balance, err := key.CheckKeyBalanceManually(req.Key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to check balance: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"key":     req.Key,
		"balance": balance,
	})
}

// handleSetKeyMode 处理设置 API 密钥使用模式的请求
func handleSetKeyMode(c *gin.Context) {
	var req struct {
		Mode string   `json:"mode" binding:"required"`
		Keys []string `json:"keys"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// 转换模式字符串为 KeyMode 类型
	var mode key.KeyMode
	switch req.Mode {
	case "single":
		mode = key.KeyModeSingle
		if len(req.Keys) != 1 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "单独使用模式需要选择一个密钥",
			})
			return
		}
	case "selected":
		mode = key.KeyModeSelected
		if len(req.Keys) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "轮询选中模式需要至少选择两个密钥",
			})
			return
		}
	case "all":
		mode = key.KeyModeAll
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid mode: %s", req.Mode),
		})
		return
	}

	// 设置 API 密钥使用模式
	if err := key.SetKeyMode(mode, req.Keys); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Failed to set key mode: %v", err),
		})
		return
	}

	// 返回成功消息
	var modeDesc string
	switch mode {
	case key.KeyModeSingle:
		modeDesc = "单独使用选中密钥"
	case key.KeyModeSelected:
		modeDesc = "轮询选中密钥"
	case key.KeyModeAll:
		modeDesc = "轮询所有密钥"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("API 密钥使用模式已设置为: %s", modeDesc),
		"mode":    string(mode),
		"keys":    req.Keys,
	})
}

// handleGetKeyMode 处理获取当前 API 密钥使用模式的请求
func handleGetKeyMode(c *gin.Context) {
	mode, keys := key.GetCurrentKeyMode()

	// 返回当前模式
	c.JSON(http.StatusOK, gin.H{
		"mode": string(mode),
		"keys": keys,
	})
}

// handleBatchAddKeys 处理批量添加 API 密钥的请求
func handleBatchAddKeys(c *gin.Context) {
	var req struct {
		Keys    []string `json:"keys" binding:"required"`
		Balance float64  `json:"balance"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效请求: %v", err),
		})
		return
	}

	// 添加所有 API 密钥
	addedCount := 0
	skippedCount := 0
	for _, _key := range req.Keys {
		if _key != "" {
			// 如果未提供余额，尝试检查余额
			balance := req.Balance
			if balance == 0 {
				checkedBalance, err := key.CheckKeyBalanceManually(_key)
				if err == nil {
					balance = checkedBalance
				}
			}

			// 只添加余额大于0的密钥
			if balance > 0 {
				config.AddApiKey(_key, balance)
				addedCount++
			} else {
				skippedCount++
			}
		}
	}

	// 重新排序 API 密钥
	config.SortApiKeysByBalance()

	// 保存到数据库
	if err := config.SaveApiKeys(); err != nil {
		logger.Error("保存API密钥到数据库失败: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("成功添加 %d 个API密钥，跳过 %d 个余额小于或等于0的密钥", addedCount, skippedCount),
		"added":   addedCount,
		"skipped": skippedCount,
	})
}

// handleDeleteKey 处理删除 API 密钥的请求
func handleDeleteKey(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Key parameter is required",
		})
		return
	}

	// 标记 API 密钥为删除状态，而不是直接删除
	if success := config.MarkApiKeyForDeletion(key); !success {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "API key not found",
		})
		return
	}

	// 立即从列表中移除已标记为删除的密钥
	config.RemoveMarkedApiKeys()

	// 保存更新后的状态
	if err := config.SaveApiKeys(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("无法保存API密钥状态: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API key deleted successfully",
	})
}

// handleStats 处理获取 API 密钥系统概要的请求
func handleStats(c *gin.Context) {
	keys := config.GetApiKeys()

	// 计算系统概要
	var totalBalance float64
	var activeKeys int
	var lastUsedTime int64
	var disabledKeys int
	var totalCalls int
	var successCalls int
	var avgSuccessRate float64
	var activeKeysBalance float64

	for _, key := range keys {
		totalBalance += key.Balance
		if key.Balance > 0 && !key.Disabled {
			activeKeys++
			activeKeysBalance += key.Balance
		}
		if key.LastUsed > lastUsedTime {
			lastUsedTime = key.LastUsed
		}
		if key.Disabled {
			disabledKeys++
		}
		totalCalls += key.TotalCalls
		successCalls += key.SuccessCalls
	}

	// 计算平均成功率
	if totalCalls > 0 {
		avgSuccessRate = float64(successCalls) / float64(totalCalls)
	} else {
		avgSuccessRate = 0
	}

	// 格式化最后使用时间
	var lastUsedTimeStr string
	if lastUsedTime > 0 {
		lastUsedTimeStr = time.Unix(lastUsedTime, 0).Format(time.RFC3339)
	} else {
		lastUsedTimeStr = "Never"
	}

	c.JSON(http.StatusOK, gin.H{
		"total_keys":          len(keys),
		"active_keys":         activeKeys,
		"disabled_keys":       disabledKeys,
		"total_balance":       totalBalance,
		"active_keys_balance": activeKeysBalance,
		"last_used_time":      lastUsedTimeStr,
		"total_calls":         totalCalls,
		"success_calls":       successCalls,
		"avg_success_rate":    avgSuccessRate,
	})
}

// handleGetLogs 处理获取日志的请求
func handleGetLogs(c *gin.Context) {
	// 获取最近的日志内容
	// 这里我们假设日志文件在当前目录下的logs/app.log
	logFilePath := "logs/app.log"

	// 检查文件是否存在
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		// 如果文件不存在，返回一个友好的消息
		c.String(http.StatusOK, "日志文件不存在或尚未创建")
		return
	}

	// 读取日志文件
	logContent, err := os.ReadFile(logFilePath)
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("读取日志文件失败: %v", err))
		return
	}

	// 如果日志文件太大，只返回最后的部分
	const maxLogSize = 100 * 1024 // 100KB
	if len(logContent) > maxLogSize {
		// 找到最后maxLogSize字节中的第一个换行符位置
		startPos := len(logContent) - maxLogSize
		for i := startPos; i < len(logContent); i++ {
			if logContent[i] == '\n' {
				startPos = i + 1
				break
			}
		}

		// 返回截断后的日志内容
		c.String(http.StatusOK, fmt.Sprintf("(日志文件较大，只显示最后部分)\n\n%s", logContent[startPos:]))
	} else {
		// 返回完整的日志内容
		c.String(http.StatusOK, string(logContent))
	}
}

// CustomLogger 自定义Gin日志中间件
func CustomLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 开始时间
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// 处理请求
		c.Next()

		// 结束时间
		end := time.Now()
		latency := end.Sub(start)
		statusCode := c.Writer.Status()

		// 获取请求中的API密钥（如果有）
		apiKey := c.GetHeader("Authorization")
		if apiKey != "" {
			// 移除Bearer前缀
			if len(apiKey) > 7 && apiKey[:7] == "Bearer " {
				apiKey = apiKey[7:]
			}

			// 记录API调用的详细信息
			logger.InfoWithKey(apiKey, "%s %s %d %v", method, path, statusCode, latency)
		} else {
			// 记录没有API密钥的请求
			logger.Info("%s %s %d %v", method, path, statusCode, latency)
		}
	}
}

// handleTestEmbeddings 处理测试embeddings API的请求
func handleTestEmbeddings(c *gin.Context) {
	// 获取API密钥
	var req struct {
		Key string `json:"key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// 测试embeddings API
	success, response, err := common.TestEmbeddings(req.Key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":  false,
			"error":    err.Error(),
			"response": response,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  success,
		"response": response,
	})
}

// handleEnableKey 处理启用 API 密钥的请求
func handleEnableKey(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Key parameter is required",
		})
		return
	}

	// 启用 API 密钥
	if success := config.EnableApiKey(key); !success {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "API key not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API key enabled successfully",
	})
}

// handleDisableKey 处理禁用 API 密钥的请求
func handleDisableKey(c *gin.Context) {
	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Key parameter is required",
		})
		return
	}

	// 禁用 API 密钥
	if success := config.DisableApiKey(key); !success {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "API key not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API key disabled successfully",
	})
}

// handleDeleteZeroBalanceKeys 处理删除余额为0或负数的API密钥的请求
func handleDeleteZeroBalanceKeys(c *gin.Context) {
	keys := config.GetApiKeys()

	// 过滤出余额小于或等于0的API密钥
	var zeroOrNegativeBalanceKeys []string
	for _, key := range keys {
		if key.Balance <= 0 {
			zeroOrNegativeBalanceKeys = append(zeroOrNegativeBalanceKeys, key.Key)
		}
	}

	// 标记这些API密钥为删除状态
	for _, key := range zeroOrNegativeBalanceKeys {
		config.MarkApiKeyForDeletion(key)
	}

	// 立即从列表中移除已标记为删除的密钥
	config.RemoveMarkedApiKeys()

	// 保存更新后的状态
	if err := config.SaveApiKeys(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("无法保存API密钥状态: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      fmt.Sprintf("已删除 %d 个余额小于或等于0的API密钥", len(zeroOrNegativeBalanceKeys)),
		"deleted_keys": zeroOrNegativeBalanceKeys,
	})
}

// handleRequestStats 处理获取请求统计数据的请求
func handleRequestStats(c *gin.Context) {
	// 获取当前RPM和TPM
	rpm, tpm := config.GetCurrentRequestStats()
	// 获取当前RPD和TPD
	rpd := config.GetCurrentRPD()
	tpd := config.GetCurrentTPD()

	// 获取所有API密钥的统计数据
	keys := config.GetApiKeys()
	keyStats := make([]map[string]interface{}, 0)

	for _, key := range keys {
		// 跳过已标记为删除的密钥
		if key.Delete {
			continue
		}

		// 计算成功率
		successRate := 0.0
		if key.TotalCalls > 0 {
			successRate = float64(key.SuccessCalls) / float64(key.TotalCalls)
		}

		// 添加密钥的统计数据
		keyStats = append(keyStats, map[string]interface{}{
			"key":          key.Key,
			"rpm":          key.RequestsPerMinute,
			"tpm":          key.TokensPerMinute,
			"total_calls":  key.TotalCalls,
			"success_rate": successRate,
			"score":        key.Score,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"rpm":       rpm,
		"tpm":       tpm,
		"rpd":       rpd,
		"tpd":       tpd,
		"key_stats": keyStats,
	})
}

func handleTestChat(c *gin.Context) {
	// 获取API密钥
	var req struct {
		Key string `json:"key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// 测试模型列表API
	success, response, err := common.TestChatAPI(req.Key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":  false,
			"error":    err.Error(),
			"response": response,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  success,
		"response": response,
	})
}

// handleTestImages 处理测试图片生成API的请求
func handleTestImages(c *gin.Context) {
	// 获取API密钥
	var req struct {
		Key string `json:"key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// 测试图片生成API
	success, response, err := common.TestImageGeneration(req.Key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":  false,
			"error":    err.Error(),
			"response": response,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  success,
		"response": response,
	})
}

// handleTestModels 处理测试模型列表API的请求
func handleTestModels(c *gin.Context) {
	// 获取API密钥
	var req struct {
		Key string `json:"key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// 测试模型列表API
	success, response, err := common.TestModelsAPI(req.Key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":  false,
			"error":    err.Error(),
			"response": response,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  success,
		"response": response,
	})
}

// handleTestRerank 处理测试重排序API的请求
func handleTestRerank(c *gin.Context) {
	// 获取API密钥
	var req struct {
		Key string `json:"key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Invalid request: %v", err),
		})
		return
	}

	// 测试重排序API
	success, response, err := common.TestRerankAPI(req.Key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":  false,
			"error":    err.Error(),
			"response": response,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  success,
		"response": response,
	})
}

// handleGetTestKey 处理获取测试用的API密钥的请求
func handleGetTestKey(c *gin.Context) {
	// 使用系统配置的方式获取API密钥
	apiKey, err := key.GetNextApiKey()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"error": fmt.Sprintf("获取API密钥失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"key": apiKey,
	})
}

// StartWebServer 启动 Web 服务器
func StartWebServer(port int) {
	// 设置Gin为发布模式，避免在控制台输出调试信息
	gin.SetMode(gin.ReleaseMode)

	// 创建不带默认中间件的Gin路由
	router := gin.New()

	// 使用自定义日志中间件和恢复中间件
	router.Use(CustomLogger(), gin.Recovery())

	// 设置 Web 服务器
	SetupWebServer(router)

	// 设置 API 代理
	SetupApiProxy(router)

	// 启动服务器
	addr := fmt.Sprintf(":%d", port)

	if err := router.Run(addr); err != nil {
		logger.Fatal("Failed to start web server: %v", err)
	}
}

// handleGetCurrentRequestStats 获取当前请求速率统计
func handleGetCurrentRequestStats(c *gin.Context) {
	// 获取全局RPM和TPM
	rpm, tpm := config.GetCurrentRequestStats()

	// 获取全局RPD和TPD
	rpd := config.GetCurrentRPD()
	tpd := config.GetCurrentTPD()

	// 添加日志，记录RPD和TPD的值
	logger.Info("当前请求统计 - RPM: %d, TPM: %d, RPD: %d, TPD: %d", rpm, tpm, rpd, tpd)

	// 获取所有API密钥
	allKeys := config.GetApiKeys()

	// 使用公共函数计算密钥得分
	keysWithScores := key.CalculateKeyScores(allKeys)

	// 构建返回的密钥统计数据
	keyStats := make([]map[string]interface{}, 0, len(keysWithScores))

	for _, ks := range keysWithScores {
		k := ks.Key

		// 掩盖密钥（直接在这里实现，不调用MaskKey函数）
		var maskedKey string
		if len(k.Key) <= 6 {
			maskedKey = "******"
		} else {
			maskedKey = k.Key[:6] + "******"
		}

		keyStats = append(keyStats, map[string]interface{}{
			"key":                  maskedKey,
			"rpm":                  k.RequestsPerMinute,
			"tpm":                  k.TokensPerMinute,
			"disabled":             k.Disabled,
			"total_calls":          k.TotalCalls,
			"success_calls":        k.SuccessCalls,
			"success_rate":         k.SuccessRate,
			"consecutive_failures": k.ConsecutiveFailures,
			"score":                ks.Score, // 添加得分字段
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"rpm":       rpm,
		"tpm":       tpm,
		"rpd":       rpd,
		"tpd":       tpd,
		"timestamp": time.Now().Unix(),
		"key_stats": keyStats,
	})
}

func handleIndex(c *gin.Context) {
	// 获取配置
	cfg := config.GetConfig()

	// 添加调试日志
	logger.Info("配置值 - AutoUpdateInterval: %d, StatsRefreshInterval: %d, RateRefreshInterval: %d",
		cfg.App.AutoUpdateInterval,
		cfg.App.StatsRefreshInterval,
		cfg.App.RateRefreshInterval)

	c.HTML(http.StatusOK, "index.html", gin.H{
		"title":                  cfg.App.Title,
		"max_balance_display":    cfg.App.MaxBalanceDisplay,
		"items_per_page":         cfg.App.ItemsPerPage,
		"auto_update_interval":   cfg.App.AutoUpdateInterval,
		"stats_refresh_interval": cfg.App.StatsRefreshInterval,
		"rate_refresh_interval":  cfg.App.RateRefreshInterval,
	})
}

// handleGetDailyStats 获取每日统计数据
func handleGetDailyStats(c *gin.Context) {
	// 获取所有日期的统计数据
	stats, err := config.GetAllDailyStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取每日统计数据失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// handleGetDailyStatsByDate 获取指定日期的统计数据
func handleGetDailyStatsByDate(c *gin.Context) {
	// 获取日期参数
	date := c.Param("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "日期参数不能为空",
		})
		return
	}

	// 获取指定日期的统计数据
	stats, err := config.GetDailyStats(date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("获取每日统计数据失败: %v", err),
		})
		return
	}

	if stats == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": fmt.Sprintf("未找到日期 %s 的统计数据", date),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
	})
}

// handleGetSettings 处理获取系统设置的请求
func handleGetSettings(c *gin.Context) {
	// 获取当前配置
	cfg := config.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "无法获取系统配置",
		})
		return
	}

	// 创建与前端匹配的配置数据结构
	configData := gin.H{
		"server": gin.H{
			"port": cfg.Server.Port,
		},
		"api_proxy": gin.H{
			"base_url":             cfg.ApiProxy.BaseURL,
			"model_index":          cfg.ApiProxy.ModelIndex,
			"model_key_strategies": cfg.App.ModelKeyStrategies,
			"retry": gin.H{
				"max_retries":             cfg.ApiProxy.Retry.MaxRetries,
				"retry_delay_ms":          cfg.ApiProxy.Retry.RetryDelayMs,
				"retry_on_status_codes":   cfg.ApiProxy.Retry.RetryOnStatusCodes,
				"retry_on_network_errors": cfg.ApiProxy.Retry.RetryOnNetworkErrors,
			},
		},
		"proxy": gin.H{
			"http_proxy":  cfg.Proxy.HttpProxy,
			"https_proxy": cfg.Proxy.HttpsProxy,
			"socks_proxy": cfg.Proxy.SocksProxy,
			"proxy_type":  cfg.Proxy.ProxyType,
			"enabled":     cfg.Proxy.Enabled,
		},
		"app": gin.H{
			"title":                    cfg.App.Title,
			"min_balance_threshold":    cfg.App.MinBalanceThreshold,
			"max_balance_display":      cfg.App.MaxBalanceDisplay,
			"items_per_page":           cfg.App.ItemsPerPage,
			"max_stats_entries":        cfg.App.MaxStatsEntries,
			"recovery_interval":        cfg.App.RecoveryInterval,
			"max_consecutive_failures": cfg.App.MaxConsecutiveFailures,
			"balance_weight":           cfg.App.BalanceWeight,
			"success_rate_weight":      cfg.App.SuccessRateWeight,
			"rpm_weight":               cfg.App.RPMWeight,
			"tpm_weight":               cfg.App.TPMWeight,
			"auto_update_interval":     cfg.App.AutoUpdateInterval,
			"stats_refresh_interval":   cfg.App.StatsRefreshInterval,
			"rate_refresh_interval":    cfg.App.RateRefreshInterval,
			"hide_icon":                cfg.App.HideIcon,
		},
		"log": gin.H{
			"max_size_mb": cfg.Log.MaxSizeMB,
			"level":       cfg.Log.Level,
		},
	}

	// 返回配置信息
	c.JSON(http.StatusOK, configData)
}

// handleSaveSettings 处理保存系统设置的请求
func handleSaveSettings(c *gin.Context) {
	// 先获取当前配置作为默认值
	currentConfig := config.GetConfig()
	if currentConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "无法获取当前系统配置",
		})
		return
	}

	// 创建配置的副本
	newConfig := *currentConfig

	// 解析请求体到临时结构
	var configData map[string]interface{}
	if err := c.ShouldBindJSON(&configData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的配置数据: %v", err),
		})
		return
	}

	// 服务器设置
	if server, ok := configData["server"].(map[string]interface{}); ok {
		if port, ok := server["port"].(float64); ok {
			newConfig.Server.Port = int(port)
		}
	}

	// API代理设置
	if apiProxy, ok := configData["api_proxy"].(map[string]interface{}); ok {
		if baseURL, ok := apiProxy["base_url"].(string); ok {
			newConfig.ApiProxy.BaseURL = baseURL
		}
		if modelIndex, ok := apiProxy["model_index"].(float64); ok {
			newConfig.ApiProxy.ModelIndex = int(modelIndex)
		}

		// 处理模型特定策略
		if modelKeyStrategies, ok := apiProxy["model_key_strategies"].(map[string]interface{}); ok {
			// 清空现有策略
			newConfig.App.ModelKeyStrategies = make(map[string]int)

			// 添加新策略
			for modelName, strategy := range modelKeyStrategies {
				if strategyValue, ok := strategy.(float64); ok {
					newConfig.App.ModelKeyStrategies[modelName] = int(strategyValue)
				}
			}
		}

		// 重试配置
		if retry, ok := apiProxy["retry"].(map[string]interface{}); ok {
			if maxRetries, ok := retry["max_retries"].(float64); ok {
				newConfig.ApiProxy.Retry.MaxRetries = int(maxRetries)
			}
			if retryDelay, ok := retry["retry_delay_ms"].(float64); ok {
				newConfig.ApiProxy.Retry.RetryDelayMs = int(retryDelay)
			}
			if networkErrors, ok := retry["retry_on_network_errors"].(bool); ok {
				newConfig.ApiProxy.Retry.RetryOnNetworkErrors = networkErrors
			}
			if statusCodes, ok := retry["retry_on_status_codes"].([]interface{}); ok {
				codes := make([]int, 0, len(statusCodes))
				for _, code := range statusCodes {
					if c, ok := code.(float64); ok {
						codes = append(codes, int(c))
					}
				}
				newConfig.ApiProxy.Retry.RetryOnStatusCodes = codes
			}
		}
	}

	// 代理设置
	if proxy, ok := configData["proxy"].(map[string]interface{}); ok {
		if enabled, ok := proxy["enabled"].(bool); ok {
			newConfig.Proxy.Enabled = enabled
		}
		if proxyType, ok := proxy["proxy_type"].(string); ok {
			newConfig.Proxy.ProxyType = proxyType
		}
		if httpProxy, ok := proxy["http_proxy"].(string); ok {
			newConfig.Proxy.HttpProxy = httpProxy
		}
		if httpsProxy, ok := proxy["https_proxy"].(string); ok {
			newConfig.Proxy.HttpsProxy = httpsProxy
		}
		if socksProxy, ok := proxy["socks_proxy"].(string); ok {
			newConfig.Proxy.SocksProxy = socksProxy
		}
	}

	// 应用设置
	if app, ok := configData["app"].(map[string]interface{}); ok {
		if title, ok := app["title"].(string); ok {
			newConfig.App.Title = title
		}
		if minBalance, ok := app["min_balance_threshold"].(float64); ok {
			newConfig.App.MinBalanceThreshold = minBalance
		}
		if maxBalance, ok := app["max_balance_display"].(float64); ok {
			newConfig.App.MaxBalanceDisplay = maxBalance
		}
		if itemsPerPage, ok := app["items_per_page"].(float64); ok {
			newConfig.App.ItemsPerPage = int(itemsPerPage)
		}
		if maxStats, ok := app["max_stats_entries"].(float64); ok {
			newConfig.App.MaxStatsEntries = int(maxStats)
		}
		if recoveryInterval, ok := app["recovery_interval"].(float64); ok {
			newConfig.App.RecoveryInterval = int(recoveryInterval)
		}
		if maxFailures, ok := app["max_consecutive_failures"].(float64); ok {
			newConfig.App.MaxConsecutiveFailures = int(maxFailures)
		}
		if hideIcon, ok := app["hide_icon"].(bool); ok {
			newConfig.App.HideIcon = hideIcon
		}

		// 权重配置
		if balanceWeight, ok := app["balance_weight"].(float64); ok {
			newConfig.App.BalanceWeight = balanceWeight
		}
		if successRateWeight, ok := app["success_rate_weight"].(float64); ok {
			newConfig.App.SuccessRateWeight = successRateWeight
		}
		if rpmWeight, ok := app["rpm_weight"].(float64); ok {
			newConfig.App.RPMWeight = rpmWeight
		}
		if tpmWeight, ok := app["tpm_weight"].(float64); ok {
			newConfig.App.TPMWeight = tpmWeight
		}

		// 自动更新配置
		if autoUpdate, ok := app["auto_update_interval"].(float64); ok {
			newConfig.App.AutoUpdateInterval = int(autoUpdate)
		}
		if statsRefresh, ok := app["stats_refresh_interval"].(float64); ok {
			newConfig.App.StatsRefreshInterval = int(statsRefresh)
		}
		if rateRefresh, ok := app["rate_refresh_interval"].(float64); ok {
			newConfig.App.RateRefreshInterval = int(rateRefresh)
		}
	}

	// 日志设置
	if log, ok := configData["log"].(map[string]interface{}); ok {
		if maxSize, ok := log["max_size_mb"].(float64); ok {
			newConfig.Log.MaxSizeMB = int(maxSize)
		}
		if level, ok := log["level"].(string); ok {
			newConfig.Log.Level = level
		}
	}

	// 更新配置
	config.UpdateConfig(&newConfig)

	// 保存到数据库
	if err := config.SaveConfigToDB(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("保存配置到数据库失败: %v", err),
		})
		return
	}

	// 返回成功消息
	c.JSON(http.StatusOK, gin.H{
		"message": "配置保存成功",
	})
}

// handleRefreshAllKeysBalance 处理刷新所有API密钥余额的请求
func handleRefreshAllKeysBalance(c *gin.Context) {
	// 使用新的ForceRefreshAllKeysBalance函数，该函数带有2秒超时
	err := key.ForceRefreshAllKeysBalance()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("刷新API密钥余额失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "所有API密钥余额刷新成功",
	})
}

// handleSystemRestart 处理系统重启请求
func handleSystemRestart(c *gin.Context) {
	// 返回成功消息
	c.JSON(http.StatusOK, gin.H{
		"message": "系统重启请求已接收，程序将在几秒钟内重启",
	})

	// 使用goroutine异步执行重启，以便先返回响应
	go func() {
		// 等待一小段时间确保响应已发送
		time.Sleep(1 * time.Second)

		// 导入cmd/flowsilicon/windows包中的函数会导致循环依赖
		// 所以这里我们复制restartProgram的实现

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

		// 获取可执行文件所在目录
		executableDir, err := filepath.Dir(execPath), nil
		if err != nil {
			logger.Error("获取可执行文件目录失败: %v", err)
			return
		}

		// 设置工作目录
		cmd.Dir = executableDir

		// 传递所有当前环境变量
		cmd.Env = os.Environ()

		// 检查是否为GUI模式（Windows特定）
		if runtime.GOOS == "windows" {
			// 使用syscall.GetConsoleWindow来检测是否有控制台窗口
			kernel32 := syscall.NewLazyDLL("kernel32.dll")
			getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
			hwnd, _, _ := getConsoleWindow.Call()
			isGui := hwnd == 0

			if isGui {
				logger.Info("以GUI模式重启程序")
				// 在Windows上，使用特定的启动标志使窗口隐藏
				cmd.SysProcAttr = &syscall.SysProcAttr{
					HideWindow: true,
				}
			} else {
				logger.Info("以控制台模式重启程序")
			}
		} else if runtime.GOOS == "linux" {
			// Linux下的GUI模式判断，通过环境变量控制
			guiMode := os.Getenv("FLOWSILICON_GUI")
			if guiMode == "1" {
				logger.Info("以GUI模式重启程序")
				// 确保环境变量中包含GUI模式标记
				hasGuiEnv := false
				for i, e := range cmd.Env {
					if strings.HasPrefix(e, "FLOWSILICON_GUI=") {
						cmd.Env[i] = "FLOWSILICON_GUI=1"
						hasGuiEnv = true
						break
					}
				}
				if !hasGuiEnv {
					cmd.Env = append(cmd.Env, "FLOWSILICON_GUI=1")
				}
			} else {
				logger.Info("以控制台模式重启程序")
			}
		}

		// 启动新进程
		err = cmd.Start()
		if err != nil {
			logger.Error("重启程序失败: %v", err)
			return
		}

		logger.Info("新进程已启动，进程ID: %d，命令行参数: %v，工作目录: %s",
			cmd.Process.Pid, args, executableDir)

		// 设置退出标志
		logger.Info("当前程序将在重启成功后退出")

		// 如果使用了systray，需要通知退出
		// 这部分在WEB API中可能无法直接访问systray变量
		// 所以我们直接退出程序
		os.Exit(0)
	}()
}

// handleApiKeyProxy 处理API密钥获取的代理请求
func handleApiKeyProxy(c *gin.Context) {
	// 从请求中获取授权令牌
	authToken := c.GetHeader("X-Auth-Token")
	if authToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "未提供认证令牌",
		})
		return
	}

	// 构建目标URL
	targetURL := "https://sili-api.killerbest.com/admin/api/keys"

	// 创建一个新的HTTP请求
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		logger.Error("创建HTTP请求失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("创建请求失败: %v", err),
		})
		return
	}

	// 添加授权头
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", authToken))
	req.Header.Set("Content-Type", "application/json")

	// 创建HTTP客户端并发送请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Error("发送HTTP请求失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("请求失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取响应内容失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   fmt.Sprintf("读取响应失败: %v", err),
		})
		return
	}

	// 设置与原始响应相同的Content-Type
	c.Header("Content-Type", resp.Header.Get("Content-Type"))

	// 返回原始响应状态码和内容
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}
