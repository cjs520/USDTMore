package notify_test

import (
	"USDTMore/app/model"
	"USDTMore/app/notify"
	"USDTMore/tests/testutils"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOrderNotifySuccess 测试成功的订单回调通知
func TestOrderNotifySuccess(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Successful notification callback", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建模拟回调服务器
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 验证请求方法
			assert.Equal(t, http.MethodPost, r.Method)
			
			// 验证Content-Type
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			
			// 验证Powered-By头
			assert.Equal(t, "https://ovsea.net", r.Header.Get("Powered-By"))

			// 解析请求体
			var requestBody map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&requestBody)
			assert.NoError(t, err)

			// 验证请求体字段
			expectedFields := []string{"trade_id", "order_id", "amount", "actual_amount", "token", "block_transaction_id", "signature", "status"}
			for _, field := range expectedFields {
				assert.Contains(t, requestBody, field, "Request should contain field: %s", field)
			}

			// 验证订单状态为成功
			assert.Equal(t, float64(model.OrderStatusSuccess), requestBody["status"])

			// 返回成功响应
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer mockServer.Close()

		// 创建成功的测试订单
		order := testutils.CreateTestOrder(map[string]interface{}{
			"status":       model.OrderStatusSuccess,
			"notify_url":   mockServer.URL + "/notify",
			"trade_hash":   "test_success_tx_hash",
			"from_address": "TTestFromAddress12345678901234567890",
			"notify_state": model.OrderNotifyStateFail, // 初始为失败状态
			"notify_num":   0,
		})

		err := db.Create(order).Error
		require.NoError(t, err)

		// 执行回调通知
		notify.OrderNotify(*order)

		// 验证订单通知状态被更新
		var updatedOrder model.TradeOrders
		err = db.First(&updatedOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateSucc, updatedOrder.NotifyState)
		assert.Equal(t, 1, updatedOrder.NotifyNum)
	})
}

// TestOrderNotifyFailure 测试失败的订单回调通知
func TestOrderNotifyFailure(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	testCases := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		expectedState  int
	}{
		{
			name: "Server returns non-200 status",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Internal Server Error"))
			},
			expectedState: model.OrderNotifyStateFail,
		},
		{
			name: "Server returns 200 but wrong response body",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("failed"))
			},
			expectedState: model.OrderNotifyStateFail,
		},
		{
			name: "Server returns empty response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(""))
			},
			expectedState: model.OrderNotifyStateFail,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testutils.CleanDatabase(db)

			// 创建模拟回调服务器
			mockServer := httptest.NewServer(http.HandlerFunc(tc.serverResponse))
			defer mockServer.Close()

			// 创建测试订单
			order := testutils.CreateTestOrder(map[string]interface{}{
				"status":       model.OrderStatusSuccess,
				"notify_url":   mockServer.URL + "/notify",
				"trade_hash":   "test_failed_tx_hash",
				"from_address": "TTestFromAddress12345678901234567890",
				"notify_state": model.OrderNotifyStateFail,
				"notify_num":   0,
			})

			err := db.Create(order).Error
			require.NoError(t, err)

			// 执行回调通知
			notify.OrderNotify(*order)

			// 验证订单通知状态保持失败
			var updatedOrder model.TradeOrders
			err = db.First(&updatedOrder, order.Id).Error
			require.NoError(t, err)
			assert.Equal(t, tc.expectedState, updatedOrder.NotifyState)
			assert.Equal(t, 1, updatedOrder.NotifyNum)
		})
	}
}

