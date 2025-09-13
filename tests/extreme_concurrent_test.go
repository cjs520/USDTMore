package tests

import (
	"USDTMore/app/model"
	"USDTMore/app/service"
	"USDTMore/tests/testutils"
	"context"
	"fmt"
	"math/rand"
	"runtime"
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

// ExtremeConcurrentTestSuite 极端并发场景测试套件
type ExtremeConcurrentTestSuite struct {
	suite.Suite
	container         testcontainers.Container
	db                *gorm.DB
	amountService     service.AmountService
	orderService      service.OrderService
	repository        service.OrderRepository
	testWallets       []model.WalletAddress
	extremeMetrics    *ExtremeLoadMetrics
	degradationRules  *DegradationRules
	circuitBreaker    *service.CircuitBreaker
}

// ExtremeLoadMetrics 极端负载测试指标
type ExtremeLoadMetrics struct {
	// 负载测试配置
	MaxConcurrency     int
	TestDuration       time.Duration
	TotalOperations    int64
	
	// 成功率指标
	SuccessfulOps      int64
	FailedOps          int64
	TimeoutOps         int64
	SuccessRate        float64
	
	// 性能指标
	ThroughputPeak     float64
	ThroughputAverage  float64
	ThroughputLow      float64
	LatencyDistribution map[string]time.Duration
	
	// 资源使用指标
	MaxMemoryUsage     int64
	MaxGoroutines      int64
	MaxDBConnections   int
	CPUUsagePeak       float64
	MemoryLeakDetected bool
	
	// 错误统计
	ErrorTypes         map[string]int64
	PanicCount         int64
	DeadlockCount      int64
	ConnectionErrors   int64
	
	// 系统稳定性
	SystemStable       bool
	RecoveryTime       time.Duration
	DegradationEvents  int64
	CircuitBreakerTrips int64
	
	// 数据一致性
	DataInconsistencies int64
	DuplicateAmounts    int64
	OrphanedRecords     int64
	
	TestStartTime      time.Time
	TestEndTime        time.Time
}

// DegradationRules 系统降级规则
type DegradationRules struct {
	// 触发条件
	MaxErrorRate       float64  // 最大错误率
	MaxLatency         time.Duration // 最大延迟
	MaxMemoryUsage     int64    // 最大内存使用
	MaxConnectionUsage float64  // 最大连接使用率
	
	// 降级策略
	EnableRateLimiting    bool
	ReduceConnectionPool  bool
	EnableCircuitBreaker  bool
	SkipNonCriticalOps   bool
	
	// 恢复条件
	RecoveryErrorRate     float64
	RecoveryLatency       time.Duration
	RecoveryDuration      time.Duration
}

// SetupSuite 设置测试套件
func (s *ExtremeConcurrentTestSuite) SetupSuite() {
	ctx := context.Background()
	
	// 设置测试数据库
	var err error
	s.container, s.db = testutils.SetupTestDB(ctx, s.T())
	require.NoError(s.T(), err)
	
	// 初始化服务层
	s.repository = service.NewOrderRepository(s.db)
	s.amountService = service.NewAmountService(s.repository)
	s.orderService = service.NewOrderService(s.repository, s.amountService)
	
	// 初始化断路器
	s.circuitBreaker = service.NewCircuitBreaker(service.CircuitBreakerConfig{
		MaxFailures:  10,
		ResetTimeout: 30 * time.Second,
		HalfOpenMax:  5,
	})
	
	// 设置系统降级规则
	s.degradationRules = &DegradationRules{
		MaxErrorRate:         50.0, // 50%错误率触发降级
		MaxLatency:           2 * time.Second,
		MaxMemoryUsage:       1024 * 1024 * 1024, // 1GB
		MaxConnectionUsage:   0.8, // 80%连接使用率
		EnableRateLimiting:   true,
		EnableCircuitBreaker: true,
		RecoveryErrorRate:    10.0, // 10%错误率以下开始恢复
		RecoveryLatency:      500 * time.Millisecond,
		RecoveryDuration:     10 * time.Second,
	}
	
	// 初始化极端测试指标
	s.extremeMetrics = &ExtremeLoadMetrics{
		ErrorTypes:          make(map[string]int64),
		LatencyDistribution: make(map[string]time.Duration),
		TestStartTime:       time.Now(),
	}
	
	s.T().Log("极端并发测试套件初始化完成")
}

// TearDownSuite 清理测试套件
func (s *ExtremeConcurrentTestSuite) TearDownSuite() {
	if s.container != nil {
		ctx := context.Background()
		testutils.TeardownTestDB(ctx, s.container)
	}
}

// SetupTest 每个测试前的设置
func (s *ExtremeConcurrentTestSuite) SetupTest() {
	// 清理数据库
	testutils.CleanDatabase(s.db)
	
	// 设置测试钱包地址
	s.setupTestWallets()
	
	// 重置指标
	s.resetExtremeMetrics()
}

// setupTestWallets 设置测试钱包地址
func (s *ExtremeConcurrentTestSuite) setupTestWallets() {
	s.testWallets = []model.WalletAddress{
		*testutils.CreateTestWalletAddress("TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"),
		*testutils.CreateTestWalletAddress("BSC", "0x55d398326f99059ff775485246999027b3197955"),
		*testutils.CreateTestWalletAddress("POLY", "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"),
		*testutils.CreateTestWalletAddress("OP", "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58"),
		*testutils.CreateTestWalletAddress("ETH", "0xA0b86a33E6417bd4E5fA7e3b2e0Ea5aB8f2B0C5d"),
		*testutils.CreateTestWalletAddress("ARB", "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9"),
	}
	
	for _, wallet := range s.testWallets {
		err := s.db.Create(&wallet).Error
		require.NoError(s.T(), err)
	}
}

// resetExtremeMetrics 重置极端测试指标
func (s *ExtremeConcurrentTestSuite) resetExtremeMetrics() {
	s.extremeMetrics = &ExtremeLoadMetrics{
		ErrorTypes:          make(map[string]int64),
		LatencyDistribution: make(map[string]time.Duration),
		TestStartTime:       time.Now(),
	}
}

// TestThousandConcurrentOrderCreation 测试1000+并发订单创建
func (s *ExtremeConcurrentTestSuite) TestThousandConcurrentOrderCreation() {
	s.T().Log("开始1000+并发订单创建压力测试...")
	
	concurrency := 1000
	operationsPerWorker := 2
	totalOperations := int64(concurrency * operationsPerWorker)
	
	s.extremeMetrics.MaxConcurrency = concurrency
	s.extremeMetrics.TotalOperations = totalOperations
	
	// 记录初始资源状态
	initialMemory := testutils.GetMemoryUsage()
	initialGoroutines := testutils.GetGoroutineCount()
	
	var wg sync.WaitGroup
	var successful int64
	var failed int64
	var timeouts int64
	
	// 用于监控吞吐量的通道
	throughputChan := make(chan time.Time, totalOperations)
	
	startTime := time.Now()
	
	// 启动资源监控
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	go s.monitorSystemResources(monitorCtx)
	
	s.T().Logf("启动 %d 个并发工作器，每个执行 %d 次操作...", concurrency, operationsPerWorker)
	
	// 并发创建订单
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < operationsPerWorker; j++ {
				opStart := time.Now()
				
				// 检查是否需要降级
				if s.shouldDegradeService() {
					atomic.AddInt64(&s.extremeMetrics.DegradationEvents, 1)
					time.Sleep(time.Millisecond * 10) // 模拟降级延迟
				}
				
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				
				success := s.executeOrderCreationWithCircuitBreaker(ctx, workerID, j)
				cancel()
				
				opEnd := time.Now()
				throughputChan <- opEnd
				
				if success {
					atomic.AddInt64(&successful, 1)
				} else {
					atomic.AddInt64(&failed, 1)
					if opEnd.Sub(opStart) > 25*time.Second {
						atomic.AddInt64(&timeouts, 1)
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	monitorCancel()
	close(throughputChan)
	
	totalDuration := time.Since(startTime)
	s.extremeMetrics.TestEndTime = time.Now()
	
	// 计算性能指标
	s.calculateExtremeConcurrencyMetrics(throughputChan, totalDuration, successful, failed, timeouts)
	
	// 检查系统稳定性
	s.checkSystemStability(initialMemory, initialGoroutines)
	
	// 验证数据一致性
	s.verifyDataConsistency()
	
	// 输出测试结果
	s.reportExtremeConcurrencyResults()
	
	// 验证极端并发目标
	s.verifyExtremeConcurrencyGoals()
}

// executeOrderCreationWithCircuitBreaker 使用断路器执行订单创建
func (s *ExtremeConcurrentTestSuite) executeOrderCreationWithCircuitBreaker(ctx context.Context, workerID, iteration int) bool {
	return s.circuitBreaker.Execute(func() error {
		// 先计算金额
		money := 100.0 + float64(iteration%100)
		address, amount, err := s.amountService.CalcTradeAmount(ctx, s.testWallets, 7.2, money)
		if err != nil {
			s.recordError("amount_calculation", err)
			return err
		}
		
		// 创建订单请求
		req := service.CreateOrderRequest{
			OrderID:   fmt.Sprintf("EXTREME_%d_%d_%d", workerID, iteration, time.Now().UnixNano()),
			TradeID:   fmt.Sprintf("EXTREME_%d_%d_%d", workerID, iteration, time.Now().UnixNano()),
			Chain:     address.Chain,
			Address:   address.Address,
			Amount:    amount,
			Money:     money,
			UsdtRate:  "7.2",
			ReturnURL: fmt.Sprintf("https://example.com/return/%d", workerID),
			NotifyURL: fmt.Sprintf("https://example.com/notify/%d", workerID),
			ExpiredAt: time.Now().Add(time.Hour),
		}
		
		// 创建订单
		_, err = s.orderService.CreateOrder(ctx, req)
		if err != nil {
			s.recordError("order_creation", err)
			return err
		}
		
		return nil
	}) == nil
}

// TestConnectionPoolExhaustion 测试连接池耗尽场景
func (s *ExtremeConcurrentTestSuite) TestConnectionPoolExhaustion() {
	s.T().Log("开始测试连接池耗尽场景...")
	
	// 获取当前连接池配置
	sqlDB, err := s.db.DB()
	require.NoError(s.T(), err)
	
	stats := sqlDB.Stats()
	maxConnections := stats.MaxOpenConnections
	
	s.T().Logf("当前连接池配置: MaxOpen=%d, Idle=%d", maxConnections, stats.Idle)
	
	// 启动超过连接池容量的并发数
	concurrency := maxConnections + 50
	var wg sync.WaitGroup
	var connectionErrors int64
	var successful int64
	var waitingForConnection int64
	
	startTime := time.Now()
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			
			// 执行长时间的数据库操作
			queryStart := time.Now()
			var orders []model.TradeOrders
			err := s.db.WithContext(ctx).Where("status = ?", model.OrderStatusWaiting).Find(&orders).Error
			queryDuration := time.Since(queryStart)
			
			if err != nil {
				if isConnectionError(err) {
					atomic.AddInt64(&connectionErrors, 1)
				}
				s.recordError("connection_pool", err)
			} else {
				atomic.AddInt64(&successful, 1)
				
				// 如果查询等待时间过长，认为是在等待连接
				if queryDuration > time.Second {
					atomic.AddInt64(&waitingForConnection, 1)
				}
			}
			
			// 模拟业务处理时间
			time.Sleep(time.Millisecond * time.Duration(50+rand.Intn(100)))
		}(i)
	}
	
	wg.Wait()
	
	totalDuration := time.Since(startTime)
	
	// 获取最终连接池状态
	finalStats := sqlDB.Stats()
	
	s.T().Logf("=== 连接池耗尽测试结果 ===")
	s.T().Logf("并发数: %d (连接池容量: %d)", concurrency, maxConnections)
	s.T().Logf("成功操作: %d", successful)
	s.T().Logf("连接错误: %d", connectionErrors)
	s.T().Logf("等待连接: %d", waitingForConnection)
	s.T().Logf("总耗时: %v", totalDuration)
	s.T().Logf("最终连接池状态: OpenConnections=%d, InUse=%d, Idle=%d", 
		finalStats.OpenConnections, finalStats.InUse, finalStats.Idle)
	s.T().Logf("连接等待统计: WaitCount=%d, WaitDuration=%v", 
		finalStats.WaitCount, finalStats.WaitDuration)
	
	// 验证系统在连接池压力下的表现
	successRate := float64(successful) / float64(concurrency) * 100
	assert.GreaterOrEqual(s.T(), successRate, 80.0, "即使在连接池压力下，成功率应该 >= 80%")
	
	// 验证连接池没有泄漏
	assert.LessOrEqual(s.T(), finalStats.OpenConnections, maxConnections+5, 
		"连接数不应该远超过配置的最大值")
}

// TestLongTermHighLoad 测试长时间高负载稳定性
func (s *ExtremeConcurrentTestSuite) TestLongTermHighLoad() {
	s.T().Log("开始长时间高负载稳定性测试...")
	
	testDuration := 2 * time.Minute // 2分钟持续负载
	concurrency := 200
	operationInterval := 100 * time.Millisecond
	
	s.extremeMetrics.TestDuration = testDuration
	
	// 记录初始状态
	initialMemory := testutils.GetMemoryUsage()
	initialGoroutines := testutils.GetGoroutineCount()
	
	var wg sync.WaitGroup
	var totalOps int64
	var successfulOps int64
	var errorOps int64
	stopChan := make(chan struct{})
	
	// 启动系统监控
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	go s.monitorSystemResources(monitorCtx)
	go s.monitorPerformanceMetrics(monitorCtx, &totalOps, &successfulOps, &errorOps)
	
	startTime := time.Now()
	s.T().Logf("启动长期负载测试，持续时间: %v，并发数: %d", testDuration, concurrency)
	
	// 启动工作协程
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			operationCount := 0
			for {
				select {
				case <-stopChan:
					return
				default:
					atomic.AddInt64(&totalOps, 1)
					operationCount++
					
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					
					success := s.executeMixedOperations(ctx, workerID, operationCount)
					if success {
						atomic.AddInt64(&successfulOps, 1)
					} else {
						atomic.AddInt64(&errorOps, 1)
					}
					
					cancel()
					time.Sleep(operationInterval)
				}
			}
		}(i)
	}
	
	// 运行指定时间
	time.Sleep(testDuration)
	close(stopChan)
	wg.Wait()
	monitorCancel()
	
	actualDuration := time.Since(startTime)
	
	// 强制垃圾回收后检查资源
	testutils.ForceGC()
	time.Sleep(time.Second)
	
	finalMemory := testutils.GetMemoryUsage()
	finalGoroutines := testutils.GetGoroutineCount()
	
	// 计算稳定性指标
	successRate := float64(successfulOps) / float64(totalOps) * 100
	throughput := float64(successfulOps) / actualDuration.Seconds()
	memoryIncrease := finalMemory - initialMemory
	goroutineIncrease := finalGoroutines - initialGoroutines
	
	s.T().Logf("=== 长期负载测试结果 ===")
	s.T().Logf("实际运行时间: %v", actualDuration)
	s.T().Logf("总操作数: %d", totalOps)
	s.T().Logf("成功操作: %d", successfulOps)
	s.T().Logf("失败操作: %d", errorOps)
	s.T().Logf("成功率: %.2f%%", successRate)
	s.T().Logf("平均吞吐量: %.2f ops/s", throughput)
	s.T().Logf("内存变化: %d KB", memoryIncrease/1024)
	s.T().Logf("Goroutine变化: %d", goroutineIncrease)
	
	// 更新指标
	s.extremeMetrics.SuccessRate = successRate
	s.extremeMetrics.ThroughputAverage = throughput
	s.extremeMetrics.MaxMemoryUsage = finalMemory
	s.extremeMetrics.MaxGoroutines = finalGoroutines
	
	// 验证长期稳定性
	assert.GreaterOrEqual(s.T(), successRate, 90.0, "长期负载下成功率应该 >= 90%")
	assert.GreaterOrEqual(s.T(), throughput, 50.0, "长期负载下吞吐量应该 >= 50 ops/s")
	
	memoryIncreasePercent := float64(memoryIncrease) / float64(initialMemory) * 100
	assert.LessOrEqual(s.T(), memoryIncreasePercent, 200.0, "长期运行内存增长应该 <= 200%")
	
	assert.LessOrEqual(s.T(), goroutineIncrease, int64(concurrency), "Goroutine增长应该在合理范围内")
	
	// 标记系统稳定性
	s.extremeMetrics.SystemStable = successRate >= 90.0 && memoryIncreasePercent <= 200.0
}

