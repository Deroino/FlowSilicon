/**
  @author: Hanhai
  @since: 2025/3/16 20:44:20
  @desc: 模型特定的密钥选择策略
**/

package key

import (
	"flowsilicon/internal/config"
	"time"
)

// GetModelSpecificKey 根据模型名称获取特定的密钥
func GetModelSpecificKey(modelName string) (string, bool, error) {
	// 检查是否有针对该模型的特定策略配置
	cfg := config.GetConfig()
	if strategyID, exists := cfg.App.ModelKeyStrategies[modelName]; exists {
		// 根据策略ID选择密钥
		switch strategyID {
		case 1: // 高成功率策略
			key, err := getHighSuccessRateKey(modelName)
			return key, true, err
		case 2: // 高分数策略
			key, err := GetOptimalApiKey()
			return key, true, err
		case 3: // 低RPM策略
			key, err := getFastResponseKey()
			return key, true, err
		case 4: // 低TPM策略
			activeKeys := config.GetActiveApiKeys()
			if len(activeKeys) == 0 {
				return "", true, ErrNoActiveKeys
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
				key, err := getAnyAvailableKey()
				return key, true, err
			}

			config.UpdateApiKeyLastUsed(bestKey, time.Now().Unix())
			return bestKey, true, nil
		case 5: // 高余额策略
			key, err := getHighestBalanceKey()
			return key, true, err
		default:
			key, err := GetOptimalApiKey()
			return key, true, err
		}
	}

	// 没有找到特定策略
	return "", false, nil
}
