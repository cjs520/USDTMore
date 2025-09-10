package performance_test

import (
	"USDTMore/app/model"
	"USDTMore/app/notify"
	"USDTMore/tests/testutils"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BenchmarkOrderCreation 基准测试：订单创建性能
func BenchmarkOrderCreation(b *testing.B) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, b)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建钱包地址
	wallet := testutils.CreateTestWalletAddress("TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
	err := db.Create(wallet).Error
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			order := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id": fmt.Sprintf("BENCH_ORDER_%d_%d", b.N, i),
				"order_id": fmt.Sprintf("BENCH_%d_%d", b.N, i),
			})
			
			err := db.Create(order).Error
			if err != nil {
				b.Fatalf("Failed to create order: %v", err)
			}
			i++
		}
	})
}

// BenchmarkOrderQuery 基准测试：订单查询性能
func BenchmarkOrderQuery(b *testing.B) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, b)
	defer testutils.TeardownTestDB(ctx, container)

	// 预创建大量订单
	numOrders := 10000
	tradeIds := make([]string, numOrders)
	
	for i := 0; i < numOrders; i++ {
		order := testutils.CreateTestOrder(map[string]interface{}{
			"trade_id": fmt.Sprintf("QUERY_TEST_%d", i),
			"status":   model.OrderStatusWaiting,
		})
		err := db.Create(order).Error
		require.NoError(b, err)
		tradeIds[i] = order.TradeId
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			randomId := tradeIds[rand.Intn(len(tradeIds))]
			_, exists := model.GetTradeOrder(randomId)
			if !exists {
				b.Fatalf("Order should exist: %s", randomId)
			}
		}
	})
}

// BenchmarkAmountCalculation 基准测试：金额计算性能
func BenchmarkAmountCalculation(b *testing.B) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, b)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建多个钱包地址
	wallets := make([]model.WalletAddress, 10)
	for i := 0; i < 10; i++ {
		wallet := testutils.CreateTestWalletAddress("TRON", fmt.Sprintf("TAddress%d123456789012345678901234%d", i, i))
		err := db.Create(wallet).Error
		require.NoError(b, err)
		wallets[i] = *wallet
	}

	// 创建一些冲突订单
	for i := 0; i < 100; i++ {
		amount := decimal.NewFromFloat(100.0 + float64(i)*0.01).StringFixed(2)
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":   "TRON",
			"address": wallets[i%len(wallets)].Address,
			"amount":  amount,
			"status":  model.OrderStatusWaiting,
		})
		err := db.Create(order).Error
		require.NoError(b, err)
	}

	rate := 7.20
	money := 100.0

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = model.CalcTradeAmount(wallets, rate, money)
	}
}

// BenchmarkNotification 基准测试：通知发送性能
func BenchmarkNotification(b *testing.B) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, b)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建模拟回调服务器
	requestCount := int64(0)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer mockServer.Close()

	// 预创建订单
	orders := make([]*model.TradeOrders, b.N)
	for i := 0; i < b.N; i++ {
		order := testutils.CreateTestOrder(map[string]interface{}{
			"trade_id":     fmt.Sprintf("NOTIFY_BENCH_%d", i),
			"status":       model.OrderStatusSuccess,
			"notify_url":   mockServer.URL + "/notify",
			"trade_hash":   fmt.Sprintf("bench_tx_%d", i),
			"from_address": "TTestFromAddress12345678901234567890",
		})
		err := db.Create(order).Error
		require.NoError(b, err)
		orders[i] = order
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i < len(orders) {
				notify.OrderNotify(*orders[i])
				i++
			}
		}
	})

	b.StopTimer()
	
	// 等待所有请求完成
	time.Sleep(1 * time.Second)
	finalCount := atomic.LoadInt64(&requestCount)
	b.Logf("Total requests sent: %d", finalCount)
}