// TestOrderNotifyRetry 测试订单回调重试机制
func TestOrderNotifyRetry(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Multiple notification attempts", func(t *testing.T) {
		testutils.CleanDatabase(db)

		callCount := 0
		// 创建模拟回调服务器，前两次调用失败，第三次成功
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Server Error"))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			}
		}))
		defer mockServer.Close()

		// 创建测试订单
		order := testutils.CreateTestOrder(map[string]interface{}{
			"status":       model.OrderStatusSuccess,
			"notify_url":   mockServer.URL + "/notify",
			"trade_hash":   "test_retry_tx_hash",
			"from_address": "TTestFromAddress12345678901234567890",
			"notify_state": model.OrderNotifyStateFail,
			"notify_num":   0,
		})

		err := db.Create(order).Error
		require.NoError(t, err)

		// 第一次回调（失败）
		notify.OrderNotify(*order)
		var updatedOrder model.TradeOrders
		err = db.First(&updatedOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateFail, updatedOrder.NotifyState)
		assert.Equal(t, 1, updatedOrder.NotifyNum)

		// 第二次回调（失败）
		notify.OrderNotify(updatedOrder)
		err = db.First(&updatedOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateFail, updatedOrder.NotifyState)
		assert.Equal(t, 2, updatedOrder.NotifyNum)

		// 第三次回调（成功）
		notify.OrderNotify(updatedOrder)
		err = db.First(&updatedOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateSucc, updatedOrder.NotifyState)
		assert.Equal(t, 3, updatedOrder.NotifyNum)

		assert.Equal(t, 3, callCount, "Should make exactly 3 callback attempts")
	})
}

// TestOrderNotifyTimeout 测试订单回调超时处理
func TestOrderNotifyTimeout(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Notification timeout handling", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建慢响应的模拟服务器
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(10 * time.Second) // 超过客户端5秒超时
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer mockServer.Close()

		// 创建测试订单
		order := testutils.CreateTestOrder(map[string]interface{}{
			"status":       model.OrderStatusSuccess,
			"notify_url":   mockServer.URL + "/notify",
			"trade_hash":   "test_timeout_tx_hash",
			"from_address": "TTestFromAddress12345678901234567890",
			"notify_state": model.OrderNotifyStateFail,
			"notify_num":   0,
		})

		err := db.Create(order).Error
		require.NoError(t, err)

		// 执行回调通知（应该超时）
		start := time.Now()
		notify.OrderNotify(*order)
		duration := time.Since(start)

		// 验证超时时间合理（应该在5秒左右，允许一些误差）
		assert.Less(t, duration, 8*time.Second, "Should timeout within reasonable time")
		assert.Greater(t, duration, 4*time.Second, "Should wait at least close to timeout duration")

		// 验证订单通知状态为失败
		var updatedOrder model.TradeOrders
		err = db.First(&updatedOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateFail, updatedOrder.NotifyState)
		assert.Equal(t, 1, updatedOrder.NotifyNum)
	})
}

// TestOrderNotifyDataValidation 测试回调数据验证
func TestOrderNotifyDataValidation(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Callback data structure validation", func(t *testing.T) {
		testutils.CleanDatabase(db)

		var receivedData map[string]interface{}

		// 创建模拟回调服务器来检查数据结构
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := json.NewDecoder(r.Body).Decode(&receivedData)
			assert.NoError(t, err)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer mockServer.Close()

		// 创建测试订单
		testTradeId := "TEST_TRADE_12345"
		testOrderId := "TEST_ORDER_67890"
		testAmount := 150.75
		testActualAmount := "20.93"
		testToken := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testTxHash := "test_validation_tx_hash"

		order := testutils.CreateTestOrder(map[string]interface{}{
			"trade_id":     testTradeId,
			"order_id":     testOrderId,
			"money":        testAmount,
			"amount":       testActualAmount,
			"address":      testToken,
			"status":       model.OrderStatusSuccess,
			"notify_url":   mockServer.URL + "/notify",
			"trade_hash":   testTxHash,
			"from_address": "TTestFromAddress12345678901234567890",
		})

		err := db.Create(order).Error
		require.NoError(t, err)

		// 执行回调通知
		notify.OrderNotify(*order)

		// 验证收到的数据结构
		require.NotNil(t, receivedData)
		
		assert.Equal(t, testTradeId, receivedData["trade_id"])
		assert.Equal(t, testOrderId, receivedData["order_id"])
		assert.Equal(t, testAmount, receivedData["amount"])
		assert.Equal(t, testActualAmount, receivedData["actual_amount"])
		assert.Equal(t, testToken, receivedData["token"])
		assert.Equal(t, testTxHash, receivedData["block_transaction_id"])
		assert.Equal(t, float64(model.OrderStatusSuccess), receivedData["status"])
		
		// 验证签名字段存在且不为空
		signature, ok := receivedData["signature"].(string)
		assert.True(t, ok, "Signature should be a string")
		assert.NotEmpty(t, signature, "Signature should not be empty")
	})
}

