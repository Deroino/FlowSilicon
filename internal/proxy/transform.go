/**
  @author: Hanhai
  @since: 2025/3/16 21:36:12
  @desc:
**/

package proxy

import (
	"bytes"
	"encoding/json"
	"flowsilicon/internal/logger"
	"flowsilicon/pkg/utils"
	"fmt"
	"strings"
	"time"
)

// TransformRequestBody 转换请求体，处理OpenAI和硅基流动API之间的差异
func TransformRequestBody(body []byte, path string) ([]byte, error) {
	// 如果请求体为空，直接返回
	if len(body) == 0 {
		return body, nil
	}

	// 解析JSON
	var requestData map[string]interface{}
	if err := json.Unmarshal(body, &requestData); err != nil {
		return nil, err
	}

	// 处理chat/completions请求
	if strings.Contains(path, "/chat/completions") {
		// 检查是否有model字段
		if model, ok := requestData["model"].(string); ok {
			// 针对Deepseek R1模型的特殊处理
			if strings.Contains(strings.ToLower(model), "deepseek") && strings.Contains(model, "r1") {
				// 检查并设置合适的max_tokens值
				if maxTokens, exists := requestData["max_tokens"]; !exists {
					// 如果未设置max_tokens，为Deepseek R1设置默认值16000
					requestData["max_tokens"] = 16000
					logger.Info("为Deepseek R1自动设置max_tokens=16000")
				} else if maxTokenValue, ok := maxTokens.(float64); ok && maxTokenValue < 1000 {
					// 如果设置了但值太小，调整到更合理的值
					requestData["max_tokens"] = 16000
					logger.Info("Deepseek R1检测到过小的max_tokens值(%v)，自动调整为16000", maxTokenValue)
				}

				// 确保流式输出
				if stream, exists := requestData["stream"]; !exists || stream != true {
					requestData["stream"] = true
					logger.Info("为Deepseek R1强制启用流式输出(stream=true)")
				}

				// 添加足够的超时时间
				requestData["timeout"] = 3600 // 60分钟
				logger.Info("为Deepseek R1设置API超时时间为60分钟")
			}
		} else {
			// 如果没有提供模型，使用默认模型
			requestData["model"] = "GLM-4"
		}
	}

	// 处理completions请求
	if strings.Contains(path, "/completions") && !strings.Contains(path, "/chat/completions") {
		// 检查是否有model字段
		if model, ok := requestData["model"].(string); ok {
			// 针对Deepseek R1模型的特殊处理
			if strings.Contains(strings.ToLower(model), "deepseek") && strings.Contains(model, "r1") {
				// 检查并设置合适的max_tokens值
				if maxTokens, exists := requestData["max_tokens"]; !exists {
					// 如果未设置max_tokens，为Deepseek R1设置默认值16000
					requestData["max_tokens"] = 16000
					logger.Info("为Deepseek R1自动设置max_tokens=16000")
				} else if maxTokenValue, ok := maxTokens.(float64); ok && maxTokenValue < 1000 {
					// 如果设置了但值太小，调整到更合理的值
					requestData["max_tokens"] = 16000
					logger.Info("Deepseek R1检测到过小的max_tokens值(%v)，自动调整为16000", maxTokenValue)
				}

				// 确保流式输出
				if stream, exists := requestData["stream"]; !exists || stream != true {
					requestData["stream"] = true
					logger.Info("为Deepseek R1强制启用流式输出(stream=true)")
				}

				// 添加足够的超时时间
				requestData["timeout"] = 3600 // 60分钟
				logger.Info("为Deepseek R1设置API超时时间为60分钟")
			}
		} else {
			// 如果没有提供模型，使用默认模型
			requestData["model"] = "GLM-4"
		}
	}

	// 处理重排序请求
	if strings.Contains(path, "/rerank") {
		// 记录日志，便于调试
		logger.Info("处理重排序请求: %s", path)

		// 检查是否有model字段
		if _, ok := requestData["model"].(string); !ok {
			// 如果没有提供模型，使用默认模型
			requestData["model"] = "BAAI/bge-reranker-v2-m3"
			logger.Info("未提供model字段，使用默认模型: BAAI/bge-reranker-v2-m3")
		} else {
			logger.Info("使用提供的模型: %s", requestData["model"])
		}

		// 检查必要字段
		if _, ok := requestData["query"]; !ok {
			logger.Error("请求中缺少query字段")
			return nil, fmt.Errorf("请求中缺少query字段")
		}

		if _, ok := requestData["documents"]; !ok {
			logger.Error("请求中缺少documents字段")
			return nil, fmt.Errorf("请求中缺少documents字段")
		}

		// 设置默认值
		if _, ok := requestData["top_n"]; !ok {
			requestData["top_n"] = 10
		}

		if _, ok := requestData["return_documents"]; !ok {
			requestData["return_documents"] = true
		}

		// 记录转换后的请求体，便于调试
		jsonData, _ := json.Marshal(requestData)
		logger.Info("转换后的重排序请求体: %s", string(jsonData))

		return json.Marshal(requestData)
	}

	// 处理图片生成请求
	if strings.Contains(path, "/images/generations") {
		// 记录日志，便于调试
		logger.Info("处理图片生成请求: %s", path)

		// 检查是否有model字段
		if _, ok := requestData["model"].(string); !ok {
			// 如果没有提供模型，使用默认模型
			requestData["model"] = "stabilityai/stable-diffusion-xl-base-1.0"
			logger.Info("未提供model字段，使用默认模型: stabilityai/stable-diffusion-xl-base-1.0")
		} else {
			logger.Info("使用提供的模型: %s", requestData["model"])
		}

		// 检查必要字段
		if _, ok := requestData["prompt"]; !ok {
			logger.Error("请求中缺少prompt字段")
			return nil, fmt.Errorf("请求中缺少prompt字段")
		}

		if _, ok := requestData["n"]; !ok {
			requestData["n"] = 1
		}

		if _, ok := requestData["size"]; !ok {
			requestData["size"] = "1024x1024"
		}

		if _, ok := requestData["guidance_scale"]; !ok {
			requestData["guidance_scale"] = 7.5
		}

		// 删除stream字段，图片生成API不支持流式响应
		if _, hasStream := requestData["stream"]; hasStream {
			delete(requestData, "stream")
			logger.Info("删除stream字段，图片生成API不支持流式响应")
		}

		// 记录转换后的请求体，便于调试
		jsonData, _ := json.Marshal(requestData)
		logger.Info("转换后的图片生成请求体: %s", string(jsonData))

		return json.Marshal(requestData)
	}

	// 处理embeddings请求
	if strings.Contains(path, "/embeddings") {
		// 记录日志，便于调试
		logger.Info("处理embeddings请求: %s", path)

		// 检查是否有model字段
		if _, ok := requestData["model"].(string); !ok {
			// 如果没有提供模型，使用默认模型
			requestData["model"] = "BAAI/bge-m3"
			logger.Info("未提供model字段，使用默认模型: BAAI/bge-m3")
		} else {
			logger.Info("使用提供的模型: %s", requestData["model"])
		}

		// 检查input字段格式
		if input, ok := requestData["input"]; ok {
			// 如果input是字符串，转换为字符串数组
			if inputStr, isString := input.(string); isString {
				requestData["input"] = []string{inputStr}
				logger.Info("将input字符串转换为数组: [%s]", inputStr)
			} else if inputArray, isArray := input.([]interface{}); isArray {
				logger.Info("input是数组，长度: %d", len(inputArray))
			} else {
				logger.Info("input字段类型: %T", input)
			}
		} else {
			logger.Error("请求中缺少input字段")
			return nil, fmt.Errorf("请求中缺少input字段")
		}

		// 硅基流动API需要的格式
		// 创建新的请求体，符合硅基流动API的要求
		newRequestData := map[string]interface{}{
			"model": requestData["model"],
			"input": requestData["input"],
		}

		// 记录转换后的请求体，便于调试
		jsonData, _ := json.Marshal(newRequestData)
		logger.Info("转换后的embeddings请求体: %s", string(jsonData))

		return json.Marshal(newRequestData)
	}

	// 不再打印转换后的请求体

	// 重新序列化为JSON
	return json.Marshal(requestData)
}

