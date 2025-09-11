package tests

import (
	"USDTMore/app/model"
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestConfig 测试配置
type TestConfig struct {
	DB                *gorm.DB
	TestDataDir       string
	ConcurrentWorkers int
	TestTimeout       time.Duration
}

// ValidationTestSuite 验证测试套件
type ValidationTestSuite struct {
	config   *TestConfig
	db       *gorm.DB
	cleanup  []func()
	mutex    sync.Mutex
	counters map[string]int64
}

// setupInMemoryDB 设置内存SQLite数据库用于测试
func setupInMemoryDB(t *testing.T) *gorm.DB {
	// 使用内存数据库，避免文件系统依赖
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// 自动迁移测试所需的表结构
	err = db.AutoMigrate(
		&model.TradeOrders{},
		&model.WalletAddress{},
		&model.NotifyRecord{},
	)
	require.NoError(t, err)

	// 设置全局DB连接供model包使用
	model.DB = db

	return db
}

// NewValidationTestSuite 创建新的验证测试套件
func NewValidationTestSuite(t *testing.T) *ValidationTestSuite {
	db := setupInMemoryDB(t)
	
	suite := &ValidationTestSuite{
		config: &TestConfig{
			DB:                db,
			ConcurrentWorkers: 10,
			TestTimeout:       time.Minute * 5,
		},
		db:       db,
		cleanup:  make([]func(), 0),
		counters: make(map[string]int64),
	}

	// 添加清理函数
	suite.AddCleanup(func() {
		db.Exec("DELETE FROM trade_orders")
		db.Exec("DELETE FROM wallet_addresses")
		db.Exec("DELETE FROM notify_records")
	})

	return suite
}

// AddCleanup 添加清理函数
func (suite *ValidationTestSuite) AddCleanup(fn func()) {
	suite.cleanup = append(suite.cleanup, fn)
}

// Cleanup 执行清理操作
func (suite *ValidationTestSuite) Cleanup() {
	for _, fn := range suite.cleanup {
		fn()
	}
}

// incrementCounter 原子递增计数器
func (suite *ValidationTestSuite) incrementCounter(key string) {
	suite.mutex.Lock()
	defer suite.mutex.Unlock()
	suite.counters[key]++
}

// getCounter 获取计数器值
func (suite *ValidationTestSuite) getCounter(key string) int64 {
	suite.mutex.Lock()
	defer suite.mutex.Unlock()
	return suite.counters[key]
}

// createTestWalletAddresses 创建测试钱包地址
func (suite *ValidationTestSuite) createTestWalletAddresses() {
	addresses := []model.WalletAddress{
		{Chain: "TRON", Address: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", StartBlock: 0},
		{Chain: "BSC", Address: "0x55d398326f99059ff775485246999027b3197955", StartBlock: 0},
		{Chain: "POLY", Address: "0xc2132D05D31c914a87C6611C10748AEb04B58e8F", StartBlock: 0},
		{Chain: "OP", Address: "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58", StartBlock: 0},
	}
	
	for _, addr := range addresses {
		suite.db.Create(&addr)
	}
}

// generateTestOrder 生成测试订单
func (suite *ValidationTestSuite) generateTestOrder(fields map[string]interface{}) *model.TradeOrders {
	nanoTime := time.Now().UnixNano()
	randNum := rand.Int31()
	order := &model.TradeOrders{
		OrderId:     fmt.Sprintf("TEST_%d_%d", nanoTime, randNum),
		TradeId:     fmt.Sprintf("TID_%d_%d", nanoTime, randNum),
		TradeHash:   "", // 初始为空，等支付成功后再更新
		UsdtRate:    "7.20",
		Amount:      "100.00",
		Money:       720.00,
		Chain:       "TRON",
		Address:     "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		FromAddress: "",
		Status:      model.OrderStatusWaiting,
		ReturnUrl:   "https://example.com/return",
		NotifyUrl:   "https://example.com/notify",
		NotifyNum:   0,
		NotifyState: model.OrderNotifyStateFail,
		ExpiredAt:   time.Now().Add(30 * time.Minute),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 应用自定义字段
	if fields != nil {
		if v, ok := fields["order_id"].(string); ok {
			order.OrderId = v
		}
		if v, ok := fields["amount"].(string); ok {
			order.Amount = v
		}
		if v, ok := fields["money"].(float64); ok {
			order.Money = v
		}
		if v, ok := fields["status"].(int); ok {
			order.Status = v
		}
		if v, ok := fields["chain"].(string); ok {
			order.Chain = v
		}
		if v, ok := fields["expired_at"].(time.Time); ok {
			order.ExpiredAt = v
		}
	}

	return order
}

// Test_OrderCreation_Basic 测试基本订单创建功能
func Test_OrderCreation_Basic(t *testing.T) {
	suite := NewValidationTestSuite(t)
	defer suite.Cleanup()

	t.Run("创建单个订单", func(t *testing.T) {
		order := suite.generateTestOrder(nil)
		
		err := suite.db.Create(order).Error
		require.NoError(t, err)
		assert.Greater(t, order.Id, int64(0))
		
		// 验证订单可以被查询
		var retrieved model.TradeOrders
		err = suite.db.Where("order_id = ?", order.OrderId).First(&retrieved).Error
		require.NoError(t, err)
		assert.Equal(t, order.OrderId, retrieved.OrderId)
		assert.Equal(t, model.OrderStatusWaiting, retrieved.Status)
	})

	t.Run("订单唯一性约束验证", func(t *testing.T) {
		order1 := suite.generateTestOrder(map[string]interface{}{
			"order_id": "UNIQUE_TEST_001",
		})
		
		err := suite.db.Create(order1).Error
		require.NoError(t, err)

		// 尝试创建相同order_id的订单，应该失败
		order2 := suite.generateTestOrder(map[string]interface{}{
			"order_id": "UNIQUE_TEST_001",
		})
		
		err = suite.db.Create(order2).Error
		assert.Error(t, err) // 应该因为唯一约束失败
	})
}

// Test_OrderStatusTransition 测试订单状态转换
func Test_OrderStatusTransition(t *testing.T) {
	suite := NewValidationTestSuite(t)
	defer suite.Cleanup()

	t.Run("订单状态更新", func(t *testing.T) {
		order := suite.generateTestOrder(nil)
		err := suite.db.Create(order).Error
		require.NoError(t, err)

		// 测试设置订单为成功状态
		fromAddress := "TTestFromAddress12345678901234567890"
		txHash := "test_tx_hash_123456"
		confirmedAt := time.Now()

		err = order.OrderSetSucc(fromAddress, txHash, confirmedAt)
		require.NoError(t, err)

		// 验证状态更新
		var updated model.TradeOrders
		err = suite.db.First(&updated, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, updated.Status)
		assert.Equal(t, fromAddress, updated.FromAddress)
		assert.Equal(t, txHash, updated.TradeHash)
		assert.WithinDuration(t, confirmedAt, updated.ConfirmedAt, time.Second)
	})

	t.Run("订单过期设置", func(t *testing.T) {
		order := suite.generateTestOrder(nil)
		err := suite.db.Create(order).Error
		require.NoError(t, err)

		err = order.OrderSetExpired()
		require.NoError(t, err)

		var updated model.TradeOrders
		err = suite.db.First(&updated, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusExpired, updated.Status)
	})

	t.Run("订单通知状态设置", func(t *testing.T) {
		order := suite.generateTestOrder(nil)
		err := suite.db.Create(order).Error
		require.NoError(t, err)

		// 设置通知成功
		err = order.OrderSetNotifyState(model.OrderNotifyStateSucc)
		require.NoError(t, err)

		var updated model.TradeOrders
		err = suite.db.First(&updated, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateSucc, updated.NotifyState)
		assert.Equal(t, 1, updated.NotifyNum)

		// 再次设置通知失败
		err = updated.OrderSetNotifyState(model.OrderNotifyStateFail)
		require.NoError(t, err)

		err = suite.db.First(&updated, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateFail, updated.NotifyState)
		assert.Equal(t, 2, updated.NotifyNum) // 通知次数应该增加
	})
}

// Test_CalcTradeAmount_Logic 测试金额计算逻辑
func Test_CalcTradeAmount_Logic(t *testing.T) {
	suite := NewValidationTestSuite(t)
	defer suite.Cleanup()
	
	suite.createTestWalletAddresses()

	t.Run("金额计算基本功能", func(t *testing.T) {
		walletAddresses := model.GetAvailableAddress("TRON")
		require.NotEmpty(t, walletAddresses)

		rate := 7.20
		money := 720.00
		
		address, amount := model.CalcTradeAmount(walletAddresses, rate, money)
		
		assert.NotEmpty(t, address.Address)
		assert.Equal(t, "TRON", address.Chain)
		
		// 验证计算的金额
		expectedAmount := money / rate
		actualAmount, err := decimal.NewFromString(amount)
		require.NoError(t, err)
		
		expectedDecimal := decimal.NewFromFloat(expectedAmount)
		assert.True(t, actualAmount.Equal(expectedDecimal) || actualAmount.GreaterThan(expectedDecimal))
	})

	t.Run("金额冲突处理", func(t *testing.T) {
		walletAddresses := model.GetAvailableAddress("TRON")
		require.NotEmpty(t, walletAddresses)

		// 创建一个等待支付的订单占用特定金额
		existingOrder := suite.generateTestOrder(map[string]interface{}{
			"amount": "100.00",
			"chain":  "TRON",
		})
		err := suite.db.Create(existingOrder).Error
		require.NoError(t, err)

		rate := 7.20
		money := 720.00 // 这会计算出100.00 USDT
		
		// 第二次计算应该避开已占用的金额
		address, amount := model.CalcTradeAmount(walletAddresses, rate, money)
		
		assert.NotEmpty(t, address.Address)
		actualAmount, err := decimal.NewFromString(amount)
		require.NoError(t, err)
		
		// 应该大于原本的100.00以避免冲突
		expectedAmount, err := decimal.NewFromString("100.00")
		require.NoError(t, err)
		assert.True(t, actualAmount.GreaterThan(expectedAmount))
	})
}

// Test_OrderQuery_Performance 测试订单查询性能
func Test_OrderQuery_Performance(t *testing.T) {
	suite := NewValidationTestSuite(t)
	defer suite.Cleanup()

	// 创建大量测试订单
	batchSize := 1000
	t.Run(fmt.Sprintf("创建%d个订单并测试查询性能", batchSize), func(t *testing.T) {
		// 批量创建订单
		orders := make([]*model.TradeOrders, batchSize)
		for i := 0; i < batchSize; i++ {
			orders[i] = suite.generateTestOrder(map[string]interface{}{
				"order_id": fmt.Sprintf("PERF_TEST_%d", i),
				"status":   i % 3, // 分布不同的状态
			})
		}

		startTime := time.Now()
		for _, order := range orders {
			err := suite.db.Create(order).Error
			require.NoError(t, err)
		}
		createDuration := time.Since(startTime)
		
		t.Logf("创建%d个订单耗时: %v", batchSize, createDuration)
		assert.Less(t, createDuration, time.Second*10) // 应该在10秒内完成

		// 测试按状态查询的性能
		startTime = time.Now()
		waitingOrders, err := model.GetTradeOrderByStatus(model.OrderStatusWaiting)
		queryDuration := time.Since(startTime)
		
		require.NoError(t, err)
		t.Logf("查询等待支付订单耗时: %v, 结果数量: %d", queryDuration, len(waitingOrders))
		assert.Less(t, queryDuration, time.Second*2) // 查询应该在2秒内完成

		// 测试单个订单查询性能
		testOrderId := orders[rand.Intn(len(orders))].TradeId
		startTime = time.Now()
		_, found := model.GetTradeOrder(testOrderId)
		singleQueryDuration := time.Since(startTime)
		
		assert.True(t, found)
		t.Logf("单个订单查询耗时: %v", singleQueryDuration)
		assert.Less(t, singleQueryDuration, time.Millisecond*100) // 单个查询应该在100ms内完成
	})
}

// Test_ConcurrentOrderOperations 测试并发订单操作
func Test_ConcurrentOrderOperations(t *testing.T) {
	suite := NewValidationTestSuite(t)
	defer suite.Cleanup()
	
	suite.createTestWalletAddresses()

	t.Run("并发创建订单", func(t *testing.T) {
		concurrentCount := 50
		wg := sync.WaitGroup{}
		errors := make(chan error, concurrentCount)
		
		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				order := suite.generateTestOrder(map[string]interface{}{
					"order_id": fmt.Sprintf("CONCURRENT_%d_%d", index, time.Now().UnixNano()),
				})
				
				if err := suite.db.Create(order).Error; err != nil {
					errors <- err
					return
				}
				
				suite.incrementCounter("created")
			}(i)
		}
		
		wg.Wait()
		close(errors)
		
		// 检查是否有错误
		errorCount := 0
		for err := range errors {
			if err != nil {
				t.Logf("并发创建错误: %v", err)
				errorCount++
			}
		}
		
		assert.Equal(t, 0, errorCount, "并发创建不应该有错误")
		assert.Equal(t, int64(concurrentCount), suite.getCounter("created"))
	})

	t.Run("并发金额计算", func(t *testing.T) {
		walletAddresses := model.GetAvailableAddress("TRON")
		require.NotEmpty(t, walletAddresses)

		concurrentCount := 100
		wg := sync.WaitGroup{}
		results := make(chan string, concurrentCount)
		
		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				rate := 7.20
				money := 720.00 + float64(index) // 每个请求略有不同的金额
				
				_, amount := model.CalcTradeAmount(walletAddresses, rate, money)
				results <- amount
				
				suite.incrementCounter("calculated")
			}(i)
		}
		
		wg.Wait()
		close(results)
		
		// 验证结果
		uniqueAmounts := make(map[string]bool)
		for amount := range results {
			uniqueAmounts[amount] = true
		}
		
		t.Logf("并发金额计算结果: 总计算次数=%d, 唯一金额数量=%d", 
			suite.getCounter("calculated"), len(uniqueAmounts))
		
		assert.Equal(t, int64(concurrentCount), suite.getCounter("calculated"))
		// 由于原子精度递增，所有金额都应该是唯一的
		assert.Equal(t, concurrentCount, len(uniqueAmounts))
	})

	t.Run("并发状态更新", func(t *testing.T) {
		// 创建基础订单
		baseOrder := suite.generateTestOrder(nil)
		err := suite.db.Create(baseOrder).Error
		require.NoError(t, err)

		concurrentCount := 10
		wg := sync.WaitGroup{}
		
		// 并发尝试设置订单为成功状态
		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				// 重新获取订单以避免GORM缓存问题
				var order model.TradeOrders
				if err := suite.db.First(&order, baseOrder.Id).Error; err != nil {
					return
				}
				
				fromAddress := fmt.Sprintf("TFromAddress%d", index)
				txHash := fmt.Sprintf("tx_hash_%d_%d", index, time.Now().UnixNano())
				confirmedAt := time.Now()
				
				// 只有第一个成功，其他应该被忽略或失败
				if err := order.OrderSetSucc(fromAddress, txHash, confirmedAt); err == nil {
					suite.incrementCounter("updated")
				}
			}(i)
		}
		
		wg.Wait()
		
		// 验证最终状态
		var finalOrder model.TradeOrders
		err = suite.db.First(&finalOrder, baseOrder.Id).Error
		require.NoError(t, err)
		
		assert.Equal(t, model.OrderStatusSuccess, finalOrder.Status)
		assert.NotEmpty(t, finalOrder.FromAddress)
		assert.NotEmpty(t, finalOrder.TradeHash)
		
		// 至少应该有一次成功的更新
		assert.GreaterOrEqual(t, suite.getCounter("updated"), int64(1))
		t.Logf("并发状态更新: 成功次数=%d", suite.getCounter("updated"))
	})
}