// TestMemoryLeakUnderExtremeLoad 测试极端负载下的内存泄漏
func (s *ExtremeConcurrentTestSuite) TestMemoryLeakUnderExtremeLoad() {
	s.T().Log("开始极端负载下内存泄漏测试...")
	
	iterations := 50
	operationsPerIteration := 200
	concurrencyPerIteration := 100
	
	memorySnapshots := make([]int64, 0, iterations)
	
	for iteration := 0; iteration < iterations; iteration++ {
		// 记录每次迭代前的内存使用
		testutils.ForceGC()
		time.Sleep(100 * time.Millisecond)
		currentMemory := testutils.GetMemoryUsage()
		memorySnapshots = append(memorySnapshots, currentMemory)
		
		var wg sync.WaitGroup
		
		// 执行高强度操作
		for i := 0; i < concurrencyPerIteration; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < operationsPerIteration; j++ {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					
					// 执行各种操作以测试内存泄漏
					switch j % 5 {
					case 0:
						// 金额计算
						s.amountService.CalcTradeAmount(ctx, s.testWallets, 7.2, 100.0+float64(j))
					case 1:
						// 订单查询
						s.repository.GetOrdersByStatus(ctx, model.OrderStatusWaiting)
					case 2:
						// 创建临时订单
						req := service.CreateOrderRequest{
							OrderID:   fmt.Sprintf("LEAK_%d_%d_%d", iteration, workerID, j),
							TradeID:   fmt.Sprintf("LEAK_%d_%d_%d", iteration, workerID, j),
							Chain:     "TRON",
							Address:   "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
							Amount:    fmt.Sprintf("%.2f", 100.0+float64(j)),
							Money:     720.0 + float64(j),
							UsdtRate:  "7.2",
							ExpiredAt: time.Now().Add(time.Hour),
						}
						s.orderService.CreateOrder(ctx, req)
					case 3:
						// 检查冲突
						s.amountService.CheckAmountConflict(ctx, "TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", "100.00")
					case 4:
						// 数据库事务
						s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
							var count int64
							return tx.Model(&model.TradeOrders{}).Count(&count).Error
						})
					}
					
					cancel()
				}
			}(i)
		}
		
		wg.Wait()
		
		// 清理测试数据
		s.db.Where("order_id LIKE ?", "LEAK_%").Delete(&model.TradeOrders{})
		
		// 每10次迭代报告进度
		if iteration%10 == 0 && iteration > 0 {
			s.T().Logf("内存泄漏测试进度: %d/%d, 当前内存: %d KB", 
				iteration, iterations, currentMemory/1024)
		}
	}
	
	// 强制垃圾回收
	testutils.ForceGC()
	time.Sleep(2 * time.Second)
	finalMemory := testutils.GetMemoryUsage()
	memorySnapshots = append(memorySnapshots, finalMemory)
	
	// 分析内存趋势
	initialMemory := memorySnapshots[0]
	memoryIncrease := finalMemory - initialMemory
	memoryIncreasePercent := float64(memoryIncrease) / float64(initialMemory) * 100
	
	// 计算内存增长趋势
	memoryTrend := s.calculateMemoryTrend(memorySnapshots)
	
	s.T().Logf("=== 内存泄漏测试结果 ===")
	s.T().Logf("测试迭代: %d", iterations)
	s.T().Logf("每次操作数: %d", operationsPerIteration*concurrencyPerIteration)
	s.T().Logf("初始内存: %d KB", initialMemory/1024)
	s.T().Logf("最终内存: %d KB", finalMemory/1024)
	s.T().Logf("内存增长: %d KB (%.2f%%)", memoryIncrease/1024, memoryIncreasePercent)
	s.T().Logf("内存趋势: %.2f KB/iteration", memoryTrend/1024)
	
	// 判断是否存在内存泄漏
	leakDetected := memoryIncreasePercent > 100.0 || memoryTrend > 1024*1024 // 每次迭代增长超过1MB
	s.extremeMetrics.MemoryLeakDetected = leakDetected
	
	if leakDetected {
		s.T().Errorf("检测到内存泄漏：增长 %.2f%%，趋势 %.2f KB/iter", 
			memoryIncreasePercent, memoryTrend/1024)
	}
	
	assert.False(s.T(), leakDetected, "极端负载下不应该出现明显的内存泄漏")
}

