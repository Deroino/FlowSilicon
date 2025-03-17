/**
  @author: Hanhai
  @since: 2025/3/16 20:44:00
  @desc:
**/

package config

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Server struct {
		Port int `mapstructure:"port"`
	} `mapstructure:"server"`
	ApiProxy struct {
		BaseURL    string `mapstructure:"base_url"`
		ModelIndex int    `mapstructure:"model_index"` // 当前使用的模型索引
	} `mapstructure:"api_proxy"`
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
		StatsRefreshInterval int `mapstructure:"stats_refresh_interval"` // 统计信息自动刷新间隔（秒）
		RateRefreshInterval  int `mapstructure:"rate_refresh_interval"`  // 速率监控自动刷新间隔（秒）
		// 模型特定的密钥选择策略
		ModelKeyStrategies map[string]int `mapstructure:"model_key_strategies"` // 模型特定的密钥选择策略
	} `mapstructure:"app"`
	Log struct {
		MaxSizeMB int `mapstructure:"max_size_mb"` // 日志文件最大大小（MB）
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
}

// RequestStats 请求统计结构
type RequestStats struct {
	Timestamp    int64 `json:"timestamp"`     // 时间戳
	RequestCount int   `json:"request_count"` // 请求数
	TokenCount   int   `json:"token_count"`   // 令牌数
}

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

// LoadConfig 加载配置文件
func LoadConfig(configPath string) (*Config, error) {
	var err error
	configOnce.Do(func() {
		viper.SetConfigFile(configPath)
		viper.SetConfigType("yaml")
		viper.AutomaticEnv()

		if err = viper.ReadInConfig(); err != nil {
			log.Printf("Error reading config file: %s", err)
			return
		}

		config = &Config{}
		if err = viper.Unmarshal(config); err != nil {
			log.Printf("Unable to decode config into struct: %v", err)
			return
		}

		// 添加配置值日志
		log.Printf("配置加载成功 - AutoUpdateInterval: %d, StatsRefreshInterval: %d, RateRefreshInterval: %d",
			config.App.AutoUpdateInterval,
			config.App.StatsRefreshInterval,
			config.App.RateRefreshInterval)

		// 初始化lastResetDay为今天的开始时间
		now := time.Now()
		lastResetDay = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

		// 加载成功后初始化API密钥
		LoadApiKeys()
	})

	return config, err
}

// GetConfig 获取配置
func GetConfig() *Config {
	if config == nil {
		// 创建一个默认配置而不是调用Fatal
		config = &Config{}
		config.Server.Port = 8080                              // 默认端口
		config.ApiProxy.BaseURL = "https://api.siliconflow.cn" // 默认API基础URL
		config.ApiProxy.ModelIndex = 0                         // 默认模型索引

		// 应用默认配置
		config.App.Title = "API 密钥管理系统"       // 默认标题
		config.App.MinBalanceThreshold = 1.8  // 默认最低余额阈值
		config.App.MaxBalanceDisplay = 14.0   // 默认余额显示最大值
		config.App.ItemsPerPage = 5           // 默认每页显示的密钥数量
		config.App.MaxStatsEntries = 60       // 默认最大统计条目数
		config.App.RecoveryInterval = 10      // 默认恢复检查间隔（分钟）
		config.App.MaxConsecutiveFailures = 5 // 默认最大连续失败次数

		// 设置默认权重
		config.App.BalanceWeight = 0.4     // 默认余额权重
		config.App.SuccessRateWeight = 0.3 // 默认成功率权重
		config.App.RPMWeight = 0.15        // 默认RPM权重
		config.App.TPMWeight = 0.15        // 默认TPM权重

		// 设置默认刷新间隔
		config.App.AutoUpdateInterval = 300  // 默认API密钥信息自动更新间隔（5分钟）
		config.App.StatsRefreshInterval = 30 // 默认统计信息自动刷新间隔（30秒）
		config.App.RateRefreshInterval = 10  // 默认速率监控自动刷新间隔（10秒）

		// 设置日志配置默认值
		config.Log.MaxSizeMB = 10 // 默认日志文件最大大小为10MB

		// 初始化模型特定的密钥选择策略
		config.App.ModelKeyStrategies = make(map[string]int)
		// 设置默认的模型特定策略
		config.App.ModelKeyStrategies["deepseek-ai/DeepSeek-V3"] = 1 // 高成功率策略
	}
	return config
}

