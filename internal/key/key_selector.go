/**
  @author: Hanhai
  @desc: API密钥选择器，提供多种智能选择策略和负载均衡算法
**/

package key

import (
	"fmt"
	"sync"
	"time"

	"flowsilicon/internal/common"
	"flowsilicon/internal/config"
	"flowsilicon/internal/logger"
	"flowsilicon/pkg/utils"
)

// 添加用于轮询的全局变量
var (
	// 记录每种策略的当前轮询索引
	strategyRoundRobinIndex map[string]int = make(map[string]int)
	// 互斥锁保护轮询索引的并发访问
	rrMutex sync.Mutex
)

// GetOptimalApiKey 智能负载均衡算法选择最佳API密钥
func GetOptimalApiKey() (string, error) {
	// 使用新的公共函数获取得分最高的密钥
	key, _, err := GetOptimalApiKeyWithScore()
	return key, err
}

// RequestType 定义请求类型
type RequestType string

// 获取任意可用密钥
func getAnyAvailableKey() (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}
	return activeKeys[0].Key, nil
}

// 获取余额最高的密钥
func getHighestBalanceKey() (string, error) {
	return getHighestBalanceKeyWithRoundRobin()
}

// 获取余额最高的密钥（支持轮询）
func getHighestBalanceKeyWithRoundRobin() (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	// 先找出最高余额值
	var highestBalance float64 = -1
	for _, key := range activeKeys {
		if key.Balance > highestBalance {
			highestBalance = key.Balance
		}
	}

	// 收集所有具有最高余额的密钥
	var highestBalanceKeys []config.ApiKey
	for _, key := range activeKeys {
		if key.Balance == highestBalance {
			highestBalanceKeys = append(highestBalanceKeys, key)
		}
	}

	// 增加详细日志
	logger.Info("找到%d个具有相同最高余额(%.2f)的密钥", len(highestBalanceKeys), highestBalance)

	// 记录所有找到的密钥以便调试
	if len(highestBalanceKeys) > 1 {
		keyList := ""
		for i, k := range highestBalanceKeys {
			if i > 0 {
				keyList += ", "
			}
			keyList += utils.MaskKey(k.Key)
		}
		logger.Info("可用于轮询的高余额密钥列表: %s", keyList)
	}

	// 记录当前轮询索引
	rrMutex.Lock()
	currentIndex := strategyRoundRobinIndex["high_balance"]
	rrMutex.Unlock()

	logger.Info("轮询选择: 策略=high_balance, 当前索引=%d, 总密钥数=%d",
		currentIndex, len(highestBalanceKeys))

	// 使用轮询选择器获取密钥
	selectedKey := selectKeyByRoundRobin(highestBalanceKeys, "high_balance")
	if selectedKey == "" {
		return "", common.ErrNoActiveKeys
	}

	// 记录选中的密钥和更新后的索引
	rrMutex.Lock()
	newIndex := strategyRoundRobinIndex["high_balance"]
	rrMutex.Unlock()

	logger.Info("轮询结果: 策略=high_balance, 选择密钥=%s, 新索引=%d",
		utils.MaskKey(selectedKey), newIndex)

	// 更新最后使用时间
	config.UpdateApiKeyLastUsed(selectedKey, time.Now().Unix())
	return selectedKey, nil
}

// 获取历史成功率高的密钥
func getHighSuccessRateKey(modelName string) (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	// 找出最高成功率
	var bestRate float64 = -1
	for _, key := range activeKeys {
		if key.Balance < config.GetConfig().App.MinBalanceThreshold {
			continue
		}

		if key.SuccessRate > bestRate {
			bestRate = key.SuccessRate
		}
	}

	// 收集所有具有最高成功率的密钥
	var highSuccessKeys []config.ApiKey
	for _, key := range activeKeys {
		if key.Balance >= config.GetConfig().App.MinBalanceThreshold && key.SuccessRate == bestRate {
			highSuccessKeys = append(highSuccessKeys, key)
		}
	}

	if len(highSuccessKeys) == 0 {
		return getAnyAvailableKey()
	}

	// 增加详细日志
	logger.Info("找到%d个具有相同最高成功率(%.2f)的密钥", len(highSuccessKeys), bestRate)

	// 记录所有找到的密钥以便调试
	if len(highSuccessKeys) > 1 {
		keyList := ""
		for i, k := range highSuccessKeys {
			if i > 0 {
				keyList += ", "
			}
			keyList += utils.MaskKey(k.Key)
		}
		logger.Info("可用于轮询的密钥列表: %s", keyList)
	}

	// 使用轮询选择器
	strategyKey := "high_success_rate"
	if modelName != "" {
		strategyKey = "high_success_rate_" + modelName
	}

	// 记录当前轮询索引
	rrMutex.Lock()
	currentIndex := strategyRoundRobinIndex[strategyKey]
	rrMutex.Unlock()

	logger.Info("轮询选择: 策略=%s, 当前索引=%d, 总密钥数=%d",
		strategyKey, currentIndex, len(highSuccessKeys))

	selectedKey := selectKeyByRoundRobin(highSuccessKeys, strategyKey)

	// 记录选中的密钥和更新后的索引
	rrMutex.Lock()
	newIndex := strategyRoundRobinIndex[strategyKey]
	rrMutex.Unlock()

	logger.Info("轮询结果: 策略=%s, 选择密钥=%s, 新索引=%d",
		strategyKey, utils.MaskKey(selectedKey), newIndex)

	config.UpdateApiKeyLastUsed(selectedKey, time.Now().Unix())
	return selectedKey, nil
}

