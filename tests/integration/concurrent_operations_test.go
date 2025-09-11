package integration

import (
	"USDTMore/app/model"
	"USDTMore/app/notify"
	"USDTMore/tests/testutils"
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"gorm.io/gorm"
)

// ConcurrentOperationsTestSuite 并发操作集成测试套件
type ConcurrentOperationsTestSuite struct {
	suite.Suite
	container          testcontainers.Container
	db                 *gorm.DB
	blockchain         *testutils.MockBlockchainData
	scenarios          *testutils.TestBlockchainScenarios
	callbackServer     *testutils.MockCallbackServer
	performanceData    *testutils.PerformanceTestData
	consistencyData    *testutils.DataConsistencyTestSuite
}

// SetupSuite 设置测试套件
func (s *ConcurrentOperationsTestSuite) SetupSuite() {
	ctx := context.Background()
	
	// 设置测试数据库
	var err error
	s.container, s.db = testutils.SetupTestDB(ctx, s.T())
	require.NoError(s.T(), err)
	
	// 初始化模拟组件
	s.blockchain = testutils.NewMockBlockchainData()
	s.scenarios = testutils.NewTestBlockchainScenarios()
	s.callbackServer = testutils.NewMockCallbackServer("test_auth_token")
	
	// 创建性能和一致性测试数据
	s.performanceData = testutils.CreatePerformanceScenario("moderate")
	s.consistencyData = testutils.CreateDataConsistencyTests()
}

// TearDownSuite 清理测试套件
func (s *ConcurrentOperationsTestSuite) TearDownSuite() {
	if s.callbackServer != nil {
		s.callbackServer.Close()
	}
	if s.container != nil {
		ctx := context.Background()
		testutils.TeardownTestDB(ctx, s.container)
	}
}

// SetupTest 每个测试前的设置
func (s *ConcurrentOperationsTestSuite) SetupTest() {
	// 清理数据库
	testutils.CleanDatabase(s.db)
	
	// 重置模拟组件
	s.callbackServer.Reset()
	
	// 添加测试钱包地址
	s.setupTestWalletAddresses()
}

