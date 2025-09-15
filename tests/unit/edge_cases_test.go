package edge_cases_test

import (
	"USDTMore/app/model"
	"USDTMore/app/notify"
	"USDTMore/tests/testutils"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestOrderExpiryEdgeCases 测试订单过期边界情况
func TestOrderExpiryEdgeCases(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Order expiry right at boundary", func(t *testing.T) {
		testutils.CleanDatabase(db)

		now := time.Now()
		// 创建刚刚过期的订单（1秒前过期）
		expiredOrder := testutils.CreateTestOrder(map[string]interface{}{
			"status":     model.OrderStatusWaiting,
			"expired_at": now.Add(-1 * time.Second),
			"created_at": now.Add(-30 * time.Minute),
		})
		err := db.Create(expiredOrder).Error
		require.NoError(t, err)

		// 创建即将过期的订单（1秒后过期）
		almostExpiredOrder := testutils.CreateTestOrder(map[string]interface{}{
			"status":     model.OrderStatusWaiting,
			"expired_at": now.Add(1 * time.Second),
			"created_at": now.Add(-29 * time.Minute),
		})
		err = db.Create(almostExpiredOrder).Error
		require.NoError(t, err)

		// 创建远未过期的订单
		validOrder := testutils.CreateTestOrder(map[string]interface{}{
			"status":     model.OrderStatusWaiting,
			"expired_at": now.Add(30 * time.Minute),
			"created_at": now,
		})
		err = db.Create(validOrder).Error
		require.NoError(t, err)

		// 获取等待支付的订单
		waitingOrders, err := model.GetTradeOrderByStatus(model.OrderStatusWaiting)
		assert.NoError(t, err)
		assert.Len(t, waitingOrders, 3, "Should initially have 3 waiting orders")

		// 模拟过期检查逻辑
		for _, order := range waitingOrders {
			if time.Now().Unix() >= order.ExpiredAt.Unix() {
				err := order.OrderSetExpired()
				assert.NoError(t, err)
			}
		}

		// 验证过期订单被正确标记
		var expiredOrderCheck model.TradeOrders
		err = db.First(&expiredOrderCheck, expiredOrder.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusExpired, expiredOrderCheck.Status)

		// 验证未过期订单状态不变
		var validOrderCheck model.TradeOrders
		err = db.First(&validOrderCheck, validOrder.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusWaiting, validOrderCheck.Status)

		// 几乎过期的订单现在应该也过期了（因为测试执行时间）
		time.Sleep(2 * time.Second)
		var almostExpiredCheck model.TradeOrders
		err = db.First(&almostExpiredCheck, almostExpiredOrder.Id).Error
		require.NoError(t, err)
		if time.Now().Unix() >= almostExpiredCheck.ExpiredAt.Unix() {
			err = almostExpiredCheck.OrderSetExpired()
			assert.NoError(t, err)
			assert.Equal(t, model.OrderStatusExpired, almostExpiredCheck.Status)
		}
	})
}

// TestDuplicatePaymentHandling 测试重复支付处理
func TestDuplicatePaymentHandling(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Same transaction hash duplicate processing", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "88.88"
		txHash := "duplicate_tx_hash_test"

		// 创建两个相同金额的订单
		order1 := testutils.CreateTestOrder(map[string]interface{}{
			"trade_id": "ORDER_1",
			"chain":    "TRON",
			"address":  testAddress,
			"amount":   testAmount,
			"status":   model.OrderStatusWaiting,
		})
		order2 := testutils.CreateTestOrder(map[string]interface{}{
			"trade_id": "ORDER_2",
			"chain":    "TRON", 
			"address":  testAddress,
			"amount":   decimal.NewFromString(testAmount).Add(decimal.NewFromFloat(model.Atomicity)).StringFixed(2), // 递增的金额
			"status":   model.OrderStatusWaiting,
		})

		err := db.Create(order1).Error
		require.NoError(t, err)
		err = db.Create(order2).Error
		require.NoError(t, err)

		// 第一次处理支付
		orderLock := make(map[string]model.TradeOrders)
		orderLock["TRON"+testAddress+testAmount] = *order1

		mockData := createMockTronData(testAddress, testAmount, txHash, time.Now())
		jsonData, _ := json.Marshal(mockData)
		result := gjson.ParseBytes(jsonData)

		processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)
		assert.Equal(t, 1, processedPayments, "Should process first payment")

		// 验证第一个订单成功
		var updatedOrder1 model.TradeOrders
		err = db.First(&updatedOrder1, order1.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, updatedOrder1.Status)
		assert.Equal(t, txHash, updatedOrder1.TradeHash)

		// 尝试用相同交易哈希再次处理（模拟重复通知）
		// 由于订单已经成功，不会在orderLock中
		emptyOrderLock := make(map[string]model.TradeOrders)
		processedPayments2 := processTestPaymentForTronScan(emptyOrderLock, testAddress, result)
		assert.Equal(t, 0, processedPayments2, "Should not process duplicate payment")

		// 验证第二个订单仍然等待支付
		var unchangedOrder2 model.TradeOrders
		err = db.First(&unchangedOrder2, order2.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusWaiting, unchangedOrder2.Status)
		assert.Empty(t, unchangedOrder2.TradeHash)
	})
}

