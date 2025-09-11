package testutils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CallbackRequest 回调请求结构
type CallbackRequest struct {
	TradeId            string  `json:"trade_id"`
	OrderId            string  `json:"order_id"`
	Amount             float64 `json:"amount"`
	ActualAmount       string  `json:"actual_amount"`
	Token              string  `json:"token"`
	BlockTransactionId string  `json:"block_transaction_id"`
	Status             int     `json:"status"`
	Signature          string  `json:"signature"`
	Timestamp          time.Time
	Headers            http.Header
}

// CallbackResponse 回调响应配置
type CallbackResponse struct {
	StatusCode int
	Body       string
	Delay      time.Duration
	Headers    map[string]string
}

// MockCallbackServer 模拟回调服务器
type MockCallbackServer struct {
	server      *httptest.Server
	mutex       sync.RWMutex
	requests    []CallbackRequest
	responses   map[string]CallbackResponse // key: order_id
	authToken   string
	callCount   int64
	failCount   int64
	successRate float64
	
	// 用于测试重试机制
	failureRules map[string]int // order_id -> fail times
}

// NewMockCallbackServer 创建模拟回调服务器
func NewMockCallbackServer(authToken string) *MockCallbackServer {
	mockServer := &MockCallbackServer{
		requests:     make([]CallbackRequest, 0),
		responses:    make(map[string]CallbackResponse),
		authToken:    authToken,
		successRate:  1.0,
		failureRules: make(map[string]int),
	}
	
	// 创建HTTP服务器
	mockServer.server = httptest.NewServer(http.HandlerFunc(mockServer.handleCallback))
	
	return mockServer
}

// handleCallback 处理回调请求
func (m *MockCallbackServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&m.callCount, 1)
	
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	
	// 解析JSON
	var req CallbackRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	req.Timestamp = time.Now()
	req.Headers = r.Header.Clone()
	
	// 验证签名
	if !m.validateSignature(req) {
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		atomic.AddInt64(&m.failCount, 1)
		return
	}
	
	m.mutex.Lock()
	m.requests = append(m.requests, req)
	m.mutex.Unlock()
	
	// 检查是否需要模拟失败
	if m.shouldFail(req.OrderId) {
		atomic.AddInt64(&m.failCount, 1)
		http.Error(w, "Simulated failure", http.StatusInternalServerError)
		return
	}
	
	// 获取响应配置
	response := m.getResponse(req.OrderId)
	
	// 模拟延迟
	if response.Delay > 0 {
		time.Sleep(response.Delay)
	}
	
	// 设置响应头
	for key, value := range response.Headers {
		w.Header().Set(key, value)
	}
	
	// 返回响应
	w.WriteHeader(response.StatusCode)
	w.Write([]byte(response.Body))
}

// validateSignature 验证签名
func (m *MockCallbackServer) validateSignature(req CallbackRequest) bool {
	if m.authToken == "" {
		return true // 如果没有设置token，跳过验证
	}
	
	// 重构数据用于签名验证
	data := map[string]interface{}{
		"trade_id":             req.TradeId,
		"order_id":             req.OrderId,
		"amount":               req.Amount,
		"actual_amount":        req.ActualAmount,
		"token":                req.Token,
		"block_transaction_id": req.BlockTransactionId,
		"status":               req.Status,
	}
	
	expectedSignature := m.generateSignature(data)
	return req.Signature == expectedSignature
}

// generateSignature 生成签名
func (m *MockCallbackServer) generateSignature(data map[string]interface{}) string {
	// 获取所有键并排序
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	// 构建签名字符串
	var signStr strings.Builder
	for _, k := range keys {
		v := data[k]
		signStr.WriteString(k)
		signStr.WriteString("=")
		signStr.WriteString(fmt.Sprintf("%v", v))
		signStr.WriteString("&")
	}
	
	// 添加token
	signStr.WriteString("token=")
	signStr.WriteString(m.authToken)
	
	// 计算HMAC-SHA256
	h := hmac.New(sha256.New, []byte(m.authToken))
	h.Write([]byte(signStr.String()))
	return hex.EncodeToString(h.Sum(nil))
}

