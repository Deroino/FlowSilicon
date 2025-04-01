/**
  @author: Hanhai
  @desc: API密钥验证中间件，用于验证API调用的授权
**/

package middleware

import (
	"flowsilicon/internal/config"
	"flowsilicon/internal/logger"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// APIKeyMiddleware 检查API请求是否包含有效的API密钥
func APIKeyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取当前配置
		cfg := config.GetConfig()
		if cfg == nil {
			logger.Error("无法获取系统配置")
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": gin.H{
					"message": "服务器内部错误",
					"type":    "server_error",
					"code":    500,
				},
			})
			c.Abort()
			return
		}

		// 检查是否启用了API密钥验证
		if !cfg.Security.ApiKeyEnabled {
			// 未启用API密钥验证，直接放行
			c.Next()
			return
		}

		// 获取API密钥
		apiKey := extractAPIKey(c)
		if apiKey == "" {
			logger.Info("API请求未提供API密钥")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "请在Authorization头部提供有效的API密钥",
					"type":    "unauthorized",
					"code":    401,
				},
			})
			c.Abort()
			return
		}

		// 验证API密钥
		if apiKey != cfg.Security.ApiKey {
			logger.Info("API请求提供了无效的API密钥")
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": gin.H{
					"message": "无效的API密钥",
					"type":    "unauthorized",
					"code":    401,
				},
			})
			c.Abort()
			return
		}

		// API密钥验证通过，继续处理请求
		c.Next()
	}
}

// extractAPIKey 从请求中提取API密钥
func extractAPIKey(c *gin.Context) string {
	// 尝试从Authorization头部获取API密钥
	auth := c.GetHeader("Authorization")
	if auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
		return auth
	}

	// 尝试从查询参数获取API密钥
	apiKey := c.Query("api_key")
	if apiKey != "" {
		return apiKey
	}

	// 如果是POST请求，尝试从form参数获取API密钥
	if c.Request.Method == "POST" {
		apiKey = c.PostForm("api_key")
		if apiKey != "" {
			return apiKey
		}
	}

	return ""
}