// GetApiKeys 获取所有API密钥
func GetApiKeys() []ApiKey {
	keysMutex.RLock()
	defer keysMutex.RUnlock()

	// 返回副本以避免外部修改
	keysCopy := make([]ApiKey, len(apiKeys))
	copy(keysCopy, apiKeys)
	return keysCopy
}

// LoadApiKeys 从文件加载API密钥
func LoadApiKeys() error {
	// 尝试从文件加载API密钥
	keysFile := "./data/api_keys.json"

	// 检查文件是否存在
	if _, err := os.Stat(keysFile); os.IsNotExist(err) {
		log.Printf("API密钥文件不存在，将使用空列表")
		apiKeys = make([]ApiKey, 0)
		return nil
	}

	// 读取文件
	data, err := os.ReadFile(keysFile)
	if err != nil {
		log.Printf("读取API密钥文件失败: %v", err)
		apiKeys = make([]ApiKey, 0)
		return err
	}

	// 解析JSON
	keysMutex.Lock()
	defer keysMutex.Unlock()

	if err := json.Unmarshal(data, &apiKeys); err != nil {
		log.Printf("解析API密钥文件失败: %v", err)
		apiKeys = make([]ApiKey, 0)
		return err
	}

	// 确保所有密钥的RPM和TPM初始化为0，并初始化RecentRequests数组
	for i := range apiKeys {
		apiKeys[i].RequestsPerMinute = 0
		apiKeys[i].TokensPerMinute = 0
		apiKeys[i].RecentRequests = make([]RequestStats, 0)
	}

	// 检查并禁用低余额密钥，启用高余额密钥
	keysUpdated := false
	for i := range apiKeys {
		if apiKeys[i].Balance < config.App.MinBalanceThreshold {
			// 只有余额小于阈值的密钥才会被禁用
			if !apiKeys[i].Disabled {
				apiKeys[i].Disabled = true
				apiKeys[i].DisabledAt = time.Now().Unix()
				keysUpdated = true
				log.Printf("API密钥 %s 余额 %.2f 低于阈值 %.2f，已自动禁用",
					apiKeys[i].Key, apiKeys[i].Balance, config.App.MinBalanceThreshold)
			}
		} else {
			// 余额充足的密钥确保是启用状态
			if apiKeys[i].Disabled {
				apiKeys[i].Disabled = false
				apiKeys[i].DisabledAt = 0
				keysUpdated = true
				log.Printf("API密钥 %s 余额 %.2f 高于阈值 %.2f，已自动启用",
					apiKeys[i].Key, apiKeys[i].Balance, config.App.MinBalanceThreshold)
			}
		}
	}

	// 如果有密钥状态被更新，保存到文件
	if keysUpdated {
		if err := SaveApiKeysWithLock(false); err != nil {
			log.Printf("保存更新后的API密钥状态失败: %v", err)
		}
	}

	log.Printf("成功加载 %d 个API密钥", len(apiKeys))
	return nil
}

// UpdateApiKeys 更新API密钥列表
func UpdateApiKeys(keys []ApiKey) {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	// 检查每个密钥的余额并设置禁用状态
	for i := range keys {
		if keys[i].Balance < config.App.MinBalanceThreshold {
			keys[i].Disabled = true
			keys[i].DisabledAt = time.Now().Unix()
			log.Printf("API密钥 %s 余额 %.2f 低于阈值 %.2f，已自动禁用",
				keys[i].Key, keys[i].Balance, config.App.MinBalanceThreshold)
		} else {
			keys[i].Disabled = false
			keys[i].DisabledAt = 0
		}
	}

	apiKeys = keys
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
}

// RemoveApiKey 删除API密钥
func RemoveApiKey(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			// 从切片中删除元素
			apiKeys = append(apiKeys[:i], apiKeys[i+1:]...)
			return true
		}
	}

	return false
}

