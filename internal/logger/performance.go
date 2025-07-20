/**
  @author: Hanhai
  @desc: 性能监控日志，专门记录请求耗时和性能指标
**/

package logger

import (
	"fmt"
	"sync"
	"time"
)

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	RequestCount      int64         // 请求总数
	SuccessCount      int64         // 成功请求数
	FailedCount       int64         // 失败请求数
	TotalDuration     time.Duration // 总耗时
	MaxDuration       time.Duration // 最大耗时
	MinDuration       time.Duration // 最小耗时
	SlowRequestCount  int64         // 慢请求数量（超过阈值）
	SlowRequestThreshold time.Duration // 慢请求阈值
}

var (
	perfMetrics      = &PerformanceMetrics{
		MinDuration: time.Hour, // 初始化为很大的值
		SlowRequestThreshold: 5 * time.Second, // 默认5秒为慢请求
	}
	perfMutex sync.RWMutex
)

// SetSlowRequestThreshold 设置慢请求阈值
func SetSlowRequestThreshold(threshold time.Duration) {
	perfMutex.Lock()
	defer perfMutex.Unlock()
	perfMetrics.SlowRequestThreshold = threshold
}

// RecordRequestMetrics 记录请求性能指标
func RecordRequestMetrics(duration time.Duration, success bool) {
	perfMutex.Lock()
	defer perfMutex.Unlock()
	
	perfMetrics.RequestCount++
	perfMetrics.TotalDuration += duration
	
	if success {
		perfMetrics.SuccessCount++
	} else {
		perfMetrics.FailedCount++
	}
	
	if duration > perfMetrics.MaxDuration {
		perfMetrics.MaxDuration = duration
	}
	
	if duration < perfMetrics.MinDuration {
		perfMetrics.MinDuration = duration
	}
	
	if duration > perfMetrics.SlowRequestThreshold {
		perfMetrics.SlowRequestCount++
	}
}

// GetPerformanceMetrics 获取性能指标快照
func GetPerformanceMetrics() PerformanceMetrics {
	perfMutex.RLock()
	defer perfMutex.RUnlock()
	
	return *perfMetrics
}

// ResetPerformanceMetrics 重置性能指标
func ResetPerformanceMetrics() {
	perfMutex.Lock()
	defer perfMutex.Unlock()
	
	perfMetrics = &PerformanceMetrics{
		MinDuration: time.Hour,
		SlowRequestThreshold: perfMetrics.SlowRequestThreshold, // 保留阈值设置
	}
}

// LogPerformanceSummary 记录性能摘要
func LogPerformanceSummary() {
	metrics := GetPerformanceMetrics()
	
	if metrics.RequestCount == 0 {
		Info("性能摘要: 暂无请求数据")
		return
	}
	
	avgDuration := metrics.TotalDuration / time.Duration(metrics.RequestCount)
	successRate := float64(metrics.SuccessCount) / float64(metrics.RequestCount) * 100
	
	Info("=== 性能监控摘要 ===")
	Info("请求总数: %d, 成功: %d, 失败: %d, 成功率: %.2f%%", 
		metrics.RequestCount, metrics.SuccessCount, metrics.FailedCount, successRate)
	Info("平均耗时: %v, 最大耗时: %v, 最小耗时: %v", 
		avgDuration, metrics.MaxDuration, metrics.MinDuration)
	Info("慢请求数量: %d (阈值: %v)", metrics.SlowRequestCount, metrics.SlowRequestThreshold)
	Info("==================")
}

// TimeTracker 时间追踪器
type TimeTracker struct {
	name      string
	startTime time.Time
	steps     []TimeStep
	mu        sync.Mutex
}

// TimeStep 时间步骤
type TimeStep struct {
	Name     string
	Duration time.Duration
	Time     time.Time
}

// NewTimeTracker 创建新的时间追踪器
func NewTimeTracker(name string) *TimeTracker {
	return &TimeTracker{
		name:      name,
		startTime: time.Now(),
		steps:     make([]TimeStep, 0),
	}
}

// Step 记录一个步骤的耗时
func (tt *TimeTracker) Step(stepName string) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	
	now := time.Now()
	var duration time.Duration
	
	if len(tt.steps) == 0 {
		duration = now.Sub(tt.startTime)
	} else {
		duration = now.Sub(tt.steps[len(tt.steps)-1].Time)
	}
	
	tt.steps = append(tt.steps, TimeStep{
		Name:     stepName,
		Duration: duration,
		Time:     now,
	})
}

// LogSteps 记录所有步骤的耗时
func (tt *TimeTracker) LogSteps() {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	
	if len(tt.steps) == 0 {
		return
	}
	
	totalDuration := time.Since(tt.startTime)
	
	logStr := fmt.Sprintf("时间追踪 [%s] - 总耗时: %v", tt.name, totalDuration)
	
	for i, step := range tt.steps {
		logStr += fmt.Sprintf("\n  步骤%d [%s]: %v", i+1, step.Name, step.Duration)
	}
	
	Info(logStr)
}

// GetTotalDuration 获取总耗时
func (tt *TimeTracker) GetTotalDuration() time.Duration {
	return time.Since(tt.startTime)
}

// LogSlowRequest 记录慢请求详情
func LogSlowRequest(requestID, path string, duration time.Duration, details map[string]interface{}) {
	Warn("慢请求检测 - RequestID: %s, Path: %s, Duration: %v", requestID, path, duration)
	
	if len(details) > 0 {
		detailStr := "详细信息: "
		for k, v := range details {
			detailStr += fmt.Sprintf("%s=%v, ", k, v)
		}
		Warn(detailStr)
	}
}