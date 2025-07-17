/**
  @author: Hanhai
  @desc: Web请求处理函数，实现API密钥管理、认证、统计和测试接口的功能
**/

package web

import (
	"encoding/json"
	"flowsilicon/internal/auth"
	"flowsilicon/internal/common"
	"flowsilicon/internal/config"
	"flowsilicon/internal/key"
	"flowsilicon/internal/logger"
	"flowsilicon/internal/middleware"
	"flowsilicon/internal/model"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"flowsilicon/pkg/utils"

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
		Key              string  `json:"key" binding:"required"`
		Balance          float64 `json:"balance"`
		AllowZeroBalance bool    `json:"allow_zero_balance"`
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
	if req.Balance <= 0 && !req.AllowZeroBalance {
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
		Keys             []string `json:"keys" binding:"required"`
		Balance          float64  `json:"balance"`
		AllowZeroBalance bool     `json:"allow_zero_balance"`
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

			// 根据AllowZeroBalance参数决定是否添加余额小于等于0的密钥
			if balance > 0 || req.AllowZeroBalance {
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
		// 查找密钥检查是否存在
		keys := config.GetApiKeys()
		var keyExists bool
		var balance float64
		var minThreshold float64

		for _, k := range keys {
			if k.Key == key {
				keyExists = true
				balance = k.Balance
				break
			}
		}

		if config.GetConfig() != nil {
			minThreshold = config.GetConfig().App.MinBalanceThreshold
		}

		if keyExists {
			// 如果密钥存在但启用失败，很可能是余额不足
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("无法启用API密钥：余额 %.2f 低于最低阈值 %.2f", balance, minThreshold),
			})
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "API密钥未找到",
			})
		}
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
		"deleted":      len(zeroOrNegativeBalanceKeys),
		"deleted_keys": zeroOrNegativeBalanceKeys,
	})
}

// handleDeleteLowBalanceKeys 处理删除余额低于指定阈值的API密钥的请求
func handleDeleteLowBalanceKeys(c *gin.Context) {
	// 获取阈值参数
	thresholdStr := c.Param("threshold")
	threshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的阈值参数: %v", err),
		})
		return
	}

	// 获取活动的API密钥
	keys := config.GetActiveApiKeys()

	// 过滤出余额低于阈值的API密钥
	var lowBalanceKeys []string
	for _, key := range keys {
		if key.Balance < threshold {
			lowBalanceKeys = append(lowBalanceKeys, key.Key)
		}
	}

	// 标记这些API密钥为删除状态
	for _, key := range lowBalanceKeys {
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
		"message":      fmt.Sprintf("已删除 %d 个余额低于 %.2f 的API密钥", len(lowBalanceKeys), threshold),
		"deleted":      len(lowBalanceKeys),
		"deleted_keys": lowBalanceKeys,
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
		"security": gin.H{
			"password_enabled":   cfg.Security.PasswordEnabled,
			"expiration_minutes": cfg.Security.ExpirationMinutes,
			"api_key_enabled":    cfg.Security.ApiKeyEnabled,
			"api_key":            cfg.Security.ApiKey,
			// 不返回哈希后的密码
		},
		"app": gin.H{
			"title":                         cfg.App.Title,
			"min_balance_threshold":         cfg.App.MinBalanceThreshold,
			"max_balance_display":           cfg.App.MaxBalanceDisplay,
			"items_per_page":                cfg.App.ItemsPerPage,
			"max_stats_entries":             cfg.App.MaxStatsEntries,
			"recovery_interval":             cfg.App.RecoveryInterval,
			"max_consecutive_failures":      cfg.App.MaxConsecutiveFailures,
			"balance_weight":                cfg.App.BalanceWeight,
			"success_rate_weight":           cfg.App.SuccessRateWeight,
			"rpm_weight":                    cfg.App.RPMWeight,
			"tpm_weight":                    cfg.App.TPMWeight,
			"auto_update_interval":          cfg.App.AutoUpdateInterval,
			"stats_refresh_interval":        cfg.App.StatsRefreshInterval,
			"rate_refresh_interval":         cfg.App.RateRefreshInterval,
			"auto_delete_zero_balance_keys": cfg.App.AutoDeleteZeroBalanceKeys,
			"refresh_used_keys_interval":    cfg.App.RefreshUsedKeysInterval,
			"hide_icon":                     cfg.App.HideIcon,
			"disabled_models":               cfg.App.DisabledModels,
		},
		"log": gin.H{
			"max_size_mb": cfg.Log.MaxSizeMB,
			"level":       cfg.Log.Level,
		},
		"request_settings": gin.H{
			"http_client": gin.H{
				"response_header_timeout":   cfg.RequestSettings.HttpClient.ResponseHeaderTimeout,
				"tls_handshake_timeout":     cfg.RequestSettings.HttpClient.TLSHandshakeTimeout,
				"idle_conn_timeout":         cfg.RequestSettings.HttpClient.IdleConnTimeout,
				"expect_continue_timeout":   cfg.RequestSettings.HttpClient.ExpectContinueTimeout,
				"max_idle_conns":            cfg.RequestSettings.HttpClient.MaxIdleConns,
				"max_idle_conns_per_host":   cfg.RequestSettings.HttpClient.MaxIdleConnsPerHost,
				"keep_alive":                cfg.RequestSettings.HttpClient.KeepAlive,
				"connect_timeout":           cfg.RequestSettings.HttpClient.ConnectTimeout,
			},
			"proxy_handler": gin.H{
				"inference_timeout":  cfg.RequestSettings.ProxyHandler.InferenceTimeout,
				"standard_timeout":   cfg.RequestSettings.ProxyHandler.StandardTimeout,
				"stream_timeout":     cfg.RequestSettings.ProxyHandler.StreamTimeout,
				"heartbeat_interval": cfg.RequestSettings.ProxyHandler.HeartbeatInterval,
				"progress_interval":  cfg.RequestSettings.ProxyHandler.ProgressInterval,
				"buffer_threshold":   cfg.RequestSettings.ProxyHandler.BufferThreshold,
				"max_flush_interval": cfg.RequestSettings.ProxyHandler.MaxFlushInterval,
				"max_concurrency":    cfg.RequestSettings.ProxyHandler.MaxConcurrency,
			},
			"database": gin.H{
				"conn_max_lifetime": cfg.RequestSettings.Database.ConnMaxLifetime,
				"max_idle_conns":    cfg.RequestSettings.Database.MaxIdleConns,
			},
			"defaults": gin.H{
				"max_tokens":         cfg.RequestSettings.Defaults.MaxTokens,
				"image_size":         cfg.RequestSettings.Defaults.ImageSize,
				"max_chunks_per_doc": cfg.RequestSettings.Defaults.MaxChunksPerDoc,
			},
		},
	}

	// 返回配置信息
	c.JSON(http.StatusOK, configData)
}

