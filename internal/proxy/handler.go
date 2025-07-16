/**
  @author: Hanhai
  @desc: API代理处理模块，负责转发和处理各类API请求，包含重试逻辑和流式响应处理
**/

package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flowsilicon/internal/config"
	"flowsilicon/internal/key"
	"flowsilicon/internal/logger"
	"flowsilicon/internal/model"
	"flowsilicon/pkg/utils"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// 在成功处理请求后更新调用次数
func updateModelCallCount(modelName string) {
	if modelName == "" {
		return
	}

	// 检查模型名称格式
	if strings.Contains(modelName, "/") || strings.Contains(modelName, "-") {
		// 更新模型调用次数
		err := model.UpdateModelCallCount(modelName)
		if err != nil {
			// 如果是数据库相关错误才记录警告
			if strings.Contains(err.Error(), "数据库") {
				logger.Warn("更新模型 %s 调用次数失败: %v", modelName, err)
			} else {
				// 其他错误使用Info级别记录，避免警告日志过多
				logger.Info("更新模型 %s 调用次数跳过: %v", modelName, err)
			}
		} else {
			logger.Info("更新模型 %s 调用次数成功", modelName)
		}
	} else {
		// 模型名称格式不符合预期，使用Info记录
		logger.Info("模型名称格式不符合预期，跳过更新调用次数: %s", modelName)
	}
}

// 处理 API 代理请求
func HandleApiProxy(c *gin.Context) {
	// 检查是否有直接从以前的流式响应中设置的标志
	if streamCompleted, exists := c.Get("stream_completed"); exists && streamCompleted.(bool) {
		logger.Info("检测到从流式响应完成后的后续请求，直接返回OK")
		c.Status(http.StatusOK)
		return
	}

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

	// 检查模型是否被禁用
	if modelName != "" && isModelDisabled(modelName) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("模型 %s 已被禁用", modelName),
				"type":    "invalid_request_error",
				"code":    403,
			},
		})
		return
	}

	// 调用处理请求的函数，包含重试逻辑
	success := handleApiProxyWithRetry(c, targetURL, bodyBytes, requestType, modelName, tokenEstimate)

	// 如果请求成功且有模型名称，更新模型调用次数
	if success && modelName != "" {
		go updateModelCallCount(modelName)
	}
}

// isModelDisabled 检查模型是否被禁用
func isModelDisabled(modelName string) bool {
	cfg := config.GetConfig()
	if cfg == nil || cfg.App.DisabledModels == nil {
		return false
	}

	for _, disabledModel := range cfg.App.DisabledModels {
		if disabledModel == modelName {
			return true
		}
	}
	return false
}

// 添加带重试逻辑的API代理处理函数
func handleApiProxyWithRetry(c *gin.Context, targetURL string, bodyBytes []byte, requestType string, modelName string, tokenEstimate int) bool {
	// 获取配置
	cfg := config.GetConfig()
	retryConfig := cfg.ApiProxy.Retry

	// 如果最大重试次数为0，直接处理一次请求
	if retryConfig.MaxRetries <= 0 {
		success, _ := processApiRequest(c, targetURL, bodyBytes, requestType, modelName, tokenEstimate)
		return success
	}

	// 第一次尝试
	firstTry, err := processApiRequest(c, targetURL, bodyBytes, requestType, modelName, tokenEstimate)
	if firstTry {
		// 请求成功，直接返回
		return true
	}

	// 检查是否需要重试
	if !shouldRetry(err, retryConfig) {
		return false
	}

	// 进行重试
	for i := 0; i < retryConfig.MaxRetries; i++ {
		// 等待重试间隔
		if i > 0 {
			time.Sleep(time.Duration(retryConfig.RetryDelayMs) * time.Millisecond)
		}

		// 记录重试信息
		logger.Warn("API请求第%d次重试: %s, 错误: %v", i+1, targetURL, err)

		// 获取另一个API密钥进行重试
		apiKey, err := key.GetBestKeyForRequest(requestType, modelName, tokenEstimate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "No suitable API keys available for retry",
			})
			return false
		}

		// 记录重试信息
		maskedKey := utils.MaskKey(apiKey)
		logger.Info("使用新的API密钥重试请求: %s", maskedKey)

		// 创建新的请求
		req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(bodyBytes))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create request for retry: %v", err),
			})
			return false
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
		utils.SetCommonHeaders(req, apiKey)

		// 创建 HTTP 客户端
		client := utils.CreateClient()

		// 发送请求
		resp, err := client.Do(req)
		if err != nil {
			// 更新密钥失败记录
			key.UpdateApiKeyStatus(apiKey, false)

			// 记录错误并继续重试
			logger.Error("发送请求失败: %v", err)
			continue
		}
		defer resp.Body.Close()

		// 记录请求信息
		logger.InfoWithKey(maskedKey, "API请求重试: %s %s", c.Request.Method, c.Request.URL.Path)

		// 读取响应体
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			// 更新密钥失败记录
			key.UpdateApiKeyStatus(apiKey, false)
			continue
		}

		// 检查响应状态码
		success := resp.StatusCode >= 200 && resp.StatusCode < 300

		// 更新密钥状态
		key.UpdateApiKeyStatus(apiKey, success)

		// 统计请求数据
		tokenCount := utils.EstimateTokenCount(bodyBytes, respBody)
		config.AddKeyRequestStat(apiKey, 1, tokenCount)

		// 更新每日统计数据
		modelNameForStats := extractModelName(c.Request, respBody)
		promptTokensCount, completionTokensCount := extractTokenCounts(respBody)
		if promptTokensCount == 0 && completionTokensCount == 0 {
			promptTokensCount = tokenCount / 2
			completionTokensCount = tokenCount - promptTokensCount
		}
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

		// 如果请求成功，返回
		if success {
			return true
		}
	}

	// 所有重试都失败，返回错误
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": "All retry attempts failed",
	})
	return false
}

// 处理API请求，返回是否成功处理和可能的错误
func processApiRequest(c *gin.Context, targetURL string, bodyBytes []byte, requestType string, modelName string, tokenEstimate int) (bool, error) {
	// 检查是否是流式响应完成后的后续请求
	if streamCompleted, exists := c.Get("stream_completed"); exists && streamCompleted.(bool) {
		logger.Info("检测到流式响应完成后的后续请求，跳过处理")
		// 返回成功，避免处理这个请求
		c.Status(http.StatusOK)
		return true, nil
	}

	// 根据请求类型选择最佳的API密钥
	apiKey, err := key.GetBestKeyForRequest(requestType, modelName, tokenEstimate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "No suitable API keys available",
		})
		return false, err
	}

	// 创建新的请求
	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create request: %v", err),
		})
		return false, err
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
	utils.SetCommonHeaders(req, apiKey)

	// 创建 HTTP 客户端
	client := utils.CreateClient()

	// 发送请求
	resp, err := client.Do(req)

	if err != nil {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)
		return false, err
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
		return false, err
	}

	// 检查响应状态码
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	// 如果请求失败，返回错误
	if !success {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)
		return false, fmt.Errorf("API请求失败，状态码: %d", resp.StatusCode)
	}

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

	return true, nil
}

