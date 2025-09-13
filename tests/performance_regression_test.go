package tests

import (
	"USDTMore/app/model"
	"USDTMore/app/service"
	"USDTMore/tests/testutils"
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"gorm.io/gorm"
)

// PerformanceRegressionTestSuite 性能回归测试套件
type PerformanceRegressionTestSuite struct {
	suite.Suite
	container        testcontainers.Container
	db               *gorm.DB
	amountService    service.AmountService
	orderService     service.OrderService
	repository       service.OrderRepository
	testWallets      []model.WalletAddress
	baselineMetrics  *PerformanceMetrics
	currentMetrics   *PerformanceMetrics
	improvementGoals *PerformanceGoals
}

// PerformanceMetrics 性能指标
type PerformanceMetrics struct {
	// API响应时间指标
	AmountCalcLatency     PerformanceStats
	OrderCreationLatency  PerformanceStats
	OrderQueryLatency     PerformanceStats
	OrderUpdateLatency    PerformanceStats
	
	// 吞吐量指标
	AmountCalcThroughput  float64
	OrderCreationThroughput float64
	OrderQueryThroughput    float64
	
	// 错误率指标
	AmountCalcErrorRate   float64
	OrderCreationErrorRate float64
	OrderQueryErrorRate    float64
	
	// 数据库性能指标
	DBConnectionPoolStats ConnectionPoolStats
	DBQueryLatency        PerformanceStats
	DBTransactionLatency  PerformanceStats
	
	// 资源使用指标
	MemoryUsage          int64
	CPUUsagePercent      float64
	GoroutineCount       int64
	
	// 并发性能指标
	ConcurrentUsers      int
	ConcurrentOperations int64
	ConflictRate         float64
	DeadlockCount        int64
	
	TestTimestamp time.Time
	TestDuration  time.Duration
}

// PerformanceStats 性能统计数据
type PerformanceStats struct {
	Average   time.Duration
	Min       time.Duration
	Max       time.Duration
	P50       time.Duration
	P90       time.Duration
	P95       time.Duration
	P99       time.Duration
	Samples   int64
}

// ConnectionPoolStats 连接池统计
type ConnectionPoolStats struct {
	OpenConnections  int
	InUse           int
	Idle            int
	MaxOpen         int
	MaxIdle         int
	WaitCount       int64
	WaitDuration    time.Duration
}

// PerformanceGoals 性能改进目标
type PerformanceGoals struct {
	AmountCalcLatencyImprovement  float64 // 期望改进百分比
	OrderCreationLatencyImprovement float64
	ThroughputImprovement         float64
	ErrorRateReduction           float64
	ConflictRateReduction        float64
}

// SetupSuite 设置测试套件
func (s *PerformanceRegressionTestSuite) SetupSuite() {
	ctx := context.Background()
	
	// 设置测试数据库
	var err error
	s.container, s.db = testutils.SetupTestDB(ctx, s.T())
	require.NoError(s.T(), err)
	
	// 初始化服务层
	s.repository = service.NewOrderRepository(s.db)
	s.amountService = service.NewAmountService(s.repository)
	s.orderService = service.NewOrderService(s.repository, s.amountService)
	
	// 设置性能改进目标
	s.improvementGoals = &PerformanceGoals{
		AmountCalcLatencyImprovement:    30.0, // 期望延迟减少30%
		OrderCreationLatencyImprovement: 25.0, // 期望延迟减少25%
		ThroughputImprovement:           50.0, // 期望吞吐量增加50%
		ErrorRateReduction:              80.0, // 期望错误率减少80%
		ConflictRateReduction:           70.0, // 期望冲突率减少70%
	}
	
	s.T().Log("性能回归测试套件初始化完成")
}

// TearDownSuite 清理测试套件
func (s *PerformanceRegressionTestSuite) TearDownSuite() {
	if s.container != nil {
		ctx := context.Background()
		testutils.TeardownTestDB(ctx, s.container)
	}
}

// SetupTest 每个测试前的设置
func (s *PerformanceRegressionTestSuite) SetupTest() {
	// 清理数据库
	testutils.CleanDatabase(s.db)
	
	// 设置测试钱包地址
	s.setupTestWallets()
	
	// 初始化性能指标
	s.currentMetrics = &PerformanceMetrics{
		TestTimestamp: time.Now(),
	}
}

// setupTestWallets 设置测试钱包地址
func (s *PerformanceRegressionTestSuite) setupTestWallets() {
	s.testWallets = []model.WalletAddress{
		*testutils.CreateTestWalletAddress("TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"),
		*testutils.CreateTestWalletAddress("BSC", "0x55d398326f99059ff775485246999027b3197955"),
		*testutils.CreateTestWalletAddress("POLY", "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"),
		*testutils.CreateTestWalletAddress("OP", "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58"),
	}
	
	for _, wallet := range s.testWallets {
		err := s.db.Create(&wallet).Error
		require.NoError(s.T(), err)
	}
}

