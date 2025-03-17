/**
  @author: Hanhai
  @since: 2025/3/16 20:43:43
  @desc:
**/

package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flowsilicon/internal/config"
	"flowsilicon/internal/key"
	"flowsilicon/internal/logger"
	"flowsilicon/pkg/utils"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// 处理 API 代理请求
func HandleApiProxy(c *gin.Context) {
	// 获取配置
	cfg := config.GetConfig()
	baseURL := cfg.ApiProxy.BaseURL

	// 获取请求路径
	path := c.Param("path")

	// 构建目标 URL
	targetURL := fmt.Sprintf("%s%s", baseURL, path)

	// 读取请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read request body: %v", err),
		})
		return
	}

	// 分析请求类型和估计token数量
	requestType, modelName, tokenEstimate := AnalyzeRequest(path, bodyBytes)

	// 根据请求类型选择最佳的API密钥
	apiKey, err := key.GetBestKeyForRequest(requestType, modelName, tokenEstimate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "No suitable API keys available",
		})
		return
	}

	// 创建新的请求
	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create request: %v", err),
		})
		return
	}

	// 复制原始请求的 headers
	for name, values := range c.Request.Header {
		// 跳过一些特定的 headers
		if strings.ToLower(name) == "host" || strings.ToLower(name) == "authorization" {
			continue
		}
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// 设置 Authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// 发送请求
	resp, err := client.Do(req)

	if err != nil {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to send request: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	// 记录请求信息
	maskedKey := utils.MaskKey(apiKey)
	logger.InfoWithKey(maskedKey, "API请求: %s %s", c.Request.Method, c.Request.URL.Path)

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read response body: %v", err),
		})
		return
	}

	// 检查响应状态码
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	// 更新密钥状态
	key.UpdateApiKeyStatus(apiKey, success)

	// 统计请求数据
	tokenCount := utils.EstimateTokenCount(bodyBytes, respBody)
	config.AddKeyRequestStat(apiKey, 1, tokenCount)

	// 更新每日统计数据
	// 尝试从请求中提取模型信息
	modelNameForStats := extractModelName(c.Request, respBody)
	// 提取令牌计数
	promptTokensCount, completionTokensCount := extractTokenCounts(respBody)
	if promptTokensCount == 0 && completionTokensCount == 0 {
		// 如果无法从响应中提取令牌计数，使用估算值
		promptTokensCount = tokenCount / 2
		completionTokensCount = tokenCount - promptTokensCount
	}
	// 添加到每日统计
	config.AddDailyRequestStat(apiKey, modelNameForStats, 1, promptTokensCount, completionTokensCount, success)

	// 复制响应 headers
	for name, values := range resp.Header {
		for _, value := range values {
			c.Header(name, value)
		}
	}

	// 设置响应状态码
	c.Status(resp.StatusCode)

	// 写入响应体
	c.Writer.Write(respBody)
}