// TransformResponseBody 转换响应体，处理硅基流动API和OpenAI之间的差异
func TransformResponseBody(body []byte, path string) ([]byte, error) {
	// 如果响应体为空，直接返回
	if len(body) == 0 {
		return body, nil
	}

	// 不再打印原始响应体

	// 尝试解析JSON
	var responseData map[string]interface{}
	if err := json.Unmarshal(body, &responseData); err != nil {
		return body, nil // 如果不是有效的JSON，返回原始响应
	}

	// 检查是否是标准的Chat Completion或Completion响应格式
	if choices, hasChoices := responseData["choices"].([]interface{}); hasChoices && len(choices) > 0 {
		if model, hasModel := responseData["model"].(string); hasModel {
			// 记录已识别的模型和响应格式
			logger.Info("识别到标准响应格式: 模型=%s, 类型=%s", model, responseData["object"])

			// 检查是否是DeepSeek模型
			if strings.Contains(strings.ToLower(model), "deepseek") {
				logger.Info("识别到DeepSeek模型响应: %s", model)
			}

			// 已经是标准格式，不需要转换
			return body, nil
		}
	}

	// 检查是否有硅基流动格式的错误响应 (code, message 字段)
	if code, hasCode := responseData["code"]; hasCode {
		message := ""
		if msg, hasMsg := responseData["message"].(string); hasMsg {
			message = msg
		}

		// 转换为OpenAI格式的错误响应
		openAIError := map[string]interface{}{
			"error": map[string]interface{}{
				"message": message,
				"type":    "invalid_request_error",
				"code":    code,
			},
		}

		// 不再记录转换错误响应的详细信息
		return json.Marshal(openAIError)
	}

	// 处理重排序响应
	if results, hasResults := responseData["results"]; hasResults {
		logger.Info("检测到results字段，处理重排序响应")

		// 检查results是否为数组
		if resultsArray, isArray := results.([]interface{}); isArray {
			logger.Info("results字段是数组，长度: %d", len(resultsArray))

			// 检查响应格式是否已经符合要求
			if len(resultsArray) > 0 {
				if resultObj, isMap := resultsArray[0].(map[string]interface{}); isMap {
					if _, hasIndex := resultObj["index"]; hasIndex {
						if _, hasScore := resultObj["relevance_score"]; hasScore {
							logger.Info("响应已经是正确的重排序格式")
							return body, nil
						}
					}
				}
			}

			// 如果需要转换，可以在这里添加转换逻辑
		}
	}

	// 处理图片生成响应
	if images, hasImages := responseData["images"]; hasImages {
		logger.Info("检测到images字段，处理图片生成响应")

		// 检查images是否为数组
		if imagesArray, isArray := images.([]interface{}); isArray {
			logger.Info("images字段是数组，长度: %d", len(imagesArray))

			// 检查响应格式是否已经符合要求
			if len(imagesArray) > 0 {
				if imageObj, isMap := imagesArray[0].(map[string]interface{}); isMap {
					if _, hasUrl := imageObj["url"]; hasUrl {
						logger.Info("响应已经是正确的图片生成格式")
						return body, nil
					}
				}
			}

			// 转换为标准格式
			standardImages := make([]map[string]interface{}, 0, len(imagesArray))
			for _, img := range imagesArray {
				if imgStr, isString := img.(string); isString {
					// 如果是字符串URL，转换为对象
					standardImages = append(standardImages, map[string]interface{}{
						"url": imgStr,
					})
				} else if imgObj, isMap := img.(map[string]interface{}); isMap {
					// 如果已经是对象，但没有url字段
					if _, hasUrl := imgObj["url"]; hasUrl {
						standardImages = append(standardImages, imgObj)
					} else if imageValue, hasImage := imgObj["image"]; hasImage {
						// 如果有image字段而不是url字段
						imgObj["url"] = imageValue
						delete(imgObj, "image")
						standardImages = append(standardImages, imgObj)
					}
				}
			}

			// 创建标准响应
			standardResponse := map[string]interface{}{
				"images": standardImages,
			}

			// 添加其他字段
			if timings, hasTimings := responseData["timings"]; hasTimings {
				standardResponse["timings"] = timings
			} else {
				standardResponse["timings"] = map[string]interface{}{
					"inference": 0,
				}
			}

			if seed, hasSeed := responseData["seed"]; hasSeed {
				standardResponse["seed"] = seed
			}

			jsonResp, err := json.Marshal(standardResponse)
			if err != nil {
				logger.Error("序列化标准图片生成响应失败: %v", err)
				return body, nil
			}

			logger.Info("转换为标准图片生成响应格式")
			return jsonResp, nil
		}
	}

	// 处理embeddings响应
	if data, hasData := responseData["data"]; hasData {
		logger.Info("检测到data字段，尝试处理embeddings响应")

		// 记录data字段类型
		logger.Info("data字段类型: %T", data)

		if dataMap, isMap := data.(map[string]interface{}); isMap {
			// 记录dataMap中的所有键
			keys := utils.GetMapKeys(dataMap)
			logger.Info("data字段是对象，包含的字段: %v", keys)

			// 检查是否是embeddings响应
			if embedding, hasEmbedding := dataMap["embedding"]; hasEmbedding {
				logger.Info("检测到embedding字段，处理embeddings响应")
				logger.Info("embedding字段类型: %T", embedding)

				// 创建OpenAI格式的embeddings响应
				openAIResponse := map[string]interface{}{
					"object": "list",
					"data": []map[string]interface{}{
						{
							"object":    "embedding",
							"embedding": embedding,
							"index":     0,
						},
					},
					"model": "embedding-2",
					"usage": map[string]interface{}{
						"prompt_tokens": 0,
						"total_tokens":  0,
					},
				}

				jsonResp, err := json.Marshal(openAIResponse)
				if err != nil {
					logger.Error("序列化OpenAI格式响应失败: %v", err)
					return body, nil
				}

				logger.Info("转换为OpenAI格式的embeddings响应: %s", string(jsonResp))
				return jsonResp, nil
			}
		} else if dataArray, isArray := data.([]interface{}); isArray {
			logger.Info("data字段是数组，长度: %d", len(dataArray))

			// 检查数组中是否包含embedding
			if len(dataArray) > 0 {
				if firstItem, isMap := dataArray[0].(map[string]interface{}); isMap {
					keys := utils.GetMapKeys(firstItem)
					logger.Info("data[0]是对象，包含的字段: %v", keys)

					if _, hasEmbedding := firstItem["embedding"]; hasEmbedding {
						logger.Info("data[0]中包含embedding字段，已经是OpenAI格式")
						return body, nil
					}
				}
			}
		}
	}

	// 处理直接返回的embedding数组
	if embedding, hasEmbedding := responseData["embedding"]; hasEmbedding {
		logger.Info("检测到直接返回的embedding字段，处理embeddings响应")
		// 记录embedding类型，便于调试
		logger.Info("embedding字段类型: %T", embedding)

		// 检查embedding是否为数组
		_, isArray := embedding.([]interface{})
		_, isFloat64Array := embedding.([]float64)

		if !isArray && !isFloat64Array {
			logger.Error("embedding字段不是数组类型")
		}

		// 创建OpenAI格式的embeddings响应
		openAIResponse := map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"embedding": embedding,
					"index":     0,
				},
			},
			"model": "embedding-2",
			"usage": map[string]interface{}{
				"prompt_tokens": 0,
				"total_tokens":  0,
			},
		}
		jsonResp, err := json.Marshal(openAIResponse)
		if err != nil {
			logger.Error("序列化OpenAI格式响应失败: %v", err)
			return body, nil
		}

		logger.Info("转换为OpenAI格式的embeddings响应: %s", string(jsonResp))
		return jsonResp, nil
	}

	// 检查是否是硅基流动的嵌入响应格式
	if result, hasResult := responseData["result"]; hasResult {
		logger.Info("检测到result字段，可能是硅基流动的嵌入响应格式")

		if resultMap, isMap := result.(map[string]interface{}); isMap {
			keys := utils.GetMapKeys(resultMap)
			logger.Info("result字段是对象，包含的字段: %v", keys)

			if embedding, hasEmbedding := resultMap["embedding"]; hasEmbedding {
				logger.Info("result中包含embedding字段，类型: %T", embedding)

				// 创建OpenAI格式的embeddings响应
				openAIResponse := map[string]interface{}{
					"object": "list",
					"data": []map[string]interface{}{
						{
							"object":    "embedding",
							"embedding": embedding,
							"index":     0,
						},
					},
					"model": "embedding-2",
					"usage": map[string]interface{}{
						"prompt_tokens": 0,
						"total_tokens":  0,
					},
				}
				jsonResp, err := json.Marshal(openAIResponse)
				if err != nil {
					logger.Error("序列化OpenAI格式响应失败: %v", err)
					return body, nil
				}

				logger.Info("转换为OpenAI格式的embeddings响应: %s", string(jsonResp))
				return jsonResp, nil
			}
		}
	}

	// 记录未能识别的响应格式
	jsonBody, _ := json.Marshal(responseData)
	logger.Info("未能识别的响应格式: %s", string(jsonBody))

	return body, nil
}