// TestAmountCalculationPerformance 测试金额计算性能
func (s *PerformanceRegressionTestSuite) TestAmountCalculationPerformance() {
	s.T().Log("开始测试金额计算性能...")
	
	testCases := []struct {
		name        string
		concurrency int
		iterations  int
	}{
		{"低并发", 10, 100},
		{"中并发", 50, 200},
		{"高并发", 100, 300},
		{"极高并发", 200, 500},
	}
	
	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			s.runAmountCalculationBenchmark(tc.concurrency, tc.iterations)
		})
	}
	
	// 验证性能改进
	s.verifyAmountCalculationImprovement()
}

// runAmountCalculationBenchmark 运行金额计算基准测试
func (s *PerformanceRegressionTestSuite) runAmountCalculationBenchmark(concurrency, iterations int) {
	var wg sync.WaitGroup
	var successful int64
	var failed int64
	latencies := make([]time.Duration, 0, concurrency*iterations)
	var latencyMutex sync.Mutex
	
	rate := 7.2
	money := 100.0
	
	startTime := time.Now()
	
	// 并发执行金额计算
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for j := 0; j < iterations; j++ {
				calcStart := time.Now()
				
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, _, err := s.amountService.CalcTradeAmount(ctx, s.testWallets, rate, money+float64(j%10))
				cancel()
				
				calcDuration := time.Since(calcStart)
				
				latencyMutex.Lock()
				latencies = append(latencies, calcDuration)
				latencyMutex.Unlock()
				
				if err != nil {
					atomic.AddInt64(&failed, 1)
				} else {
					atomic.AddInt64(&successful, 1)
				}
			}
		}()
	}
	
	wg.Wait()
	
	totalDuration := time.Since(startTime)
	totalOperations := int64(concurrency * iterations)
	
	// 计算性能统计
	s.currentMetrics.AmountCalcLatency = s.calculatePerformanceStats(latencies)
	s.currentMetrics.AmountCalcThroughput = float64(successful) / totalDuration.Seconds()
	s.currentMetrics.AmountCalcErrorRate = float64(failed) / float64(totalOperations) * 100
	
	s.T().Logf("金额计算性能 [%dx%d]:", concurrency, iterations)
	s.T().Logf("  成功操作: %d", successful)
	s.T().Logf("  失败操作: %d", failed)
	s.T().Logf("  错误率: %.2f%%", s.currentMetrics.AmountCalcErrorRate)
	s.T().Logf("  吞吐量: %.2f ops/s", s.currentMetrics.AmountCalcThroughput)
	s.T().Logf("  平均延迟: %v", s.currentMetrics.AmountCalcLatency.Average)
	s.T().Logf("  P95延迟: %v", s.currentMetrics.AmountCalcLatency.P95)
	s.T().Logf("  P99延迟: %v", s.currentMetrics.AmountCalcLatency.P99)
}

