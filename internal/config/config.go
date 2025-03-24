/**
  @author: Hanhai
  @since: 2025/3/16 20:44:00
  @desc:
**/

package config

import (
	"flowsilicon/internal/logger"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

var (
	config     *Config
	configOnce sync.Once
	apiKeys    []ApiKey
	keysMutex  sync.RWMutex

	// 请求统计相关
	requestStats []RequestStats // 保存最近的请求统计数据
	statsLock    sync.RWMutex   // 用于保护requestStats

	// 当天累计数据
	dailyRequestCount int       // 当天累计请求数
	dailyTokenCount   int       // 当天累计令牌数
	lastResetDay      time.Time // 上次重置日期
)

// Config 应用配置结构
type Config struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
	ApiProxy struct {
		BaseURL    string      `mapstructure:"base_url"`
		ModelIndex int         `mapstructure:"model_index"` // 当前使用的模型索引
		Retry      RetryConfig `mapstructure:"retry"`       // 重试配置
	} `mapstructure:"api_proxy"`
	Proxy struct {
		HttpProxy  string `mapstructure:"http_proxy"`  // HTTP代理地址
		HttpsProxy string `mapstructure:"https_proxy"` // HTTPS代理地址
		SocksProxy string `mapstructure:"socks_proxy"` // SOCKS5代理地址
		ProxyType  string `mapstructure:"proxy_type"`  // 代理类型：http, https, socks5
		Enabled    bool   `mapstructure:"enabled"`     // 是否启用代理
	} `mapstructure:"proxy"`
	App struct {
		Title                  string  `mapstructure:"title"`                    // 应用标题
		MinBalanceThreshold    float64 `mapstructure:"min_balance_threshold"`    // 最低余额阈值
		MaxBalanceDisplay      float64 `mapstructure:"max_balance_display"`      // 余额显示最大值
		ItemsPerPage           int     `mapstructure:"items_per_page"`           // 每页显示的密钥数量
		MaxStatsEntries        int     `mapstructure:"max_stats_entries"`        // 最大统计条目数
		RecoveryInterval       int     `mapstructure:"recovery_interval"`        // 恢复检查间隔（分钟）
		MaxConsecutiveFailures int     `mapstructure:"max_consecutive_failures"` // 最大连续失败次数
		// 权重配置
		BalanceWeight     float64 `mapstructure:"balance_weight"`      // 余额评分权重
		SuccessRateWeight float64 `mapstructure:"success_rate_weight"` // 成功率评分权重
		RPMWeight         float64 `mapstructure:"rpm_weight"`          // RPM评分权重
		TPMWeight         float64 `mapstructure:"tpm_weight"`          // TPM评分权重
		// 自动更新配置
		AutoUpdateInterval   int `mapstructure:"auto_update_interval"`   // API密钥信息自动更新间隔（秒）
		StatsRefreshInterval int `mapstructure:"stats_refresh_interval"` // 系统概要自动刷新间隔（秒）
		RateRefreshInterval  int `mapstructure:"rate_refresh_interval"`  // 速率监控自动刷新间隔（秒）
		// 模型特定的密钥选择策略
		ModelKeyStrategies map[string]int `mapstructure:"model_key_strategies"` // 模型特定的密钥选择策略
		// 系统托盘图标设置
		HideIcon bool `mapstructure:"hide_icon"` // 是否隐藏系统托盘图标
	} `mapstructure:"app"`
	Log struct {
		MaxSizeMB int    `mapstructure:"max_size_mb"` // 日志文件最大大小（MB）
		Level     string `mapstructure:"level"`       // 日志等级（debug, info, warn, error, fatal）
	} `mapstructure:"log"`
}

// ApiKey API密钥结构
type ApiKey struct {
	Key      string  `json:"key"`
	Balance  float64 `json:"balance"`
	LastUsed int64   `json:"last_used"` // Unix时间戳
	// 新增字段
	TotalCalls          int     `json:"total_calls"`          // 总调用次数
	SuccessCalls        int     `json:"success_calls"`        // 成功调用次数
	SuccessRate         float64 `json:"success_rate"`         // 成功率
	ConsecutiveFailures int     `json:"consecutive_failures"` // 连续失败次数
	Disabled            bool    `json:"disabled"`             // 是否禁用
	DisabledAt          int64   `json:"disabled_at"`          // 禁用时间戳
	LastTested          int64   `json:"last_tested"`          // 最后一次测试时间戳
	// 新增RPM和TPM统计
	RequestsPerMinute int            `json:"rpm"` // 每分钟请求数
	TokensPerMinute   int            `json:"tpm"` // 每分钟令牌数
	RecentRequests    []RequestStats `json:"-"`   // 最近的请求统计，不序列化
	// 新增得分字段
	Score float64 `json:"score"` // 综合得分
	// 新增删除标记字段
	Delete bool `json:"delete"` // 是否标记为删除
	// 新增使用标记字段
	IsUsed bool `json:"is_used"` // 是否被使用过
}