// TestSystemDegradationAndRecovery 测试系统降级和恢复
func (s *ExtremeConcurrentTestSuite) TestSystemDegradationAndRecovery() {
	s.T().Log("开始测试系统降级和恢复机制...")
	
	phases := []struct {
		name        string
		concurrency int
		duration    time.Duration
		errorRate   float64
	}{
		{"正常负载", 50, 30 * time.Second, 0.0},
		{"高负载触发降级", 300, 45 * time.Second, 0.3},
		{"降级运行", 100, 30 * time.Second, 0.1},
		{"恢复正常", 50, 30 * time.Second, 0.0},
	}
	
	var degradationTriggered bool
	var recoveryTime time.Time
	
	for phaseIdx, phase := range phases {
		s.T().Logf("执行阶段 %d: %s (并发: %d, 持续: %v)", 
			phaseIdx+1, phase.name, phase.concurrency, phase.duration)
		
		phaseStart := time.Now()
		var wg sync.WaitGroup
		var successful int64
		var failed int64
		var degradationEvents int64
		
		stopChan := make(chan struct{})
		
		// 启动工作负载
		for i := 0; i < phase.concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				
				opCount := 0
				for {
					select {
					case <-stopChan:
						return
					default:
						opCount++
						
						// 模拟错误率
						if rand.Float64() < phase.errorRate {
							atomic.AddInt64(&failed, 1)
							s.recordError("simulated_error", fmt.Errorf("simulated error"))
							time.Sleep(time.Millisecond * 10)
							continue
						}
						
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						
						// 检查是否需要降级
						if s.shouldDegradeService() {
							atomic.AddInt64(&degradationEvents, 1)
							if !degradationTriggered {
								degradationTriggered = true
								s.T().Logf("系统降级已触发 (阶段: %s)", phase.name)
							}
							// 降级处理：简化操作
							time.Sleep(time.Millisecond * 5)
							atomic.AddInt64(&successful, 1)
						} else {
							// 正常操作
							success := s.executeMixedOperations(ctx, workerID, opCount)
							if success {
								atomic.AddInt64(&successful, 1)
							} else {
								atomic.AddInt64(&failed, 1)
							}
							
							// 检查是否从降级状态恢复
							if degradationTriggered && atomic.LoadInt64(&degradationEvents) == 0 && recoveryTime.IsZero() {
								recoveryTime = time.Now()
								s.T().Logf("系统开始恢复 (阶段: %s)", phase.name)
							}
						}
						
						cancel()
						time.Sleep(time.Millisecond * 20)
					}
				}
			}(i)
		}
		
		// 运行指定时间
		time.Sleep(phase.duration)
		close(stopChan)
		wg.Wait()
		
		_ = time.Since(phaseStart) // phaseDuration - could be used for more detailed logging
		successRate := float64(successful) / float64(successful+failed) * 100
		
		s.T().Logf("阶段 %s 结果: 成功=%d, 失败=%d, 成功率=%.2f%%, 降级事件=%d", 
			phase.name, successful, failed, successRate, degradationEvents)
		
		// 更新指标
		atomic.AddInt64(&s.extremeMetrics.DegradationEvents, degradationEvents)
		
		// 阶段间休息
		if phaseIdx < len(phases)-1 {
			time.Sleep(5 * time.Second)
		}
	}
	
	// 计算恢复时间
	if !recoveryTime.IsZero() {
		s.extremeMetrics.RecoveryTime = time.Since(recoveryTime)
		s.T().Logf("系统恢复时间: %v", s.extremeMetrics.RecoveryTime)
	}
	
	// 验证降级和恢复机制
	assert.True(s.T(), degradationTriggered, "高负载下应该触发降级机制")
	assert.Greater(s.T(), s.extremeMetrics.DegradationEvents, int64(0), "应该记录到降级事件")
	
	if !recoveryTime.IsZero() {
		assert.LessOrEqual(s.T(), s.extremeMetrics.RecoveryTime, 2*time.Minute, 
			"系统恢复时间应该在合理范围内")
	}
}

