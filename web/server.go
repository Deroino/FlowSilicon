/*
*

	@author: Hanhai
	@since: 2025/3/16 21:57:00
	@desc:

*
*/
package web

import (
	"embed"
	"flowsilicon/internal/proxy"
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"

	"flowsilicon/internal/config"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// SetupApiProxy 设置 API 代理路由
func SetupApiProxy(router *gin.Engine) {
	// 代理所有 API 请求
	router.Any("/api/*path", proxy.HandleApiProxy)

	// 添加对 OpenAI 格式 API 的支持
	router.Any("/v1/*path", proxy.HandleOpenAIProxy)
}

// SetupKeysAPI 设置API密钥相关路由
func SetupKeysAPI(router *gin.Engine) {
	// 获取当前请求统计
	router.GET("/request-stats/current", handleGetCurrentRequestStats)

	// 获取每日统计数据
	router.GET("/request-stats/daily", handleGetDailyStats)

	// 获取指定日期的统计数据
	router.GET("/request-stats/daily/:date", handleGetDailyStatsByDate)
}

// SetupWebServer 设置 Web 服务器
func SetupWebServer(router *gin.Engine) {
	// 加载模板
	templ := template.Must(template.New("").ParseFS(templatesFS, "templates/*.html"))
	router.SetHTMLTemplate(templ)

	// 静态文件 - 使用嵌入式文件系统
	router.StaticFS("/static", http.FS(staticFS))

	// 静态文件 - 直接从文件系统提供
	router.Static("/static-fs", "./web/static")

	// 首页
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title":                  config.GetConfig().App.Title,
			"max_balance_display":    config.GetConfig().App.MaxBalanceDisplay,
			"items_per_page":         config.GetConfig().App.ItemsPerPage,
			"auto_update_interval":   config.GetConfig().App.AutoUpdateInterval,
			"stats_refresh_interval": config.GetConfig().App.StatsRefreshInterval,
			"rate_refresh_interval":  config.GetConfig().App.RateRefreshInterval,
		})
	})

	// API 密钥管理
	router.GET("/keys", handleListKeys)
	router.POST("/keys", handleAddKey)
	router.DELETE("/keys/:key", handleDeleteKey)
	router.POST("/keys/batch", handleBatchAddKeys)
	router.POST("/keys/check", handleCheckKey)
	router.POST("/keys/mode", handleSetKeyMode)
	router.GET("/keys/mode", handleGetKeyMode)
	router.POST("/keys/:key/enable", handleEnableKey)
	router.POST("/keys/:key/disable", handleDisableKey)
	router.DELETE("/keys/zero-balance", handleDeleteZeroBalanceKeys)
	router.GET("/test-key", handleGetTestKey)

	// API 密钥统计
	router.GET("/stats", handleStats)

	// 日志查看
	router.GET("/logs", handleGetLogs)

	// 测试embeddings API
	router.POST("/test-chat", handleTestChat)

	// 测试embeddings API
	router.POST("/test-embeddings", handleTestEmbeddings)

	// 测试图片生成API
	router.POST("/test-images", handleTestImages)

	// 测试模型列表API
	router.POST("/test-models", handleTestModels)

	// 测试重排序API
	router.POST("/test-rerank", handleTestRerank)

	// 请求统计数据
	router.GET("/request-stats", handleRequestStats)
	// 注释掉重复的路由
	// router.GET("/request-stats/current", handleCurrentRequestStats)
}