// 处理 OpenAI 格式的 API 代理请求
func HandleOpenAIProxy(c *gin.Context) {
	// 检查是否有直接从以前的流式响应中设置的标志
	if streamCompleted, exists := c.Get("stream_completed"); exists && streamCompleted.(bool) {
		logger.Info("检测到从流式响应完成后的后续请求，直接返回OK")
		c.Status(http.StatusOK)
		return
	}

	// 对于流式请求，设置较长的超时时间
	if strings.Contains(c.Request.URL.Path, "/chat/completions") || strings.Contains(c.Request.URL.Path, "/completions") {
		// 检查是否可能是流式请求
		var requestData map[string]interface{}
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // 恢复body

		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if stream, ok := requestData["stream"].(bool); ok && stream {
				// 尝试获取模型名称，检查是否是Deepseek R1
				if model, ok := requestData["model"].(string); ok &&
					strings.Contains(strings.ToLower(model), "deepseek") &&
					strings.Contains(model, "r1") {
					// 对于Deepseek R1流式请求，做特殊处理
					logger.Info("检测到Deepseek R1模型流式请求，应用特殊优化设置")
					// 禁用各种可能的缓冲机制
					c.Writer.Header().Set("X-Accel-Buffering", "no") // 禁用Nginx缓冲
					c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
					c.Writer.Header().Set("Connection", "keep-alive")
					c.Writer.Header().Set("Transfer-Encoding", "chunked")
					c.Writer.Header().Set("Content-Type", "text/event-stream")
					c.Writer.Header().Set("X-Content-Type-Options", "nosniff")

					// 使用background上下文并设置更长的超时
					ctx := context.Background()
					// 创建一个新的上下文，使用配置的超时时间
					cfg := config.GetConfig()
					ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.RequestSettings.ProxyHandler.StandardTimeout)*time.Minute)
					defer cancel()
					c.Request = c.Request.WithContext(ctx)

					// 设置更长的读写超时
					if h, ok := c.Writer.(http.Hijacker); ok {
						conn, _, err := h.Hijack()
						if err == nil {
							if tc, ok := conn.(*net.TCPConn); ok {
								tc.SetKeepAlive(true)
								tc.SetKeepAlivePeriod(30 * time.Second)
								tc.SetReadBuffer(65536)  // 64KB
								tc.SetWriteBuffer(65536) // 64KB
							}
						}
					}
				}
			}

			// 检查模型是否被禁用
			if model, ok := requestData["model"].(string); ok && isModelDisabled(model) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": map[string]interface{}{
						"message": fmt.Sprintf("模型 %s 已被禁用", model),
						"type":    "invalid_request_error",
						"code":    403,
					},
				})
				return
			}
		}
	}

	// 获取配置
	cfg := config.GetConfig()
	baseURL := cfg.ApiProxy.BaseURL

	// 获取请求路径
	path := c.Param("path")

	// 检查请求是否来自无版本号的路径
	isVersionlessPath := false
	fullPath := c.Request.URL.Path
	if !strings.HasPrefix(fullPath, "/v1/") {
		// 这是一个无版本号的路径，需要特殊处理
		isVersionlessPath = true
	}

	// 构建目标 URL
	var targetURL string
	// 无论是否是无版本号路径，都确保目标URL包含/v1前缀
	if isVersionlessPath {
		// 从完整路径中提取路径部分
		// 例如，/chat/completions 变为 /v1/chat/completions
		targetURL = fmt.Sprintf("%s/v1%s", baseURL, fullPath)
		logger.Info("检测到无版本号路径请求: %s，转发到: %s", fullPath, targetURL)
	} else {
		// 带有版本号的标准路径
		targetURL = fmt.Sprintf("%s/v1%s", baseURL, path)
		logger.Info("检测到标准版本号路径请求: %s，转发到: %s", "/v1"+path, targetURL)
	}

	// 如果是 /models 请求，使用特殊处理
	if strings.HasSuffix(fullPath, "/models") {
		logger.Info("检测到模型列表请求: %s", fullPath)
		// 模型列表请求不需要请求体，直接处理
		HandleModelsRequest(c, "")
		return
	}

	// 如果是 /user/info 请求，使用特殊处理
	if strings.HasSuffix(fullPath, "/user/info") {
		logger.Info("检测到用户信息请求: %s", fullPath)
		// 简单转发用户信息请求
		forwardUserInfoRequest(c, targetURL)
		return
	}

	// 读取请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read request body: %v", err),
		})
		return
	}

	// 检查请求体是否为空或者无效JSON，除了GET请求
	if c.Request.Method != http.MethodGet && (len(bodyBytes) == 0 || !json.Valid(bodyBytes)) {
		// 仅当不是GET请求时才进行此检查
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"message": "Request body is empty or invalid JSON",
				"type":    "invalid_request_error",
				"code":    400,
			},
		})
		return
	}

	// 第二次检查是否为JSON请求，并获取模型名称（如果第一次检查没有获取到）
	if c.Request.Method != http.MethodGet && len(bodyBytes) > 0 && json.Valid(bodyBytes) {
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			if model, ok := requestData["model"].(string); ok && isModelDisabled(model) {
				c.JSON(http.StatusForbidden, gin.H{
					"error": map[string]interface{}{
						"message": fmt.Sprintf("模型 %s 已被禁用", model),
						"type":    "invalid_request_error",
						"code":    403,
					},
				})
				return
			}
		}
	}

	// 检查chat/completions请求中是否缺少必要字段
	if strings.Contains(fullPath, "/chat/completions") {
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			// 检查是否存在messages字段
			if messages, hasMessages := requestData["messages"]; !hasMessages {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": map[string]interface{}{
						"message": "Message field is required for chat completions requests",
						"type":    "invalid_request_error",
						"code":    400,
					},
				})
				return
			} else {
				// 确保messages是一个数组
				messagesArray, isArray := messages.([]interface{})
				if !isArray || len(messagesArray) == 0 {
					c.JSON(http.StatusBadRequest, gin.H{
						"error": map[string]interface{}{
							"message": "Messages must be a non-empty array",
							"type":    "invalid_request_error",
							"code":    400,
						},
					})
					return
				}
			}
		}
	}

	// 检查completions请求中是否缺少必要字段
	if strings.Contains(fullPath, "/completions") && !strings.Contains(fullPath, "/chat/completions") {
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			// 检查是否存在prompt字段
			if _, hasPrompt := requestData["prompt"]; !hasPrompt {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": map[string]interface{}{
						"message": "Prompt field is required for completions requests",
						"type":    "invalid_request_error",
						"code":    400,
					},
				})
				return
			}
		}
	}

	// 检查embeddings请求中是否缺少必要字段
	if strings.Contains(fullPath, "/embeddings") {
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			// 检查是否存在input字段
			_, hasInput := requestData["input"]
			_, hasModel := requestData["model"]
			if !hasInput || !hasModel {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": map[string]interface{}{
						"message": "Input field is required for embeddings requests",
						"type":    "invalid_request_error",
						"code":    400,
					},
				})
				return
			}
		}
	}

	// 检查rerank请求中是否缺少必要字段
	if strings.Contains(fullPath, "/rerank") {
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			// 检查是否存在query字段
			if _, hasQuery := requestData["query"]; !hasQuery {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": map[string]interface{}{
						"message": "Query field is required for rerank requests",
						"type":    "invalid_request_error",
						"code":    400,
					},
				})
				return
			}
			// 检查是否存在documents字段
			if _, hasDocuments := requestData["documents"]; !hasDocuments {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": map[string]interface{}{
						"message": "Documents field is required for rerank requests",
						"type":    "invalid_request_error",
						"code":    400,
					},
				})
				return
			}
		}
	}

	// 检查images/generations请求中是否缺少必要字段
	if strings.Contains(fullPath, "/images/generations") {
		var requestData map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &requestData); err == nil {
			// 检查是否存在prompt字段
			if _, hasPrompt := requestData["prompt"]; !hasPrompt {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": map[string]interface{}{
						"message": "Prompt field is required for image generation requests",
						"type":    "invalid_request_error",
						"code":    400,
					},
				})
				return
			}
		}
	}

	// 分析请求类型和估计token数量
	var requestPath string
	if isVersionlessPath {
		// 对于无版本号路径，我们需要将完整路径作为分析依据
		requestPath = fullPath
	} else {
		requestPath = path
	}
	requestType, modelName, tokenEstimate := AnalyzeOpenAIRequest(requestPath, bodyBytes)

	// 转换请求体为硅基流动格式
	transformedBody, err := TransformRequestBody(bodyBytes, requestPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to transform request body: %v", err),
		})
		return
	}

	// 调用带重试逻辑的函数处理OpenAI格式请求
	success := processOpenAIRequestWithRetry(c, targetURL, transformedBody, bodyBytes, requestType, modelName, tokenEstimate, requestPath)

	// 如果请求成功且有模型名称，更新模型调用次数
	if success && modelName != "" {
		go updateModelCallCount(modelName)
	}
}