// RequestStats 请求统计结构
type RequestStats struct {
	Timestamp    int64 `json:"timestamp"`     // 时间戳
	RequestCount int   `json:"request_count"` // 请求数
	TokenCount   int   `json:"token_count"`   // 令牌数
}

// standardizeModelKeyStrategies 统一模型名称的大小写处理
func standardizeModelKeyStrategies() {
	if config == nil || config.App.ModelKeyStrategies == nil {
		return
	}

	// 创建一个新的映射，用于存储标准化后的键值对
	standardizedStrategies := make(map[string]int)

	// 将所有键统一为小写形式
	for model, strategy := range config.App.ModelKeyStrategies {
		standardModel := model // 保留原始格式，以便日志记录
		logger.Info("模型策略配置: 原始=%s, 策略=%d", model, strategy)
		standardizedStrategies[standardModel] = strategy
	}

	// 替换原有映射
	config.App.ModelKeyStrategies = standardizedStrategies
	logger.Info("模型策略配置标准化完成: %v", config.App.ModelKeyStrategies)
}

// GetConfig 获取配置
func GetConfig() *Config {

	return config
}

// GetApiKeys 获取所有API密钥
func GetApiKeys() []ApiKey {
	keysMutex.RLock()
	defer keysMutex.RUnlock()

	// 过滤掉标记为删除的密钥
	filteredKeys := make([]ApiKey, 0, len(apiKeys))
	for _, key := range apiKeys {
		if !key.Delete {
			filteredKeys = append(filteredKeys, key)
		}
	}

	// 返回副本以避免外部修改
	keysCopy := make([]ApiKey, len(filteredKeys))
	copy(keysCopy, filteredKeys)
	return keysCopy
}

// MaskKey 遮盖API密钥，只显示前4位和后4位
func MaskKey(key string) string {
	if len(key) <= 8 {
		return key // 密钥太短，返回原样
	}

	prefix := key[:4]
	suffix := key[len(key)-4:]
	return prefix + "..." + suffix
}

// AddApiKey 添加新的API密钥
func AddApiKey(key string, balance float64) {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	// 检查密钥是否已存在
	for i, k := range apiKeys {
		if k.Key == key {
			// 更新现有密钥的余额
			apiKeys[i].Balance = balance
			// 检查余额并设置禁用状态
			if balance < config.App.MinBalanceThreshold {
				apiKeys[i].Disabled = true
				apiKeys[i].DisabledAt = time.Now().Unix()
				log.Printf("API密钥 %s 余额 %.2f 低于阈值 %.2f，已自动禁用",
					key, balance, config.App.MinBalanceThreshold)
			} else {
				apiKeys[i].Disabled = false
				apiKeys[i].DisabledAt = 0
			}

			// 保存更新到数据库
			if err := AddApiKeyToDB(apiKeys[i]); err != nil {
				logger.Error("保存API密钥到数据库失败: %v", err)
			}
			return
		}
	}

	// 添加新密钥
	newKey := ApiKey{
		Key:     key,
		Balance: balance,
	}

	// 检查余额并设置初始禁用状态
	if balance < config.App.MinBalanceThreshold {
		newKey.Disabled = true
		newKey.DisabledAt = time.Now().Unix()
		log.Printf("新增API密钥 %s 余额 %.2f 低于阈值 %.2f，已自动禁用",
			key, balance, config.App.MinBalanceThreshold)
	}

	apiKeys = append(apiKeys, newKey)

	// 保存新密钥到数据库
	if err := AddApiKeyToDB(newKey); err != nil {
		logger.Error("添加API密钥到数据库失败: %v", err)
	}
}

// UpdateApiKeyBalance 更新 API 密钥余额
func UpdateApiKeyBalance(key string, balance float64) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			// 更新余额
			apiKeys[i].Balance = balance

			// 如果余额低于余额阈值且密钥未禁用，则禁用密钥
			if balance < config.App.MinBalanceThreshold && !k.Disabled {
				apiKeys[i].Disabled = true
				apiKeys[i].DisabledAt = time.Now().Unix()
				log.Printf("API密钥 %s 余额低于阈值 %.2f，已自动禁用", MaskKey(key), config.App.MinBalanceThreshold)
			} else if balance >= config.App.MinBalanceThreshold && k.Disabled && k.DisabledAt == 0 {
				// 如果余额充足且密钥是禁用的（但不是手动禁用的），则启用密钥
				apiKeys[i].Disabled = false
				log.Printf("API密钥 %s 余额已恢复到阈值 %.2f 以上，已自动启用", MaskKey(key), config.App.MinBalanceThreshold)
			}

			// 保存更新到数据库
			keyCopy := apiKeys[i]
			keyCopy.RecentRequests = nil // 清空不需要保存的字段
			if err := AddApiKeyToDB(keyCopy); err != nil {
				logger.Error("更新API密钥余额到数据库失败: %v", err)
			}

			return true
		}
	}

	return false
}

