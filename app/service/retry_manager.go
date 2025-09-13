package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"gorm.io/gorm"
)

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries    int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	Jitter        bool
	RetryableErrs []error
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:    3,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      2 * time.Second,
		Multiplier:    2.0,
		Jitter:        true,
		RetryableErrs: []error{
			gorm.ErrRecordNotFound,
			errors.New("version conflict"),
			errors.New("database connection failed"),
		},
	}
}

// RetryManager 重试管理器
type RetryManager struct {
	config RetryConfig
}

// NewRetryManager 创建重试管理器
func NewRetryManager() *RetryManager {
	return &RetryManager{
		config: DefaultRetryConfig(),
	}
}

// NewRetryManagerWithConfig 使用自定义配置创建重试管理器
func NewRetryManagerWithConfig(config RetryConfig) *RetryManager {
	return &RetryManager{
		config: config,
	}
}

// ExecuteWithRetry 执行带重试的操作
func (rm *RetryManager) ExecuteWithRetry(ctx context.Context, operation func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt <= rm.config.MaxRetries; attempt++ {
		// 检查上下文是否被取消
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 执行操作
		lastErr = operation(ctx)
		if lastErr == nil {
			return nil // 成功
		}

		// 检查是否是可重试的错误
		if !rm.isRetryableError(lastErr) {
			return lastErr // 不可重试的错误，直接返回
		}

		// 如果是最后一次尝试，不再等待
		if attempt == rm.config.MaxRetries {
			break
		}

		// 计算重试延迟
		delay := rm.calculateDelay(attempt)
		
		// 等待重试
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
			// 继续下一次重试
		}
	}

	return fmt.Errorf("operation failed after %d retries, last error: %w", rm.config.MaxRetries, lastErr)
}

// isRetryableError 判断错误是否可重试
func (rm *RetryManager) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 检查具体的可重试错误类型
	for _, retryableErr := range rm.config.RetryableErrs {
		if errors.Is(err, retryableErr) || 
		   (retryableErr.Error() != "" && containsError(err.Error(), retryableErr.Error())) {
			return true
		}
	}

	// 检查常见的可重试错误模式
	errStr := err.Error()
	retryablePatterns := []string{
		"connection reset",
		"connection refused",
		"timeout",
		"temporary failure",
		"deadlock",
		"lock wait timeout",
		"version conflict",
		"optimistic lock",
	}

	for _, pattern := range retryablePatterns {
		if containsError(errStr, pattern) {
			return true
		}
	}

	return false
}

// calculateDelay 计算重试延迟
func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
	delay := rm.config.BaseDelay
	
	// 指数退避
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * rm.config.Multiplier)
	}
	
	// 限制最大延迟
	if delay > rm.config.MaxDelay {
		delay = rm.config.MaxDelay
	}
	
	// 添加随机抖动以避免惊群效应
	if rm.config.Jitter {
		jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
		delay += jitter
	}
	
	return delay
}

// containsError 检查错误信息中是否包含特定字符串
func containsError(errStr, pattern string) bool {
	return len(errStr) > 0 && len(pattern) > 0 && 
		   (errStr == pattern || 
		    len(errStr) > len(pattern) && 
		    (errStr[:len(pattern)] == pattern || 
		     errStr[len(errStr)-len(pattern):] == pattern ||
		     containsSubstring(errStr, pattern)))
}

// containsSubstring 简单的子字符串包含检查
func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// NewRetryableError 创建可重试错误（用于测试）
func NewRetryableError(msg string) error {
	return &retryableError{msg: msg}
}

type retryableError struct {
	msg string
}

func (e *retryableError) Error() string {
	return e.msg
}

// CircuitBreakerConfig 断路器配置
type CircuitBreakerConfig struct {
	MaxFailures     int
	ResetTimeout    time.Duration
	HalfOpenMax     int
}

// CircuitBreaker 断路器实现
type CircuitBreaker struct {
	config       CircuitBreakerConfig
	failures     int
	lastFailTime time.Time
	state        CircuitState
	halfOpenReq  int
}

// CircuitState 断路器状态
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// NewCircuitBreaker 创建断路器
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  StateClosed,
	}
}

// Execute 执行操作（带断路器）
func (cb *CircuitBreaker) Execute(operation func() error) error {
	switch cb.state {
	case StateOpen:
		if time.Since(cb.lastFailTime) > cb.config.ResetTimeout {
			cb.state = StateHalfOpen
			cb.halfOpenReq = 0
		} else {
			return errors.New("circuit breaker is open")
		}
	case StateHalfOpen:
		if cb.halfOpenReq >= cb.config.HalfOpenMax {
			return errors.New("circuit breaker is half-open, max requests reached")
		}
		cb.halfOpenReq++
	}

	err := operation()
	
	if err != nil {
		cb.onFailure()
		return err
	}
	
	cb.onSuccess()
	return nil
}

// onFailure 处理失败
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailTime = time.Now()
	
	if cb.failures >= cb.config.MaxFailures {
		cb.state = StateOpen
	}
}

// onSuccess 处理成功
func (cb *CircuitBreaker) onSuccess() {
	cb.failures = 0
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
	}
}