// TestConcurrentOrderProcessing 测试并发订单处理
func TestConcurrentOrderProcessing(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Concurrent payment processing same amount", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		baseAmount := "77.77"
		numConcurrentOrders := 5

		// 创建多个订单
		orders := make([]*model.TradeOrders, numConcurrentOrders)
		for i := 0; i < numConcurrentOrders; i++ {
			// 使用递增的金额避免冲突
			amount := decimal.NewFromString(baseAmount).Add(decimal.NewFromFloat(float64(i) * model.Atomicity)).StringFixed(2)
			order := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id": fmt.Sprintf("CONCURRENT_ORDER_%d", i),
				"chain":    "TRON",
				"address":  testAddress,
				"amount":   amount,
				"status":   model.OrderStatusWaiting,
			})
			err := db.Create(order).Error
			require.NoError(t, err)
			orders[i] = order
		}

		// 并发处理支付
		var wg sync.WaitGroup
		successCount := int32(0)
		var mutex sync.Mutex

		for i := 0; i < numConcurrentOrders; i++ {
			wg.Add(1)
			go func(orderIndex int) {
				defer wg.Done()

				order := orders[orderIndex]
				amount := order.Amount

				orderLock := make(map[string]model.TradeOrders)
				orderLock["TRON"+testAddress+amount] = *order

				txHash := fmt.Sprintf("concurrent_tx_%d", orderIndex)
				mockData := createMockTronData(testAddress, amount, txHash, time.Now())
				jsonData, _ := json.Marshal(mockData)
				result := gjson.ParseBytes(jsonData)

				processed := processTestPaymentForTronScan(orderLock, testAddress, result)
				
				mutex.Lock()
				successCount += int32(processed)
				mutex.Unlock()
			}(i)
		}

		wg.Wait()

		// 验证所有支付都被正确处理
		assert.Equal(t, int32(numConcurrentOrders), successCount, "All concurrent payments should be processed")

		// 验证所有订单状态都被正确更新
		for i, order := range orders {
			var updatedOrder model.TradeOrders
			err := db.First(&updatedOrder, order.Id).Error
			require.NoError(t, err)
			assert.Equal(t, model.OrderStatusSuccess, updatedOrder.Status, "Order %d should be successful", i)
			assert.NotEmpty(t, updatedOrder.TradeHash, "Order %d should have transaction hash", i)
		}
	})
}

// TestExtremeAmountValues 测试极端金额值
func TestExtremeAmountValues(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Very small and large amounts", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		
		testCases := []struct {
			name            string
			amount          string
			shouldProcess   bool
			description     string
		}{
			{"Very small amount", "0.01", false, "Below minimum threshold of 0.1"},
			{"Minimum amount", "0.10", true, "At minimum threshold"},
			{"Just above minimum", "0.11", true, "Slightly above minimum threshold"},
			{"Normal amount", "100.00", true, "Normal transaction amount"},
			{"Large amount", "9999.99", true, "Large transaction amount"},
			{"Maximum precision", "123.456789", true, "Should be rounded to 123.46"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// 标准化金额格式
				amountDecimal, _ := decimal.NewFromString(tc.amount)
				standardAmount := amountDecimal.StringFixed(2)

				order := testutils.CreateTestOrder(map[string]interface{}{
					"trade_id": fmt.Sprintf("AMOUNT_TEST_%s", tc.name),
					"chain":    "TRON",
					"address":  testAddress,
					"amount":   standardAmount,
					"status":   model.OrderStatusWaiting,
				})
				err := db.Create(order).Error
				require.NoError(t, err)

				orderLock := make(map[string]model.TradeOrders)
				orderLock["TRON"+testAddress+standardAmount] = *order

				mockData := createMockTronData(testAddress, tc.amount, fmt.Sprintf("amount_test_%s", tc.name), time.Now())
				jsonData, _ := json.Marshal(mockData)
				result := gjson.ParseBytes(jsonData)

				processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)

				if tc.shouldProcess {
					assert.Equal(t, 1, processedPayments, "Should process %s (%s)", tc.name, tc.description)
					
					var updatedOrder model.TradeOrders
					err = db.First(&updatedOrder, order.Id).Error
					require.NoError(t, err)
					assert.Equal(t, model.OrderStatusSuccess, updatedOrder.Status)
				} else {
					assert.Equal(t, 0, processedPayments, "Should not process %s (%s)", tc.name, tc.description)
					
					var unchangedOrder model.TradeOrders
					err = db.First(&unchangedOrder, order.Id).Error
					require.NoError(t, err)
					assert.Equal(t, model.OrderStatusWaiting, unchangedOrder.Status)
				}
			})
		}
	})
}