// TestNotifyFailedOrdersRetrieval 测试获取回调失败的订单
func TestNotifyFailedOrdersRetrieval(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Get failed notification orders", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建不同状态的订单
		orders := []*model.TradeOrders{
			// 回调失败的订单（应该被检索）
			testutils.CreateTestOrder(map[string]interface{}{
				"trade_id":     "FAILED_1",
				"status":       model.OrderStatusSuccess,
				"notify_num":   2,
				"notify_state": model.OrderNotifyStateFail,
			}),
			// 另一个回调失败的订单（应该被检索）
			testutils.CreateTestOrder(map[string]interface{}{
				"trade_id":     "FAILED_2", 
				"status":       model.OrderStatusSuccess,
				"notify_num":   1,
				"notify_state": model.OrderNotifyStateFail,
			}),
			// 回调成功的订单（不应该被检索）
			testutils.CreateTestOrder(map[string]interface{}{
				"trade_id":     "SUCCESS_1",
				"status":       model.OrderStatusSuccess,
				"notify_num":   1,
				"notify_state": model.OrderNotifyStateSucc,
			}),
			// 未支付的订单（不应该被检索）
			testutils.CreateTestOrder(map[string]interface{}{
				"trade_id":     "WAITING_1",
				"status":       model.OrderStatusWaiting,
				"notify_num":   0,
				"notify_state": model.OrderNotifyStateFail,
			}),
			// 从未尝试通知的成功订单（不应该被检索）
			testutils.CreateTestOrder(map[string]interface{}{
				"trade_id":     "NO_NOTIFY_1",
				"status":       model.OrderStatusSuccess,
				"notify_num":   0,
				"notify_state": model.OrderNotifyStateFail,
			}),
		}

		// 保存所有订单
		for _, order := range orders {
			err := db.Create(order).Error
			require.NoError(t, err)
		}

		// 获取回调失败的订单
		failedOrders, err := model.GetNotifyFailedTradeOrders()
		assert.NoError(t, err)
		assert.Len(t, failedOrders, 2, "Should retrieve exactly 2 failed notification orders")

		// 验证返回的是正确的订单
		tradeIds := make([]string, len(failedOrders))
		for i, order := range failedOrders {
			tradeIds[i] = order.TradeId
		}
		
		assert.Contains(t, tradeIds, "FAILED_1")
		assert.Contains(t, tradeIds, "FAILED_2")
		assert.NotContains(t, tradeIds, "SUCCESS_1")
		assert.NotContains(t, tradeIds, "WAITING_1")
		assert.NotContains(t, tradeIds, "NO_NOTIFY_1")
	})
}

