/**
  @author: Hanhai
  @desc: API密钥管理模块，提供密钥选择、余额检查和自动禁用恢复功能
**/

package key

import (
	"context"
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
)

var (
	currentKeyIndex int
	keyIndexMutex   sync.Mutex
	client          *resty.Client

	// 密钥使用模式
	currentMode  KeyMode = KeyModeAll
	selectedKeys []string
	modeMutex    sync.RWMutex

	// cron调度器实例
	cronScheduler *cron.Cron
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
	if checkIntervalMinutes < 60 {
		checkIntervalMinutes = 60 // 最小1分钟
	}

	// 创建定时任务
	cronScheduler = cron.New()

	// 添加定时任务，每隔指定时间检查一次 API 密钥余额
	spec := fmt.Sprintf("@every %dm", checkIntervalMinutes)
	cronScheduler.AddFunc(spec, checkAllKeysBalance)

	// 添加定时任务，每隔 RecoveryInterval 分钟尝试恢复被禁用的密钥
	recoverySpec := fmt.Sprintf("@every %dm", cfg.App.RecoveryInterval)
	cronScheduler.AddFunc(recoverySpec, tryRecoverDisabledKeys)

	// 添加定时任务，定时刷新已使用过的API密钥余额
	refreshUsedKeysInterval := cfg.App.RefreshUsedKeysInterval
	// 如果配置值小于等于0，使用默认值
	if refreshUsedKeysInterval <= 0 {
		refreshUsedKeysInterval = 60 // 默认每60分钟刷新一次
	}
	refreshUsedKeysSpec := fmt.Sprintf("@every %dm", refreshUsedKeysInterval)
	cronScheduler.AddFunc(refreshUsedKeysSpec, RefreshUsedKeysBalance)

	// 启动定时任务
	cronScheduler.Start()
}

