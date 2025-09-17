package http

import (
	"sync"
	"time"
)

// EtherscanRateLimiter Etherscan API限流器
type EtherscanRateLimiter struct {
	minInterval time.Duration // 最小请求间隔
	maxRPS      int           // 每秒最大请求数
	requests    []time.Time   // 请求时间窗口
	mutex       sync.Mutex    // 并发安全
}

// NewEtherscanRateLimiter 创建新的Etherscan限流器
func NewEtherscanRateLimiter() *EtherscanRateLimiter {
	return &EtherscanRateLimiter{
		minInterval: 200 * time.Millisecond, // 最小间隔200ms
		maxRPS:      5,                      // 每秒最多5个请求
		requests:    make([]time.Time, 0),
	}
}

// Wait 等待直到可以发送请求
func (r *EtherscanRateLimiter) Wait() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	now := time.Now()

	// 清理1秒前的请求记录
	cutoff := now.Add(-time.Second)
	validRequests := make([]time.Time, 0)
	for _, reqTime := range r.requests {
		if reqTime.After(cutoff) {
			validRequests = append(validRequests, reqTime)
		}
	}
	r.requests = validRequests

	// 检查是否超过每秒最大请求数
	if len(r.requests) >= r.maxRPS {
		// 等待到最早请求的1秒后
		waitUntil := r.requests[0].Add(time.Second)
		if waitUntil.After(now) {
			time.Sleep(waitUntil.Sub(now))
		}
	}

	// 检查最小间隔
	if len(r.requests) > 0 {
		lastRequest := r.requests[len(r.requests)-1]
		if elapsed := now.Sub(lastRequest); elapsed < r.minInterval {
			time.Sleep(r.minInterval - elapsed)
		}
	}

	// 记录当前请求时间
	r.requests = append(r.requests, time.Now())
}

// 全局Etherscan限流器实例
var etherscanLimiter = NewEtherscanRateLimiter()

// WaitForEtherscan 为Etherscan API请求等待
func WaitForEtherscan() {
	etherscanLimiter.Wait()
}