// 处理 OpenAI 格式的 API 代理请求
func HandleOpenAIProxy(c *gin.Context) {
	// 获取配置
	cfg := config.GetConfig()
	baseURL := cfg.ApiProxy.BaseURL

	// 获取请求路径
	path := c.Param("path")

	// 构建目标 URL
	targetURL := fmt.Sprintf("%s/v1%s", baseURL, path)

	// 读取请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read request body: %v", err),
		})
		return
	}

	// 分析请求类型和估计token数量
	requestType, modelName, tokenEstimate := AnalyzeOpenAIRequest(path, bodyBytes)

	// 根据请求类型选择最佳的API密钥
	apiKey, err := key.GetBestKeyForRequest(requestType, modelName, tokenEstimate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "No suitable API keys available",
		})
		return
	}

	// TODO 处理
	// 如果是 /v1/models 请求，使用特殊处理
	if path == "/models" {
		HandleModelsRequest(c, apiKey)
		return
	}

	// 转换请求体为硅基流动格式
	transformedBody, err := TransformRequestBody(bodyBytes, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to transform request body: %v", err),
		})
		return
	}

	// 创建新的请求
	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(transformedBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create request: %v", err),
		})
		return
	}

	// 复制原始请求的 headers
	for name, values := range c.Request.Header {
		// 跳过一些特定的 headers
		if strings.ToLower(name) == "host" || strings.ToLower(name) == "authorization" {
			continue
		}
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}

	// 设置 Authorization header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	// 设置 Content-Type header
	req.Header.Set("Content-Type", "application/json")

	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// 检查是否是流式请求
	isStreamRequest := false
	if strings.Contains(path, "/chat/completions") || strings.Contains(path, "/completions") {
		// 检查请求体中的stream参数
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if stream, ok := requestData["stream"].(bool); ok && stream {
				isStreamRequest = true
			}
		}
	}

	// 如果是流式请求，使用特殊处理
	if isStreamRequest {
		// 发送请求
		resp, err := client.Do(req)
		if err != nil {
			// 更新密钥失败记录
			key.UpdateApiKeyStatus(apiKey, false)

			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to send request: %v", err),
			})
			return
		}

		// 处理流式响应
		HandleStreamResponse(c, resp.Body, apiKey, bodyBytes)
		return
	}

	// 发送请求
	resp, err := client.Do(req)

	if err != nil {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to send request: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	// 记录请求信息
	maskedKey := utils.MaskKey(apiKey)
	logger.InfoWithKey(maskedKey, "请求状态码: %d", resp.StatusCode)

	// 检查响应状态码
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	// 更新密钥状态
	key.UpdateApiKeyStatus(apiKey, success)

	// 非流式请求的处理
	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read response body: %v", err),
		})
		return
	}

	// 统计请求数据
	tokenCount := utils.EstimateTokenCount(transformedBody, respBody)
	config.AddKeyRequestStat(apiKey, 1, tokenCount)

	// 更新每日统计数据
	// 尝试从请求中提取模型信息
	modelNameForStats := extractModelName(c.Request, respBody)
	// 提取令牌计数
	promptTokensCount, completionTokensCount := extractTokenCounts(respBody)
	if promptTokensCount == 0 && completionTokensCount == 0 {
		// 如果无法从响应中提取令牌计数，使用估算值
		promptTokensCount = tokenCount / 2
		completionTokensCount = tokenCount - promptTokensCount
	}
	// 添加到每日统计
	config.AddDailyRequestStat(apiKey, modelNameForStats, 1, promptTokensCount, completionTokensCount, success)

	// 记录原始响应体，便于调试
	if len(respBody) > 0 {
		logger.Info("原始响应体: %s", string(respBody))
	}

	// 转换响应体，处理差异
	transformedResp, err := TransformResponseBody(respBody)
	if err != nil {
		// 如果转换失败，使用原始响应
		logger.Error("转换响应体失败: %v", err)
		transformedResp = respBody
	}

	// 复制响应 headers
	for name, values := range resp.Header {
		for _, value := range values {
			c.Header(name, value)
		}
	}

	// 设置响应状态码
	c.Status(resp.StatusCode)

	// 写入响应体
	c.Writer.Write(transformedResp)
}

// handleModelsRequest 处理获取模型列表的请求
func HandleModelsRequest(c *gin.Context, apiKey string) {
	logger.Info("处理模型列表请求")

	// 获取配置
	cfg := config.GetConfig()
	baseURL := cfg.ApiProxy.BaseURL
	targetURL := fmt.Sprintf("%s/v1/models", baseURL)

	logger.Info("获取模型列表,目标URL: %s", targetURL)

	// 创建请求
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		logger.Error("创建请求失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("创建请求失败: %v", err),
		})
		return
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	// 创建HTTP客户端
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// 发送请求
	logger.Info("正在发送模型列表请求...")
	resp, err := client.Do(req)
	if err != nil {
		logger.Error("发送请求失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("发送请求失败: %v", err),
		})
		return
	}
	defer resp.Body.Close()

	logger.Info("模型列表请求状态码: %d", resp.StatusCode)

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("读取响应体失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("读取响应体失败: %v", err),
		})
		return
	}

	// 如果API返回错误，直接将错误传递给客户端
	if resp.StatusCode != http.StatusOK {
		logger.Error("API返回错误，状态码: %d, 响应: %s", resp.StatusCode, string(respBody))
		c.Status(resp.StatusCode)
		c.Writer.Write(respBody)
		return
	}

	// 设置响应头
	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}

	// 返回API的原始响应
	c.Status(resp.StatusCode)
	c.Writer.Write(respBody)

	logger.Info("成功返回模型列表")
}