// 获取响应速度快的密钥
func getFastResponseKey() (string, error) {
	// 使用低RPM策略
	return getLowRPMKey()
}

// getLowRPMKey 获取RPM最低的密钥
func getLowRPMKey() (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	// 找出最低RPM值
	var lowestRPM int = 999999
	for _, key := range activeKeys {
		if key.Balance < config.GetConfig().App.MinBalanceThreshold {
			continue
		}

		if key.RequestsPerMinute < lowestRPM {
			lowestRPM = key.RequestsPerMinute
		}
	}

	// 收集所有RPM最低的密钥
	var lowestRPMKeys []config.ApiKey
	for _, key := range activeKeys {
		if key.Balance >= config.GetConfig().App.MinBalanceThreshold && key.RequestsPerMinute == lowestRPM {
			lowestRPMKeys = append(lowestRPMKeys, key)
		}
	}

	if len(lowestRPMKeys) == 0 {
		return getAnyAvailableKey()
	}

	// 增加详细日志
	logger.Info("找到%d个具有相同最低RPM(%d)的密钥", len(lowestRPMKeys), lowestRPM)

	// 记录所有找到的密钥以便调试
	if len(lowestRPMKeys) > 1 {
		keyList := ""
		for i, k := range lowestRPMKeys {
			if i > 0 {
				keyList += ", "
			}
			keyList += utils.MaskKey(k.Key)
		}
		logger.Info("可用于轮询的低RPM密钥列表: %s", keyList)
	}

	// 记录当前轮询索引
	rrMutex.Lock()
	currentIndex := strategyRoundRobinIndex["low_rpm"]
	rrMutex.Unlock()

	logger.Info("轮询选择: 策略=low_rpm, 当前索引=%d, 总密钥数=%d",
		currentIndex, len(lowestRPMKeys))

	// 使用轮询选择器
	selectedKey := selectKeyByRoundRobin(lowestRPMKeys, "low_rpm")

	// 记录选中的密钥和更新后的索引
	rrMutex.Lock()
	newIndex := strategyRoundRobinIndex["low_rpm"]
	rrMutex.Unlock()

	logger.Info("轮询结果: 策略=low_rpm, 选择密钥=%s, 新索引=%d",
		utils.MaskKey(selectedKey), newIndex)

	config.UpdateApiKeyLastUsed(selectedKey, time.Now().Unix())
	return selectedKey, nil
}