// 添加带重试逻辑的OpenAI请求处理函数
func processOpenAIRequestWithRetry(c *gin.Context, targetURL string, transformedBody []byte, originalBody []byte, requestType string, modelName string, tokenEstimate int, path string) bool {
	// 检查是否有直接从以前的流式响应中设置的标志
	if streamCompleted, exists := c.Get("stream_completed"); exists && streamCompleted.(bool) {
		logger.Info("检测到从流式响应完成后的后续请求，直接返回OK")
		c.Status(http.StatusOK)
		return true
	}

	// 获取配置
	cfg := config.GetConfig()
	retryConfig := cfg.ApiProxy.Retry

	// 检查是否是流式请求
	isStreamRequest := false
	if strings.Contains(path, "/chat/completions") || strings.Contains(path, "/completions") {
		var requestData map[string]interface{}
		if err := json.Unmarshal(originalBody, &requestData); err == nil {
			if stream, ok := requestData["stream"].(bool); ok && stream {
				isStreamRequest = true
			}
		}
	}

	// 流式请求需要特殊处理，暂不支持重试
	if isStreamRequest {
		handleOpenAIStreamRequest(c, targetURL, transformedBody, requestType, modelName, tokenEstimate, originalBody)
		return true
	}

	// 如果最大重试次数为0，直接处理一次请求
	if retryConfig.MaxRetries <= 0 {
		success, _ := processOpenAIRequest(c, targetURL, transformedBody, originalBody, requestType, modelName, tokenEstimate, path)
		return success
	}

	// 第一次尝试
	firstTry, err := processOpenAIRequest(c, targetURL, transformedBody, originalBody, requestType, modelName, tokenEstimate, path)
	if firstTry {
		// 请求成功，直接返回
		return true
	}

	// 检查是否需要重试
	if !shouldRetry(err, retryConfig) {
		return false
	}

	// 进行重试
	for i := 0; i < retryConfig.MaxRetries; i++ {
		// 等待重试间隔
		if i > 0 {
			time.Sleep(time.Duration(retryConfig.RetryDelayMs) * time.Millisecond)
		}

		// 记录重试信息
		logger.Warn("OpenAI格式API请求第%d次重试: %s, 错误: %v", i+1, targetURL, err)

		// 获取另一个API密钥进行重试
		apiKey, err := key.GetBestKeyForRequest(requestType, modelName, tokenEstimate)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "No suitable API keys available for retry",
			})
			return false
		}

		// 记录重试信息
		maskedKey := utils.MaskKey(apiKey)
		logger.Info("使用新的API密钥重试OpenAI格式请求: %s", maskedKey)

		// 创建新的请求
		req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(transformedBody))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to create request for retry: %v", err),
			})
			return false
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
		utils.SetCommonHeaders(req, apiKey)

		// 创建 HTTP 客户端
		client := utils.CreateClient()

		// 发送请求
		resp, err := client.Do(req)
		if err != nil {
			// 区分连接错误和其他错误类型
			if strings.Contains(err.Error(), "context deadline exceeded") ||
				strings.Contains(err.Error(), "timeout") {
				logger.Error("请求处理超时: %v", err)
				c.JSON(http.StatusGatewayTimeout, gin.H{
					"error": gin.H{
						"message": "请求处理超时，已达到最大响应时间限制",
						"type":    "timeout_error",
						"code":    "context_deadline_exceeded",
					},
				})
			} else if strings.Contains(err.Error(), "canceled") {
				logger.Info("请求被取消: %v", err)
				// 客户端已断开，不需要返回任何内容
			} else {
				logger.Error("发送请求失败: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to send request: %v", err),
				})
			}

			// 更新密钥失败记录
			key.UpdateApiKeyStatus(apiKey, false)
			continue
		}
		defer resp.Body.Close()

		// 记录请求信息
		logger.InfoWithKey(maskedKey, "OpenAI格式API请求重试: %s %s", c.Request.Method, c.Request.URL.Path)

		// 读取响应体
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			// 更新密钥失败记录
			key.UpdateApiKeyStatus(apiKey, false)
			continue
		}

		// 检查响应状态码
		success := resp.StatusCode >= 200 && resp.StatusCode < 300

		// 更新密钥状态
		key.UpdateApiKeyStatus(apiKey, success)

		// 统计请求数据
		tokenCount := utils.EstimateTokenCount(originalBody, respBody)
		config.AddKeyRequestStat(apiKey, 1, tokenCount)

		// 提取令牌计数
		promptTokensCount, completionTokensCount := extractTokenCounts(respBody)
		if promptTokensCount == 0 && completionTokensCount == 0 {
			promptTokensCount = tokenCount / 2
			completionTokensCount = tokenCount - promptTokensCount
		}

		// 添加到每日统计
		config.AddDailyRequestStat(apiKey, modelName, 1, promptTokensCount, completionTokensCount, success)

		// 转换响应为OpenAI格式
		openAIResponse, err := TransformResponseBody(respBody, path)
		if err != nil {
			continue
		}

		// 返回转换后的响应
		c.Header("Content-Type", "application/json")
		c.Status(resp.StatusCode)
		c.Writer.Write(openAIResponse)

		// 如果请求成功，返回
		if success {
			return true
		}
	}

	// 所有重试都失败，返回错误
	c.JSON(http.StatusInternalServerError, gin.H{
		"error": "All retry attempts failed",
	})
	return false
}

// shouldRetry 判断是否需要重试
func shouldRetry(err error, retryConfig config.RetryConfig) bool {
	// 如果是网络错误且配置允许重试网络错误
	if err != nil && retryConfig.RetryOnNetworkErrors {
		return true
	}

	// 如果是HTTP响应错误，检查状态码是否需要重试
	if err != nil {
		// 尝试从错误信息中提取状态码
		if strings.Contains(err.Error(), "status code:") {
			for _, code := range retryConfig.RetryOnStatusCodes {
				if strings.Contains(err.Error(), fmt.Sprintf("status code: %d", code)) {
					return true
				}
			}
		}
	}

	return false
}