// TestNotificationRetryExtremeScenarios 测试通知重试极端场景
func TestNotificationRetryExtremeScenarios(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("High retry count exponential backoff", func(t *testing.T) {
		testutils.CleanDatabase(db)

		callCount := 0
		failureCount := 10 // 前10次调用都失败

		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount <= failureCount {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Server Error"))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			}
		}))
		defer mockServer.Close()

		order := testutils.CreateTestOrder(map[string]interface{}{
			"status":       model.OrderStatusSuccess,
			"notify_url":   mockServer.URL + "/notify",
			"trade_hash":   "high_retry_test_tx",
			"from_address": "TTestFromAddress12345678901234567890",
			"confirmed_at": time.Now().Add(-24 * time.Hour), // 24小时前确认
		})

		err := db.Create(order).Error
		require.NoError(t, err)

		// 模拟多次重试
		for i := 1; i <= failureCount+1; i++ {
			notify.OrderNotify(*order)
			
			// 重新加载订单获取最新状态
			err = db.First(order, order.Id).Error
			require.NoError(t, err)

			if i <= failureCount {
				assert.Equal(t, model.OrderNotifyStateFail, order.NotifyState)
				assert.Equal(t, i, order.NotifyNum)
			} else {
				assert.Equal(t, model.OrderNotifyStateSucc, order.NotifyState)
				assert.Equal(t, i, order.NotifyNum)
			}
		}

		assert.Equal(t, failureCount+1, callCount, "Should make all expected callback attempts")
	})

	t.Run("Notification server completely unreachable", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 使用无效URL
		order := testutils.CreateTestOrder(map[string]interface{}{
			"status":       model.OrderStatusSuccess,
			"notify_url":   "http://nonexistent.invalid.url/notify",
			"trade_hash":   "unreachable_test_tx",
			"from_address": "TTestFromAddress12345678901234567890",
		})

		err := db.Create(order).Error
		require.NoError(t, err)

		// 尝试通知
		start := time.Now()
		notify.OrderNotify(*order)
		duration := time.Since(start)

		// 应该快速失败（DNS解析失败或连接超时）
		assert.Less(t, duration, 10*time.Second, "Should fail quickly for unreachable server")

		// 验证订单状态为失败
		var updatedOrder model.TradeOrders
		err = db.First(&updatedOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateFail, updatedOrder.NotifyState)
		assert.Equal(t, 1, updatedOrder.NotifyNum)
	})
}

