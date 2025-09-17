package http

import (
	"sync"
	"time"
)

// EtherscanRateLimiter Etherscan API专用限流器
type EtherscanRateLimiter struct {
	mu           sync.Mutex
	lastRequest  time.Time
	minInterval  time.Duration
	requestCount int
	windowStart  time.Time
	maxRequests  int
}

// NewEtherscanRateLimiter 创建Etherscan API限流器
func NewEtherscanRateLimiter() *EtherscanRateLimiter {
	return &EtherscanRateLimiter{
		minInterval: 200 * time.Millisecond, // 最小请求间隔200ms
		maxRequests: 5,                      // 每秒最多5个请求
		windowStart: time.Now(),
	}
}

// Wait 等待直到可以发送请求
func (rl *EtherscanRateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 检查是否需要重置计数窗口
	if now.Sub(rl.windowStart) >= time.Second {
		rl.requestCount = 0
		rl.windowStart = now
	}

	// 如果当前窗口内请求数已达上限，等待到下一个窗口
	if rl.requestCount >= rl.maxRequests {
		waitTime := time.Second - now.Sub(rl.windowStart)
		if waitTime > 0 {
			time.Sleep(waitTime)
			// 重置窗口
			rl.requestCount = 0
			rl.windowStart = time.Now()
		}
	}

	// 确保最小请求间隔
	if !rl.lastRequest.IsZero() {
		elapsed := now.Sub(rl.lastRequest)
		if elapsed < rl.minInterval {
			time.Sleep(rl.minInterval - elapsed)
		}
	}

	rl.lastRequest = time.Now()
	rl.requestCount++
}

// 全局Etherscan限流器实例
var etherscanLimiter = NewEtherscanRateLimiter()

// WaitForEtherscan 为Etherscan API请求等待
func WaitForEtherscan() {
	etherscanLimiter.Wait()
}