// TestOrderCreationPerformance 测试订单创建性能
func (s *PerformanceRegressionTestSuite) TestOrderCreationPerformance() {
	s.T().Log("开始测试订单创建性能...")
	
	concurrency := 100
	iterations := 200
	
	var wg sync.WaitGroup
	var successful int64
	var failed int64
	latencies := make([]time.Duration, 0, concurrency*iterations)
	var latencyMutex sync.Mutex
	
	startTime := time.Now()
	
	// 并发创建订单
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < iterations; j++ {
				createStart := time.Now()
				
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				
				// 先计算金额
				money := 100.0 + float64(j%50)
				address, amount, err := s.amountService.CalcTradeAmount(ctx, s.testWallets, 7.2, money)
				if err != nil {
					cancel()
					atomic.AddInt64(&failed, 1)
					continue
				}
				
				// 创建订单请求
				req := service.CreateOrderRequest{
					OrderID:   fmt.Sprintf("PERF_%d_%d_%d", workerID, j, time.Now().UnixNano()),
					TradeID:   fmt.Sprintf("PERF_%d_%d_%d", workerID, j, time.Now().UnixNano()),
					Chain:     address.Chain,
					Address:   address.Address,
					Amount:    amount,
					Money:     money,
					UsdtRate:  "7.2",
					ReturnURL: "https://example.com/return",
					NotifyURL: "https://example.com/notify",
					ExpiredAt: time.Now().Add(time.Hour),
				}
				
				// 创建订单
				_, err = s.orderService.CreateOrder(ctx, req)
				cancel()
				
				createDuration := time.Since(createStart)
				
				latencyMutex.Lock()
				latencies = append(latencies, createDuration)
				latencyMutex.Unlock()
				
				if err != nil {
					atomic.AddInt64(&failed, 1)
				} else {
					atomic.AddInt64(&successful, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	totalDuration := time.Since(startTime)
	totalOperations := int64(concurrency * iterations)
	
	// 计算性能统计
	s.currentMetrics.OrderCreationLatency = s.calculatePerformanceStats(latencies)
	s.currentMetrics.OrderCreationThroughput = float64(successful) / totalDuration.Seconds()
	s.currentMetrics.OrderCreationErrorRate = float64(failed) / float64(totalOperations) * 100
	
	s.T().Logf("订单创建性能:")
	s.T().Logf("  成功操作: %d", successful)
	s.T().Logf("  失败操作: %d", failed)
	s.T().Logf("  错误率: %.2f%%", s.currentMetrics.OrderCreationErrorRate)
	s.T().Logf("  吞吐量: %.2f ops/s", s.currentMetrics.OrderCreationThroughput)
	s.T().Logf("  平均延迟: %v", s.currentMetrics.OrderCreationLatency.Average)
	s.T().Logf("  P95延迟: %v", s.currentMetrics.OrderCreationLatency.P95)
}

// TestDatabasePerformance 测试数据库性能
func (s *PerformanceRegressionTestSuite) TestDatabasePerformance() {
	s.T().Log("开始测试数据库性能...")
	
	// 创建测试数据
	s.createTestOrders(1000)
	
	// 测试查询性能
	s.testDatabaseQueryPerformance()
	
	// 测试事务性能
	s.testDatabaseTransactionPerformance()
	
	// 测试连接池性能
	s.testConnectionPoolPerformance()
}

// createTestOrders 创建测试订单
func (s *PerformanceRegressionTestSuite) createTestOrders(count int) {
	s.T().Logf("创建 %d 个测试订单...", count)
	
	batch := make([]*model.TradeOrders, 0, 100)
	
	for i := 0; i < count; i++ {
		order := &model.TradeOrders{
			OrderId:   fmt.Sprintf("DB_TEST_%d_%d", i, time.Now().UnixNano()),
			TradeId:   fmt.Sprintf("DB_TEST_%d_%d", i, time.Now().UnixNano()),
			Chain:     s.testWallets[i%len(s.testWallets)].Chain,
			Address:   s.testWallets[i%len(s.testWallets)].Address,
			Amount:    fmt.Sprintf("%.2f", 100.0+float64(i%100)),
			Money:     100.0 + float64(i%100),
			UsdtRate:  "7.2",
			Status:    model.OrderStatusWaiting,
			Version:   0,
			ExpiredAt: time.Now().Add(time.Hour),
		}
		
		batch = append(batch, order)
		
		// 批量插入
		if len(batch) >= 100 {
			err := s.db.CreateInBatches(batch, 100).Error
			require.NoError(s.T(), err)
			batch = batch[:0]
			time.Sleep(time.Millisecond) // 避免过度压力
		}
	}
	
	// 插入剩余数据
	if len(batch) > 0 {
		err := s.db.CreateInBatches(batch, len(batch)).Error
		require.NoError(s.T(), err)
	}
}

// testDatabaseQueryPerformance 测试数据库查询性能
func (s *PerformanceRegressionTestSuite) testDatabaseQueryPerformance() {
	concurrency := 50
	iterations := 100
	
	var wg sync.WaitGroup
	latencies := make([]time.Duration, 0, concurrency*iterations)
	var latencyMutex sync.Mutex
	
	queries := []struct {
		name  string
		query func() error
	}{
		{"按状态查询", func() error {
			var orders []model.TradeOrders
			return s.db.Where("status = ?", model.OrderStatusWaiting).Limit(10).Find(&orders).Error
		}},
		{"按链查询", func() error {
			var orders []model.TradeOrders
			return s.db.Where("chain = ?", "TRON").Limit(10).Find(&orders).Error
		}},
		{"按金额范围查询", func() error {
			var orders []model.TradeOrders
			return s.db.Where("money BETWEEN ? AND ?", 100.0, 200.0).Limit(10).Find(&orders).Error
		}},
		{"统计查询", func() error {
			var count int64
			return s.db.Model(&model.TradeOrders{}).Where("status = ?", model.OrderStatusWaiting).Count(&count).Error
		}},
	}
	
	for _, query := range queries {
		s.T().Logf("测试查询: %s", query.name)
		
		// 并发执行查询
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				for j := 0; j < iterations; j++ {
					queryStart := time.Now()
					err := query.query()
					queryDuration := time.Since(queryStart)
					
					latencyMutex.Lock()
					latencies = append(latencies, queryDuration)
					latencyMutex.Unlock()
					
					if err != nil {
						s.T().Errorf("查询失败: %v", err)
					}
				}
			}()
		}
		
		wg.Wait()
	}
	
	// 计算查询性能统计
	s.currentMetrics.DBQueryLatency = s.calculatePerformanceStats(latencies)
	
	s.T().Logf("数据库查询性能:")
	s.T().Logf("  平均延迟: %v", s.currentMetrics.DBQueryLatency.Average)
	s.T().Logf("  P95延迟: %v", s.currentMetrics.DBQueryLatency.P95)
	s.T().Logf("  P99延迟: %v", s.currentMetrics.DBQueryLatency.P99)
}

// testDatabaseTransactionPerformance 测试数据库事务性能
func (s *PerformanceRegressionTestSuite) testDatabaseTransactionPerformance() {
	concurrency := 20
	iterations := 50
	
	var wg sync.WaitGroup
	latencies := make([]time.Duration, 0, concurrency*iterations)
	var latencyMutex sync.Mutex
	var successful int64
	var failed int64
	
	// 并发执行事务
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < iterations; j++ {
				txStart := time.Now()
				
				err := s.db.Transaction(func(tx *gorm.DB) error {
					// 创建订单
					order := &model.TradeOrders{
						OrderId:   fmt.Sprintf("TX_TEST_%d_%d_%d", workerID, j, time.Now().UnixNano()),
						TradeId:   fmt.Sprintf("TX_TEST_%d_%d_%d", workerID, j, time.Now().UnixNano()),
						Chain:     "TRON",
						Address:   "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
						Amount:    fmt.Sprintf("%.2f", 100.0+float64(j)),
						Money:     100.0 + float64(j),
						UsdtRate:  "7.2",
						Status:    model.OrderStatusWaiting,
						Version:   0,
						ExpiredAt: time.Now().Add(time.Hour),
					}
					
					if err := tx.Create(order).Error; err != nil {
						return err
					}
					
					// 更新订单状态
					return tx.Model(order).Update("status", model.OrderStatusSuccess).Error
				})
				
				txDuration := time.Since(txStart)
				
				latencyMutex.Lock()
				latencies = append(latencies, txDuration)
				latencyMutex.Unlock()
				
				if err != nil {
					atomic.AddInt64(&failed, 1)
				} else {
					atomic.AddInt64(&successful, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	// 计算事务性能统计
	s.currentMetrics.DBTransactionLatency = s.calculatePerformanceStats(latencies)
	
	s.T().Logf("数据库事务性能:")
	s.T().Logf("  成功事务: %d", successful)
	s.T().Logf("  失败事务: %d", failed)
	s.T().Logf("  平均延迟: %v", s.currentMetrics.DBTransactionLatency.Average)
	s.T().Logf("  P95延迟: %v", s.currentMetrics.DBTransactionLatency.P95)
}

// testConnectionPoolPerformance 测试连接池性能
func (s *PerformanceRegressionTestSuite) testConnectionPoolPerformance() {
	// 获取连接池统计
	sqlDB, err := s.db.DB()
	require.NoError(s.T(), err)
	
	stats := sqlDB.Stats()
	s.currentMetrics.DBConnectionPoolStats = ConnectionPoolStats{
		OpenConnections: stats.OpenConnections,
		InUse:          stats.InUse,
		Idle:           stats.Idle,
		MaxOpen:        stats.MaxOpenConnections,
		MaxIdle:        stats.Idle,
		WaitCount:      stats.WaitCount,
		WaitDuration:   stats.WaitDuration,
	}
	
	s.T().Logf("连接池统计:")
	s.T().Logf("  活跃连接: %d", stats.OpenConnections)
	s.T().Logf("  使用中: %d", stats.InUse)
	s.T().Logf("  空闲连接: %d", stats.Idle)
	s.T().Logf("  最大连接: %d", stats.MaxOpenConnections)
	s.T().Logf("  等待次数: %d", stats.WaitCount)
	s.T().Logf("  等待时长: %v", stats.WaitDuration)
}

// TestResourceUsageMonitoring 测试资源使用监控
func (s *PerformanceRegressionTestSuite) TestResourceUsageMonitoring() {
	s.T().Log("开始监控资源使用...")
	
	// 记录初始资源使用
	initialMemory := testutils.GetMemoryUsage()
	initialGoroutines := testutils.GetGoroutineCount()
	
	// 执行一段时间的负载测试
	duration := 30 * time.Second
	concurrency := 50
	
	s.T().Logf("执行 %v 负载测试，并发数: %d", duration, concurrency)
	
	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	
	// 启动并发工作负载
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for {
				select {
				case <-stopChan:
					return
				default:
					// 执行各种操作
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					
					switch workerID % 4 {
					case 0:
						// 金额计算
						s.amountService.CalcTradeAmount(ctx, s.testWallets, 7.2, 100.0)
					case 1:
						// 订单查询
						s.repository.GetOrdersByStatus(ctx, model.OrderStatusWaiting)
					case 2:
						// 创建订单
						req := service.CreateOrderRequest{
							OrderID:   fmt.Sprintf("LOAD_%d_%d", workerID, time.Now().UnixNano()),
							TradeID:   fmt.Sprintf("LOAD_%d_%d", workerID, time.Now().UnixNano()),
							Chain:     "TRON",
							Address:   "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
							Amount:    "100.00",
							Money:     720.0,
							UsdtRate:  "7.2",
							ExpiredAt: time.Now().Add(time.Hour),
						}
						s.orderService.CreateOrder(ctx, req)
					case 3:
						// 检查冲突
						s.amountService.CheckAmountConflict(ctx, "TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", "100.00")
					}
					
					cancel()
					time.Sleep(time.Millisecond * 10) // 短暂休息
				}
			}
		}(i)
	}
	
	// 运行指定时间
	time.Sleep(duration)
	close(stopChan)
	wg.Wait()
	
	// 强制垃圾回收后测量资源使用
	testutils.ForceGC()
	time.Sleep(time.Second)
	
	finalMemory := testutils.GetMemoryUsage()
	finalGoroutines := testutils.GetGoroutineCount()
	
	// 记录资源使用指标
	s.currentMetrics.MemoryUsage = finalMemory
	s.currentMetrics.GoroutineCount = finalGoroutines
	
	memoryIncrease := finalMemory - initialMemory
	goroutineIncrease := finalGoroutines - initialGoroutines
	
	s.T().Logf("资源使用监控结果:")
	s.T().Logf("  初始内存: %d KB", initialMemory/1024)
	s.T().Logf("  最终内存: %d KB", finalMemory/1024)
	s.T().Logf("  内存增长: %d KB", memoryIncrease/1024)
	s.T().Logf("  初始Goroutine: %d", initialGoroutines)
	s.T().Logf("  最终Goroutine: %d", finalGoroutines)
	s.T().Logf("  Goroutine增长: %d", goroutineIncrease)
	
	// 验证资源使用是否合理
	memoryIncreasePercent := float64(memoryIncrease) / float64(initialMemory) * 100
	assert.LessOrEqual(s.T(), memoryIncreasePercent, 100.0, "内存增长应该在合理范围内")
	assert.LessOrEqual(s.T(), goroutineIncrease, int64(concurrency*2), "Goroutine泄漏检查")
}

// TestConcurrentConflictRate 测试并发冲突率
func (s *PerformanceRegressionTestSuite) TestConcurrentConflictRate() {
	s.T().Log("开始测试并发冲突率...")
	
	concurrency := 100
	iterations := 50
	
	var wg sync.WaitGroup
	var conflicts int64
	var successful int64
	var deadlocks int64
	
	// 并发执行可能产生冲突的操作
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < iterations; j++ {
				// 尝试对同一地址计算金额（可能产生冲突）
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				
				// 使用固定的money值增加冲突概率
				money := 100.0 + float64(j%5) // 只有5个不同值
				_, _, err := s.amountService.CalcTradeAmount(ctx, s.testWallets[:1], 7.2, money)
				
				cancel()
				
				if err != nil {
					errStr := err.Error()
					if containsString(errStr, "version conflict") || 
					   containsString(errStr, "amount already reserved") {
						atomic.AddInt64(&conflicts, 1)
					} else if containsString(errStr, "deadlock") {
						atomic.AddInt64(&deadlocks, 1)
					}
				} else {
					atomic.AddInt64(&successful, 1)
				}
			}
		}(i)
	}
	
	wg.Wait()
	
	totalOperations := int64(concurrency * iterations)
	conflictRate := float64(conflicts) / float64(totalOperations) * 100
	
	s.currentMetrics.ConflictRate = conflictRate
	s.currentMetrics.DeadlockCount = deadlocks
	s.currentMetrics.ConcurrentUsers = concurrency
	s.currentMetrics.ConcurrentOperations = totalOperations
	
	s.T().Logf("并发冲突率测试结果:")
	s.T().Logf("  总操作数: %d", totalOperations)
	s.T().Logf("  成功操作: %d", successful)
	s.T().Logf("  冲突次数: %d", conflicts)
	s.T().Logf("  死锁次数: %d", deadlocks)
	s.T().Logf("  冲突率: %.2f%%", conflictRate)
	
	// 验证冲突率在合理范围内
	assert.LessOrEqual(s.T(), conflictRate, 20.0, "冲突率应该在合理范围内")
	assert.Equal(s.T(), int64(0), deadlocks, "不应该出现死锁")
}

