/**
  @author: Hanhai
  @since: 2025/3/16 20:43:23
  @desc:
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
