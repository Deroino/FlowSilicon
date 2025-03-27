/**
  @author: Hanhai
  @since: 2025/3/16 20:43:30
  @desc:
**/

package proxy

import (
	"encoding/json"
	"flowsilicon/internal/logger"
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
				logger.Info("提取到聊天模型名称: %s", modelName)
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
				logger.Info("提取到补全模型名称: %s", modelName)
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

	logger.Info("请求分析结果: 类型=%s, 模型=%s, 估计token=%d", requestType, modelName, tokenEstimate)
	return requestType, modelName, tokenEstimate
}

// AnalyzeOpenAIRequest 分析 OpenAI 格式的请求，确定请求类型和估计token数量
func AnalyzeOpenAIRequest(path string, bodyBytes []byte) (string, string, int) {
	// 默认请求类型、模型名称和token估计值
	requestType := "unknown"
	modelName := "unknown"
	tokenEstimate := 0

	// 预处理路径，确保能正确识别无版本号路径
	if strings.HasPrefix(path, "/chat") && !strings.Contains(path, "/completions") {
		// 将/chat路径视为/chat/completions
		path = "/chat/completions"
		logger.Info("分析请求时将/chat路径视为/chat/completions")
	}

	// 根据路径确定请求类型
	if strings.Contains(path, "/chat/completions") {
		requestType = "chat"

		// 检查是否是流式请求
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if stream, ok := requestData["stream"].(bool); ok && stream {
				requestType = "streaming"
			}

			// 获取模型名称
			if model, ok := requestData["model"].(string); ok {
				modelName = model
				logger.Info("提取到聊天模型名称: %s", modelName)
			}

			// 估计token数量
			if messages, ok := requestData["messages"].([]interface{}); ok {
				// 便于调试，记录消息数量
				logger.Info("消息数组长度: %d", len(messages))

				// 估计所有消息的token数量
				for _, msg := range messages {
					if msgMap, ok := msg.(map[string]interface{}); ok {
						if content, ok := msgMap["content"].(string); ok {
							// 使用更精确的token估算
							tokenEstimate += utils.EstimateStringTokens(content)
						}
					}
				}
			}
		}
	} else if strings.Contains(path, "/completions") || path == "/completions" {
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
				logger.Info("提取到补全模型名称: %s", modelName)
			}

			// 估计token数量
			if prompt, ok := requestData["prompt"].(string); ok {
				// 使用更精确的token估算
				tokenEstimate = utils.EstimateStringTokens(prompt)
			}
		}
	} else if strings.Contains(path, "/embeddings") || path == "/embeddings" {
		requestType = "embeddings"

		// 解析请求体中的模型信息
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if model, ok := requestData["model"].(string); ok {
				modelName = model
			}

			// 估计embedding请求的token数量
			if input, ok := requestData["input"].(string); ok {
				tokenEstimate = utils.EstimateStringTokens(input)
			} else if inputArray, ok := requestData["input"].([]interface{}); ok {
				// 如果input是数组，估计所有元素的总token数
				for _, item := range inputArray {
					if str, ok := item.(string); ok {
						tokenEstimate += utils.EstimateStringTokens(str)
					}
				}
			}
		}
	} else if strings.Contains(path, "/rerank") || path == "/rerank" {
		requestType = "rerank"

		// 解析请求体获取模型名称
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if model, ok := requestData["model"].(string); ok {
				modelName = model
			}

			// 估计重排序请求的token数量
			if query, ok := requestData["query"].(string); ok {
				tokenEstimate += utils.EstimateStringTokens(query)
			}

			if documents, ok := requestData["documents"].([]interface{}); ok {
				for _, doc := range documents {
					if str, ok := doc.(string); ok {
						tokenEstimate += utils.EstimateStringTokens(str)
					}
				}
			}
		}
	} else if strings.Contains(path, "/images") || strings.HasPrefix(path, "/images") {
		requestType = "images"
		// 图像请求没有明确的token计数，但我们可以设置一个默认值
		tokenEstimate = 1000

		// 解析请求体获取模型名称
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if model, ok := requestData["model"].(string); ok {
				modelName = model
			}
		}
	} else if strings.Contains(path, "/audio") || strings.HasPrefix(path, "/audio") {
		requestType = "audio"
		// 音频请求没有明确的token计数，设置合理的默认值
		tokenEstimate = 5000

		// 解析请求体获取模型名称
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if model, ok := requestData["model"].(string); ok {
				modelName = model
			}
		}
	} else if strings.Contains(path, "/models") || path == "/models" {
		requestType = "models"
		// 模型列表请求几乎不消耗token
		tokenEstimate = 100
	} else if strings.Contains(path, "/user/info") || path == "/user/info" {
		requestType = "user_info"
		// 用户信息请求几乎不消耗token
		tokenEstimate = 10
	}

	// 如果token估计值很大，将请求类型标记为大型请求
	if tokenEstimate > 5000 {
		requestType = "large_completion"
	}

	logger.Info("请求分析结果: 类型=%s, 模型=%s, 估计token=%d, 路径=%s", requestType, modelName, tokenEstimate, path)
	return requestType, modelName, tokenEstimate
}