// getLowTPMKey 获取TPM最低的密钥
func getLowTPMKey() (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	// 找出最低TPM值
	var lowestTPM int = 999999
	for _, key := range activeKeys {
		if key.Balance < config.GetConfig().App.MinBalanceThreshold {
			continue
		}

		if key.TokensPerMinute < lowestTPM {
			lowestTPM = key.TokensPerMinute
		}
	}

	// 收集所有TPM最低的密钥
	var lowestTPMKeys []config.ApiKey
	for _, key := range activeKeys {
		if key.Balance >= config.GetConfig().App.MinBalanceThreshold && key.TokensPerMinute == lowestTPM {
			lowestTPMKeys = append(lowestTPMKeys, key)
		}
	}

	if len(lowestTPMKeys) == 0 {
		return getAnyAvailableKey()
	}

	// 增加详细日志
	logger.Info("找到%d个具有相同最低TPM(%d)的密钥", len(lowestTPMKeys), lowestTPM)

	// 记录所有找到的密钥以便调试
	if len(lowestTPMKeys) > 1 {
		keyList := ""
		for i, k := range lowestTPMKeys {
			if i > 0 {
				keyList += ", "
			}
			keyList += utils.MaskKey(k.Key)
		}
		logger.Info("可用于轮询的低TPM密钥列表: %s", keyList)
	}

	// 记录当前轮询索引
	rrMutex.Lock()
	currentIndex := strategyRoundRobinIndex["low_tpm"]
	rrMutex.Unlock()

	logger.Info("轮询选择: 策略=low_tpm, 当前索引=%d, 总密钥数=%d",
		currentIndex, len(lowestTPMKeys))

	// 使用轮询选择器
	selectedKey := selectKeyByRoundRobin(lowestTPMKeys, "low_tpm")

	// 记录选中的密钥和更新后的索引
	rrMutex.Lock()
	newIndex := strategyRoundRobinIndex["low_tpm"]
	rrMutex.Unlock()

	logger.Info("轮询结果: 策略=low_tpm, 选择密钥=%s, 新索引=%d",
		utils.MaskKey(selectedKey), newIndex)

	config.UpdateApiKeyLastUsed(selectedKey, time.Now().Unix())
	return selectedKey, nil
}

// GetBestKeyForRequest 根据请求类型选择最佳密钥
func GetBestKeyForRequest(requestType string, modelName string, tokenEstimate int) (string, error) {

	// 添加调试日志
	logger.Info("GetBestKeyForRequest被调用: 模型=%s, 请求类型=%s, 预估token=%d", modelName, requestType, tokenEstimate)

	// 检查是否有针对该模型的特定策略配置
	key, found, err := GetModelSpecificKey(modelName)
	logger.Info("模型特定策略查找结果: 模型=%s, 找到策略=%v", modelName, found)

	if found {
		logger.Info("使用模型特定策略: 模型=%s, 选择密钥=%s", modelName, utils.MaskKey(key))
		return key, err
	}

	// 对于大型请求，选择余额高的密钥
	if tokenEstimate > 5000 {
		return getHighestBalanceKey()
	}

	// 对于流式请求，选择响应速度快的密钥
	if requestType == "streaming" {
		return getFastResponseKey()
	}

	// 默认使用普通轮询策略（而不是智能负载均衡策略）
	return getRoundRobinKey()
}

// selectKeyByRoundRobin 使用轮询方式从密钥列表中选择一个
func selectKeyByRoundRobin(keys []config.ApiKey, strategyName string) string {
	if len(keys) == 0 {
		return ""
	}

	// 只有一个密钥时直接返回
	if len(keys) == 1 {
		logger.Info("轮询: 策略=%s 只有1个密钥可用，直接返回", strategyName)
		return keys[0].Key
	}

	// 获取当前索引
	rrMutex.Lock()

	// 确保索引存在
	index, exists := strategyRoundRobinIndex[strategyName]
	if !exists {
		logger.Info("轮询: 策略=%s 首次使用，初始化索引为0", strategyName)
		index = 0
	}

	// 确保索引在有效范围内
	if index >= len(keys) {
		logger.Info("轮询: 策略=%s 索引越界(%d >= %d)，重置为0",
			strategyName, index, len(keys))
		index = 0
	}

	// 获取当前密钥
	selectedKey := keys[index].Key

	// 更新索引
	strategyRoundRobinIndex[strategyName] = (index + 1) % len(keys)

	logger.Info("轮询: 策略=%s 从索引%d选择密钥%s, 下次索引更新为%d",
		strategyName, index, utils.MaskKey(selectedKey),
		strategyRoundRobinIndex[strategyName])

	rrMutex.Unlock()

	return selectedKey
}

