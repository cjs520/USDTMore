package tests

import (
	"USDTMore/app/model"
	"fmt"
	"math/rand"
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

// setupSimpleTestDB 设置简单的测试数据库
func setupSimpleTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// 只迁移基本表结构，跳过唯一约束问题
	err = db.AutoMigrate(&model.WalletAddress{})
	require.NoError(t, err)

	// 手动创建订单表，移除problematic unique constraint
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS trade_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id VARCHAR(255) NOT NULL UNIQUE,
			trade_id VARCHAR(255) NOT NULL UNIQUE,
			trade_hash VARCHAR(64) DEFAULT '',
			usdt_rate VARCHAR(10) NOT NULL,
			amount DECIMAL(10,2) NOT NULL DEFAULT 0,
			money DECIMAL(10,2) NOT NULL DEFAULT 0,
			chain VARCHAR(255) NOT NULL,
			address VARCHAR(34) NOT NULL,
			from_address VARCHAR(34) NOT NULL DEFAULT '',
			status TINYINT(1) NOT NULL DEFAULT 0,
			return_url VARCHAR(255) NOT NULL DEFAULT '',
			notify_url VARCHAR(255) NOT NULL DEFAULT '',
			notify_num INT(11) NOT NULL DEFAULT 0,
			notify_state TINYINT(1) NOT NULL DEFAULT 0,
			expired_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			confirmed_at TIMESTAMP NULL
		)
	`).Error
	require.NoError(t, err)

	model.DB = db
	return db
}

// createSimpleTestOrder 创建简单的测试订单
func createSimpleTestOrder(customFields map[string]interface{}) *model.TradeOrders {
	nanoTime := time.Now().UnixNano()
	randNum := rand.Int31()
	
	order := &model.TradeOrders{
		OrderId:     fmt.Sprintf("SIMPLE_%d_%d", nanoTime, randNum),
		TradeId:     fmt.Sprintf("STID_%d_%d", nanoTime, randNum),
		TradeHash:   "",
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
	}

	// 应用自定义字段
	if customFields != nil {
		if v, ok := customFields["order_id"].(string); ok {
			order.OrderId = v
		}
		if v, ok := customFields["amount"].(string); ok {
			order.Amount = v
		}
		if v, ok := customFields["money"].(float64); ok {
			order.Money = v
		}
		if v, ok := customFields["status"].(int); ok {
			order.Status = v
		}
		if v, ok := customFields["chain"].(string); ok {
			order.Chain = v
		}
		if v, ok := customFields["expired_at"].(time.Time); ok {
			order.ExpiredAt = v
		}
	}

	return order
}

// Test_Simple_OrderCreation 简单订单创建测试
func Test_Simple_OrderCreation(t *testing.T) {
	db := setupSimpleTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	t.Run("基本订单创建", func(t *testing.T) {
		order := createSimpleTestOrder(nil)
		
		err := db.Create(order).Error
		require.NoError(t, err, "订单创建应该成功")
		assert.Greater(t, order.Id, int64(0), "订单ID应该大于0")

		// 验证订单可以被查询
		var retrieved model.TradeOrders
		err = db.Where("order_id = ?", order.OrderId).First(&retrieved).Error
		require.NoError(t, err, "应该能够查询到创建的订单")
		assert.Equal(t, order.OrderId, retrieved.OrderId)
		assert.Equal(t, model.OrderStatusWaiting, retrieved.Status)
	})

	t.Run("订单状态转换", func(t *testing.T) {
		order := createSimpleTestOrder(nil)
		err := db.Create(order).Error
		require.NoError(t, err)

		// 设置订单为成功状态
		fromAddress := "TTestFromAddress12345678901234567890"
		txHash := fmt.Sprintf("tx_%d", time.Now().UnixNano())
		confirmedAt := time.Now()

		err = order.OrderSetSucc(fromAddress, txHash, confirmedAt)
		require.NoError(t, err, "设置订单成功状态应该无错误")

		// 验证状态更新
		var updated model.TradeOrders
		err = db.First(&updated, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, updated.Status)
		assert.Equal(t, fromAddress, updated.FromAddress)
		assert.Equal(t, txHash, updated.TradeHash)

		t.Logf("订单状态更新成功: %s -> status=%d, tx_hash=%s", 
			order.OrderId, updated.Status, updated.TradeHash)
	})
}

// Test_Simple_CalcTradeAmount 简单金额计算测试
func Test_Simple_CalcTradeAmount(t *testing.T) {
	db := setupSimpleTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	// 创建测试钱包地址
	address := &model.WalletAddress{
		Chain:   "TRON",
		Address: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
	}
	err := db.Create(address).Error
	require.NoError(t, err)

	t.Run("金额计算基本功能", func(t *testing.T) {
		walletAddresses := []model.WalletAddress{*address}
		
		rate := 7.20
		money := 720.00
		
		resultAddress, amount := model.CalcTradeAmount(walletAddresses, rate, money)
		
		assert.NotEmpty(t, resultAddress.Address, "应该返回有效地址")
		assert.Equal(t, "TRON", resultAddress.Chain)
		assert.NotEmpty(t, amount, "应该返回有效金额")
		
		// 验证金额计算的合理性
		parsedAmount, err := decimal.NewFromString(amount)
		require.NoError(t, err)
		
		expectedAmount := money / rate
		expectedDecimal := decimal.NewFromFloat(expectedAmount)
		
		// 由于原子精度调整，实际金额应该大于等于期望值
		assert.True(t, parsedAmount.GreaterThanOrEqual(expectedDecimal),
			"计算的金额应该大于等于期望值")
		
		t.Logf("金额计算结果: rate=%.2f, money=%.2f, expected=%.2f, actual=%s",
			rate, money, expectedAmount, amount)
	})

	t.Run("金额冲突避免", func(t *testing.T) {
		walletAddresses := []model.WalletAddress{*address}
		
		// 创建一个占用特定金额的订单
		existingOrder := createSimpleTestOrder(map[string]interface{}{
			"amount": "100.00",
			"chain":  "TRON",
		})
		err := db.Create(existingOrder).Error
		require.NoError(t, err)

		rate := 7.20
		money := 720.00 // 这会计算出100.00 USDT

		// 第二次计算应该避开已占用的金额
		resultAddress, amount := model.CalcTradeAmount(walletAddresses, rate, money)
		
		assert.NotEmpty(t, resultAddress.Address)
		actualAmount, err := decimal.NewFromString(amount)
		require.NoError(t, err)
		
		expectedAmount, err := decimal.NewFromString("100.00")
		require.NoError(t, err)
		
		// 应该大于原本的100.00以避免冲突
		assert.True(t, actualAmount.GreaterThan(expectedAmount),
			"金额计算应该避免与已有订单冲突")
		
		t.Logf("冲突避免测试: 原金额=100.00, 新金额=%s", amount)
	})
}

// Test_Simple_ConcurrentOperations 简单并发操作测试
func Test_Simple_ConcurrentOperations(t *testing.T) {
	db := setupSimpleTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	// 创建测试钱包地址
	address := &model.WalletAddress{
		Chain:   "TRON",
		Address: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
	}
	err := db.Create(address).Error
	require.NoError(t, err)

	t.Run("并发订单创建", func(t *testing.T) {
		concurrentCount := 20
		successCount := int64(0)
		errorCount := int64(0)
		var mutex sync.Mutex

		wg := sync.WaitGroup{}
		
		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				// 确保每个订单有唯一的标识
				time.Sleep(time.Duration(index) * time.Microsecond)
				order := createSimpleTestOrder(map[string]interface{}{
					"order_id": fmt.Sprintf("CONCURRENT_%d_%d", index, time.Now().UnixNano()),
					"trade_id": fmt.Sprintf("CTID_%d_%d", index, time.Now().UnixNano()),
				})
				
				if err := db.Create(order).Error; err != nil {
					mutex.Lock()
					errorCount++
					mutex.Unlock()
					t.Logf("并发创建错误 [%d]: %v", index, err)
				} else {
					mutex.Lock()
					successCount++
					mutex.Unlock()
				}
			}(i)
		}
		
		wg.Wait()
		
		t.Logf("并发创建结果: 成功=%d, 失败=%d", successCount, errorCount)
		
		// 大部分应该成功
		expectedMinimum := int64(float64(concurrentCount) * 0.8)
		assert.Greater(t, successCount, expectedMinimum,
			"大部分并发创建应该成功")
	})

	t.Run("并发金额计算", func(t *testing.T) {
		walletAddresses := []model.WalletAddress{*address}
		concurrentCount := 50
		results := make(chan string, concurrentCount)
		
		wg := sync.WaitGroup{}
		
		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				rate := 7.20
				money := 720.00 + float64(index)*0.01 // 每个请求略有不同的金额
				
				_, amount := model.CalcTradeAmount(walletAddresses, rate, money)
				results <- amount
			}(i)
		}
		
		wg.Wait()
		close(results)
		
		// 收集结果
		uniqueAmounts := make(map[string]bool)
		totalResults := 0
		for amount := range results {
			uniqueAmounts[amount] = true
			totalResults++
		}
		
		t.Logf("并发金额计算: 总计算=%d, 唯一金额=%d", totalResults, len(uniqueAmounts))
		
		assert.Equal(t, concurrentCount, totalResults)
		// 由于原子精度递增，大部分金额都应该是唯一的
		expectedUnique := concurrentCount * 80 / 100
		assert.Greater(t, len(uniqueAmounts), expectedUnique,
			"大部分金额应该是唯一的")
	})
}

// Benchmark_Simple_OrderCreation 简单订单创建性能基准
func Benchmark_Simple_OrderCreation(b *testing.B) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	
	// 创建简化的表结构
	db.Exec(`
		CREATE TABLE trade_orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			order_id VARCHAR(255) UNIQUE,
			trade_id VARCHAR(255) UNIQUE,
			trade_hash VARCHAR(64) DEFAULT '',
			usdt_rate VARCHAR(10),
			amount DECIMAL(10,2),
			money DECIMAL(10,2),
			chain VARCHAR(255),
			address VARCHAR(34),
			from_address VARCHAR(34) DEFAULT '',
			status TINYINT(1) DEFAULT 0,
			return_url VARCHAR(255) DEFAULT '',
			notify_url VARCHAR(255) DEFAULT '',
			notify_num INT(11) DEFAULT 0,
			notify_state TINYINT(1) DEFAULT 0,
			expired_at TIMESTAMP,
			created_at TIMESTAMP,
			updated_at TIMESTAMP,
			confirmed_at TIMESTAMP NULL
		)
	`)
	
	model.DB = db
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		order := &model.TradeOrders{
			OrderId:   fmt.Sprintf("BENCH_%d_%d", i, time.Now().UnixNano()),
			TradeId:   fmt.Sprintf("BTID_%d_%d", i, time.Now().UnixNano()),
			UsdtRate:  "7.20",
			Amount:    "100.00",
			Money:     720.00,
			Chain:     "TRON",
			Address:   "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
			Status:    model.OrderStatusWaiting,
			ExpiredAt: time.Now().Add(30 * time.Minute),
		}
		
		if err := db.Create(order).Error; err != nil {
			b.Errorf("订单创建失败: %v", err)
		}
	}
}

// Benchmark_Simple_CalcTradeAmount 简单金额计算性能基准
func Benchmark_Simple_CalcTradeAmount(b *testing.B) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatal(err)
	}
	
	db.AutoMigrate(&model.WalletAddress{}, &model.TradeOrders{})
	model.DB = db
	
	// 创建测试钱包地址
	address := &model.WalletAddress{
		Chain:   "TRON", 
		Address: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
	}
	db.Create(address)
	
	walletAddresses := []model.WalletAddress{*address}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		rate := 7.20
		money := 720.00 + float64(i%100)*0.01
		
		address, amount := model.CalcTradeAmount(walletAddresses, rate, money)
		if address.Address == "" || amount == "" {
			b.Error("金额计算失败")
		}
	}
}

// Test_Simple_PerformanceValidation 简单性能验证测试
func Test_Simple_PerformanceValidation(t *testing.T) {
	db := setupSimpleTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	t.Run("批量订单创建性能", func(t *testing.T) {
		batchSize := 100
		startTime := time.Now()
		
		for i := 0; i < batchSize; i++ {
			order := createSimpleTestOrder(map[string]interface{}{
				"order_id": fmt.Sprintf("BATCH_%d_%d", i, time.Now().UnixNano()),
			})
			
			err := db.Create(order).Error
			require.NoError(t, err)
		}
		
		duration := time.Since(startTime)
		tps := float64(batchSize) / duration.Seconds()
		
		t.Logf("批量创建性能: %d个订单耗时%v, TPS=%.2f", batchSize, duration, tps)
		
		// 基本性能要求：每秒至少处理50个订单
		assert.Greater(t, tps, 50.0, "TPS应该大于50")
	})

	t.Run("查询性能验证", func(t *testing.T) {
		// 先创建一些测试数据
		for i := 0; i < 50; i++ {
			order := createSimpleTestOrder(map[string]interface{}{
				"order_id": fmt.Sprintf("QUERY_TEST_%d", i),
				"status":   i % 3, // 分布不同的状态
			})
			db.Create(order)
		}
		
		// 测试状态查询性能
		startTime := time.Now()
		orders, err := model.GetTradeOrderByStatus(model.OrderStatusWaiting)
		queryDuration := time.Since(startTime)
		
		require.NoError(t, err)
		assert.Greater(t, len(orders), 0, "应该查询到等待支付的订单")
		assert.Less(t, queryDuration, time.Millisecond*100, "查询应该在100ms内完成")
		
		t.Logf("状态查询性能: 查询到%d个订单，耗时%v", len(orders), queryDuration)
	})
}

// Test_Simple_BusinessLogic 简单业务逻辑验证
func Test_Simple_BusinessLogic(t *testing.T) {
	db := setupSimpleTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	t.Run("订单过期处理逻辑", func(t *testing.T) {
		// 创建已过期和未过期的订单
		expiredOrder := createSimpleTestOrder(map[string]interface{}{
			"order_id":   "EXPIRED_TEST",
			"expired_at": time.Now().Add(-1 * time.Hour),
		})
		err := db.Create(expiredOrder).Error
		require.NoError(t, err)
		
		validOrder := createSimpleTestOrder(map[string]interface{}{
			"order_id":   "VALID_TEST", 
			"expired_at": time.Now().Add(1 * time.Hour),
		})
		err = db.Create(validOrder).Error
		require.NoError(t, err)
		
		// 查询过期订单
		var expiredOrders []model.TradeOrders
		err = db.Where("status = ? AND expired_at < ?", 
			model.OrderStatusWaiting, time.Now()).Find(&expiredOrders).Error
		require.NoError(t, err)
		
		assert.Len(t, expiredOrders, 1, "应该只有一个过期订单")
		assert.Equal(t, "EXPIRED_TEST", expiredOrders[0].OrderId)
		
		// 设置为过期状态
		err = expiredOrders[0].OrderSetExpired()
		require.NoError(t, err)
		
		// 验证状态更新
		var updated model.TradeOrders
		err = db.First(&updated, expiredOrders[0].Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusExpired, updated.Status)
		
		t.Logf("过期订单处理成功: %s 状态更新为 %d", updated.OrderId, updated.Status)
	})

	t.Run("状态标签功能", func(t *testing.T) {
		testCases := []struct {
			status       int
			expectedText string
		}{
			{model.OrderStatusWaiting, "🟡 等待支付"},
			{model.OrderStatusSuccess, "🟢 收款成功"},
			{model.OrderStatusExpired, "🔴 交易过期"},
		}
		
		for _, tc := range testCases {
			order := createSimpleTestOrder(map[string]interface{}{
				"status": tc.status,
			})
			
			label := order.GetStatusLabel()
			assert.Equal(t, tc.expectedText, label)
			
			t.Logf("状态标签测试: status=%d -> label='%s'", tc.status, label)
		}
	})
}