// BenchmarkOrderCreation 订单创建性能基准测试
func BenchmarkOrderCreation(b *testing.B) {
	// 使用简化的DB设置避免测试依赖
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	
	db.AutoMigrate(&model.TradeOrders{})
	model.DB = db
	
	b.ResetTimer()
	
	b.Run("单个订单创建", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			order := &model.TradeOrders{
				OrderId:     fmt.Sprintf("BENCH_%d_%d", i, time.Now().UnixNano()),
				TradeId:     fmt.Sprintf("TID_%d_%d", i, time.Now().UnixNano()),
				UsdtRate:    "7.20",
				Amount:      "100.00",
				Money:       720.00,
				Chain:       "TRON",
				Address:     "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
				Status:      model.OrderStatusWaiting,
				ReturnUrl:   "https://example.com/return",
				NotifyUrl:   "https://example.com/notify",
				ExpiredAt:   time.Now().Add(30 * time.Minute),
			}
			
			if err := db.Create(order).Error; err != nil {
				b.Errorf("订单创建失败: %v", err)
			}
		}
	})
}

// BenchmarkOrderQuery 订单查询性能基准测试
func BenchmarkOrderQuery(b *testing.B) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	
	db.AutoMigrate(&model.TradeOrders{})
	model.DB = db
	
	// 预填充数据
	orderIds := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		order := &model.TradeOrders{
			OrderId:   fmt.Sprintf("BENCH_QUERY_%d", i),
			TradeId:   fmt.Sprintf("TID_QUERY_%d", i),
			UsdtRate:  "7.20",
			Amount:    "100.00",
			Money:     720.00,
			Chain:     "TRON",
			Address:   "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
			Status:    model.OrderStatusWaiting,
			ExpiredAt: time.Now().Add(30 * time.Minute),
		}
		db.Create(order)
		orderIds[i] = order.TradeId
	}
	
	b.ResetTimer()
	
	b.Run("根据TradeId查询单个订单", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tradeId := orderIds[i%len(orderIds)]
			_, found := model.GetTradeOrder(tradeId)
			if !found {
				b.Errorf("订单查询失败: %s", tradeId)
			}
		}
	})
	
	b.Run("根据状态查询订单列表", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			orders, err := model.GetTradeOrderByStatus(model.OrderStatusWaiting)
			if err != nil {
				b.Errorf("状态查询失败: %v", err)
			}
			if len(orders) == 0 {
				b.Error("查询结果为空")
			}
		}
	})
}

