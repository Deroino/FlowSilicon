/**
  @author: Hanhai
  @desc: 通用工具函数集，提供字符计数、密钥掩码和Map操作等功能
**/

package utils

import (
	"encoding/json"
	"os/exec"
)

// estimateTokenCount 估算请求和响应的令牌数
func EstimateTokenCount(requestBody, responseBody []byte) int {
	// 如果是JSON格式，尝试从响应中获取实际的令牌数
	var respData map[string]interface{}
	if err := json.Unmarshal(responseBody, &respData); err == nil {
		if usage, ok := respData["usage"].(map[string]interface{}); ok {
			if totalTokens, ok := usage["total_tokens"].(float64); ok {
				return int(totalTokens)
			}

			// 尝试从 prompt_tokens 和 completion_tokens 计算 total_tokens
			var promptTokens, completionTokens float64
			var hasPrompt, hasCompletion bool

			if pt, ok := usage["prompt_tokens"].(float64); ok {
				promptTokens = pt
				hasPrompt = true
			}

			if ct, ok := usage["completion_tokens"].(float64); ok {
				completionTokens = ct
				hasCompletion = true
			}

			if hasPrompt && hasCompletion {
				return int(promptTokens + completionTokens)
			}
		}
	}

	// 如果无法从响应中获取实际的令牌数，则使用更精确的估算方法
	requestTokens := EstimateStringTokens(string(requestBody))
	responseTokens := EstimateStringTokens(string(responseBody))

	return requestTokens + responseTokens
}

// EstimateStringTokens 估算字符串中的令牌数
// 英文文本：每5个字符算一个token
// 中文文本：每个中文字符算一个token
func EstimateStringTokens(text string) int {
	englishChars := 0
	chineseChars := 0

	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF { // 基本汉字范围
			chineseChars++
		} else {
			englishChars++
		}
	}

	// 英文字符每5个算一个token，中文字符每个算一个token
	englishTokens := englishChars / 5
	if englishChars%5 > 0 {
		englishTokens++ // 处理余数
	}

	return englishTokens + chineseChars
}

// getMapKeys 获取map的所有键
func GetMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// MaskKey 掩盖 API 密钥（用于日志）
func MaskKey(key string) string {
	if len(key) <= 6 {
		return "******"
	}
	return key[:6] + "******"
}

// SetupWindowsRestartCommand 设置Windows重启命令的特定属性
// 对于非Windows平台，这个函数不执行任何操作
func SetupWindowsRestartCommand(cmd *exec.Cmd, isGuiMode bool) {
	// 使用专用的平台特定函数
	setupWindowsSysProcAttr(cmd, isGuiMode)
}