// UpdateApiKeyBalance 更新API密钥余额并自动禁用低余额密钥
func UpdateApiKeyBalance(key string, balance float64) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].Balance = balance
			statusChanged := false

			// 只在余额低于阈值时禁用密钥
			if balance < config.App.MinBalanceThreshold && !k.Disabled {
				apiKeys[i].Disabled = true
				apiKeys[i].DisabledAt = time.Now().Unix()
				statusChanged = true
				log.Printf("API密钥 %s 余额更新为 %.2f，低于阈值 %.2f，已自动禁用",
					key, balance, config.App.MinBalanceThreshold)
			} else if balance >= config.App.MinBalanceThreshold && k.Disabled {
				// 余额充足时确保密钥是启用状态
				apiKeys[i].Disabled = false
				apiKeys[i].DisabledAt = 0
				statusChanged = true
				log.Printf("API密钥 %s 余额更新为 %.2f，高于阈值 %.2f，已自动启用",
					key, balance, config.App.MinBalanceThreshold)
			}

			// 如果状态发生变化，保存到文件
			if statusChanged {
				if err := SaveApiKeysWithLock(false); err != nil {
					log.Printf("保存更新后的API密钥状态失败: %v", err)
				}
			}

			return true
		}
	}

	return false
}

// UpdateApiKeyLastUsed 更新API密钥最后使用时间
func UpdateApiKeyLastUsed(key string, timestamp int64) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].LastUsed = timestamp
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
	log.Printf("API密钥 %s 余额=%.2f(%.2f), 成功率=%.2f(%.2f), RPM=%d(%.2f), TPM=%d(%.2f), 总分=%.2f",
		key.Key[:6]+"******", key.Balance, balanceScore, key.SuccessRate, successRateScore,
		key.RequestsPerMinute, rpmScore, key.TokensPerMinute, tpmScore, totalScore)

	return totalScore
}

// UpdateApiKeySuccess 更新API密钥成功调用记录
func UpdateApiKeySuccess(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].TotalCalls++
			apiKeys[i].SuccessCalls++
			apiKeys[i].ConsecutiveFailures = 0
			apiKeys[i].SuccessRate = float64(apiKeys[i].SuccessCalls) / float64(apiKeys[i].TotalCalls)
			return true
		}
	}

	return false
}

// UpdateApiKeyFailure 更新API密钥失败调用记录
func UpdateApiKeyFailure(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			apiKeys[i].TotalCalls++
			apiKeys[i].ConsecutiveFailures++
			apiKeys[i].SuccessRate = float64(apiKeys[i].SuccessCalls) / float64(apiKeys[i].TotalCalls)
			return true
		}
	}

	return false
}

// DisableApiKey 禁用API密钥
func DisableApiKey(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			// 如果已经禁用，不需要再做操作
			if k.Disabled {
				return true
			}

			apiKeys[i].Disabled = true
			apiKeys[i].DisabledAt = time.Now().Unix()

			// 保存更新到文件
			if err := SaveApiKeysWithLock(false); err != nil {
				log.Printf("保存API密钥状态失败: %v", err)
			}

			return true
		}
	}

	return false
}

// EnableApiKey 启用API密钥
func EnableApiKey(key string) bool {
	keysMutex.Lock()
	defer keysMutex.Unlock()

	for i, k := range apiKeys {
		if k.Key == key {
			// 如果余额不足，不允许启用
			if k.Balance < config.App.MinBalanceThreshold {
				log.Printf("无法启用API密钥 %s：余额 %.2f 低于阈值 %.2f",
					key, k.Balance, config.App.MinBalanceThreshold)
				return false
			}

			// 如果已经启用，不需要再做操作
			if !k.Disabled {
				return true
			}

			apiKeys[i].Disabled = false
			apiKeys[i].DisabledAt = 0
			apiKeys[i].ConsecutiveFailures = 0

			// 保存更新到文件
			if err := SaveApiKeysWithLock(false); err != nil {
				log.Printf("保存API密钥状态失败: %v", err)
			}

			return true
		}
	}

	return false
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
	keysMutex.RLock()
	defer keysMutex.RUnlock()

	var activeKeys []ApiKey
	for _, k := range apiKeys {
		if !k.Disabled && k.Balance >= config.App.MinBalanceThreshold {
			activeKeys = append(activeKeys, k)
		}
	}

	// 返回副本以避免外部修改
	keysCopy := make([]ApiKey, len(activeKeys))
	copy(keysCopy, activeKeys)
	return keysCopy
}

