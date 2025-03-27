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
