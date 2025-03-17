/**
  @author: Hanhai
  @since: 2025/3/16 20:42:44
  @desc:
**/

package key

import (
	"time"

	"flowsilicon/internal/common"
	"flowsilicon/internal/config"
)

// GetOptimalApiKey 智能负载均衡算法选择最佳API密钥
func GetOptimalApiKey() (string, error) {
	// 使用新的公共函数获取得分最高的密钥
	key, _, err := GetOptimalApiKeyWithScore()
	return key, err
}

// RequestType 定义请求类型
type RequestType string

const (
	RequestTypeEmbedding       RequestType = "embedding"
	RequestTypeCompletion      RequestType = "completion"
	RequestTypeLargeCompletion RequestType = "large_completion"
	RequestTypeStreaming       RequestType = "streaming"
)

// 根据余额范围获取密钥
func getKeysByBalanceRange(minBalance, maxBalance float64) []config.ApiKey {
	activeKeys := config.GetActiveApiKeys()
	var result []config.ApiKey

	for _, key := range activeKeys {
		if key.Balance >= minBalance && key.Balance < maxBalance {
			result = append(result, key)
		}
	}

	return result
}

// 从密钥列表中随机选择一个
func getRandomKey(keys []config.ApiKey) string {
	if len(keys) == 0 {
		return ""
	}

	// 简单实现，使用第一个密钥
	// 实际应用中可以使用随机数生成器
	return keys[0].Key
}

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
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	var highestKey string
	var highestBalance float64 = -1

	for _, key := range activeKeys {
		if key.Balance > highestBalance {
			highestBalance = key.Balance
			highestKey = key.Key
		}
	}

	return highestKey, nil
}

// 获取非保留密钥
func getNonReservedKey(reservedKeys map[string]bool) (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	for _, key := range activeKeys {
		if !reservedKeys[key.Key] && key.Balance >= config.GetConfig().App.MinBalanceThreshold {
			return key.Key, nil
		}
	}

	// 如果没有非保留密钥，则返回任意密钥
	return activeKeys[0].Key, nil
}

// GetKeyByBalanceTier 余额分层策略
func GetKeyByBalanceTier(requestType RequestType) (string, error) {
	// 将密钥分为高、中、低三个层级
	highBalanceKeys := getKeysByBalanceRange(10.0, 999.0)
	mediumBalanceKeys := getKeysByBalanceRange(5.0, 10.0)
	lowBalanceKeys := getKeysByBalanceRange(1.0, 5.0)

	switch requestType {
	case RequestTypeEmbedding: // 嵌入请求消耗较少，可以使用低余额密钥
		if len(lowBalanceKeys) > 0 {
			key := getRandomKey(lowBalanceKeys)
			config.UpdateApiKeyLastUsed(key, time.Now().Unix())
			return key, nil
		}
		fallthrough
	case RequestTypeCompletion: // 普通补全请求使用中等余额密钥
		if len(mediumBalanceKeys) > 0 {
			key := getRandomKey(mediumBalanceKeys)
			config.UpdateApiKeyLastUsed(key, time.Now().Unix())
			return key, nil
		}
		fallthrough
	case RequestTypeLargeCompletion: // 大型补全请求使用高余额密钥
		if len(highBalanceKeys) > 0 {
			key := getRandomKey(highBalanceKeys)
			config.UpdateApiKeyLastUsed(key, time.Now().Unix())
			return key, nil
		}
	}

	// 如果没有找到合适的密钥，返回任意可用密钥
	return getAnyAvailableKey()
}

// 获取历史成功率高的密钥
func getHighSuccessRateKey(modelName string) (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	var bestKey string
	var bestRate float64 = -1

	for _, key := range activeKeys {
		if key.Balance < config.GetConfig().App.MinBalanceThreshold {
			continue
		}

		if key.SuccessRate > bestRate {
			bestRate = key.SuccessRate
			bestKey = key.Key
		}
	}

	if bestKey == "" {
		return getAnyAvailableKey()
	}

	config.UpdateApiKeyLastUsed(bestKey, time.Now().Unix())
	return bestKey, nil
}

// 获取响应速度快的密钥
func getFastResponseKey() (string, error) {
	// 这里可以实现基于历史响应时间的选择逻辑
	// 简化实现，使用负载最低的密钥作为响应速度快的密钥
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	var bestKey string
	var lowestRPM int = 999999

	for _, key := range activeKeys {
		if key.Balance < config.GetConfig().App.MinBalanceThreshold {
			continue
		}

		if key.RequestsPerMinute < lowestRPM {
			lowestRPM = key.RequestsPerMinute
			bestKey = key.Key
		}
	}

	if bestKey == "" {
		return getAnyAvailableKey()
	}

	config.UpdateApiKeyLastUsed(bestKey, time.Now().Unix())
	return bestKey, nil
}

