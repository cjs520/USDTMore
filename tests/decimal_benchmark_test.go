package tests

import (
	"USDTMore/app/model"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestDecimalFieldPerformance 测试decimal字段的性能
func TestDecimalFieldPerformance(t *testing.T) {
	// 初始化数据库连接
	cfg, err := model.LoadConfig("../config.yaml")
	require.NoError(t, err, "加载配置失败")

	// 构建DSN
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable TimeZone=Asia/Shanghai",
		cfg.Database.Host, cfg.Database.User, cfg.Database.Password, cfg.Database.Dbname, cfg.Database.Port)
	
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err, "初始化数据库失败")

	// 确保表结构已创建
	err = database.AutoMigrate(
		&model.TradeOrders{},
		&model.WalletAddress{},
		&model.TransactionsData{},
	)
	require.NoError(t, err, "数据库迁移失败")

	// 测试decimal字段的CRUD性能
	t.Run("DecimalCRUDPerformance", func(t *testing.T) {
		// 准备测试数据
		testCount := 1000
		orders := make([]*model.TradeOrders, testCount)
		
		for i := 0; i < testCount; i++ {
			amount, _ := decimal.NewFromString(fmt.Sprintf("%.8f", 100.0+float64(i)*0.01))
			money, _ := decimal.NewFromString(fmt.Sprintf("%.2f", 720.0+float64(i)*0.1))
			usdtRate, _ := decimal.NewFromString("7.2")
			
			orders[i] = &model.TradeOrders{
				OrderId:   fmt.Sprintf("perf_test_%d", i),
				TradeId:   fmt.Sprintf("trade_%d", i),
				Amount:    amount,
				Money:     money,
				UsdtRate:  usdtRate,
				Chain:     "TRON",
				Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
				Status:    1,
				ExpiredAt: time.Now().Add(time.Hour),
			}
		}

		// 批量插入性能测试
		start := time.Now()
		err := database.CreateInBatches(orders, 100).Error
		insertDuration := time.Since(start)
		require.NoError(t, err, "批量插入失败")
		
		t.Logf("批量插入 %d 条记录耗时: %v (%.2f records/s)", 
			testCount, insertDuration, float64(testCount)/insertDuration.Seconds())

		// 查询性能测试 - 精确查询
		start = time.Now()
		var foundOrder model.TradeOrders
		err = database.Where("order_id = ?", "perf_test_500").First(&foundOrder).Error
		queryDuration := time.Since(start)
		require.NoError(t, err, "查询失败")
		t.Logf("单条精确查询耗时: %v", queryDuration)

		// 验证decimal精度
		expectedAmount, _ := decimal.NewFromString(fmt.Sprintf("%.8f", 100.0+500*0.01))
		assert.True(t, foundOrder.Amount.Equal(expectedAmount), 
			"Amount精度不匹配: 期望=%s, 实际=%s", 
			expectedAmount.String(), foundOrder.Amount.String())

		// 范围查询性能测试
		start = time.Now()
		var rangeOrders []model.TradeOrders
		minAmount, _ := decimal.NewFromString("102.00")
		maxAmount, _ := decimal.NewFromString("103.00")
		err = database.Where("amount >= ? AND amount <= ?", minAmount, maxAmount).
			Find(&rangeOrders).Error
		rangeDuration := time.Since(start)
		require.NoError(t, err, "范围查询失败")
		t.Logf("范围查询找到 %d 条记录，耗时: %v", len(rangeOrders), rangeDuration)

		// 聚合查询性能测试
		start = time.Now()
		var totalAmount decimal.Decimal
		err = database.Model(&model.TradeOrders{}).
			Where("order_id LIKE ?", "perf_test_%").
			Select("SUM(amount)").Scan(&totalAmount).Error
		aggDuration := time.Since(start)
		require.NoError(t, err, "聚合查询失败")
		t.Logf("聚合查询（SUM）耗时: %v, 总额: %s", aggDuration, totalAmount.String())

		// 更新性能测试
		start = time.Now()
		newRate, _ := decimal.NewFromString("7.5")
		err = database.Model(&model.TradeOrders{}).
			Where("order_id LIKE ?", "perf_test_%").
			Update("usdt_rate", newRate).Error
		updateDuration := time.Since(start)
		require.NoError(t, err, "批量更新失败")
		t.Logf("批量更新 %d 条记录耗时: %v", testCount, updateDuration)

		// 清理测试数据
		err = database.Where("order_id LIKE ?", "perf_test_%").Delete(&model.TradeOrders{}).Error
		require.NoError(t, err, "清理测试数据失败")
	})

	// 测试decimal字段的计算性能
	t.Run("DecimalCalculationPerformance", func(t *testing.T) {
		iterations := 10000
		
		// 测试decimal加法性能
		start := time.Now()
		sum := decimal.Zero
		for i := 0; i < iterations; i++ {
			value, _ := decimal.NewFromString(fmt.Sprintf("%.8f", float64(i)*0.00000001))
			sum = sum.Add(value)
		}
		addDuration := time.Since(start)
		t.Logf("Decimal加法 %d 次耗时: %v (%.0f ops/s)", 
			iterations, addDuration, float64(iterations)/addDuration.Seconds())

		// 测试decimal乘法性能
		start = time.Now()
		product := decimal.NewFromInt(1)
		multiplier, _ := decimal.NewFromString("1.00000001")
		for i := 0; i < iterations; i++ {
			product = product.Mul(multiplier)
		}
		mulDuration := time.Since(start)
		t.Logf("Decimal乘法 %d 次耗时: %v (%.0f ops/s)", 
			iterations, mulDuration, float64(iterations)/mulDuration.Seconds())

		// 测试decimal比较性能
		start = time.Now()
		value1, _ := decimal.NewFromString("100.12345678")
		value2, _ := decimal.NewFromString("100.12345679")
		compareCount := 0
		for i := 0; i < iterations; i++ {
			if value1.LessThan(value2) {
				compareCount++
			}
		}
		compareDuration := time.Since(start)
		t.Logf("Decimal比较 %d 次耗时: %v (%.0f ops/s)", 
			iterations, compareDuration, float64(iterations)/compareDuration.Seconds())
	})

	// 测试并发场景下的decimal字段性能
	t.Run("ConcurrentDecimalOperations", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		concurrency := 10
		operationsPerGoroutine := 100
		
		start := time.Now()
		errChan := make(chan error, concurrency)
		
		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				for j := 0; j < operationsPerGoroutine; j++ {
					select {
					case <-ctx.Done():
						errChan <- ctx.Err()
						return
					default:
						amount, _ := decimal.NewFromString(fmt.Sprintf("%.8f", 100.0+float64(workerID*100+j)*0.01))
						money, _ := decimal.NewFromString(fmt.Sprintf("%.2f", 720.0+float64(workerID*100+j)*0.1))
						usdtRate, _ := decimal.NewFromString("7.2")
						
						order := &model.TradeOrders{
							OrderId:   fmt.Sprintf("concurrent_%d_%d", workerID, j),
							TradeId:   fmt.Sprintf("trade_%d_%d", workerID, j),
							Amount:    amount,
							Money:     money,
							UsdtRate:  usdtRate,
							Chain:     "TRON",
							Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
							Status:    1,
							ExpiredAt: time.Now().Add(time.Hour),
						}
						
						if err := database.Create(order).Error; err != nil {
							errChan <- err
							return
						}
					}
				}
				errChan <- nil
			}(i)
		}

		// 等待所有goroutine完成
		for i := 0; i < concurrency; i++ {
			if err := <-errChan; err != nil {
				t.Errorf("并发操作失败: %v", err)
			}
		}
		
		concurrentDuration := time.Since(start)
		totalOperations := concurrency * operationsPerGoroutine
		t.Logf("并发执行 %d 个操作耗时: %v (%.2f ops/s)", 
			totalOperations, concurrentDuration, float64(totalOperations)/concurrentDuration.Seconds())

		// 清理并发测试数据
		err = database.Where("order_id LIKE ?", "concurrent_%").Delete(&model.TradeOrders{}).Error
		require.NoError(t, err, "清理并发测试数据失败")
	})
}

// BenchmarkDecimalOperations 基准测试decimal操作
func BenchmarkDecimalOperations(b *testing.B) {
	b.Run("DecimalCreation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = decimal.NewFromString("123.45678901")
		}
	})

	b.Run("DecimalAddition", func(b *testing.B) {
		value1, _ := decimal.NewFromString("123.45678901")
		value2, _ := decimal.NewFromString("987.65432109")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = value1.Add(value2)
		}
	})

	b.Run("DecimalMultiplication", func(b *testing.B) {
		value1, _ := decimal.NewFromString("123.45678901")
		value2, _ := decimal.NewFromString("7.2")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = value1.Mul(value2)
		}
	})

	b.Run("DecimalComparison", func(b *testing.B) {
		value1, _ := decimal.NewFromString("123.45678901")
		value2, _ := decimal.NewFromString("123.45678902")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = value1.LessThan(value2)
		}
	})
}