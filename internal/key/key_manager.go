/**
  @author: Hanhai
  @since: 2025/3/16 20:42:20
  @desc:
**/

package key

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/robfig/cron/v3"

	"flowsilicon/internal/common"
	"flowsilicon/internal/config"
	"flowsilicon/internal/logger"
)

// KeyMode 定义 API 密钥使用模式
type KeyMode string

const (
	// KeyModeAll 轮询所有密钥
	KeyModeAll KeyMode = "all"
	// KeyModeSingle 使用单个密钥
	KeyModeSingle KeyMode = "single"
	// KeyModeSelected 轮询选中的密钥
	KeyModeSelected KeyMode = "selected"

	// 新增常量
	MaxConsecutiveFailures = 5  // 最大连续失败次数，超过此值将禁用密钥
	RecoveryInterval       = 10 // 恢复检查间隔（分钟）
)

var (
	currentKeyIndex int
	keyIndexMutex   sync.Mutex
	client          *resty.Client

	// 密钥使用模式
	currentMode  KeyMode = KeyModeAll
	selectedKeys []string
	modeMutex    sync.RWMutex
)

// 初始化 HTTP 客户端
func init() {
	client = resty.New()
	client.SetTimeout(30 * time.Second)
}

// StartKeyManager 启动 API 密钥管理器
func StartKeyManager() {
	// 从配置文件获取自动更新间隔
	cfg := config.GetConfig()
	checkInterval := cfg.App.AutoUpdateInterval

	// 如果配置值小于等于0，使用默认值
	if checkInterval <= 0 {
		checkInterval = 60 // 默认每60秒检查一次
	}

	// 将秒转换为分钟，因为cron表达式使用分钟
	checkIntervalMinutes := checkInterval / 60
	if checkIntervalMinutes < 1 {
		checkIntervalMinutes = 1 // 最小1分钟
	}

	// 创建定时任务
	c := cron.New()

	// 添加定时任务，每隔指定时间检查一次 API 密钥余额
	spec := fmt.Sprintf("@every %dm", checkIntervalMinutes)
	c.AddFunc(spec, checkAllKeysBalance)

	// 添加定时任务，每隔 RecoveryInterval 分钟尝试恢复被禁用的密钥
	recoverySpec := fmt.Sprintf("@every %dm", cfg.App.RecoveryInterval)
	c.AddFunc(recoverySpec, tryRecoverDisabledKeys)

	// 启动定时任务
	c.Start()
}

// checkAllKeysBalance 检查所有 API 密钥的余额
func checkAllKeysBalance() {
	keys := config.GetApiKeys()
	logger.Info("开始检查 %d 个API密钥的余额", len(keys))

	// 创建一个等待组，用于等待所有检查完成
	var wg sync.WaitGroup

	for i := range keys {
		wg.Add(1)
		go func(key config.ApiKey) {
			defer wg.Done()

			// 检查余额
			balance, err := CheckKeyBalance(key.Key)
			if err != nil {
				logger.Error("检查API密钥 %s 余额失败: %v", MaskKey(key.Key), err)
				return
			}

			logger.Info("API密钥 %s 余额: %.2f", MaskKey(key.Key), balance)

			// 如果余额为0，可以考虑移除该密钥
			if balance <= 0 {
				logger.Info("API密钥 %s 余额为0，移除该密钥", MaskKey(key.Key))
				config.RemoveApiKey(key.Key)
				return
			}

			// 如果余额低于阈值但状态为启用，禁用它
			if balance < config.GetConfig().App.MinBalanceThreshold && !key.Disabled {
				logger.Info("API密钥 %s 余额 %.2f 低于阈值 %.2f，禁用该密钥",
					MaskKey(key.Key), balance, config.GetConfig().App.MinBalanceThreshold)
				config.DisableApiKey(key.Key)
				return
			}

			// 如果余额高于阈值但状态为禁用，启用它
			if balance >= config.GetConfig().App.MinBalanceThreshold && key.Disabled {
				logger.Info("API密钥 %s 余额 %.2f 高于阈值 %.2f，启用该密钥",
					MaskKey(key.Key), balance, config.GetConfig().App.MinBalanceThreshold)
				config.EnableApiKey(key.Key)
				return
			}

			// 更新余额
			config.UpdateApiKeyBalance(key.Key, balance)
		}(keys[i])
	}

	// 等待所有检查完成
	wg.Wait()

	// 保存更新后的密钥状态
	if err := config.SaveApiKeys(); err != nil {
		logger.Error("保存API密钥状态失败: %v", err)
	}

	// 重新排序 API 密钥（按照综合得分从高到低）
	config.SortApiKeysByBalance()

	logger.Info("API密钥余额检查完成")
}