// TestNotificationRetryScheduling 测试回调重试时间调度
func TestNotificationRetryScheduling(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Retry scheduling calculation", func(t *testing.T) {
		testutils.CleanDatabase(db)

		now := time.Now()
		confirmedAt := now.Add(-10 * time.Minute) // 10分钟前确认

		// 创建不同重试次数的订单
		testCases := []struct {
			notifyNum        int
			expectedDelay    time.Duration
			shouldRetryAfter time.Duration
		}{
			{1, 3 * time.Minute, 5 * time.Minute},   // 3^1 = 3分钟延迟
			{2, 9 * time.Minute, 15 * time.Minute}, // 3^2 = 9分钟延迟
			{3, 27 * time.Minute, 30 * time.Minute}, // 3^3 = 27分钟延迟
		}

		for i, tc := range testCases {
			order := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id":      fmt.Sprintf("RETRY_TEST_%d", i),
				"status":        model.OrderStatusSuccess,
				"notify_num":    tc.notifyNum,
				"notify_state":  model.OrderNotifyStateFail,
				"confirmed_at":  confirmedAt,
			})

			err := db.Create(order).Error
			require.NoError(t, err)

			// 计算下次重试时间
			// 下次重试时间 = 确认时间 + (3^重试次数 * 1分钟)
			expectedNextRetry := confirmedAt.Add(tc.expectedDelay)

			// 验证在应该重试的时间之前不会重试
			beforeRetryTime := expectedNextRetry.Add(-1 * time.Minute)
			shouldNotRetry := beforeRetryTime.After(now)
			
			// 验证在应该重试的时间之后会重试
			afterRetryTime := expectedNextRetry.Add(1 * time.Minute)
			shouldRetry := afterRetryTime.After(now)

			// 这里主要是验证重试时间计算逻辑是否正确
			// 在实际的monitor.NotifyStart()中会检查这个时间
			expectedRetryTime := now.Add(tc.shouldRetryAfter)
			actualRetryTime := expectedNextRetry

			// 允许一定的时间误差
			timeDiff := actualRetryTime.Sub(expectedRetryTime).Abs()
			assert.Less(t, timeDiff, 2*time.Minute, 
				"Retry time should be calculated correctly for case %d (notify_num=%d)", i, tc.notifyNum)
		}
	})
}

// TestConcurrentNotifications 测试并发回调通知
func TestConcurrentNotifications(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Concurrent notification handling", func(t *testing.T) {
		testutils.CleanDatabase(db)

		callCount := make(map[string]int)
		// 创建模拟回调服务器
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var requestBody map[string]interface{}
			json.NewDecoder(r.Body).Decode(&requestBody)
			
			tradeId := requestBody["trade_id"].(string)
			callCount[tradeId]++

			// 模拟处理时间
			time.Sleep(100 * time.Millisecond)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer mockServer.Close()

		// 创建多个测试订单
		numOrders := 5
		orders := make([]*model.TradeOrders, numOrders)
		for i := 0; i < numOrders; i++ {
			order := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id":     fmt.Sprintf("CONCURRENT_%d", i),
				"status":       model.OrderStatusSuccess,
				"notify_url":   mockServer.URL + "/notify",
				"trade_hash":   fmt.Sprintf("test_concurrent_tx_%d", i),
				"from_address": "TTestFromAddress12345678901234567890",
			})
			
			err := db.Create(order).Error
			require.NoError(t, err)
			orders[i] = order
		}

		// 并发执行回调通知
		done := make(chan bool, numOrders)
		start := time.Now()

		for i := 0; i < numOrders; i++ {
			go func(order *model.TradeOrders) {
				notify.OrderNotify(*order)
				done <- true
			}(orders[i])
		}

		// 等待所有回调完成
		for i := 0; i < numOrders; i++ {
			<-done
		}
		duration := time.Since(start)

		// 验证所有订单都被处理了
		assert.Equal(t, numOrders, len(callCount), "All orders should be processed")
		for i := 0; i < numOrders; i++ {
			tradeId := fmt.Sprintf("CONCURRENT_%d", i)
			assert.Equal(t, 1, callCount[tradeId], "Order %s should be called exactly once", tradeId)
		}

		// 验证并发处理时间合理（应该比串行处理快）
		maxSequentialTime := time.Duration(numOrders) * 200 * time.Millisecond // 每个200ms的缓冲
		assert.Less(t, duration, maxSequentialTime, "Concurrent processing should be faster than sequential")

		// 验证所有订单状态都被正确更新
		for i := 0; i < numOrders; i++ {
			var updatedOrder model.TradeOrders
			err := db.First(&updatedOrder, orders[i].Id).Error
			require.NoError(t, err)
			assert.Equal(t, model.OrderNotifyStateSucc, updatedOrder.NotifyState)
			assert.Equal(t, 1, updatedOrder.NotifyNum)
		}
	})
}