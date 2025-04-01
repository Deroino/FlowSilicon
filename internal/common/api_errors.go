/**
  @author: Hanhai
  @desc: 公共 API 错误处理
**/

package common

import (
	"fmt"
)

// ApiError API错误结构
type ApiError struct {
	Message string
	Code    int
}

// Error 实现 error 接口
func (e *ApiError) Error() string {
	return fmt.Sprintf("API错误 %d: %s", e.Code, e.Message)
}

// NewApiError 创建一个新的API错误
func NewApiError(message string, code int) error {
	return &ApiError{
		Message: message,
		Code:    code,
	}
}

// ErrNoActiveKeys 没有可用的API密钥错误
var ErrNoActiveKeys = NewApiError("没有可用的API密钥", 500)