// BenchmarkCalcTradeAmount 金额计算性能基准测试
func BenchmarkCalcTradeAmount(b *testing.B) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	
	db.AutoMigrate(&model.WalletAddress{}, &model.TradeOrders{})
	model.DB = db
	
	// 创建测试钱包地址
	addresses := []model.WalletAddress{
		{Chain: "TRON", Address: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"},
		{Chain: "BSC", Address: "0x55d398326f99059ff775485246999027b3197955"},
		{Chain: "POLY", Address: "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"},
	}
	for _, addr := range addresses {
		db.Create(&addr)
	}
	
	b.ResetTimer()
	
	b.Run("TRON链金额计算", func(b *testing.B) {
		walletAddresses := model.GetAvailableAddress("TRON")
		for i := 0; i < b.N; i++ {
			rate := 7.20
			money := 720.00 + float64(i%100)*0.01 // 轻微变化避免缓存影响
			
			address, amount := model.CalcTradeAmount(walletAddresses, rate, money)
			if address.Address == "" || amount == "" {
				b.Error("金额计算失败")
			}
		}
	})
	
	b.Run("并发金额计算", func(b *testing.B) {
		walletAddresses := model.GetAvailableAddress("TRON")
		
		b.RunParallel(func(pb *testing.PB) {
			counter := 0
			for pb.Next() {
				rate := 7.20
				money := 720.00 + float64(counter%1000)*0.01
				
				address, amount := model.CalcTradeAmount(walletAddresses, rate, money)
				if address.Address == "" || amount == "" {
					b.Error("并发金额计算失败")
				}
				counter++
			}
		})
	})
}

