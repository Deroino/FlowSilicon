/**
  @author: Hanhai
  @since: 2025/3/16 20:45:30
  @desc: 密钥选择策略定义
**/

package key

// KeySelectionStrategy 定义密钥选择策略类型
type KeySelectionStrategy int

// 密钥选择策略常量
const (
	StrategyHighSuccessRate KeySelectionStrategy = 1 // 高成功率策略
	StrategyHighScore       KeySelectionStrategy = 2 // 高分数策略
	StrategyLowRPM          KeySelectionStrategy = 3 // 低RPM策略
	StrategyLowTPM          KeySelectionStrategy = 4 // 低TPM策略
	StrategyHighBalance     KeySelectionStrategy = 5 // 高余额策略
)
