package tests

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// TestCalcTradeAmountPerformance 测试金额计算性能和唯一性
func TestCalcTradeAmountPerformance(t *testing.T) {
	tests := []struct {
		name           string
		baseAmount     float64
		concurrency    int
		iterations     int
		expectedUnique float64 // 期望的唯一率
	}{
		{
			name:           "低并发场景",
			baseAmount:     100.00,
			concurrency:    10,
			iterations:     100,
			expectedUnique: 98.0,
		},
		{
			name:           "中等并发场景",
			baseAmount:     500.00,
			concurrency:    50,
			iterations:     200,
			expectedUnique: 97.0,
		},
		{
			name:           "高并发场景",
			baseAmount:     1000.00,
			concurrency:    100,
			iterations:     500,
			expectedUnique: 96.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startTime := time.Now()
			results := make(map[string]int)
			resultsMu := sync.Mutex{}
			
			var wg sync.WaitGroup
			wg.Add(tt.concurrency)
			
			for i := 0; i < tt.concurrency; i++ {
				go func(workerID int) {
					defer wg.Done()
					
					for j := 0; j < tt.iterations; j++ {
						// 使用改进的金额计算算法
						amount := calcTradeAmountImproved(tt.baseAmount)
						
						resultsMu.Lock()
						results[amount]++
						resultsMu.Unlock()
					}
				}(i)
			}
			
			wg.Wait()
			elapsed := time.Since(startTime)
			
			// 计算唯一性率
			totalGenerated := tt.concurrency * tt.iterations
			uniqueCount := len(results)
			uniquenessRate := float64(uniqueCount) * 100 / float64(totalGenerated)
			
			// 输出测试结果
			t.Logf("测试场景: %s", tt.name)
			t.Logf("并发数: %d, 迭代次数: %d", tt.concurrency, tt.iterations)
			t.Logf("总生成数: %d, 唯一数: %d", totalGenerated, uniqueCount)
			t.Logf("唯一性率: %.2f%%", uniquenessRate)
			t.Logf("执行时间: %v", elapsed)
			t.Logf("平均每次计算: %v", elapsed/time.Duration(totalGenerated))
			
			// 断言唯一性率达到预期
			assert.GreaterOrEqual(t, uniquenessRate, tt.expectedUnique,
				"唯一性率应该达到 %.2f%% 以上", tt.expectedUnique)
		})
	}
}

// TestConcurrentOrderCreationPerformance 测试并发订单创建性能
func TestConcurrentOrderCreationPerformance(t *testing.T) {
	type orderRequest struct {
		OrderID string
		Amount  float64
		Chain   string
	}
	
	scenarios := []struct {
		name        string
		concurrent  int
		orders      int
		expectedTPS float64
	}{
		{
			name:        "标准负载",
			concurrent:  20,
			orders:      100,
			expectedTPS: 100,
		},
		{
			name:        "高负载",
			concurrent:  50,
			orders:      500,
			expectedTPS: 200,
		},
		{
			name:        "峰值负载",
			concurrent:  100,
			orders:      1000,
			expectedTPS: 300,
		},
	}
	
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			var successCount int64
			var failCount int64
			
			startTime := time.Now()
			var wg sync.WaitGroup
			wg.Add(sc.concurrent)
			
			orderChan := make(chan orderRequest, sc.orders)
			
			// 生成订单请求
			for i := 0; i < sc.orders; i++ {
				orderChan <- orderRequest{
					OrderID: fmt.Sprintf("TEST_%d_%d", time.Now().Unix(), i),
					Amount:  100.0 + rand.Float64()*900.0,
					Chain:   []string{"TRON", "BSC", "POLY", "OP"}[rand.Intn(4)],
				}
			}
			close(orderChan)
			
			// 并发处理订单
			for i := 0; i < sc.concurrent; i++ {
				go func(workerID int) {
					defer wg.Done()
					
					for range orderChan {
						// 模拟订单创建处理
						processTime := time.Duration(5+rand.Intn(10)) * time.Millisecond
						time.Sleep(processTime)
						
						// 模拟成功率 (95%+)
						if rand.Float64() > 0.05 {
							atomic.AddInt64(&successCount, 1)
						} else {
							atomic.AddInt64(&failCount, 1)
						}
					}
				}(i)
			}
			
			wg.Wait()
			elapsed := time.Since(startTime)
			
			// 计算性能指标
			totalProcessed := successCount + failCount
			successRate := float64(successCount) * 100 / float64(totalProcessed)
			tps := float64(totalProcessed) / elapsed.Seconds()
			avgLatency := elapsed / time.Duration(totalProcessed)
			
			// 输出结果
			t.Logf("场景: %s", sc.name)
			t.Logf("并发数: %d, 订单数: %d", sc.concurrent, sc.orders)
			t.Logf("成功: %d, 失败: %d", successCount, failCount)
			t.Logf("成功率: %.2f%%", successRate)
			t.Logf("TPS: %.2f", tps)
			t.Logf("平均延迟: %v", avgLatency)
			t.Logf("总耗时: %v", elapsed)
			
			// 断言
			assert.GreaterOrEqual(t, successRate, 95.0, "成功率应该达到95%以上")
			assert.GreaterOrEqual(t, tps, sc.expectedTPS, "TPS应该达到预期值")
		})
	}
}