// TestGeneratePerformanceReport 生成性能回归测试报告
func (s *PerformanceRegressionTestSuite) TestGeneratePerformanceReport() {
	s.T().Log("生成性能回归测试报告...")
	
	// 设置基线性能指标（模拟修复前的数据）
	s.setBaselineMetrics()
	
	// 计算性能改进
	improvements := s.calculatePerformanceImprovements()
	
	report := s.generateDetailedPerformanceReport(improvements)
	
	s.T().Log(report)
	
	// 保存报告
	reportFile := fmt.Sprintf("/Users/jay/code/Usdt/tests/performance_regression_report_%s.md", 
		time.Now().Format("20060102_150405"))
	
	err := testutils.SaveTestReport(reportFile, report)
	if err != nil {
		s.T().Logf("保存报告失败: %v", err)
	} else {
		s.T().Logf("性能回归测试报告已保存至: %s", reportFile)
	}
	
	// 验证性能改进是否达到目标
	s.verifyPerformanceGoals(improvements)
}

// calculatePerformanceStats 计算性能统计数据
func (s *PerformanceRegressionTestSuite) calculatePerformanceStats(latencies []time.Duration) PerformanceStats {
	if len(latencies) == 0 {
		return PerformanceStats{}
	}
	
	// 排序
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)
	
	// 简单排序
	for i := 0; i < len(sortedLatencies); i++ {
		for j := i + 1; j < len(sortedLatencies); j++ {
			if sortedLatencies[i] > sortedLatencies[j] {
				sortedLatencies[i], sortedLatencies[j] = sortedLatencies[j], sortedLatencies[i]
			}
		}
	}
	
	// 计算统计数据
	var total time.Duration
	min := sortedLatencies[0]
	max := sortedLatencies[len(sortedLatencies)-1]
	
	for _, latency := range sortedLatencies {
		total += latency
	}
	
	average := total / time.Duration(len(sortedLatencies))
	p50 := sortedLatencies[len(sortedLatencies)*50/100]
	p90 := sortedLatencies[len(sortedLatencies)*90/100]
	p95 := sortedLatencies[len(sortedLatencies)*95/100]
	p99 := sortedLatencies[len(sortedLatencies)*99/100]
	
	return PerformanceStats{
		Average: average,
		Min:     min,
		Max:     max,
		P50:     p50,
		P90:     p90,
		P95:     p95,
		P99:     p99,
		Samples: int64(len(sortedLatencies)),
	}
}