// StopKeyManager 停止API密钥管理器
func StopKeyManager() {
	if cronScheduler != nil {
		cronScheduler.Stop()
		cronScheduler = nil
		logger.Info("API密钥管理器已停止")
	}
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

			// 如果余额为0或负数，根据配置决定是否标记为删除
			if balance <= 0 {
				if config.GetConfig().App.AutoDeleteZeroBalanceKeys {
					logger.Info("API密钥 %s 余额为 %.2f，标记为删除", MaskKey(key.Key), balance)
					config.MarkApiKeyForDeletion(key.Key)
				} else {
					logger.Info("API密钥 %s 余额为 %.2f，但自动删除已禁用", MaskKey(key.Key), balance)
					// 更新余额
					config.UpdateApiKeyBalance(key.Key, balance)
				}
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

	// 从JSON中删除标记为删除的密钥
	config.RemoveMarkedApiKeys()

	// 重新排序 API 密钥（按照综合得分从高到低）
	config.SortApiKeysByBalance()

	logger.Info("API密钥余额检查完成")
}

// CheckKeyBalance 检查 API 密钥余额
// TODO 等待优化
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

	if err = json.Unmarshal(resp.Body(), &result); err != nil {
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
			// TODO 注释
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

// ForceRefreshAllKeysBalance 强制刷新所有API密钥的余额
// 在程序启动时调用，确保所有API密钥的余额都是最新的
// 设置30秒超时限制，如果超时则报错
func ForceRefreshAllKeysBalance() error {
	keys := config.GetApiKeys()
	logger.Info("启动时强制刷新 %d 个API密钥的余额", len(keys))

	// 创建一个等待组，用于等待所有检查完成
	var wg sync.WaitGroup

	// 使用带超时的上下文，强制限制30秒超时
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 创建一个通道用来通知完成
	done := make(chan struct{})

	// 记录错误信息
	var refreshErr error
	var errMu sync.Mutex

	// 限制并发数，避免并发过高导致请求失败
	// 每次最多同时处理50个请求
	const maxConcurrency = 50
	semaphore := make(chan struct{}, maxConcurrency)

	for i := range keys {
		wg.Add(1)
		go func(key config.ApiKey) {
			// 获取信号量
			semaphore <- struct{}{}
			defer func() {
				// 释放信号量
				<-semaphore
				wg.Done()
			}()

			// 检查上下文是否已取消
			select {
			case <-ctx.Done():
				errMu.Lock()
				if refreshErr == nil {
					refreshErr = fmt.Errorf("刷新余额超时了")
				}
				errMu.Unlock()
				logger.Error("强制刷新: 检查API密钥 %s 时上下文已取消", MaskKey(key.Key))
				return
			default:
				// 继续执行
			}

			// 检查余额
			balance, err := CheckKeyBalance(key.Key)
			if err != nil {
				logger.Error("强制刷新: 检查API密钥 %s 余额失败: %v", MaskKey(key.Key), err)
				return
			}

			logger.Info("强制刷新: API密钥 %s 余额: %.2f", MaskKey(key.Key), balance)

			// 如果余额为0或负数，根据配置决定是否标记为删除
			if balance <= 0 {
				if config.GetConfig().App.AutoDeleteZeroBalanceKeys {
					logger.Info("强制刷新: API密钥 %s 余额为 %.2f，标记为删除", MaskKey(key.Key), balance)
					config.MarkApiKeyForDeletion(key.Key)
				} else {
					logger.Info("强制刷新: API密钥 %s 余额为 %.2f，但自动删除已禁用", MaskKey(key.Key), balance)
					// 更新余额
					config.UpdateApiKeyBalance(key.Key, balance)
				}
				return
			}

			// 如果余额低于阈值但状态为启用，禁用它
			if balance < config.GetConfig().App.MinBalanceThreshold && !key.Disabled {
				logger.Info("强制刷新: API密钥 %s 余额 %.2f 低于阈值 %.2f，禁用该密钥",
					MaskKey(key.Key), balance, config.GetConfig().App.MinBalanceThreshold)
				config.DisableApiKey(key.Key)
				return
			}

			// 如果余额高于阈值但状态为禁用，启用它
			if balance >= config.GetConfig().App.MinBalanceThreshold && key.Disabled {
				logger.Info("强制刷新: API密钥 %s 余额 %.2f 高于阈值 %.2f，启用该密钥",
					MaskKey(key.Key), balance, config.GetConfig().App.MinBalanceThreshold)
				config.EnableApiKey(key.Key)
				return
			}

			// 更新余额
			config.UpdateApiKeyBalance(key.Key, balance)
		}(keys[i])
	}

	// 在一个goroutine中等待所有检查完成
	go func() {
		wg.Wait()
		close(done)
	}()

	// 等待完成或超时
	select {
	case <-done:
		logger.Info("所有API密钥余额检查已完成")
	case <-ctx.Done():
		logger.Warn("API密钥余额检查超时，超过30秒限制")
		if refreshErr == nil {
			refreshErr = fmt.Errorf("刷新余额超时了，请稍后再试")
		}
	}

	// 保存更新后的密钥状态
	if err := config.SaveApiKeys(); err != nil {
		logger.Error("强制刷新: 保存API密钥状态失败: %v", err)
		if refreshErr == nil {
			refreshErr = err
		}
	} else {
		logger.Info("强制刷新: 保存API密钥状态成功")
	}

	// 从JSON中删除标记为删除的密钥
	config.RemoveMarkedApiKeys()

	// 重新排序 API 密钥（按照综合得分从高到低）
	config.SortApiKeysByBalance()

	logger.Info("强制刷新API密钥余额完成")
	return refreshErr
}

// RefreshUsedKeysBalance 刷新已使用过的API密钥的余额
// 设置24小时过期时间，过期后会重置IsUsed标记为false
func RefreshUsedKeysBalance() {
	usedKeys := config.GetUsedApiKeys()
	if len(usedKeys) == 0 {
		logger.Info("没有使用过的API密钥需要刷新余额")
		return
	}

	logger.Info("开始刷新 %d 个已使用过的API密钥的余额", len(usedKeys))

	// 检查是否有超过24小时未使用的密钥
	now := time.Now().Unix()
	var keysToReset []string
	var keysToRefresh []config.ApiKey

	// 筛选出需要重置标记和需要刷新的密钥
	for _, key := range usedKeys {
		// 如果最后使用时间超过24小时，重置标记
		if now-key.LastUsed > 24*60*60 {
			keysToReset = append(keysToReset, key.Key)
		} else {
			keysToRefresh = append(keysToRefresh, key)
		}
	}

	// 重置超过24小时未使用的密钥的标记
	for _, keyStr := range keysToReset {
		logger.Info("API密钥 %s 超过24小时未使用，重置使用标记", MaskKey(keyStr))
		config.MarkApiKeyAsUnused(keyStr)
	}

	if len(keysToRefresh) == 0 {
		logger.Info("没有需要刷新余额的已使用API密钥")
		return
	}

	// 创建一个等待组，用于等待所有检查完成
	var wg sync.WaitGroup

	for i := range keysToRefresh {
		wg.Add(1)
		go func(key config.ApiKey) {
			defer wg.Done()

			// 检查余额
			balance, err := CheckKeyBalance(key.Key)
			if err != nil {
				logger.Error("刷新已使用密钥: 检查API密钥 %s 余额失败: %v", MaskKey(key.Key), err)
				return
			}

			logger.Info("刷新已使用密钥: API密钥 %s 余额: %.2f", MaskKey(key.Key), balance)

			// 如果余额为0或负数，根据配置决定是否标记为删除
			if balance <= 0 {
				if config.GetConfig().App.AutoDeleteZeroBalanceKeys {
					logger.Info("刷新已使用密钥: API密钥 %s 余额为 %.2f，标记为删除", MaskKey(key.Key), balance)
					config.MarkApiKeyForDeletion(key.Key)
				} else {
					logger.Info("刷新已使用密钥: API密钥 %s 余额为 %.2f，但自动删除已禁用", MaskKey(key.Key), balance)
					// 更新余额
					config.UpdateApiKeyBalance(key.Key, balance)
				}
				return
			}

			// 如果余额低于阈值但状态为启用，禁用它
			if balance < config.GetConfig().App.MinBalanceThreshold && !key.Disabled {
				logger.Info("刷新已使用密钥: API密钥 %s 余额 %.2f 低于阈值 %.2f，禁用该密钥",
					MaskKey(key.Key), balance, config.GetConfig().App.MinBalanceThreshold)
				config.DisableApiKey(key.Key)
				return
			}

			// 如果余额高于阈值但状态为禁用，启用它
			if balance >= config.GetConfig().App.MinBalanceThreshold && key.Disabled {
				logger.Info("刷新已使用密钥: API密钥 %s 余额 %.2f 高于阈值 %.2f，启用该密钥",
					MaskKey(key.Key), balance, config.GetConfig().App.MinBalanceThreshold)
				config.EnableApiKey(key.Key)
				return
			}

			// 更新余额
			config.UpdateApiKeyBalance(key.Key, balance)
		}(keysToRefresh[i])
	}

	// 等待所有检查完成
	wg.Wait()

	// 保存更新后的密钥状态
	if err := config.SaveApiKeys(); err != nil {
		logger.Error("刷新已使用密钥: 保存API密钥状态失败: %v", err)
	}

	// 从JSON中删除标记为删除的密钥
	config.RemoveMarkedApiKeys()

	// 重新排序 API 密钥（按照综合得分从高到低）
	config.SortApiKeysByBalance()

	logger.Info("已使用API密钥余额刷新完成")
}
