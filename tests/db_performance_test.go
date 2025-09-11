package tests

import (
	"USDTMore/app/config"
	"USDTMore/app/model"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PerformanceResult 性能测试结果结构
type PerformanceResult struct {
	TestName        string        `json:"test_name"`
	DatabaseType    string        `json:"database_type"`
	Duration        time.Duration `json:"duration"`
	OperationsCount int           `json:"operations_count"`
	OpsPerSecond    float64       `json:"ops_per_second"`
	AvgLatency      time.Duration `json:"avg_latency"`
	MinLatency      time.Duration `json:"min_latency"`
	MaxLatency      time.Duration `json:"max_latency"`
	Success         bool          `json:"success"`
	Error           string        `json:"error,omitempty"`
	Description     string        `json:"description"`
}

// DatabasePerformanceTester 数据库性能测试器
type DatabasePerformanceTester struct {
	results []PerformanceResult
	sqliteDB *gorm.DB
	postgresDB *gorm.DB
	testDataPath string
}

// NewDatabasePerformanceTester 创建新的数据库性能测试器
func NewDatabasePerformanceTester() *DatabasePerformanceTester {
	return &DatabasePerformanceTester{
		results: make([]PerformanceResult, 0),
		testDataPath: "/tmp/performance_test.db",
	}
}

// addResult 添加测试结果
func (dpt *DatabasePerformanceTester) addResult(testName, dbType, description string, duration time.Duration, opsCount int, success bool, err error, latencies []time.Duration) {
	result := PerformanceResult{
		TestName:        testName,
		DatabaseType:    dbType,
		Description:     description,
		Duration:        duration,
		OperationsCount: opsCount,
		Success:         success,
	}
	
	if opsCount > 0 && duration > 0 {
		result.OpsPerSecond = float64(opsCount) / duration.Seconds()
	}
	
	if len(latencies) > 0 {
		var total time.Duration
		min := latencies[0]
		max := latencies[0]
		
		for _, lat := range latencies {
			total += lat
			if lat < min {
				min = lat
			}
			if lat > max {
				max = lat
			}
		}
		
		result.AvgLatency = total / time.Duration(len(latencies))
		result.MinLatency = min
		result.MaxLatency = max
	}
	
	if err != nil {
		result.Error = err.Error()
	}
	
	dpt.results = append(dpt.results, result)
}

// setupDatabases 设置测试数据库
func (dpt *DatabasePerformanceTester) setupDatabases() error {
	var err error
	
	// 清理旧的测试数据
	os.Remove(dpt.testDataPath)
	
	// 设置SQLite
	dpt.sqliteDB, err = gorm.Open(sqlite.Open(dpt.testDataPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		PrepareStmt: true, // 启用预编译语句以提升性能
	})
	if err != nil {
		return fmt.Errorf("failed to setup SQLite: %v", err)
	}
	
	// 配置SQLite性能优化
	dpt.sqliteDB.Exec("PRAGMA journal_mode = WAL")
	dpt.sqliteDB.Exec("PRAGMA synchronous = NORMAL")
	dpt.sqliteDB.Exec("PRAGMA cache_size = -64000") // 64MB cache
	dpt.sqliteDB.Exec("PRAGMA temp_store = MEMORY")
	
	// 设置SQLite连接池
	sqlDB, _ := dpt.sqliteDB.DB()
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetMaxOpenConns(1)
	
	// 自动迁移
	if err := dpt.sqliteDB.AutoMigrate(&model.WalletAddress{}, &model.TradeOrders{}, &model.NotifyRecord{}); err != nil {
		return fmt.Errorf("failed to migrate SQLite: %v", err)
	}
	
	// 设置PostgreSQL（如果可用）
	if os.Getenv("POSTGRESQL_DSN") != "" || (os.Getenv("DB_HOST") != "" && os.Getenv("DB_PASSWORD") != "") {
		dsn := config.GetPostgreSQLDSN()
		dpt.postgresDB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
			PrepareStmt: true, // 启用预编译语句以提升性能
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		if err != nil {
			log.Printf("PostgreSQL setup failed (will skip PostgreSQL tests): %v", err)
		} else {
			// 配置PostgreSQL连接池
			pgDB, _ := dpt.postgresDB.DB()
			pgDB.SetMaxIdleConns(10)
			pgDB.SetMaxOpenConns(50)
			pgDB.SetConnMaxLifetime(5 * time.Minute)
			
			// 清理测试表（如果存在）
			dpt.postgresDB.Migrator().DropTable(&model.WalletAddress{}, &model.TradeOrders{}, &model.NotifyRecord{})
			
			// 自动迁移
			if err := dpt.postgresDB.AutoMigrate(&model.WalletAddress{}, &model.TradeOrders{}, &model.NotifyRecord{}); err != nil {
				return fmt.Errorf("failed to migrate PostgreSQL: %v", err)
			}
		}
	}
	
	log.Println("Test databases setup completed")
	return nil
}

// generateWalletAddresses 生成钱包地址测试数据
func (dpt *DatabasePerformanceTester) generateWalletAddresses(count int) []model.WalletAddress {
	chains := []string{"TRON", "POLY", "BSC", "ARB", "OP"}
	wallets := make([]model.WalletAddress, count)
	
	for i := 0; i < count; i++ {
		wallets[i] = model.WalletAddress{
			Chain: chains[rand.Intn(len(chains))],
			Address: fmt.Sprintf("0x%016x%016x", rand.Int63(), rand.Int63()),
			StartBlock: int64(rand.Intn(1000000)),
			InAmount: rand.Float64() * 1000,
			OutAmount: rand.Float64() * 500,
			Count: int64(rand.Intn(100)),
			Status: 1,
			OtherNotify: rand.Intn(2),
		}
	}
	
	return wallets
}

// generateTradeOrders 生成交易订单测试数据
func (dpt *DatabasePerformanceTester) generateTradeOrders(count int) []model.TradeOrders {
	chains := []string{"TRON", "POLY", "BSC", "ARB", "OP"}
	orders := make([]model.TradeOrders, count)
	
	for i := 0; i < count; i++ {
		amount := rand.Float64() * 1000
		orders[i] = model.TradeOrders{
			Chain: chains[rand.Intn(len(chains))],
			Address: fmt.Sprintf("0x%016x%016x", rand.Int63(), rand.Int63()),
			Amount: fmt.Sprintf("%.2f", amount),
			Status: rand.Intn(3),
			TradeHash: fmt.Sprintf("hash_%016x", rand.Int63()),
			OrderId: fmt.Sprintf("order_%d_%d", i, rand.Int63()),
			TradeId: fmt.Sprintf("trade_%d_%d", i, rand.Int63()),
			UsdtRate: "7.20",
			Money: amount,
		}
	}
	
	return orders
}

// TestBulkInsert 测试批量插入性能
func (dpt *DatabasePerformanceTester) TestBulkInsert(db *gorm.DB, dbType string, batchSize int) {
	testName := "Bulk Insert"
	
	wallets := dpt.generateWalletAddresses(batchSize)
	latencies := make([]time.Duration, 0, batchSize)
	
	start := time.Now()
	
	// 批量插入
	if err := db.CreateInBatches(wallets, 100).Error; err != nil {
		dpt.addResult(testName, dbType, fmt.Sprintf("Insert %d wallet addresses in batches", batchSize), 
			time.Since(start), 0, false, err, nil)
		return
	}
	
	duration := time.Since(start)
	
	// 记录单个操作的平均延迟
	avgLatency := duration / time.Duration(batchSize)
	for i := 0; i < batchSize; i++ {
		latencies = append(latencies, avgLatency)
	}
	
	dpt.addResult(testName, dbType, fmt.Sprintf("Insert %d wallet addresses in batches", batchSize), 
		duration, batchSize, true, nil, latencies)
}

// TestSingleInsert 测试单条插入性能
func (dpt *DatabasePerformanceTester) TestSingleInsert(db *gorm.DB, dbType string, count int) {
	testName := "Single Insert"
	
	wallets := dpt.generateWalletAddresses(count)
	latencies := make([]time.Duration, 0, count)
	
	start := time.Now()
	successCount := 0
	
	for _, wallet := range wallets {
		opStart := time.Now()
		if err := db.Create(&wallet).Error; err != nil {
			latencies = append(latencies, time.Since(opStart))
			continue
		}
		latencies = append(latencies, time.Since(opStart))
		successCount++
	}
	
	duration := time.Since(start)
	
	dpt.addResult(testName, dbType, fmt.Sprintf("Insert %d wallet addresses individually", count), 
		duration, successCount, successCount == count, nil, latencies)
}

// TestQuery 测试查询性能
func (dpt *DatabasePerformanceTester) TestQuery(db *gorm.DB, dbType string, queryCount int) {
	testName := "Query Performance"
	
	// 首先插入一些测试数据
	wallets := dpt.generateWalletAddresses(1000)
	if err := db.CreateInBatches(wallets, 100).Error; err != nil {
		dpt.addResult(testName, dbType, "Setup query test data", 0, 0, false, err, nil)
		return
	}
	
	chains := []string{"TRON", "POLY", "BSC", "ARB", "OP"}
	latencies := make([]time.Duration, 0, queryCount)
	
	start := time.Now()
	successCount := 0
	
	for i := 0; i < queryCount; i++ {
		chain := chains[rand.Intn(len(chains))]
		
		opStart := time.Now()
		var result []model.WalletAddress
		if err := db.Where("chain = ? AND status = ?", chain, 1).Limit(10).Find(&result).Error; err != nil {
			latencies = append(latencies, time.Since(opStart))
			continue
		}
		latencies = append(latencies, time.Since(opStart))
		successCount++
	}
	
	duration := time.Since(start)
	
	dpt.addResult(testName, dbType, fmt.Sprintf("Execute %d queries with WHERE conditions", queryCount), 
		duration, successCount, successCount == queryCount, nil, latencies)
}

// TestUpdate 测试更新性能
func (dpt *DatabasePerformanceTester) TestUpdate(db *gorm.DB, dbType string, updateCount int) {
	testName := "Update Performance"
	
	// 首先插入一些测试数据
	wallets := dpt.generateWalletAddresses(updateCount)
	if err := db.CreateInBatches(wallets, 100).Error; err != nil {
		dpt.addResult(testName, dbType, "Setup update test data", 0, 0, false, err, nil)
		return
	}
	
	// 获取插入的记录ID
	var insertedWallets []model.WalletAddress
	if err := db.Limit(updateCount).Find(&insertedWallets).Error; err != nil {
		dpt.addResult(testName, dbType, "Retrieve records for update test", 0, 0, false, err, nil)
		return
	}
	
	latencies := make([]time.Duration, 0, len(insertedWallets))
	start := time.Now()
	successCount := 0
	
	for _, wallet := range insertedWallets {
		newAmount := rand.Float64() * 1000
		
		opStart := time.Now()
		if err := db.Model(&wallet).Update("in_amount", newAmount).Error; err != nil {
			latencies = append(latencies, time.Since(opStart))
			continue
		}
		latencies = append(latencies, time.Since(opStart))
		successCount++
	}
	
	duration := time.Since(start)
	
	dpt.addResult(testName, dbType, fmt.Sprintf("Update %d records individually", len(insertedWallets)), 
		duration, successCount, successCount == len(insertedWallets), nil, latencies)
}

// TestDelete 测试删除性能
func (dpt *DatabasePerformanceTester) TestDelete(db *gorm.DB, dbType string, deleteCount int) {
	testName := "Delete Performance"
	
	// 首先插入一些测试数据
	wallets := dpt.generateWalletAddresses(deleteCount)
	if err := db.CreateInBatches(wallets, 100).Error; err != nil {
		dpt.addResult(testName, dbType, "Setup delete test data", 0, 0, false, err, nil)
		return
	}
	
	// 获取插入的记录ID
	var insertedWallets []model.WalletAddress
	if err := db.Limit(deleteCount).Find(&insertedWallets).Error; err != nil {
		dpt.addResult(testName, dbType, "Retrieve records for delete test", 0, 0, false, err, nil)
		return
	}
	
	latencies := make([]time.Duration, 0, len(insertedWallets))
	start := time.Now()
	successCount := 0
	
	for _, wallet := range insertedWallets {
		opStart := time.Now()
		if err := db.Delete(&wallet).Error; err != nil {
			latencies = append(latencies, time.Since(opStart))
			continue
		}
		latencies = append(latencies, time.Since(opStart))
		successCount++
	}
	
	duration := time.Since(start)
	
	dpt.addResult(testName, dbType, fmt.Sprintf("Delete %d records individually", len(insertedWallets)), 
		duration, successCount, successCount == len(insertedWallets), nil, latencies)
}

// TestConcurrentOperations 测试并发操作性能
func (dpt *DatabasePerformanceTester) TestConcurrentOperations(db *gorm.DB, dbType string, concurrency int, opsPerWorker int) {
	testName := "Concurrent Operations"
	
	// 准备测试数据
	baseWallets := dpt.generateWalletAddresses(1000)
	if err := db.CreateInBatches(baseWallets, 100).Error; err != nil {
		dpt.addResult(testName, dbType, "Setup concurrent test data", 0, 0, false, err, nil)
		return
	}
	
	var wg sync.WaitGroup
	latencyChan := make(chan time.Duration, concurrency*opsPerWorker)
	errorChan := make(chan error, concurrency*opsPerWorker)
	
	start := time.Now()
	
	// 启动并发工作者
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < opsPerWorker; j++ {
				opStart := time.Now()
				
				// 随机执行不同类型的操作
				switch rand.Intn(4) {
				case 0: // Insert
					wallet := model.WalletAddress{
						Chain: "TEST",
						Address: fmt.Sprintf("worker_%d_op_%d_%016x", workerID, j, rand.Int63()),
						Status: 1,
					}
					err := db.Create(&wallet).Error
					latencyChan <- time.Since(opStart)
					if err != nil {
						errorChan <- err
					}
					
				case 1: // Query
					var wallets []model.WalletAddress
					err := db.Where("chain = ?", "TRON").Limit(5).Find(&wallets).Error
					latencyChan <- time.Since(opStart)
					if err != nil {
						errorChan <- err
					}
					
				case 2: // Update
					err := db.Model(&model.WalletAddress{}).Where("chain = ?", "POLY").
						Update("in_amount", rand.Float64()*100).Error
					latencyChan <- time.Since(opStart)
					if err != nil {
						errorChan <- err
					}
					
				case 3: // Count
					var count int64
					err := db.Model(&model.WalletAddress{}).Where("status = ?", 1).Count(&count).Error
					latencyChan <- time.Since(opStart)
					if err != nil {
						errorChan <- err
					}
				}
			}
		}(i)
	}
	
	wg.Wait()
	close(latencyChan)
	close(errorChan)
	
	duration := time.Since(start)
	
	// 收集延迟数据
	latencies := make([]time.Duration, 0)
	for lat := range latencyChan {
		latencies = append(latencies, lat)
	}
	
	// 收集错误
	errors := make([]error, 0)
	for err := range errorChan {
		errors = append(errors, err)
	}
	
	totalOps := concurrency * opsPerWorker
	successOps := totalOps - len(errors)
	
	var combinedErr error
	if len(errors) > 0 {
		combinedErr = fmt.Errorf("encountered %d errors during concurrent operations", len(errors))
	}
	
	dpt.addResult(testName, dbType, fmt.Sprintf("%d workers × %d ops/worker = %d total ops", concurrency, opsPerWorker, totalOps), 
		duration, successOps, len(errors) == 0, combinedErr, latencies)
}