// CheckKeyBalance 检查 API 密钥余额
func CheckKeyBalance(key string) (float64, error) {
	// 使用硅基流动 API 的用户信息接口
	userInfoURL := "https://api.siliconflow.cn/v1/user/info"

	resp, err := client.R().
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", key)).
		Get(userInfoURL)

	if err != nil {
		return 0, fmt.Errorf("请求失败: %w", err)
	}

	if resp.StatusCode() != 200 {
		return 0, fmt.Errorf("API 返回状态码 %d", resp.StatusCode())
	}

	// 解析响应
	var result SiliconFlowUserInfoResponse

	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return 0, fmt.Errorf("解析响应失败: %w", err)
	}

	// 检查 API 响应状态
	if !result.Status || result.Code != 20000 {
		return 0, fmt.Errorf("API 响应错误: %s", result.Message)
	}

	// 解析余额字符串为浮点数
	balance, err := strconv.ParseFloat(result.Data.TotalBalance, 64)
	if err != nil {
		return 0, fmt.Errorf("解析余额失败: %w", err)
	}

	// 不再记录余额信息

	return balance, nil
}

// SiliconFlowUserInfoResponse 硅基流动用户信息响应结构
type SiliconFlowUserInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  bool   `json:"status"`
	Data    struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Image         string `json:"image"`
		Email         string `json:"email"`
		IsAdmin       bool   `json:"isAdmin"`
		Balance       string `json:"balance"`
		Status        string `json:"status"`
		Introduction  string `json:"introduction"`
		Role          string `json:"role"`
		ChargeBalance string `json:"chargeBalance"`
		TotalBalance  string `json:"totalBalance"`
		Category      string `json:"category"`
	} `json:"data"`
}

// GetNextApiKey 获取下一个要使用的 API 密钥
func GetNextApiKey() (string, error) {
	modeMutex.RLock()
	mode := currentMode
	keys := selectedKeys
	modeMutex.RUnlock()

	// 根据不同的模式选择密钥
	switch mode {
	case KeyModeSingle:
		// 单个密钥模式
		if len(keys) == 0 {
			return "", common.NewApiError("no key selected for single mode", 500)
		}

		// 检查选中的密钥是否存在且未禁用
		allKeys := config.GetApiKeys()
		for _, k := range allKeys {
			if k.Key == keys[0] && !k.Disabled {
				// 检查余额是否充足
				if k.Balance < config.GetConfig().App.MinBalanceThreshold {
					return "", common.NewApiError("selected key has insufficient balance", 500)
				}

				// 更新最后使用时间
				config.UpdateApiKeyLastUsed(k.Key, time.Now().Unix())
				return k.Key, nil
			}
		}

		return "", common.NewApiError("selected key not found or disabled", 500)

	case KeyModeSelected:
		// 选中密钥轮询模式
		if len(keys) == 0 {
			return "", common.NewApiError("no keys selected for selected mode", 500)
		}

		// 创建一个映射，用于快速查找密钥是否在选中列表中
		keyMap := make(map[string]bool)
		for _, k := range keys {
			keyMap[k] = true
		}

		// 过滤出选中的且未禁用的密钥，且余额充足
		var selectedKeysList []config.ApiKey
		allKeys := config.GetApiKeys()
		for _, k := range allKeys {
			if keyMap[k.Key] && !k.Disabled && k.Balance >= config.GetConfig().App.MinBalanceThreshold {
				selectedKeysList = append(selectedKeysList, k)
			}
		}

		if len(selectedKeysList) == 0 {
			return "", common.NewApiError("no active selected keys with sufficient balance found", 500)
		}

		keyIndexMutex.Lock()
		defer keyIndexMutex.Unlock()

		// 确保索引在有效范围内
		if currentKeyIndex >= len(selectedKeysList) {
			currentKeyIndex = 0
		}

		// 获取当前密钥
		key := selectedKeysList[currentKeyIndex].Key

		// 更新最后使用时间
		config.UpdateApiKeyLastUsed(key, time.Now().Unix())

		// 移动到下一个密钥
		currentKeyIndex = (currentKeyIndex + 1) % len(selectedKeysList)

		return key, nil

	default:
		// 默认使用智能负载均衡策略
		return GetOptimalApiKey()
	}
}

// SetKeyMode 设置 API 密钥使用模式
func SetKeyMode(mode KeyMode, keys []string) error {
	modeMutex.Lock()
	defer modeMutex.Unlock()

	// 验证模式
	switch mode {
	case KeyModeSingle:
		if len(keys) != 1 {
			return fmt.Errorf("single mode requires exactly one key")
		}
	case KeyModeSelected:
		if len(keys) == 0 {
			return fmt.Errorf("selected mode requires at least one key")
		}
		if len(keys) < 2 {
			return fmt.Errorf("selected mode requires at least two keys")
		}
	case KeyModeAll:
		// 轮询所有密钥不需要特殊验证
	default:
		return fmt.Errorf("invalid key mode: %s", mode)
	}

	// 设置模式和选中的密钥
	currentMode = mode
	selectedKeys = keys

	// TODO 注释日志
	logger.Info("设置API密钥使用模式: %s, 选中的密钥: %v", mode, keys)

	// 重置当前密钥索引
	ResetCurrentKeyIndex()

	return nil
}

