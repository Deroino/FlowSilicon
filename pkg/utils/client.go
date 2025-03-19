/**
  @author: Hanhai
  @since: 2025/3/18 23:50:00
  @desc: HTTP代理
**/

package utils

import (
	"flowsilicon/internal/config"
	"flowsilicon/internal/logger"
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

// CreateDeepseekClient 创建专门为Deepseek R1模型优化的HTTP客户端
func CreateDeepseekClient() *http.Client {
	// 创建自定义传输层
	transport := &http.Transport{
		// TCP连接设置
		DialContext: (&net.Dialer{
			Timeout:   180 * time.Second, // 连接超时增加到3分钟
			KeepAlive: 180 * time.Second, // 更激进的保活
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          200,               // 增加空闲连接数
		MaxIdleConnsPerHost:   50,                // 增加每主机空闲连接数
		IdleConnTimeout:       60 * time.Minute,  // 延长空闲连接超时到60分钟
		ResponseHeaderTimeout: 30 * time.Minute,  // 响应头超时增加到30分钟
		ExpectContinueTimeout: 5 * time.Second,   // 允许更长的初始响应等待
		TLSHandshakeTimeout:   180 * time.Second, // TLS握手超时增加到3分钟
		// 禁用HTTP/2.0压缩，但启用HTTP/2.0本身
		DisableCompression: true,
		// 强制尝试使用HTTP/2.0
		ForceAttemptHTTP2: true,
		// 启用TCP保活
		DisableKeepAlives: false,
		// 写入缓冲区大小
		WriteBufferSize: 262144, // 256KB
		// 读取缓冲区大小
		ReadBufferSize: 262144, // 256KB
		// 设置更长的正文读取限制
		MaxResponseHeaderBytes: 64 << 10, // 64KB
	}

	// 创建自定义客户端
	client := &http.Client{
		// 设置一个超长的超时（4小时）
		Timeout:   240 * time.Minute,
		Transport: transport,
	}

	logger.Info("已创建超长超时的Deepseek客户端，超时时间: 240分钟(4小时)，TCP保活: 180秒，缓冲区: 256KB")
	return client
}

// CreateClientWithTimeout 创建配置了代理的HTTP客户端，使用指定超时时间
func CreateClientWithTimeout(timeout time.Duration) *http.Client {
	// 获取配置
	cfg := config.GetConfig()

	// 创建Transport
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		// 添加TCP连接的保持活动设置
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		// 增加TLS握手超时
		TLSHandshakeTimeout: 30 * time.Second,
		// 响应体超时
		ResponseHeaderTimeout: 60 * time.Second,
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
				transport.DialContext = dialer.(proxy.ContextDialer).DialContext
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