// monitorSystemResources 监控系统资源
func (s *ExtremeConcurrentTestSuite) monitorSystemResources(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 监控内存使用
			currentMemory := testutils.GetMemoryUsage()
			if currentMemory > s.extremeMetrics.MaxMemoryUsage {
				s.extremeMetrics.MaxMemoryUsage = currentMemory
			}
			
			// 监控Goroutine数量
			currentGoroutines := testutils.GetGoroutineCount()
			if currentGoroutines > s.extremeMetrics.MaxGoroutines {
				s.extremeMetrics.MaxGoroutines = currentGoroutines
			}
			
			// 监控数据库连接
			if sqlDB, err := s.db.DB(); err == nil {
				stats := sqlDB.Stats()
				if stats.OpenConnections > s.extremeMetrics.MaxDBConnections {
					s.extremeMetrics.MaxDBConnections = stats.OpenConnections
				}
			}
			
			// 检查是否需要降级
			if s.shouldDegradeService() {
				atomic.AddInt64(&s.extremeMetrics.DegradationEvents, 1)
			}
		}
	}
}

// monitorPerformanceMetrics 监控性能指标
func (s *ExtremeConcurrentTestSuite) monitorPerformanceMetrics(ctx context.Context, totalOps, successfulOps, errorOps *int64) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	var lastSuccessful int64
	_ = lastSuccessful // Will be used in ticker
	var lastTime time.Time = time.Now()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			currentSuccessful := atomic.LoadInt64(successfulOps)
			_ = atomic.LoadInt64(totalOps) // currentTotal - could be used for more metrics
			currentTime := time.Now()
			
			if !lastTime.IsZero() {
				timeDiff := currentTime.Sub(lastTime).Seconds()
				opsDiff := currentSuccessful - lastSuccessful
				throughput := float64(opsDiff) / timeDiff
				
				// 更新吞吐量指标
				if throughput > s.extremeMetrics.ThroughputPeak {
					s.extremeMetrics.ThroughputPeak = throughput
				}
				
				if s.extremeMetrics.ThroughputLow == 0 || throughput < s.extremeMetrics.ThroughputLow {
					s.extremeMetrics.ThroughputLow = throughput
				}
				
				s.T().Logf("当前吞吐量: %.2f ops/s, 累计成功: %d, 累计失败: %d", 
					throughput, currentSuccessful, atomic.LoadInt64(errorOps))
			}
			
			lastSuccessful = currentSuccessful
			// lastTotal = currentTotal // Not used currently
			lastTime = currentTime
		}
	}
}