// TestTransaction 测试事务性能
func (dpt *DatabasePerformanceTester) TestTransaction(db *gorm.DB, dbType string, txCount int, opsPerTx int) {
	testName := "Transaction Performance"
	
	latencies := make([]time.Duration, 0, txCount)
	start := time.Now()
	successCount := 0
	
	for i := 0; i < txCount; i++ {
		txStart := time.Now()
		
		err := db.Transaction(func(tx *gorm.DB) error {
			for j := 0; j < opsPerTx; j++ {
				wallet := model.WalletAddress{
					Chain: "TX_TEST",
					Address: fmt.Sprintf("tx_%d_op_%d_%016x", i, j, rand.Int63()),
					Status: 1,
				}
				if err := tx.Create(&wallet).Error; err != nil {
					return err
				}
			}
			return nil
		})
		
		latencies = append(latencies, time.Since(txStart))
		
		if err == nil {
			successCount++
		}
	}
	
	duration := time.Since(start)
	totalOps := txCount * opsPerTx
	
	dpt.addResult(testName, dbType, fmt.Sprintf("%d transactions × %d ops/tx = %d total ops", txCount, opsPerTx, totalOps), 
		duration, successCount*opsPerTx, successCount == txCount, nil, latencies)
}

// RunPerformanceTests 运行性能测试
func (dpt *DatabasePerformanceTester) runDatabaseTests(db *gorm.DB, dbType string) {
	log.Printf("Running performance tests for %s...", dbType)
	
	// 基础CRUD操作测试
	dpt.TestBulkInsert(db, dbType, 1000)
	dpt.TestSingleInsert(db, dbType, 100)
	dpt.TestQuery(db, dbType, 500)
	dpt.TestUpdate(db, dbType, 200)
	dpt.TestDelete(db, dbType, 200)
	
	// 并发测试
	dpt.TestConcurrentOperations(db, dbType, 10, 50)
	
	// 事务测试
	dpt.TestTransaction(db, dbType, 50, 10)
	
	log.Printf("Completed performance tests for %s", dbType)
}

