/**
  @author: Hanhai
  @desc: handler中使用新日志系统的示例
**/

package proxy

import (
	"flowsilicon/internal/logger"
	"flowsilicon/pkg/utils"
	"time"
	
	"github.com/gin-gonic/gin"
)

// 示例：如何在现有handler中集成新的日志系统
func exampleHandlerWithLogging(c *gin.Context) {
	// 获取请求日志记录器
	rl := GetRequestLogger(c)
	
	// 获取时间追踪器
	tracker := GetTimeTracker(c)
	
	// 设置额外信息
	rl.SetModel("gpt-3.5-turbo").
		SetExtra("user_id", "12345").
		SetExtra("token_estimate", 1000)
	
	// 步骤1：验证请求
	rl.Info("开始验证请求参数")
	tracker.Step("验证请求")
	
	// ... 验证逻辑 ...
	
	// 步骤2：选择API密钥
	rl.Info("选择最佳API密钥")
	tracker.Step("选择密钥")
	
	apiKey := "sk-xxx..."
	maskedKey := utils.MaskKey(apiKey)
	rl.SetExtra("api_key", maskedKey)
	
	// 步骤3：发送请求
	rl.Info("发送请求到上游服务")
	tracker.Step("发送请求")
	
	// 模拟请求
	time.Sleep(2 * time.Second)
	
	// 步骤4：处理响应
	tracker.Step("处理响应")
	
	// 如果出错
	if false { // 示例条件
		rl.ErrorWithDuration("请求失败: %v", "timeout")
		return
	}
	
	// 成功完成
	rl.InfoWithDuration("请求处理成功")
	
	// 记录详细的时间步骤（用于调试）
	if tracker.GetTotalDuration() > 3*time.Second {
		tracker.LogSteps()
	}
}

// 示例：改进后的 processApiRequest
func processApiRequestWithLogging(c *gin.Context, targetURL string, bodyBytes []byte, requestType string, modelName string, tokenEstimate int) (bool, error) {
	// 获取日志记录器
	rl := GetRequestLogger(c)
	tracker := GetTimeTracker(c)
	
	// 更新日志上下文
	rl.SetModel(modelName).
		SetExtra("request_type", requestType).
		SetExtra("token_estimate", tokenEstimate).
		SetExtra("target_url", targetURL)
	
	// 记录开始处理
	rl.Info("开始处理API请求 - 类型: %s, 模型: %s", requestType, modelName)
	
	// 时间追踪
	tracker.Step("准备请求")
	
	// ... 原有的请求处理逻辑 ...
	
	// 发送请求前
	tracker.Step("发送HTTP请求")
	
	// 发送请求
	// resp, err := client.Do(req)
	
	// 请求完成后
	tracker.Step("读取响应")
	
	// 处理完成
	duration := tracker.GetTotalDuration()
	
	// 根据结果记录不同级别的日志
	if duration > 10*time.Second {
		rl.WarnWithDuration("请求耗时过长")
	} else {
		rl.InfoWithDuration("请求完成")
	}
	
	return true, nil
}

// StartPerformanceReporter 定期输出性能报告
func StartPerformanceReporter() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			// 输出性能摘要
			logger.LogPerformanceSummary()
			
			// 重置计数器（可选）
			// logger.ResetPerformanceMetrics()
		}
	}()
}