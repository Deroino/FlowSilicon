/**
  @author: Hanhai
  @desc: API密钥评分相关函数
**/

package key

import (
	"time"

	"flowsilicon/internal/common"
	"flowsilicon/internal/config"
)

// KeyWithScore 带有得分的API密钥结构
type KeyWithScore struct {
	Key   config.ApiKey
	Score float64
}

// CalculateKeyScores 计算API密钥的权重得分
// 返回按得分从高到低排序的密钥列表
func CalculateKeyScores(allKeys []config.ApiKey) []KeyWithScore {
	// 获取配置
	cfg := config.GetConfig()
	minBalanceThreshold := cfg.App.MinBalanceThreshold

	// 先过滤出未禁用且余额充足的密钥
	var activeKeys []config.ApiKey
	for _, k := range allKeys {
		if !k.Disabled && k.Balance >= minBalanceThreshold {
			activeKeys = append(activeKeys, k)
		}
	}

	// 如果没有活跃的密钥，返回空列表
	if len(activeKeys) == 0 {
		return []KeyWithScore{}
	}

	// 找出各维度的最大值，用于归一化
	var maxRPM, maxTPM int

	for _, k := range activeKeys {
		if k.RequestsPerMinute > maxRPM {
			maxRPM = k.RequestsPerMinute
		}
		if k.TokensPerMinute > maxTPM {
			maxTPM = k.TokensPerMinute
		}
	}

	// 使用配置中的最大余额显示值作为归一化基准
	maxBalance := cfg.App.MaxBalanceDisplay
	if maxBalance <= 0 {
		maxBalance = 14.0 // 默认最大余额显示值
	}

	// 避免除以零
	if maxRPM == 0 {
		maxRPM = 1
	}
	if maxTPM == 0 {
		maxTPM = 1
	}

	// 获取配置的权重，如果未配置则使用默认值
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

	var keysWithScores []KeyWithScore

	// 计算每个活跃密钥的得分
	for _, k := range activeKeys {
		// 1. 余额得分（余额越高，得分越高）
		balanceScore := (k.Balance / maxBalance) * balanceWeight
		if balanceScore > balanceWeight {
			balanceScore = balanceWeight // 确保不超过权重上限
		}

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

		keysWithScores = append(keysWithScores, KeyWithScore{
			Key:   k,
			Score: totalScore,
		})
	}

	// 按得分从高到低排序
	for i := 0; i < len(keysWithScores)-1; i++ {
		for j := 0; j < len(keysWithScores)-i-1; j++ {
			if keysWithScores[j].Score < keysWithScores[j+1].Score {
				keysWithScores[j], keysWithScores[j+1] = keysWithScores[j+1], keysWithScores[j]
			}
		}
	}

	// 将禁用的密钥和余额不足的密钥添加到末尾
	for _, k := range allKeys {
		if k.Disabled || k.Balance < minBalanceThreshold {
			isExist := false
			for _, ks := range keysWithScores {
				if ks.Key.Key == k.Key {
					isExist = true
					break
				}
			}

			if !isExist {
				keysWithScores = append(keysWithScores, KeyWithScore{
					Key:   k,
					Score: 0, // 禁用或余额不足的密钥得分为0
				})
			}
		}
	}

	return keysWithScores
}

// GetOptimalApiKeyWithScore 获取得分最高的API密钥
func GetOptimalApiKeyWithScore() (string, float64, error) {
	activeKeys := config.GetActiveApiKeys()

	if len(activeKeys) == 0 {
		return "", 0, common.ErrNoActiveKeys
	}

	// 计算密钥得分
	keysWithScores := CalculateKeyScores(activeKeys)

	if len(keysWithScores) == 0 {
		return "", 0, common.ErrNoActiveKeys
	}

	// 获取得分最高的密钥
	bestKey := keysWithScores[0].Key.Key
	bestScore := keysWithScores[0].Score

	// 更新最后使用时间
	config.UpdateApiKeyLastUsed(bestKey, time.Now().Unix())

	return bestKey, bestScore, nil
}
