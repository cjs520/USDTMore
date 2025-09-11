package tests

import (
	"USDTMore/app/config"
	"USDTMore/app/model"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestResult 测试结果结构
type TestResult struct {
	TestName    string        `json:"test_name"`
	Passed      bool          `json:"passed"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	Description string        `json:"description"`
}

// DatabaseConnectionTester 数据库连接测试器
type DatabaseConnectionTester struct {
	results []TestResult
}

// NewDatabaseConnectionTester 创建新的数据库连接测试器
func NewDatabaseConnectionTester() *DatabaseConnectionTester {
	return &DatabaseConnectionTester{
		results: make([]TestResult, 0),
	}
}

// addResult 添加测试结果
func (dct *DatabaseConnectionTester) addResult(testName, description string, passed bool, duration time.Duration, err error) {
	result := TestResult{
		TestName:    testName,
		Description: description,
		Passed:      passed,
		Duration:    duration,
	}
	if err != nil {
		result.Error = err.Error()
	}
	dct.results = append(dct.results, result)
}


// TestPostgreSQLConnection 测试PostgreSQL数据库连接
func (dct *DatabaseConnectionTester) TestPostgreSQLConnection() {
	start := time.Now()
	
	// 检查PostgreSQL环境变量
	if os.Getenv("POSTGRESQL_DSN") == "" && 
		(os.Getenv("DB_HOST") == "" || os.Getenv("DB_PASSWORD") == "") {
		dct.addResult("PostgreSQL Connection", "Check PostgreSQL environment variables", false, time.Since(start), 
			fmt.Errorf("missing PostgreSQL configuration: POSTGRESQL_DSN or DB_HOST/DB_PASSWORD"))
		return
	}
	
	// 设置临时PostgreSQL环境
	os.Setenv("DB_TYPE", "postgresql")
	
	// 获取DSN
	dsn := config.GetPostgreSQLDSN()
	if dsn == "" {
		dct.addResult("PostgreSQL Connection", "Get PostgreSQL DSN", false, time.Since(start), 
			fmt.Errorf("PostgreSQL DSN is empty"))
		return
	}
	
	// 测试PostgreSQL连接
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	
	duration := time.Since(start)
	
	if err != nil {
		dct.addResult("PostgreSQL Connection", "Test PostgreSQL database connection", false, duration, err)
		return
	}
	
	// 测试ping
	sqlDB, err := db.DB()
	if err != nil {
		dct.addResult("PostgreSQL Connection", "Get underlying SQL DB instance", false, duration, err)
		return
	}
	
	if err := sqlDB.Ping(); err != nil {
		dct.addResult("PostgreSQL Connection", "Ping PostgreSQL database", false, duration, err)
		return
	}
	
	// 测试基本操作
	if err := db.AutoMigrate(&model.WalletAddress{}, &model.TradeOrders{}, &model.NotifyRecord{}); err != nil {
		dct.addResult("PostgreSQL Connection", "Auto migrate tables", false, duration, err)
		sqlDB.Close()
		return
	}
	
	sqlDB.Close()
	dct.addResult("PostgreSQL Connection", "Complete PostgreSQL connection and migration test", true, duration, nil)
}

// TestConnectionPooling 测试连接池
func (dct *DatabaseConnectionTester) TestConnectionPooling() {
	start := time.Now()
	
	dsn := config.GetPostgreSQLDSN()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	
	if err != nil {
		dct.addResult("Connection Pooling", "Create database connection for pool testing", false, time.Since(start), err)
		return
	}
	
	sqlDB, err := db.DB()
	if err != nil {
		dct.addResult("Connection Pooling", "Get SQL DB instance for pool testing", false, time.Since(start), err)
		return
	}
	defer sqlDB.Close()
	
	// 配置连接池
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(1 * time.Minute)
	
	// 测试并发连接
	concurrency := 20
	errChan := make(chan error, concurrency)
	
	for i := 0; i < concurrency; i++ {
		go func(index int) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			if err := sqlDB.PingContext(ctx); err != nil {
				errChan <- fmt.Errorf("concurrent ping %d failed: %v", index, err)
				return
			}
			errChan <- nil
		}(i)
	}
	
	// 收集结果
	for i := 0; i < concurrency; i++ {
		if err := <-errChan; err != nil {
			dct.addResult("Connection Pooling", "Test concurrent database connections", false, time.Since(start), err)
			return
		}
	}
	
	// 检查连接池统计信息
	stats := sqlDB.Stats()
	log.Printf("Connection Pool Stats: OpenConnections=%d, InUse=%d, Idle=%d", 
		stats.OpenConnections, stats.InUse, stats.Idle)
	
	dct.addResult("Connection Pooling", "Test connection pooling with concurrent access", true, time.Since(start), nil)
}

// TestConnectionTimeout 测试连接超时
func (dct *DatabaseConnectionTester) TestConnectionTimeout() {
	start := time.Now()
	
	// 测试无效的PostgreSQL连接（超时测试）
	invalidDSN := "host=192.0.2.0 port=5432 user=invalid password=invalid dbname=invalid sslmode=disable connect_timeout=2"
	
	db, err := gorm.Open(postgres.Open(invalidDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	
	if err == nil {
		// 尝试ping，应该超时
		sqlDB, _ := db.DB()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		
		err = sqlDB.PingContext(ctx)
		if sqlDB != nil {
			sqlDB.Close()
		}
	}
	
	duration := time.Since(start)
	
	// 超时是预期的，所以这个测试应该通过
	if err != nil && (duration >= 2*time.Second && duration <= 10*time.Second) {
		dct.addResult("Connection Timeout", "Test database connection timeout handling", true, duration, nil)
	} else {
		dct.addResult("Connection Timeout", "Test database connection timeout handling", false, duration, 
			fmt.Errorf("timeout test did not behave as expected: duration=%v, err=%v", duration, err))
	}
}

// TestPostgreSQLConfiguration 测试PostgreSQL配置
func (dct *DatabaseConnectionTester) TestPostgreSQLConfiguration() {
	start := time.Now()
	
	// 测试数据库类型始终返回PostgreSQL
	if config.GetDatabaseType() != "postgresql" {
		dct.addResult("PostgreSQL Configuration", "Verify database type is PostgreSQL", false, time.Since(start), 
			fmt.Errorf("expected database type 'postgresql', got '%s'", config.GetDatabaseType()))
		return
	}
	
	// 测试连接字符串
	connStr := config.GetDatabaseConnectionString()
	if connStr == "" {
		dct.addResult("PostgreSQL Configuration", "Get database connection string", false, time.Since(start), 
			fmt.Errorf("database connection string is empty"))
		return
	}
	
	dct.addResult("PostgreSQL Configuration", "Test PostgreSQL configuration settings", true, time.Since(start), nil)
}

// RunAllTests 运行所有连接测试
func (dct *DatabaseConnectionTester) RunAllTests() {
	log.Println("Starting PostgreSQL database connection tests...")
	
	dct.TestPostgreSQLConnection()
	dct.TestConnectionPooling()
	dct.TestConnectionTimeout()
	dct.TestPostgreSQLConfiguration()
	
	log.Println("PostgreSQL database connection tests completed.")
}

// PrintResults 打印测试结果
func (dct *DatabaseConnectionTester) PrintResults() {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("DATABASE CONNECTION TEST RESULTS")
	fmt.Println(strings.Repeat("=", 80))
	
	totalTests := len(dct.results)
	passedTests := 0
	
	for _, result := range dct.results {
		status := "✅ PASS"
		if !result.Passed {
			status = "❌ FAIL"
		} else {
			passedTests++
		}
		
		fmt.Printf("%s | %-30s | %8s | %s\n", 
			status, result.TestName, result.Duration.Truncate(time.Millisecond), result.Description)
		
		if result.Error != "" {
			fmt.Printf("     Error: %s\n", result.Error)
		}
	}
	
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("SUMMARY: %d/%d tests passed (%.1f%%)\n", 
		passedTests, totalTests, float64(passedTests)/float64(totalTests)*100)
	fmt.Println(strings.Repeat("=", 80))
}

// GetResults 获取测试结果
func (dct *DatabaseConnectionTester) GetResults() []TestResult {
	return dct.results
}

// TestMain 主测试入口（用于go test）
func TestDatabaseConnections(t *testing.T) {
	tester := NewDatabaseConnectionTester()
	tester.RunAllTests()
	
	// 检查是否有失败的测试
	for _, result := range tester.GetResults() {
		if !result.Passed {
			t.Errorf("Test '%s' failed: %s", result.TestName, result.Error)
		}
	}
	
	tester.PrintResults()
}

// 独立运行的main函数
func main() {
	tester := NewDatabaseConnectionTester()
	tester.RunAllTests()
	tester.PrintResults()
}