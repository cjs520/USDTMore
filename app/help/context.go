package help

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// ContextManager 管理应用程序的上下文和优雅关闭
type ContextManager struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan os.Signal
	once       sync.Once
}

var (
	contextManager     *ContextManager
	contextManagerOnce sync.Once
)

// GetContextManager 获取上下文管理器单例
func GetContextManager() *ContextManager {
	contextManagerOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		contextManager = &ContextManager{
			ctx:        ctx,
			cancel:     cancel,
			shutdownCh: make(chan os.Signal, 1),
		}
		
		// 监听系统信号
		signal.Notify(contextManager.shutdownCh, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
		
		// 启动信号处理goroutine
		go contextManager.handleShutdown()
	})
	return contextManager
}

// GetContext 获取应用程序上下文
func (cm *ContextManager) GetContext() context.Context {
	return cm.ctx
}

// AddWorker 添加一个工作goroutine
func (cm *ContextManager) AddWorker() {
	cm.wg.Add(1)
}

// WorkerDone 标记一个工作goroutine完成
func (cm *ContextManager) WorkerDone() {
	cm.wg.Done()
}

// handleShutdown 处理关闭信号
func (cm *ContextManager) handleShutdown() {
	<-cm.shutdownCh
	fmt.Println("收到关闭信号，开始优雅关闭...")
	cm.cancel()
}

// WaitForShutdown 等待所有工作goroutine完成
func (cm *ContextManager) WaitForShutdown(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		cm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("所有服务已优雅关闭")
	case <-time.After(timeout):
		fmt.Println("等待服务关闭超时，强制退出")
	}

	// 清理HTTP客户端连接
	CleanupClients()
}

// RunWithContext 在上下文中运行函数
func (cm *ContextManager) RunWithContext(name string, fn func(context.Context)) {
	cm.AddWorker()
	go func() {
		defer cm.WorkerDone()
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("工作goroutine发生panic: %s, %v\n", name, r)
			}
		}()
		fn(cm.ctx)
	}()
}

// IsShuttingDown 检查是否正在关闭
func (cm *ContextManager) IsShuttingDown() bool {
	select {
	case <-cm.ctx.Done():
		return true
	default:
		return false
	}
}