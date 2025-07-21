/**
  @author: Hanhai
  @desc: HTTP代理
**/

package utils

import (
	"flowsilicon/internal/config"
	"flowsilicon/internal/logger"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// CreateClient 创建配置了代理的HTTP客户端，默认60秒超时
func CreateClient() *http.Client {
	return CreateClientWithTimeout(60 * time.Second)
}

// CreateClientWithTimeout 创建配置了代理的HTTP客户端，使用指定超时时间
func CreateClientWithTimeout(timeout time.Duration) *http.Client {
	// 获取配置
	cfg := config.GetConfig()

	// 创建Transport
	transport := &http.Transport{
		MaxIdleConns:        cfg.RequestSettings.HttpClient.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.RequestSettings.HttpClient.MaxIdleConnsPerHost,
		IdleConnTimeout:     time.Duration(cfg.RequestSettings.HttpClient.IdleConnTimeout) * time.Second,
		// 添加TCP连接的保持活动设置
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(cfg.RequestSettings.HttpClient.ConnectTimeout) * time.Second,
			KeepAlive: time.Duration(cfg.RequestSettings.HttpClient.KeepAlive) * time.Second,
		}).DialContext,
		// 增加TLS握手超时
		TLSHandshakeTimeout: time.Duration(cfg.RequestSettings.HttpClient.TLSHandshakeTimeout) * time.Second,
		// 响应体超时
		ResponseHeaderTimeout: time.Duration(cfg.RequestSettings.HttpClient.ResponseHeaderTimeout) * time.Second,
		// 启用HTTP/2.0
		ForceAttemptHTTP2: true,
	}

	// 如果启用了代理，设置代理
	if cfg.Proxy.Enabled {
		if cfg.Proxy.ProxyType == "socks5" && cfg.Proxy.SocksProxy != "" {
			// 使用SOCKS5代理
			logger.Info("使用SOCKS5代理: %s", cfg.Proxy.SocksProxy)

			// 创建SOCKS5代理拨号器
			dialer, err := proxy.SOCKS5("tcp", cfg.Proxy.SocksProxy, nil, proxy.Direct)
			if err != nil {
				logger.Error("创建SOCKS5代理拨号器失败: %v", err)
			} else {
				// 设置自定义拨号函数
				if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
					transport.DialContext = contextDialer.DialContext
				} else {
					logger.Error("无法将代理转换为ContextDialer")
				}
			}
		} else {
			// 使用HTTP/HTTPS代理
			proxyFunc := func(req *http.Request) (*url.URL, error) {
				// 根据请求协议选择代理
				if req.URL.Scheme == "https" && cfg.Proxy.HttpsProxy != "" {
					logger.Info("使用HTTPS代理: %s", cfg.Proxy.HttpsProxy)
					return url.Parse(cfg.Proxy.HttpsProxy)
				}
				if req.URL.Scheme == "http" && cfg.Proxy.HttpProxy != "" {
					logger.Info("使用HTTP代理: %s", cfg.Proxy.HttpProxy)
					return url.Parse(cfg.Proxy.HttpProxy)
				}
				return nil, nil // 不使用代理
			}
			transport.Proxy = proxyFunc
		}
	}

	// 创建并返回客户端
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

// SetCommonHeaders 设置HTTP请求的通用头部
// 包括Authorization、Content-Type和Accept-Encoding
func SetCommonHeaders(req *http.Request, token string) {
	// 设置授权头
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	// 设置内容类型
	req.Header.Set("Content-Type", "application/json")
	// 设置Accept-Encoding为identity，解决Cloudflare转发时的乱码问题
	req.Header.Set("Accept-Encoding", "identity")
}

// SetInferenceModelHeaders 设置推理模型HTTP请求的特殊头部
// 适用于类型为7的推理模型，包括DeepseekR1等需要特殊处理的模型
func SetInferenceModelHeaders(req *http.Request) {
	// 禁用Nginx缓冲
	req.Header.Set("X-Accel-Buffering", "no")
	// 设置缓存控制
	req.Header.Set("Cache-Control", "no-cache, no-transform")
	// 保持连接
	req.Header.Set("Connection", "keep-alive")
	// 分块传输编码
	req.Header.Set("Transfer-Encoding", "chunked")
	// 设置较长的Keep-Alive超时
	req.Header.Set("Keep-Alive", "timeout=600")
	// 设置高优先级（可能被某些服务忽略，但不影响）
	req.Header.Set("X-Inference-Priority", "high")
}