// shouldDegradeService 判断是否应该降级服务
func (s *ExtremeConcurrentTestSuite) shouldDegradeService() bool {
	// 检查内存使用
	currentMemory := testutils.GetMemoryUsage()
	if currentMemory > s.degradationRules.MaxMemoryUsage {
		return true
	}
	
	// 检查连接池使用率
	if sqlDB, err := s.db.DB(); err == nil {
		stats := sqlDB.Stats()
		if stats.MaxOpenConnections > 0 {
			usageRate := float64(stats.InUse) / float64(stats.MaxOpenConnections)
			if usageRate > s.degradationRules.MaxConnectionUsage {
				return true
			}
		}
	}
	
	return false
}

// executeMixedOperations 执行混合操作
func (s *ExtremeConcurrentTestSuite) executeMixedOperations(ctx context.Context, workerID, opCount int) bool {
	switch opCount % 4 {
	case 0:
		// 金额计算
		_, _, err := s.amountService.CalcTradeAmount(ctx, s.testWallets, 7.2, 100.0+float64(opCount%50))
		return err == nil
	case 1:
		// 订单查询
		_, err := s.repository.GetOrdersByStatus(ctx, model.OrderStatusWaiting)
		return err == nil
	case 2:
		// 创建订单
		req := service.CreateOrderRequest{
			OrderID:   fmt.Sprintf("MIXED_%d_%d_%d", workerID, opCount, time.Now().UnixNano()),
			TradeID:   fmt.Sprintf("MIXED_%d_%d_%d", workerID, opCount, time.Now().UnixNano()),
			Chain:     s.testWallets[opCount%len(s.testWallets)].Chain,
			Address:   s.testWallets[opCount%len(s.testWallets)].Address,
			Amount:    fmt.Sprintf("%.2f", 100.0+float64(opCount%50)),
			Money:     100.0 + float64(opCount%50),
			UsdtRate:  "7.2",
			ExpiredAt: time.Now().Add(time.Hour),
		}
		_, err := s.orderService.CreateOrder(ctx, req)
		return err == nil
	case 3:
		// 检查冲突
		err := s.amountService.CheckAmountConflict(ctx, "TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", "100.00")
		return err == nil
	}
	return false
}