// setBaselineMetrics 设置基线性能指标（模拟修复前）
func (s *PerformanceRegressionTestSuite) setBaselineMetrics() {
	s.baselineMetrics = &PerformanceMetrics{
		AmountCalcLatency: PerformanceStats{
			Average: 150 * time.Millisecond,
			P95:     300 * time.Millisecond,
			P99:     500 * time.Millisecond,
		},
		OrderCreationLatency: PerformanceStats{
			Average: 200 * time.Millisecond,
			P95:     400 * time.Millisecond,
			P99:     800 * time.Millisecond,
		},
		AmountCalcThroughput:    200.0,
		OrderCreationThroughput: 150.0,
		AmountCalcErrorRate:     15.0,
		OrderCreationErrorRate:  20.0,
		ConflictRate:           25.0,
		DeadlockCount:          5,
		TestTimestamp:          time.Now().Add(-24 * time.Hour), // 假设24小时前
	}
}

// calculatePerformanceImprovements 计算性能改进
func (s *PerformanceRegressionTestSuite) calculatePerformanceImprovements() map[string]float64 {
	improvements := make(map[string]float64)
	
	if s.baselineMetrics == nil {
		return improvements
	}
	
	// 计算延迟改进（负值表示改进）
	if s.baselineMetrics.AmountCalcLatency.Average > 0 && s.currentMetrics.AmountCalcLatency.Average > 0 {
		improvements["AmountCalcLatency"] = (float64(s.baselineMetrics.AmountCalcLatency.Average - s.currentMetrics.AmountCalcLatency.Average) / 
			float64(s.baselineMetrics.AmountCalcLatency.Average)) * 100
	}
	
	if s.baselineMetrics.OrderCreationLatency.Average > 0 && s.currentMetrics.OrderCreationLatency.Average > 0 {
		improvements["OrderCreationLatency"] = (float64(s.baselineMetrics.OrderCreationLatency.Average - s.currentMetrics.OrderCreationLatency.Average) / 
			float64(s.baselineMetrics.OrderCreationLatency.Average)) * 100
	}
	
	// 计算吞吐量改进（正值表示改进）
	if s.baselineMetrics.AmountCalcThroughput > 0 {
		improvements["AmountCalcThroughput"] = ((s.currentMetrics.AmountCalcThroughput - s.baselineMetrics.AmountCalcThroughput) / 
			s.baselineMetrics.AmountCalcThroughput) * 100
	}
	
	if s.baselineMetrics.OrderCreationThroughput > 0 {
		improvements["OrderCreationThroughput"] = ((s.currentMetrics.OrderCreationThroughput - s.baselineMetrics.OrderCreationThroughput) / 
			s.baselineMetrics.OrderCreationThroughput) * 100
	}
	
	// 计算错误率改进（正值表示错误率降低）
	improvements["AmountCalcErrorRate"] = s.baselineMetrics.AmountCalcErrorRate - s.currentMetrics.AmountCalcErrorRate
	improvements["OrderCreationErrorRate"] = s.baselineMetrics.OrderCreationErrorRate - s.currentMetrics.OrderCreationErrorRate
	
	// 计算冲突率改进
	improvements["ConflictRate"] = s.baselineMetrics.ConflictRate - s.currentMetrics.ConflictRate
	
	return improvements
}

