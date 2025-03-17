/**
  @author: Hanhai
  @since: 2025/3/16 20:43:30
  @desc:
**/

package proxy

import (
	"encoding/json"
	"flowsilicon/pkg/utils"
	"strings"
)

// 分析请求类型和估计token数量
func AnalyzeRequest(path string, bodyBytes []byte) (string, string, int) {
	// 默认值
	requestType := "completion"
	modelName := ""
	tokenEstimate := 0

	// 根据路径判断请求类型
	if strings.Contains(path, "/embeddings") {
		requestType = "embedding"
	} else if strings.Contains(path, "/chat/completions") {
		requestType = "completion"

		// 检查是否是流式请求
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if stream, ok := requestData["stream"].(bool); ok && stream {
				requestType = "streaming"
			}

			// 获取模型名称
			if model, ok := requestData["model"].(string); ok {
				modelName = model
			}

			// 估计token数量
			if messages, ok := requestData["messages"].([]interface{}); ok {
				// 基础token：每个消息对象约100个token
				tokenEstimate = len(messages) * 100

				// 更精确估计：计算消息内容的长度
				for _, msg := range messages {
					if msgObj, ok := msg.(map[string]interface{}); ok {
						if content, ok := msgObj["content"].(string); ok {
							// 使用更精确的token估算
							tokenEstimate += utils.EstimateStringTokens(content)
						}
					}
				}
			}
		}
	} else if strings.Contains(path, "/completions") {
		requestType = "completion"

		// 检查是否是流式请求
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if stream, ok := requestData["stream"].(bool); ok && stream {
				requestType = "streaming"
			}

			// 获取模型名称
			if model, ok := requestData["model"].(string); ok {
				modelName = model
			}

			// 估计token数量
			if prompt, ok := requestData["prompt"].(string); ok {
				// 使用更精确的token估算
				tokenEstimate = utils.EstimateStringTokens(prompt)
			}
		}
	}

	// 如果token估计值很大，将请求类型标记为大型请求
	if tokenEstimate > 5000 {
		requestType = "large_completion"
	}

	return requestType, modelName, tokenEstimate
}

// analyzeOpenAIRequest 分析OpenAI格式的请求类型和估计token数量
func AnalyzeOpenAIRequest(path string, bodyBytes []byte) (string, string, int) {
	// 默认值
	requestType := "completion"
	modelName := ""
	tokenEstimate := 0

	// 根据路径判断请求类型
	if strings.Contains(path, "/embeddings") {
		requestType = "embedding"
	} else if strings.Contains(path, "/chat/completions") {
		requestType = "completion"

		// 检查是否是流式请求
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if stream, ok := requestData["stream"].(bool); ok && stream {
				requestType = "streaming"
			}

			// 获取模型名称
			if model, ok := requestData["model"].(string); ok {
				modelName = model
			}

			// 估计token数量
			if messages, ok := requestData["messages"].([]interface{}); ok {
				// 基础token：每个消息对象约100个token
				tokenEstimate = len(messages) * 100

				// 更精确估计：计算消息内容的长度
				for _, msg := range messages {
					if msgObj, ok := msg.(map[string]interface{}); ok {
						if content, ok := msgObj["content"].(string); ok {
							// 使用更精确的token估算
							tokenEstimate += utils.EstimateStringTokens(content)
						}
					}
				}
			}
		}
	} else if strings.Contains(path, "/completions") {
		requestType = "completion"

		// 检查是否是流式请求
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if stream, ok := requestData["stream"].(bool); ok && stream {
				requestType = "streaming"
			}

			// 获取模型名称
			if model, ok := requestData["model"].(string); ok {
				modelName = model
			}

			// 估计token数量
			if prompt, ok := requestData["prompt"].(string); ok {
				// 使用更精确的token估算
				tokenEstimate = utils.EstimateStringTokens(prompt)
			}
		}
	}

	// 如果token估计值很大，将请求类型标记为大型请求
	if tokenEstimate > 5000 {
		requestType = "large_completion"
	}

	return requestType, modelName, tokenEstimate
}
