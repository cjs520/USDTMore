package tests

import (
	"USDTMore/app/model"
	"USDTMore/tests/testutils"
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

// SchemaPerformanceTestSuite 新模式性能测试套件
type SchemaPerformanceTestSuite struct {
	suite.Suite
	ctx       context.Context
	container interface{}
	db        *gorm.DB
}

func (suite *SchemaPerformanceTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	var err error
	suite.container, suite.db = testutils.SetupTestDB(suite.ctx, suite.T())
	require.NoError(suite.T(), err)
	testutils.TestDB = suite.db
}

func (suite *SchemaPerformanceTestSuite) TearDownSuite() {
	testutils.TeardownTestDB(suite.ctx, suite.container)
}

func (suite *SchemaPerformanceTestSuite) SetupTest() {
	testutils.CleanDatabase(suite.db)
}

// TestDecimalFieldPerformance 测试decimal字段性能
func (suite *SchemaPerformanceTestSuite) TestDecimalFieldPerformance() {
	t := suite.T()

	// 1. 测试decimal字段插入性能
	suite.Run("DecimalInsertPerformance", func() {
		numRecords := 10000
		orders := make([]*model.TradeOrders, numRecords)
		
		// 准备测试数据
		for i := 0; i < numRecords; i++ {
			orders[i] = &model.TradeOrders{
				OrderId:   fmt.Sprintf("decimal_perf_order_%06d", i),
				TradeId:   fmt.Sprintf("decimal_perf_trade_%06d", i),
				UsdtRate:  decimal.NewFromFloat(6.5 + rand.Float64()*2), // 6.5-8.5范围
				Amount:    decimal.NewFromFloat(rand.Float64() * 1000),   // 0-1000范围
				Money:     decimal.NewFromFloat(rand.Float64() * 10000),  // 0-10000范围
				Chain:     []string{"TRON", "BSC", "POLYGON", "OPTIMISM"}[i%4],
				Address:   fmt.Sprintf("addr_%d", i%100),
				Status:    int16(1 + i%3),
				Version:   0,
				ExpiredAt: time.Now().Add(time.Duration(i%24) * time.Hour),
			}
		}

		// 测试批量插入性能
		start := time.Now()
		batchSize := 500
		
		for i := 0; i < len(orders); i += batchSize {
			end := i + batchSize
			if end > len(orders) {
				end = len(orders)
			}
			
			err := suite.db.CreateInBatches(orders[i:end], batchSize).Error
			require.NoError(t, err)
		}
		
		insertDuration := time.Since(start)
		insertRate := float64(numRecords) / insertDuration.Seconds()
		
		t.Logf("插入%d条记录耗时: %v", numRecords, insertDuration)
		t.Logf("插入速率: %.2f 记录/秒", insertRate)
		
		// 性能要求：至少1000记录/秒
		assert.Greater(t, insertRate, 1000.0, "decimal字段插入性能应该至少1000记录/秒")
		assert.Less(t, insertDuration, 30*time.Second, "插入10000条记录应该在30秒内完成")
	})

	// 2. 测试decimal字段查询性能
	suite.Run("DecimalQueryPerformance", func() {
		// 首先创建测试数据
		numRecords := 5000
		orders := make([]*model.TradeOrders, numRecords)
		
		for i := 0; i < numRecords; i++ {
			orders[i] = &model.TradeOrders{
				OrderId:   fmt.Sprintf("decimal_query_order_%06d", i),
				TradeId:   fmt.Sprintf("decimal_query_trade_%06d", i),
				UsdtRate:  decimal.NewFromFloat(7.0),
				Amount:    decimal.NewFromFloat(100.0 + float64(i)*0.01),
				Money:     decimal.NewFromFloat(700.0 + float64(i)*0.07),
				Chain:     "TRON",
				Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
				Status:    1,
				Version:   0,
				ExpiredAt: time.Now().Add(time.Hour),
			}
		}
		
		err := suite.db.CreateInBatches(orders, 500).Error
		require.NoError(t, err)

		// 测试各种decimal字段查询
		queryTests := []struct {
			name     string
			query    func() *gorm.DB
			maxTime  time.Duration
		}{
			{
				name: "按Amount范围查询",
				query: func() *gorm.DB {
					return suite.db.Where("amount BETWEEN ? AND ?", 
						decimal.NewFromFloat(150.0), decimal.NewFromFloat(200.0))
				},
				maxTime: 100 * time.Millisecond,
			},
			{
				name: "按Money精确查询",
				query: func() *gorm.DB {
					return suite.db.Where("money = ?", decimal.NewFromFloat(735.0))
				},
				maxTime: 50 * time.Millisecond,
			},
			{
				name: "按UsdtRate排序查询",
				query: func() *gorm.DB {
					return suite.db.Order("usdt_rate DESC").Limit(100)
				},
				maxTime: 100 * time.Millisecond,
			},
			{
				name: "decimal聚合查询",
				query: func() *gorm.DB {
					return suite.db.Select("AVG(amount) as avg_amount, SUM(money) as total_money")
				},
				maxTime: 200 * time.Millisecond,
			},
		}

		for _, test := range queryTests {
			t.Run(test.name, func(t *testing.T) {
				start := time.Now()
				
				var results []map[string]interface{}
				err := test.query().Find(&results).Error
				
				duration := time.Since(start)
				
				require.NoError(t, err, "查询不应该出错")
				assert.Less(t, duration, test.maxTime,
					"查询时间 %v 应该小于 %v", duration, test.maxTime)
				
				t.Logf("%s: 查询时间=%v, 结果数=%d", test.name, duration, len(results))
			})
		}
	})

	// 3. 测试decimal字段计算性能
	suite.Run("DecimalCalculationPerformance", func() {
		numCalculations := 10000
		
		start := time.Now()
		
		for i := 0; i < numCalculations; i++ {
			money := decimal.NewFromFloat(100.0 + rand.Float64()*1000)
			rate := decimal.NewFromFloat(6.5 + rand.Float64()*2)
			
			// 模拟业务计算
			amount := money.Div(rate)
			formatted := amount.StringFixed(8)
			
			// 验证计算结果合理性
			assert.Greater(t, amount.InexactFloat64(), 0.0, "计算结果应该大于0")
			assert.NotEmpty(t, formatted, "格式化结果不应为空")
		}
		
		calculationDuration := time.Since(start)
		calculationRate := float64(numCalculations) / calculationDuration.Seconds()
		
		t.Logf("执行%d次decimal计算耗时: %v", numCalculations, calculationDuration)
		t.Logf("计算速率: %.2f 计算/秒", calculationRate)
		
		// 性能要求：至少10000计算/秒
		assert.Greater(t, calculationRate, 10000.0, "decimal计算性能应该至少10000计算/秒")
	})
}

// TestTimestamptzFieldPerformance 测试timestamptz字段性能
func (suite *SchemaPerformanceTestSuite) TestTimestamptzFieldPerformance() {
	t := suite.T()

	// 1. 测试时间字段插入性能
	suite.Run("TimestampInsertPerformance", func() {
		numRecords := 5000
		orders := make([]*model.TradeOrders, numRecords)
		
		baseTime := time.Now()
		for i := 0; i < numRecords; i++ {
			createdAt := baseTime.Add(time.Duration(i) * time.Minute)
			expiredAt := createdAt.Add(24 * time.Hour)
			
			orders[i] = &model.TradeOrders{
				OrderId:   fmt.Sprintf("time_perf_order_%06d", i),
				TradeId:   fmt.Sprintf("time_perf_trade_%06d", i),
				UsdtRate:  decimal.NewFromFloat(7.0),
				Amount:    decimal.NewFromFloat(100.0),
				Money:     decimal.NewFromFloat(700.0),
				Chain:     "TRON",
				Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
				Status:    1,
				Version:   0,
				ExpiredAt: expiredAt,
				CreatedAt: createdAt,
			}
		}

		start := time.Now()
		err := suite.db.CreateInBatches(orders, 500).Error
		insertDuration := time.Since(start)
		
		require.NoError(t, err)
		insertRate := float64(numRecords) / insertDuration.Seconds()
		
		t.Logf("插入%d条时间记录耗时: %v", numRecords, insertDuration)
		t.Logf("插入速率: %.2f 记录/秒", insertRate)
		
		assert.Greater(t, insertRate, 1000.0, "timestamptz字段插入性能应该至少1000记录/秒")
	})

	// 2. 测试时间字段查询性能
	suite.Run("TimestampQueryPerformance", func() {
		now := time.Now()
		past := now.Add(-24 * time.Hour)
		future := now.Add(24 * time.Hour)

		timeQueryTests := []struct {
			name     string
			query    func() *gorm.DB
			maxTime  time.Duration
		}{
			{
				name: "按创建时间范围查询",
				query: func() *gorm.DB {
					return suite.db.Where("created_at BETWEEN ? AND ?", past, now)
				},
				maxTime: 100 * time.Millisecond,
			},
			{
				name: "按过期时间查询",
				query: func() *gorm.DB {
					return suite.db.Where("expired_at > ?", now)
				},
				maxTime: 100 * time.Millisecond,
			},
			{
				name: "时间排序查询",
				query: func() *gorm.DB {
					return suite.db.Order("created_at DESC").Limit(1000)
				},
				maxTime: 150 * time.Millisecond,
			},
			{
				name: "时间聚合查询",
				query: func() *gorm.DB {
					return suite.db.Select("DATE(created_at) as date, COUNT(*) as count").
						Group("DATE(created_at)")
				},
				maxTime: 200 * time.Millisecond,
			},
		}

		for _, test := range timeQueryTests {
			t.Run(test.name, func(t *testing.T) {
				start := time.Now()
				
				var results []map[string]interface{}
				err := test.query().Find(&results).Error
				
				duration := time.Since(start)
				
				require.NoError(t, err, "时间查询不应该出错")
				assert.Less(t, duration, test.maxTime,
					"查询时间 %v 应该小于 %v", duration, test.maxTime)
				
				t.Logf("%s: 查询时间=%v, 结果数=%d", test.name, duration, len(results))
			})
		}
	})
}

// TestIndexPerformanceOptimization 测试索引性能优化
func (suite *SchemaPerformanceTestSuite) TestIndexPerformanceOptimization() {
	t := suite.T()

	// 1. 准备大量测试数据
	suite.Run("PrepareIndexTestData", func() {
		numRecords := 50000
		
		start := time.Now()
		batchSize := 1000
		
		for batch := 0; batch < numRecords/batchSize; batch++ {
			orders := make([]*model.TradeOrders, batchSize)
			
			for i := 0; i < batchSize; i++ {
				recordIndex := batch*batchSize + i
				orders[i] = &model.TradeOrders{
					OrderId:   fmt.Sprintf("idx_perf_order_%08d", recordIndex),
					TradeId:   fmt.Sprintf("idx_perf_trade_%08d", recordIndex),
					UsdtRate:  decimal.NewFromFloat(6.5 + rand.Float64()*2),
					Amount:    decimal.NewFromFloat(rand.Float64() * 1000),
					Money:     decimal.NewFromFloat(rand.Float64() * 10000),
					Chain:     []string{"TRON", "BSC", "POLYGON", "OPTIMISM", "ARBITRUM"}[recordIndex%5],
					Address:   fmt.Sprintf("addr_%d", recordIndex%1000),
					Status:    int16(1 + recordIndex%3),
					Version:   0,
					ExpiredAt: time.Now().Add(time.Duration(recordIndex%168) * time.Hour), // 一周内随机
				}
			}
			
			err := suite.db.CreateInBatches(orders, batchSize).Error
			require.NoError(t, err, "批次 %d 插入失败", batch)
		}
		
		prepareDuration := time.Since(start)
		t.Logf("准备%d条索引测试数据耗时: %v", numRecords, prepareDuration)
	})

	// 2. 测试各种索引查询性能
	suite.Run("IndexQueryPerformance", func() {
		indexQueryTests := []struct {
			name           string
			query          func() *gorm.DB
			maxTime        time.Duration
			minResults     int
			expectIndex    bool // 是否期望使用索引
		}{
			{
				name: "单字段索引查询-状态",
				query: func() *gorm.DB {
					return suite.db.Where("status = ?", 1)
				},
				maxTime:     50 * time.Millisecond,
				minResults:  1,
				expectIndex: true,
			},
			{
				name: "单字段索引查询-链",
				query: func() *gorm.DB {
					return suite.db.Where("chain = ?", "TRON")
				},
				maxTime:     50 * time.Millisecond,
				minResults:  1,
				expectIndex: true,
			},
			{
				name: "复合索引查询-状态+链",
				query: func() *gorm.DB {
					return suite.db.Where("status = ? AND chain = ?", 1, "TRON")
				},
				maxTime:     50 * time.Millisecond,
				minResults:  1,
				expectIndex: true,
			},
			{
				name: "复合索引查询-状态+链+地址",
				query: func() *gorm.DB {
					return suite.db.Where("status = ? AND chain = ? AND address = ?", 1, "TRON", "addr_0")
				},
				maxTime:     30 * time.Millisecond,
				minResults:  0, // 可能没有匹配结果
				expectIndex: true,
			},
			{
				name: "范围查询-时间索引",
				query: func() *gorm.DB {
					return suite.db.Where("expired_at > ?", time.Now())
				},
				maxTime:     100 * time.Millisecond,
				minResults:  1,
				expectIndex: true,
			},
			{
				name: "ORDER BY索引字段",
				query: func() *gorm.DB {
					return suite.db.Order("status, chain").Limit(1000)
				},
				maxTime:     150 * time.Millisecond,
				minResults:  1000,
				expectIndex: true,
			},
		}

		for _, test := range indexQueryTests {
			t.Run(test.name, func(t *testing.T) {
				start := time.Now()
				
				var results []model.TradeOrders
				err := test.query().Find(&results).Error
				
				duration := time.Since(start)
				
				require.NoError(t, err, "索引查询不应该出错")
				if test.minResults > 0 {
					assert.GreaterOrEqual(t, len(results), test.minResults, 
						"应该返回至少%d条结果", test.minResults)
				}
				assert.Less(t, duration, test.maxTime,
					"查询时间 %v 应该小于 %v", duration, test.maxTime)
				
				t.Logf("%s: 查询时间=%v, 结果数=%d", test.name, duration, len(results))
			})
		}
	})

	// 3. 测试并发查询性能
	suite.Run("ConcurrentQueryPerformance", func() {
		numGoroutines := 50
		queriesPerGoroutine := 20
		
		var wg sync.WaitGroup
		results := make(chan time.Duration, numGoroutines*queriesPerGoroutine)
		errors := make(chan error, numGoroutines*queriesPerGoroutine)
		
		start := time.Now()
		
		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				
				for q := 0; q < queriesPerGoroutine; q++ {
					queryStart := time.Now()
					
					// 随机选择查询类型
					queryType := q % 4
					var err error
					var count int64
					
					switch queryType {
					case 0:
						err = suite.db.Model(&model.TradeOrders{}).
							Where("status = ?", 1).Count(&count).Error
					case 1:
						err = suite.db.Model(&model.TradeOrders{}).
							Where("chain = ?", "TRON").Count(&count).Error
					case 2:
						err = suite.db.Model(&model.TradeOrders{}).
							Where("status = ? AND chain = ?", 1, "BSC").Count(&count).Error
					case 3:
						err = suite.db.Model(&model.TradeOrders{}).
							Where("expired_at > ?", time.Now()).Count(&count).Error
					}
					
					queryDuration := time.Since(queryStart)
					
					if err != nil {
						errors <- err
					} else {
						results <- queryDuration
					}
				}
			}(g)
		}
		
		wg.Wait()
		close(results)
		close(errors)
		
		totalDuration := time.Since(start)
		
		// 检查错误
		var errorCount int
		for err := range errors {
			t.Errorf("并发查询错误: %v", err)
			errorCount++
		}
		assert.Equal(t, 0, errorCount, "不应该有查询错误")
		
		// 分析查询时间
		var totalQueryTime time.Duration
		var maxQueryTime time.Duration
		var queryCount int
		
		for queryTime := range results {
			totalQueryTime += queryTime
			if queryTime > maxQueryTime {
				maxQueryTime = queryTime
			}
			queryCount++
		}
		
		avgQueryTime := totalQueryTime / time.Duration(queryCount)
		qps := float64(queryCount) / totalDuration.Seconds()
		
		t.Logf("并发查询统计:")
		t.Logf("  总查询数: %d", queryCount)
		t.Logf("  总耗时: %v", totalDuration)
		t.Logf("  平均查询时间: %v", avgQueryTime)
		t.Logf("  最大查询时间: %v", maxQueryTime)
		t.Logf("  QPS: %.2f", qps)
		
		// 性能要求
		assert.Less(t, avgQueryTime, 100*time.Millisecond, "平均查询时间应该小于100ms")
		assert.Less(t, maxQueryTime, 500*time.Millisecond, "最大查询时间应该小于500ms")
		assert.Greater(t, qps, 200.0, "QPS应该大于200")
	})
}