// UpdateApiKeyLastUsed 更新 API 密钥最后使用时间
func UpdateApiKeyLastUsed(key string, timestamp int64) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].LastUsed = timestamp

			// 保存更新到数据库
			if db != nil {
				_, err := db.Exec(`UPDATE `+apikeysTableName+` 
					SET last_used = ? WHERE key = ?`, timestamp, key)
				if err != nil {
					logger.Error("更新API密钥最后使用时间到数据库失败: %v", err)
				}
			}

			return true
		}
	}

	return false
}

// MarkApiKeyAsUsed 标记 API 密钥为已使用
func MarkApiKeyAsUsed(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].IsUsed = true

			// 保存更新到数据库
			if db != nil {
				_, err := db.Exec(`UPDATE `+apikeysTableName+` 
					SET is_used = ? WHERE key = ?`, true, key)
				if err != nil {
					logger.Error("更新API密钥使用状态到数据库失败: %v", err)
				}
			}

			return true
		}
	}
	return false
}

// SortApiKeysByScore 按分数排序API密钥（从高到低）
func SortApiKeysByScore() {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	// 使用快速排序算法
	quickSort(apiKeys, 0, len(apiKeys)-1)
}

// 保留原函数名作为别名，以保持向后兼容性
func SortApiKeysByBalance() {
	// 调用新的按分数排序函数
	SortApiKeysByScore()
}

// quickSort 快速排序算法实现
func quickSort(keys []ApiKey, low, high int) {
	if low < high {
		// 获取分区点
		pi := partition(keys, low, high)
		// 递归排序左右两部分
		quickSort(keys, low, pi-1)
		quickSort(keys, pi+1, high)
	}
}

// partition 快速排序的分区函数
func partition(keys []ApiKey, low, high int) int {
	// 选择最右边的元素作为基准
	pivot := calculateScore(keys[high])
	i := low - 1 // 小于基准值的元素的最后位置

	for j := low; j < high; j++ {
		// 如果当前元素的分数大于基准值
		if calculateScore(keys[j]) > pivot {
			i++ // 移动小于基准值的元素的最后位置
			// 交换元素
			keys[i], keys[j] = keys[j], keys[i]
		}
	}
	// 将基准值放到正确的位置
	keys[i+1], keys[high] = keys[high], keys[i+1]
	return i + 1
}

// calculateScore 计算单个API密钥的综合得分
func calculateScore(key ApiKey) float64 {
	// 获取配置的权重
	cfg := GetConfig()
	balanceWeight := cfg.App.BalanceWeight
	if balanceWeight <= 0 {
		balanceWeight = 0.4 // 默认权重40%
	}

	successRateWeight := cfg.App.SuccessRateWeight
	if successRateWeight <= 0 {
		successRateWeight = 0.3 // 默认权重30%
	}

	rpmWeight := cfg.App.RPMWeight
	if rpmWeight <= 0 {
		rpmWeight = 0.15 // 默认权重15%
	}

	tpmWeight := cfg.App.TPMWeight
	if tpmWeight <= 0 {
		tpmWeight = 0.15 // 默认权重15%
	}

	// 确保权重总和为1
	totalWeight := balanceWeight + successRateWeight + rpmWeight + tpmWeight
	if totalWeight != 1.0 {
		// 归一化权重
		balanceWeight = balanceWeight / totalWeight
		successRateWeight = successRateWeight / totalWeight
		rpmWeight = rpmWeight / totalWeight
		tpmWeight = tpmWeight / totalWeight
	}

	// 如果密钥被禁用，返回最低分
	if key.Disabled {
		return 0
	}

	// 找出最大余额值，用于归一化
	// 使用配置中的最大余额显示值作为归一化基准
	maxBalance := cfg.App.MaxBalanceDisplay
	if maxBalance <= 0 {
		maxBalance = 14.0 // 默认最大余额显示值
	}

	// 1. 余额得分（余额越高，得分越高）
	// 使用归一化的余额值计算得分
	balanceScore := (key.Balance / maxBalance) * balanceWeight
	if balanceScore > balanceWeight {
		balanceScore = balanceWeight // 确保不超过权重上限
	}

	// 2. 成功率得分（成功率越高，得分越高）
	successRateScore := 0.0
	if key.TotalCalls > 0 {
		successRateScore = key.SuccessRate * successRateWeight
	} else {
		// 如果没有调用记录，假设成功率为100%
		successRateScore = successRateWeight
	}

	// 3. RPM得分（RPM越低，得分越高）
	rpmScore := 0.0
	if key.RequestsPerMinute > 0 {
		// 使用1减去归一化的RPM值，这样RPM越低得分越高
		// 假设最大RPM为100，可以根据实际情况调整
		rpmScore = (1 - float64(key.RequestsPerMinute)/100.0) * rpmWeight
		if rpmScore < 0 {
			rpmScore = 0 // 防止负分
		}
	} else {
		rpmScore = rpmWeight // 如果RPM为0，给予最高分
	}

	// 4. TPM得分（TPM越低，得分越高）
	tpmScore := 0.0
	if key.TokensPerMinute > 0 {
		// 使用1减去归一化的TPM值，这样TPM越低得分越高
		// 假设最大TPM为5000，可以根据实际情况调整
		tpmScore = (1 - float64(key.TokensPerMinute)/5000.0) * tpmWeight
		if tpmScore < 0 {
			tpmScore = 0 // 防止负分
		}
	} else {
		tpmScore = tpmWeight // 如果TPM为0，给予最高分
	}

	// 计算综合得分
	totalScore := balanceScore + successRateScore + rpmScore + tpmScore

	//TODO 注释日志
	//记录日志，便于调试
	// log.Printf("API密钥 %s 余额=%.2f(%.2f), 成功率=%.2f(%.2f), RPM=%d(%.2f), TPM=%d(%.2f), 总分=%.2f",
	// 	key.Key[:6]+"******", key.Balance, balanceScore, key.SuccessRate, successRateScore,
	// 	key.RequestsPerMinute, rpmScore, key.TokensPerMinute, tpmScore, totalScore)

	return totalScore
}