// GetDisabledApiKeys 获取所有已禁用的API密钥
func GetDisabledApiKeys() []ApiKey {
	keysMutex.RLock()
	defer keysMutex.RUnlock()

	var disabledKeys []ApiKey
	for _, k := range apiKeys {
		if k.Disabled {
			disabledKeys = append(disabledKeys, k)
		}
	}

	// 返回副本以避免外部修改
	keysCopy := make([]ApiKey, len(disabledKeys))
	copy(keysCopy, disabledKeys)
	return keysCopy
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

// GetRequestStats 获取请求统计数据
func GetRequestStats() []RequestStats {
	statsLock.RLock()
	defer statsLock.RUnlock()

	// 返回副本以避免外部修改
	statsCopy := make([]RequestStats, len(requestStats))
	copy(statsCopy, requestStats)
	return statsCopy
}

// GetCurrentRPM 获取当前每分钟请求数
func GetCurrentRPM() int {
	statsLock.RLock()
	defer statsLock.RUnlock()

	if len(requestStats) == 0 {
		return 0
	}

	// 获取最近5分钟的数据
	startIdx := 0
	if len(requestStats) > 5 {
		startIdx = len(requestStats) - 5
	}

	totalRequests := 0
	for i := startIdx; i < len(requestStats); i++ {
		totalRequests += requestStats[i].RequestCount
	}

	// 计算平均每分钟请求数
	minutes := len(requestStats) - startIdx
	if minutes == 0 {
		return 0
	}

	return totalRequests / minutes
}

// GetCurrentTPM 获取当前每分钟令牌数
func GetCurrentTPM() int {
	statsLock.RLock()
	defer statsLock.RUnlock()

	if len(requestStats) == 0 {
		return 0
	}

	// 获取最近5分钟的数据
	startIdx := 0
	if len(requestStats) > 5 {
		startIdx = len(requestStats) - 5
	}

	totalTokens := 0
	for i := startIdx; i < len(requestStats); i++ {
		totalTokens += requestStats[i].TokenCount
	}

	// 计算平均每分钟令牌数
	minutes := len(requestStats) - startIdx
	if minutes == 0 {
		return 0
	}

	return totalTokens / minutes
}

// isSameMinute 判断两个时间戳是否在同一分钟内
func isSameMinute(ts1, ts2 int64) bool {
	t1 := time.Unix(ts1, 0)
	t2 := time.Unix(ts2, 0)
	return t1.Year() == t2.Year() && t1.Month() == t2.Month() && t1.Day() == t2.Day() && t1.Hour() == t2.Hour() && t1.Minute() == t2.Minute()
}

// UpdateApiKeyRequestStats 更新API密钥的请求统计信息
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

// SaveConfig 保存配置到文件
// func SaveConfig() error {
// 	if config == nil {
// 		logger.Error("配置为空，无法保存")
// 		return fmt.Errorf("配置为空")
// 	}

// 	// 使用viper保存配置
// 	if err := viper.WriteConfig(); err != nil {
// 		logger.Error("保存配置文件失败: %v", err)
// 		return err
// 	}

// 	logger.Info("成功保存配置到文件")
// 	return nil
// }

// SaveApiKeys 保存API密钥到文件
func SaveApiKeys() error {
	return SaveApiKeysWithLock(true)
}

// SaveApiKeysWithLock 保存API密钥到文件，可选择是否获取锁
func SaveApiKeysWithLock(withLock bool) error {
	if withLock {
		keysMutex.RLock()
		defer keysMutex.RUnlock()
	}

	// 创建API密钥的副本，以便修改
	keysCopy := make([]ApiKey, len(apiKeys))
	copy(keysCopy, apiKeys)

	// 清空所有密钥的RecentRequests数组，并重置RPM和TPM
	for i := range keysCopy {
		// RecentRequests字段已经标记为json:"-"，不会被序列化
		// 但为了确保数据一致性，我们也清空它
		keysCopy[i].RecentRequests = nil
		// 重置RPM和TPM为0，这样在程序重启后不会保留旧值
		keysCopy[i].RequestsPerMinute = 0
		keysCopy[i].TokensPerMinute = 0
	}

	// 将API密钥序列化为JSON
	data, err := json.MarshalIndent(keysCopy, "", "  ")
	if err != nil {
		log.Printf("序列化API密钥失败: %v", err)
		return err
	}

	// 写入文件
	keysFile := "./data/api_keys.json"
	if err := os.WriteFile(keysFile, data, 0644); err != nil {
		log.Printf("写入API密钥文件失败: %v", err)
		return err
	}

	log.Printf("成功保存 %d 个API密钥到文件", len(keysCopy))
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