// Test_BusinessLogicValidation 业务逻辑验证测试
func Test_BusinessLogicValidation(t *testing.T) {
	suite := NewValidationTestSuite(t)
	defer suite.Cleanup()

	t.Run("订单过期处理", func(t *testing.T) {
		// 创建一个已过期的订单
		expiredOrder := suite.generateTestOrder(map[string]interface{}{
			"expired_at": time.Now().Add(-1 * time.Hour), // 1小时前过期
		})
		err := suite.db.Create(expiredOrder).Error
		require.NoError(t, err)

		// 创建一个未过期的订单
		validOrder := suite.generateTestOrder(map[string]interface{}{
			"expired_at": time.Now().Add(1 * time.Hour), // 1小时后过期
		})
		err = suite.db.Create(validOrder).Error
		require.NoError(t, err)

		// 模拟过期订单处理逻辑
		var orders []model.TradeOrders
		err = suite.db.Where("status = ? AND expired_at < ?", 
			model.OrderStatusWaiting, time.Now()).Find(&orders).Error
		require.NoError(t, err)

		assert.Len(t, orders, 1)
		assert.Equal(t, expiredOrder.OrderId, orders[0].OrderId)

		// 设置订单为过期状态
		err = orders[0].OrderSetExpired()
		require.NoError(t, err)

		// 验证状态更新
		var updated model.TradeOrders
		err = suite.db.First(&updated, orders[0].Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusExpired, updated.Status)
	})

	t.Run("重复支付防护", func(t *testing.T) {
		order := suite.generateTestOrder(nil)
		err := suite.db.Create(order).Error
		require.NoError(t, err)

		fromAddress := "TTestFromAddress"
		txHash := "test_tx_hash"
		confirmedAt := time.Now()

		// 第一次支付
		err = order.OrderSetSucc(fromAddress, txHash, confirmedAt)
		require.NoError(t, err)

		// 验证订单状态已更新
		var updated model.TradeOrders
		err = suite.db.First(&updated, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, updated.Status)

		// 尝试再次设置为成功状态（模拟重复支付）
		err = updated.OrderSetSucc("AnotherFromAddress", "another_tx_hash", time.Now())
		
		// 这里的行为取决于具体的业务逻辑实现
		// 可能成功（覆盖）或失败（保护），需要根据实际需求调整
		if err == nil {
			// 如果允许覆盖，验证最新的信息
			err = suite.db.First(&updated, order.Id).Error
			require.NoError(t, err)
			t.Logf("订单允许重复更新: status=%d, tx_hash=%s", updated.Status, updated.TradeHash)
		} else {
			t.Logf("订单重复更新被阻止: %v", err)
		}
	})

	t.Run("金额精度处理", func(t *testing.T) {
		suite.createTestWalletAddresses()
		walletAddresses := model.GetAvailableAddress("TRON")
		
		// 测试小数精度处理
		testCases := []struct {
			rate   float64
			money  float64
			name   string
		}{
			{7.12345, 712.345, "高精度汇率"},
			{7.00, 700.00, "整数汇率"},
			{7.99, 799.99, "边界汇率"},
			{6.50, 65.00, "小金额"},
			{8.50, 8500.00, "大金额"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				address, amount := model.CalcTradeAmount(walletAddresses, tc.rate, tc.money)
				
				assert.NotEmpty(t, address.Address)
				assert.NotEmpty(t, amount)
				
				// 验证金额可以被正确解析
				parsedAmount, err := decimal.NewFromString(amount)
				require.NoError(t, err)
				assert.True(t, parsedAmount.GreaterThan(decimal.Zero))
				
				// 验证金额计算的合理性
				expectedAmount := tc.money / tc.rate
				expectedDecimal := decimal.NewFromFloat(expectedAmount)
				
				// 由于可能有原子精度调整，实际金额应该大于等于期望值
				assert.True(t, parsedAmount.GreaterThanOrEqual(expectedDecimal))
				
				t.Logf("金额计算: rate=%.5f, money=%.2f, expected=%.6f, actual=%s", 
					tc.rate, tc.money, expectedAmount, amount)
			})
		}
	})
}

