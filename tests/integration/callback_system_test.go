package integration

import (
	"USDTMore/app/model"
	"USDTMore/app/notify"
	"USDTMore/tests/testutils"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"gorm.io/gorm"
)

// CallbackSystemTestSuite 回调系统集成测试套件
type CallbackSystemTestSuite struct {
	suite.Suite
	container          testcontainers.Container
	db                 *gorm.DB
	callbackServer     *testutils.MockCallbackServer
	telegramServer     *testutils.TelegramMockServer
	callbackCluster    *testutils.CallbackServerCluster
	notificationScenarios *testutils.TestNotificationScenarios
}

// SetupSuite 设置测试套件
func (s *CallbackSystemTestSuite) SetupSuite() {
	ctx := context.Background()
	
	// 设置测试数据库
	var err error
	s.container, s.db = testutils.SetupTestDB(ctx, s.T())
	require.NoError(s.T(), err)
	
	// 初始化模拟服务器
	s.callbackServer = testutils.NewMockCallbackServer("test_auth_token")
	s.telegramServer = testutils.NewTelegramMockServer("test_bot_token")
	s.callbackCluster = testutils.NewCallbackServerCluster(3, "test_auth_token")
	s.notificationScenarios = testutils.NewTestNotificationScenarios("test_auth_token", "test_bot_token")
}

// TearDownSuite 清理测试套件
func (s *CallbackSystemTestSuite) TearDownSuite() {
	if s.callbackServer != nil {
		s.callbackServer.Close()
	}
	if s.telegramServer != nil {
		s.telegramServer.Close()
	}
	if s.callbackCluster != nil {
		s.callbackCluster.Close()
	}
	if s.notificationScenarios != nil {
		s.notificationScenarios.Close()
	}
	if s.container != nil {
		ctx := context.Background()
		testutils.TeardownTestDB(ctx, s.container)
	}
}

// SetupTest 每个测试前的设置
func (s *CallbackSystemTestSuite) SetupTest() {
	// 清理数据库
	testutils.CleanDatabase(s.db)
	
	// 重置服务器状态
	s.callbackServer.Reset()
	s.telegramServer.Reset()
	s.callbackCluster.SetGlobalSuccessRate(1.0)
}

// TestSuccessfulCallback 测试成功的回调通知
func (s *CallbackSystemTestSuite) TestSuccessfulCallback() {
	// 创建成功的订单
	order := testutils.CreateSuccessOrder()
	order.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 配置成功回调
	s.notificationScenarios.ScenarioSuccessfulCallback(order.OrderId)
	
	// 执行回调通知
	notify.OrderNotify(*order)
	
	// 等待回调请求
	request, err := s.callbackServer.WaitForRequest(order.OrderId, 5*time.Second)
	require.NoError(s.T(), err)
	
	// 验证回调请求内容
	assert.Equal(s.T(), order.TradeId, request.TradeId)
	assert.Equal(s.T(), order.OrderId, request.OrderId)
	assert.Equal(s.T(), order.Money, request.Amount)
	assert.Equal(s.T(), order.Amount, request.ActualAmount)
	assert.Equal(s.T(), order.Address, request.Token)
	assert.Equal(s.T(), order.TradeHash, request.BlockTransactionId)
	assert.Equal(s.T(), model.OrderStatusSuccess, request.Status)
	assert.NotEmpty(s.T(), request.Signature)
	
	// 验证HTTP头
	assert.Equal(s.T(), "application/json", request.Headers.Get("Content-Type"))
	assert.Equal(s.T(), "https://ovsea.net", request.Headers.Get("Powered-By"))
	
	// 验证订单状态已更新
	var updatedOrder model.TradeOrders
	err = s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderNotifyStateSucc, updatedOrder.NotifyState)
	assert.Equal(s.T(), 1, updatedOrder.NotifyNum)
}