// TestSchemaScalabilityPerformance 测试模式可扩展性性能
func (suite *SchemaPerformanceTestSuite) TestSchemaScalabilityPerformance() {
	t := suite.T()

	// 1. 测试大数据集下的性能
	suite.Run("LargeDatasetPerformance", func() {
		// 测试不同数据量级下的查询性能
		dataSizes := []int{1000, 5000, 10000, 25000}
		
		for _, size := range dataSizes {
			t.Run(fmt.Sprintf("DataSize_%d", size), func(t *testing.T) {
				// 清理并准备指定数量的数据
				testutils.CleanDatabase(suite.db)
				
				orders := make([]*model.TradeOrders, size)
				for i := 0; i < size; i++ {
					orders[i] = &model.TradeOrders{
						OrderId:   fmt.Sprintf("scale_order_%08d", i),
						TradeId:   fmt.Sprintf("scale_trade_%08d", i),
						UsdtRate:  decimal.NewFromFloat(6.5 + rand.Float64()*2),
						Amount:    decimal.NewFromFloat(rand.Float64() * 1000),
						Money:     decimal.NewFromFloat(rand.Float64() * 10000),
						Chain:     []string{"TRON", "BSC", "POLYGON"}[i%3],
						Address:   fmt.Sprintf("addr_%d", i%100),
						Status:    int16(1 + i%3),
						Version:   0,
						ExpiredAt: time.Now().Add(time.Duration(i%24) * time.Hour),
					}
				}
				
				// 插入性能测试
				insertStart := time.Now()
				err := suite.db.CreateInBatches(orders, 1000).Error
				insertDuration := time.Since(insertStart)
				require.NoError(t, err)
				
				// 查询性能测试
				queryStart := time.Now()
				var results []model.TradeOrders
				err = suite.db.Where("status = ? AND chain = ?", 1, "TRON").Find(&results).Error
				queryDuration := time.Since(queryStart)
				require.NoError(t, err)
				
				insertRate := float64(size) / insertDuration.Seconds()
				t.Logf("数据量 %d: 插入耗时=%v (%.2f记录/秒), 查询耗时=%v, 结果数=%d",
					size, insertDuration, insertRate, queryDuration, len(results))
				
				// 性能不应该随数据量线性下降
				assert.Less(t, queryDuration, 200*time.Millisecond, 
					"数据量 %d 时查询时间应该小于200ms", size)
			})
		}
	})

	// 2. 测试内存使用效率
	suite.Run("MemoryUsageEfficiency", func() {
		// 创建大量记录并监控性能
		numRecords := 20000
		orders := make([]*model.TradeOrders, numRecords)
		
		for i := 0; i < numRecords; i++ {
			orders[i] = &model.TradeOrders{
				OrderId:   fmt.Sprintf("memory_test_order_%08d", i),
				TradeId:   fmt.Sprintf("memory_test_trade_%08d", i),
				UsdtRate:  decimal.NewFromFloat(7.0),
				Amount:    decimal.NewFromFloat(100.0),
				Money:     decimal.NewFromFloat(700.0),
				Chain:     "TRON",
				Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
				Status:    1,
				Version:   0,
				ExpiredAt: time.Now().Add(time.Hour),
			}
		}
		
		// 批量插入
		start := time.Now()
		err := suite.db.CreateInBatches(orders, 1000).Error
		insertDuration := time.Since(start)
		require.NoError(t, err)
		
		// 大量查询测试内存效率
		queryStart := time.Now()
		for i := 0; i < 100; i++ {
			var results []model.TradeOrders
			err := suite.db.Where("status = ?", 1).Limit(1000).Find(&results).Error
			require.NoError(t, err)
		}
		queryDuration := time.Since(queryStart)
		
		t.Logf("内存效率测试: 插入%d记录耗时=%v, 100次查询耗时=%v",
			numRecords, insertDuration, queryDuration)
		
		assert.Less(t, queryDuration, 5*time.Second, "100次查询应该在5秒内完成")
	})
}