// GetCurrentKeyMode 获取当前 API 密钥使用模式
func GetCurrentKeyMode() (KeyMode, []string) {
	modeMutex.RLock()
	defer modeMutex.RUnlock()

	// 返回副本以避免外部修改
	keysCopy := make([]string, len(selectedKeys))
	copy(keysCopy, selectedKeys)

	return currentMode, keysCopy
}

// resetCurrentKeyIndex 重置当前密钥索引
func ResetCurrentKeyIndex() {
	keyIndexMutex.Lock()
	defer keyIndexMutex.Unlock()
	currentKeyIndex = 0
}

// MaskKey 掩盖 API 密钥（用于日志）
func MaskKey(key string) string {
	if len(key) <= 6 {
		return "******"
	}
	return key[:6] + "******"
}

// CheckKeyBalanceManually 手动检查API密钥的余额
func CheckKeyBalanceManually(apiKey string) (float64, error) {
	// 直接调用CheckKeyBalance函数
	balance, err := CheckKeyBalance(apiKey)
	if err != nil {
		return 0, err
	}

	return balance, nil
}

// tryRecoverDisabledKeys 尝试恢复被禁用的密钥
func tryRecoverDisabledKeys() {
	disabledKeys := config.GetDisabledApiKeys()

	// 创建一个等待组，用于等待所有检查完成
	var wg sync.WaitGroup

	for i := range disabledKeys {
		wg.Add(1)
		go func(key config.ApiKey) {
			defer wg.Done()

			// 检查是否已经过了足够的时间
			now := time.Now().Unix()
			if now-key.DisabledAt < int64(config.GetConfig().App.RecoveryInterval*60) {
				// 还没到恢复检查时间
				return
			}

			// 首先检查密钥余额是否满足最低阈值要求
			balance, err := CheckKeyBalance(key.Key)
			if err != nil {
				logger.Error("恢复检查: 检查API密钥 %s 余额失败: %v", MaskKey(key.Key), err)
				return
			}

			// 如果余额低于最低阈值，不恢复该密钥
			if balance < config.GetConfig().App.MinBalanceThreshold {
				logger.Info("恢复检查: API密钥 %s 余额 %.2f 低于阈值 %.2f，不恢复该密钥",
					MaskKey(key.Key), balance, config.GetConfig().App.MinBalanceThreshold)

				// 更新密钥余额
				config.UpdateApiKeyBalance(key.Key, balance)
				return
			}

			// 检查密钥是否可用
			success, _, err := common.TestChatAPI(key.Key)

			// 更新最后测试时间
			config.UpdateApiKeyLastTested(key.Key, now)

			if err != nil || !success {
				// 测试失败，继续保持禁用状态
				logger.Info("恢复检查: API密钥 %s 测试失败，继续保持禁用状态", MaskKey(key.Key))
				return
			}

			// 测试成功，恢复密钥
			logger.Info("恢复检查: API密钥 %s 测试成功，余额 %.2f 高于阈值 %.2f，恢复该密钥",
				MaskKey(key.Key), balance, config.GetConfig().App.MinBalanceThreshold)

			// 更新密钥余额并启用
			config.UpdateApiKeyBalance(key.Key, balance)
			config.EnableApiKey(key.Key)
		}(disabledKeys[i])
	}

	// 等待所有检查完成
	wg.Wait()

	// 保存更新后的密钥状态
	if err := config.SaveApiKeys(); err != nil {
		logger.Error("保存API密钥状态失败: %v", err)
	}
}

// UpdateApiKeyStatus 根据API调用结果更新密钥状态
func UpdateApiKeyStatus(key string, success bool) {
	if success {
		// 成功调用，更新成功记录
		config.UpdateApiKeySuccess(key)
	} else {
		// 失败调用，更新失败记录
		config.UpdateApiKeyFailure(key)

		// 获取密钥信息
		allKeys := config.GetApiKeys()
		for _, k := range allKeys {
			if k.Key == key {
				// 检查连续失败次数是否超过阈值
				if k.ConsecutiveFailures >= config.GetConfig().App.MaxConsecutiveFailures {
					// 禁用密钥
					config.DisableApiKey(key)
				}
				break
			}
		}
	}

	// 重新排序密钥
	config.SortApiKeysByPriority()
}