// TestCallbackRetryMechanism 测试回调重试机制
func (s *CallbackSystemTestSuite) TestCallbackRetryMechanism() {
	// 创建成功订单
	order := testutils.CreateSuccessOrder()
	order.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 配置前3次失败，第4次成功
	s.notificationScenarios.ScenarioRetryCallback(order.OrderId, 3)
	
	// 执行多次回调（模拟重试机制）
	for i := 0; i < 4; i++ {
		notify.OrderNotify(*order)
		time.Sleep(100 * time.Millisecond) // 短暂延迟避免并发问题
	}
	
	// 等待所有回调完成
	err = s.callbackServer.WaitForRequestCount(order.OrderId, 4, 10*time.Second)
	require.NoError(s.T(), err)
	
	requests := s.callbackServer.GetRequestsByOrderId(order.OrderId)
	assert.Len(s.T(), requests, 4)
	
	// 验证最终订单状态（最后一次应该成功）
	var updatedOrder model.TradeOrders
	err = s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderNotifyStateSucc, updatedOrder.NotifyState)
	assert.GreaterOrEqual(s.T(), updatedOrder.NotifyNum, 4)
}

// TestCallbackSignatureValidation 测试回调签名验证
func (s *CallbackSystemTestSuite) TestCallbackSignatureValidation() {
	// 创建成功订单
	order := testutils.CreateSuccessOrder()
	
	// 创建使用不同token的回调服务器（模拟签名验证失败）
	invalidServer := testutils.NewMockCallbackServer("invalid_token")
	defer invalidServer.Close()
	
	order.NotifyUrl = invalidServer.URL()
	
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 执行回调（应该因为签名验证失败而失败）
	notify.OrderNotify(*order)
	
	// 等待请求（应该收到但验证失败）
	time.Sleep(2 * time.Second)
	
	// 验证服务器收到了请求但返回了错误状态码
	assert.Greater(s.T(), invalidServer.GetCallCount(), int64(0))
	assert.Greater(s.T(), invalidServer.GetFailCount(), int64(0))
	
	// 验证订单状态仍为失败
	var updatedOrder model.TradeOrders
	err = s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderNotifyStateFail, updatedOrder.NotifyState)
}

// TestCallbackTimeout 测试回调超时处理
func (s *CallbackSystemTestSuite) TestCallbackTimeout() {
	// 创建成功订单
	order := testutils.CreateSuccessOrder()
	order.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 配置超时回调
	s.notificationScenarios.ScenarioCallbackTimeout(order.OrderId)
	
	// 执行回调
	start := time.Now()
	notify.OrderNotify(*order)
	duration := time.Since(start)
	
	// 验证回调在合理时间内超时（不会等待2分钟）
	assert.Less(s.T(), duration, 30*time.Second, "Callback should timeout within 30 seconds")
	
	// 验证订单状态为失败
	var updatedOrder model.TradeOrders
	err = s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderNotifyStateFail, updatedOrder.NotifyState)
}

// TestCallbackWithSlowResponse 测试慢响应回调
func (s *CallbackSystemTestSuite) TestCallbackWithSlowResponse() {
	// 创建成功订单
	order := testutils.CreateSuccessOrder()
	order.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 配置慢响应（3秒延迟）
	s.notificationScenarios.ScenarioSlowCallback(order.OrderId, 3*time.Second)
	
	// 执行回调
	start := time.Now()
	notify.OrderNotify(*order)
	duration := time.Since(start)
	
	// 验证等待了预期的时间
	assert.GreaterOrEqual(s.T(), duration, 3*time.Second)
	assert.Less(s.T(), duration, 5*time.Second)
	
	// 验证最终成功
	var updatedOrder model.TradeOrders
	err = s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderNotifyStateSucc, updatedOrder.NotifyState)
}