// recordError 记录错误
func (s *ExtremeConcurrentTestSuite) recordError(errorType string, err error) {
	if err == nil {
		return
	}
	
	// Safe map update for error types (this should be protected by mutex in production)
	if s.extremeMetrics.ErrorTypes == nil {
		s.extremeMetrics.ErrorTypes = make(map[string]int64)
	}
	s.extremeMetrics.ErrorTypes[errorType]++
	
	errStr := err.Error()
	if isConnectionError(err) {
		atomic.AddInt64(&s.extremeMetrics.ConnectionErrors, 1)
	}
	if isDeadlock(errStr) {
		atomic.AddInt64(&s.extremeMetrics.DeadlockCount, 1)
	}
}

// calculateExtremeConcurrencyMetrics 计算极端并发指标
func (s *ExtremeConcurrentTestSuite) calculateExtremeConcurrencyMetrics(throughputChan <-chan time.Time, totalDuration time.Duration, successful, failed, timeouts int64) {
	s.extremeMetrics.SuccessfulOps = successful
	s.extremeMetrics.FailedOps = failed
	s.extremeMetrics.TimeoutOps = timeouts
	s.extremeMetrics.SuccessRate = float64(successful) / float64(successful+failed) * 100
	s.extremeMetrics.ThroughputAverage = float64(successful) / totalDuration.Seconds()
	
	// 计算吞吐量峰值
	var throughputSamples []float64
	var lastTime time.Time
	var operationsInWindow int
	
	for timestamp := range throughputChan {
		if lastTime.IsZero() {
			lastTime = timestamp
			operationsInWindow = 1
			continue
		}
		
		timeDiff := timestamp.Sub(lastTime)
		if timeDiff >= time.Second {
			throughput := float64(operationsInWindow) / timeDiff.Seconds()
			throughputSamples = append(throughputSamples, throughput)
			
			if throughput > s.extremeMetrics.ThroughputPeak {
				s.extremeMetrics.ThroughputPeak = throughput
			}
			
			lastTime = timestamp
			operationsInWindow = 1
		} else {
			operationsInWindow++
		}
	}
	
	// 计算吞吐量最低值
	if len(throughputSamples) > 0 {
		s.extremeMetrics.ThroughputLow = throughputSamples[0]
		for _, tp := range throughputSamples {
			if tp < s.extremeMetrics.ThroughputLow {
				s.extremeMetrics.ThroughputLow = tp
			}
		}
	}
}

// checkSystemStability 检查系统稳定性
func (s *ExtremeConcurrentTestSuite) checkSystemStability(initialMemory, initialGoroutines int64) {
	finalMemory := testutils.GetMemoryUsage()
	finalGoroutines := testutils.GetGoroutineCount()
	
	memoryIncrease := finalMemory - initialMemory
	goroutineIncrease := finalGoroutines - initialGoroutines
	
	// 系统稳定性判断标准
	memoryStable := float64(memoryIncrease)/float64(initialMemory) < 3.0 // 内存增长不超过300%
	goroutineStable := goroutineIncrease < int64(s.extremeMetrics.MaxConcurrency*2) // Goroutine增长合理
	errorRateStable := s.extremeMetrics.SuccessRate > 85.0 // 成功率超过85%
	
	s.extremeMetrics.SystemStable = memoryStable && goroutineStable && errorRateStable
	
	s.T().Logf("系统稳定性检查:")
	s.T().Logf("  内存稳定: %v (增长: %d KB)", memoryStable, memoryIncrease/1024)
	s.T().Logf("  Goroutine稳定: %v (增长: %d)", goroutineStable, goroutineIncrease)
	s.T().Logf("  错误率稳定: %v (成功率: %.2f%%)", errorRateStable, s.extremeMetrics.SuccessRate)
	s.T().Logf("  总体稳定: %v", s.extremeMetrics.SystemStable)
}

