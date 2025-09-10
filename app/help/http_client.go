package help

import (
	"net/http"
	"sync"
	"time"
)

// HTTPClientManager 管理全局HTTP客户端
type HTTPClientManager struct {
	clients map[string]*http.Client
	mu      sync.RWMutex
}

var (
	clientManager     *HTTPClientManager
	clientManagerOnce sync.Once
)

// GetHTTPClientManager 获取HTTP客户端管理器单例
func GetHTTPClientManager() *HTTPClientManager {
	clientManagerOnce.Do(func() {
		clientManager = &HTTPClientManager{
			clients: make(map[string]*http.Client),
		}
	})
	return clientManager
}

// GetClient 获取指定超时的HTTP客户端
func (m *HTTPClientManager) GetClient(timeout time.Duration) *http.Client {
	key := timeout.String()
	
	m.mu.RLock()
	if client, exists := m.clients[key]; exists {
		m.mu.RUnlock()
		return client
	}
	m.mu.RUnlock()
	
	// 创建新客户端
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 双重检查
	if client, exists := m.clients[key]; exists {
		return client
	}
	
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}
	
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	
	m.clients[key] = client
	return client
}

// GetDefaultClient 获取默认15秒超时的HTTP客户端
func GetDefaultClient() *http.Client {
	return GetHTTPClientManager().GetClient(15 * time.Second)
}

// GetShortTimeoutClient 获取5秒超时的HTTP客户端
func GetShortTimeoutClient() *http.Client {
	return GetHTTPClientManager().GetClient(5 * time.Second)
}

// GetLongTimeoutClient 获取30秒超时的HTTP客户端
func GetLongTimeoutClient() *http.Client {
	return GetHTTPClientManager().GetClient(30 * time.Second)
}

// CleanupClients 清理所有HTTP客户端连接（用于优雅关闭）
func CleanupClients() {
	manager := GetHTTPClientManager()
	manager.mu.Lock()
	defer manager.mu.Unlock()
	
	for _, client := range manager.clients {
		if transport, ok := client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
}