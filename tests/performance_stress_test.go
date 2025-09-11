package tests

import (
	"USDTMore/app/model"
	"fmt"
	"math/rand"
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

// StressTestMetrics 压力测试指标
type StressTestMetrics struct {
	StartTime           time.Time
	EndTime             time.Time
	Duration            time.Duration
	TotalOperations     int64
	SuccessfulOps       int64
	FailedOps           int64
	TPS                 float64
	AverageLatency      time.Duration
	MaxLatency          time.Duration
	MinLatency          time.Duration
	ConcurrentUsers     int
	ErrorRate           float64
	MemoryUsageMB       float64
	CPUUtilization      float64
}

// setupStressTestDB 设置压力测试数据库
func setupStressTestDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		PrepareStmt: true, // 启用预处理语句以提高性能
	})
	if err != nil {
		return nil, err
	}

	// 设置SQLite优化参数
	db.Exec("PRAGMA journal_mode = WAL")
	db.Exec("PRAGMA synchronous = NORMAL")
	db.Exec("PRAGMA cache_size = 1000000")
	db.Exec("PRAGMA temp_store = memory")

	// 创建表结构
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
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			confirmed_at TIMESTAMP NULL
		)
	`).Error
	if err != nil {
		return nil, err
	}

	// 创建索引以提高查询性能
	db.Exec("CREATE INDEX IF NOT EXISTS idx_order_id ON trade_orders(order_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_trade_id ON trade_orders(trade_id)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_status ON trade_orders(status)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_expired_at ON trade_orders(expired_at)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_chain_address_amount ON trade_orders(chain, address, amount)")

	err = db.AutoMigrate(&model.WalletAddress{})
	if err != nil {
		return nil, err
	}

	model.DB = db
	return db, nil
}

// createStressTestOrder 创建压力测试订单
func createStressTestOrder(index int) *model.TradeOrders {
	now := time.Now()
	return &model.TradeOrders{
		OrderId:     fmt.Sprintf("STRESS_%d_%d", index, now.UnixNano()),
		TradeId:     fmt.Sprintf("STID_%d_%d", index, now.UnixNano()),
		TradeHash:   "",
		UsdtRate:    fmt.Sprintf("%.2f", 7.0+rand.Float64()*2.0),
		Amount:      fmt.Sprintf("%.2f", 10.0+rand.Float64()*1000.0),
		Money:       100.0 + rand.Float64()*5000.0,
		Chain:       []string{"TRON", "BSC", "POLY", "OP"}[rand.Intn(4)],
		Address:     fmt.Sprintf("ADDR_%d", rand.Intn(100)),
		FromAddress: "",
		Status:      model.OrderStatusWaiting,
		ReturnUrl:   "https://example.com/return",
		NotifyUrl:   "https://example.com/notify",
		NotifyNum:   0,
		NotifyState: model.OrderNotifyStateFail,
		ExpiredAt:   now.Add(30 * time.Minute),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Test_HighVolumeOrderCreation 高容量订单创建测试
func Test_HighVolumeOrderCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试（短测试模式）")
	}

	db, err := setupStressTestDB()
	require.NoError(t, err)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	testCases := []struct {
		name            string
		totalOrders     int
		concurrentUsers int
		expectedTPS     float64
	}{
		{"轻负载测试", 500, 5, 50.0},
		{"中等负载测试", 1000, 10, 100.0},
		{"重负载测试", 2000, 20, 150.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := runHighVolumeTest(t, db, tc.totalOrders, tc.concurrentUsers)
			
			t.Logf("=== %s 结果 ===", tc.name)
			t.Logf("总订单数: %d", metrics.TotalOperations)
			t.Logf("成功创建: %d", metrics.SuccessfulOps)
			t.Logf("失败数量: %d", metrics.FailedOps)
			t.Logf("测试耗时: %v", metrics.Duration)
			t.Logf("实际TPS: %.2f", metrics.TPS)
			t.Logf("错误率: %.2f%%", metrics.ErrorRate)
			t.Logf("平均延迟: %v", metrics.AverageLatency)
			t.Logf("最大延迟: %v", metrics.MaxLatency)
			
			// 断言性能要求
			assert.Greater(t, metrics.TPS, tc.expectedTPS, 
				fmt.Sprintf("TPS应该大于%.1f", tc.expectedTPS))
			assert.Less(t, metrics.ErrorRate, 5.0, "错误率应该小于5%")
			assert.Less(t, metrics.AverageLatency, time.Millisecond*100, 
				"平均延迟应该小于100ms")
		})
	}
}

// runHighVolumeTest 运行高容量测试
func runHighVolumeTest(t *testing.T, db *gorm.DB, totalOrders, concurrentUsers int) *StressTestMetrics {
	metrics := &StressTestMetrics{
		StartTime:       time.Now(),
		TotalOperations: int64(totalOrders),
		ConcurrentUsers: concurrentUsers,
		MinLatency:      time.Hour, // 初始化为一个很大的值
	}

	var successCount int64
	var failCount int64
	var totalLatency int64
	var maxLatency int64
	var minLatency int64 = int64(time.Hour)

	ordersPerUser := totalOrders / concurrentUsers
	remaining := totalOrders % concurrentUsers

	wg := sync.WaitGroup{}
	
	for user := 0; user < concurrentUsers; user++ {
		wg.Add(1)
		userOrders := ordersPerUser
		if user == 0 {
			userOrders += remaining // 第一个用户处理剩余的订单
		}
		
		go func(userID, orders int) {
			defer wg.Done()
			
			for i := 0; i < orders; i++ {
				startTime := time.Now()
				order := createStressTestOrder(userID*1000 + i)
				
				if err := db.Create(order).Error; err != nil {
					atomic.AddInt64(&failCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
				
				// 记录延迟
				latency := time.Since(startTime).Nanoseconds()
				atomic.AddInt64(&totalLatency, latency)
				
				// 更新最大延迟
				for {
					current := atomic.LoadInt64(&maxLatency)
					if latency <= current || atomic.CompareAndSwapInt64(&maxLatency, current, latency) {
						break
					}
				}
				
				// 更新最小延迟
				for {
					current := atomic.LoadInt64(&minLatency)
					if latency >= current || atomic.CompareAndSwapInt64(&minLatency, current, latency) {
						break
					}
				}
			}
		}(user, userOrders)
	}
	
	wg.Wait()
	
	metrics.EndTime = time.Now()
	metrics.Duration = metrics.EndTime.Sub(metrics.StartTime)
	metrics.SuccessfulOps = successCount
	metrics.FailedOps = failCount
	metrics.TPS = float64(successCount) / metrics.Duration.Seconds()
	metrics.ErrorRate = float64(failCount) / float64(totalOrders) * 100
	metrics.AverageLatency = time.Duration(totalLatency / int64(totalOrders))
	metrics.MaxLatency = time.Duration(maxLatency)
	metrics.MinLatency = time.Duration(minLatency)
	
	return metrics
}

// Test_CalcTradeAmountStress 金额计算压力测试
func Test_CalcTradeAmountStress(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过压力测试（短测试模式）")
	}

	db, err := setupStressTestDB()
	require.NoError(t, err)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	// 创建多个测试钱包地址
	chains := []string{"TRON", "BSC", "POLY", "OP"}
	for _, chain := range chains {
		for i := 0; i < 5; i++ {
			address := &model.WalletAddress{
				Chain:   chain,
				Address: fmt.Sprintf("%s_ADDRESS_%d", chain, i),
			}
			db.Create(address)
		}
	}

	t.Run("金额计算并发压力测试", func(t *testing.T) {
		concurrentUsers := 50
		calculationsPerUser := 100
		totalCalculations := concurrentUsers * calculationsPerUser

		startTime := time.Now()
		var successCount int64
		var failCount int64
		uniqueResults := sync.Map{}

		wg := sync.WaitGroup{}
		for user := 0; user < concurrentUsers; user++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()
				
				for i := 0; i < calculationsPerUser; i++ {
					chain := chains[rand.Intn(len(chains))]
					walletAddresses := model.GetAvailableAddress(chain)
					
					if len(walletAddresses) == 0 {
						atomic.AddInt64(&failCount, 1)
						continue
					}
					
					rate := 6.5 + rand.Float64()*2.0 // 6.5-8.5
					money := 100.0 + rand.Float64()*900.0 // 100-1000
					
					address, amount := model.CalcTradeAmount(walletAddresses, rate, money)
					
					if address.Address != "" && amount != "" {
						atomic.AddInt64(&successCount, 1)
						// 记录唯一结果
						key := fmt.Sprintf("%s_%s_%s", chain, address.Address, amount)
						uniqueResults.Store(key, true)
					} else {
						atomic.AddInt64(&failCount, 1)
					}
				}
			}(user)
		}
		
		wg.Wait()
		duration := time.Since(startTime)
		
		// 统计唯一结果数量
		uniqueCount := 0
		uniqueResults.Range(func(key, value interface{}) bool {
			uniqueCount++
			return true
		})
		
		tps := float64(successCount) / duration.Seconds()
		successRate := float64(successCount) / float64(totalCalculations) * 100
		uniquenessRate := float64(uniqueCount) / float64(successCount) * 100
		
		t.Logf("=== 金额计算并发压力测试结果 ===")
		t.Logf("并发用户: %d", concurrentUsers)
		t.Logf("每用户计算次数: %d", calculationsPerUser)
		t.Logf("总计算次数: %d", totalCalculations)
		t.Logf("成功计算: %d", successCount)
		t.Logf("失败计算: %d", failCount)
		t.Logf("测试耗时: %v", duration)
		t.Logf("计算TPS: %.2f", tps)
		t.Logf("成功率: %.2f%%", successRate)
		t.Logf("唯一结果: %d", uniqueCount)
		t.Logf("唯一性: %.2f%%", uniquenessRate)
		
		// 断言测试结果
		assert.Greater(t, tps, 500.0, "金额计算TPS应该大于500")
		assert.Greater(t, successRate, 95.0, "成功率应该大于95%")
		assert.Greater(t, uniquenessRate, 80.0, "唯一性应该大于80%")
	})
}

// Test_LongRunningStability 长时间运行稳定性测试
func Test_LongRunningStability(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过长时间稳定性测试（短测试模式）")
	}

	db, err := setupStressTestDB()
	require.NoError(t, err)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	t.Run("5分钟稳定性测试", func(t *testing.T) {
		testDuration := 60 * time.Second // 简化为1分钟以便测试
		concurrentWorkers := 5
		
		var totalOps int64
		var successOps int64
		var errors []string
		var errorsMutex sync.Mutex
		
		startTime := time.Now()
		wg := sync.WaitGroup{}
		stopChan := make(chan struct{})
		
		// 启动定时器
		go func() {
			time.Sleep(testDuration)
			close(stopChan)
		}()
		
		// 启动工作协程
		for worker := 0; worker < concurrentWorkers; worker++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				counter := 0
				
				for {
					select {
					case <-stopChan:
						return
					default:
						counter++
						atomic.AddInt64(&totalOps, 1)
						
						order := createStressTestOrder(workerID*10000 + counter)
						
						if err := db.Create(order).Error; err != nil {
							errorsMutex.Lock()
							if len(errors) < 10 { // 只记录前10个错误
								errors = append(errors, err.Error())
							}
							errorsMutex.Unlock()
						} else {
							atomic.AddInt64(&successOps, 1)
						}
						
						// 小延迟避免过度占用CPU
						time.Sleep(time.Millisecond * 10)
					}
				}
			}(worker)
		}
		
		wg.Wait()
		actualDuration := time.Since(startTime)
		
		avgTPS := float64(successOps) / actualDuration.Seconds()
		errorRate := float64(totalOps-successOps) / float64(totalOps) * 100
		
		t.Logf("=== 稳定性测试结果 ===")
		t.Logf("测试时长: %v", actualDuration)
		t.Logf("并发工作者: %d", concurrentWorkers)
		t.Logf("总操作数: %d", totalOps)
		t.Logf("成功操作: %d", successOps)
		t.Logf("平均TPS: %.2f", avgTPS)
		t.Logf("错误率: %.2f%%", errorRate)
		
		if len(errors) > 0 {
			t.Logf("前几个错误:")
			for i, err := range errors {
				t.Logf("  %d. %s", i+1, err)
			}
		}
		
		// 断言稳定性要求
		assert.Greater(t, avgTPS, 20.0, "长时间运行平均TPS应该大于20")
		assert.Less(t, errorRate, 10.0, "长时间运行错误率应该小于10%")
		assert.Greater(t, totalOps, int64(100), "应该执行足够多的操作")
	})
}

// Benchmark_OrderOperations 订单操作基准测试
func Benchmark_OrderOperations(b *testing.B) {
	db, err := setupStressTestDB()
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()
	
	b.Run("订单创建", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			order := createStressTestOrder(i)
			if err := db.Create(order).Error; err != nil {
				b.Errorf("订单创建失败: %v", err)
			}
		}
	})
	
	// 预填充一些数据用于查询测试
	for i := 0; i < 1000; i++ {
		order := createStressTestOrder(i)
		db.Create(order)
	}
	
	b.Run("订单查询", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var order model.TradeOrders
			db.Where("order_id LIKE ?", "STRESS_%").First(&order)
		}
	})
	
	b.Run("状态更新", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			order := createStressTestOrder(i)
			db.Create(order)
			
			order.Status = model.OrderStatusSuccess
			order.TradeHash = fmt.Sprintf("hash_%d", i)
			order.FromAddress = fmt.Sprintf("from_%d", i)
			
			db.Save(order)
		}
	})
}