// TestSchemaPerformanceRegression 测试性能回归
func (suite *SchemaPerformanceTestSuite) TestSchemaPerformanceRegression() {
	t := suite.T()

	// 1. 基准性能测试
	suite.Run("BaselinePerformanceTest", func() {
		// 准备标准测试数据集
		numRecords := 10000
		orders := make([]*model.TradeOrders, numRecords)
		
		for i := 0; i < numRecords; i++ {
			orders[i] = &model.TradeOrders{
				OrderId:   fmt.Sprintf("baseline_order_%08d", i),
				TradeId:   fmt.Sprintf("baseline_trade_%08d", i),
				UsdtRate:  decimal.NewFromFloat(7.0),
				Amount:    decimal.NewFromFloat(100.0 + float64(i)*0.01),
				Money:     decimal.NewFromFloat(700.0),
				Chain:     []string{"TRON", "BSC", "POLYGON", "OPTIMISM"}[i%4],
				Address:   fmt.Sprintf("addr_%d", i%100),
				Status:    int16(1 + i%3),
				Version:   0,
				ExpiredAt: time.Now().Add(time.Duration(i%24) * time.Hour),
			}
		}
		
		// 基准插入测试
		insertStart := time.Now()
		err := suite.db.CreateInBatches(orders, 500).Error
		insertDuration := time.Since(insertStart)
		require.NoError(t, err)
		
		// 基准查询测试集
		benchmarkQueries := []struct {
			name  string
			query func() *gorm.DB
		}{
			{
				name: "简单条件查询",
				query: func() *gorm.DB {
					return suite.db.Where("status = ?", 1)
				},
			},
			{
				name: "复合条件查询",
				query: func() *gorm.DB {
					return suite.db.Where("status = ? AND chain = ?", 1, "TRON")
				},
			},
			{
				name: "范围查询",
				query: func() *gorm.DB {
					return suite.db.Where("amount BETWEEN ? AND ?", 
						decimal.NewFromFloat(150.0), decimal.NewFromFloat(200.0))
				},
			},
			{
				name: "排序查询",
				query: func() *gorm.DB {
					return suite.db.Order("created_at DESC").Limit(1000)
				},
			},
		}
		
		queryDurations := make(map[string]time.Duration)
		
		for _, benchmark := range benchmarkQueries {
			queryStart := time.Now()
			
			var results []model.TradeOrders
			err := benchmark.query().Find(&results).Error
			
			queryDuration := time.Since(queryStart)
			require.NoError(t, err, "%s 查询不应该出错", benchmark.name)
			
			queryDurations[benchmark.name] = queryDuration
			t.Logf("%s: 耗时=%v, 结果数=%d", benchmark.name, queryDuration, len(results))
		}
		
		// 性能基线要求
		insertRate := float64(numRecords) / insertDuration.Seconds()
		assert.Greater(t, insertRate, 2000.0, "插入性能应该至少2000记录/秒")
		
		for name, duration := range queryDurations {
			maxDuration := 200 * time.Millisecond
			if name == "排序查询" {
				maxDuration = 300 * time.Millisecond
			}
			assert.Less(t, duration, maxDuration, "%s 性能不应该退化", name)
		}
	})
}

// TestSchemaPerformance 运行新模式性能测试
func TestSchemaPerformance(t *testing.T) {
	suite.Run(t, new(SchemaPerformanceTestSuite))
}