// 处理OpenAI流式请求
func handleOpenAIStreamRequest(c *gin.Context, targetURL string, transformedBody []byte, requestType string, modelName string, tokenEstimate int, originalBody []byte) {
	// 检查是否有直接从以前的流式响应中设置的标志
	if streamCompleted, exists := c.Get("stream_completed"); exists && streamCompleted.(bool) {
		logger.Info("检测到从流式响应完成后的后续请求，直接返回OK")
		c.Status(http.StatusOK)
		return
	}

	// 检查请求体中的stream字段是否为true
	var requestData map[string]interface{}
	if err := json.Unmarshal(originalBody, &requestData); err == nil {
		if stream, exists := requestData["stream"]; exists {
			// 如果stream字段存在且为false，则应该使用非流式处理
			if streamBool, ok := stream.(bool); ok && !streamBool {
				logger.Info("检测到请求中stream=false，转为非流式请求处理")
				// 处理为非流式请求
				_, err := processOpenAIRequest(c, targetURL, transformedBody, originalBody, requestType, modelName, tokenEstimate, c.Request.URL.Path)
				if err != nil {
					logger.Error("处理非流式请求失败: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"error": fmt.Sprintf("处理请求失败: %v", err),
					})
				}
				return
			}
		}
	}

	// 检查模型是否被禁用
	if modelName != "" && isModelDisabled(modelName) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("模型 %s 已被禁用", modelName),
				"type":    "invalid_request_error",
				"code":    403,
			},
		})
		return
	}

	// 根据请求类型选择最佳的API密钥
	apiKey, err := key.GetBestKeyForRequest(requestType, modelName, tokenEstimate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "No suitable API keys available",
		})
		return
	}

	// 检查是否是推理模型（类型为7）
	isReasonModelType := false
	if modelName != "" {
		isReasonModelType = isReasonModel(modelName)
		if isReasonModelType {
			logger.Info("检测到推理模型请求：%s，使用专用优化客户端和更长的超时设置", modelName)
		}
	}

	// 创建带有超时的上下文，设置合理的超时时间
	var requestTimeout time.Duration
	cfg := config.GetConfig()
	if isReasonModelType {
		// 为推理模型创建更长的超时时间
		requestTimeout = time.Duration(cfg.RequestSettings.ProxyHandler.InferenceTimeout) * time.Minute
		logger.Info("为推理模型设置%d分钟的请求超时", cfg.RequestSettings.ProxyHandler.InferenceTimeout)
	} else {
		// 为其他模型使用标准超时
		requestTimeout = time.Duration(cfg.RequestSettings.ProxyHandler.StandardTimeout) * time.Minute
		logger.Info("为普通模型设置%d分钟的请求超时", cfg.RequestSettings.ProxyHandler.StandardTimeout)
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel() // 确保函数结束时取消上下文

	// 创建新的请求，使用我们的超时上下文
	req, err := http.NewRequestWithContext(ctx, c.Request.Method, targetURL, bytes.NewBuffer(transformedBody))
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

	// 设置 Authorization header 和其他通用头
	utils.SetCommonHeaders(req, apiKey)

	// 为推理模型添加特殊请求头
	if isReasonModelType {
		utils.SetInferenceModelHeaders(req)
	}

	// 创建 HTTP 客户端，根据模型类型选择合适的超时设置和响应头
	var client *http.Client
	if isReasonModelType {
		// 使用推理模型专用客户端
		client = utils.CreateInferenceModelClient(requestTimeout)
		logger.Info("推理模型使用优化客户端和%v的请求超时", requestTimeout)
		// 设置推理模型专用的流式响应头
		utils.SetInferenceStreamResponseHeaders(c.Writer)
	} else {
		// 使用普通模型专用客户端
		client = utils.CreateStandardModelClient(requestTimeout)
		logger.Info("普通模型使用标准客户端和%v的请求超时", requestTimeout)
		// 设置标准流式响应头
		utils.SetStreamResponseHeaders(c.Writer)
	}

	// 设置响应头，指示这是流式响应
	// 注意：这是一个重复的设置，上面已经根据模型类型设置了适当的响应头，这行将被移除
	logger.Info("跳过重复的响应头设置")
	// utils.SetStreamResponseHeaders(c.Writer)

	// 监听客户端连接关闭
	clientCtx, clientCancel := context.WithCancel(ctx)
	go func() {
		<-c.Request.Context().Done()
		logger.Info("检测到客户端已断开连接，取消流式请求")
		clientCancel() // 取消请求
	}()
	defer clientCancel()

	// 发送请求，使用上下文控制超时
	resp, err := client.Do(req.WithContext(clientCtx))
	if err != nil {
		// 区分连接错误和其他错误类型
		if strings.Contains(err.Error(), "context deadline exceeded") ||
			strings.Contains(err.Error(), "timeout") {
			logger.Error("请求处理超时: %v", err)
			c.JSON(http.StatusGatewayTimeout, gin.H{
				"error": gin.H{
					"message": "请求处理超时，已达到最大响应时间限制",
					"type":    "timeout_error",
					"code":    "context_deadline_exceeded",
				},
			})
		} else if strings.Contains(err.Error(), "canceled") {
			logger.Info("请求被取消: %v", err)
			// 客户端已断开，不需要返回任何内容
		} else {
			logger.Error("发送请求失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to send request: %v", err),
			})
		}

		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)
		return
	}

	// 检查状态码
	if resp.StatusCode != http.StatusOK {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)

		// 尝试读取错误消息
		errBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		// 记录详细的状态码和错误信息
		if err != nil {
			logger.Error("读取错误响应体失败: %v", err)
			errBody = []byte("无法读取响应内容")
		} else if len(errBody) == 0 {
			logger.Error("流式请求返回非200状态码: %d, 但响应体为空", resp.StatusCode)
			errBody = []byte(fmt.Sprintf("服务器返回 %d 状态码，但未提供具体错误信息", resp.StatusCode))
		} else {
			logger.Error("流式请求返回非200状态码: %d, 响应: %s", resp.StatusCode, string(errBody))
		}

		// 尝试解析JSON错误消息
		var errorResponse struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Error   struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}

		var errorMessage string
		var errorType string
		var errorCode interface{}

		// 如果响应体不为空，尝试解析JSON
		if len(errBody) > 0 {
			if err := json.Unmarshal(errBody, &errorResponse); err == nil {
				// 首先尝试获取标准OpenAI错误格式
				if errorResponse.Error.Message != "" {
					errorMessage = errorResponse.Error.Message
					errorType = errorResponse.Error.Type
					errorCode = errorResponse.Error.Code
				} else if errorResponse.Message != "" {
					// 然后尝试获取自定义API错误格式
					errorMessage = errorResponse.Message
					errorType = "api_error"
					errorCode = errorResponse.Code
				}
			}
		}

		// 如果无法解析，使用原始错误信息或默认信息
		if errorMessage == "" {
			if len(errBody) > 0 {
				errorMessage = string(errBody)
			} else {
				errorMessage = fmt.Sprintf("服务器返回了 %d 状态码，但未提供详细信息", resp.StatusCode)
			}
			errorType = "unknown_error"
			errorCode = resp.StatusCode
		}

		// 以结构化方式返回错误
		c.JSON(resp.StatusCode, gin.H{
			"error": gin.H{
				"message": errorMessage,
				"type":    errorType,
				"code":    errorCode,
			},
		})
		return
	}

	// 记录成功启动流式响应
	logger.Info("成功启动流式响应，正在处理响应流...")

	// 处理流式响应，传递与当前请求相同的超时上下文
	HandleStreamResponse(c, resp.Body, apiKey, originalBody)
}

