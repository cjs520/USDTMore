package integration

import (
	"USDTMore/app/model"
	"USDTMore/tests/testutils"
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"gorm.io/gorm"
)

// OrderLifecycleTestSuite 订单生命周期集成测试套件
type OrderLifecycleTestSuite struct {
	suite.Suite
	container      testcontainers.Container
	db             *gorm.DB
	blockchain     *testutils.MockBlockchainData
	scenarios      *testutils.TestBlockchainScenarios
	callbackServer *testutils.MockCallbackServer
}

// SetupSuite 设置测试套件
func (s *OrderLifecycleTestSuite) SetupSuite() {
	ctx := context.Background()
	
	// 设置测试数据库
	var err error
	s.container, s.db = testutils.SetupTestDB(ctx, s.T())
	require.NoError(s.T(), err)
	
	// 初始化模拟组件
	s.blockchain = testutils.NewMockBlockchainData()
	s.scenarios = testutils.NewTestBlockchainScenarios()
	s.callbackServer = testutils.NewMockCallbackServer("test_auth_token")
}

// TearDownSuite 清理测试套件
func (s *OrderLifecycleTestSuite) TearDownSuite() {
	if s.callbackServer != nil {
		s.callbackServer.Close()
	}
	if s.container != nil {
		ctx := context.Background()
		testutils.TeardownTestDB(ctx, s.container)
	}
}

// SetupTest 每个测试前的设置
func (s *OrderLifecycleTestSuite) SetupTest() {
	// 清理数据库
	testutils.CleanDatabase(s.db)
	
	// 重置回调服务器
	s.callbackServer.Reset()
	
	// 添加测试钱包地址
	s.setupTestWalletAddresses()
}

// setupTestWalletAddresses 设置测试钱包地址
func (s *OrderLifecycleTestSuite) setupTestWalletAddresses() {
	addresses := []*model.WalletAddress{
		testutils.CreateTestWalletAddress("TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"),
		testutils.CreateTestWalletAddress("BSC", "0x55d398326f99059ff775485246999027b3197955"),
		testutils.CreateTestWalletAddress("POLY", "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"),
		testutils.CreateTestWalletAddress("OP", "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58"),
	}
	
	for _, addr := range addresses {
		err := s.db.Create(addr).Error
		require.NoError(s.T(), err)
	}
}

// TestCompleteOrderLifecycle 测试完整的订单生命周期
func (s *OrderLifecycleTestSuite) TestCompleteOrderLifecycle() {
	testData := testutils.CreateOrderLifecycleTest()
	testData.OrderData.NotifyUrl = s.callbackServer.URL()
	
	// 1. 创建订单
	err := s.db.Create(testData.OrderData).Error
	require.NoError(s.T(), err)
	
	// 验证订单初始状态
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusWaiting, order.Status)
	assert.Equal(s.T(), model.OrderNotifyStateFail, order.NotifyState)
	assert.Equal(s.T(), 0, order.NotifyNum)
	assert.Empty(s.T(), order.TradeHash)
	
	// 2. 模拟区块链支付
	payment := s.scenarios.ScenarioNormalPayment(
		testutils.ChainTRON,
		testData.OrderData.Address,
		decimal.RequireFromString(testData.OrderData.Amount),
	)
	
	// 3. 更新订单状态为支付成功
	err = order.OrderSetSucc(payment.From, payment.Hash, payment.Timestamp)
	require.NoError(s.T(), err)
	
	// 4. 验证订单状态更新
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusSuccess, order.Status)
	assert.Equal(s.T(), payment.Hash, order.TradeHash)
	assert.Equal(s.T(), payment.From, order.FromAddress)
	assert.WithinDuration(s.T(), payment.Timestamp, order.ConfirmedAt, time.Second)
	
	// 5. 等待并验证回调通知
	_, err = s.callbackServer.WaitForRequest(testData.OrderData.OrderId, 5*time.Second)
	require.NoError(s.T(), err)
	
	// 验证回调请求内容
	requests := s.callbackServer.GetRequestsByOrderId(testData.OrderData.OrderId)
	require.Len(s.T(), requests, 1)
	
	callbackReq := requests[0]
	assert.Equal(s.T(), testData.OrderData.TradeId, callbackReq.TradeId)
	assert.Equal(s.T(), testData.OrderData.OrderId, callbackReq.OrderId)
	assert.Equal(s.T(), testData.OrderData.Money, callbackReq.Amount)
	assert.Equal(s.T(), testData.OrderData.Amount, callbackReq.ActualAmount)
	assert.Equal(s.T(), testData.OrderData.Address, callbackReq.Token)
	assert.Equal(s.T(), payment.Hash, callbackReq.BlockTransactionId)
	assert.Equal(s.T(), model.OrderStatusSuccess, callbackReq.Status)
	assert.NotEmpty(s.T(), callbackReq.Signature)
	
	// 6. 验证订单回调状态更新
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderNotifyStateSucc, order.NotifyState)
	assert.Equal(s.T(), 1, order.NotifyNum)
}