// GetOptimalApiKeyWithRoundRobin 获取得分最高的API密钥，带轮询功能
func GetOptimalApiKeyWithRoundRobin() (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	// 计算密钥得分
	keysWithScores := CalculateKeyScores(activeKeys)
	if len(keysWithScores) == 0 {
		return "", common.ErrNoActiveKeys
	}

	// 获取最高分数
	highestScore := keysWithScores[0].Score

	// 收集所有具有最高分数的密钥
	var highestScoreKeys []config.ApiKey
	for _, keyWithScore := range keysWithScores {
		if keyWithScore.Score == highestScore {
			highestScoreKeys = append(highestScoreKeys, keyWithScore.Key)
		}
	}

	// 增加详细日志
	logger.Info("找到%d个具有相同最高分数(%.4f)的密钥", len(highestScoreKeys), highestScore)

	// 记录所有找到的密钥以便调试
	if len(highestScoreKeys) > 1 {
		keyList := ""
		for i, k := range highestScoreKeys {
			if i > 0 {
				keyList += ", "
			}
			keyList += utils.MaskKey(k.Key)
		}
		logger.Info("可用于轮询的高分数密钥列表: %s", keyList)
	}

	// 记录当前轮询索引
	rrMutex.Lock()
	currentIndex := strategyRoundRobinIndex["high_score"]
	rrMutex.Unlock()

	logger.Info("轮询选择: 策略=high_score, 当前索引=%d, 总密钥数=%d",
		currentIndex, len(highestScoreKeys))

	// 使用轮询选择器
	selectedKey := selectKeyByRoundRobin(highestScoreKeys, "high_score")
	if selectedKey == "" {
		return "", common.ErrNoActiveKeys
	}

	// 记录选中的密钥和更新后的索引
	rrMutex.Lock()
	newIndex := strategyRoundRobinIndex["high_score"]
	rrMutex.Unlock()

	logger.Info("轮询结果: 策略=high_score, 选择密钥=%s, 新索引=%d",
		utils.MaskKey(selectedKey), newIndex)

	// 更新最后使用时间
	config.UpdateApiKeyLastUsed(selectedKey, time.Now().Unix())

	return selectedKey, nil
}

// getRoundRobinKey 实现普通轮询策略，轮询所有可用的API密钥
func getRoundRobinKey() (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	// 增加详细日志
	logger.Info("轮询策略: 找到%d个可用的API密钥进行轮询", len(activeKeys))

	// 记录所有找到的密钥以便调试
	if len(activeKeys) > 1 {
		keyList := ""
		for i, k := range activeKeys {
			if i > 0 {
				keyList += ", "
			}
			keyList += utils.MaskKey(k.Key)
		}
		logger.Info("可用于轮询的API密钥列表: %s", keyList)
	}

	// 记录当前轮询索引
	rrMutex.Lock()
	currentIndex := strategyRoundRobinIndex["round_robin"]
	rrMutex.Unlock()

	logger.Info("轮询选择: 策略=round_robin, 当前索引=%d, 总密钥数=%d",
		currentIndex, len(activeKeys))

	// 使用轮询选择器获取密钥
	selectedKey := selectKeyByRoundRobin(activeKeys, "round_robin")
	if selectedKey == "" {
		return "", common.ErrNoActiveKeys
	}

	// 记录选中的密钥和更新后的索引
	rrMutex.Lock()
	newIndex := strategyRoundRobinIndex["round_robin"]
	rrMutex.Unlock()

	logger.Info("轮询结果: 策略=round_robin, 选择密钥=%s, 新索引=%d",
		utils.MaskKey(selectedKey), newIndex)

	// 更新最后使用时间
	config.UpdateApiKeyLastUsed(selectedKey, time.Now().Unix())
	return selectedKey, nil
}

// 获取余额最低的密钥（支持轮询）
func getLowestBalanceKeyWithRoundRobin() (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	// 先找出最低余额值
	var lowestBalance float64 = 999999.0
	for _, key := range activeKeys {
		if key.Balance < lowestBalance {
			lowestBalance = key.Balance
		}
	}

	// 收集所有具有最低余额的密钥
	var lowestBalanceKeys []config.ApiKey
	for _, key := range activeKeys {
		if key.Balance == lowestBalance {
			lowestBalanceKeys = append(lowestBalanceKeys, key)
		}
	}

	// 增加详细日志
	logger.Info("找到%d个具有相同最低余额(%.2f)的密钥", len(lowestBalanceKeys), lowestBalance)

	// 记录所有找到的密钥以便调试
	if len(lowestBalanceKeys) > 1 {
		keyList := ""
		for i, k := range lowestBalanceKeys {
			if i > 0 {
				keyList += ", "
			}
			keyList += utils.MaskKey(k.Key)
		}
		logger.Info("可用于轮询的低余额密钥列表: %s", keyList)
	}

	// 记录当前轮询索引
	rrMutex.Lock()
	currentIndex := strategyRoundRobinIndex["low_balance"]
	rrMutex.Unlock()

	logger.Info("轮询选择: 策略=low_balance, 当前索引=%d, 总密钥数=%d",
		currentIndex, len(lowestBalanceKeys))

	// 使用轮询选择器获取密钥
	selectedKey := selectKeyByRoundRobin(lowestBalanceKeys, "low_balance")
	if selectedKey == "" {
		return "", common.ErrNoActiveKeys
	}

	// 记录选中的密钥和更新后的索引
	rrMutex.Lock()
	newIndex := strategyRoundRobinIndex["low_balance"]
	rrMutex.Unlock()

	logger.Info("轮询结果: 策略=low_balance, 选择密钥=%s, 新索引=%d",
		utils.MaskKey(selectedKey), newIndex)

	// 更新最后使用时间
	config.UpdateApiKeyLastUsed(selectedKey, time.Now().Unix())
	return selectedKey, nil
}