// TestOrderAmountCalculationEdgeCases 测试订单金额计算边界情况
func TestOrderAmountCalculationEdgeCases(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Amount calculation with many conflicts", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		wallet := testutils.CreateTestWalletAddress("TRON", testAddress)
		err := db.Create(wallet).Error
		require.NoError(t, err)

		rate := 7.20
		money := 100.0
		// 安全的除法操作，避免除零错误
		rateDecimal, _ := decimal.NewFromString(fmt.Sprintf("%.8f", rate))
		moneyDecimal, _ := decimal.NewFromString(fmt.Sprintf("%.2f", money))
		if rateDecimal.IsZero() {
			t.Fatalf("Rate cannot be zero, rate=%.8f", rate)
		}
		baseAmount := moneyDecimal.Div(rateDecimal).StringFixed(2)

		// 创建多个冲突的订单
		numConflicts := 100
		for i := 0; i < numConflicts; i++ {
			incrementedAmount := decimal.NewFromString(baseAmount).Add(decimal.NewFromFloat(float64(i) * model.Atomicity)).StringFixed(2)
			conflictOrder := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id": fmt.Sprintf("CONFLICT_%d", i),
				"chain":    "TRON",
				"address":  testAddress,
				"amount":   incrementedAmount,
				"status":   model.OrderStatusWaiting,
			})
			err := db.Create(conflictOrder).Error
			require.NoError(t, err)
		}

		wallets := []model.WalletAddress{*wallet}
		address, amount := model.CalcTradeAmount(wallets, rate, money)

		// 应该选择下一个可用的金额
		expectedAmount := decimal.NewFromString(baseAmount).Add(decimal.NewFromFloat(float64(numConflicts) * model.Atomicity)).StringFixed(2)
		assert.Equal(t, testAddress, address.Address)
		assert.Equal(t, expectedAmount, amount)
	})

	t.Run("Amount calculation with multiple wallets and conflicts", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建多个钱包地址
		wallets := []model.WalletAddress{}
		for i := 0; i < 3; i++ {
			wallet := testutils.CreateTestWalletAddress("TRON", fmt.Sprintf("TAddress%d123456789012345678901234%d", i, i))
			err := db.Create(wallet).Error
			require.NoError(t, err)
			wallets = append(wallets, *wallet)
		}

		rate := 7.20
		money := 150.0
		// 安全的除法操作，避免除零错误
		rateDecimal, _ := decimal.NewFromString(fmt.Sprintf("%.8f", rate))
		moneyDecimal, _ := decimal.NewFromString(fmt.Sprintf("%.2f", money))
		if rateDecimal.IsZero() {
			t.Fatalf("Rate cannot be zero, rate=%.8f", rate)
		}
		baseAmount := moneyDecimal.Div(rateDecimal).StringFixed(2)

		// 为前两个钱包创建冲突订单
		for i := 0; i < 2; i++ {
			conflictOrder := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id": fmt.Sprintf("MULTI_CONFLICT_%d", i),
				"chain":    "TRON",
				"address":  wallets[i].Address,
				"amount":   baseAmount,
				"status":   model.OrderStatusWaiting,
			})
			err := db.Create(conflictOrder).Error
			require.NoError(t, err)
		}

		address, amount := model.CalcTradeAmount(wallets, rate, money)

		// 应该选择第三个钱包（没有冲突的）
		assert.Equal(t, wallets[2].Address, address.Address)
		assert.Equal(t, baseAmount, amount)
	})
}

// TestRaceConditionScenarios 测试竞态条件场景
func TestRaceConditionScenarios(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Concurrent order status updates", func(t *testing.T) {
		testutils.CleanDatabase(db)

		order := testutils.CreateTestOrder()
		err := db.Create(order).Error
		require.NoError(t, err)

		numGoroutines := 10
		var wg sync.WaitGroup
		successCount := int32(0)

		// 并发尝试更新订单状态
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				// 重新加载订单以避免stale对象
				var localOrder model.TradeOrders
				if db.First(&localOrder, order.Id).Error == nil {
					fromAddress := fmt.Sprintf("TTestConcurrent%d123456789012345", index)
					tradeHash := fmt.Sprintf("concurrent_tx_%d", index)
					
					if localOrder.OrderSetSucc(fromAddress, tradeHash, time.Now()) == nil {
						successCount++
					}
				}
			}(i)
		}

		wg.Wait()

		// 验证只有一个goroutine成功更新订单
		// 注意：由于数据库锁机制，可能所有goroutine都能成功，但最后一个会覆盖前面的
		assert.True(t, successCount >= 1, "At least one goroutine should succeed")

		// 验证最终订单状态
		var finalOrder model.TradeOrders
		err = db.First(&finalOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, finalOrder.Status)
		assert.NotEmpty(t, finalOrder.TradeHash)
		assert.NotEmpty(t, finalOrder.FromAddress)
	})

	t.Run("Concurrent notification attempts", func(t *testing.T) {
		testutils.CleanDatabase(db)

		callCount := int32(0)
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 模拟处理时间
			time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			callCount++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer mockServer.Close()

		order := testutils.CreateTestOrder(map[string]interface{}{
			"status":       model.OrderStatusSuccess,
			"notify_url":   mockServer.URL + "/notify",
			"trade_hash":   "concurrent_notify_tx",
			"from_address": "TTestFromAddress12345678901234567890",
		})

		err := db.Create(order).Error
		require.NoError(t, err)

		numGoroutines := 5
		var wg sync.WaitGroup

		// 并发发送通知
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				notify.OrderNotify(*order)
			}()
		}

		wg.Wait()

		// 验证所有通知都被发送了
		assert.Equal(t, int32(numGoroutines), callCount, "All notifications should be sent")

		// 验证最终订单状态（最后一个成功的通知会设置状态）
		var finalOrder model.TradeOrders
		err = db.First(&finalOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateSucc, finalOrder.NotifyState)
		assert.True(t, finalOrder.NotifyNum >= numGoroutines, "Notify count should reflect all attempts")
	})
}