// TestHighConcurrencyOrderProcessing 测试高并发订单处理
func TestHighConcurrencyOrderProcessing(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("High concurrency order creation", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建多个钱包地址以支持并发
		numWallets := 5
		wallets := make([]*model.WalletAddress, numWallets)
		for i := 0; i < numWallets; i++ {
			wallet := testutils.CreateTestWalletAddress("TRON", fmt.Sprintf("TAddress%d123456789012345678901234%d", i, i))
			err := db.Create(wallet).Error
			require.NoError(t, err)
			wallets[i] = wallet
		}

		numGoroutines := 50
		ordersPerGoroutine := 20
		totalOrders := numGoroutines * ordersPerGoroutine

		var wg sync.WaitGroup
		createdOrders := make(chan string, totalOrders)
		errors := make(chan error, totalOrders)

		start := time.Now()

		// 并发创建订单
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineId int) {
				defer wg.Done()

				for j := 0; j < ordersPerGoroutine; j++ {
					order := testutils.CreateTestOrder(map[string]interface{}{
						"trade_id": fmt.Sprintf("CONCURRENT_%d_%d", goroutineId, j),
						"order_id": fmt.Sprintf("ORDER_%d_%d", goroutineId, j),
						"chain":    "TRON",
						"address":  wallets[goroutineId%numWallets].Address,
						"status":   model.OrderStatusWaiting,
					})

					if err := db.Create(order).Error; err != nil {
						errors <- err
					} else {
						createdOrders <- order.TradeId
					}
				}
			}(i)
		}

		wg.Wait()
		close(createdOrders)
		close(errors)
		
		duration := time.Since(start)

		// 检查错误
		errorCount := len(errors)
		if errorCount > 0 {
			t.Logf("Encountered %d errors during concurrent creation", errorCount)
		}

		// 检查成功创建的订单数量
		successCount := len(createdOrders)
		assert.Greater(t, successCount, totalOrders*8/10, "At least 80%% of orders should be created successfully")

		t.Logf("Created %d orders in %v (%.2f orders/sec)", successCount, duration, float64(successCount)/duration.Seconds())

		// 验证数据库中的订单数量
		var dbOrderCount int64
		err := db.Model(&model.TradeOrders{}).Count(&dbOrderCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(successCount), dbOrderCount, "Database should contain all successfully created orders")
	})
}

// TestHighThroughputPaymentProcessing 测试高吞吐量支付处理
func TestHighThroughputPaymentProcessing(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("High throughput payment processing", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		numPayments := 1000

		// 创建大量等待支付的订单
		orders := make([]*model.TradeOrders, numPayments)
		for i := 0; i < numPayments; i++ {
			amount := decimal.NewFromFloat(100.0 + float64(i)*0.01).StringFixed(2)
			order := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id": fmt.Sprintf("THROUGHPUT_TEST_%d", i),
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
		numWorkers := 10
		paymentChannel := make(chan *model.TradeOrders, numPayments)
		var processedCount int64
		var wg sync.WaitGroup

		start := time.Now()

		// 启动工作协程
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func(workerId int) {
				defer wg.Done()

				for order := range paymentChannel {
					fromAddress := fmt.Sprintf("TTestPayer%d12345678901234567890123", workerId)
					txHash := fmt.Sprintf("throughput_tx_%s_%d", order.TradeId, workerId)
					
					if err := order.OrderSetSucc(fromAddress, txHash, time.Now()); err == nil {
						atomic.AddInt64(&processedCount, 1)
					}
					
					// 模拟处理时间
					time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)
				}
			}(i)
		}

		// 发送支付到处理队列
		for _, order := range orders {
			paymentChannel <- order
		}
		close(paymentChannel)

		wg.Wait()
		duration := time.Since(start)

		successCount := atomic.LoadInt64(&processedCount)
		throughput := float64(successCount) / duration.Seconds()

		t.Logf("Processed %d payments in %v (%.2f payments/sec)", successCount, duration, throughput)
		assert.Equal(t, int64(numPayments), successCount, "All payments should be processed")
		assert.Greater(t, throughput, 100.0, "Throughput should be > 100 payments/sec")

		// 验证所有订单状态都已更新
		var successOrderCount int64
		err := db.Model(&model.TradeOrders{}).Where("status = ?", model.OrderStatusSuccess).Count(&successOrderCount).Error
		require.NoError(t, err)
		assert.Equal(t, successCount, successOrderCount, "All processed orders should be marked as successful")
	})
}

