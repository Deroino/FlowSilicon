/**
  @author: AI
  @desc: CORS中间件，处理跨域请求
**/

package middleware

import (
	"github.com/gin-gonic/gin"
)

// CorsMiddleware 创建一个处理跨域请求的中间件
func CorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 允许的域名
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		// 允许的HTTP方法
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		// 允许的HTTP头
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		// 允许暴露的头信息
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		// 允许凭证，如cookie
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		// 处理预检请求
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// BalanceCorsMiddleware 专门为Balance相关URL路径创建的跨域中间件
func BalanceCorsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 判断是否是Balance相关路径
		if isBalanceRelatedPath(c.Request.URL.Path) {
			// 允许的域名
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			// 允许的HTTP方法
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
			// 允许的HTTP头
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
			// 允许暴露的头信息
			c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
			// 允许凭证，如cookie
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

			// 处理预检请求
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(204)
				return
			}
		}

		c.Next()
	}
}

// isBalanceRelatedPath 判断是否是Balance相关的路径
func isBalanceRelatedPath(path string) bool {
	// 这里列出所有与Balance查询相关的URL路径
	balancePaths := []string{
		"/keys/refresh",
		"/keys/balance",
		"/keys/status",
		"/api/key/balance",
		"/v1/user/info",
		"/api/keys",
		// 添加其他相关路径
	}

	for _, balancePath := range balancePaths {
		if path == balancePath || path == balancePath+"/" {
			return true
		}
	}

	return false
}