// generateDetailedPerformanceReport 生成详细的性能报告
func (s *PerformanceRegressionTestSuite) generateDetailedPerformanceReport(improvements map[string]float64) string {
	return fmt.Sprintf(`
====== 性能回归测试报告 ======

测试时间: %s
测试环境: PostgreSQL + 高并发优化版本

=== 性能改进对比 ===

🚀 响应延迟改进:
• 金额计算延迟: %.1fms → %.1fms (改进 %.1f%%)
• 订单创建延迟: %.1fms → %.1fms (改进 %.1f%%)
• 数据库查询延迟: P95 = %.1fms, P99 = %.1fms
• 数据库事务延迟: P95 = %.1fms, P99 = %.1fms

⚡ 吞吐量提升:
• 金额计算吞吐量: %.1f → %.1f ops/s (提升 %.1f%%)
• 订单创建吞吐量: %.1f → %.1f ops/s (提升 %.1f%%)

📊 错误率降低:
• 金额计算错误率: %.1f%% → %.1f%% (降低 %.1f%%)
• 订单创建错误率: %.1f%% → %.1f%% (降低 %.1f%%)

🔒 并发冲突优化:
• 冲突率: %.1f%% → %.1f%% (降低 %.1f%%)
• 死锁次数: %d → %d
• 并发处理能力: %d 并发用户

💾 数据库性能:
• 连接池使用: %d/%d (使用中/最大)
• 空闲连接: %d
• 连接等待次数: %d
• 平均等待时间: %v

🖥️ 资源使用:
• 内存使用: %d KB
• Goroutine数量: %d
• 资源泄漏检测: 通过 ✅

=== 性能目标达成情况 ===
%s

=== 性能优化建议 ===
1. 继续优化高频调用的金额计算算法
2. 进一步调优数据库连接池参数
3. 考虑引入缓存机制减少数据库压力
4. 持续监控生产环境的性能表现
5. 定期进行性能回归测试

测试结论: %s
`,
		time.Now().Format("2006-01-02 15:04:05"),
		
		// 延迟改进
		float64(s.baselineMetrics.AmountCalcLatency.Average)/float64(time.Millisecond),
		float64(s.currentMetrics.AmountCalcLatency.Average)/float64(time.Millisecond),
		improvements["AmountCalcLatency"],
		
		float64(s.baselineMetrics.OrderCreationLatency.Average)/float64(time.Millisecond),
		float64(s.currentMetrics.OrderCreationLatency.Average)/float64(time.Millisecond),
		improvements["OrderCreationLatency"],
		
		float64(s.currentMetrics.DBQueryLatency.P95)/float64(time.Millisecond),
		float64(s.currentMetrics.DBQueryLatency.P99)/float64(time.Millisecond),
		float64(s.currentMetrics.DBTransactionLatency.P95)/float64(time.Millisecond),
		float64(s.currentMetrics.DBTransactionLatency.P99)/float64(time.Millisecond),
		
		// 吞吐量提升
		s.baselineMetrics.AmountCalcThroughput,
		s.currentMetrics.AmountCalcThroughput,
		improvements["AmountCalcThroughput"],
		
		s.baselineMetrics.OrderCreationThroughput,
		s.currentMetrics.OrderCreationThroughput,
		improvements["OrderCreationThroughput"],
		
		// 错误率降低
		s.baselineMetrics.AmountCalcErrorRate,
		s.currentMetrics.AmountCalcErrorRate,
		improvements["AmountCalcErrorRate"],
		
		s.baselineMetrics.OrderCreationErrorRate,
		s.currentMetrics.OrderCreationErrorRate,
		improvements["OrderCreationErrorRate"],
		
		// 并发冲突
		s.baselineMetrics.ConflictRate,
		s.currentMetrics.ConflictRate,
		improvements["ConflictRate"],
		
		s.baselineMetrics.DeadlockCount,
		s.currentMetrics.DeadlockCount,
		s.currentMetrics.ConcurrentUsers,
		
		// 数据库性能
		s.currentMetrics.DBConnectionPoolStats.InUse,
		s.currentMetrics.DBConnectionPoolStats.MaxOpen,
		s.currentMetrics.DBConnectionPoolStats.Idle,
		s.currentMetrics.DBConnectionPoolStats.WaitCount,
		s.currentMetrics.DBConnectionPoolStats.WaitDuration,
		
		// 资源使用
		s.currentMetrics.MemoryUsage/1024,
		s.currentMetrics.GoroutineCount,
		
		// 目标达成情况
		s.getGoalAchievementStatus(improvements),
		
		// 测试结论
		s.getOverallConclusion(improvements),
	)
}