// UpdateApiKeySuccess 更新API密钥成功调用统计
func UpdateApiKeySuccess(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].TotalCalls++
			apiKeys[i].SuccessCalls++
			apiKeys[i].SuccessRate = float64(apiKeys[i].SuccessCalls) / float64(apiKeys[i].TotalCalls)
			apiKeys[i].ConsecutiveFailures = 0

			// 保存更新到数据库
			if db != nil {
				_, err := db.Exec(`UPDATE `+apikeysTableName+` 
					SET total_calls = ?, success_calls = ?, success_rate = ?, consecutive_failures = ? 
					WHERE key = ?`,
					apiKeys[i].TotalCalls, apiKeys[i].SuccessCalls, apiKeys[i].SuccessRate, 0, key)
				if err != nil {
					logger.Error("更新API密钥成功调用统计到数据库失败: %v", err)
				}
			}

			return true
		}
	}

	return false
}

// UpdateApiKeyFailure 更新API密钥失败调用统计
func UpdateApiKeyFailure(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].TotalCalls++
			apiKeys[i].SuccessRate = float64(apiKeys[i].SuccessCalls) / float64(apiKeys[i].TotalCalls)
			apiKeys[i].ConsecutiveFailures++

			// 保存更新到数据库
			if db != nil {
				_, err := db.Exec(`UPDATE `+apikeysTableName+` 
					SET total_calls = ?, success_rate = ?, consecutive_failures = ? 
					WHERE key = ?`,
					apiKeys[i].TotalCalls, apiKeys[i].SuccessRate, apiKeys[i].ConsecutiveFailures, key)
				if err != nil {
					logger.Error("更新API密钥失败调用统计到数据库失败: %v", err)
				}
			}

			return true
		}
	}

	return false
}

// DisableApiKey 禁用API密钥
func DisableApiKey(key string) bool {
	keysMutex.Lock()

	var keyFound bool
	var keyDisabledAt int64

	// 先查找密钥并更新状态，但不保存
	for i, k := range apiKeys {
		if k.Key == key {
			// 如果已经禁用，不需要再做操作
			if k.Disabled {
				keysMutex.Unlock()
				return true
			}

			keyFound = true
			keyDisabledAt = time.Now().Unix()

			// 更新内存中的状态
			apiKeys[i].Disabled = true
			apiKeys[i].DisabledAt = keyDisabledAt
			break
		}
	}

	// 如果没找到密钥，直接返回
	if !keyFound {
		keysMutex.Unlock()
		return false
	}

	// 释放锁后再保存到数据库
	keysMutex.Unlock()

	// 保存更新到数据库
	if db != nil {
		_, err := db.Exec(`UPDATE `+apikeysTableName+` 
			SET disabled = ?, disabled_at = ? 
			WHERE key = ?`,
			true, keyDisabledAt, key)
		if err != nil {
			logger.Error("更新API密钥禁用状态到数据库失败: %v", err)
		} else {
			logger.Info("已更新API密钥禁用状态到数据库: %s", MaskKey(key))
		}
	}

	return true
}

// EnableApiKey 启用API密钥
func EnableApiKey(key string) bool {
	keysMutex.Lock()

	var keyFound bool
	var minThreshold float64

	// 首先获取阈值
	if config != nil {
		minThreshold = config.App.MinBalanceThreshold
	}

	// 查找密钥并检查状态
	for i, k := range apiKeys {
		if k.Key == key {
			keyFound = true

			// 如果余额不足，不允许启用
			if k.Balance < minThreshold {
				logger.Error("无法启用API密钥 %s：余额 %.2f 低于阈值 %.2f",
					MaskKey(key), k.Balance, minThreshold)
				keysMutex.Unlock()
				return false
			}

			// 如果已经启用，不需要再做操作
			if !k.Disabled {
				keysMutex.Unlock()
				return true
			}

			// 更新内存中的状态
			apiKeys[i].Disabled = false
			apiKeys[i].DisabledAt = 0
			apiKeys[i].ConsecutiveFailures = 0
			break
		}
	}

	// 如果没找到密钥，直接返回
	if !keyFound {
		keysMutex.Unlock()
		return false
	}

	// 释放锁后再保存到数据库
	keysMutex.Unlock()

	// 保存更新到数据库
	if db != nil {
		_, err := db.Exec(`UPDATE `+apikeysTableName+` 
			SET disabled = ?, disabled_at = ?, consecutive_failures = ? 
			WHERE key = ?`,
			false, 0, 0, key)
		if err != nil {
			logger.Error("更新API密钥启用状态到数据库失败: %v", err)
		} else {
			logger.Info("已更新API密钥启用状态到数据库: %s", MaskKey(key))
		}
	}

	return true
}