// TestMemoryAndGoroutineLeaks 测试内存和goroutine泄漏
func TestMemoryAndGoroutineLeaks(t *testing.T) {
	// 记录初始状态
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)
	initialGoroutines := runtime.NumGoroutine()
	
	t.Logf("初始内存: %.2f MB", float64(initialMem.Alloc)/1024/1024)
	t.Logf("初始Goroutines: %d", initialGoroutines)
	
	// 执行大量并发操作
	iterations := 10
	for i := 0; i < iterations; i++ {
		var wg sync.WaitGroup
		concurrency := 100
		
		wg.Add(concurrency)
		for j := 0; j < concurrency; j++ {
			go func() {
				defer wg.Done()
				
				// 模拟一些工作
				time.Sleep(10 * time.Millisecond)
				
				// 生成金额
				for k := 0; k < 100; k++ {
					_ = calcTradeAmountImproved(100.0)
				}
			}()
		}
		wg.Wait()
		
		// 每次迭代后检查
		runtime.GC()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		goroutines := runtime.NumGoroutine()
		
		t.Logf("迭代 %d - 内存: %.2f MB, Goroutines: %d",
			i+1, float64(mem.Alloc)/1024/1024, goroutines)
	}
	
	// 等待清理
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	
	// 最终检查
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	finalGoroutines := runtime.NumGoroutine()
	
	t.Logf("最终内存: %.2f MB", float64(finalMem.Alloc)/1024/1024)
	t.Logf("最终Goroutines: %d", finalGoroutines)
	
	// 计算泄漏
	memoryIncrease := float64(finalMem.Alloc-initialMem.Alloc) / 1024 / 1024
	goroutineIncrease := finalGoroutines - initialGoroutines
	
	t.Logf("内存增长: %.2f MB", memoryIncrease)
	t.Logf("Goroutine增长: %d", goroutineIncrease)
	
	// 断言无明显泄漏
	assert.LessOrEqual(t, memoryIncrease, 10.0, "内存增长不应超过10MB")
	assert.LessOrEqual(t, goroutineIncrease, 5, "Goroutine增长不应超过5个")
}

// BenchmarkCalcTradeAmount 基准测试金额计算
func BenchmarkCalcTradeAmount(b *testing.B) {
	amounts := []float64{10.0, 100.0, 1000.0, 10000.0}
	
	for _, amount := range amounts {
		b.Run(fmt.Sprintf("Amount_%.0f", amount), func(b *testing.B) {
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					_ = calcTradeAmountImproved(amount)
				}
			})
		})
	}
}

// calcTradeAmountImproved 改进的金额计算函数
func calcTradeAmountImproved(money float64) string {
	// 使用高精度计算
	amount := decimal.NewFromFloat(money)
	atomicity := decimal.NewFromFloat(0.01)
	
	// 生成随机偏移
	rand.Seed(time.Now().UnixNano())
	offset := rand.Intn(100)
	randomOffset := decimal.NewFromFloat(float64(offset)).Mul(atomicity)
	
	// 计算最终金额
	finalAmount := amount.Add(randomOffset)
	
	// 格式化为字符串，保留2位小数
	return finalAmount.StringFixed(2)
}

// TestAPIResponseTime 测试API响应时间
func TestAPIResponseTime(t *testing.T) {
	endpoints := []struct {
		name         string
		expectedP50  time.Duration
		expectedP95  time.Duration
		expectedP99  time.Duration
	}{
		{
			name:        "创建订单",
			expectedP50: 50 * time.Millisecond,
			expectedP95: 200 * time.Millisecond,
			expectedP99: 500 * time.Millisecond,
		},
		{
			name:        "查询订单",
			expectedP50: 20 * time.Millisecond,
			expectedP95: 100 * time.Millisecond,
			expectedP99: 200 * time.Millisecond,
		},
		{
			name:        "支付回调",
			expectedP50: 100 * time.Millisecond,
			expectedP95: 300 * time.Millisecond,
			expectedP99: 800 * time.Millisecond,
		},
	}
	
	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			var latencies []time.Duration
			iterations := 1000
			
			for i := 0; i < iterations; i++ {
				start := time.Now()
				
				// 模拟API处理时间
				baseTime := 10 * time.Millisecond
				variableTime := time.Duration(rand.Intn(100)) * time.Millisecond
				time.Sleep(baseTime + variableTime)
				
				latencies = append(latencies, time.Since(start))
			}
			
			// 计算百分位数
			p50 := calculatePercentile(latencies, 50)
			p95 := calculatePercentile(latencies, 95)
			p99 := calculatePercentile(latencies, 99)
			
			t.Logf("API: %s", ep.name)
			t.Logf("P50: %v (期望: <%v)", p50, ep.expectedP50)
			t.Logf("P95: %v (期望: <%v)", p95, ep.expectedP95)
			t.Logf("P99: %v (期望: <%v)", p99, ep.expectedP99)
			
			// 验证性能指标
			assert.LessOrEqual(t, p50, ep.expectedP50, "P50延迟应该在预期范围内")
			assert.LessOrEqual(t, p95, ep.expectedP95, "P95延迟应该在预期范围内")
			assert.LessOrEqual(t, p99, ep.expectedP99, "P99延迟应该在预期范围内")
		})
	}
}

// calculatePercentile 计算百分位数
func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	
	// 简单的百分位数计算
	index := len(latencies) * percentile / 100
	if index >= len(latencies) {
		index = len(latencies) - 1
	}
	
	return latencies[index]
}