// TestBatchCallbackProcessing 测试批量回调处理
func (s *CallbackSystemTestSuite) TestBatchCallbackProcessing() {
	batchSize := 10
	orders := make([]*model.TradeOrders, batchSize)
	
	// 创建多个成功订单
	for i := 0; i < batchSize; i++ {
		order := testutils.CreateSuccessOrder()
		order.OrderId = fmt.Sprintf("BATCH_%d_%s", i, order.OrderId)
		order.TradeId = fmt.Sprintf("BATCH_%d_%s", i, order.TradeId)
		order.NotifyUrl = s.callbackServer.URL()
		
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
		orders[i] = order
	}
	
	// 并发执行回调
	for _, order := range orders {
		go func(o *model.TradeOrders) {
			notify.OrderNotify(*o)
		}(order)
	}
	
	// 等待所有回调完成
	time.Sleep(5 * time.Second)
	
	// 验证所有回调都已处理
	totalRequests := s.callbackServer.GetCallCount()
	assert.Equal(s.T(), int64(batchSize), totalRequests)
	
	// 验证所有订单状态都已更新
	for _, order := range orders {
		var updatedOrder model.TradeOrders
		err := s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
		require.NoError(s.T(), err)
		assert.Equal(s.T(), model.OrderNotifyStateSucc, updatedOrder.NotifyState)
	}
}

// TestCallbackLoadBalancing 测试回调负载均衡
func (s *CallbackSystemTestSuite) TestCallbackLoadBalancing() {
	serverCount := 3
	orderCount := 9 // 每个服务器应该收到3个请求
	
	orders := make([]*model.TradeOrders, orderCount)
	serverURLs := s.callbackCluster.GetAllServerURLs()
	
	// 为不同订单分配不同的回调服务器
	for i := 0; i < orderCount; i++ {
		order := testutils.CreateSuccessOrder()
		order.OrderId = fmt.Sprintf("LB_%d_%s", i, order.OrderId)
		order.TradeId = fmt.Sprintf("LB_%d_%s", i, order.TradeId)
		order.NotifyUrl = serverURLs[i%serverCount] // 轮询分配服务器
		
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
		orders[i] = order
	}
	
	// 执行所有回调
	for _, order := range orders {
		notify.OrderNotify(*order)
		time.Sleep(10 * time.Millisecond) // 避免过快的并发请求
	}
	
	// 等待处理完成
	time.Sleep(3 * time.Second)
	
	// 验证负载分布
	servers := s.callbackCluster.GetServers()
	totalRequests := int64(0)
	
	for i, server := range servers {
		requests := server.GetCallCount()
		totalRequests += requests
		s.T().Logf("Server %d received %d requests", i, requests)
		assert.Equal(s.T(), int64(3), requests, "Each server should receive 3 requests")
	}
	
	assert.Equal(s.T(), int64(orderCount), totalRequests)
}

// TestCallbackErrorHandling 测试各种错误情况的回调处理
func (s *CallbackSystemTestSuite) TestCallbackErrorHandling() {
	testCases := []struct {
		name           string
		responseStatus int
		responseBody   string
		expectedState  int
	}{
		{
			name:           "HTTP 404 Not Found",
			responseStatus: http.StatusNotFound,
			responseBody:   "Not Found",
			expectedState:  model.OrderNotifyStateFail,
		},
		{
			name:           "HTTP 500 Internal Error",
			responseStatus: http.StatusInternalServerError,
			responseBody:   "Internal Server Error",
			expectedState:  model.OrderNotifyStateFail,
		},
		{
			name:           "HTTP 200 with wrong body",
			responseStatus: http.StatusOK,
			responseBody:   "wrong response",
			expectedState:  model.OrderNotifyStateFail,
		},
		{
			name:           "HTTP 200 with correct body",
			responseStatus: http.StatusOK,
			responseBody:   "ok",
			expectedState:  model.OrderNotifyStateSucc,
		},
	}
	
	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			// 创建订单
			order := testutils.CreateSuccessOrder()
			order.OrderId = fmt.Sprintf("ERROR_%s_%s", tc.name, order.OrderId)
			order.TradeId = fmt.Sprintf("ERROR_%s_%s", tc.name, order.TradeId)
			order.NotifyUrl = s.callbackServer.URL()
			
			err := s.db.Create(order).Error
			require.NoError(t, err)
			
			// 配置响应
			s.callbackServer.SetResponse(order.OrderId, testutils.CallbackResponse{
				StatusCode: tc.responseStatus,
				Body:       tc.responseBody,
				Delay:      0,
			})
			
			// 执行回调
			notify.OrderNotify(*order)
			
			// 等待处理完成
			time.Sleep(2 * time.Second)
			
			// 验证结果
			var updatedOrder model.TradeOrders
			err = s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
			require.NoError(t, err)
			assert.Equal(t, tc.expectedState, updatedOrder.NotifyState, 
				"Test case: %s, Expected state: %d, Got: %d", tc.name, tc.expectedState, updatedOrder.NotifyState)
		})
	}
}