// TransformStreamEvent 转换流式响应事件，确保与OpenAI API格式兼容
func TransformStreamEvent(data []byte) ([]byte, error) {
	// 如果数据为空或者是[DONE]事件，直接返回
	if len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
		return data, nil
	}

	// 尝试解析JSON
	var eventData map[string]interface{}
	if err := json.Unmarshal(data, &eventData); err != nil {
		// 如果解析失败，返回原始数据
		return data, nil
	}

	// 检查是否需要转换
	if _, hasChoices := eventData["choices"]; hasChoices {
		// 检查是否是Deepseek R1模型的回复
		if model, hasModel := eventData["model"].(string); hasModel {
			if strings.Contains(strings.ToLower(model), "deepseek") && strings.Contains(model, "r1") {
				logger.Info("检测到Deepseek R1流式响应")

				// 确保choices是数组
				choices, ok := eventData["choices"].([]interface{})
				if !ok || len(choices) == 0 {
					return data, nil
				}

				// 获取首个choice
				choice, ok := choices[0].(map[string]interface{})
				if !ok {
					return data, nil
				}

				// 确保delta存在
				if delta, hasDelta := choice["delta"].(map[string]interface{}); hasDelta {
					// 确保content存在，即使是空字符串
					if _, hasContent := delta["content"]; !hasContent {
						delta["content"] = ""
						choice["delta"] = delta
						choices[0] = choice
						eventData["choices"] = choices

						// 重新编码修改后的事件
						modifiedData, err := json.Marshal(eventData)
						if err != nil {
							return data, nil
						}
						return modifiedData, nil
					}
				}

				// 将finish_reason为null或缺少的情况处理为明确的null
				if _, hasFinishReason := choice["finish_reason"]; !hasFinishReason {
					choice["finish_reason"] = nil
					choices[0] = choice
					eventData["choices"] = choices

					// 重新编码确保finish_reason存在
					modifiedData, err := json.Marshal(eventData)
					if err != nil {
						return data, nil
					}
					return modifiedData, nil
				}
			}
		}

		// 已经是OpenAI格式，不需要转换
		return data, nil
	}

	// 构建OpenAI格式的响应
	openAIEvent := map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-%s", time.Now().Format("20060102150405")),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   "GLM-4", // 默认模型
		"choices": []map[string]interface{}{
			{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": nil,
			},
		},
	}

	// 从原始事件中提取内容
	if content, hasContent := eventData["content"].(string); hasContent {
		openAIEvent["choices"].([]map[string]interface{})[0]["delta"].(map[string]interface{})["content"] = content
	} else if text, hasText := eventData["text"].(string); hasText {
		openAIEvent["choices"].([]map[string]interface{})[0]["delta"].(map[string]interface{})["content"] = text
	}

	// 检查是否有finish_reason
	if reason, hasReason := eventData["finish_reason"].(string); hasReason && reason != "" {
		openAIEvent["choices"].([]map[string]interface{})[0]["finish_reason"] = reason
	}

	// 转换为JSON
	result, err := json.Marshal(openAIEvent)
	if err != nil {
		return data, nil // 如果转换失败，返回原始数据
	}

	return result, nil
}