// shouldFail 检查是否应该模拟失败
func (m *MockCallbackServer) shouldFail(orderId string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// 检查特定订单的失败规则
	if failTimes, exists := m.failureRules[orderId]; exists && failTimes > 0 {
		m.failureRules[orderId] = failTimes - 1
		return true
	}
	
	// 基于成功率的随机失败
	if m.successRate < 1.0 {
		random := float64(time.Now().UnixNano()%1000) / 1000.0
		return random > m.successRate
	}
	
	return false
}

// getResponse 获取响应配置
func (m *MockCallbackServer) getResponse(orderId string) CallbackResponse {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if response, exists := m.responses[orderId]; exists {
		return response
	}
	
	// 默认成功响应
	return CallbackResponse{
		StatusCode: http.StatusOK,
		Body:       "ok",
		Delay:      0,
		Headers:    make(map[string]string),
	}
}

// URL 获取服务器URL
func (m *MockCallbackServer) URL() string {
	return m.server.URL
}

// Close 关闭服务器
func (m *MockCallbackServer) Close() {
	m.server.Close()
}

// SetResponse 设置特定订单的响应
func (m *MockCallbackServer) SetResponse(orderId string, response CallbackResponse) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.responses[orderId] = response
}

// SetSuccessRate 设置成功率
func (m *MockCallbackServer) SetSuccessRate(rate float64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.successRate = rate
}

// SetFailureRule 设置特定订单的失败次数
func (m *MockCallbackServer) SetFailureRule(orderId string, failTimes int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.failureRules[orderId] = failTimes
}

// GetRequests 获取所有接收到的请求
func (m *MockCallbackServer) GetRequests() []CallbackRequest {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	// 返回副本避免并发修改
	requests := make([]CallbackRequest, len(m.requests))
	copy(requests, m.requests)
	return requests
}

// GetRequestsByOrderId 获取特定订单的请求
func (m *MockCallbackServer) GetRequestsByOrderId(orderId string) []CallbackRequest {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	var result []CallbackRequest
	for _, req := range m.requests {
		if req.OrderId == orderId {
			result = append(result, req)
		}
	}
	return result
}

// GetCallCount 获取总调用次数
func (m *MockCallbackServer) GetCallCount() int64 {
	return atomic.LoadInt64(&m.callCount)
}

// GetFailCount 获取失败次数
func (m *MockCallbackServer) GetFailCount() int64 {
	return atomic.LoadInt64(&m.failCount)
}

// Reset 重置服务器状态
func (m *MockCallbackServer) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.requests = make([]CallbackRequest, 0)
	m.responses = make(map[string]CallbackResponse)
	m.failureRules = make(map[string]int)
	atomic.StoreInt64(&m.callCount, 0)
	atomic.StoreInt64(&m.failCount, 0)
	m.successRate = 1.0
}