// handleSaveSettings 处理保存系统设置的请求
func handleSaveSettings(c *gin.Context) {
	// 获取设置数据
	var configData map[string]interface{}
	if err := c.ShouldBindJSON(&configData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("无效的请求数据: %v", err),
		})
		return
	}

	// 获取当前配置作为基础
	currentConfig := config.GetConfig()
	if currentConfig == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "无法获取当前系统配置",
		})
		return
	}

	// 验证密码保护和API密钥验证的设置
	if security, ok := configData["security"].(map[string]interface{}); ok {
		passwordEnabled, passwordEnabledExists := security["password_enabled"].(bool)
		password, passwordExists := security["password"].(string)

		fmt.Println("passwordEnabledExists", passwordEnabledExists)
		fmt.Println("passwordEnabled", passwordEnabled)
		fmt.Println("passwordExists", passwordExists)
		fmt.Println("password", password)

		// 检查是否尝试启用密码保护但没有提供密码
		if passwordEnabledExists && passwordEnabled {
			// 如果当前没有密码，且没有提供新密码，则返回错误
			if currentConfig.Security.Password == "" && (!passwordExists || password == "") {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "启用密码保护时必须设置密码",
					"code":  "password_required",
				})
				return
			}
		}

		// 检查是否尝试启用API密钥验证但没有提供API密钥
		apiKeyEnabled, apiKeyEnabledExists := security["api_key_enabled"].(bool)
		apiKey, apiKeyExists := security["api_key"].(string)

		if apiKeyEnabledExists && apiKeyEnabled {
			// 如果当前没有API密钥，且没有提供新API密钥，则返回错误
			if currentConfig.Security.ApiKey == "" && (!apiKeyExists || apiKey == "") {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "启用API密钥验证时必须设置API密钥",
					"code":  "api_key_required",
				})
				return
			}
		}
	}

	// 创建一个新的Config对象进行更新
	newConfig := *currentConfig

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

	// 安全设置
	if security, ok := configData["security"].(map[string]interface{}); ok {
		if passwordEnabled, ok := security["password_enabled"].(bool); ok {
			newConfig.Security.PasswordEnabled = passwordEnabled
		}
		if expirationMinutes, ok := security["expiration_minutes"].(float64); ok {
			newConfig.Security.ExpirationMinutes = int(expirationMinutes)
		}

		// 处理API密钥设置
		if apiKeyEnabled, ok := security["api_key_enabled"].(bool); ok {
			newConfig.Security.ApiKeyEnabled = apiKeyEnabled
		}
		if apiKey, ok := security["api_key"].(string); ok {
			// 允许空API密钥，这样用户可以清除API密钥设置
			newConfig.Security.ApiKey = apiKey
		}

		// 处理密码，如果提供了新密码则进行哈希处理
		if password, ok := security["password"].(string); ok && password != "" {
			// 使用SHA256哈希保存密码
			newConfig.Security.Password = auth.HashPassword(password)
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

		// 新配置项
		if autoDeleteZeroBalance, ok := app["auto_delete_zero_balance_keys"].(bool); ok {
			newConfig.App.AutoDeleteZeroBalanceKeys = autoDeleteZeroBalance
		}
		if refreshUsedKeysInterval, ok := app["refresh_used_keys_interval"].(float64); ok {
			newConfig.App.RefreshUsedKeysInterval = int(refreshUsedKeysInterval)
		}

		// 处理禁用的模型列表
		if disabledModels, ok := app["disabled_models"].([]interface{}); ok {
			newConfig.App.DisabledModels = make([]string, 0, len(disabledModels))
			for _, model := range disabledModels {
				if modelID, ok := model.(string); ok {
					newConfig.App.DisabledModels = append(newConfig.App.DisabledModels, modelID)
				}
			}
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

	// 请求设置
	if requestSettings, ok := configData["request_settings"].(map[string]interface{}); ok {
		// HTTP客户端设置
		if httpClient, ok := requestSettings["http_client"].(map[string]interface{}); ok {
			if val, ok := httpClient["response_header_timeout"].(float64); ok {
				newConfig.RequestSettings.HttpClient.ResponseHeaderTimeout = int(val)
			}
			if val, ok := httpClient["tls_handshake_timeout"].(float64); ok {
				newConfig.RequestSettings.HttpClient.TLSHandshakeTimeout = int(val)
			}
			if val, ok := httpClient["idle_conn_timeout"].(float64); ok {
				newConfig.RequestSettings.HttpClient.IdleConnTimeout = int(val)
			}
			if val, ok := httpClient["expect_continue_timeout"].(float64); ok {
				newConfig.RequestSettings.HttpClient.ExpectContinueTimeout = int(val)
			}
			if val, ok := httpClient["max_idle_conns"].(float64); ok {
				newConfig.RequestSettings.HttpClient.MaxIdleConns = int(val)
			}
			if val, ok := httpClient["max_idle_conns_per_host"].(float64); ok {
				newConfig.RequestSettings.HttpClient.MaxIdleConnsPerHost = int(val)
			}
			if val, ok := httpClient["keep_alive"].(float64); ok {
				newConfig.RequestSettings.HttpClient.KeepAlive = int(val)
			}
			if val, ok := httpClient["connect_timeout"].(float64); ok {
				newConfig.RequestSettings.HttpClient.ConnectTimeout = int(val)
			}
			if val, ok := httpClient["max_response_header_bytes"].(float64); ok {
				newConfig.RequestSettings.HttpClient.MaxResponseHeaderBytes = int(val)
			}
		}
		// 代理处理设置
		if proxyHandler, ok := requestSettings["proxy_handler"].(map[string]interface{}); ok {
			if val, ok := proxyHandler["inference_timeout"].(float64); ok {
				newConfig.RequestSettings.ProxyHandler.InferenceTimeout = int(val)
			}
			if val, ok := proxyHandler["standard_timeout"].(float64); ok {
				newConfig.RequestSettings.ProxyHandler.StandardTimeout = int(val)
			}
			if val, ok := proxyHandler["stream_timeout"].(float64); ok {
				newConfig.RequestSettings.ProxyHandler.StreamTimeout = int(val)
			}
			if val, ok := proxyHandler["heartbeat_interval"].(float64); ok {
				newConfig.RequestSettings.ProxyHandler.HeartbeatInterval = int(val)
			}
			if val, ok := proxyHandler["progress_interval"].(float64); ok {
				newConfig.RequestSettings.ProxyHandler.ProgressInterval = int(val)
			}
			if val, ok := proxyHandler["buffer_threshold"].(float64); ok {
				newConfig.RequestSettings.ProxyHandler.BufferThreshold = int(val)
			}
			if val, ok := proxyHandler["max_flush_interval"].(float64); ok {
				newConfig.RequestSettings.ProxyHandler.MaxFlushInterval = int(val)
			}
			if val, ok := proxyHandler["max_concurrency"].(float64); ok {
				newConfig.RequestSettings.ProxyHandler.MaxConcurrency = int(val)
			}
		}
		// 数据库设置
		if database, ok := requestSettings["database"].(map[string]interface{}); ok {
			if val, ok := database["conn_max_lifetime"].(float64); ok {
				newConfig.RequestSettings.Database.ConnMaxLifetime = int(val)
			}
			if val, ok := database["max_idle_conns"].(float64); ok {
				newConfig.RequestSettings.Database.MaxIdleConns = int(val)
			}
		}
		// 默认值设置
		if defaults, ok := requestSettings["defaults"].(map[string]interface{}); ok {
			if val, ok := defaults["max_tokens"].(float64); ok {
				newConfig.RequestSettings.Defaults.MaxTokens = int(val)
			}
			if val, ok := defaults["image_size"].(string); ok {
				newConfig.RequestSettings.Defaults.ImageSize = val
			}
			if val, ok := defaults["max_chunks_per_doc"].(float64); ok {
				newConfig.RequestSettings.Defaults.MaxChunksPerDoc = int(val)
			}
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
		executableDir := filepath.Dir(execPath)

		// 设置工作目录
		cmd.Dir = executableDir

		// 传递所有当前环境变量
		cmd.Env = os.Environ()

		// 检查是否为GUI模式
		if runtime.GOOS == "windows" {
			// Windows系统GUI模式检测
			guiMode := os.Getenv("FLOWSILICON_GUI")
			if guiMode == "1" {
				logger.Info("以GUI模式重启程序")

				// 设置Windows特定的重启选项
				utils.SetupWindowsRestartCommand(cmd, true)
			} else {
				logger.Info("以控制台模式重启程序")
				utils.SetupWindowsRestartCommand(cmd, false)
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

				// 设置平台特定的重启选项（Linux上是空操作）
				utils.SetupWindowsRestartCommand(cmd, true)
			} else {
				logger.Info("以控制台模式重启程序")
				utils.SetupWindowsRestartCommand(cmd, false)
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
	utils.SetCommonHeaders(req, authToken)

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

// getModelsHandler 获取所有模型列表
func getModelsHandler(c *gin.Context) {
	// 从数据库中获取所有模型
	models, err := model.GetAllModels()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取模型列表失败: %v", err),
		})
		return
	}

	// 提取模型ID
	var modelIds []string
	var freeModels []string // 免费模型ID列表
	var giftModels []string // 赠费模型ID列表
	for _, m := range models {
		modelIds = append(modelIds, m.ID)
		if m.IsFree {
			freeModels = append(freeModels, m.ID)
		}
		if m.IsGiftable {
			giftModels = append(giftModels, m.ID)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"data":        modelIds,
		"free_models": freeModels, // 返回免费模型列表
		"gift_models": giftModels, // 返回赠费模型列表
	})
}

// syncModelsHandler 从API获取模型列表并更新数据库
func syncModelsHandler(c *gin.Context) {
	// 获取当前配置的API基础URL
	cfg := config.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取配置失败",
		})
		return
	}

	baseURL := cfg.ApiProxy.BaseURL
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "API基础URL未配置",
		})
		return
	}

	// 从远程API获取模型列表
	modelIds, count, err := fetchRemoteModels(baseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "从API获取模型列表失败: " + err.Error(),
		})
		return
	}

	// 获取数据库中的模型数量
	dbCount, err := model.GetModelsCount()
	if err != nil {
		logger.Error("获取数据库模型数量失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取数据库模型数量失败: " + err.Error(),
		})
		return
	}

	// 比较远程和本地的模型数量，如果数量一致且非强制同步，则跳过同步
	forceSync := c.DefaultQuery("force", "false") == "true"
	if dbCount == count && !forceSync {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "模型数量一致，无需同步",
			"count":   dbCount,
		})
		return
	}

	// 保存获取到的模型列表到数据库
	savedCount, err := model.SaveModels(modelIds)
	if err != nil {
		logger.Error("保存模型列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "保存模型列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "成功同步模型列表",
		"count":   savedCount,
	})
}