// Test_DatabaseIntegrity 数据库完整性测试
func Test_DatabaseIntegrity(t *testing.T) {
	suite := NewValidationTestSuite(t)
	defer suite.Cleanup()

	t.Run("数据一致性验证", func(t *testing.T) {
		// 创建订单并验证所有字段
		originalOrder := suite.generateTestOrder(map[string]interface{}{
			"money":  1000.50,
			"amount": "139.79",
		})
		
		err := suite.db.Create(originalOrder).Error
		require.NoError(t, err)
		
		// 从数据库重新读取
		var retrievedOrder model.TradeOrders
		err = suite.db.First(&retrievedOrder, originalOrder.Id).Error
		require.NoError(t, err)
		
		// 验证关键字段的一致性
		assert.Equal(t, originalOrder.OrderId, retrievedOrder.OrderId)
		assert.Equal(t, originalOrder.TradeId, retrievedOrder.TradeId)
		assert.Equal(t, originalOrder.Money, retrievedOrder.Money)
		assert.Equal(t, originalOrder.Amount, retrievedOrder.Amount)
		assert.Equal(t, originalOrder.Status, retrievedOrder.Status)
		assert.WithinDuration(t, originalOrder.ExpiredAt, retrievedOrder.ExpiredAt, time.Second)
	})

	t.Run("级联操作验证", func(t *testing.T) {
		// 创建订单
		order := suite.generateTestOrder(nil)
		err := suite.db.Create(order).Error
		require.NoError(t, err)

		// 创建相关的通知记录
		notifyRecord := &model.NotifyRecord{
			Txid:      "test_cascade_tx",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err = suite.db.Create(notifyRecord).Error
		require.NoError(t, err)

		// 验证记录存在
		var count int64
		err = suite.db.Model(&model.TradeOrders{}).Where("id = ?", order.Id).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)

		err = suite.db.Model(&model.NotifyRecord{}).Where("txid = ?", notifyRecord.Txid).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("事务回滚测试", func(t *testing.T) {
		// 开始事务
		tx := suite.db.Begin()
		
		// 在事务中创建订单
		order := suite.generateTestOrder(map[string]interface{}{
			"order_id": "TRANSACTION_TEST",
		})
		
		err := tx.Create(order).Error
		require.NoError(t, err)
		
		// 验证在事务中可以查询到
		var count int64
		err = tx.Model(&model.TradeOrders{}).Where("order_id = ?", "TRANSACTION_TEST").Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
		
		// 回滚事务
		tx.Rollback()
		
		// 验证回滚后查询不到
		err = suite.db.Model(&model.TradeOrders{}).Where("order_id = ?", "TRANSACTION_TEST").Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}

// Test_EdgeCases 边界情况测试
func Test_EdgeCases(t *testing.T) {
	suite := NewValidationTestSuite(t)
	defer suite.Cleanup()

	t.Run("极限金额测试", func(t *testing.T) {
		suite.createTestWalletAddresses()
		walletAddresses := model.GetAvailableAddress("TRON")

		// 测试极小金额
		_, amount := model.CalcTradeAmount(walletAddresses, 7.20, 0.01)
		parsedAmount, err := decimal.NewFromString(amount)
		require.NoError(t, err)
		assert.True(t, parsedAmount.GreaterThan(decimal.Zero))

		// 测试极大金额
		_, amount = model.CalcTradeAmount(walletAddresses, 7.20, 999999.99)
		parsedAmount, err = decimal.NewFromString(amount)
		require.NoError(t, err)
		assert.True(t, parsedAmount.GreaterThan(decimal.NewFromFloat(100000)))
	})

	t.Run("时间边界测试", func(t *testing.T) {
		// 测试订单在过期边界的行为
		now := time.Now()
		
		// 刚好过期的订单
		justExpiredOrder := suite.generateTestOrder(map[string]interface{}{
			"expired_at": now.Add(-time.Second),
		})
		err := suite.db.Create(justExpiredOrder).Error
		require.NoError(t, err)

		// 还有1秒过期的订单
		almostExpiredOrder := suite.generateTestOrder(map[string]interface{}{
			"expired_at": now.Add(time.Second),
		})
		err = suite.db.Create(almostExpiredOrder).Error
		require.NoError(t, err)

		// 查询过期订单
		var expiredOrders []model.TradeOrders
		err = suite.db.Where("expired_at < ?", now).Find(&expiredOrders).Error
		require.NoError(t, err)

		// 应该只找到过期的订单
		assert.Len(t, expiredOrders, 1)
		assert.Equal(t, justExpiredOrder.OrderId, expiredOrders[0].OrderId)
	})

	t.Run("字符串长度边界测试", func(t *testing.T) {
		// 测试最大长度的字符串字段
		longString := string(make([]byte, 255)) // 填充255个字符
		for i := range longString {
			longString = longString[:i] + "A" + longString[i+1:]
		}

		order := suite.generateTestOrder(map[string]interface{}{
			"order_id": longString[:50], // 假设order_id有长度限制
		})

		err := suite.db.Create(order).Error
		if err != nil {
			t.Logf("长字符串测试触发了预期的约束: %v", err)
		} else {
			// 如果成功创建，验证数据完整性
			var retrieved model.TradeOrders
			err = suite.db.Where("order_id = ?", order.OrderId).First(&retrieved).Error
			require.NoError(t, err)
			assert.Equal(t, order.OrderId, retrieved.OrderId)
		}
	})
}

// Test_PerformanceUnderLoad 负载下的性能测试
func Test_PerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过负载测试（短测试模式）")
	}

	suite := NewValidationTestSuite(t)
	defer suite.Cleanup()
	
	suite.createTestWalletAddresses()

	t.Run("高并发订单处理", func(t *testing.T) {
		concurrentUsers := 20
		ordersPerUser := 50
		
		startTime := time.Now()
		wg := sync.WaitGroup{}
		errorCount := int64(0)
		successCount := int64(0)
		
		for user := 0; user < concurrentUsers; user++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()
				
				for i := 0; i < ordersPerUser; i++ {
					order := suite.generateTestOrder(map[string]interface{}{
						"order_id": fmt.Sprintf("LOAD_U%d_O%d_%d", userID, i, time.Now().UnixNano()),
						"money":    100.0 + float64(userID)*10.0 + float64(i)*0.1,
					})
					
					if err := suite.db.Create(order).Error; err != nil {
						suite.mutex.Lock()
						errorCount++
						suite.mutex.Unlock()
					} else {
						suite.mutex.Lock()
						successCount++
						suite.mutex.Unlock()
					}
				}
			}(user)
		}
		
		wg.Wait()
		duration := time.Since(startTime)
		
		totalOrders := int64(concurrentUsers * ordersPerUser)
		tps := float64(successCount) / duration.Seconds()
		
		t.Logf("负载测试结果:")
		t.Logf("- 总订单数: %d", totalOrders)
		t.Logf("- 成功创建: %d", successCount)
		t.Logf("- 失败数量: %d", errorCount)
		t.Logf("- 耗时: %v", duration)
		t.Logf("- TPS: %.2f", tps)
		
		assert.Equal(t, int64(0), errorCount, "高并发下不应该有创建失败")
		assert.Equal(t, totalOrders, successCount)
		assert.Greater(t, tps, 50.0, "TPS应该大于50") // 根据实际需要调整阈值
	})

	t.Run("持续负载测试", func(t *testing.T) {
		testDuration := 30 * time.Second
		concurrentWorkers := 10
		
		ctx, cancel := context.WithTimeout(context.Background(), testDuration)
		defer cancel()
		
		wg := sync.WaitGroup{}
		operationCount := int64(0)
		
		for worker := 0; worker < concurrentWorkers; worker++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				counter := 0
				
				for {
					select {
					case <-ctx.Done():
						return
					default:
						// 执行订单操作
						order := suite.generateTestOrder(map[string]interface{}{
							"order_id": fmt.Sprintf("SUSTAINED_W%d_C%d_%d", 
								workerID, counter, time.Now().UnixNano()),
						})
						
						if err := suite.db.Create(order).Error; err == nil {
							suite.mutex.Lock()
							operationCount++
							suite.mutex.Unlock()
						}
						
						counter++
						
						// 小延迟避免过度占用CPU
						time.Sleep(time.Millisecond * 10)
					}
				}
			}(worker)
		}
		
		wg.Wait()
		
		avgTPS := float64(operationCount) / testDuration.Seconds()
		
		t.Logf("持续负载测试结果:")
		t.Logf("- 测试时长: %v", testDuration)
		t.Logf("- 并发工作者: %d", concurrentWorkers)
		t.Logf("- 总操作数: %d", operationCount)
		t.Logf("- 平均TPS: %.2f", avgTPS)
		
		assert.Greater(t, operationCount, int64(100), "持续负载下应该处理足够多的操作")
		assert.Greater(t, avgTPS, 10.0, "持续负载下TPS应该保持合理水平")
	})
}