// verifyDataConsistency 验证数据一致性
func (s *ExtremeConcurrentTestSuite) verifyDataConsistency() {
	s.T().Log("验证数据一致性...")
	
	// 检查重复金额
	var duplicateAmounts int64
	err := s.db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT chain, address, amount, COUNT(*) as cnt
			FROM trade_orders 
			WHERE status = ? 
			GROUP BY chain, address, amount 
			HAVING COUNT(*) > 1
		) duplicates
	`, model.OrderStatusWaiting).Scan(&duplicateAmounts).Error
	
	if err != nil {
		s.T().Logf("检查重复金额失败: %v", err)
	} else {
		s.extremeMetrics.DuplicateAmounts = duplicateAmounts
	}
	
	// 检查孤立记录
	var orphanedRecords int64
	err = s.db.Raw(`
		SELECT COUNT(*) FROM trade_orders 
		WHERE chain NOT IN (SELECT DISTINCT chain FROM wallet_address)
	`).Scan(&orphanedRecords).Error
	
	if err != nil {
		s.T().Logf("检查孤立记录失败: %v", err)
	} else {
		s.extremeMetrics.OrphanedRecords = orphanedRecords
	}
	
	// 计算数据不一致总数
	s.extremeMetrics.DataInconsistencies = duplicateAmounts + orphanedRecords
	
	s.T().Logf("数据一致性检查结果:")
	s.T().Logf("  重复金额: %d", duplicateAmounts)
	s.T().Logf("  孤立记录: %d", orphanedRecords)
	s.T().Logf("  总不一致: %d", s.extremeMetrics.DataInconsistencies)
}

// calculateMemoryTrend 计算内存增长趋势
func (s *ExtremeConcurrentTestSuite) calculateMemoryTrend(snapshots []int64) float64 {
	if len(snapshots) < 2 {
		return 0
	}
	
	// 简单线性回归计算趋势
	n := float64(len(snapshots))
	var sumX, sumY, sumXY, sumX2 float64
	
	for i, memory := range snapshots {
		x := float64(i)
		y := float64(memory)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	
	// 计算斜率 (slope)
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	return slope
}

// reportExtremeConcurrencyResults 报告极端并发测试结果
func (s *ExtremeConcurrentTestSuite) reportExtremeConcurrencyResults() {
	s.T().Logf("=== 极端并发测试结果 ===")
	s.T().Logf("最大并发数: %d", s.extremeMetrics.MaxConcurrency)
	s.T().Logf("总操作数: %d", s.extremeMetrics.TotalOperations)
	s.T().Logf("成功操作: %d", s.extremeMetrics.SuccessfulOps)
	s.T().Logf("失败操作: %d", s.extremeMetrics.FailedOps)
	s.T().Logf("超时操作: %d", s.extremeMetrics.TimeoutOps)
	s.T().Logf("成功率: %.2f%%", s.extremeMetrics.SuccessRate)
	s.T().Logf("峰值吞吐量: %.2f ops/s", s.extremeMetrics.ThroughputPeak)
	s.T().Logf("平均吞吐量: %.2f ops/s", s.extremeMetrics.ThroughputAverage)
	s.T().Logf("最低吞吐量: %.2f ops/s", s.extremeMetrics.ThroughputLow)
	s.T().Logf("最大内存: %d KB", s.extremeMetrics.MaxMemoryUsage/1024)
	s.T().Logf("最大Goroutine: %d", s.extremeMetrics.MaxGoroutines)
	s.T().Logf("最大DB连接: %d", s.extremeMetrics.MaxDBConnections)
	s.T().Logf("系统稳定: %v", s.extremeMetrics.SystemStable)
	s.T().Logf("降级事件: %d", s.extremeMetrics.DegradationEvents)
	s.T().Logf("数据不一致: %d", s.extremeMetrics.DataInconsistencies)
	
	s.T().Logf("错误统计:")
	for errorType, count := range s.extremeMetrics.ErrorTypes {
		s.T().Logf("  %s: %d", errorType, count)
	}
}

// verifyExtremeConcurrencyGoals 验证极端并发目标
func (s *ExtremeConcurrentTestSuite) verifyExtremeConcurrencyGoals() {
	// 验证系统在极端并发下的表现目标
	assert.GreaterOrEqual(s.T(), s.extremeMetrics.SuccessRate, 80.0, 
		"极端并发下成功率应该 >= 80%")
	
	assert.GreaterOrEqual(s.T(), s.extremeMetrics.ThroughputAverage, 100.0,
		"极端并发下平均吞吐量应该 >= 100 ops/s")
	
	assert.True(s.T(), s.extremeMetrics.SystemStable,
		"极端并发下系统应该保持稳定")
	
	assert.LessOrEqual(s.T(), s.extremeMetrics.DataInconsistencies, int64(10),
		"数据不一致应该在可接受范围内")
	
	assert.Equal(s.T(), int64(0), s.extremeMetrics.DeadlockCount,
		"不应该出现死锁")
	
	// 验证资源使用合理性
	assert.LessOrEqual(s.T(), s.extremeMetrics.MaxGoroutines, int64(s.extremeMetrics.MaxConcurrency*3),
		"Goroutine数量应该在合理范围内")
}

// TestGenerateExtremeLoadReport 生成极端负载测试报告
func (s *ExtremeConcurrentTestSuite) TestGenerateExtremeLoadReport() {
	s.T().Log("生成极端负载测试综合报告...")
	
	report := fmt.Sprintf(`
====== 极端并发场景测试报告 ======

测试时间: %s
测试环境: PostgreSQL + 极端负载场景

=== 极端并发测试结果 ===
🚀 并发能力:
• 最大并发数: %d
• 总操作数: %d
• 成功操作: %d
• 失败操作: %d
• 超时操作: %d
• 成功率: %.2f%%

⚡ 性能表现:
• 峰值吞吐量: %.2f ops/s
• 平均吞吐量: %.2f ops/s
• 最低吞吐量: %.2f ops/s
• 性能波动范围: %.2f - %.2f ops/s

🖥️ 资源使用:
• 最大内存使用: %d KB
• 最大Goroutine数: %d
• 最大数据库连接: %d
• 连接错误次数: %d

🔧 系统稳定性:
• 系统稳定状态: %s
• 降级事件次数: %d
• 恢复时间: %v
• 断路器触发: %d 次
• 死锁检测: %d 次

🔍 数据一致性:
• 数据不一致总数: %d
• 重复金额检测: %d
• 孤立记录检测: %d
• 内存泄漏检测: %s

📊 错误分析:
%s

=== 极端场景表现评估 ===
1. 高并发处理能力: %s
2. 系统稳定性: %s  
3. 资源使用效率: %s
4. 错误处理机制: %s
5. 数据一致性保证: %s

=== 极端场景改进建议 ===
• 优化连接池配置以支持更高并发
• 完善系统监控和告警机制
• 加强降级策略的精细化控制
• 持续优化内存使用和垃圾回收
• 建立完善的容量规划机制