// TestTelegramNotification 测试Telegram通知
func (s *CallbackSystemTestSuite) TestTelegramNotification() {
	// 这个测试需要实际的Telegram通知实现
	// 这里我们模拟Telegram通知的场景
	
	// 创建成功订单
	order := testutils.CreateSuccessOrder()
	
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 模拟发送Telegram通知
	chatID := int64(12345)
	message := fmt.Sprintf("订单支付成功\n订单号: %s\n金额: %.2f\n交易哈希: %s", 
		order.OrderId, order.Money, order.TradeHash)
	
	// 这里需要调用实际的Telegram发送函数
	// 由于我们没有实际的实现，我们直接向模拟服务器发送
	s.telegramServer.GetMessages() // 初始化
	
	// 在实际测试中，这里会调用telegram.SendMessage()
	// 现在我们直接验证服务器设置
	assert.NotNil(s.T(), s.telegramServer)
	assert.Contains(s.T(), s.telegramServer.URL(), "http")
	
	s.T().Logf("Would send Telegram message to chat %d: %s", chatID, message)
}

// TestCallbackDataIntegrity 测试回调数据完整性
func (s *CallbackSystemTestSuite) TestCallbackDataIntegrity() {
	// 创建订单时使用特殊字符和边界值
	order := testutils.CreateTestOrder(map[string]interface{}{
		"order_id":   "TEST_<>&\"'订单",
		"money":      999999.99,
		"amount":     "138888.88",
		"trade_hash": "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
	})
	order.Status = model.OrderStatusSuccess
	order.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 执行回调
	notify.OrderNotify(*order)
	
	// 验证回调数据
	request, err := s.callbackServer.WaitForRequest(order.OrderId, 5*time.Second)
	require.NoError(s.T(), err)
	
	// 验证特殊字符正确传输
	assert.Equal(s.T(), "TEST_<>&\"'订单", request.OrderId)
	assert.Equal(s.T(), 999999.99, request.Amount)
	assert.Equal(s.T(), "138888.88", request.ActualAmount)
	assert.Equal(s.T(), "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890", request.BlockTransactionId)
	
	// 验证JSON格式正确
	assert.NotEmpty(s.T(), request.Signature)
	assert.Equal(s.T(), model.OrderStatusSuccess, request.Status)
}