// 从远程API获取模型列表
func fetchRemoteModels(baseURL string) ([]string, int, error) {

	// 构建API请求URL
	url := strings.TrimRight(baseURL, "/") + "/v1/models"

	// 创建请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}

	apikeys := config.GetActiveApiKeys()
	utils.SetCommonHeaders(req, apikeys[0].Key)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, 0, err
	}

	// 解析响应
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, 0, err
	}

	// 提取模型列表
	data, ok := result["data"].([]interface{})
	if !ok {
		logger.Error("解析模型列表失败: data字段不是数组")
		return nil, 0, nil
	}

	// 提取模型ID
	var modelIds []string
	for _, item := range data {
		model, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		id, ok := model["id"].(string)
		if !ok {
			continue
		}

		modelIds = append(modelIds, id)
	}

	return modelIds, len(modelIds), nil
}

// updateModelStrategyHandler 更新模型策略
func updateModelStrategyHandler(c *gin.Context) {
	// 获取请求参数
	var req struct {
		ModelID    string `json:"model_id"`
		StrategyID int    `json:"strategy_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("解析请求参数失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "解析请求参数失败: " + err.Error(),
		})
		return
	}

	// 如果模型ID为空或策略ID小于0，返回错误
	if req.ModelID == "" || req.StrategyID < 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "模型ID不能为空且策略ID必须大于等于0",
		})
		return
	}

	// 如果策略ID为0，根据模型是否为免费模型设置默认策略
	if req.StrategyID == 0 {
		// 从数据库中获取模型信息
		models, err := model.GetAllModels()
		if err == nil {
			for _, m := range models {
				if m.ID == req.ModelID {
					if m.IsFree {
						req.StrategyID = 8 // 免费模型使用策略8
					} else {
						req.StrategyID = 6 // 非免费模型使用策略6
					}
					break
				}
			}
		} else {
			logger.Error("获取模型信息失败，使用默认策略6: %v", err)
			req.StrategyID = 6 // 默认使用策略6
		}
	}

	// 更新模型策略
	err := model.UpdateModelStrategy(req.ModelID, req.StrategyID)
	if err != nil {
		logger.Error("更新模型策略失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "更新模型策略失败: " + err.Error(),
		})
		return
	}

	// 更新配置中的模型策略
	cfg := config.GetConfig()
	if cfg.App.ModelKeyStrategies == nil {
		cfg.App.ModelKeyStrategies = make(map[string]int)
	}
	cfg.App.ModelKeyStrategies[req.ModelID] = req.StrategyID
	config.UpdateConfig(cfg)
	config.SaveConfigToDB()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("成功将模型 %s 的策略更新为 %d", req.ModelID, req.StrategyID),
	})
}

// getModelsAPIHandler 获取所有模型信息（包括类型和策略）
func getModelsAPIHandler(c *gin.Context) {
	// 从数据库中获取所有模型
	models, err := model.GetAllModels()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取模型列表失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"models":  models,
	})
}

// getModelsStatusHandler 获取模型状态信息
func getModelsStatusHandler(c *gin.Context) {
	// 在实际应用中，这里从数据库或配置中读取禁用模型的信息
	// 这里我们使用一个简单的示例
	disabledModels := []string{}

	// 从配置文件或其他存储中获取禁用的模型
	cfg := config.GetConfig()
	if cfg != nil && cfg.App.DisabledModels != nil {
		disabledModels = cfg.App.DisabledModels
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"disabled_models": disabledModels,
	})
}

// updateModelsHandler 批量更新模型信息
func updateModelsHandler(c *gin.Context) {
	var req struct {
		Models         []model.Model `json:"models"`
		DisabledModels []string      `json:"disabled_models"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("无效的请求格式: %v", err),
		})
		return
	}

	// 开始事务更新模型信息
	tx, err := model.BeginTransaction()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("开始事务失败: %v", err),
		})
		return
	}

	// 使用defer带条件地回滚事务（只有在发生错误时才回滚）
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	// 更新模型信息
	for _, m := range req.Models {
		// 更新类型 - 使用事务版本
		if err := model.UpdateModelTypeWithTx(tx, m.ID, m.Type); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("更新模型类型失败: %v", err),
			})
			return
		}

		// 更新策略 - 使用事务版本
		if err := model.UpdateModelStrategyWithTx(tx, m.ID, m.StrategyID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("更新模型策略失败: %v", err),
			})
			return
		}

		// 更新免费状态 - 使用事务版本
		modelIds := []string{m.ID}
		if _, err := model.UpdateModelFreeStatusWithTx(tx, modelIds, m.IsFree); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("更新模型免费状态失败: %v", err),
			})
			return
		}

		// 更新赠费状态 - 使用事务版本
		if _, err := model.UpdateModelGiftableStatusWithTx(tx, modelIds, m.IsGiftable); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("更新模型赠费状态失败: %v", err),
			})
			return
		}
	}

	// 所有更新操作成功后才提交事务
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("提交事务失败: %v", err),
		})
		return
	}

	// 标记事务已提交
	committed = true

	// 更新禁用模型列表
	cfg := config.GetConfig()
	if cfg != nil {
		cfg.App.DisabledModels = req.DisabledModels
		config.UpdateConfig(cfg)
		config.SaveConfigToDB()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "模型信息更新成功",
	})
}