// CreateInferenceModelClient 创建适用于推理模型的HTTP客户端
// 使用更长的超时时间和更优化的连接设置
func CreateInferenceModelClient(requestTimeout time.Duration) *http.Client {
	// 获取配置
	cfg := config.GetConfig()

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(cfg.RequestSettings.HttpClient.ConnectTimeout) * time.Second, // 连接超时
			KeepAlive: time.Duration(cfg.RequestSettings.HttpClient.KeepAlive) * time.Second,     // 保持连接活跃
			DualStack: true,
		}).DialContext,
		MaxIdleConns:           cfg.RequestSettings.HttpClient.MaxIdleConns,                                                    // 最大空闲连接数
		IdleConnTimeout:        time.Duration(cfg.RequestSettings.HttpClient.IdleConnTimeout) * time.Second,                  // 空闲连接超时
		TLSHandshakeTimeout:    time.Duration(cfg.RequestSettings.HttpClient.TLSHandshakeTimeout) * time.Second,              // TLS握手超时
		ExpectContinueTimeout:  time.Duration(cfg.RequestSettings.HttpClient.ExpectContinueTimeout) * time.Second,            // 100-continue状态码的等待时间
		ResponseHeaderTimeout:  time.Duration(cfg.RequestSettings.HttpClient.ResponseHeaderTimeout) * time.Second,            // 响应头超时
		MaxResponseHeaderBytes: int64(cfg.RequestSettings.HttpClient.MaxResponseHeaderBytes),                                  // 最大响应头大小
	}

	return &http.Client{
		Transport: transport,
		// 客户端总超时设置的略大于上下文超时，让上下文控制主要超时行为
		Timeout: requestTimeout + 30*time.Second,
	}
}

// CreateStandardModelClient 创建适用于普通模型的HTTP客户端
// 使用标准的超时时间和连接设置
func CreateStandardModelClient(requestTimeout time.Duration) *http.Client {
	// 获取配置
	cfg := config.GetConfig()

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   time.Duration(cfg.RequestSettings.HttpClient.ConnectTimeout) * time.Second,
			KeepAlive: time.Duration(cfg.RequestSettings.HttpClient.KeepAlive) * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          cfg.RequestSettings.HttpClient.MaxIdleConns,
		IdleConnTimeout:       time.Duration(cfg.RequestSettings.HttpClient.IdleConnTimeout) * time.Second,
		TLSHandshakeTimeout:   time.Duration(cfg.RequestSettings.HttpClient.TLSHandshakeTimeout) * time.Second,
		ExpectContinueTimeout: time.Duration(cfg.RequestSettings.HttpClient.ExpectContinueTimeout) * time.Second,
		ResponseHeaderTimeout: time.Duration(cfg.RequestSettings.HttpClient.ResponseHeaderTimeout) * time.Second,
	}

	return &http.Client{
		Transport: transport,
		// 客户端总超时设置的略大于上下文超时，让上下文控制主要超时行为
		Timeout: requestTimeout + 10*time.Second,
	}
}

// SetStreamResponseHeaders 设置HTTP响应的流式响应头
// 用于SSE (Server-Sent Events) 流式输出
func SetStreamResponseHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")
}

// SetInferenceStreamResponseHeaders 设置推理模型HTTP响应的特殊流式响应头
// 为推理模型添加一些额外的响应头以改善性能
func SetInferenceStreamResponseHeaders(w http.ResponseWriter) {
	// 设置基本的流式响应头
	SetStreamResponseHeaders(w)
	// 添加推理模型特有的头部
	w.Header().Set("X-Accel-Buffering", "no") // 禁用nginx缓冲
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	// 长连接设置
	w.Header().Set("Keep-Alive", "timeout=600")
}