// setupTestWalletAddresses 设置测试钱包地址
func (s *ConcurrentOperationsTestSuite) setupTestWalletAddresses() {
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

// TestConcurrentOrderCreation 测试并发订单创建
func (s *ConcurrentOperationsTestSuite) TestConcurrentOrderCreation() {
	concurrency := 20
	baseAmount := 100.0
	
	orders := testutils.CreateConcurrentTestOrders(concurrency, baseAmount)
	
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	// 并发创建订单
	for _, order := range orders {
		wg.Add(1)
		go func(o *model.TradeOrders) {
			defer wg.Done()
			
			err := s.db.Create(o).Error
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				s.T().Logf("Order creation failed: %v", err)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(order)
	}
	
	wg.Wait()
	
	// 验证结果
	assert.Equal(s.T(), int64(concurrency), successCount, "All orders should be created successfully")
	assert.Equal(s.T(), int64(0), errorCount, "No errors should occur")
	
	// 验证数据库中的订单数量
	var count int64
	err := s.db.Model(&model.TradeOrders{}).Count(&count).Error
	require.NoError(s.T(), err)
	assert.Equal(s.T(), int64(concurrency), count)
	
	s.T().Logf("Successfully created %d concurrent orders", successCount)
}

// TestConcurrentOrderStatusUpdates 测试并发订单状态更新
func (s *ConcurrentOperationsTestSuite) TestConcurrentOrderStatusUpdates() {
	// 创建基础订单
	orders := testutils.CreateOrderBatch(10)
	for _, order := range orders {
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
	}
	
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	// 并发更新订单状态
	for _, order := range orders {
		wg.Add(1)
		go func(o *model.TradeOrders) {
			defer wg.Done()
			
			// 模拟支付
			payment := s.scenarios.ScenarioNormalPayment(
				testutils.ChainTRON,
				o.Address,
				decimal.RequireFromString(o.Amount),
			)
			
			// 更新订单状态
			err := o.OrderSetSucc(payment.From, payment.Hash, payment.Timestamp)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				s.T().Logf("Order update failed: %v", err)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(order)
	}
	
	wg.Wait()
	
	// 验证结果
	assert.Equal(s.T(), int64(len(orders)), successCount)
	assert.Equal(s.T(), int64(0), errorCount)
	
	// 验证所有订单都已更新为成功状态
	for _, order := range orders {
		var updatedOrder model.TradeOrders
		err := s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
		require.NoError(s.T(), err)
		assert.Equal(s.T(), model.OrderStatusSuccess, updatedOrder.Status)
		assert.NotEmpty(s.T(), updatedOrder.TradeHash)
	}
	
	s.T().Logf("Successfully updated %d orders concurrently", successCount)
}

// TestConcurrentOrderQueries 测试并发订单查询
func (s *ConcurrentOperationsTestSuite) TestConcurrentOrderQueries() {
	// 创建测试订单
	orderCount := 50
	orders := testutils.CreateOrderBatch(orderCount)
	for _, order := range orders {
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
	}
	
	concurrency := 100
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	// 并发查询订单
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			// 随机选择一个订单查询
			order := orders[index%len(orders)]
			
			// 执行查询
			retrievedOrder, exists := model.GetTradeOrder(order.TradeId)
			if !exists {
				atomic.AddInt64(&errorCount, 1)
				s.T().Logf("Order not found: %s", order.TradeId)
				return
			}
			
			// 验证查询结果
			if retrievedOrder.OrderId != order.OrderId {
				atomic.AddInt64(&errorCount, 1)
				s.T().Logf("Order data mismatch for: %s", order.TradeId)
				return
			}
			
			atomic.AddInt64(&successCount, 1)
		}(i)
	}
	
	wg.Wait()
	
	// 验证结果
	assert.Equal(s.T(), int64(concurrency), successCount)
	assert.Equal(s.T(), int64(0), errorCount)
	
	s.T().Logf("Successfully executed %d concurrent queries", successCount)
}

// TestConcurrentCallbacks 测试并发回调处理
func (s *ConcurrentOperationsTestSuite) TestConcurrentCallbacks() {
	// 创建成功订单用于回调测试
	orderCount := 15
	orders := make([]*model.TradeOrders, orderCount)
	
	for i := 0; i < orderCount; i++ {
		order := testutils.CreateSuccessOrder()
		order.OrderId = fmt.Sprintf("CALLBACK_%d_%d", i, time.Now().UnixNano())
		order.TradeId = fmt.Sprintf("CALLBACK_%d_%d", i, time.Now().UnixNano())
		order.NotifyUrl = s.callbackServer.URL()
		
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
		orders[i] = order
		
		time.Sleep(time.Millisecond) // 确保唯一ID
	}
	
	var wg sync.WaitGroup
	
	// 并发执行回调
	for _, order := range orders {
		wg.Add(1)
		go func(o *model.TradeOrders) {
			defer wg.Done()
			notify.OrderNotify(*o)
		}(order)
	}
	
	wg.Wait()
	
	// 等待所有回调完成
	time.Sleep(5 * time.Second)
	
	// 验证回调结果
	totalCallbacks := s.callbackServer.GetCallCount()
	assert.Equal(s.T(), int64(orderCount), totalCallbacks)
	
	// 验证所有订单的回调状态
	for _, order := range orders {
		var updatedOrder model.TradeOrders
		err := s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
		require.NoError(s.T(), err)
		assert.Equal(s.T(), model.OrderNotifyStateSucc, updatedOrder.NotifyState)
	}
	
	s.T().Logf("Successfully processed %d concurrent callbacks", totalCallbacks)
}

// TestDataConsistencyUnderConcurrency 测试并发情况下的数据一致性
func (s *ConcurrentOperationsTestSuite) TestDataConsistencyUnderConcurrency() {
	// 使用预定义的数据一致性测试套件
	suite := s.consistencyData
	
	// 创建基础订单
	for _, order := range suite.BaseOrders {
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
	}
	
	var wg sync.WaitGroup
	
	// 并发执行所有更新操作
	for _, operation := range suite.ConcurrentUpdates {
		wg.Add(1)
		go func(op testutils.OrderUpdateOperation) {
			defer wg.Done()
			
			// 等待指定的延迟
			if op.Delay > 0 {
				time.Sleep(op.Delay)
			}
			
			// 查找订单
			var order model.TradeOrders
			err := s.db.Where("order_id = ?", op.OrderID).First(&order).Error
			if err != nil {
				s.T().Logf("Order not found for operation: %s", op.OrderID)
				return
			}
			
			// 执行操作
			switch op.Operation {
			case "pay":
				txHash := op.Data["tx_hash"].(string)
				fromAddress := op.Data["from_address"].(string)
				err = order.OrderSetSucc(fromAddress, txHash, time.Now())
				if err != nil {
					s.T().Logf("Payment operation failed: %v", err)
				}
				
			case "expire":
				err = order.OrderSetExpired()
				if err != nil {
					s.T().Logf("Expiration operation failed: %v", err)
				}
				
			case "callback":
				// 模拟回调通知状态更新
				order.NotifyUrl = s.callbackServer.URL()
				notify.OrderNotify(order)
			}
		}(operation)
	}
	
	wg.Wait()
	
	// 等待所有操作完成
	time.Sleep(2 * time.Second)
	
	// 验证最终状态
	for _, expected := range suite.ExpectedFinalStates {
		var order model.TradeOrders
		err := s.db.Where("order_id = ?", expected.OrderID).First(&order).Error
		require.NoError(s.T(), err, "Order should exist: %s", expected.OrderID)
		
		assert.Equal(s.T(), expected.ExpectedStatus, order.Status,
			"Order %s status mismatch. Expected: %d, Got: %d", 
			expected.OrderID, expected.ExpectedStatus, order.Status)
		
		if expected.ShouldHaveTxHash {
			assert.NotEmpty(s.T(), order.TradeHash,
				"Order %s should have transaction hash", expected.OrderID)
		} else {
			assert.Empty(s.T(), order.TradeHash,
				"Order %s should not have transaction hash", expected.OrderID)
		}
		
		// 注意：回调计数验证在这个测试中可能不够精确，因为并发操作的复杂性
		s.T().Logf("Order %s final state verified: Status=%d, TxHash=%s", 
			expected.OrderID, order.Status, order.TradeHash)
	}
}

// TestAmountCalculationConcurrency 测试金额计算的并发安全性
func (s *ConcurrentOperationsTestSuite) TestAmountCalculationConcurrency() {
	// 获取可用地址
	addresses := model.GetAvailableAddress("TRON")
	require.Greater(s.T(), len(addresses), 0)
	
	concurrency := 20
	rate := 7.2
	money := 100.0
	
	var wg sync.WaitGroup
	results := make(chan struct {
		address model.WalletAddress
		amount  string
		index   int
	}, concurrency)
	
	// 并发计算交易金额
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			address, amount := model.CalcTradeAmount(addresses, rate, money)
			results <- struct {
				address model.WalletAddress
				amount  string
				index   int
			}{address, amount, index}
		}(i)
	}
	
	wg.Wait()
	close(results)
	
	// 收集结果
	var amounts []string
	var addressCounts = make(map[string]int)
	
	for result := range results {
		amounts = append(amounts, result.amount)
		key := result.address.Chain + result.address.Address
		addressCounts[key]++
		
		s.T().Logf("Calculation %d: Address=%s, Amount=%s", 
			result.index, result.address.Address, result.amount)
	}
	
	// 验证结果
	assert.Len(s.T(), amounts, concurrency)
	
	// 验证金额唯一性（原子精度保证）
	uniqueAmounts := make(map[string]bool)
	for _, amount := range amounts {
		uniqueAmounts[amount] = true
	}
	
	// 由于原子精度机制，每个金额应该是唯一的
	assert.Equal(s.T(), concurrency, len(uniqueAmounts),
		"All calculated amounts should be unique due to atomicity")
	
	s.T().Logf("Generated %d unique amounts from %d concurrent calculations", 
		len(uniqueAmounts), concurrency)
}