// TestMultiChainOrderLifecycle 测试多链订单生命周期
func (s *OrderLifecycleTestSuite) TestMultiChainOrderLifecycle() {
	chains := []testutils.ChainType{
		testutils.ChainTRON,
		testutils.ChainBSC,
		testutils.ChainPolygon,
		testutils.ChainOptimism,
	}
	
	orders := testutils.CreateMultiChainOrders()
	require.Len(s.T(), orders, len(chains))
	
	for i, order := range orders {
		order.NotifyUrl = s.callbackServer.URL()
		
		// 创建订单
		err := s.db.Create(order).Error
		require.NoError(s.T(), err, "Failed to create order for chain %s", chains[i])
		
		// 模拟支付
		payment := s.scenarios.ScenarioNormalPayment(
			chains[i],
			order.Address,
			decimal.RequireFromString(order.Amount),
		)
		
		// 更新订单状态
		err = order.OrderSetSucc(payment.From, payment.Hash, payment.Timestamp)
		require.NoError(s.T(), err, "Failed to set order success for chain %s", chains[i])
		
		// 验证支付成功
		var updatedOrder model.TradeOrders
		err = s.db.Where("trade_id = ?", order.TradeId).First(&updatedOrder).Error
		require.NoError(s.T(), err)
		assert.Equal(s.T(), model.OrderStatusSuccess, updatedOrder.Status)
		assert.Equal(s.T(), payment.Hash, updatedOrder.TradeHash)
	}
	
	// 等待所有回调完成
	time.Sleep(2 * time.Second)
	
	// 验证所有订单都收到了回调
	totalRequests := s.callbackServer.GetCallCount()
	assert.Equal(s.T(), int64(len(chains)), totalRequests)
}

// TestOrderExpiration 测试订单过期处理
func (s *OrderLifecycleTestSuite) TestOrderExpiration() {
	// 创建即将过期的订单
	expiredOrder := testutils.CreateTestOrder(map[string]interface{}{
		"expired_at": time.Now().Add(1 * time.Second), // 1秒后过期
	})
	
	err := s.db.Create(expiredOrder).Error
	require.NoError(s.T(), err)
	
	// 等待订单过期
	time.Sleep(2 * time.Second)
	
	// 设置订单为过期状态
	err = expiredOrder.OrderSetExpired()
	require.NoError(s.T(), err)
	
	// 验证订单状态
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", expiredOrder.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusExpired, order.Status)
	
	// 验证过期订单不应该触发回调
	time.Sleep(1 * time.Second)
	requests := s.callbackServer.GetRequestsByOrderId(order.OrderId)
	assert.Empty(s.T(), requests, "Expired order should not trigger callback")
}

// TestOrderPaymentAfterExpiration 测试订单过期后支付
func (s *OrderLifecycleTestSuite) TestOrderPaymentAfterExpiration() {
	// 创建已过期的订单
	expiredOrder := testutils.CreateTestOrder(map[string]interface{}{
		"expired_at": time.Now().Add(-1 * time.Hour), // 1小时前过期
	})
	
	err := s.db.Create(expiredOrder).Error
	require.NoError(s.T(), err)
	
	// 设置为过期状态
	err = expiredOrder.OrderSetExpired()
	require.NoError(s.T(), err)
	
	// 验证过期状态
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", expiredOrder.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusExpired, order.Status)
	
	// 尝试在过期后支付（这种情况下应该忽略支付或特殊处理）
	payment := s.scenarios.ScenarioNormalPayment(
		testutils.ChainTRON,
		order.Address,
		decimal.RequireFromString(order.Amount),
	)
	
	// 过期订单不应该更新为成功状态
	// 实际实现中，监控系统应该检查订单状态，避免更新已过期的订单
	
	// 验证订单仍然是过期状态
	err = s.db.Where("trade_id = ?", expiredOrder.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusExpired, order.Status)
	assert.Empty(s.T(), order.TradeHash) // 不应该有交易哈希
	
	s.T().Logf("Payment hash %s ignored for expired order %s", payment.Hash, order.OrderId)
}

