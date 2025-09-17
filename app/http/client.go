package http

import (
	"USDTMore/app/config"
	"USDTMore/app/log"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HTTPClient 统一的HTTP客户端配置
type HTTPClient struct {
	client           *http.Client
	EtherscanLimiter *EtherscanRateLimiter
	sessionWarmedUp  map[string]bool // 记录已预热的域名
	sessionMutex     sync.RWMutex    // 保护sessionWarmedUp的并发访问
}

// NewHTTPClient 创建新的HTTP客户端，包含连接池和超时配置
func NewHTTPClient() *HTTPClient {
	// 从配置文件获取超时时间，针对Etherscan API优化
	httpTimeout := time.Duration(config.GetHttpTimeout()) * time.Second

	// 创建Cookie jar以支持会话管理
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Error("创建Cookie jar失败:", err)
		jar = nil
	}

	transport := &http.Transport{
		// 连接池配置 - 针对Etherscan API优化
		MaxIdleConns:        50,                // 减少最大空闲连接数，避免触发限制
		MaxIdleConnsPerHost: 5,                 // 每个主机的最大空闲连接数，避免被检测为爬虫
		MaxConnsPerHost:     20,                // 每个主机的最大连接数，避免过多并发
		IdleConnTimeout:     120 * time.Second, // 空闲连接超时时间

		// 连接超时配置 - 大幅增加超时时间以应对Etherscan API
		DialContext: (&net.Dialer{
			Timeout:   60 * time.Second, // 连接超时增加到60秒
			KeepAlive: 60 * time.Second, // Keep-Alive时间增加
		}).DialContext,

		// TLS和HTTP配置 - 大幅增加超时时间
		TLSHandshakeTimeout:   45 * time.Second,  // TLS握手超时增加到45秒
		ResponseHeaderTimeout: 150 * time.Second, // 响应头超时时间增加到150秒
		ExpectContinueTimeout: 2 * time.Second,

		// 启用HTTP/2但允许降级到HTTP/1.1
		ForceAttemptHTTP2: true,

		// 启用压缩以模拟浏览器行为
		DisableCompression: false,

		// 禁用连接复用以避免被检测（针对Etherscan）
		DisableKeepAlives: false, // 保持Keep-Alive，但限制连接数
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   httpTimeout, // 使用配置的超时时间
		Jar:       jar,         // 启用Cookie支持

		// 自动处理重定向
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 最多允许10次重定向
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	return &HTTPClient{
		client:           client,
		EtherscanLimiter: NewEtherscanRateLimiter(),
		sessionWarmedUp:  make(map[string]bool),
	}
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

// Get 执行GET请求，包含重试机制和Etherscan会话预热
func (c *HTTPClient) Get(url string, headers map[string]string, maxRetries int) (*http.Response, error) {
	// 检查是否为Etherscan域名，如果是则先预热会话
	if isEtherscan, domain := c.isEtherscanDomain(url); isEtherscan {
		if err := c.WarmupEtherscanSession(domain); err != nil {
			log.Warn(fmt.Sprintf("预热 %s 会话失败: %v", domain, err))
		}

		// 应用Etherscan API限流
		c.EtherscanLimiter.Wait()
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建GET请求失败: %w", err)
	}

	// 为Etherscan请求设置浏览器头部
	if isEtherscan, _ := c.isEtherscanDomain(url); isEtherscan {
		c.setBrowserHeaders(req)
	}

	// 设置用户自定义请求头（会覆盖默认头部）
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

// WarmupEtherscanSession 预热Etherscan会话，获取必要的cookies（特别是cf_clearance）
func (c *HTTPClient) WarmupEtherscanSession(domain string) error {
	c.sessionMutex.RLock()
	if c.sessionWarmedUp[domain] {
		c.sessionMutex.RUnlock()
		return nil // 已经预热过
	}
	c.sessionMutex.RUnlock()

	c.sessionMutex.Lock()
	defer c.sessionMutex.Unlock()

	// 双重检查
	if c.sessionWarmedUp[domain] {
		return nil
	}

	log.Info(fmt.Sprintf("开始预热 %s 会话...", domain))

	// 优化预热过程，重点获取Cloudflare验证cookie
	steps := []string{
		fmt.Sprintf("https://%s", domain), // 主页 - 关键步骤，获取cf_clearance
	}

	for i, stepURL := range steps {
		// 创建预热请求
		req, err := http.NewRequest("GET", stepURL, nil)
		if err != nil {
			log.Warn(fmt.Sprintf("创建预热请求失败 (步骤 %d): %v", i+1, err))
			continue
		}

		// 使用与API请求相同的头部，确保一致性
		c.setBrowserHeaders(req)

		// 执行预热请求
		resp, err := c.client.Do(req)
		if err != nil {
			log.Warn(fmt.Sprintf("预热请求失败 (步骤 %d): %v", i+1, err))
			continue
		}

		// 检查是否获取到关键cookie
		if c.client.Jar != nil {
			u, _ := url.Parse(stepURL)
			cookies := c.client.Jar.Cookies(u)
			hasCfClearance := false
			for _, cookie := range cookies {
				if cookie.Name == "cf_clearance" {
					hasCfClearance = true
					log.Info(fmt.Sprintf("成功获取Cloudflare验证cookie: %s", cookie.Name))
					break
				}
			}
			if !hasCfClearance {
				log.Warn("未获取到cf_clearance cookie，可能影响后续API请求")
			}
		}

		// 读取响应体（模拟浏览器行为）
		_, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Warn(fmt.Sprintf("读取预热响应失败 (步骤 %d): %v", i+1, err))
		}

		// 等待Cloudflare验证完成
		time.Sleep(3 * time.Second)
	}

	// 标记为已预热
	c.sessionWarmedUp[domain] = true
	log.Info(fmt.Sprintf("%s 会话预热完成", domain))

	return nil
}

// setBrowserHeadersForDocument 设置文档请求的浏览器头部
func (c *HTTPClient) setBrowserHeadersForDocument(req *http.Request) {
	headers := map[string]string{
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8",
		"Accept-Encoding":           "gzip, deflate, br",
		"DNT":                       "1",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Cache-Control":             "max-age=0",
		"sec-ch-ua":                 `"Google Chrome";v="119", "Chromium";v="119", "Not?A_Brand";v="24"`,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        `"Windows"`,
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// setBrowserHeadersForNavigate 设置导航请求的浏览器头部
func (c *HTTPClient) setBrowserHeadersForNavigate(req *http.Request, referer string) {
	headers := map[string]string{
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8",
		"Accept-Encoding":           "gzip, deflate, br",
		"DNT":                       "1",
		"Connection":                "keep-alive",
		"Upgrade-Insecure-Requests": "1",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "same-origin",
		"Cache-Control":             "max-age=0",
		"Referer":                   referer,
		"sec-ch-ua":                 `"Google Chrome";v="119", "Chromium";v="119", "Not?A_Brand";v="24"`,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        `"Windows"`,
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// setBrowserHeaders 设置完整的浏览器特征头部
func (c *HTTPClient) setBrowserHeaders(req *http.Request) {
	// 完全匹配成功curl请求的头部
	headers := map[string]string{
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
		"Accept-Encoding":           "gzip, deflate, br",
		"DNT":                       "1",
		"Cache-Control":             "max-age=0",
		"Priority":                  "u=0, i",
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
		"sec-ch-ua":                 `"Chromium";v="140", "Not=A?Brand";v="24", "Microsoft Edge";v="140"`,
		"sec-ch-ua-mobile":          "?0",
		"sec-ch-ua-platform":        `"Windows"`,
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// isEtherscanDomain 检查是否为Etherscan相关域名
func (c *HTTPClient) isEtherscanDomain(urlStr string) (bool, string) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false, ""
	}

	domain := u.Host
	etherscanDomains := []string{
		"api.etherscan.io",
		"api-sepolia.etherscan.io",
		"api-holesky.etherscan.io",
		"api.bscscan.com",
		"api-testnet.bscscan.com",
		"api.polygonscan.com",
		"api-testnet.polygonscan.com",
		"api.arbiscan.io",
		"api-sepolia.arbiscan.io",
		"api.optimistic.etherscan.io",
		"api-sepolia-optimistic.etherscan.io",
		"api.basescan.org",
		"api-sepolia.basescan.org",
	}

	for _, ethDomain := range etherscanDomains {
		if strings.Contains(domain, ethDomain) {
			return true, ethDomain
		}
	}

	return false, ""
}

// 全局HTTP客户端实例
var DefaultClient = NewHTTPClient()
