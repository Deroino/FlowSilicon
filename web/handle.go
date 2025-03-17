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
	"net/http"
	"os"
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
		balance, err := key.CheckKeyBalanceManually(req.Key)
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

	// 删除 API 密钥
	if success := config.RemoveApiKey(key); !success {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "API key not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API key deleted successfully",
	})
}

// handleStats 处理获取 API 密钥统计信息的请求
func handleStats(c *gin.Context) {
	keys := config.GetApiKeys()

	// 计算统计信息
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

	// 删除这些API密钥
	for _, key := range zeroOrNegativeBalanceKeys {
		config.RemoveApiKey(key)
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已删除 %d 个余额小于或等于0的API密钥", len(zeroOrNegativeBalanceKeys)),
		"deleted": zeroOrNegativeBalanceKeys,
	})
}

// handleRequestStats 处理获取请求统计数据的请求
func handleRequestStats(c *gin.Context) {
	stats := config.GetRequestStats()
	c.JSON(http.StatusOK, gin.H{
		"stats": stats,
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