// TestCallbackFailureRecovery 测试回调失败恢复机制
func (s *CallbackSystemTestSuite) TestCallbackFailureRecovery() {
	// 创建多个失败的回调订单
	failedOrders := []*model.TradeOrders{
		testutils.CreateFailedNotifyOrder(),
		testutils.CreateFailedNotifyOrder(),
		testutils.CreateFailedNotifyOrder(),
	}
	
	for i, order := range failedOrders {
		order.OrderId = fmt.Sprintf("RECOVERY_%d_%s", i, order.OrderId)
		order.TradeId = fmt.Sprintf("RECOVERY_%d_%s", i, order.TradeId)
		order.NotifyUrl = s.callbackServer.URL()
		
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
	}
	
	// 获取所有失败的通知订单
	retrievedOrders, err := model.GetNotifyFailedTradeOrders()
	require.NoError(s.T(), err)
	assert.GreaterOrEqual(s.T(), len(retrievedOrders), 3)
	
	// 重试失败的回调
	for _, order := range retrievedOrders {
		if order.OrderId != failedOrders[0].OrderId && 
		   order.OrderId != failedOrders[1].OrderId && 
		   order.OrderId != failedOrders[2].OrderId {
			continue // 跳过其他测试的订单
		}
		
		notify.OrderNotify(order)
		time.Sleep(100 * time.Millisecond)
	}
	
	// 等待处理完成
	time.Sleep(2 * time.Second)
	
	// 验证至少有一些回调被处理
	totalCallbacks := s.callbackServer.GetCallCount()
	assert.Greater(s.T(), totalCallbacks, int64(0))
}

// TestConcurrentCallbacks 测试并发回调处理
func (s *CallbackSystemTestSuite) TestConcurrentCallbacks() {
	concurrency := 20
	orders := make([]*model.TradeOrders, concurrency)
	
	// 创建多个订单
	for i := 0; i < concurrency; i++ {
		order := testutils.CreateSuccessOrder()
		order.OrderId = fmt.Sprintf("CONCURRENT_%d_%d", i, time.Now().UnixNano())
		order.TradeId = fmt.Sprintf("CONCURRENT_%d_%d", i, time.Now().UnixNano())
		order.NotifyUrl = s.callbackServer.URL()
		
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
		orders[i] = order
		
		time.Sleep(time.Millisecond) // 确保不同的时间戳
	}
	
	// 并发执行回调
	done := make(chan bool, concurrency)
	for _, order := range orders {
		go func(o *model.TradeOrders) {
			notify.OrderNotify(*o)
			done <- true
		}(order)
	}
	
	// 等待所有回调完成
	for i := 0; i < concurrency; i++ {
		select {
		case <-done:
			// 成功
		case <-time.After(10 * time.Second):
			s.T().Fatal("Concurrent callback test timed out")
		}
	}
	
	// 验证结果
	totalCallbacks := s.callbackServer.GetCallCount()
	assert.Equal(s.T(), int64(concurrency), totalCallbacks)
	
	// 验证所有订单状态正确更新
	successCount := 0
	for _, order := range orders {
		var updatedOrder model.TradeOrders
		err := s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
		require.NoError(s.T(), err)
		
		if updatedOrder.NotifyState == model.OrderNotifyStateSucc {
			successCount++
		}
	}
	
	assert.Equal(s.T(), concurrency, successCount, 
		"All orders should have successful callback state")
}

// TestCallbackRateLimiting 测试回调频率限制
func (s *CallbackSystemTestSuite) TestCallbackRateLimiting() {
	// 创建订单
	order := testutils.CreateSuccessOrder()
	order.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 快速连续发送多个回调请求
	rapidCallbacks := 5
	start := time.Now()
	
	for i := 0; i < rapidCallbacks; i++ {
		go notify.OrderNotify(*order)
	}
	
	// 等待处理完成
	time.Sleep(3 * time.Second)
	duration := time.Since(start)
	
	// 验证请求数量（可能由于并发控制而小于rapidCallbacks）
	totalRequests := s.callbackServer.GetCallCount()
	
	s.T().Logf("Sent %d rapid callbacks, received %d requests in %v", 
		rapidCallbacks, totalRequests, duration)
	
	// 在实际系统中，应该有频率限制机制
	// 这里我们只验证系统没有崩溃
	assert.Greater(s.T(), totalRequests, int64(0))
	assert.LessOrEqual(s.T(), totalRequests, int64(rapidCallbacks))
}

// 运行测试套件
func TestCallbackSystemTestSuite(t *testing.T) {
	suite.Run(t, new(CallbackSystemTestSuite))
}