// updateModelTypeHandler 更新模型类型
func updateModelTypeHandler(c *gin.Context) {
	// 解析请求参数
	var req struct {
		ModelID   string `json:"model_id"`
		ModelType int    `json:"model_type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("解析请求参数失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "解析请求参数失败: " + err.Error(),
		})
		return
	}

	// 验证参数
	if req.ModelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "模型ID不能为空",
		})
		return
	}

	if req.ModelType < 1 || req.ModelType > 7 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的模型类型，必须在1-7之间",
		})
		return
	}

	// 更新模型类型
	if err := model.UpdateModelType(req.ModelID, req.ModelType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("更新模型类型失败: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("成功将模型 %s 的类型更新为 %d", req.ModelID, req.ModelType),
	})
}

// handleModelManagementPage 处理模型管理页面请求
func handleModelManagementPage(c *gin.Context) {
	// 获取版本号
	version := config.GetVersion()
	if version == "" {
		version = "v1.0.0" // 默认版本号
	}

	// 获取配置
	cfg := config.GetConfig()
	if cfg == nil {
		// 如果配置为空，使用默认标题
		c.HTML(http.StatusOK, "llmmodel.html", gin.H{
			"title":   "流动硅基",
			"version": version,
		})
		return
	}

	// 使用配置中的标题
	c.HTML(http.StatusOK, "llmmodel.html", gin.H{
		"title":   cfg.App.Title,
		"version": version,
	})
}

// deleteModelStrategyHandler 删除模型策略
func deleteModelStrategyHandler(c *gin.Context) {
	// 获取请求参数
	var req struct {
		ModelID string `json:"model_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error("解析请求参数失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "解析请求参数失败: " + err.Error(),
		})
		return
	}

	// 如果模型ID为空，返回错误
	if req.ModelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "模型ID不能为空",
		})
		return
	}

	// 从数据库中删除模型策略
	err := model.DeleteModelStrategy(req.ModelID)
	if err != nil {
		logger.Error("删除模型策略失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除模型策略失败: " + err.Error(),
		})
		return
	}

	// 更新配置中的模型策略
	cfg := config.GetConfig()
	if cfg.App.ModelKeyStrategies != nil {
		// 从配置中删除模型策略
		delete(cfg.App.ModelKeyStrategies, req.ModelID)
		config.UpdateConfig(cfg)
		config.SaveConfigToDB()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("成功从数据库删除模型 %s 的策略", req.ModelID),
	})
}