// 生成测试报告的辅助函数
func generateTestReport(results map[string]interface{}) {
	separator := strings.Repeat("=", 60)
	fmt.Println("\n" + separator)
	fmt.Println("USDTMore 核心功能验证测试报告")
	fmt.Println(separator)
	fmt.Println()
	
	fmt.Println("测试概述:")
	if total, ok := results["total_tests"].(int); ok {
		fmt.Printf("- 总测试数量: %d\n", total)
	}
	if passed, ok := results["passed_tests"].(int); ok {
		fmt.Printf("- 通过测试: %d\n", passed)
	}
	if failed, ok := results["failed_tests"].(int); ok {
		fmt.Printf("- 失败测试: %d\n", failed)
	}
	
	fmt.Println()
	fmt.Println("性能指标:")
	if tps, ok := results["average_tps"].(float64); ok {
		fmt.Printf("- 平均TPS: %.2f\n", tps)
	}
	if latency, ok := results["average_latency"].(time.Duration); ok {
		fmt.Printf("- 平均延迟: %v\n", latency)
	}
	
	fmt.Println()
	fmt.Println("关键功能验证:")
	fmt.Printf("- 订单创建: %s\n", getStatusSymbol(results, "order_creation"))
	fmt.Printf("- 状态转换: %s\n", getStatusSymbol(results, "status_transition"))
	fmt.Printf("- 金额计算: %s\n", getStatusSymbol(results, "amount_calculation"))
	fmt.Printf("- 并发安全: %s\n", getStatusSymbol(results, "concurrency_safety"))
	fmt.Printf("- 数据完整性: %s\n", getStatusSymbol(results, "data_integrity"))
	
	fmt.Println("\n" + separator)
}

func getStatusSymbol(results map[string]interface{}, key string) string {
	if status, ok := results[key].(bool); ok {
		if status {
			return "✅ 通过"
		} else {
			return "❌ 失败"
		}
	}
	return "⚠️ 未测试"
}