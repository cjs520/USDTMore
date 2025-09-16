package http

import (
	"USDTMore/app/config"
	"USDTMore/app/log"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

// HTTPClient 统一的HTTP客户端配置
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient 创建新的HTTP客户端，包含连接池和超时配置
func NewHTTPClient() *HTTPClient {
	// 从配置文件获取超时时间
	httpTimeout := time.Duration(config.GetHttpTimeout()) * time.Second

	transport := &http.Transport{
		// 连接池配置
		MaxIdleConns:        100,              // 最大空闲连接数
		MaxIdleConnsPerHost: 10,               // 每个主机的最大空闲连接数
		MaxConnsPerHost:     50,               // 每个主机的最大连接数
		IdleConnTimeout:     90 * time.Second, // 空闲连接超时时间

		// 连接超时配置 - 增加超时时间以应对慢速API
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second, // 连接超时增加到30秒
			KeepAlive: 30 * time.Second, // Keep-Alive时间
		}).DialContext,

		// TLS和HTTP配置 - 增加超时时间
		TLSHandshakeTimeout:   20 * time.Second, // TLS握手超时增加到20秒
		ResponseHeaderTimeout: 90 * time.Second, // 响应头超时时间增加到90秒
		ExpectContinueTimeout: 1 * time.Second,

		// 启用HTTP/2但允许降级到HTTP/1.1
		ForceAttemptHTTP2: true,

		// 禁用压缩以避免某些API的兼容性问题
		DisableCompression: false,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   httpTimeout, // 使用配置的超时时间
	}

	return &HTTPClient{client: client}
}

// DoWithRetry 执行HTTP请求，包含重试机制
func (c *HTTPClient) DoWithRetry(req *http.Request, maxRetries int) (*http.Response, error) {
	var lastErr error

	// 记录请求日志（如果启用）
	if config.IsRequestLogEnabled() {
		log.Info(fmt.Sprintf("HTTP请求: %s %s", req.Method, req.URL.String()))
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 克隆请求以支持重试
		reqClone := req.Clone(context.Background())

		startTime := time.Now()
		resp, err := c.client.Do(reqClone)
		duration := time.Since(startTime)

		if err == nil {
			// 记录响应时间（如果启用日志）
			if config.IsRequestLogEnabled() {
				log.Info(fmt.Sprintf("HTTP响应: %s %s - %d - %v", req.Method, req.URL.String(), resp.StatusCode, duration))
			}

			// 检查HTTP状态码
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return resp, nil
			}

			// 对于客户端错误（4xx），不重试
			if resp.StatusCode >= 400 && resp.StatusCode < 500 {
				resp.Body.Close()
				return nil, fmt.Errorf("HTTP客户端错误: %d", resp.StatusCode)
			}

			// 对于服务器错误（5xx），记录错误并重试
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP服务器错误: %d", resp.StatusCode)
		} else {
			lastErr = err
			if config.IsRequestLogEnabled() {
				log.Error(fmt.Sprintf("HTTP请求失败: %s %s - %v - %v", req.Method, req.URL.String(), err, duration))
			}
		}

		// 如果不是最后一次尝试，等待后重试
		if attempt < maxRetries {
			// 检查错误是否可重试
			if !IsRetryableError(lastErr) {
				if config.IsRequestLogEnabled() {
					log.Info(fmt.Sprintf("错误不可重试，停止重试: %s", req.URL.String()))
				}
				break
			}

			// 使用智能延迟算法
			waitTime := GetRetryDelay(lastErr, attempt)
			if config.IsRequestLogEnabled() {
				log.Info(fmt.Sprintf("重试请求 %d/%d，等待 %v: %s", attempt+1, maxRetries, waitTime, req.URL.String()))
			}
			time.Sleep(waitTime)
		}
	}

	return nil, fmt.Errorf("请求失败，已重试%d次: %w", maxRetries, lastErr)
}

// Get 执行GET请求，包含重试机制
func (c *HTTPClient) Get(url string, headers map[string]string, maxRetries int) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建GET请求失败: %w", err)
	}

	// 设置请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.DoWithRetry(req, maxRetries)
}

// Post 执行POST请求，包含重试机制
func (c *HTTPClient) Post(url string, body io.Reader, headers map[string]string, maxRetries int) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("创建POST请求失败: %w", err)
	}

	// 设置请求头
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return c.DoWithRetry(req, maxRetries)
}

// GetBody 获取响应体内容并自动关闭
func GetResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	return body, nil
}

// 全局HTTP客户端实例
var DefaultClient = NewHTTPClient()