// getLowestBalanceKey 获取余额最低的密钥
func getLowestBalanceKey() (string, error) {
	return getLowestBalanceKeyWithRoundRobin()
}

// getFreeModelKey 实现免费模型的策略
// 先轮询is_delete为1的密钥，再轮询disabled为1的密钥，再轮询is_used为0的密钥，最后使用低余额策略
func getFreeModelKey() (string, error) {
	// 获取所有API密钥（包括禁用的，但不包括已标记为删除的）
	allKeys := config.GetApiKeys()
	if len(allKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	// 1. 首先尝试使用已标记为删除的密钥（不在GetApiKeys结果中，需要单独获取）
	deletedKeys, err := getDeletedApiKeys()
	if err != nil {
		logger.Error("获取已删除密钥失败: %v", err)
	} else if len(deletedKeys) > 0 {
		logger.Info("找到%d个已删除的密钥，尝试使用", len(deletedKeys))

		// 使用轮询选择器
		selectedKey := selectKeyByRoundRobin(deletedKeys, "free_deleted")
		if selectedKey != "" {
			logger.Info("使用已删除的密钥: %s", utils.MaskKey(selectedKey))
			return selectedKey, nil
		}
	}

	// 2. 尝试使用已禁用的密钥
	var disabledKeys []config.ApiKey
	for _, key := range allKeys {
		if key.Disabled {
			disabledKeys = append(disabledKeys, key)
		}
	}

	if len(disabledKeys) > 0 {
		logger.Info("找到%d个已禁用的密钥，尝试使用", len(disabledKeys))

		// 使用轮询选择器
		selectedKey := selectKeyByRoundRobin(disabledKeys, "free_disabled")
		if selectedKey != "" {
			logger.Info("使用已禁用的密钥: %s", utils.MaskKey(selectedKey))
			return selectedKey, nil
		}
	}

	// 3. 尝试使用未使用过的密钥
	var unusedKeys []config.ApiKey
	for _, key := range allKeys {
		if !key.IsUsed && !key.Disabled {
			unusedKeys = append(unusedKeys, key)
		}
	}

	if len(unusedKeys) > 0 {
		logger.Info("找到%d个未使用过的密钥，尝试使用", len(unusedKeys))

		// 使用轮询选择器
		selectedKey := selectKeyByRoundRobin(unusedKeys, "free_unused")
		if selectedKey != "" {
			logger.Info("使用未使用过的密钥: %s", utils.MaskKey(selectedKey))
			return selectedKey, nil
		}
	}

	// 4. 最后尝试使用低余额策略
	logger.Info("尝试使用低余额策略选择密钥")
	return getLowestBalanceKey()
}

// getDeletedApiKeys 获取所有标记为已删除的API密钥
func getDeletedApiKeys() ([]config.ApiKey, error) {
	// 从数据库中查询已标记为删除的密钥
	if config.DB() == nil {
		return nil, fmt.Errorf("数据库连接未初始化")
	}

	rows, err := config.DB().Query(`SELECT 
		key, balance, last_used, total_calls, success_calls, success_rate, 
		consecutive_failures, disabled, disabled_at, last_tested, rpm, tpm, score, is_delete, is_used 
		FROM apikeys WHERE is_delete = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deletedKeys []config.ApiKey
	for rows.Next() {
		var key config.ApiKey
		if err := rows.Scan(
			&key.Key,
			&key.Balance,
			&key.LastUsed,
			&key.TotalCalls,
			&key.SuccessCalls,
			&key.SuccessRate,
			&key.ConsecutiveFailures,
			&key.Disabled,
			&key.DisabledAt,
			&key.LastTested,
			&key.RequestsPerMinute,
			&key.TokensPerMinute,
			&key.Score,
			&key.Delete,
			&key.IsUsed,
		); err != nil {
			return nil, err
		}

		deletedKeys = append(deletedKeys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return deletedKeys, nil
}