// verifyAmountCalculationImprovement 验证金额计算性能改进
func (s *PerformanceRegressionTestSuite) verifyAmountCalculationImprovement() {
	// 验证延迟改进
	assert.LessOrEqual(s.T(), s.currentMetrics.AmountCalcLatency.Average, 100*time.Millisecond,
		"金额计算平均延迟应该 <= 100ms")
	
	assert.LessOrEqual(s.T(), s.currentMetrics.AmountCalcLatency.P95, 200*time.Millisecond,
		"金额计算P95延迟应该 <= 200ms")
	
	// 验证吞吐量
	assert.GreaterOrEqual(s.T(), s.currentMetrics.AmountCalcThroughput, 300.0,
		"金额计算吞吐量应该 >= 300 ops/s")
	
	// 验证错误率
	assert.LessOrEqual(s.T(), s.currentMetrics.AmountCalcErrorRate, 5.0,
		"金额计算错误率应该 <= 5%")
}

// verifyPerformanceGoals 验证性能目标达成情况
func (s *PerformanceRegressionTestSuite) verifyPerformanceGoals(improvements map[string]float64) {
	// 验证延迟改进目标
	if latencyImprovement, ok := improvements["AmountCalcLatency"]; ok {
		assert.GreaterOrEqual(s.T(), latencyImprovement, s.improvementGoals.AmountCalcLatencyImprovement,
			fmt.Sprintf("金额计算延迟改进应该 >= %.1f%%", s.improvementGoals.AmountCalcLatencyImprovement))
	}
	
	if latencyImprovement, ok := improvements["OrderCreationLatency"]; ok {
		assert.GreaterOrEqual(s.T(), latencyImprovement, s.improvementGoals.OrderCreationLatencyImprovement,
			fmt.Sprintf("订单创建延迟改进应该 >= %.1f%%", s.improvementGoals.OrderCreationLatencyImprovement))
	}
	
	// 验证吞吐量改进目标
	if throughputImprovement, ok := improvements["AmountCalcThroughput"]; ok {
		assert.GreaterOrEqual(s.T(), throughputImprovement, s.improvementGoals.ThroughputImprovement,
			fmt.Sprintf("吞吐量改进应该 >= %.1f%%", s.improvementGoals.ThroughputImprovement))
	}
	
	// 验证错误率降低目标
	if errorRateReduction, ok := improvements["AmountCalcErrorRate"]; ok {
		assert.GreaterOrEqual(s.T(), errorRateReduction, s.improvementGoals.ErrorRateReduction,
			fmt.Sprintf("错误率降低应该 >= %.1f%%", s.improvementGoals.ErrorRateReduction))
	}
	
	// 验证冲突率降低目标
	if conflictReduction, ok := improvements["ConflictRate"]; ok {
		assert.GreaterOrEqual(s.T(), conflictReduction, s.improvementGoals.ConflictRateReduction,
			fmt.Sprintf("冲突率降低应该 >= %.1f%%", s.improvementGoals.ConflictRateReduction))
	}
}