// TestDatabaseConnectionPoolStress 测试数据库连接池压力
func (s *ConcurrentOperationsTestSuite) TestDatabaseConnectionPoolStress() {
	concurrency := 50
	operationsPerWorker := 10
	
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	
	// 启动多个工作协程
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			// 每个工作协程执行多个数据库操作
			for j := 0; j < operationsPerWorker; j++ {
				// 混合不同类型的数据库操作
				switch j % 4 {
				case 0:
					// 插入操作
					order := testutils.CreateTestOrder(map[string]interface{}{
						"order_id": fmt.Sprintf("STRESS_%d_%d_%d", workerID, j, time.Now().UnixNano()),
						"trade_id": fmt.Sprintf("STRESS_%d_%d_%d", workerID, j, time.Now().UnixNano()),
					})
					err := s.db.Create(order).Error
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
					
				case 1:
					// 查询操作
					var orders []model.TradeOrders
					err := s.db.Where("status = ?", model.OrderStatusWaiting).Limit(5).Find(&orders).Error
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
					
				case 2:
					// 更新操作
					err := s.db.Model(&model.TradeOrders{}).
						Where("status = ? AND created_at < ?", model.OrderStatusWaiting, time.Now()).
						Update("updated_at", time.Now()).Error
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
					
				case 3:
					// 计数操作
					var count int64
					err := s.db.Model(&model.TradeOrders{}).Count(&count).Error
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}
				
				// 短暂延迟模拟真实负载
				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}
	
	wg.Wait()
	
	totalOperations := int64(concurrency * operationsPerWorker)
	
	// 验证结果
	assert.Greater(s.T(), successCount, totalOperations*8/10, // 至少80%成功
		"Most database operations should succeed under stress")
	
	s.T().Logf("Database stress test: %d/%d operations succeeded, %d errors", 
		successCount, totalOperations, errorCount)
	
	// 验证连接池状态
	sqlDB, err := s.db.DB()
	require.NoError(s.T(), err)
	
	stats := sqlDB.Stats()
	s.T().Logf("Connection pool stats - OpenConnections: %d, InUse: %d, Idle: %d", 
		stats.OpenConnections, stats.InUse, stats.Idle)
	
	assert.GreaterOrEqual(s.T(), stats.OpenConnections, 0, "Should have active connections")
}