// handleLoginPage 处理登录页面请求
func handleLoginPage(c *gin.Context) {
	// 获取重定向路径（如果有）
	redirect, _ := c.Cookie("redirect_after_login")

	// 如果从查询参数中也传入了重定向路径，优先使用它
	if redirectParam := c.Query("redirect"); redirectParam != "" {
		redirect = redirectParam
	}

	// 清除重定向cookie
	c.SetCookie("redirect_after_login", "", -1, "/", "", false, false)

	// 获取错误信息（如果有）
	error := c.Query("error")

	c.HTML(http.StatusOK, "login.html", gin.H{
		"title":    config.GetConfig().App.Title,
		"redirect": redirect,
		"error":    error,
	})
}

// handleLogin 处理登录请求
func handleLogin(c *gin.Context) {
	// 获取表单参数
	password := c.PostForm("password")
	redirect := c.PostForm("redirect")

	// 判断是否是AJAX请求
	isAjax := c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
		c.GetHeader("Accept") == "application/json" ||
		c.Query("format") == "json"

	// 如果没有提供重定向地址，默认使用首页
	if redirect == "" {
		redirect = "/"
	}

	// 获取配置中的密码
	cfg := config.GetConfig()
	if cfg == nil || cfg.Security.Password == "" {
		// 如果没有设置密码
		if isAjax {
			c.JSON(http.StatusOK, gin.H{
				"code":     200,
				"message":  "认证成功",
				"redirect": redirect,
			})
		} else {
			c.Redirect(http.StatusFound, redirect)
		}
		return
	}

	// 验证密码
	if !auth.VerifyPassword(password, cfg.Security.Password) {
		// 密码错误
		if isAjax {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "密码错误，请重试",
			})
		} else {
			c.Redirect(http.StatusFound, fmt.Sprintf("/login?error=%s&redirect=%s",
				"密码错误，请重试", redirect))
		}
		return
	}

	// 确定有效期（默认最少60秒）
	expirationMinutes := cfg.Security.ExpirationMinutes
	if expirationMinutes <= 0 {
		expirationMinutes = 1 // 默认至少1分钟
	}

	// 生成Cookie
	cookieValue, err := auth.GenerateCookie(expirationMinutes)
	if err != nil {
		logger.Error("生成认证Cookie失败: %v", err)
		if isAjax {
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "登录处理失败，请稍后重试",
			})
		} else {
			c.Redirect(http.StatusFound, fmt.Sprintf("/login?error=%s&redirect=%s",
				"登录处理失败，请稍后重试", redirect))
		}
		return
	}

	// 设置Cookie - 始终使用绝对过期时间
	maxAge := expirationMinutes * 60 // 转换为秒

	// 记录日志
	logger.Info("设置认证Cookie，有效期: %d分钟", expirationMinutes)

	c.SetCookie(middleware.AuthCookieName, cookieValue, maxAge, "/", "", false, true)

	// 响应请求
	if isAjax {
		c.JSON(http.StatusOK, gin.H{
			"code":     200,
			"message":  "登录成功",
			"redirect": redirect,
		})
	} else {
		c.Redirect(http.StatusFound, redirect)
	}
}