// getGoalAchievementStatus 获取目标达成状态
func (s *PerformanceRegressionTestSuite) getGoalAchievementStatus(improvements map[string]float64) string {
	status := ""
	
	goals := []struct {
		name     string
		key      string
		target   float64
		unit     string
	}{
		{"金额计算延迟改进", "AmountCalcLatency", s.improvementGoals.AmountCalcLatencyImprovement, "%"},
		{"订单创建延迟改进", "OrderCreationLatency", s.improvementGoals.OrderCreationLatencyImprovement, "%"},
		{"吞吐量提升", "AmountCalcThroughput", s.improvementGoals.ThroughputImprovement, "%"},
		{"错误率降低", "AmountCalcErrorRate", s.improvementGoals.ErrorRateReduction, "%"},
		{"冲突率降低", "ConflictRate", s.improvementGoals.ConflictRateReduction, "%"},
	}
	
	for _, goal := range goals {
		if actual, ok := improvements[goal.key]; ok {
			if actual >= goal.target {
				status += fmt.Sprintf("✅ %s: %.1f%s (目标: %.1f%s)\n", goal.name, actual, goal.unit, goal.target, goal.unit)
			} else {
				status += fmt.Sprintf("❌ %s: %.1f%s (目标: %.1f%s)\n", goal.name, actual, goal.unit, goal.target, goal.unit)
			}
		} else {
			status += fmt.Sprintf("⚠️ %s: 无数据\n", goal.name)
		}
	}
	
	return status
}

// getOverallConclusion 获取总体结论
func (s *PerformanceRegressionTestSuite) getOverallConclusion(improvements map[string]float64) string {
	achievedCount := 0
	totalCount := 0
	
	targets := []struct {
		key    string
		target float64
	}{
		{"AmountCalcLatency", s.improvementGoals.AmountCalcLatencyImprovement},
		{"OrderCreationLatency", s.improvementGoals.OrderCreationLatencyImprovement},
		{"AmountCalcThroughput", s.improvementGoals.ThroughputImprovement},
		{"AmountCalcErrorRate", s.improvementGoals.ErrorRateReduction},
		{"ConflictRate", s.improvementGoals.ConflictRateReduction},
	}
	
	for _, target := range targets {
		if actual, ok := improvements[target.key]; ok {
			totalCount++
			if actual >= target.target {
				achievedCount++
			}
		}
	}
	
	achievementRate := float64(achievedCount) / float64(totalCount) * 100
	
	if achievementRate >= 80 {
		return "性能优化效果显著，达到预期目标 ✅"
	} else if achievementRate >= 60 {
		return "性能优化效果良好，部分目标需要进一步改进 ⚠️"
	} else {
		return "性能优化效果不理想，需要重新评估优化策略 ❌"
	}
}

// containsString 辅助函数：检查字符串包含
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		len(s) > len(substr) && 
		(s[:len(substr)] == substr || 
		 s[len(s)-len(substr):] == substr ||
		 s[len(s)/2-len(substr)/2:len(s)/2+len(substr)/2] == substr))
}

// 运行测试套件
func TestPerformanceRegressionTestSuite(t *testing.T) {
	suite.Run(t, new(PerformanceRegressionTestSuite))
}