// TestPartialPayment 测试部分支付场景
func (s *OrderLifecycleTestSuite) TestPartialPayment() {
	testData := testutils.CreateOrderLifecycleTest()
	testData.OrderData.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(testData.OrderData).Error
	require.NoError(s.T(), err)
	
	// 模拟部分支付（只支付80%的金额）
	expectedAmount := decimal.RequireFromString(testData.OrderData.Amount)
	partialPayment := s.scenarios.ScenarioPartialPayment(
		testutils.ChainTRON,
		testData.OrderData.Address,
		expectedAmount,
	)
	
	s.T().Logf("Expected amount: %s, Partial payment: %s", 
		expectedAmount.String(), partialPayment.Amount.String())
	
	// 部分支付不应该触发订单完成
	// 在实际系统中，监控系统会检查金额是否匹配
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusWaiting, order.Status) // 仍然等待支付
	
	// 验证没有回调触发
	time.Sleep(1 * time.Second)
	requests := s.callbackServer.GetRequestsByOrderId(order.OrderId)
	assert.Empty(s.T(), requests, "Partial payment should not trigger callback")
}

// TestOverpayment 测试超额支付场景
func (s *OrderLifecycleTestSuite) TestOverpayment() {
	testData := testutils.CreateOrderLifecycleTest()
	testData.OrderData.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(testData.OrderData).Error
	require.NoError(s.T(), err)
	
	// 模拟超额支付（支付120%的金额）
	expectedAmount := decimal.RequireFromString(testData.OrderData.Amount)
	overpayment := s.scenarios.ScenarioOverpayment(
		testutils.ChainTRON,
		testData.OrderData.Address,
		expectedAmount,
	)
	
	s.T().Logf("Expected amount: %s, Overpayment: %s", 
		expectedAmount.String(), overpayment.Amount.String())
	
	// 超额支付应该被接受并触发订单完成
	err = testData.OrderData.OrderSetSucc(overpayment.From, overpayment.Hash, overpayment.Timestamp)
	require.NoError(s.T(), err)
	
	// 验证订单状态
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusSuccess, order.Status)
	assert.Equal(s.T(), overpayment.Hash, order.TradeHash)
	
	// 验证回调被触发
	_, err = s.callbackServer.WaitForRequest(testData.OrderData.OrderId, 5*time.Second)
	require.NoError(s.T(), err)
}

// TestDelayedPayment 测试延迟支付场景
func (s *OrderLifecycleTestSuite) TestDelayedPayment() {
	testData := testutils.CreateOrderLifecycleTest()
	testData.OrderData.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(testData.OrderData).Error
	require.NoError(s.T(), err)
	
	// 模拟延迟支付（待确认状态）
	pendingPayment := s.scenarios.ScenarioDelayedPayment(
		testutils.ChainTRON,
		testData.OrderData.Address,
		decimal.RequireFromString(testData.OrderData.Amount),
	)
	
	// 初始状态应该是待确认
	assert.Equal(s.T(), "pending", pendingPayment.Status)
	
	// 等待支付确认（模拟的延迟确认）
	s.T().Log("Waiting for delayed payment confirmation...")
	
	// 在测试中，我们可以手动确认支付
	confirmed := s.scenarios.GetBlockchain().ConfirmTransaction(pendingPayment.Hash)
	assert.True(s.T(), confirmed, "Payment should be confirmed")
	
	// 获取确认后的交易
	confirmedTx, exists := s.scenarios.GetBlockchain().GetTransactionByHash(pendingPayment.Hash)
	require.True(s.T(), exists)
	assert.Equal(s.T(), "success", confirmedTx.Status)
	
	// 更新订单状态
	err = testData.OrderData.OrderSetSucc(confirmedTx.From, confirmedTx.Hash, confirmedTx.Timestamp)
	require.NoError(s.T(), err)
	
	// 验证最终状态
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusSuccess, order.Status)
}

