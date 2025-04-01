/**
  @author: Hanhai
  @desc: 认证中间件，用于验证用户身份并管理登录状态
**/

package middleware

import (
	"flowsilicon/internal/auth"
	"flowsilicon/internal/config"
	"flowsilicon/internal/logger"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 认证中间件常量
const (
	AuthCookieName = "flowsilicon_auth"
)

// AuthMiddleware 检查请求是否包含有效的认证标记
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取当前配置
		cfg := config.GetConfig()
		if cfg == nil {
			logger.Error("无法获取系统配置")
			c.JSON(http.StatusInternalServerError, gin.H{
				"code":    500,
				"message": "服务器内部错误",
			})
			c.Abort()
			return
		}

		// 检查是否启用了密码保护
		if !cfg.Security.PasswordEnabled {
			// 未启用密码保护，直接放行
			c.Next()
			return
		}

		// 检查白名单路径，如登录页面和登录API
		if isWhitelistPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		// 从Cookie中获取令牌
		cookie, err := c.Cookie(AuthCookieName)
		if err != nil || cookie == "" {
			logger.Info("用户未认证，重定向到登录页面: %s", c.Request.URL.Path)

			// 如果是API请求，返回401错误
			if strings.HasPrefix(c.Request.URL.Path, "/api/") ||
				strings.HasPrefix(c.Request.URL.Path, "/settings/") ||
				strings.HasPrefix(c.Request.URL.Path, "/keys/") {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "请先登录",
				})
				c.Abort()
				return
			}

			// 否则重定向到登录页面
			// 保存原始请求路径，以便登录后重定向回来
			c.SetCookie("redirect_after_login", c.Request.URL.Path, 300, "/", "", false, false)
			// 重定向到登录页面
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// 验证令牌
		valid, err := auth.ParseCookie(cookie)
		if err != nil || !valid {
			logger.Info("无效的认证令牌: %v", err)

			// 清除无效的Cookie
			c.SetCookie(AuthCookieName, "", -1, "/", "", false, true)

			// 如果是API请求，返回401错误
			if strings.HasPrefix(c.Request.URL.Path, "/api/") ||
				strings.HasPrefix(c.Request.URL.Path, "/settings/") ||
				strings.HasPrefix(c.Request.URL.Path, "/keys/") {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    401,
					"message": "认证已过期，请重新登录",
				})
				c.Abort()
				return
			}

			// 保存原始请求路径，以便登录后重定向回来
			c.SetCookie("redirect_after_login", c.Request.URL.Path, 300, "/", "", false, false)
			// 重定向到登录页面
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}

		// 认证通过，继续处理请求
		c.Next()
	}
}

// isWhitelistPath 检查路径是否在白名单中
func isWhitelistPath(path string) bool {
	// 白名单路径列表
	whitelist := []string{
		"/login",       // 登录页面
		"/auth/login",  // 登录API
		"/auth/check",  // 认证检查API
		"/static/",     // 静态资源
		"/static-fs/",  // 嵌入式静态资源
		"/favicon.ico", // 网站图标
	}

	for _, prefix := range whitelist {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}