// RunAllTests 运行所有性能测试
func (dpt *DatabasePerformanceTester) RunAllTests() {
	log.Println("Starting database performance tests...")
	
	if err := dpt.setupDatabases(); err != nil {
		log.Printf("Failed to setup databases: %v", err)
		return
	}
	
	defer func() {
		os.Remove(dpt.testDataPath) // 清理SQLite测试文件
		
		if dpt.sqliteDB != nil {
			if sqlDB, err := dpt.sqliteDB.DB(); err == nil {
				sqlDB.Close()
			}
		}
		
		if dpt.postgresDB != nil {
			if sqlDB, err := dpt.postgresDB.DB(); err == nil {
				sqlDB.Close()
			}
		}
	}()
	
	// 测试SQLite
	if dpt.sqliteDB != nil {
		dpt.runDatabaseTests(dpt.sqliteDB, "SQLite")
	}
	
	// 测试PostgreSQL（如果可用）
	if dpt.postgresDB != nil {
		dpt.runDatabaseTests(dpt.postgresDB, "PostgreSQL")
	}
	
	log.Println("Database performance tests completed.")
}

// PrintResults 打印性能测试结果
func (dpt *DatabasePerformanceTester) PrintResults() {
	fmt.Println("\n" + strings.Repeat("=", 120))
	fmt.Println("DATABASE PERFORMANCE TEST RESULTS")
	fmt.Println(strings.Repeat("=", 120))
	
	// 按数据库类型分组结果
	sqliteResults := make([]PerformanceResult, 0)
	postgresResults := make([]PerformanceResult, 0)
	
	for _, result := range dpt.results {
		if result.DatabaseType == "SQLite" {
			sqliteResults = append(sqliteResults, result)
		} else {
			postgresResults = append(postgresResults, result)
		}
	}
	
	// 打印SQLite结果
	if len(sqliteResults) > 0 {
		fmt.Println("\nSQLite Performance:")
		fmt.Println(strings.Repeat("-", 60))
		dpt.printResultGroup(sqliteResults)
	}
	
	// 打印PostgreSQL结果
	if len(postgresResults) > 0 {
		fmt.Println("\nPostgreSQL Performance:")
		fmt.Println(strings.Repeat("-", 60))
		dpt.printResultGroup(postgresResults)
	}
	
	// 打印对比
	if len(sqliteResults) > 0 && len(postgresResults) > 0 {
		fmt.Println("\nPerformance Comparison (PostgreSQL vs SQLite):")
		fmt.Println(strings.Repeat("-", 80))
		dpt.printComparison(sqliteResults, postgresResults)
	}
	
	fmt.Println(strings.Repeat("=", 120))
}