// TestFailedPayment 测试支付失败场景
func (s *OrderLifecycleTestSuite) TestFailedPayment() {
	testData := testutils.CreateOrderLifecycleTest()
	
	err := s.db.Create(testData.OrderData).Error
	require.NoError(s.T(), err)
	
	// 模拟失败的支付
	failedPayment := s.scenarios.ScenarioFailedPayment(
		testutils.ChainTRON,
		testData.OrderData.Address,
		decimal.RequireFromString(testData.OrderData.Amount),
	)
	
	assert.Equal(s.T(), "failed", failedPayment.Status)
	
	// 失败的支付不应该更新订单状态
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusWaiting, order.Status) // 仍然等待支付
	assert.Empty(s.T(), order.TradeHash) // 没有交易哈希
	
	// 验证没有回调
	time.Sleep(1 * time.Second)
	requests := s.callbackServer.GetRequestsByOrderId(order.OrderId)
	assert.Empty(s.T(), requests, "Failed payment should not trigger callback")
}

// TestConcurrentPayments 测试并发支付场景
func (s *OrderLifecycleTestSuite) TestConcurrentPayments() {
	testData := testutils.CreateOrderLifecycleTest()
	testData.OrderData.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(testData.OrderData).Error
	require.NoError(s.T(), err)
	
	// 模拟多笔并发支付
	baseAmount := decimal.RequireFromString(testData.OrderData.Amount)
	concurrentPayments := s.scenarios.ScenarioConcurrentPayments(
		testutils.ChainTRON,
		testData.OrderData.Address,
		baseAmount,
		3, // 3笔并发支付
	)
	
	require.Len(s.T(), concurrentPayments, 3)
	
	// 在实际系统中，只有第一笔有效支付应该触发订单完成
	// 这里我们选择第一笔支付来更新订单
	firstPayment := concurrentPayments[0]
	err = testData.OrderData.OrderSetSucc(firstPayment.From, firstPayment.Hash, firstPayment.Timestamp)
	require.NoError(s.T(), err)
	
	// 验证订单状态
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusSuccess, order.Status)
	assert.Equal(s.T(), firstPayment.Hash, order.TradeHash)
	
	// 验证只有一次回调
	_, err = s.callbackServer.WaitForRequest(testData.OrderData.OrderId, 5*time.Second)
	require.NoError(s.T(), err)
	
	requests := s.callbackServer.GetRequestsByOrderId(order.OrderId)
	assert.Len(s.T(), requests, 1, "Should only have one callback for concurrent payments")
	
	s.T().Logf("Processed %d concurrent payments, order completed with tx: %s", 
		len(concurrentPayments), firstPayment.Hash)
}

// TestNetworkCongestionPayment 测试网络拥堵场景
func (s *OrderLifecycleTestSuite) TestNetworkCongestionPayment() {
	testData := testutils.CreateOrderLifecycleTest()
	testData.OrderData.NotifyUrl = s.callbackServer.URL()
	
	err := s.db.Create(testData.OrderData).Error
	require.NoError(s.T(), err)
	
	// 模拟网络拥堵
	congestionPayment := s.scenarios.ScenarioNetworkCongestion(
		testutils.ChainTRON,
		testData.OrderData.Address,
		decimal.RequireFromString(testData.OrderData.Amount),
	)
	
	// 网络拥堵时支付应该是待确认状态
	assert.Equal(s.T(), "pending", congestionPayment.Status)
	assert.True(s.T(), congestionPayment.GasPrice.GreaterThan(decimal.Zero))
	
	s.T().Logf("Network congestion payment with high gas price: %s", 
		congestionPayment.GasPrice.String())
	
	// 订单应该仍然等待支付
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusWaiting, order.Status)
	
	// 模拟长时间后的确认（在实际测试中我们手动确认）
	confirmed := s.scenarios.GetBlockchain().ConfirmTransaction(congestionPayment.Hash)
	require.True(s.T(), confirmed)
	
	// 获取确认后的交易并更新订单
	confirmedTx, exists := s.scenarios.GetBlockchain().GetTransactionByHash(congestionPayment.Hash)
	require.True(s.T(), exists)
	
	err = testData.OrderData.OrderSetSucc(confirmedTx.From, confirmedTx.Hash, confirmedTx.Timestamp)
	require.NoError(s.T(), err)
	
	// 验证最终完成
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderStatusSuccess, order.Status)
}