// UpdateApiKeyLastTested 更新API密钥最后测试时间
func UpdateApiKeyLastTested(key string, timestamp int64) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].LastTested = timestamp
			return true
		}
	}

	return false
}

// SortApiKeysByPriority 按优先级排序API密钥（基于多维度加权评分）
func SortApiKeysByPriority() {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	// 过滤出未禁用且余额充足的密钥
	var activeKeys []ApiKey
	minBalanceThreshold := config.App.MinBalanceThreshold // 使用MinBalanceThreshold常量

	for _, k := range apiKeys {
		if !k.Disabled && k.Balance >= minBalanceThreshold {
			activeKeys = append(activeKeys, k)
		}
	}

	if len(activeKeys) == 0 {
		// 没有活跃的密钥，直接返回
		return
	}

	// 找出各维度的最大值，用于归一化
	var maxBalance float64
	var maxRPM, maxTPM int

	for _, k := range activeKeys {
		if k.Balance > maxBalance {
			maxBalance = k.Balance
		}
		if k.RequestsPerMinute > maxRPM {
			maxRPM = k.RequestsPerMinute
		}
		if k.TokensPerMinute > maxTPM {
			maxTPM = k.TokensPerMinute
		}
	}

	// 避免除以零
	if maxBalance == 0 {
		maxBalance = 1
	}
	if maxRPM == 0 {
		maxRPM = 1
	}
	if maxTPM == 0 {
		maxTPM = 1
	}

	// 计算每个密钥的综合得分
	type KeyScore struct {
		Key   string
		Score float64
	}

	var keyScores []KeyScore

	// 获取配置的权重，如果未配置则使用默认值
	balanceWeight := config.App.BalanceWeight
	if balanceWeight <= 0 {
		balanceWeight = 0.4 // 默认权重40%
	}

	successRateWeight := config.App.SuccessRateWeight
	if successRateWeight <= 0 {
		successRateWeight = 0.3 // 默认权重30%
	}

	rpmWeight := config.App.RPMWeight
	if rpmWeight <= 0 {
		rpmWeight = 0.15 // 默认权重15%
	}

	tpmWeight := config.App.TPMWeight
	if tpmWeight <= 0 {
		tpmWeight = 0.15 // 默认权重15%
	}

	// 确保权重总和为1
	totalWeight := balanceWeight + successRateWeight + rpmWeight + tpmWeight
	if totalWeight != 1.0 {
		// 归一化权重
		balanceWeight = balanceWeight / totalWeight
		successRateWeight = successRateWeight / totalWeight
		rpmWeight = rpmWeight / totalWeight
		tpmWeight = tpmWeight / totalWeight
	}

	for _, k := range activeKeys {
		// 1. 余额得分（余额越高，得分越高）
		balanceScore := (k.Balance / maxBalance) * balanceWeight

		// 2. 成功率得分（成功率越高，得分越高）
		successRateScore := 0.0
		if k.TotalCalls > 0 {
			successRateScore = k.SuccessRate * successRateWeight
		} else {
			// 如果没有调用记录，假设成功率为100%
			successRateScore = successRateWeight
		}

		// 3. RPM得分（RPM越低，得分越高）
		rpmScore := 0.0
		if k.RequestsPerMinute > 0 {
			rpmScore = (1 - float64(k.RequestsPerMinute)/float64(maxRPM)) * rpmWeight
		} else {
			rpmScore = rpmWeight // 如果RPM为0，给予最高分
		}

		// 4. TPM得分（TPM越低，得分越高）
		tpmScore := 0.0
		if k.TokensPerMinute > 0 {
			tpmScore = (1 - float64(k.TokensPerMinute)/float64(maxTPM)) * tpmWeight
		} else {
			tpmScore = tpmWeight // 如果TPM为0，给予最高分
		}

		// 计算综合得分
		totalScore := balanceScore + successRateScore + rpmScore + tpmScore

		keyScores = append(keyScores, KeyScore{
			Key:   k.Key,
			Score: totalScore,
		})
	}

	// 按得分从高到低排序
	for i := 0; i < len(keyScores)-1; i++ {
		for j := 0; j < len(keyScores)-i-1; j++ {
			if keyScores[j].Score < keyScores[j+1].Score {
				keyScores[j], keyScores[j+1] = keyScores[j+1], keyScores[j]
			}
		}
	}

	// 根据排序后的得分重新排列密钥
	var sortedKeys []ApiKey
	for _, ks := range keyScores {
		for _, k := range activeKeys {
			if k.Key == ks.Key {
				sortedKeys = append(sortedKeys, k)
				break
			}
		}
	}

	// 将禁用的密钥添加到末尾
	for _, k := range apiKeys {
		if k.Disabled {
			sortedKeys = append(sortedKeys, k)
		}
	}

	// 将余额不足的密钥添加到禁用密钥之前
	for _, k := range apiKeys {
		if !k.Disabled && k.Balance < minBalanceThreshold {
			sortedKeys = append(sortedKeys, k)
		}
	}

	// 更新排序后的密钥列表
	apiKeys = sortedKeys
}