// printResultGroup 打印结果组
func (dpt *DatabasePerformanceTester) printResultGroup(results []PerformanceResult) {
	fmt.Printf("%-25s | %8s | %10s | %12s | %12s | %12s | %s\n", 
		"Test", "Success", "Ops/Sec", "Avg Latency", "Min Latency", "Max Latency", "Description")
	fmt.Println(strings.Repeat("-", 120))
	
	for _, result := range results {
		status := "✅"
		if !result.Success {
			status = "❌"
		}
		
		opsPerSec := fmt.Sprintf("%.1f", result.OpsPerSecond)
		avgLat := result.AvgLatency.String()
		minLat := result.MinLatency.String()
		maxLat := result.MaxLatency.String()
		
		if result.AvgLatency == 0 {
			avgLat = "N/A"
		}
		if result.MinLatency == 0 {
			minLat = "N/A"
		}
		if result.MaxLatency == 0 {
			maxLat = "N/A"
		}
		
		fmt.Printf("%-25s | %8s | %10s | %12s | %12s | %12s | %s\n", 
			result.TestName, status, opsPerSec, avgLat, minLat, maxLat, result.Description)
	}
}

// printComparison 打印性能对比
func (dpt *DatabasePerformanceTester) printComparison(sqliteResults, postgresResults []PerformanceResult) {
	fmt.Printf("%-25s | %15s | %15s | %15s\n", "Test", "SQLite (ops/s)", "PostgreSQL (ops/s)", "Ratio (PG/SQLite)")
	fmt.Println(strings.Repeat("-", 80))
	
	resultMap := make(map[string]PerformanceResult)
	for _, result := range sqliteResults {
		resultMap["sqlite_"+result.TestName] = result
	}
	for _, result := range postgresResults {
		resultMap["postgres_"+result.TestName] = result
	}
	
	testNames := make(map[string]bool)
	for _, result := range sqliteResults {
		testNames[result.TestName] = true
	}
	
	for testName := range testNames {
		sqliteKey := "sqlite_" + testName
		postgresKey := "postgres_" + testName
		
		sqliteResult, sqliteOk := resultMap[sqliteKey]
		postgresResult, postgresOk := resultMap[postgresKey]
		
		if sqliteOk && postgresOk {
			ratio := "N/A"
			if sqliteResult.OpsPerSecond > 0 {
				ratio = fmt.Sprintf("%.2fx", postgresResult.OpsPerSecond/sqliteResult.OpsPerSecond)
			}
			
			fmt.Printf("%-25s | %15.1f | %15.1f | %15s\n", 
				testName, sqliteResult.OpsPerSecond, postgresResult.OpsPerSecond, ratio)
		}
	}
}

// GetResults 获取测试结果
func (dpt *DatabasePerformanceTester) GetResults() []PerformanceResult {
	return dpt.results
}

// TestMain 主测试入口（用于go test）
func TestDatabasePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}
	
	tester := NewDatabasePerformanceTester()
	tester.RunAllTests()
	
	// 打印结果（测试时不检查具体性能指标，因为它们依赖于硬件）
	tester.PrintResults()
	
	// 仅检查是否有严重错误
	for _, result := range tester.GetResults() {
		if !result.Success && result.Error != "" {
			t.Logf("Performance test '%s' on %s had issues: %s", 
				result.TestName, result.DatabaseType, result.Error)
		}
	}
}

// 独立运行的main函数
func main() {
	rand.Seed(time.Now().UnixNano())
	tester := NewDatabasePerformanceTester()
	tester.RunAllTests()
	tester.PrintResults()
}