// handleStreamResponse 处理流式响应
func HandleStreamResponse(c *gin.Context, responseBody io.ReadCloser, apiKey string, requestBody []byte) {
	logger.Info("开始处理流式响应")

	// 创建缓冲读取器
	reader := bufio.NewReader(responseBody)

	// 创建刷新写入器，确保数据立即发送
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logger.Error("流式处理失败：响应写入器不支持刷新")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Streaming not supported",
		})
		return
	}

	// 初始化计数器
	var totalTokens int
	var eventCount int

	// 初始化自适应延迟参数
	baseDelay := 10 * time.Millisecond
	maxDelay := 50 * time.Millisecond
	currentDelay := baseDelay

	// 创建缓冲区用于批量发送
	var buffer bytes.Buffer
	bufferThreshold := 1024 // 1KB

	// 读取并处理每一行SSE事件
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				logger.Error("读取流式响应时出错: %v", err)
			}
			break
		}

		// 跳过空行
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		// 处理SSE事件行
		if bytes.HasPrefix(line, []byte("data: ")) {
			eventCount++

			// 解析事件数据
			data := bytes.TrimPrefix(line, []byte("data: "))

			// 检查是否是[DONE]事件
			if bytes.Equal(bytes.TrimSpace(data), []byte("[DONE]")) {
				// 发送[DONE]事件
				buffer.WriteString("data: [DONE]\n\n")
				c.Writer.Write(buffer.Bytes())
				buffer.Reset()
				flusher.Flush()
				break
			}

			// 转换事件数据，确保与OpenAI API格式兼容
			transformedData, err := TransformStreamEvent(bytes.TrimSpace(data))
			if err != nil {
				logger.Error("转换流式事件失败: %v", err)
				// 使用原始数据
				transformedData = bytes.TrimSpace(data)
			}

			// 尝试解析JSON以估算token数量
			var jsonData map[string]interface{}
			if err := json.Unmarshal(transformedData, &jsonData); err == nil {
				// 估算token数量
				if choices, ok := jsonData["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							if content, ok := delta["content"].(string); ok {
								// 简单估算：每个字符约为0.25个token
								tokenEstimate := int(float64(len(content)) * 0.25)
								totalTokens += tokenEstimate

								// 根据内容大小动态调整延迟
								contentSize := len(content)
								if contentSize > 20 {
									// 内容较大，减少延迟
									currentDelay = baseDelay
								} else if contentSize < 5 {
									// 内容较小，增加延迟
									currentDelay = maxDelay
								} else {
									// 内容适中，使用中等延迟
									currentDelay = (baseDelay + maxDelay) / 2
								}
							}
						}
					}
				}
			}

			// 将事件添加到缓冲区
			buffer.WriteString("data: ")
			buffer.Write(transformedData)
			buffer.WriteString("\n\n")

			// 如果缓冲区超过阈值或者是第一个事件，立即发送
			if buffer.Len() >= bufferThreshold || eventCount <= 2 {
				c.Writer.Write(buffer.Bytes())
				buffer.Reset()
				flusher.Flush()
			}

			// 应用自适应延迟
			time.Sleep(currentDelay)
		} else {
			// 处理其他SSE事件类型（如event:, id:等）
			buffer.Write(line)
		}
	}

	// 发送剩余的缓冲数据
	if buffer.Len() > 0 {
		c.Writer.Write(buffer.Bytes())
		flusher.Flush()
	}

	// 统计请求数据
	config.AddKeyRequestStat(apiKey, 1, totalTokens)

	// 更新每日统计数据
	// 对于流式响应，我们无法准确获取模型名称和令牌计数，使用估计值
	modelNameForStats := "unknown"
	promptTokensCount := totalTokens / 3                     // 估计输入占1/3
	completionTokensCount := totalTokens - promptTokensCount // 估计输出占2/3
	config.AddDailyRequestStat(apiKey, modelNameForStats, 1, promptTokensCount, completionTokensCount, true)

	logger.Info("流式响应完成，估计token数: %d", totalTokens)
}

// extractModelName 从请求和响应中提取模型名称
func extractModelName(req *http.Request, respBody []byte) string {
	// 尝试从请求路径中提取模型名称
	if strings.Contains(req.URL.Path, "/v1/chat/completions") ||
		strings.Contains(req.URL.Path, "/v1/completions") {
		// 尝试从响应体中提取模型名称
		var respData map[string]interface{}
		if err := json.Unmarshal(respBody, &respData); err == nil {
			if model, ok := respData["model"].(string); ok && model != "" {
				return model
			}
		}
	}

	// 如果无法从响应中提取，尝试从请求体中提取
	if req.Body != nil {
		// 由于请求体已经被读取，无法再次读取，这里只能返回默认值
		return "unknown"
	}

	return "unknown"
}

// extractTokenCounts 从响应中提取令牌计数
func extractTokenCounts(respBody []byte) (int, int) {
	// 尝试从响应体中提取令牌计数
	var respData map[string]interface{}
	if err := json.Unmarshal(respBody, &respData); err == nil {
		if usage, ok := respData["usage"].(map[string]interface{}); ok {
			promptTokens := 0
			completionTokens := 0

			if pt, ok := usage["prompt_tokens"].(float64); ok {
				promptTokens = int(pt)
			}

			if ct, ok := usage["completion_tokens"].(float64); ok {
				completionTokens = int(ct)
			}

			return promptTokens, completionTokens
		}
	}

	return 0, 0
}
