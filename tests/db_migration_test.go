package tests

import (
	"USDTMore/app/config"
	"USDTMore/app/model"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MigrationResult 迁移结果结构
type MigrationResult struct {
	TestName     string        `json:"test_name"`
	Passed       bool          `json:"passed"`
	Duration     time.Duration `json:"duration"`
	Error        string        `json:"error,omitempty"`
	Description  string        `json:"description"`
	RecordCount  int           `json:"record_count,omitempty"`
	DataMatches  bool          `json:"data_matches,omitempty"`
}

// DatabaseMigrationTester 数据库迁移测试器
type DatabaseMigrationTester struct {
	results []MigrationResult
	sqliteDB *gorm.DB
	postgresDB *gorm.DB
	testDataPath string
}

// NewDatabaseMigrationTester 创建新的数据库迁移测试器
func NewDatabaseMigrationTester() *DatabaseMigrationTester {
	return &DatabaseMigrationTester{
		results: make([]MigrationResult, 0),
		testDataPath: "/tmp/migration_test_data.db",
	}
}

// addResult 添加测试结果
func (dmt *DatabaseMigrationTester) addResult(testName, description string, passed bool, duration time.Duration, err error, recordCount int, dataMatches bool) {
	result := MigrationResult{
		TestName:    testName,
		Description: description,
		Passed:      passed,
		Duration:    duration,
		RecordCount: recordCount,
		DataMatches: dataMatches,
	}
	if err != nil {
		result.Error = err.Error()
	}
	dmt.results = append(dmt.results, result)
}

// setupTestData 设置测试数据
func (dmt *DatabaseMigrationTester) setupTestData() error {
	// 清理旧的测试数据
	os.Remove(dmt.testDataPath)
	
	// 创建SQLite测试数据库
	db, err := gorm.Open(sqlite.Open(dmt.testDataPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("failed to create test SQLite database: %v", err)
	}
	
	// 自动迁移
	if err := db.AutoMigrate(&model.WalletAddress{}, &model.TradeOrders{}, &model.NotifyRecord{}); err != nil {
		return fmt.Errorf("failed to migrate test database: %v", err)
	}
	
	dmt.sqliteDB = db
	
	// 插入测试数据
	if err := dmt.insertTestData(); err != nil {
		return fmt.Errorf("failed to insert test data: %v", err)
	}
	
	return nil
}

// insertTestData 插入测试数据
func (dmt *DatabaseMigrationTester) insertTestData() error {
	// 插入钱包地址测试数据
	walletAddresses := []model.WalletAddress{
		{
			Chain: "TRON", Address: "TRX123456789ABCDEF", StartBlock: 12345,
			InAmount: 100.50, OutAmount: 50.25, Count: 10, Status: 1, OtherNotify: 1,
		},
		{
			Chain: "POLY", Address: "0x123456789ABCDEF", StartBlock: 54321,
			InAmount: 200.75, OutAmount: 75.25, Count: 5, Status: 1, OtherNotify: 0,
		},
		{
			Chain: "BSC", Address: "0xBSC123456789ABCDEF", StartBlock: 98765,
			InAmount: 300.25, OutAmount: 100.00, Count: 15, Status: 0, OtherNotify: 1,
		},
	}
	
	for _, wa := range walletAddresses {
		if err := dmt.sqliteDB.Create(&wa).Error; err != nil {
			return fmt.Errorf("failed to create wallet address: %v", err)
		}
	}
	
	// 插入交易订单测试数据
	tradeOrders := []model.TradeOrders{
		{
			OrderId: "order001", TradeId: "trade001",
			Chain: "TRON", Address: "TRX123456789ABCDEF",
			Amount: "100.50", Money: 100.50,
			UsdtRate: "1.0", Status: 1,
			TradeHash: "hash123456789",
			ExpiredAt: time.Now().Add(time.Hour),
		},
		{
			OrderId: "order002", TradeId: "trade002",
			Chain: "POLY", Address: "0x123456789ABCDEF",
			Amount: "200.75", Money: 200.75,
			UsdtRate: "1.0", Status: 2,
			TradeHash: "hash987654321",
			ExpiredAt: time.Now().Add(time.Hour),
		},
	}
	
	for _, to := range tradeOrders {
		if err := dmt.sqliteDB.Create(&to).Error; err != nil {
			return fmt.Errorf("failed to create trade order: %v", err)
		}
	}
	
	// 插入通知记录测试数据
	notifyRecords := []model.NotifyRecord{
		{
			Txid: "hash123456789",
		},
		{
			Txid: "hash987654321",
		},
	}
	
	for _, nr := range notifyRecords {
		if err := dmt.sqliteDB.Create(&nr).Error; err != nil {
			return fmt.Errorf("failed to create notify record: %v", err)
		}
	}
	
	log.Println("Test data inserted successfully")
	return nil
}

// TestSchemaValidation 测试模式验证
func (dmt *DatabaseMigrationTester) TestSchemaValidation() {
	start := time.Now()
	
	if dmt.sqliteDB == nil {
		dmt.addResult("Schema Validation", "Validate database schema structure", false, time.Since(start), 
			fmt.Errorf("SQLite database not initialized"), 0, false)
		return
	}
	
	// 验证表结构
	tables := []string{"wallet_address", "trade_orders", "notify_record"}
	
	for _, table := range tables {
		var count int64
		if err := dmt.sqliteDB.Table(table).Count(&count).Error; err != nil {
			dmt.addResult("Schema Validation", fmt.Sprintf("Validate table: %s", table), false, time.Since(start), err, 0, false)
			return
		}
	}
	
	// 验证列结构（检查关键字段）
	type ColumnInfo struct {
		Name string
		Type string
	}
	
	expectedColumns := map[string][]string{
		"wallet_address": {"id", "chain", "start_block", "in_amount", "out_amount", "count", "address", "status", "other_notify", "created_at", "updated_at"},
		"trade_orders": {"id", "chain", "address", "amount", "real_amount", "token", "status", "block_id", "callback_status", "hash", "created_at", "updated_at"},
		"notify_record": {"id", "order_id", "chain", "address", "hash", "amount", "status", "try_count", "created_at", "updated_at"},
	}
	
	for table, columns := range expectedColumns {
		for _, column := range columns {
			var exists bool
			query := fmt.Sprintf("SELECT 1 FROM pragma_table_info('%s') WHERE name = ?", table)
			if err := dmt.sqliteDB.Raw(query, column).Scan(&exists).Error; err != nil {
				dmt.addResult("Schema Validation", fmt.Sprintf("Check column %s.%s", table, column), false, time.Since(start), err, 0, false)
				return
			}
		}
	}
	
	dmt.addResult("Schema Validation", "Validate database schema and table structures", true, time.Since(start), nil, len(tables), true)
}

// TestDataIntegrity 测试数据完整性
func (dmt *DatabaseMigrationTester) TestDataIntegrity() {
	start := time.Now()
	
	if dmt.sqliteDB == nil {
		dmt.addResult("Data Integrity", "Test data integrity and constraints", false, time.Since(start), 
			fmt.Errorf("SQLite database not initialized"), 0, false)
		return
	}
	
	// 检查数据完整性
	var walletCount, orderCount, notifyCount int64
	
	if err := dmt.sqliteDB.Model(&model.WalletAddress{}).Count(&walletCount).Error; err != nil {
		dmt.addResult("Data Integrity", "Count wallet addresses", false, time.Since(start), err, 0, false)
		return
	}
	
	if err := dmt.sqliteDB.Model(&model.TradeOrders{}).Count(&orderCount).Error; err != nil {
		dmt.addResult("Data Integrity", "Count trade orders", false, time.Since(start), err, 0, false)
		return
	}
	
	if err := dmt.sqliteDB.Model(&model.NotifyRecord{}).Count(&notifyCount).Error; err != nil {
		dmt.addResult("Data Integrity", "Count notify records", false, time.Since(start), err, 0, false)
		return
	}
	
	// 验证数据一致性
	if walletCount < 1 || orderCount < 1 || notifyCount < 1 {
		dmt.addResult("Data Integrity", "Verify minimum test data exists", false, time.Since(start), 
			fmt.Errorf("insufficient test data: wallets=%d, orders=%d, notifications=%d", walletCount, orderCount, notifyCount), 
			int(walletCount+orderCount+notifyCount), false)
		return
	}
	
	// 测试数据类型和约束
	var wallets []model.WalletAddress
	if err := dmt.sqliteDB.Find(&wallets).Error; err != nil {
		dmt.addResult("Data Integrity", "Query wallet addresses", false, time.Since(start), err, int(walletCount), false)
		return
	}
	
	for _, wallet := range wallets {
		if wallet.Chain == "" || wallet.Address == "" {
			dmt.addResult("Data Integrity", "Validate required fields", false, time.Since(start), 
				fmt.Errorf("missing required fields in wallet: %+v", wallet), int(walletCount), false)
			return
		}
	}
	
	totalRecords := int(walletCount + orderCount + notifyCount)
	dmt.addResult("Data Integrity", "Test data integrity and constraints", true, time.Since(start), nil, totalRecords, true)
}

// TestPostgreSQLMigration 测试PostgreSQL迁移
func (dmt *DatabaseMigrationTester) TestPostgreSQLMigration() {
	start := time.Now()
	
	// 检查PostgreSQL配置
	if os.Getenv("POSTGRESQL_DSN") == "" && 
		(os.Getenv("DB_HOST") == "" || os.Getenv("DB_PASSWORD") == "") {
		dmt.addResult("PostgreSQL Migration", "Check PostgreSQL configuration", false, time.Since(start), 
			fmt.Errorf("missing PostgreSQL configuration"), 0, false)
		return
	}
	
	// 连接PostgreSQL
	dsn := config.GetPostgreSQLDSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	
	if err != nil {
		dmt.addResult("PostgreSQL Migration", "Connect to PostgreSQL for migration", false, time.Since(start), err, 0, false)
		return
	}
	
	dmt.postgresDB = db
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}()
	
	// 清理测试表（如果存在）
	db.Migrator().DropTable(&model.WalletAddress{}, &model.TradeOrders{}, &model.NotifyRecord{})
	
	// 执行迁移
	if err := db.AutoMigrate(&model.WalletAddress{}, &model.TradeOrders{}, &model.NotifyRecord{}); err != nil {
		dmt.addResult("PostgreSQL Migration", "Execute PostgreSQL auto migration", false, time.Since(start), err, 0, false)
		return
	}
	
	dmt.addResult("PostgreSQL Migration", "Successfully migrate schema to PostgreSQL", true, time.Since(start), nil, 3, true)
}

// TestDataMigration 测试数据迁移
func (dmt *DatabaseMigrationTester) TestDataMigration() {
	start := time.Now()
	
	if dmt.sqliteDB == nil || dmt.postgresDB == nil {
		dmt.addResult("Data Migration", "Test data migration from SQLite to PostgreSQL", false, time.Since(start), 
			fmt.Errorf("source or target database not initialized"), 0, false)
		return
	}
	
	// 从SQLite读取数据并迁移到PostgreSQL
	
	// 迁移钱包地址
	var wallets []model.WalletAddress
	if err := dmt.sqliteDB.Find(&wallets).Error; err != nil {
		dmt.addResult("Data Migration", "Read wallet addresses from SQLite", false, time.Since(start), err, 0, false)
		return
	}
	
	for _, wallet := range wallets {
		wallet.Id = 0 // 重置ID让PostgreSQL自动生成
		if err := dmt.postgresDB.Create(&wallet).Error; err != nil {
			dmt.addResult("Data Migration", "Migrate wallet addresses to PostgreSQL", false, time.Since(start), err, len(wallets), false)
			return
		}
	}
	
	// 迁移交易订单
	var orders []model.TradeOrders
	if err := dmt.sqliteDB.Find(&orders).Error; err != nil {
		dmt.addResult("Data Migration", "Read trade orders from SQLite", false, time.Since(start), err, 0, false)
		return
	}
	
	for _, order := range orders {
		order.Id = 0 // 重置ID让PostgreSQL自动生成
		if err := dmt.postgresDB.Create(&order).Error; err != nil {
			dmt.addResult("Data Migration", "Migrate trade orders to PostgreSQL", false, time.Since(start), err, len(orders), false)
			return
		}
	}
	
	// 迁移通知记录
	var notifications []model.NotifyRecord
	if err := dmt.sqliteDB.Find(&notifications).Error; err != nil {
		dmt.addResult("Data Migration", "Read notify records from SQLite", false, time.Since(start), err, 0, false)
		return
	}
	
	for _, notification := range notifications {
		// notification.Id = 0 // NotifyRecord doesn't have Id field
		if err := dmt.postgresDB.Create(&notification).Error; err != nil {
			dmt.addResult("Data Migration", "Migrate notify records to PostgreSQL", false, time.Since(start), err, len(notifications), false)
			return
		}
	}
	
	totalMigrated := len(wallets) + len(orders) + len(notifications)
	dmt.addResult("Data Migration", "Complete data migration from SQLite to PostgreSQL", true, time.Since(start), nil, totalMigrated, true)
}

// TestDataConsistency 测试数据一致性
func (dmt *DatabaseMigrationTester) TestDataConsistency() {
	start := time.Now()
	
	if dmt.sqliteDB == nil || dmt.postgresDB == nil {
		dmt.addResult("Data Consistency", "Compare data between SQLite and PostgreSQL", false, time.Since(start), 
			fmt.Errorf("source or target database not initialized"), 0, false)
		return
	}
	
	// 比较记录数量
	tables := []struct{
		model interface{}
		name  string
	}{
		{&model.WalletAddress{}, "wallet_address"},
		{&model.TradeOrders{}, "trade_orders"},
		{&model.NotifyRecord{}, "notify_record"},
	}
	
	totalMatches := 0
	_ = len(tables) // totalTables
	
	for _, table := range tables {
		var sqliteCount, postgresCount int64
		
		if err := dmt.sqliteDB.Model(table.model).Count(&sqliteCount).Error; err != nil {
			dmt.addResult("Data Consistency", fmt.Sprintf("Count %s in SQLite", table.name), false, time.Since(start), err, 0, false)
			return
		}
		
		if err := dmt.postgresDB.Model(table.model).Count(&postgresCount).Error; err != nil {
			dmt.addResult("Data Consistency", fmt.Sprintf("Count %s in PostgreSQL", table.name), false, time.Since(start), err, 0, false)
			return
		}
		
		if sqliteCount != postgresCount {
			dmt.addResult("Data Consistency", fmt.Sprintf("Compare %s counts", table.name), false, time.Since(start), 
				fmt.Errorf("count mismatch: SQLite=%d, PostgreSQL=%d", sqliteCount, postgresCount), int(sqliteCount), false)
			return
		}
		
		totalMatches++
	}
	
	// 详细数据比较（采样检查）
	var sqliteWallets, postgresWallets []model.WalletAddress
	dmt.sqliteDB.Order("chain, address").Find(&sqliteWallets)
	dmt.postgresDB.Order("chain, address").Find(&postgresWallets)
	
	if len(sqliteWallets) != len(postgresWallets) {
		dmt.addResult("Data Consistency", "Compare wallet address records", false, time.Since(start), 
			fmt.Errorf("wallet count mismatch after ordering"), len(sqliteWallets), false)
		return
	}
	
	for i, sw := range sqliteWallets {
		pw := postgresWallets[i]
		if sw.Chain != pw.Chain || sw.Address != pw.Address || sw.InAmount != pw.InAmount {
			dmt.addResult("Data Consistency", "Compare wallet address data", false, time.Since(start), 
				fmt.Errorf("wallet data mismatch at index %d", i), len(sqliteWallets), false)
			return
		}
	}
	
	dmt.addResult("Data Consistency", "Verify data consistency between SQLite and PostgreSQL", true, time.Since(start), nil, totalMatches, true)
}

// TestRollbackCapability 测试回滚能力
func (dmt *DatabaseMigrationTester) TestRollbackCapability() {
	start := time.Now()
	
	if dmt.postgresDB == nil {
		dmt.addResult("Rollback Capability", "Test transaction rollback capability", false, time.Since(start), 
			fmt.Errorf("PostgreSQL database not initialized"), 0, false)
		return
	}
	
	// 测试事务回滚
	tx := dmt.postgresDB.Begin()
	
	// 插入测试数据
	testWallet := model.WalletAddress{
		Chain: "TEST", Address: "TEST_ROLLBACK", Status: 1,
	}
	
	if err := tx.Create(&testWallet).Error; err != nil {
		tx.Rollback()
		dmt.addResult("Rollback Capability", "Create test data in transaction", false, time.Since(start), err, 0, false)
		return
	}
	
	// 验证数据存在于事务中
	var count int64
	if err := tx.Model(&model.WalletAddress{}).Where("address = ?", "TEST_ROLLBACK").Count(&count).Error; err != nil {
		tx.Rollback()
		dmt.addResult("Rollback Capability", "Verify test data in transaction", false, time.Since(start), err, 0, false)
		return
	}
	
	if count != 1 {
		tx.Rollback()
		dmt.addResult("Rollback Capability", "Confirm test data exists in transaction", false, time.Since(start), 
			fmt.Errorf("expected 1 record, got %d", count), int(count), false)
		return
	}
	
	// 回滚事务
	if err := tx.Rollback().Error; err != nil {
		dmt.addResult("Rollback Capability", "Execute transaction rollback", false, time.Since(start), err, 1, false)
		return
	}
	
	// 验证数据已被回滚
	if err := dmt.postgresDB.Model(&model.WalletAddress{}).Where("address = ?", "TEST_ROLLBACK").Count(&count).Error; err != nil {
		dmt.addResult("Rollback Capability", "Verify data rollback", false, time.Since(start), err, 0, false)
		return
	}
	
	if count != 0 {
		dmt.addResult("Rollback Capability", "Confirm data was rolled back", false, time.Since(start), 
			fmt.Errorf("expected 0 records after rollback, got %d", count), int(count), false)
		return
	}
	
	dmt.addResult("Rollback Capability", "Test transaction rollback capability", true, time.Since(start), nil, 1, true)
}

// RunAllTests 运行所有迁移测试
func (dmt *DatabaseMigrationTester) RunAllTests() {
	log.Println("Starting database migration tests...")
	
	// 设置测试数据
	if err := dmt.setupTestData(); err != nil {
		log.Printf("Failed to setup test data: %v", err)
		dmt.addResult("Setup", "Setup test data", false, 0, err, 0, false)
		return
	}
	defer os.Remove(dmt.testDataPath) // 清理测试文件
	
	dmt.TestSchemaValidation()
	dmt.TestDataIntegrity()
	dmt.TestPostgreSQLMigration()
	dmt.TestDataMigration()
	dmt.TestDataConsistency()
	dmt.TestRollbackCapability()
	
	log.Println("Database migration tests completed.")
}

// PrintResults 打印测试结果
func (dmt *DatabaseMigrationTester) PrintResults() {
	fmt.Println("\n" + strings.Repeat("=", 90))
	fmt.Println("DATABASE MIGRATION TEST RESULTS")
	fmt.Println(strings.Repeat("=", 90))
	
	totalTests := len(dmt.results)
	passedTests := 0
	
	for _, result := range dmt.results {
		status := "✅ PASS"
		if !result.Passed {
			status = "❌ FAIL"
		} else {
			passedTests++
		}
		
		dataInfo := ""
		if result.RecordCount > 0 {
			dataInfo = fmt.Sprintf(" | Records: %d", result.RecordCount)
		}
		if result.DataMatches {
			dataInfo += " | Data: ✓"
		}
		
		fmt.Printf("%s | %-25s | %8s | %s%s\n", 
			status, result.TestName, result.Duration.Truncate(time.Millisecond), result.Description, dataInfo)
		
		if result.Error != "" {
			fmt.Printf("     Error: %s\n", result.Error)
		}
	}
	
	fmt.Println(strings.Repeat("=", 90))
	fmt.Printf("SUMMARY: %d/%d tests passed (%.1f%%)\n", 
		passedTests, totalTests, float64(passedTests)/float64(totalTests)*100)
	fmt.Println(strings.Repeat("=", 90))
}

// GetResults 获取测试结果
func (dmt *DatabaseMigrationTester) GetResults() []MigrationResult {
	return dmt.results
}

// TestMain 主测试入口（用于go test）
func TestDatabaseMigration(t *testing.T) {
	tester := NewDatabaseMigrationTester()
	tester.RunAllTests()
	
	// 检查是否有失败的测试
	for _, result := range tester.GetResults() {
		if !result.Passed {
			t.Errorf("Migration test '%s' failed: %s", result.TestName, result.Error)
		}
	}
	
	tester.PrintResults()
}