// TestNotificationPerformanceUnderLoad 测试负载下的通知性能
func TestNotificationPerformanceUnderLoad(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Notification performance under load", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建模拟回调服务器（带延迟）
		var requestCount int64
		var totalResponseTime int64
		
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			
			// 模拟不同的响应时间
			delay := time.Duration(rand.Intn(100)) * time.Millisecond
			time.Sleep(delay)
			
			atomic.AddInt64(&requestCount, 1)
			atomic.AddInt64(&totalResponseTime, delay.Nanoseconds())
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		}))
		defer mockServer.Close()

		numNotifications := 500
		orders := make([]*model.TradeOrders, numNotifications)

		// 创建需要通知的订单
		for i := 0; i < numNotifications; i++ {
			order := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id":     fmt.Sprintf("LOAD_TEST_%d", i),
				"status":       model.OrderStatusSuccess,
				"notify_url":   mockServer.URL + "/notify",
				"trade_hash":   fmt.Sprintf("load_tx_%d", i),
				"from_address": "TTestFromAddress12345678901234567890",
			})
			err := db.Create(order).Error
			require.NoError(t, err)
			orders[i] = order
		}

		// 并发发送通知
		numWorkers := 20
		notificationChannel := make(chan *model.TradeOrders, numNotifications)
		var wg sync.WaitGroup

		start := time.Now()

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for order := range notificationChannel {
					notify.OrderNotify(*order)
				}
			}()
		}

		for _, order := range orders {
			notificationChannel <- order
		}
		close(notificationChannel)

		wg.Wait()
		duration := time.Since(start)

		// 等待所有请求完成
		time.Sleep(2 * time.Second)

		finalCount := atomic.LoadInt64(&requestCount)
		avgResponseTime := time.Duration(atomic.LoadInt64(&totalResponseTime) / finalCount)
		notificationThroughput := float64(finalCount) / duration.Seconds()

		t.Logf("Sent %d notifications in %v (%.2f notifications/sec)", finalCount, duration, notificationThroughput)
		t.Logf("Average response time: %v", avgResponseTime)

		assert.Equal(t, int64(numNotifications), finalCount, "All notifications should be sent")
		assert.Greater(t, notificationThroughput, 50.0, "Notification throughput should be > 50/sec")
		assert.Less(t, avgResponseTime, 200*time.Millisecond, "Average response time should be reasonable")

		// 验证数据库中的通知状态
		var successNotifyCount int64
		err := db.Model(&model.TradeOrders{}).Where("notify_state = ?", model.OrderNotifyStateSucc).Count(&successNotifyCount).Error
		require.NoError(t, err)
		assert.Equal(t, finalCount, successNotifyCount, "All successful notifications should be recorded")
	})
}

