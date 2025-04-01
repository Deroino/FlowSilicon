/**
  @author: Hanhai
  @desc: API错误处理模块，定义API错误类型和相关方法
**/

package proxy

import "fmt"

// ApiError 定义API错误类型
type ApiError struct {
	Message string
	Code    int
}

// Error 实现error接口
func (e *ApiError) Error() string {
	return fmt.Sprintf("%s (code: %d)", e.Message, e.Code)
}