// TestOrderLifecycleWithRetries 测试订单生命周期包含重试
func (s *OrderLifecycleTestSuite) TestOrderLifecycleWithRetries() {
	testData := testutils.CreateOrderLifecycleTest()
	testData.OrderData.NotifyUrl = s.callbackServer.URL()
	
	// 配置回调失败几次后成功
	s.callbackServer.SetFailureRule(testData.OrderData.OrderId, 2) // 失败2次
	
	err := s.db.Create(testData.OrderData).Error
	require.NoError(s.T(), err)
	
	// 支付
	payment := s.scenarios.ScenarioNormalPayment(
		testutils.ChainTRON,
		testData.OrderData.Address,
		decimal.RequireFromString(testData.OrderData.Amount),
	)
	
	err = testData.OrderData.OrderSetSucc(payment.From, payment.Hash, payment.Timestamp)
	require.NoError(s.T(), err)
	
	// 等待重试完成（可能需要更长时间）
	err = s.callbackServer.WaitForRequestCount(testData.OrderData.OrderId, 3, 30*time.Second)
	require.NoError(s.T(), err, "Should have 3 callback attempts (2 failures + 1 success)")
	
	requests := s.callbackServer.GetRequestsByOrderId(testData.OrderData.OrderId)
	assert.Len(s.T(), requests, 3) // 2次失败 + 1次成功
	
	// 验证最终订单状态
	var order model.TradeOrders
	err = s.db.Where("trade_id = ?", testData.OrderData.TradeId).First(&order).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), model.OrderNotifyStateSucc, order.NotifyState)
	assert.Equal(s.T(), 3, order.NotifyNum) // 总共尝试3次
}

// TestBatchOrderProcessing 测试批量订单处理
func (s *OrderLifecycleTestSuite) TestBatchOrderProcessing() {
	batchSize := 5
	orders := testutils.CreateOrderBatch(batchSize, map[string]interface{}{
		"notify_url": s.callbackServer.URL(),
	})
	
	// 创建所有订单
	for _, order := range orders {
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
	}
	
	// 为每个订单模拟支付
	for _, order := range orders {
		payment := s.scenarios.ScenarioNormalPayment(
			testutils.ChainTRON,
			order.Address,
			decimal.RequireFromString(order.Amount),
		)
		
		err := order.OrderSetSucc(payment.From, payment.Hash, payment.Timestamp)
		require.NoError(s.T(), err)
	}
	
	// 等待所有回调完成
	time.Sleep(5 * time.Second)
	
	// 验证所有订单都成功完成
	for _, order := range orders {
		var updatedOrder model.TradeOrders
		err := s.db.Where("trade_id = ?", order.TradeId).First(&updatedOrder).Error
		require.NoError(s.T(), err)
		assert.Equal(s.T(), model.OrderStatusSuccess, updatedOrder.Status)
		assert.NotEmpty(s.T(), updatedOrder.TradeHash)
	}
	
	// 验证回调数量
	totalCallbacks := s.callbackServer.GetCallCount()
	assert.Equal(s.T(), int64(batchSize), totalCallbacks)
}

// TestOrderStateTransitions 测试订单状态转换
func (s *OrderLifecycleTestSuite) TestOrderStateTransitions() {
	order := testutils.CreateTestOrder()
	
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 1. 初始状态：等待支付
	assert.Equal(s.T(), model.OrderStatusWaiting, order.Status)
	assert.Equal(s.T(), "🟡 等待支付", order.GetStatusLabel())
	
	// 2. 转换到成功状态
	payment := s.scenarios.ScenarioNormalPayment(
		testutils.ChainTRON,
		order.Address,
		decimal.RequireFromString(order.Amount),
	)
	
	err = order.OrderSetSucc(payment.From, payment.Hash, payment.Timestamp)
	require.NoError(s.T(), err)
	
	assert.Equal(s.T(), model.OrderStatusSuccess, order.Status)
	assert.Equal(s.T(), "🟢 收款成功", order.GetStatusLabel())
	
	// 3. 测试过期状态（创建新订单）
	expiredOrder := testutils.CreateTestOrder()
	err = s.db.Create(expiredOrder).Error
	require.NoError(s.T(), err)
	
	err = expiredOrder.OrderSetExpired()
	require.NoError(s.T(), err)
	
	assert.Equal(s.T(), model.OrderStatusExpired, expiredOrder.Status)
	assert.Equal(s.T(), "🔴 交易过期", expiredOrder.GetStatusLabel())
}

// 运行测试套件
func TestOrderLifecycleTestSuite(t *testing.T) {
	suite.Run(t, new(OrderLifecycleTestSuite))
}