测试结论: %s
`,
		time.Now().Format("2006-01-02 15:04:05"),
		
		// 并发测试结果
		s.extremeMetrics.MaxConcurrency,
		s.extremeMetrics.TotalOperations,
		s.extremeMetrics.SuccessfulOps,
		s.extremeMetrics.FailedOps,
		s.extremeMetrics.TimeoutOps,
		s.extremeMetrics.SuccessRate,
		
		// 性能表现
		s.extremeMetrics.ThroughputPeak,
		s.extremeMetrics.ThroughputAverage,
		s.extremeMetrics.ThroughputLow,
		s.extremeMetrics.ThroughputLow,
		s.extremeMetrics.ThroughputPeak,
		
		// 资源使用
		s.extremeMetrics.MaxMemoryUsage/1024,
		s.extremeMetrics.MaxGoroutines,
		s.extremeMetrics.MaxDBConnections,
		s.extremeMetrics.ConnectionErrors,
		
		// 系统稳定性
		s.getStabilityStatus(),
		s.extremeMetrics.DegradationEvents,
		s.extremeMetrics.RecoveryTime,
		s.extremeMetrics.CircuitBreakerTrips,
		s.extremeMetrics.DeadlockCount,
		
		// 数据一致性
		s.extremeMetrics.DataInconsistencies,
		s.extremeMetrics.DuplicateAmounts,
		s.extremeMetrics.OrphanedRecords,
		s.getMemoryLeakStatus(),
		
		// 错误分析
		s.getErrorAnalysis(),
		
		// 表现评估
		s.getPerformanceAssessment("concurrency"),
		s.getPerformanceAssessment("stability"),
		s.getPerformanceAssessment("resource"),
		s.getPerformanceAssessment("error"),
		s.getPerformanceAssessment("consistency"),
		
		// 总体结论
		s.getExtremeLoadConclusion(),
	)
	
	s.T().Log(report)
	
	// 保存报告
	reportFile := fmt.Sprintf("/Users/jay/code/Usdt/tests/extreme_concurrent_report_%s.md", 
		time.Now().Format("20060102_150405"))
	
	err := testutils.SaveTestReport(reportFile, report)
	if err != nil {
		s.T().Logf("保存报告失败: %v", err)
	} else {
		s.T().Logf("极端并发测试报告已保存至: %s", reportFile)
	}
}

// 辅助函数
func (s *ExtremeConcurrentTestSuite) getStabilityStatus() string {
	if s.extremeMetrics.SystemStable {
		return "✅ 稳定"
	}
	return "⚠️ 不稳定"
}

func (s *ExtremeConcurrentTestSuite) getMemoryLeakStatus() string {
	if s.extremeMetrics.MemoryLeakDetected {
		return "⚠️ 检测到泄漏"
	}
	return "✅ 无泄漏"
}

func (s *ExtremeConcurrentTestSuite) getErrorAnalysis() string {
	analysis := ""
	for errorType, count := range s.extremeMetrics.ErrorTypes {
		analysis += fmt.Sprintf("• %s: %d 次\n", errorType, count)
	}
	if analysis == "" {
		analysis = "• 无显著错误类型"
	}
	return analysis
}

func (s *ExtremeConcurrentTestSuite) getPerformanceAssessment(aspect string) string {
	switch aspect {
	case "concurrency":
		if s.extremeMetrics.SuccessRate >= 80.0 && s.extremeMetrics.ThroughputAverage >= 100.0 {
			return "✅ 优秀"
		} else if s.extremeMetrics.SuccessRate >= 70.0 {
			return "⚠️ 良好"
		}
		return "❌ 需改进"
	case "stability":
		if s.extremeMetrics.SystemStable && s.extremeMetrics.DegradationEvents < 10 {
			return "✅ 优秀"
		}
		return "⚠️ 需关注"
	case "resource":
		if s.extremeMetrics.MaxGoroutines < int64(s.extremeMetrics.MaxConcurrency*2) {
			return "✅ 高效"
		}
		return "⚠️ 可优化"
	case "error":
		if s.extremeMetrics.DeadlockCount == 0 && s.extremeMetrics.ConnectionErrors < 100 {
			return "✅ 良好"
		}
		return "⚠️ 需改进"
	case "consistency":
		if s.extremeMetrics.DataInconsistencies <= 10 {
			return "✅ 优秀"
		}
		return "⚠️ 需关注"
	}
	return "未评估"
}

func (s *ExtremeConcurrentTestSuite) getExtremeLoadConclusion() string {
	score := 0
	if s.extremeMetrics.SuccessRate >= 80.0 {
		score++
	}
	if s.extremeMetrics.SystemStable {
		score++
	}
	if s.extremeMetrics.DeadlockCount == 0 {
		score++
	}
	if s.extremeMetrics.DataInconsistencies <= 10 {
		score++
	}
	if s.extremeMetrics.ThroughputAverage >= 100.0 {
		score++
	}
	
	if score >= 4 {
		return "系统在极端并发场景下表现优秀，满足高负载需求 ✅"
	} else if score >= 3 {
		return "系统在极端并发场景下表现良好，部分方面需要优化 ⚠️"
	} else {
		return "系统在极端并发场景下表现不佳，需要重点改进 ❌"
	}
}

// 辅助函数
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	patterns := []string{"connection", "pool", "timeout", "refused", "reset"}
	for _, pattern := range patterns {
		if containsErrorString(errStr, pattern) {
			return true
		}
	}
	return false
}

func isDeadlock(errStr string) bool {
	return containsErrorString(errStr, "deadlock")
}

func containsErrorString(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// 运行测试套件
func TestExtremeConcurrentTestSuite(t *testing.T) {
	// 调整运行时设置以支持极端并发测试
	runtime.GOMAXPROCS(runtime.NumCPU())
	
	suite.Run(t, new(ExtremeConcurrentTestSuite))
}