// GetActiveApiKeys 获取所有未禁用且余额充足的API密钥
func GetActiveApiKeys() []ApiKey {
	allKeys := GetApiKeys() // 已经过滤掉标记为删除的密钥

	// 筛选出未禁用且余额充足的密钥
	var activeKeys []ApiKey
	for _, key := range allKeys {
		if !key.Disabled && key.Balance >= config.App.MinBalanceThreshold {
			activeKeys = append(activeKeys, key)
		}
	}

	return activeKeys
}

// GetDisabledApiKeys 获取所有禁用的API密钥
func GetDisabledApiKeys() []ApiKey {
	allKeys := GetApiKeys() // 已经过滤掉标记为删除的密钥

	// 筛选出已禁用的密钥
	var disabledKeys []ApiKey
	for _, key := range allKeys {
		if key.Disabled {
			disabledKeys = append(disabledKeys, key)
		}
	}

	return disabledKeys
}

// AddRequestStat 添加请求统计数据
func AddRequestStat(requestCount, tokenCount int) {
	statsLock.Lock()
	defer statsLock.Unlock()

	now := time.Now()
	nowUnix := now.Unix()

	// 检查是否需要重置每日计数器（日期变更）
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if lastResetDay.IsZero() || !isSameDay(lastResetDay, today) {
		// 日期已变更或首次运行，重置每日计数器
		dailyRequestCount = 0
		dailyTokenCount = 0
		lastResetDay = today
	}

	// 更新每日累计数据
	dailyRequestCount += requestCount
	dailyTokenCount += tokenCount

	// 如果最后一条记录是当前分钟的，则更新它
	if len(requestStats) > 0 && isSameMinute(requestStats[len(requestStats)-1].Timestamp, nowUnix) {
		requestStats[len(requestStats)-1].RequestCount += requestCount
		requestStats[len(requestStats)-1].TokenCount += tokenCount
	} else {
		// 否则添加新记录
		requestStats = append(requestStats, RequestStats{
			Timestamp:    nowUnix,
			RequestCount: requestCount,
			TokenCount:   tokenCount,
		})

		// 如果记录数超过最大值，删除最旧的记录
		if len(requestStats) > config.App.MaxStatsEntries {
			requestStats = requestStats[1:]
		}
	}
}

// isSameDay 判断两个时间是否在同一天
func isSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day()
}

// isSameMinute 判断两个时间戳是否在同一分钟内
func isSameMinute(ts1, ts2 int64) bool {
	t1 := time.Unix(ts1, 0)
	t2 := time.Unix(ts2, 0)
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day() && t1.Hour() == t2.Hour() && t1.Minute() == t2.Minute()
}

// UpdateApiKeyRequestStats 更新API密钥的请求系统概要
func UpdateApiKeyRequestStats(key string, requestCount, tokenCount int) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			// 更新最近一分钟的请求统计
			now := time.Now().Unix()

			// 如果没有最近请求记录，或者最后一条记录不是当前分钟的，则添加新记录
			if len(apiKeys[i].RecentRequests) == 0 || !isSameMinute(apiKeys[i].RecentRequests[len(apiKeys[i].RecentRequests)-1].Timestamp, now) {
				apiKeys[i].RecentRequests = append(apiKeys[i].RecentRequests, RequestStats{
					Timestamp:    now,
					RequestCount: requestCount,
					TokenCount:   tokenCount,
				})

				// 保留最近5分钟的数据
				if len(apiKeys[i].RecentRequests) > 5 {
					apiKeys[i].RecentRequests = apiKeys[i].RecentRequests[1:]
				}
			} else {
				// 更新最后一条记录
				lastIdx := len(apiKeys[i].RecentRequests) - 1
				apiKeys[i].RecentRequests[lastIdx].RequestCount += requestCount
				apiKeys[i].RecentRequests[lastIdx].TokenCount += tokenCount
			}

			// 计算最近5分钟的平均RPM和TPM
			totalRequests := 0
			totalTokens := 0

			// 清理过期的请求记录（超过5分钟的）
			fiveMinutesAgo := now - 300 // 5分钟 = 300秒
			var validRequests []RequestStats

			for _, stat := range apiKeys[i].RecentRequests {
				if stat.Timestamp >= fiveMinutesAgo {
					validRequests = append(validRequests, stat)
					totalRequests += stat.RequestCount
					totalTokens += stat.TokenCount
				}
			}

			// 更新有效的请求记录
			apiKeys[i].RecentRequests = validRequests

			// 计算平均值
			minutes := len(apiKeys[i].RecentRequests)
			if minutes > 0 {
				apiKeys[i].RequestsPerMinute = totalRequests / minutes
				apiKeys[i].TokensPerMinute = totalTokens / minutes
			} else {
				// 如果没有有效记录，将RPM和TPM设置为0
				apiKeys[i].RequestsPerMinute = 0
				apiKeys[i].TokensPerMinute = 0
			}

			return true
		}
	}

	return false
}