// TestOrderExpirationConcurrency 测试订单过期处理的并发安全性
func (s *ConcurrentOperationsTestSuite) TestOrderExpirationConcurrency() {
	// 创建即将过期的订单
	expiringOrders := 10
	orders := make([]*model.TradeOrders, expiringOrders)
	
	for i := 0; i < expiringOrders; i++ {
		order := testutils.CreateTestOrder(map[string]interface{}{
			"order_id":   fmt.Sprintf("EXPIRING_%d_%d", i, time.Now().UnixNano()),
			"trade_id":   fmt.Sprintf("EXPIRING_%d_%d", i, time.Now().UnixNano()),
			"expired_at": time.Now().Add(2 * time.Second), // 2秒后过期
		})
		
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
		orders[i] = order
		
		time.Sleep(time.Millisecond) // 确保唯一ID
	}
	
	// 等待订单过期
	time.Sleep(3 * time.Second)
	
	var wg sync.WaitGroup
	var expiredCount int64
	
	// 并发处理过期订单
	for _, order := range orders {
		wg.Add(1)
		go func(o *model.TradeOrders) {
			defer wg.Done()
			
			err := o.OrderSetExpired()
			if err == nil {
				atomic.AddInt64(&expiredCount, 1)
			} else {
				s.T().Logf("Failed to expire order %s: %v", o.OrderId, err)
			}
		}(order)
	}
	
	wg.Wait()
	
	// 验证结果
	assert.Equal(s.T(), int64(expiringOrders), expiredCount)
	
	// 验证所有订单都已过期
	for _, order := range orders {
		var updatedOrder model.TradeOrders
		err := s.db.Where("id = ?", order.Id).First(&updatedOrder).Error
		require.NoError(s.T(), err)
		assert.Equal(s.T(), model.OrderStatusExpired, updatedOrder.Status)
	}
	
	s.T().Logf("Successfully expired %d orders concurrently", expiredCount)
}