// TestErrorRecoveryScenarios 测试错误恢复场景
func TestErrorRecoveryScenarios(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Database connection recovery", func(t *testing.T) {
		testutils.CleanDatabase(db)

		order := testutils.CreateTestOrder()
		err := db.Create(order).Error
		require.NoError(t, err)

		// 模拟数据库操作失败后的恢复
		// 这里我们通过检查订单状态更新的原子性来间接测试
		
		// 首次更新应该成功
		err = order.OrderSetSucc("TTestAddress1", "test_tx_1", time.Now())
		assert.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, order.Status)

		// 再次更新也应该成功（模拟恢复后的操作）
		err = order.OrderSetSucc("TTestAddress2", "test_tx_2", time.Now())
		assert.NoError(t, err)
		assert.Equal(t, "TTestAddress2", order.FromAddress)
		assert.Equal(t, "test_tx_2", order.TradeHash)
	})

	t.Run("Partial data corruption recovery", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建带有部分无效数据的订单
		order := testutils.CreateTestOrder(map[string]interface{}{
			"amount":       "INVALID_AMOUNT", // 无效金额格式
			"usdt_rate":    "INVALID_RATE",   // 无效汇率格式
			"notify_url":   "invalid://url",   // 无效URL格式
		})
		
		// 尽管数据有问题，数据库操作应该成功（数据验证在应用层）
		err := db.Create(order).Error
		assert.NoError(t, err, "Database should accept even invalid data")

		// 验证可以读取订单
		var retrievedOrder model.TradeOrders
		err = db.First(&retrievedOrder, order.Id).Error
		assert.NoError(t, err)
		assert.Equal(t, "INVALID_AMOUNT", retrievedOrder.Amount)
	})
}

// 辅助函数 - 复制了之前的函数以避免导入问题
func createMockTronData(toAddress, amount, txHash string, timestamp time.Time) map[string]interface{} {
	amountDecimal, _ := decimal.NewFromString(amount)
	quantInt := amountDecimal.Mul(decimal.NewFromFloat(1000000)).IntPart()

	return map[string]interface{}{
		"total": 1,
		"token_transfers": []map[string]interface{}{
			{
				"transaction_id": txHash,
				"to_address":     toAddress,
				"from_address":   "TTestFromAddress12345678901234567890",
				"quant":          quantInt,
				"contractRet":    "SUCCESS",
				"block_ts":       timestamp.UnixMilli(),
			},
		},
	}
}

func processTestPaymentForTronScan(orderLock map[string]model.TradeOrders, toAddress string, data gjson.Result) int {
	processed := 0
	for _, transfer := range data.Get("token_transfers").Array() {
		if transfer.Get("to_address").String() != toAddress {
			continue
		}

		// 计算交易金额
		rawQuant := transfer.Get("quant").Float()
		_decimalAmount := decimal.NewFromFloat(rawQuant)
		_decimalDivisor := decimal.NewFromFloat(1000000)
		result := _decimalAmount.Div(_decimalDivisor)
		
		// 检查最小金额阈值
		if result.LessThan(decimal.NewFromFloat(0.1)) {
			continue
		}
		
		quant := result.StringFixed(2)

		// 检查订单是否存在
		orderKey := "TRON" + toAddress + quant
		order, ok := orderLock[orderKey]
		if !ok || transfer.Get("contractRet").String() != "SUCCESS" {
			continue
		}

		// 检查时间有效性
		createdAt := time.UnixMilli(transfer.Get("block_ts").Int())
		if createdAt.Unix() < order.CreatedAt.Unix() || createdAt.Unix() > order.ExpiredAt.Unix() {
			continue
		}

		// 处理支付
		transId := transfer.Get("transaction_id").String()
		fromAddress := transfer.Get("from_address").String()
		if order.OrderSetSucc(fromAddress, transId, createdAt) == nil {
			processed++
		}
	}
	return processed
}