// GetKeyByStrategy 根据指定的策略选择密钥
func GetKeyByStrategy(strategy KeySelectionStrategy) (string, error) {
	switch strategy {
	case StrategyHighSuccessRate:
		return getHighSuccessRateKey("")
	case StrategyHighScore:
		return GetOptimalApiKey()
	case StrategyLowRPM:
		return getLowRPMKey()
	case StrategyLowTPM:
		return getLowTPMKey()
	case StrategyHighBalance:
		return getHighestBalanceKey()
	default:
		return GetOptimalApiKey()
	}
}

// getLowRPMKey 获取RPM最低的密钥
func getLowRPMKey() (string, error) {
	// 使用getFastResponseKey函数，因为它已经实现了获取RPM最低的密钥的逻辑
	return getFastResponseKey()
}

// getLowTPMKey 获取TPM最低的密钥
func getLowTPMKey() (string, error) {
	activeKeys := config.GetActiveApiKeys()
	if len(activeKeys) == 0 {
		return "", common.ErrNoActiveKeys
	}

	var bestKey string
	var lowestTPM int = 999999

	for _, key := range activeKeys {
		if key.Balance < config.GetConfig().App.MinBalanceThreshold {
			continue
		}

		if key.TokensPerMinute < lowestTPM {
			lowestTPM = key.TokensPerMinute
			bestKey = key.Key
		}
	}

	if bestKey == "" {
		return getAnyAvailableKey()
	}

	config.UpdateApiKeyLastUsed(bestKey, time.Now().Unix())
	return bestKey, nil
}

// GetBestKeyForRequest 根据请求类型选择最佳密钥
func GetBestKeyForRequest(requestType string, modelName string, tokenEstimate int) (string, error) {
	// 对于大型请求，选择余额高的密钥
	if tokenEstimate > 5000 {
		return getHighestBalanceKey()
	}

	// 检查是否有针对该模型的特定策略配置
	key, found, err := GetModelSpecificKey(modelName)
	if found {
		return key, err
	}

	// 对于流式请求，选择响应速度快的密钥
	if requestType == "streaming" {
		return getFastResponseKey()
	}

	// 默认使用智能负载均衡策略
	return GetOptimalApiKey()
}

// 预测性故障规避
func isPredictedToFail(key string) bool {
	// 简化实现，基于连续失败次数
	activeKeys := config.GetActiveApiKeys()

	for _, k := range activeKeys {
		if k.Key == key && k.ConsecutiveFailures >= 2 {
			return true
		}
	}

	return false
}

// 保留高余额密钥的映射
var reservedHighBalanceKeys = make(map[string]bool)

// ReserveHighBalanceKeys 保留一定数量的高余额密钥
func ReserveHighBalanceKeys(count int) {
	// 清空之前的保留
	reservedHighBalanceKeys = make(map[string]bool)

	// 获取所有密钥并按余额排序
	config.SortApiKeysByBalance()
	sortedKeys := config.GetApiKeys()

	// 保留指定数量的高余额密钥
	for i := 0; i < min(count, len(sortedKeys)); i++ {
		reservedHighBalanceKeys[sortedKeys[i].Key] = true
	}
}

// GetKeyForImportantRequest 获取用于重要请求的密钥
func GetKeyForImportantRequest(isImportant bool) (string, error) {
	if isImportant {
		// 重要请求可以使用保留的高余额密钥
		return getHighestBalanceKey()
	} else {
		// 普通请求只能使用非保留密钥
		return getNonReservedKey(reservedHighBalanceKeys)
	}
}

// 错误定义
var (
	ErrNoActiveKeys = common.NewApiError("no active API keys available", 500)
)

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// calculateKeyScore 计算单个密钥的得分
func calculateKeyScore(key config.ApiKey) float64 {
	cfg := config.GetConfig()

	// 获取权重
	balanceWeight := cfg.App.BalanceWeight
	successRateWeight := cfg.App.SuccessRateWeight
	rpmWeight := cfg.App.RPMWeight
	tpmWeight := cfg.App.TPMWeight

	// 计算余额得分 (0-1)
	balanceScore := key.Balance / cfg.App.MaxBalanceDisplay
	if balanceScore > 1 {
		balanceScore = 1
	}

	// 计算成功率得分 (0-1)
	successRateScore := key.SuccessRate

	// 计算RPM得分 (0-1)，RPM越低得分越高
	rpmScore := 1.0
	if key.RequestsPerMinute > 0 {
		// 假设最大RPM为100
		maxRPM := 100.0
		rpmScore = 1 - (float64(key.RequestsPerMinute) / maxRPM)
		if rpmScore < 0 {
			rpmScore = 0
		}
	}

	// 计算TPM得分 (0-1)，TPM越低得分越高
	tpmScore := 1.0
	if key.TokensPerMinute > 0 {
		// 假设最大TPM为10000
		maxTPM := 10000.0
		tpmScore = 1 - (float64(key.TokensPerMinute) / maxTPM)
		if tpmScore < 0 {
			tpmScore = 0
		}
	}

	// 计算加权总分
	totalScore := balanceWeight*balanceScore +
		successRateWeight*successRateScore +
		rpmWeight*rpmScore +
		tpmWeight*tpmScore

	return totalScore
}