// WaitForRequest 等待特定订单的回调请求
func (m *MockCallbackServer) WaitForRequest(orderId string, timeout time.Duration) (*CallbackRequest, error) {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		requests := m.GetRequestsByOrderId(orderId)
		if len(requests) > 0 {
			return &requests[len(requests)-1], nil // 返回最新的请求
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	return nil, fmt.Errorf("timeout waiting for callback request for order %s", orderId)
}

// WaitForRequestCount 等待指定数量的回调请求
func (m *MockCallbackServer) WaitForRequestCount(orderId string, count int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		requests := m.GetRequestsByOrderId(orderId)
		if len(requests) >= count {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	return fmt.Errorf("timeout waiting for %d callback requests for order %s", count, orderId)
}

// CallbackServerCluster 回调服务器集群（用于测试负载均衡等）
type CallbackServerCluster struct {
	servers []*MockCallbackServer
	current int64
}

// NewCallbackServerCluster 创建回调服务器集群
func NewCallbackServerCluster(serverCount int, authToken string) *CallbackServerCluster {
	cluster := &CallbackServerCluster{
		servers: make([]*MockCallbackServer, serverCount),
		current: 0,
	}
	
	for i := 0; i < serverCount; i++ {
		cluster.servers[i] = NewMockCallbackServer(authToken)
	}
	
	return cluster
}

// GetNextServerURL 获取下一个服务器URL（轮询）
func (c *CallbackServerCluster) GetNextServerURL() string {
	serverIndex := atomic.AddInt64(&c.current, 1) % int64(len(c.servers))
	return c.servers[serverIndex].URL()
}

// GetRandomServerURL 获取随机服务器URL
func (c *CallbackServerCluster) GetRandomServerURL() string {
	serverIndex := time.Now().UnixNano() % int64(len(c.servers))
	return c.servers[serverIndex].URL()
}

// GetAllServerURLs 获取所有服务器URL
func (c *CallbackServerCluster) GetAllServerURLs() []string {
	urls := make([]string, len(c.servers))
	for i, server := range c.servers {
		urls[i] = server.URL()
	}
	return urls
}

// GetTotalRequests 获取所有服务器的总请求数
func (c *CallbackServerCluster) GetTotalRequests() int64 {
	var total int64
	for _, server := range c.servers {
		total += server.GetCallCount()
	}
	return total
}

// Close 关闭所有服务器
func (c *CallbackServerCluster) Close() {
	for _, server := range c.servers {
		server.Close()
	}
}

// SetGlobalSuccessRate 设置所有服务器的成功率
func (c *CallbackServerCluster) SetGlobalSuccessRate(rate float64) {
	for _, server := range c.servers {
		server.SetSuccessRate(rate)
	}
}

// GetServers 获取所有服务器实例
func (c *CallbackServerCluster) GetServers() []*MockCallbackServer {
	return c.servers
}

// TelegramMockServer 模拟Telegram机器人服务器
type TelegramMockServer struct {
	server   *httptest.Server
	mutex    sync.RWMutex
	messages []TelegramMessage
	botToken string
}

// TelegramMessage Telegram消息结构
type TelegramMessage struct {
	ChatId    int64     `json:"chat_id"`
	Text      string    `json:"text"`
	ParseMode string    `json:"parse_mode"`
	Timestamp time.Time `json:"timestamp"`
}

// NewTelegramMockServer 创建模拟Telegram服务器
func NewTelegramMockServer(botToken string) *TelegramMockServer {
	mockServer := &TelegramMockServer{
		messages: make([]TelegramMessage, 0),
		botToken: botToken,
	}
	
	mockServer.server = httptest.NewServer(http.HandlerFunc(mockServer.handleTelegramAPI))
	return mockServer
}

// handleTelegramAPI 处理Telegram API请求
func (t *TelegramMockServer) handleTelegramAPI(w http.ResponseWriter, r *http.Request) {
	// 检查是否是sendMessage API
	if !strings.Contains(r.URL.Path, "sendMessage") {
		http.Error(w, "Unknown API", http.StatusNotFound)
		return
	}
	
	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	
	// 解析消息
	var message TelegramMessage
	if err := json.Unmarshal(body, &message); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	
	message.Timestamp = time.Now()
	
	t.mutex.Lock()
	t.messages = append(t.messages, message)
	t.mutex.Unlock()
	
	// 返回成功响应
	response := map[string]interface{}{
		"ok":     true,
		"result": map[string]interface{}{
			"message_id": len(t.messages),
			"date":       time.Now().Unix(),
			"text":       message.Text,
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// URL 获取服务器URL
func (t *TelegramMockServer) URL() string {
	return t.server.URL
}

// Close 关闭服务器
func (t *TelegramMockServer) Close() {
	t.server.Close()
}

// GetMessages 获取所有消息
func (t *TelegramMockServer) GetMessages() []TelegramMessage {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	
	messages := make([]TelegramMessage, len(t.messages))
	copy(messages, t.messages)
	return messages
}

// GetMessagesByChatId 获取特定聊天的消息
func (t *TelegramMockServer) GetMessagesByChatId(chatId int64) []TelegramMessage {
	t.mutex.RLock()
	defer t.mutex.RUnlock()
	
	var result []TelegramMessage
	for _, msg := range t.messages {
		if msg.ChatId == chatId {
			result = append(result, msg)
		}
	}
	return result
}

// Reset 重置消息记录
func (t *TelegramMockServer) Reset() {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.messages = make([]TelegramMessage, 0)
}

// WaitForMessage 等待特定聊天的消息
func (t *TelegramMockServer) WaitForMessage(chatId int64, timeout time.Duration) (*TelegramMessage, error) {
	deadline := time.Now().Add(timeout)
	initialCount := len(t.GetMessagesByChatId(chatId))
	
	for time.Now().Before(deadline) {
		messages := t.GetMessagesByChatId(chatId)
		if len(messages) > initialCount {
			return &messages[len(messages)-1], nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	
	return nil, fmt.Errorf("timeout waiting for telegram message for chat %d", chatId)
}

// TestNotificationScenarios 通知测试场景
type TestNotificationScenarios struct {
	CallbackServer  *MockCallbackServer
	TelegramServer  *TelegramMockServer
	CallbackCluster *CallbackServerCluster
}

// NewTestNotificationScenarios 创建通知测试场景
func NewTestNotificationScenarios(authToken, botToken string) *TestNotificationScenarios {
	return &TestNotificationScenarios{
		CallbackServer:  NewMockCallbackServer(authToken),
		TelegramServer:  NewTelegramMockServer(botToken),
		CallbackCluster: NewCallbackServerCluster(3, authToken),
	}
}

// ScenarioSuccessfulCallback 成功回调场景
func (s *TestNotificationScenarios) ScenarioSuccessfulCallback(orderId string) {
	s.CallbackServer.SetResponse(orderId, CallbackResponse{
		StatusCode: http.StatusOK,
		Body:       "ok",
		Delay:      0,
	})
}

// ScenarioRetryCallback 重试回调场景
func (s *TestNotificationScenarios) ScenarioRetryCallback(orderId string, failTimes int) {
	s.CallbackServer.SetFailureRule(orderId, failTimes)
	// 最后一次成功
	s.CallbackServer.SetResponse(orderId, CallbackResponse{
		StatusCode: http.StatusOK,
		Body:       "ok",
		Delay:      0,
	})
}

// ScenarioSlowCallback 慢回调场景
func (s *TestNotificationScenarios) ScenarioSlowCallback(orderId string, delay time.Duration) {
	s.CallbackServer.SetResponse(orderId, CallbackResponse{
		StatusCode: http.StatusOK,
		Body:       "ok",
		Delay:      delay,
	})
}

// ScenarioInvalidSignature 无效签名场景
func (s *TestNotificationScenarios) ScenarioInvalidSignature(orderId string) {
	// 设置错误的authToken来模拟签名验证失败
	invalidServer := NewMockCallbackServer("invalid_token")
	defer invalidServer.Close()
	
	s.CallbackServer = invalidServer
}

// ScenarioCallbackTimeout 回调超时场景
func (s *TestNotificationScenarios) ScenarioCallbackTimeout(orderId string) {
	s.CallbackServer.SetResponse(orderId, CallbackResponse{
		StatusCode: http.StatusOK,
		Body:       "ok",
		Delay:      time.Minute * 2, // 2分钟延迟，超过大多数超时设置
	})
}

// Close 清理所有服务器
func (s *TestNotificationScenarios) Close() {
	s.CallbackServer.Close()
	s.TelegramServer.Close()
	s.CallbackCluster.Close()
}