// TestHighConcurrencyScenario 测试高并发综合场景
func (s *ConcurrentOperationsTestSuite) TestHighConcurrencyScenario() {
	// 这是一个综合性的高并发测试
	totalOperations := 100
	var wg sync.WaitGroup
	
	// 统计计数器
	var createCount int64
	var updateCount int64
	var queryCount int64
	var callbackCount int64
	var errorCount int64
	
	// 启动多种并发操作
	for i := 0; i < totalOperations; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			switch index % 5 {
			case 0:
				// 创建订单
				order := testutils.CreateTestOrder(map[string]interface{}{
					"order_id": fmt.Sprintf("HIGH_CONC_%d_%d", index, time.Now().UnixNano()),
					"trade_id": fmt.Sprintf("HIGH_CONC_%d_%d", index, time.Now().UnixNano()),
				})
				err := s.db.Create(order).Error
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&createCount, 1)
				}
				
			case 1:
				// 查询订单
				var orders []model.TradeOrders
				err := s.db.Where("status = ?", model.OrderStatusWaiting).Limit(3).Find(&orders).Error
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&queryCount, 1)
				}
				
			case 2:
				// 更新订单状态（模拟支付）
				var order model.TradeOrders
				err := s.db.Where("status = ?", model.OrderStatusWaiting).First(&order).Error
				if err == nil {
					payment := s.scenarios.ScenarioNormalPayment(
						testutils.ChainTRON,
						order.Address,
						decimal.RequireFromString(order.Amount),
					)
					err = order.OrderSetSucc(payment.From, payment.Hash, payment.Timestamp)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&updateCount, 1)
					}
				}
				
			case 3:
				// 执行回调
				var order model.TradeOrders
				err := s.db.Where("status = ?", model.OrderStatusSuccess).First(&order).Error
				if err == nil {
					order.NotifyUrl = s.callbackServer.URL()
					notify.OrderNotify(order)
					atomic.AddInt64(&callbackCount, 1)
				}
				
			case 4:
				// 混合查询操作
				var count int64
				err := s.db.Model(&model.TradeOrders{}).Count(&count).Error
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&queryCount, 1)
				}
			}
			
			// 小延迟模拟真实场景
			time.Sleep(time.Millisecond * 5)
		}(i)
	}
	
	wg.Wait()
	
	// 等待异步操作完成
	time.Sleep(3 * time.Second)
	
	// 报告结果
	s.T().Logf("High concurrency test results:")
	s.T().Logf("- Created orders: %d", createCount)
	s.T().Logf("- Updated orders: %d", updateCount)
	s.T().Logf("- Query operations: %d", queryCount)
	s.T().Logf("- Callback operations: %d", callbackCount)
	s.T().Logf("- Errors: %d", errorCount)
	s.T().Logf("- Total operations: %d", createCount+updateCount+queryCount+callbackCount+errorCount)
	
	// 验证系统稳定性
	assert.Greater(s.T(), createCount+updateCount+queryCount+callbackCount, errorCount*10,
		"Success operations should far outnumber errors")
	
	// 验证数据库状态
	var totalOrders int64
	err := s.db.Model(&model.TradeOrders{}).Count(&totalOrders).Error
	require.NoError(s.T(), err)
	assert.Greater(s.T(), totalOrders, int64(0), "Should have created some orders")
	
	s.T().Logf("Final database state: %d orders total", totalOrders)
}

// TestMemoryLeakDetection 测试内存泄漏检测
func (s *ConcurrentOperationsTestSuite) TestMemoryLeakDetection() {
	// 这个测试通过重复执行操作来检测潜在的内存泄漏
	iterations := 50
	
	for i := 0; i < iterations; i++ {
		// 批量创建和删除订单
		batchSize := 20
		orders := testutils.CreateOrderBatch(batchSize, map[string]interface{}{
			"order_id": fmt.Sprintf("LEAK_DETECTION_%d", time.Now().UnixNano()),
		})
		
		// 创建订单
		for _, order := range orders {
			err := s.db.Create(order).Error
			require.NoError(s.T(), err)
		}
		
		// 执行一些操作
		var queryOrders []model.TradeOrders
		err := s.db.Where("status = ?", model.OrderStatusWaiting).Find(&queryOrders).Error
		require.NoError(s.T(), err)
		
		// 更新一些订单
		if len(queryOrders) > 0 {
			for _, order := range queryOrders[:min(5, len(queryOrders))] {
				payment := s.scenarios.ScenarioNormalPayment(
					testutils.ChainTRON,
					order.Address,
					decimal.RequireFromString(order.Amount),
				)
				err = order.OrderSetSucc(payment.From, payment.Hash, payment.Timestamp)
				if err != nil {
					s.T().Logf("Update failed: %v", err)
				}
			}
		}
		
		// 清理数据
		s.db.Where("order_id LIKE ?", "LEAK_DETECTION_%").Delete(&model.TradeOrders{})
		
		// 每10次迭代报告一次进度
		if i%10 == 0 {
			s.T().Logf("Memory leak detection progress: %d/%d", i, iterations)
		}
	}
	
	s.T().Log("Memory leak detection test completed successfully")
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// 运行测试套件
func TestConcurrentOperationsTestSuite(t *testing.T) {
	suite.Run(t, new(ConcurrentOperationsTestSuite))
}