package http

import (
	"errors"
	"net"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// NetworkError 网络错误类型
type NetworkError struct {
	Type    string
	Message string
	Err     error
}

func (e *NetworkError) Error() string {
	return e.Message
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// IsRetryableError 判断错误是否可重试
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查网络错误类型
	var netErr net.Error
	if errors.As(err, &netErr) {
		// 超时错误可重试
		if netErr.Timeout() {
			return true
		}
		// 临时错误可重试
		if netErr.Temporary() {
			return true
		}
	}

	// 检查URL错误
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return IsRetryableError(urlErr.Err)
	}

	// 检查系统调用错误
	var syscallErr *syscall.Errno
	if errors.As(err, &syscallErr) {
		switch *syscallErr {
		case syscall.ECONNREFUSED, syscall.ECONNRESET, syscall.ETIMEDOUT:
			return true
		}
	}

	// 检查常见的可重试错误字符串
	errStr := strings.ToLower(err.Error())
	retryableErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"no such host",
		"network is unreachable",
		"temporary failure",
		"timeout",
		"deadline exceeded",
		"context deadline exceeded",
		"client.timeout exceeded",
		"awaiting headers",
		"too many requests",
		"rate limit",
		"service unavailable",
		"bad gateway",
		"gateway timeout",
		"server too busy",
		"unexpected error",
		"etherscan api错误",
		"api.etherscan.io",
		"502 bad gateway",
		"503 service unavailable",
		"504 gateway timeout",
	}

	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}

	return false
}

// ClassifyError 分类网络错误
func ClassifyError(err error) *NetworkError {
	if err == nil {
		return nil
	}

	errStr := strings.ToLower(err.Error())

	// 超时错误
	if strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") {
		return &NetworkError{
			Type:    "timeout",
			Message: "请求超时，请检查网络连接和服务器状态",
			Err:     err,
		}
	}

	// 连接错误
	if strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") {
		return &NetworkError{
			Type:    "connection",
			Message: "连接被拒绝或重置，请检查服务器状态",
			Err:     err,
		}
	}

	// DNS错误
	if strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "dns") {
		return &NetworkError{
			Type:    "dns",
			Message: "域名解析失败，请检查DNS设置",
			Err:     err,
		}
	}

	// 限流错误
	if strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "rate limit") {
		return &NetworkError{
			Type:    "rate_limit",
			Message: "请求频率过高，已触发API限流",
			Err:     err,
		}
	}

	// 服务器错误
	if strings.Contains(errStr, "service unavailable") ||
		strings.Contains(errStr, "bad gateway") ||
		strings.Contains(errStr, "gateway timeout") {
		return &NetworkError{
			Type:    "server",
			Message: "服务器暂时不可用，请稍后重试",
			Err:     err,
		}
	}

	// 其他网络错误
	return &NetworkError{
		Type:    "unknown",
		Message: "网络请求失败: " + err.Error(),
		Err:     err,
	}
}

// GetRetryDelay 根据错误类型和重试次数计算延迟时间
func GetRetryDelay(err error, attempt int) time.Duration {
	classified := ClassifyError(err)
	if classified == nil {
		return time.Second
	}

	baseDelay := time.Second

	switch classified.Type {
	case "timeout":
		// 超时错误使用较长的延迟，特别是对于Etherscan API
		if strings.Contains(strings.ToLower(err.Error()), "etherscan") ||
			strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") {
			baseDelay = 5 * time.Second // Etherscan API超时使用更长延迟
		} else {
			baseDelay = 2 * time.Second
		}
	case "rate_limit":
		// 限流错误使用更长的延迟
		baseDelay = 10 * time.Second
	case "server":
		// 服务器错误使用中等延迟
		baseDelay = 3 * time.Second
	default:
		baseDelay = time.Second
	}

	// 指数退避算法：延迟时间 = baseDelay * (2^attempt)
	delay := baseDelay * time.Duration(1<<uint(attempt))

	// 对于Etherscan API，限制最大延迟时间为60秒
	maxDelay := 30 * time.Second
	if strings.Contains(strings.ToLower(err.Error()), "etherscan") ||
		strings.Contains(strings.ToLower(err.Error()), "deadline exceeded") {
		maxDelay = 60 * time.Second
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	return delay
}