// 处理非流式OpenAI请求，返回是否成功处理和可能的错误
func processOpenAIRequest(c *gin.Context, targetURL string, transformedBody []byte, originalBody []byte, requestType string, modelName string, tokenEstimate int, path string) (bool, error) {
	// 检查是否是流式响应完成后的后续请求
	if streamCompleted, exists := c.Get("stream_completed"); exists && streamCompleted.(bool) {
		logger.Info("检测到流式响应完成后的后续请求，跳过模型禁用检查")
		// 返回成功，避免处理这个请求
		c.Status(http.StatusOK)
		return true, nil
	}

	// 检查模型是否被禁用
	if modelName != "" && isModelDisabled(modelName) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": map[string]interface{}{
				"message": fmt.Sprintf("模型 %s 已被禁用", modelName),
				"type":    "invalid_request_error",
				"code":    403,
			},
		})
		return false, fmt.Errorf("模型 %s 已被禁用", modelName)
	}

	// 根据请求类型选择最佳的API密钥
	apiKey, err := key.GetBestKeyForRequest(requestType, modelName, tokenEstimate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "No suitable API keys available",
		})
		return false, err
	}

	// 创建新的请求
	req, err := http.NewRequest(c.Request.Method, targetURL, bytes.NewBuffer(transformedBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to create request: %v", err),
		})
		return false, err
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
	utils.SetCommonHeaders(req, apiKey)

	// 创建 HTTP 客户端
	client := utils.CreateClient()

	// 发送请求
	resp, err := client.Do(req)

	if err != nil {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)
		return false, err
	}
	defer resp.Body.Close()

	// 记录请求信息
	maskedKey := utils.MaskKey(apiKey)
	logger.InfoWithKey(maskedKey, "OpenAI格式API请求: %s %s", c.Request.Method, c.Request.URL.Path)

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to read response body: %v", err),
		})
		return false, err
	}

	// 检查响应状态码
	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	// 如果请求失败，返回错误
	if !success {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)

		// 尝试解析JSON错误消息
		var errorResponse struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Error   struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			} `json:"error"`
		}

		var errorMessage string
		var errorType string
		var errorCode interface{}

		if err := json.Unmarshal(respBody, &errorResponse); err == nil {
			// 首先尝试获取标准OpenAI错误格式
			if errorResponse.Error.Message != "" {
				errorMessage = errorResponse.Error.Message
				errorType = errorResponse.Error.Type
				errorCode = errorResponse.Error.Code
			} else if errorResponse.Message != "" {
				// 然后尝试获取自定义API错误格式
				errorMessage = errorResponse.Message
				errorType = "api_error"
				errorCode = errorResponse.Code
			}
		}

		// 如果无法解析，使用原始错误信息
		if errorMessage == "" {
			errorMessage = string(respBody)
			errorType = "unknown_error"
			errorCode = resp.StatusCode
		}

		// 记录详细错误信息
		logger.Error("OpenAI请求失败，状态码: %d, 错误: %s", resp.StatusCode, errorMessage)

		// 以结构化方式返回错误
		c.JSON(resp.StatusCode, gin.H{
			"error": gin.H{
				"message": errorMessage,
				"type":    errorType,
				"code":    errorCode,
			},
		})

		return false, fmt.Errorf("OpenAI格式API请求失败: %s", errorMessage)
	}

	// 更新密钥状态
	key.UpdateApiKeyStatus(apiKey, success)

	// 统计请求数据
	tokenCount := utils.EstimateTokenCount(originalBody, respBody)
	config.AddKeyRequestStat(apiKey, 1, tokenCount)

	// 提取令牌计数
	promptTokensCount, completionTokensCount := extractTokenCounts(respBody)
	if promptTokensCount == 0 && completionTokensCount == 0 {
		// 如果无法从响应中提取令牌计数，使用估算值
		promptTokensCount = tokenCount / 2
		completionTokensCount = tokenCount - promptTokensCount
	}

	// 添加到每日统计
	config.AddDailyRequestStat(apiKey, modelName, 1, promptTokensCount, completionTokensCount, success)

	// 转换响应为OpenAI格式
	openAIResponse, err := TransformResponseBody(respBody, path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Sprintf("Failed to transform response body: %v", err),
		})
		return false, err
	}

	// 返回转换后的响应
	c.Header("Content-Type", "application/json")
	c.Status(resp.StatusCode)
	c.Writer.Write(openAIResponse)

	return true, nil
}

// 处理模型列表请求
func HandleModelsRequest(c *gin.Context, apiKey string) {
	// 根据请求类型选择最佳的API密钥（如果未提供）
	if apiKey == "" {
		var err error
		apiKey, err = key.GetBestKeyForRequest("completion", "", 100) // 轻量级请求
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "No suitable API keys available",
			})
			return
		}
	}

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
	utils.SetCommonHeaders(req, apiKey)
	// 创建HTTP客户端
	client := utils.CreateClient()

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

	// 过滤掉被禁用的模型
	var modelsResponse map[string]interface{}
	if err := json.Unmarshal(respBody, &modelsResponse); err == nil {
		if models, ok := modelsResponse["data"].([]interface{}); ok {
			var filteredModels []interface{}
			for _, model := range models {
				if modelObj, ok := model.(map[string]interface{}); ok {
					if modelID, ok := modelObj["id"].(string); ok && !isModelDisabled(modelID) {
						filteredModels = append(filteredModels, model)
					}
				} else {
					// 如果无法解析模型对象，保留它
					filteredModels = append(filteredModels, model)
				}
			}
			modelsResponse["data"] = filteredModels

			// 将过滤后的响应转换回JSON
			filteredResponse, err := json.Marshal(modelsResponse)
			if err == nil {
				respBody = filteredResponse
			} else {
				logger.Error("过滤模型列表后转换JSON失败: %v", err)
				// 出错时使用原始响应
			}
		}
	} else {
		logger.Error("解析模型列表响应失败: %v", err)
		// 出错时使用原始响应
	}

	// 返回API的响应（可能经过过滤）
	c.Status(resp.StatusCode)
	c.Writer.Write(respBody)

	logger.Info("成功返回模型列表")
}

