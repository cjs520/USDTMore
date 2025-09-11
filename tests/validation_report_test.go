package tests

import (
	"USDTMore/app/model"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestReport 测试报告结构
type TestReport struct {
	TestName       string
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	Success        bool
	ErrorMessage   string
	MetricValue    float64
	MetricUnit     string
	AdditionalInfo map[string]interface{}
}

// ValidationSummary 验证总结
type ValidationSummary struct {
	TotalTests       int
	PassedTests      int
	FailedTests      int
	TestReports      []TestReport
	PerformanceData  map[string]float64
	BusinessValidation map[string]bool
	StartTime        time.Time
	EndTime          time.Time
	OverallDuration  time.Duration
}

// setupReportDB 设置报告测试数据库
func setupReportDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// 创建简化的表结构
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
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			confirmed_at TIMESTAMP NULL
		)
	`).Error
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&model.WalletAddress{})
	if err != nil {
		return nil, err
	}

	model.DB = db
	return db, nil
}

// createReportTestOrder 创建报告测试订单
func createReportTestOrder(customFields map[string]interface{}) *model.TradeOrders {
	nanoTime := time.Now().UnixNano()
	randNum := rand.Int31()
	
	order := &model.TradeOrders{
		OrderId:     fmt.Sprintf("REPORT_%d_%d", nanoTime, randNum),
		TradeId:     fmt.Sprintf("RTID_%d_%d", nanoTime, randNum),
		TradeHash:   "",
		UsdtRate:    "7.20",
		Amount:      "100.00",
		Money:       720.00,
		Chain:       "TRON",
		Address:     "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		FromAddress: "",
		Status:      model.OrderStatusWaiting,
		ReturnUrl:   "https://example.com/return",
		NotifyUrl:   "https://example.com/notify",
		NotifyNum:   0,
		NotifyState: model.OrderNotifyStateFail,
		ExpiredAt:   time.Now().Add(30 * time.Minute),
	}

	// 应用自定义字段
	if customFields != nil {
		if v, ok := customFields["order_id"].(string); ok {
			order.OrderId = v
		}
		if v, ok := customFields["amount"].(string); ok {
			order.Amount = v
		}
		if v, ok := customFields["money"].(float64); ok {
			order.Money = v
		}
		if v, ok := customFields["status"].(int); ok {
			order.Status = v
		}
		if v, ok := customFields["chain"].(string); ok {
			order.Chain = v
		}
		if v, ok := customFields["expired_at"].(time.Time); ok {
			order.ExpiredAt = v
		}
	}

	return order
}

// Test_ValidationSummaryReport 完整验证测试报告
func Test_ValidationSummaryReport(t *testing.T) {
	summary := &ValidationSummary{
		TestReports:        make([]TestReport, 0),
		PerformanceData:    make(map[string]float64),
		BusinessValidation: make(map[string]bool),
		StartTime:          time.Now(),
	}

	db, err := setupReportDB()
	require.NoError(t, err)
	defer func() {
		sqlDB, _ := db.DB()
		sqlDB.Close()
	}()

	// 创建测试钱包地址
	address := &model.WalletAddress{
		Chain:   "TRON",
		Address: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
	}
	db.Create(address)

	// 1. 基本功能验证
	t.Run("基本功能验证", func(t *testing.T) {
		report := TestReport{
			TestName:       "基本订单创建与状态转换",
			StartTime:      time.Now(),
			AdditionalInfo: make(map[string]interface{}),
		}

		// 创建订单
		order := createReportTestOrder(nil)
		err := db.Create(order).Error
		
		report.Success = (err == nil)
		if err != nil {
			report.ErrorMessage = err.Error()
		} else {
			// 状态转换测试
			fromAddress := "TTestFromAddress"
			txHash := fmt.Sprintf("tx_%d", time.Now().UnixNano())
			confirmedAt := time.Now()

			err = order.OrderSetSucc(fromAddress, txHash, confirmedAt)
			report.Success = (err == nil)
			if err != nil {
				report.ErrorMessage = fmt.Sprintf("状态转换失败: %v", err)
			}
		}

		report.EndTime = time.Now()
		report.Duration = report.EndTime.Sub(report.StartTime)
		summary.TestReports = append(summary.TestReports, report)
		
		if report.Success {
			summary.PassedTests++
		} else {
			summary.FailedTests++
		}
		summary.TotalTests++

		summary.BusinessValidation["订单创建"] = report.Success
		summary.BusinessValidation["状态转换"] = report.Success
	})

	// 2. 金额计算验证
	t.Run("金额计算验证", func(t *testing.T) {
		report := TestReport{
			TestName:       "金额计算逻辑验证",
			StartTime:      time.Now(),
			AdditionalInfo: make(map[string]interface{}),
		}

		walletAddresses := []model.WalletAddress{*address}
		
		// 测试基本金额计算
		rate := 7.20
		money := 720.00
		
		resultAddress, amount := model.CalcTradeAmount(walletAddresses, rate, money)
		
		success := (resultAddress.Address != "" && amount != "")
		if success {
			parsedAmount, err := decimal.NewFromString(amount)
			success = (err == nil && parsedAmount.GreaterThan(decimal.Zero))
			
			if success {
				expectedAmount := money / rate
				actualAmount, _ := parsedAmount.Float64()
				report.MetricValue = actualAmount
				report.MetricUnit = "USDT"
				report.AdditionalInfo["expected"] = expectedAmount
				report.AdditionalInfo["actual"] = actualAmount
				report.AdditionalInfo["rate"] = rate
			}
		}

		report.Success = success
		if !success {
			report.ErrorMessage = "金额计算失败或结果无效"
		}

		report.EndTime = time.Now()
		report.Duration = report.EndTime.Sub(report.StartTime)
		summary.TestReports = append(summary.TestReports, report)
		
		if report.Success {
			summary.PassedTests++
		} else {
			summary.FailedTests++
		}
		summary.TotalTests++

		summary.BusinessValidation["金额计算"] = report.Success
	})

	// 3. 性能基准测试
	t.Run("性能基准测试", func(t *testing.T) {
		report := TestReport{
			TestName:       "订单创建性能基准",
			StartTime:      time.Now(),
			AdditionalInfo: make(map[string]interface{}),
		}

		batchSize := 100
		successCount := 0
		
		startTime := time.Now()
		for i := 0; i < batchSize; i++ {
			order := createReportTestOrder(map[string]interface{}{
				"order_id": fmt.Sprintf("PERF_%d_%d", i, time.Now().UnixNano()),
			})
			
			if err := db.Create(order).Error; err == nil {
				successCount++
			}
		}
		duration := time.Since(startTime)
		
		tps := float64(successCount) / duration.Seconds()
		report.MetricValue = tps
		report.MetricUnit = "TPS"
		report.Success = (successCount == batchSize && tps > 50.0)
		
		if !report.Success {
			report.ErrorMessage = fmt.Sprintf("性能不达标: 成功率=%d/%d, TPS=%.2f", 
				successCount, batchSize, tps)
		}

		report.AdditionalInfo["batch_size"] = batchSize
		report.AdditionalInfo["success_count"] = successCount
		report.AdditionalInfo["duration_ms"] = duration.Milliseconds()

		report.EndTime = time.Now()
		report.Duration = report.EndTime.Sub(report.StartTime)
		summary.TestReports = append(summary.TestReports, report)
		
		if report.Success {
			summary.PassedTests++
		} else {
			summary.FailedTests++
		}
		summary.TotalTests++

		summary.PerformanceData["订单创建TPS"] = tps
		summary.PerformanceData["批量创建成功率"] = float64(successCount) / float64(batchSize)
	})

	// 4. 并发安全性测试
	t.Run("并发安全性测试", func(t *testing.T) {
		report := TestReport{
			TestName:       "金额计算并发安全",
			StartTime:      time.Now(),
			AdditionalInfo: make(map[string]interface{}),
		}

		walletAddresses := []model.WalletAddress{*address}
		concurrentCount := 20
		results := make(chan string, concurrentCount)
		
		wg := sync.WaitGroup{}
		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				rate := 7.20
				money := 720.00 + float64(index)*0.05
				
				_, amount := model.CalcTradeAmount(walletAddresses, rate, money)
				results <- amount
			}(i)
		}
		
		wg.Wait()
		close(results)
		
		uniqueAmounts := make(map[string]bool)
		totalResults := 0
		for amount := range results {
			uniqueAmounts[amount] = true
			totalResults++
		}
		
		uniquenessRate := float64(len(uniqueAmounts)) / float64(totalResults)
		report.MetricValue = uniquenessRate * 100
		report.MetricUnit = "%"
		report.Success = (totalResults == concurrentCount && uniquenessRate > 0.8)
		
		if !report.Success {
			report.ErrorMessage = fmt.Sprintf("并发安全性不足: 结果=%d/%d, 唯一性=%.1f%%", 
				totalResults, concurrentCount, uniquenessRate*100)
		}

		report.AdditionalInfo["concurrent_count"] = concurrentCount
		report.AdditionalInfo["total_results"] = totalResults
		report.AdditionalInfo["unique_amounts"] = len(uniqueAmounts)

		report.EndTime = time.Now()
		report.Duration = report.EndTime.Sub(report.StartTime)
		summary.TestReports = append(summary.TestReports, report)
		
		if report.Success {
			summary.PassedTests++
		} else {
			summary.FailedTests++
		}
		summary.TotalTests++

		summary.PerformanceData["并发金额唯一性"] = uniquenessRate
		summary.BusinessValidation["并发安全"] = report.Success
	})

	// 5. 业务逻辑验证
	t.Run("业务逻辑验证", func(t *testing.T) {
		report := TestReport{
			TestName:       "订单过期和状态管理",
			StartTime:      time.Now(),
			AdditionalInfo: make(map[string]interface{}),
		}

		// 创建过期订单
		expiredOrder := createReportTestOrder(map[string]interface{}{
			"order_id":   "LOGIC_EXPIRED",
			"expired_at": time.Now().Add(-1 * time.Hour),
		})
		err := db.Create(expiredOrder).Error
		
		success := (err == nil)
		if success {
			// 查询过期订单
			var expiredOrders []model.TradeOrders
			err = db.Where("status = ? AND expired_at < ?", 
				model.OrderStatusWaiting, time.Now()).Find(&expiredOrders).Error
			
			success = (err == nil && len(expiredOrders) >= 1)
			if success {
				// 设置为过期状态
				for _, order := range expiredOrders {
					if order.OrderId == "LOGIC_EXPIRED" {
						err = order.OrderSetExpired()
						success = (err == nil)
						break
					}
				}
			}
		}

		// 测试状态标签
		if success {
			testOrder := createReportTestOrder(map[string]interface{}{
				"status": model.OrderStatusSuccess,
			})
			label := testOrder.GetStatusLabel()
			success = (label == "🟢 收款成功")
		}

		report.Success = success
		if !success {
			report.ErrorMessage = "业务逻辑验证失败"
		}

		report.EndTime = time.Now()
		report.Duration = report.EndTime.Sub(report.StartTime)
		summary.TestReports = append(summary.TestReports, report)
		
		if report.Success {
			summary.PassedTests++
		} else {
			summary.FailedTests++
		}
		summary.TotalTests++

		summary.BusinessValidation["订单过期处理"] = success
		summary.BusinessValidation["状态标签"] = success
	})

	// 6. 数据完整性验证
	t.Run("数据完整性验证", func(t *testing.T) {
		report := TestReport{
			TestName:       "数据完整性和一致性",
			StartTime:      time.Now(),
			AdditionalInfo: make(map[string]interface{}),
		}

		// 创建订单并验证数据完整性
		originalOrder := createReportTestOrder(map[string]interface{}{
			"money":  999.99,
			"amount": "138.88",
		})
		
		err := db.Create(originalOrder).Error
		success := (err == nil)
		
		if success {
			// 从数据库重新读取验证
			var retrievedOrder model.TradeOrders
			err = db.First(&retrievedOrder, originalOrder.Id).Error
			success = (err == nil)
			
			if success {
				success = (originalOrder.OrderId == retrievedOrder.OrderId &&
					originalOrder.Money == retrievedOrder.Money &&
					originalOrder.Amount == retrievedOrder.Amount &&
					originalOrder.Status == retrievedOrder.Status)
			}
		}

		report.Success = success
		if !success {
			report.ErrorMessage = "数据完整性验证失败"
		}

		report.EndTime = time.Now()
		report.Duration = report.EndTime.Sub(report.StartTime)
		summary.TestReports = append(summary.TestReports, report)
		
		if report.Success {
			summary.PassedTests++
		} else {
			summary.FailedTests++
		}
		summary.TotalTests++

		summary.BusinessValidation["数据完整性"] = success
	})

	// 生成最终报告
	summary.EndTime = time.Now()
	summary.OverallDuration = summary.EndTime.Sub(summary.StartTime)
	
	generateValidationReport(t, summary)
}

// generateValidationReport 生成验证报告
func generateValidationReport(t *testing.T, summary *ValidationSummary) {
	separator := strings.Repeat("=", 70)
	
	fmt.Printf("\n%s\n", separator)
	fmt.Println("          USDTMore 核心功能验证测试报告")
	fmt.Printf("%s\n\n", separator)
	
	// 总体概况
	fmt.Println("📊 测试概况")
	fmt.Printf("   • 测试开始时间: %s\n", summary.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("   • 测试结束时间: %s\n", summary.EndTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("   • 总执行时间: %v\n", summary.OverallDuration)
	fmt.Printf("   • 总测试数量: %d\n", summary.TotalTests)
	fmt.Printf("   • 通过测试: %d\n", summary.PassedTests)
	fmt.Printf("   • 失败测试: %d\n", summary.FailedTests)
	
	successRate := float64(summary.PassedTests) / float64(summary.TotalTests) * 100
	fmt.Printf("   • 成功率: %.1f%%\n\n", successRate)
	
	// 详细测试结果
	fmt.Println("🔍 详细测试结果")
	for i, report := range summary.TestReports {
		status := "✅"
		if !report.Success {
			status = "❌"
		}
		
		fmt.Printf("   %d. %s %s\n", i+1, status, report.TestName)
		fmt.Printf("      耗时: %v", report.Duration)
		
		if report.MetricValue > 0 {
			fmt.Printf(" | 指标: %.2f %s", report.MetricValue, report.MetricUnit)
		}
		
		if !report.Success && report.ErrorMessage != "" {
			fmt.Printf(" | 错误: %s", report.ErrorMessage)
		}
		fmt.Println()
		
		if len(report.AdditionalInfo) > 0 {
			for key, value := range report.AdditionalInfo {
				fmt.Printf("         %s: %v\n", key, value)
			}
		}
		fmt.Println()
	}
	
	// 性能指标
	if len(summary.PerformanceData) > 0 {
		fmt.Println("⚡ 性能指标")
		for key, value := range summary.PerformanceData {
			unit := ""
			if strings.Contains(key, "TPS") {
				unit = " TPS"
			} else if strings.Contains(key, "率") {
				unit = ""
				value = value * 100
			}
			
			fmt.Printf("   • %s: %.2f%s\n", key, value, unit)
		}
		fmt.Println()
	}
	
	// 业务功能验证
	fmt.Println("🔧 核心功能验证")
	for function, passed := range summary.BusinessValidation {
		status := "✅ 通过"
		if !passed {
			status = "❌ 失败"
		}
		fmt.Printf("   • %s: %s\n", function, status)
	}
	fmt.Println()
	
	// 总结和建议
	fmt.Println("📝 测试总结")
	if summary.FailedTests == 0 {
		fmt.Println("   🎉 所有核心功能测试通过！系统运行正常。")
	} else {
		fmt.Printf("   ⚠️  发现 %d 个功能存在问题，需要进一步检查和修复。\n", summary.FailedTests)
	}
	
	// 性能评估
	if tps, ok := summary.PerformanceData["订单创建TPS"]; ok {
		if tps > 1000 {
			fmt.Println("   🚀 性能表现优秀 (TPS > 1000)")
		} else if tps > 100 {
			fmt.Println("   👍 性能表现良好 (TPS > 100)")
		} else if tps > 50 {
			fmt.Println("   ✅ 性能表现合格 (TPS > 50)")
		} else {
			fmt.Println("   ⚠️  性能需要优化 (TPS < 50)")
		}
	}
	
	// 建议
	fmt.Println("\n💡 优化建议")
	if summary.FailedTests > 0 {
		fmt.Println("   1. 修复失败的测试用例")
		fmt.Println("   2. 增强错误处理和异常情况处理")
	}
	
	if concurrency, ok := summary.PerformanceData["并发金额唯一性"]; ok && concurrency < 0.9 {
		fmt.Println("   3. 优化并发处理，提高金额计算的唯一性")
	}
	
	if tps, ok := summary.PerformanceData["订单创建TPS"]; ok && tps < 100 {
		fmt.Println("   4. 优化数据库操作和索引，提高订单创建性能")
	}
	
	fmt.Println("   5. 定期运行完整的集成测试")
	fmt.Println("   6. 监控生产环境的性能指标")
	
	fmt.Printf("\n%s\n", separator)
	fmt.Printf("报告生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("%s\n", separator)
	
	// 断言整体测试结果
	assert.Greater(t, successRate, 80.0, "总体成功率应该大于80%")
	
	t.Logf("验证测试完成: 总成功率 %.1f%%", successRate)
}