// TestMemoryUsageUnderLoad 测试负载下的内存使用
func TestMemoryUsageUnderLoad(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Memory usage under load", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 记录初始内存使用情况
		var m1 runtime.MemStats
		runtime.ReadMemStats(&m1)
		initialAlloc := m1.Alloc

		// 创建大量订单进行处理
		numOrders := 10000
		batchSize := 100

		for batch := 0; batch < numOrders/batchSize; batch++ {
			orders := make([]*model.TradeOrders, batchSize)
			
			// 批量创建订单
			for i := 0; i < batchSize; i++ {
				order := testutils.CreateTestOrder(map[string]interface{}{
					"trade_id": fmt.Sprintf("MEMORY_TEST_%d_%d", batch, i),
					"order_id": fmt.Sprintf("MEM_ORDER_%d_%d", batch, i),
				})
				orders[i] = order
			}

			// 批量插入数据库
			err := db.CreateInBatches(orders, batchSize).Error
			require.NoError(t, err)

			// 批量查询和更新
			for _, order := range orders {
				_, exists := model.GetTradeOrder(order.TradeId)
				assert.True(t, exists)

				err := order.OrderSetSucc("TTestFromAddress", fmt.Sprintf("tx_%s", order.TradeId), time.Now())
				assert.NoError(t, err)
			}

			// 定期触发垃圾回收
			if batch%10 == 0 {
				runtime.GC()
			}
		}

		// 最终垃圾回收
		runtime.GC()
		runtime.GC() // 调用两次确保清理完成

		// 检查最终内存使用情况
		var m2 runtime.MemStats
		runtime.ReadMemStats(&m2)
		finalAlloc := m2.Alloc

		memoryIncrease := finalAlloc - initialAlloc
		memoryIncreasePerOrder := float64(memoryIncrease) / float64(numOrders)

		t.Logf("Initial memory: %d KB", initialAlloc/1024)
		t.Logf("Final memory: %d KB", finalAlloc/1024)
		t.Logf("Memory increase: %d KB", memoryIncrease/1024)
		t.Logf("Memory per order: %.2f bytes", memoryIncreasePerOrder)

		// 验证内存使用不会无限增长
		maxAllowedIncrease := uint64(100 * 1024 * 1024) // 100MB
		assert.Less(t, memoryIncrease, maxAllowedIncrease, "Memory increase should be bounded")

		// 验证所有订单都被正确处理
		var totalCount int64
		err := db.Model(&model.TradeOrders{}).Count(&totalCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(numOrders), totalCount, "All orders should be in database")

		var successCount int64
		err = db.Model(&model.TradeOrders{}).Where("status = ?", model.OrderStatusSuccess).Count(&successCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(numOrders), successCount, "All orders should be successful")
	})
}

// TestDatabaseConnectionPoolPerformance 测试数据库连接池性能
func TestDatabaseConnectionPoolPerformance(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Database connection pool performance", func(t *testing.T) {
		testutils.CleanDatabase(db)

		numConcurrentQueries := 100
		queriesPerConnection := 50
		totalQueries := numConcurrentQueries * queriesPerConnection

		// 预创建一些数据
		for i := 0; i < 1000; i++ {
			order := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id": fmt.Sprintf("POOL_TEST_%d", i),
			})
			err := db.Create(order).Error
			require.NoError(t, err)
		}

		var wg sync.WaitGroup
		var queryCount int64
		var errorCount int64

		start := time.Now()

		// 并发执行查询
		for i := 0; i < numConcurrentQueries; i++ {
			wg.Add(1)
			go func(goroutineId int) {
				defer wg.Done()

				for j := 0; j < queriesPerConnection; j++ {
					// 执行不同类型的查询
					switch j % 4 {
					case 0:
						// 单个订单查询
						tradeId := fmt.Sprintf("POOL_TEST_%d", rand.Intn(1000))
						_, _ = model.GetTradeOrder(tradeId)
					case 1:
						// 状态查询
						_, err := model.GetTradeOrderByStatus(model.OrderStatusWaiting)
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
						}
					case 2:
						// 计数查询
						var count int64
						err := db.Model(&model.TradeOrders{}).Count(&count).Error
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
						}
					case 3:
						// 更新查询
						err := db.Model(&model.TradeOrders{}).Where("trade_id = ?", fmt.Sprintf("POOL_TEST_%d", rand.Intn(1000))).Update("updated_at", time.Now()).Error
						if err != nil {
							atomic.AddInt64(&errorCount, 1)
						}
					}
					
					atomic.AddInt64(&queryCount, 1)
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(start)

		finalQueryCount := atomic.LoadInt64(&queryCount)
		finalErrorCount := atomic.LoadInt64(&errorCount)
		queryThroughput := float64(finalQueryCount) / duration.Seconds()

		t.Logf("Executed %d queries in %v (%.2f queries/sec)", finalQueryCount, duration, queryThroughput)
		t.Logf("Errors: %d (%.2f%%)", finalErrorCount, float64(finalErrorCount)/float64(finalQueryCount)*100)

		assert.Equal(t, int64(totalQueries), finalQueryCount, "All queries should be executed")
		assert.Less(t, finalErrorCount, finalQueryCount/100, "Error rate should be < 1%")
		assert.Greater(t, queryThroughput, 1000.0, "Query throughput should be > 1000 queries/sec")
	})
}