// AddKeyRequestStat 为特定API密钥添加请求统计数据
func AddKeyRequestStat(key string, requestCount, tokenCount int) {
	// 更新全局统计
	AddRequestStat(requestCount, tokenCount)

	// 更新特定密钥的统计
	UpdateApiKeyRequestStats(key, requestCount, tokenCount)

	// 每次请求后重新排序密钥，确保轮询算法使用最新的优先级
	SortApiKeysByPriority()
}

// GetCurrentRequestStats 获取当前的请求速率（RPM和TPM）
func GetCurrentRequestStats() (int, int) {
	statsLock.RLock()
	defer statsLock.RUnlock()

	// 如果没有统计数据，返回0
	if len(requestStats) == 0 {
		return 0, 0
	}

	// 获取当前时间
	now := time.Now().Unix()

	// 获取当前时间的前1分钟
	oneMinuteAgo := now - 60

	// 统计最近1分钟的请求和令牌数
	requestCount := 0
	tokenCount := 0

	for _, stat := range requestStats {
		if stat.Timestamp >= oneMinuteAgo {
			requestCount += stat.RequestCount
			tokenCount += stat.TokenCount
		}
	}

	return requestCount, tokenCount
}

// SaveApiKeys 保存API密钥到数据库
func SaveApiKeys() error {
	err := SaveApiKeysToDB()
	if err != nil {
		logger.Error("保存API密钥到数据库失败: %v", err)
		return err
	}

	logger.Info("API密钥成功保存到数据库")
	return nil
}

// GetCurrentRPD 获取当前每日请求数
func GetCurrentRPD() int {
	statsLock.RLock()
	defer statsLock.RUnlock()

	// 检查是否需要重置每日计数器（日期变更）
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 获取今天的开始时间
	startOfDay := today.Unix()

	// 计算今天的总请求数
	totalRequests := 0
	for _, stat := range requestStats {
		if stat.Timestamp >= startOfDay {
			totalRequests += stat.RequestCount
		}
	}

	// 如果从daily.json中获取数据
	if dailyStats, err := GetDailyStats(""); err == nil && dailyStats != nil {
		// 使用daily.json中的数据
		return dailyStats.Requests.Total
	}

	// 返回计算的每日请求数
	return totalRequests
}

// GetCurrentTPD 获取当前每日令牌数
func GetCurrentTPD() int {
	statsLock.RLock()
	defer statsLock.RUnlock()

	// 检查是否需要重置每日计数器（日期变更）
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// 获取今天的开始时间
	startOfDay := today.Unix()

	// 计算今天的总令牌数
	totalTokens := 0
	for _, stat := range requestStats {
		if stat.Timestamp >= startOfDay {
			totalTokens += stat.TokenCount
		}
	}

	// 如果从daily.json中获取数据
	if dailyStats, err := GetDailyStats(""); err == nil && dailyStats != nil {
		// 使用daily.json中的数据
		return dailyStats.Tokens.Total
	}

	// 返回计算的每日令牌数
	return totalTokens
}

// UpdateConfig 更新全局配置
func UpdateConfig(newConfig *Config) {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	// 更新全局配置
	config = newConfig

	// 标准化模型策略配置
	standardizeModelKeyStrategies()
}

// MarkApiKeyForDeletion 标记API密钥为删除状态
func MarkApiKeyForDeletion(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].Delete = true
			log.Printf("API密钥 %s 已标记为删除", MaskKey(key))

			// 更新数据库中的删除标记
			keyCopy := apiKeys[i]
			keyCopy.RecentRequests = nil // 清空不需要保存的字段
			if err := AddApiKeyToDB(keyCopy); err != nil {
				logger.Error("更新删除标记到数据库失败: %v", err)
			}

			return true
		}
	}

	return false
}

