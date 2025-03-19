/**
  @author: Hanhai
  @since: 2025/3/16 20:44:00
  @desc: 重试配置相关结构体
**/

package config

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries           int   `yaml:"max_retries" mapstructure:"max_retries"`                         // 最大重试次数
	RetryDelayMs         int   `yaml:"retry_delay_ms" mapstructure:"retry_delay_ms"`                   // 重试间隔（毫秒）
	RetryOnStatusCodes   []int `yaml:"retry_on_status_codes" mapstructure:"retry_on_status_codes"`     // 需要重试的HTTP状态码
	RetryOnNetworkErrors bool  `yaml:"retry_on_network_errors" mapstructure:"retry_on_network_errors"` // 是否对网络错误进行重试
}
