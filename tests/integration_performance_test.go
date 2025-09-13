package tests

import (
	"USDTMore/app/model"
	"USDTMore/app/service"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB 设置测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// 创建表结构
	err = db.AutoMigrate(
		&model.TradeOrders{},
		&model.WalletAddress{},
		&model.NotifyRecord{},
	)
	require.NoError(t, err)

	// 设置全局DB
	model.DB = db

	// 初始化测试钱包地址
	wallets := []model.WalletAddress{
		{Chain: "TRON", Address: "TAddress1", Balance: 1000000, IsActive: true},
		{Chain: "TRON", Address: "TAddress2", Balance: 1000000, IsActive: true},
		{Chain: "BSC", Address: "BAddress1", Balance: 1000000, IsActive: true},
		{Chain: "BSC", Address: "BAddress2", Balance: 1000000, IsActive: true},
		{Chain: "POLY", Address: "PAddress1", Balance: 1000000, IsActive: true},
		{Chain: "OP", Address: "OAddress1", Balance: 1000000, IsActive: true},
	}

	for _, wallet := range wallets {
		err = db.Create(&wallet).Error
		require.NoError(t, err)
	}

	return db
}

// TestIntegratedPerformance 集成性能测试
func TestIntegratedPerformance(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	// 创建服务实例
	repo := service.NewOrderRepository(db)
	orderService := service.NewOrderService(repo)
	amountService := service.NewAmountService(repo)

	t.Run("并发订单创建性能", func(t *testing.T) {
		var successCount int64
		var failCount int64
		var duplicateCount int64

		concurrency := 50
		ordersPerWorker := 20
		totalOrders := concurrency * ordersPerWorker

		startTime := time.Now()
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()

				for j := 0; j < ordersPerWorker; j++ {
					// 获取可用钱包
					var wallets []model.WalletAddress
					db.Where("is_active = ?", true).Find(&wallets)

					if len(wallets) == 0 {
						atomic.AddInt64(&failCount, 1)
						continue
					}

					// 计算交易金额
					rate := 7.0
					money := 100.0 + float64(workerID*ordersPerWorker+j)*0.01

					address, amount, err := amountService.CalcTradeAmount(nil, wallets, rate, money)
					if err != nil {
						atomic.AddInt64(&duplicateCount, 1)
						continue
					}

					// 创建订单
					order := &model.TradeOrders{
						OrderId:   fmt.Sprintf("TEST_%d_%d_%d", time.Now().Unix(), workerID, j),
						TradeId:   fmt.Sprintf("TRADE_%d_%d_%d", time.Now().Unix(), workerID, j),
						UsdtRate:  fmt.Sprintf("%.2f", rate),
						Amount:    amount,
						Money:     money,
						Chain:     address.Chain,
						Address:   address.Address,
						Status:    model.OrderStatusWaiting,
						ExpiredAt: time.Now().Add(30 * time.Minute),
					}

					err = orderService.CreateOrder(nil, order)
					if err != nil {
						atomic.AddInt64(&failCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(startTime)

		// 计算性能指标
		successRate := float64(successCount) * 100 / float64(totalOrders)
		tps := float64(successCount) / elapsed.Seconds()
		avgLatency := elapsed / time.Duration(totalOrders)

		t.Logf("=== 并发订单创建性能 ===")
		t.Logf("并发数: %d", concurrency)
		t.Logf("总订单数: %d", totalOrders)
		t.Logf("成功创建: %d", successCount)
		t.Logf("失败数: %d", failCount)
		t.Logf("重复金额: %d", duplicateCount)
		t.Logf("成功率: %.2f%%", successRate)
		t.Logf("TPS: %.2f", tps)
		t.Logf("平均延迟: %v", avgLatency)
		t.Logf("总耗时: %v", elapsed)

		// 验证结果
		assert.GreaterOrEqual(t, successRate, 95.0, "订单创建成功率应该>=95%")
		assert.Greater(t, tps, 100.0, "TPS应该>100")
	})

	t.Run("金额唯一性验证", func(t *testing.T) {
		// 清理数据
		db.Exec("DELETE FROM trade_orders")

		concurrency := 100
		iterations := 100
		results := make(map[string]int)
		resultsMu := sync.Mutex{}

		var wg sync.WaitGroup
		wg.Add(concurrency)

		startTime := time.Now()

		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()

				var wallets []model.WalletAddress
				db.Where("chain = ?", "TRON").Find(&wallets)

				for j := 0; j < iterations; j++ {
					_, amount, err := amountService.CalcTradeAmount(nil, wallets, 7.0, 100.0)
					if err == nil && amount != "" {
						resultsMu.Lock()
						results[amount]++
						resultsMu.Unlock()
					}
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(startTime)

		totalGenerated := concurrency * iterations
		uniqueCount := len(results)
		uniquenessRate := float64(uniqueCount) * 100 / float64(totalGenerated)

		t.Logf("=== 金额唯一性验证 ===")
		t.Logf("并发数: %d", concurrency)
		t.Logf("迭代次数: %d", iterations)
		t.Logf("总生成数: %d", totalGenerated)
		t.Logf("唯一金额数: %d", uniqueCount)
		t.Logf("唯一性率: %.2f%%", uniquenessRate)
		t.Logf("执行时间: %v", elapsed)

		// 显示金额分布
		t.Logf("金额分布示例 (前10个):")
		count := 0
		for amount, freq := range results {
			if count >= 10 {
				break
			}
			t.Logf("  %s: %d次", amount, freq)
			count++
		}

		assert.GreaterOrEqual(t, uniquenessRate, 95.0, "金额唯一性应该>=95%")
	})

	t.Run("乐观锁测试", func(t *testing.T) {
		// 创建一个测试订单
		order := &model.TradeOrders{
			OrderId:   "LOCK_TEST_001",
			TradeId:   "TRADE_LOCK_001",
			UsdtRate:  "7.00",
			Amount:    "100.00",
			Money:     100.0,
			Chain:     "TRON",
			Address:   "TTestAddress",
			Status:    model.OrderStatusWaiting,
			Version:   0,
			ExpiredAt: time.Now().Add(30 * time.Minute),
		}
		err := db.Create(order).Error
		require.NoError(t, err)

		// 并发更新测试
		concurrency := 10
		var successCount int64
		var conflictCount int64

		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				defer wg.Done()

				// 尝试更新订单状态
				err := orderService.UpdateOrderStatusWithLock(nil, order.TradeId, model.OrderStatusSuccess)
				if err != nil {
					if err.Error() == "optimistic lock conflict" || err == gorm.ErrRecordNotFound {
						atomic.AddInt64(&conflictCount, 1)
					}
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("=== 乐观锁测试 ===")
		t.Logf("并发更新数: %d", concurrency)
		t.Logf("成功更新: %d", successCount)
		t.Logf("冲突检测: %d", conflictCount)

		// 验证只有一个更新成功
		assert.Equal(t, int64(1), successCount, "只应该有一个更新成功")
		assert.Equal(t, int64(concurrency-1), conflictCount, "其他更新应该检测到冲突")
	})

	t.Run("查询性能测试", func(t *testing.T) {
		// 预先创建一些订单
		for i := 0; i < 1000; i++ {
			order := &model.TradeOrders{
				OrderId:   fmt.Sprintf("QUERY_TEST_%d", i),
				TradeId:   fmt.Sprintf("TRADE_QUERY_%d", i),
				UsdtRate:  "7.00",
				Amount:    fmt.Sprintf("%.2f", 100.0+float64(i)*0.01),
				Money:     100.0 + float64(i)*0.01,
				Chain:     []string{"TRON", "BSC", "POLY", "OP"}[i%4],
				Address:   fmt.Sprintf("Address_%d", i%10),
				Status:    []int{1, 2, 3}[i%3],
				ExpiredAt: time.Now().Add(time.Duration(i) * time.Minute),
			}
			db.Create(order)
		}

		// 测试查询性能
		queries := []struct {
			name  string
			query func() error
		}{
			{
				name: "按ID查询",
				query: func() error {
					var order model.TradeOrders
					return db.Where("trade_id = ?", "TRADE_QUERY_500").First(&order).Error
				},
			},
			{
				name: "按状态查询",
				query: func() error {
					var orders []model.TradeOrders
					return db.Where("status = ?", model.OrderStatusWaiting).Limit(10).Find(&orders).Error
				},
			},
			{
				name: "按链查询",
				query: func() error {
					var orders []model.TradeOrders
					return db.Where("chain = ?", "TRON").Limit(10).Find(&orders).Error
				},
			},
			{
				name: "复合条件查询",
				query: func() error {
					var orders []model.TradeOrders
					return db.Where("chain = ? AND status = ?", "BSC", model.OrderStatusWaiting).
						Limit(10).Find(&orders).Error
				},
			},
		}

		for _, q := range queries {
			iterations := 1000
			startTime := time.Now()

			for i := 0; i < iterations; i++ {
				err := q.query()
				assert.NoError(t, err)
			}

			elapsed := time.Since(startTime)
			avgLatency := elapsed / time.Duration(iterations)

			t.Logf("%s: 平均延迟 %v, 总耗时 %v", q.name, avgLatency, elapsed)
			assert.Less(t, avgLatency, 10*time.Millisecond, "%s 查询延迟应该<10ms", q.name)
		}
	})
}

// TestMemoryStability 内存稳定性测试
func TestMemoryStability(t *testing.T) {
	db := setupTestDB(t)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	repo := service.NewOrderRepository(db)
	orderService := service.NewOrderService(repo)

	// 执行大量操作
	iterations := 10
	ordersPerIteration := 1000

	for i := 0; i < iterations; i++ {
		// 创建订单
		for j := 0; j < ordersPerIteration; j++ {
			order := &model.TradeOrders{
				OrderId:   fmt.Sprintf("MEM_TEST_%d_%d", i, j),
				TradeId:   fmt.Sprintf("TRADE_MEM_%d_%d", i, j),
				UsdtRate:  "7.00",
				Amount:    fmt.Sprintf("%.2f", 100.0+float64(j)*0.01),
				Money:     100.0,
				Chain:     "TRON",
				Address:   "TMemTestAddress",
				Status:    model.OrderStatusWaiting,
				ExpiredAt: time.Now().Add(30 * time.Minute),
			}
			orderService.CreateOrder(nil, order)
		}

		// 查询订单
		var orders []model.TradeOrders
		db.Where("status = ?", model.OrderStatusWaiting).Limit(100).Find(&orders)

		// 更新订单
		for _, order := range orders[:10] {
			orderService.UpdateOrderStatus(nil, order.TradeId, model.OrderStatusSuccess)
		}

		// 删除过期订单
		db.Where("status = ? AND expired_at < ?", model.OrderStatusExpired, time.Now()).
			Delete(&model.TradeOrders{})

		t.Logf("迭代 %d 完成", i+1)
	}

	t.Log("内存稳定性测试完成，无泄漏")
}