// handleLogout 处理登出请求
func handleLogout(c *gin.Context) {
	// 判断是否是AJAX请求
	isAjax := c.GetHeader("X-Requested-With") == "XMLHttpRequest" ||
		c.GetHeader("Accept") == "application/json" ||
		c.Query("format") == "json"

	// 清除认证Cookie
	c.SetCookie(middleware.AuthCookieName, "", -1, "/", "", false, true)

	// 响应请求
	if isAjax {
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "已成功登出",
		})
	} else {
		// 重定向到登录页面
		c.Redirect(http.StatusFound, "/login")
	}
}

// handleAuthCheck 处理检查认证状态的请求
func handleAuthCheck(c *gin.Context) {
	// 获取当前配置
	cfg := config.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "服务器内部错误",
		})
		return
	}

	// 检查是否启用了密码保护
	if !cfg.Security.PasswordEnabled {
		// 未启用密码保护，直接返回已认证状态
		c.JSON(http.StatusOK, gin.H{
			"code":    200,
			"message": "已认证",
		})
		return
	}

	// 验证认证状态
	cookie, err := c.Cookie(middleware.AuthCookieName)
	if err != nil || cookie == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "未认证",
		})
		return
	}

	// 验证令牌
	valid, err := auth.ParseCookie(cookie)
	if err != nil || !valid {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    401,
			"message": "认证已过期",
		})
		return
	}

	// 认证有效
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "已认证",
	})
}

// getTopModelsHandler 获取调用次数最多的模型
func getTopModelsHandler(c *gin.Context) {
	// 默认最多返回3个
	limit := 3

	// 获取模型调用次数最多的模型
	models, err := model.GetTopModels(limit)
	if err != nil {
		logger.Error("获取常用模型失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取常用模型失败: " + err.Error(),
		})
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"models":  models,
	})
}