// RemoveMarkedApiKeys 删除所有标记为删除的API密钥
func RemoveMarkedApiKeys() int {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	originalLength := len(apiKeys)
	newKeys := make([]ApiKey, 0, originalLength)

	// 保存需要删除的密钥列表
	keysToDelete := make([]string, 0)

	for _, k := range apiKeys {
		if !k.Delete {
			newKeys = append(newKeys, k)
		} else {
			log.Printf("删除已标记的API密钥: %s", MaskKey(k.Key))
			// 记录需要从数据库中删除的密钥
			keysToDelete = append(keysToDelete, k.Key)
		}
	}

	deleted := originalLength - len(newKeys)
	if deleted > 0 {
		apiKeys = newKeys

		// 从数据库中删除标记的密钥
		for _, keyToDelete := range keysToDelete {
			if err := DeleteApiKeyFromDB(keyToDelete); err != nil {
				logger.Error("从数据库删除API密钥失败: %v", err)
			}
		}
	}

	return deleted
}

// EnsureDefaultConfig 检查配置表中是否有数据，如果没有则插入默认配置
func EnsureDefaultConfig(dbPath string) error {
	// 确保全局数据库连接已经初始化
	if db == nil {
		logger.Error("数据库连接未初始化，先初始化连接")
		err := InitConfigDB(dbPath)
		if err != nil {
			return fmt.Errorf("初始化数据库连接失败: %w", err)
		}
	}

	// 检查数据库连接是否可用
	if err := db.Ping(); err != nil {
		logger.Error("数据库连接不可用: %v", err)
		return fmt.Errorf("数据库连接不可用: %w", err)
	}

	// 检查是否已存在配置
	var configValue string
	err := db.QueryRow("SELECT value FROM config WHERE key = 'config'").Scan(&configValue)

	// 如果没有找到配置行或者出现其他错误，插入默认配置
	if err != nil {
		logger.Info("数据库中没有找到配置或发生错误: %v，将插入默认配置", err)

		// 使用默认版本号
		version := "v1.3.7" // 默认版本号

		// 确保版本已保存
		_, err = db.Exec("INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)", "version", version)
		if err != nil {
			logger.Error("无法插入版本号: %v", err)
			// 继续，因为这不是致命错误
		} else {
			logger.Info("已插入默认版本号: %s", version)
		}

		// 确保版本号格式正确
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		// 插入默认配置
		defaultConfig := fmt.Sprintf(`{
			"Server":{"Port":3016},
			"ApiProxy":{
				"BaseURL":"https://api.siliconflow.cn",
				"ModelIndex":0,
				"Retry":{
					"MaxRetries":2,
					"RetryDelayMs":1000,
					"RetryOnStatusCodes":[500,502,503,504],
					"RetryOnNetworkErrors":true
				}
			},
			"Proxy":{
				"HttpProxy":"",
				"HttpsProxy":"",
				"SocksProxy":"127.0.0.1:10808",
				"ProxyType":"socks5",
				"Enabled":false
			},
			"App":{
				"Title":"流动硅基 FlowSilicon %s",
				"MinBalanceThreshold":0.8,
				"MaxBalanceDisplay":14,
				"ItemsPerPage":5,
				"MaxStatsEntries":60,
				"RecoveryInterval":10,
				"MaxConsecutiveFailures":5,
				"BalanceWeight":0.4,
				"SuccessRateWeight":0.3,
				"RPMWeight":0.15,
				"TPMWeight":0.15,
				"AutoUpdateInterval":500,
				"StatsRefreshInterval":100,
				"RateRefreshInterval":150,
				"ModelKeyStrategies":{},
				"HideIcon":false
			},
			"Log":{"MaxSizeMB":1, "Level":"warn"}
		}`, version)

		// 插入默认配置到数据库
		_, err = db.Exec(
			"INSERT OR REPLACE INTO config (key, value) VALUES (?, ?)",
			"config",
			defaultConfig,
		)
		if err != nil {
			return fmt.Errorf("插入默认配置失败: %w", err)
		}

		logger.Info("已成功插入默认配置到数据库")
	} else {
		logger.Info("找到现有配置，无需插入默认配置")
	}

	return nil
}

// LoadApiKeys 加载API密钥
// 此函数为保持向后兼容性而存在，实际调用LoadApiKeysFromDB
func LoadApiKeys() error {
	logger.Info("调用LoadApiKeys函数加载API密钥")
	return LoadApiKeysFromDB()
}

// GetUsedApiKeys 获取所有已使用过的API密钥
func GetUsedApiKeys() []ApiKey {
	allKeys := GetApiKeys() // 已经过滤掉标记为删除的密钥

	// 筛选出已使用过的密钥
	var usedKeys []ApiKey
	for _, key := range allKeys {
		if key.IsUsed {
			usedKeys = append(usedKeys, key)
		}
	}

	return usedKeys
}

// MarkApiKeyAsUnused 标记 API 密钥为未使用
func MarkApiKeyAsUnused(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].IsUsed = false

			// 保存更新到数据库
			if db != nil {
				_, err := db.Exec(`UPDATE `+apikeysTableName+` 
					SET is_used = ? WHERE key = ?`, false, key)
				if err != nil {
					logger.Error("更新API密钥未使用状态到数据库失败: %v", err)
				}
			}

			return true
		}
	}
	return false
}
