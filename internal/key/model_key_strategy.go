/**
  @author: Hanhai
  @desc: 模型特定的密钥选择策略
**/

package key

import (
	"flowsilicon/internal/config"
	"flowsilicon/internal/logger"
	"flowsilicon/internal/model"
	"strings"
)

// KeySelectionStrategy 定义密钥选择策略类型
type KeySelectionStrategy int

// GetModelSpecificKey 根据模型名称获取特定的密钥
func GetModelSpecificKey(modelName string) (string, bool, error) {
	logger.Info("检查模型特定策略: 模型=%s", modelName)

	// 首先从models表中获取模型的策略
	strategyID, err := model.GetModelStrategy(modelName)
	if err != nil {
		logger.Error("从数据库获取模型策略失败: %v", err)
		// 如果获取失败，回退到配置文件中查找
		return getModelStrategyFromConfig(modelName)
	}

	// 如果找到策略（strategyID > 0），应用它
	if strategyID > 0 {
		logger.Info("从数据库找到模型特定策略: 模型=%s, 策略ID=%d", modelName, strategyID)
		return applyModelStrategy(modelName, strategyID)
	}

	// 如果数据库中没有指定策略，回退到配置文件中查找
	logger.Info("数据库中没有模型策略，回退到配置查找: 模型=%s", modelName)
	return getModelStrategyFromConfig(modelName)
}

// getModelStrategyFromConfig 从配置文件中获取模型策略（为了向后兼容）
func getModelStrategyFromConfig(modelName string) (string, bool, error) {
	// 检查是否有针对该模型的特定策略配置
	cfg := config.GetConfig()

	// 添加调试日志
	logger.Info("从配置中检查模型特定策略: 模型=%s", modelName)
	logger.Info("当前配置的模型策略列表: %v", cfg.App.ModelKeyStrategies)

	// 直接查找精确匹配
	if strategyID, exists := cfg.App.ModelKeyStrategies[modelName]; exists {
		// 记录找到的策略
		logger.Info("从配置找到模型特定策略(精确匹配): 模型=%s, 策略ID=%d", modelName, strategyID)

		// 将策略ID保存到数据库中
		if err := model.UpdateModelStrategy(modelName, strategyID); err != nil {
			logger.Error("更新模型策略到数据库失败: %v", err)
		}

		return applyModelStrategy(modelName, strategyID)
	}

	// 如果精确匹配失败，尝试不区分大小写的匹配
	modelNameLower := strings.ToLower(modelName)
	for configModel, strategyID := range cfg.App.ModelKeyStrategies {
		if strings.ToLower(configModel) == modelNameLower {
			// 记录找到的策略
			logger.Info("从配置找到模型特定策略(不区分大小写): 模型=%s 匹配配置=%s, 策略ID=%d",
				modelName, configModel, strategyID)

			// 将策略ID保存到数据库中
			if err := model.UpdateModelStrategy(modelName, strategyID); err != nil {
				logger.Error("更新模型策略到数据库失败: %v", err)
			}

			return applyModelStrategy(modelName, strategyID)
		}
	}

	// 没有找到特定策略
	logger.Info("未找到模型特定策略: 模型=%s", modelName)
	return "", false, nil
}

// applyModelStrategy 应用模型特定策略
func applyModelStrategy(modelName string, strategyID int) (string, bool, error) {
	switch strategyID {
	case 1: // 高成功率策略
		logger.Info("使用高成功率策略选择密钥: 模型=%s", modelName)
		key, err := getHighSuccessRateKey(modelName)
		return key, true, err
	case 2: // 高分数策略
		logger.Info("使用高分数策略选择密钥: 模型=%s", modelName)
		key, err := GetOptimalApiKeyWithRoundRobin()
		return key, true, err
	case 3: // 低RPM策略
		logger.Info("使用低RPM策略选择密钥: 模型=%s", modelName)
		key, err := getLowRPMKey()
		return key, true, err
	case 4: // 低TPM策略
		logger.Info("使用低TPM策略选择密钥: 模型=%s", modelName)
		key, err := getLowTPMKey()
		return key, true, err
	case 5: // 高余额策略
		logger.Info("使用高余额策略选择密钥: 模型=%s", modelName)
		key, err := getHighestBalanceKey()
		return key, true, err
	case 6: // 普通轮询策略
		logger.Info("使用普通轮询策略选择密钥: 模型=%s", modelName)
		key, err := getRoundRobinKey()
		return key, true, err
	case 7: // 低余额策略
		logger.Info("使用低余额策略选择密钥: 模型=%s", modelName)
		key, err := getLowestBalanceKey()
		return key, true, err
	case 8: // 免费模型策略
		logger.Info("使用免费模型策略选择密钥: 模型=%s", modelName)
		key, err := getFreeModelKey()
		return key, true, err
	default:
		logger.Info("使用默认策略(普通轮询)选择密钥: 模型=%s", modelName)
		key, err := getRoundRobinKey()
		return key, true, err
	}
}