// 处理流式响应
func HandleStreamResponse(c *gin.Context, responseBody io.ReadCloser, apiKey string, requestBody []byte) {
	logger.Info("开始处理流式响应")

	// 创建缓冲读取器，增加缓冲区大小以处理大型响应
	reader := bufio.NewReaderSize(responseBody, 65536) // 增加到64KB的缓冲区

	// 创建刷新写入器，确保数据立即发送
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logger.Error("流式处理失败：响应写入器不支持刷新")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Streaming not supported",
		})
		return
	}

	// 创建带超时的上下文，而不是使用无限期的background上下文
	// 检查请求体中是否包含Deepseek R1模型
	var isDeepseekR1 bool
	var requestData map[string]interface{}
	if err := json.Unmarshal(requestBody, &requestData); err == nil {
		if model, ok := requestData["model"].(string); ok {
			if strings.Contains(strings.ToLower(model), "deepseek") && strings.Contains(model, "r1") {
				isDeepseekR1 = true
				logger.Info("检测到Deepseek R1模型请求，启用特殊处理模式")
			}
		}
	}

	// 设置合理的超时时间，根据模型类型调整
	var streamTimeout time.Duration
	cfg := config.GetConfig()
	if isDeepseekR1 {
		streamTimeout = time.Duration(cfg.RequestSettings.ProxyHandler.StreamTimeout) * time.Minute // 使用配置的流式超时
		logger.Info("为Deepseek R1流式响应设置%d分钟超时", cfg.RequestSettings.ProxyHandler.StreamTimeout)
	} else {
		streamTimeout = time.Duration(cfg.RequestSettings.ProxyHandler.StandardTimeout) * time.Minute // 使用配置的标准超时
		logger.Info("为普通模型流式响应设置%d分钟超时", cfg.RequestSettings.ProxyHandler.StandardTimeout)
	}

	// 使用带超时的上下文，确保有明确的超时控制
	ctx, cancel := context.WithTimeout(context.Background(), streamTimeout)
	defer cancel()

	// 对于R1模型，立即发送一个初始响应，保持连接活跃
	if isDeepseekR1 {
		initialComment := ": 已连接到Deepseek R1服务，正在生成回答，请稍候...\n\n"
		c.Writer.Write([]byte(initialComment))
		flusher.Flush()
	}

	// 初始化计数器
	var totalTokens int
	var eventCount int
	var lastProgressTime = time.Now() // 上次进度更新时间

	// 心跳间隔 - 对Deepseek R1更频繁
	var heartbeatInterval time.Duration = time.Duration(cfg.RequestSettings.ProxyHandler.HeartbeatInterval) * time.Second
	if isDeepseekR1 {
		heartbeatInterval = time.Duration(cfg.RequestSettings.ProxyHandler.HeartbeatInterval/3) * time.Second // R1模型使用更短的心跳间隔
	}

	// 异常处理通道
	errorChan := make(chan error, 1)
	doneChan := make(chan bool, 1)

	// 创建缓冲区用于批量发送
	var buffer bytes.Buffer
	// 对于Deepseek R1，降低缓冲区阈值，确保更频繁发送数据
	bufferThreshold := cfg.RequestSettings.ProxyHandler.BufferThreshold
	if isDeepseekR1 {
		bufferThreshold = cfg.RequestSettings.ProxyHandler.BufferThreshold / 4 // R1模型使用更小的缓冲区阈值
	}

	// 上次刷新时间
	lastFlushTime := time.Now()
	// 最大刷新间隔 (毫秒)，对Deepseek R1使用更短的间隔
	maxFlushInterval := time.Duration(cfg.RequestSettings.ProxyHandler.MaxFlushInterval) * time.Millisecond
	if isDeepseekR1 {
		maxFlushInterval = time.Duration(cfg.RequestSettings.ProxyHandler.MaxFlushInterval/2) * time.Millisecond // R1模型使用更短的刷新间隔
	}

	// 进度报告间隔
	progressInterval := time.Duration(cfg.RequestSettings.ProxyHandler.ProgressInterval) * time.Second

	// 连接已断开标志
	var connectionClosed atomic.Bool

	// 监听客户端连接关闭
	go func() {
		<-c.Request.Context().Done()
		connectionClosed.Store(true)
		cancel() // 取消我们的上下文
		logger.Info("检测到客户端连接已关闭")
	}()

	// 监听我们自己的上下文超时
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			logger.Warn("流式响应处理超时（%v）：已达到最大处理时间限制", streamTimeout)
			if !connectionClosed.Load() {
				// 向客户端发送超时通知
				timeoutMsg := "data: {\"error\":{\"message\":\"处理超时，已达到最大响应时间限制\",\"type\":\"timeout_error\",\"code\":\"context_deadline_exceeded\"}}\n\n"
				c.Writer.Write([]byte(timeoutMsg))
				flusher.Flush()
			}
		}
	}()

	// 启动心跳协程，定期发送注释保持连接活跃
	go func() {
		heartbeatTicker := time.NewTicker(heartbeatInterval)
		defer heartbeatTicker.Stop()

		heartbeatCount := 0
		dataSentCount := 0 // 跟踪发送的数据心跳数量

		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatTicker.C:
				if connectionClosed.Load() {
					return
				}

				heartbeatCount++

				// 对于Deepseek R1，每隔几次心跳发送一个额外的数据包
				if isDeepseekR1 && heartbeatCount%3 == 0 {
					dataSentCount++
					keepaliveData := []byte(fmt.Sprintf("data: {\"id\":\"chatcmpl-hb%d\",\"object\":\"chat.completion.chunk\",\"created\":%d,\"model\":\"deepseek-r1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\"},\"finish_reason\":null}]}\n\n",
						dataSentCount, time.Now().Unix()))
					_, err := c.Writer.Write(keepaliveData)
					if err != nil {
						if !connectionClosed.Load() {
							logger.Error("数据包心跳发送失败: %v", err)
						}
					} else {
						flusher.Flush()
					}
				}

				// 发送心跳注释（作为SSE注释，客户端会忽略）
				heartbeatMsg := fmt.Sprintf(": heartbeat-%d\n\n", heartbeatCount)
				_, err := c.Writer.Write([]byte(heartbeatMsg))
				if err != nil {
					if !connectionClosed.Load() {
						logger.Error("心跳发送失败: %v", err)
						errorChan <- fmt.Errorf("心跳发送失败: %v", err)
					}
					return
				}
				flusher.Flush()

				// 仅在Debug级别记录心跳
				if heartbeatCount%5 == 0 {
					logger.Info("已发送%d次心跳以保持连接活跃, 数据包心跳: %d", heartbeatCount, dataSentCount)
				}
			}
		}
	}()

	// 读取并处理每一行SSE事件
	go func() {
		defer func() {
			doneChan <- true
			close(doneChan)
		}()

		readTimeoutChan := make(chan error, 1)

		for {
			// 首先检查上下文是否已取消
			select {
			case <-ctx.Done():
				if ctx.Err() == context.DeadlineExceeded {
					errorChan <- fmt.Errorf("流式响应处理超时: %v", ctx.Err())
				} else {
					errorChan <- ctx.Err()
				}
				return
			default:
				// 继续处理
			}

			if connectionClosed.Load() {
				return
			}

			// 定期报告进度，避免客户端认为连接已断开
			if time.Since(lastProgressTime) > progressInterval {
				logger.Info("流式响应处理中，已处理 %d 个事件，约 %d tokens", eventCount, totalTokens)
				lastProgressTime = time.Now()
			}

			// 使用带超时的上下文创建一个读取操作
			readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)

			// 使用goroutine包装读取操作
			go func() {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					readTimeoutChan <- err
					return
				}

				// 处理接收到的行
				if len(bytes.TrimSpace(line)) == 0 {
					// 空行不处理
					readTimeoutChan <- nil
					return
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
						if !connectionClosed.Load() {
							_, err := c.Writer.Write(buffer.Bytes())
							if err == nil {
								buffer.Reset()
								flusher.Flush()
							}
						}
						readTimeoutChan <- io.EOF // 使用EOF表示正常结束
						return
					}

					// 转换事件数据，确保与OpenAI API格式兼容
					transformedData, err := TransformStreamEvent(bytes.TrimSpace(data))
					if err != nil {
						logger.Error("转换流式事件失败: %v", err)
						// 使用原始数据
						transformedData = bytes.TrimSpace(data)
					}

					// 更新token估算
					var jsonData map[string]interface{}
					if err := json.Unmarshal(transformedData, &jsonData); err == nil {
						// 首先尝试从usage中获取total_tokens
						if usage, ok := jsonData["usage"].(map[string]interface{}); ok {
							if tt, ok := usage["total_tokens"].(float64); ok {
								// 更新总tokens数量为API返回的值
								if eventCount <= 3 || eventCount%50 == 0 {
									logger.Info("事件#%d: 从API返回的usage中读取total_tokens=%d", eventCount, int(tt))
								}
								// 使用API返回的token数量
								totalTokens = int(tt)
								// 继续处理，但不再进行token估算
							}
						} else if choices, ok := jsonData["choices"].([]interface{}); ok && len(choices) > 0 {
							// 如果没有usage字段，继续使用原来的估算方法
							if choice, ok := choices[0].(map[string]interface{}); ok {
								if delta, ok := choice["delta"].(map[string]interface{}); ok {
									if content, ok := delta["content"].(string); ok {
										// 简单估算：每个字符约为0.25个token
										tokenEstimate := int(float64(len(content)) * 0.2)
										if tokenEstimate == 0 && len(content) > 0 {
											tokenEstimate = 1 // 确保至少有1个token
										}
										totalTokens += tokenEstimate

										// 每100个事件记录一次token统计情况
										if eventCount%100 == 0 || eventCount <= 3 {
											logger.Info("事件#%d: 内容长度=%d字符, 估计tokens=%d, 累计tokens=%d",
												eventCount, len(content), tokenEstimate, totalTokens)
										}
									} else {
										// 如果无法提取content但delta不为空，尝试其他方式估算
										deltaJSON, _ := json.Marshal(delta)
										deltaStr := string(deltaJSON)
										if len(deltaStr) > 0 {
											// 记录无法直接提取content的情况
											if eventCount <= 10 || eventCount%100 == 0 {
												logger.Info("事件#%d: 无法提取content，delta=%s", eventCount, deltaStr)
											}

											// 仍然尝试估算token
											tokenEstimate := int(float64(len(deltaStr)) * 0.1) // 对JSON格式的内容降低估算比例
											if tokenEstimate == 0 && len(deltaStr) > 0 {
												tokenEstimate = 1
											}
											totalTokens += tokenEstimate
										}
									}
								} else {
									// 如果无法提取delta但choice不为空，记录问题
									if eventCount <= 10 || eventCount%100 == 0 {
										choiceJSON, _ := json.Marshal(choice)
										logger.Info("事件#%d: 无法提取delta，choice=%s", eventCount, string(choiceJSON))
									}

									// 确保每个事件至少计算一些token
									if eventCount%5 == 0 { // 每5个事件增加1个token（保守估计）
										totalTokens += 1
									}
								}
							} else {
								// 如果无法正确解析choice，记录问题
								if eventCount <= 10 || eventCount%100 == 0 {
									if len(choices) > 0 {
										choiceData, _ := json.Marshal(choices[0])
										logger.Info("事件#%d: choice格式异常，原始数据=%s", eventCount, string(choiceData))
									}
								}

								// 确保计数不为零
								if eventCount%5 == 0 {
									totalTokens += 1
								}
							}
						} else {
							// 如果无法提取choices，尝试直接从原始数据估算
							jsonString := string(transformedData)
							// 对Deepseek R1特殊处理
							if isDeepseekR1 && strings.Contains(jsonString, "\"choices\"") {
								// 从原始JSON字符串中查找content
								contentIndex := strings.Index(jsonString, "\"content\":")
								if contentIndex > 0 {
									// 粗略提取content内容
									contentStart := contentIndex + 11 // "content":"
									contentEnd := strings.Index(jsonString[contentStart:], "\"")
									if contentEnd > 0 {
										content := jsonString[contentStart : contentStart+contentEnd]
										if len(content) > 0 {
											tokenEstimate := int(float64(len(content)) * 0.25)
											if tokenEstimate == 0 {
												tokenEstimate = 1
											}
											totalTokens += tokenEstimate

											if eventCount%50 == 0 || eventCount <= 3 {
												logger.Info("事件#%d(字符串解析): 内容长度=%d字符, 估计tokens=%d, 累计tokens=%d",
													eventCount, len(content), tokenEstimate, totalTokens)
											}
										}
									}
								} else {
									// 没有找到content但仍是有效事件
									if eventCount%10 == 0 {
										totalTokens += 1 // 每10个无内容事件算1个token
									}
								}
							} else {
								// 保守估计，每10个事件至少计1个token
								if eventCount%10 == 0 {
									totalTokens += 1

									if eventCount <= 10 || eventCount%100 == 0 {
										logger.Info("事件#%d: 无法提取choices，使用保守估计", eventCount)
									}
								}
							}
						}
					} else {
						// JSON解析失败，使用基于事件的保守估计
						if eventCount%10 == 0 {
							totalTokens += 1 // 每10个事件至少计1个token

							if eventCount <= 10 || eventCount%100 == 0 {
								logger.Info("事件#%d: JSON解析失败: %v", eventCount, err)
							}
						}
					}

					// 添加到缓冲区
					buffer.WriteString("data: ")
					buffer.Write(transformedData)
					buffer.WriteString("\n\n")

					// 对于Deepseek R1，几乎总是立即刷新
					timeToFlush := buffer.Len() >= bufferThreshold ||
						eventCount <= 3 ||
						time.Since(lastFlushTime) >= maxFlushInterval ||
						isDeepseekR1

					if timeToFlush && !connectionClosed.Load() {
						_, writeErr := c.Writer.Write(buffer.Bytes())
						if writeErr != nil {
							readTimeoutChan <- writeErr
							return
						}
						buffer.Reset()
						flusher.Flush()
						lastFlushTime = time.Now()
					}
				} else {
					// 处理其他SSE事件(注释等)
					buffer.Write(line)
					buffer.WriteString("\n")

					// 定期刷新
					if (buffer.Len() >= bufferThreshold || time.Since(lastFlushTime) >= maxFlushInterval*2) && !connectionClosed.Load() {
						c.Writer.Write(buffer.Bytes())
						buffer.Reset()
						flusher.Flush()
						lastFlushTime = time.Now()
					}
				}

				readTimeoutChan <- nil
			}()

			// 等待读取结果或超时
			select {
			case err := <-readTimeoutChan:
				readCancel() // 取消读取上下文

				if err != nil {
					if err == io.EOF {
						// 正常结束
						errorChan <- nil
						return
					}

					// 对于Deepseek R1，特殊处理超时和上下文取消
					if isDeepseekR1 {
						if err == context.Canceled ||
							strings.Contains(err.Error(), "context canceled") ||
							strings.Contains(err.Error(), "deadline exceeded") ||
							strings.Contains(err.Error(), "timeout") {
							// 记录为信息而不是错误
							logger.Info("Deepseek R1读取超时或取消，继续处理: %v", err)
							// 发送一个空的delta事件保持连接活跃
							if !connectionClosed.Load() {
								keepaliveData := []byte("data: {\"id\":\"chatcmpl-keep-alive\",\"object\":\"chat.completion.chunk\",\"created\":" +
									fmt.Sprintf("%d", time.Now().Unix()) +
									",\"model\":\"deepseek-r1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\"},\"finish_reason\":null}]}\n\n")
								c.Writer.Write(keepaliveData)
								flusher.Flush()
							}
							// 继续处理，不中断
							continue
						}
					}

					errorChan <- err
					return
				}
			case <-readCtx.Done():
				readCancel() // 确保取消读取上下文

				// 读取超时处理
				if isDeepseekR1 {
					logger.Info("Deepseek R1读取操作超时，发送保持活动包")
					// 发送一个空的delta事件
					if !connectionClosed.Load() {
						keepaliveData := []byte("data: {\"id\":\"chatcmpl-keep-alive\",\"object\":\"chat.completion.chunk\",\"created\":" +
							fmt.Sprintf("%d", time.Now().Unix()) +
							",\"model\":\"deepseek-r1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\"},\"finish_reason\":null}]}\n\n")
						c.Writer.Write(keepaliveData)
						flusher.Flush()
					}
					// 继续处理，不中断
					continue
				} else {
					// 非Deepseek模型，读取超时视为错误
					errorChan <- fmt.Errorf("读取操作超时: %v", readCtx.Err())
					return
				}
			case <-ctx.Done():
				readCancel() // 确保取消读取上下文

				// 主上下文被取消
				if isDeepseekR1 {
					logger.Info("Deepseek R1上下文已取消，可能是正常完成")
					// 发送最后一个事件
					if !connectionClosed.Load() {
						finalData := []byte("data: {\"id\":\"chatcmpl-final\",\"object\":\"chat.completion.chunk\",\"created\":" +
							fmt.Sprintf("%d", time.Now().Unix()) +
							",\"model\":\"deepseek-r1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
						c.Writer.Write(finalData)
						flusher.Flush()
					}
				}

				if ctx.Err() == context.DeadlineExceeded {
					errorChan <- fmt.Errorf("流式响应处理总时间超出限制: %v", ctx.Err())
				} else {
					errorChan <- ctx.Err()
				}
				return
			}
		}
	}()

	// 等待处理完成
	var err error
	select {
	case err = <-errorChan:
		// 处理结束或出错
	case <-doneChan:
		// 正常完成
		err = nil
	case <-ctx.Done():
		// 上下文取消
		if ctx.Err() == context.DeadlineExceeded {
			err = fmt.Errorf("流式响应处理超时: %v", ctx.Err())
		} else {
			err = ctx.Err()
		}
	}

	// 发送剩余的缓冲数据
	if buffer.Len() > 0 && !connectionClosed.Load() {
		c.Writer.Write(buffer.Bytes())
		flusher.Flush()
	}

	// 处理错误信息
	if err == nil || err == io.EOF {
		logger.Info("流式响应正常完成")
	} else if err == context.Canceled || connectionClosed.Load() {
		logger.Info("客户端取消了连接")
	} else if strings.Contains(err.Error(), "deadline exceeded") {
		if isDeepseekR1 {
			// 对于Deepseek R1，超时结束也视为正常
			logger.Info("Deepseek R1流式响应由于超时而结束: %v", err)
		} else {
			// 对于其他模型，记录为警告
			logger.Warn("流式响应由于上下文超时而结束: %v", err)
		}

		// 尝试向客户端发送超时通知（如果连接仍然有效）
		if !connectionClosed.Load() {
			timeoutNotice := "data: {\"id\":\"timeout-notice\",\"object\":\"chat.completion.chunk\",\"created\":" +
				fmt.Sprintf("%d", time.Now().Unix()) +
				",\"model\":\"generic\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"\\n\\n[系统通知: 响应生成已达到最大时间限制]\"},\"finish_reason\":\"timeout\"}]}\n\n"
			c.Writer.Write([]byte(timeoutNotice))
			flusher.Flush()

			// 发送[DONE]事件标记结束
			c.Writer.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
		}
	} else {
		logger.Error("流式响应错误: %v", err)
	}

	// 统计请求数据
	// 确保token统计至少有一个合理的最小值
	if totalTokens < eventCount/4 {
		// 如果计算的token异常少，使用事件数作为保底估计
		minTokens := eventCount / 4 // 保守估计每4个事件至少1个token
		logger.Info("Token估计值(%d)过低，调整为基于事件数的保底估计: %d", totalTokens, minTokens)
		totalTokens = minTokens
	}

	// 记录最终使用的token数
	var tokenSource string
	if totalTokens > 0 {
		tokenSource = "API返回或有效估算"
	} else {
		tokenSource = "基于事件的保底估算"
	}
	logger.Info("流式响应最终统计: total_tokens=%d (来源: %s)",
		totalTokens, tokenSource)

	config.AddKeyRequestStat(apiKey, 1, totalTokens)

	// 更新每日统计数据
	modelNameForStats := "unknown"
	// 尝试从请求体中提取模型名称
	if requestData != nil {
		if model, ok := requestData["model"].(string); ok && model != "" {
			modelNameForStats = model
		}
	}

	// 计算prompt和completion的分配比例
	promptTokensCount := totalTokens / 3                     // 估计输入占1/3
	completionTokensCount := totalTokens - promptTokensCount // 估计输出占2/3

	// 添加到每日统计
	config.AddDailyRequestStat(apiKey, modelNameForStats, 1, promptTokensCount, completionTokensCount, true)

	logger.Info("流式响应完成，总tokens=%d (prompt=%d, completion=%d)，处理了 %d 个事件",
		totalTokens, promptTokensCount, completionTokensCount, eventCount)

	// 确保响应已经完成并标记为结束
	// 检查是否已经发送了[DONE]事件，如果没有，发送一个
	if !bytes.Contains(buffer.Bytes(), []byte("data: [DONE]")) && !connectionClosed.Load() {
		// 发送最终的[DONE]事件
		logger.Info("发送最终的[DONE]事件以确保客户端知道流已结束")
		c.Writer.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}

	// 设置响应完成标志，防止后续请求误判为403
	// 注意：这里由于客户端可能在流式响应完成后自动发送结束请求，需要确保这个请求不会被错误处理
	c.Set("stream_completed", true)
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

			// 首先尝试直接获取total_tokens
			if tt, ok := usage["total_tokens"].(float64); ok {
				// 如果没有详细的提示和完成令牌数，估算分配
				promptTokens = int(tt) / 3                // 估算提示占1/3
				completionTokens = int(tt) - promptTokens // 估算完成占2/3

				// 然后尝试获取更精确的提示和完成令牌数
				if pt, ok := usage["prompt_tokens"].(float64); ok {
					promptTokens = int(pt)
				}

				if ct, ok := usage["completion_tokens"].(float64); ok {
					completionTokens = int(ct)
				}

				return promptTokens, completionTokens
			}

			// 如果没有找到total_tokens，尝试使用原来的方法
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

// forwardUserInfoRequest 处理用户信息请求
func forwardUserInfoRequest(c *gin.Context, targetURL string) {
	// 获取最佳API密钥
	apiKey, err := key.GetBestKeyForRequest("user_info", "", 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "No suitable API keys available",
		})
		return
	}

	// 创建新的请求
	req, err := http.NewRequest(c.Request.Method, targetURL, nil)
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
	utils.SetCommonHeaders(req, apiKey)

	// 创建 HTTP 客户端
	client := utils.CreateClient()

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
	logger.InfoWithKey(maskedKey, "用户信息请求: %s %s", c.Request.Method, c.Request.URL.Path)

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

	// 如果请求失败，返回错误
	if !success {
		// 更新密钥失败记录
		key.UpdateApiKeyStatus(apiKey, false)
		c.JSON(resp.StatusCode, gin.H{
			"error": fmt.Sprintf("API请求失败，状态码: %d", resp.StatusCode),
		})
		return
	}

	// 更新密钥状态
	key.UpdateApiKeyStatus(apiKey, success)

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
