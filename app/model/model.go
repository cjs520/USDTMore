package model

import (
	"USDTMore/app/config"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
	
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB
var _err error

// 数据库连接重试配置
const (
	maxRetries = 5
	baseDelay  = 1 * time.Second
	maxDelay   = 30 * time.Second
)

// 数据库迁移状态管理
var (
	migrationMutex    sync.RWMutex
	migrationComplete bool
	migrationLockFile = "/tmp/usdtmore_migration.lock"
)

func Init() error {
	var db *gorm.DB
	var err error
	
	// 初始化PostgreSQL数据库
	db, err = initPostgreSQLWithRetry()
	if err != nil {
		return fmt.Errorf("PostgreSQL initialization failed: %v", err)
	}
	
	DB = db
	
	// 执行数据库迁移
	if err = AutoMigrate(); err != nil {
		return fmt.Errorf("database migration failed: %v", err)
	}

	// 添加初始钱包地址
	addStartWalletAddress()

	log.Printf("PostgreSQL database initialized successfully")
	return nil
}

// initPostgreSQLWithRetry 带重试机制的PostgreSQL初始化
func initPostgreSQLWithRetry() (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	
	for i := 0; i < maxRetries; i++ {
		db, err = initPostgreSQL()
		if err == nil {
			// 测试数据库连接
			if sqlDB, dbErr := db.DB(); dbErr == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					return db, nil
				} else {
					err = pingErr
				}
			} else {
				err = dbErr
			}
		}
		
		if i < maxRetries-1 {
			delay := time.Duration(i+1) * baseDelay
			if delay > maxDelay {
				delay = maxDelay
			}
			log.Printf("PostgreSQL connection attempt %d failed: %v. Retrying in %v...", i+1, err, delay)
			time.Sleep(delay)
		}
	}
	
	return nil, fmt.Errorf("failed to connect to PostgreSQL after %d attempts: %v", maxRetries, err)
}


// initPostgreSQL 初始化PostgreSQL连接
func initPostgreSQL() (*gorm.DB, error) {
	dsn := config.GetPostgreSQLDSN()
	if dsn == "" {
		return nil, fmt.Errorf("PostgreSQL DSN is empty")
	}
	
	// 配置GORM日志级别
	logLevel := logger.Warn
	if config.GetDbDebug() {
		logLevel = logger.Info
	}
	
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().Local()
		},
		// 禁用外键约束检查（可选）
		DisableForeignKeyConstraintWhenMigrating: true,
	}
	
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open PostgreSQL connection: %v", err)
	}
	
	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %v", err)
	}
	
	// 设置连接池参数 - 基于高并发优化
	maxIdleConns := config.GetDbMaxIdleConns()
	maxOpenConns := config.GetDbMaxOpenConns()
	connMaxLifetime := config.GetDbConnMaxLifetime()
	connMaxIdleTime := config.GetDbConnMaxIdleTime()
	
	// 动态调整连接池参数（基于并发测试结果）
	if maxOpenConns < 50 {
		maxOpenConns = 50 // 高并发场景最小值
		log.Printf("Adjusting MaxOpenConns to %d for high concurrency support", maxOpenConns)
	}
	if maxIdleConns < 20 {
		maxIdleConns = 20 // 保持足够的空闲连接
		log.Printf("Adjusting MaxIdleConns to %d for better performance", maxIdleConns)
	}
	
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	
	log.Printf("PostgreSQL connection pool configured: MaxIdleConns=%d, MaxOpenConns=%d, ConnMaxLifetime=%v, ConnMaxIdleTime=%v",
		maxIdleConns, maxOpenConns, connMaxLifetime, connMaxIdleTime)
	
	// 启动连接池健康检查
	go startConnectionPoolMonitoring(sqlDB)
	
	return db, nil
}


// AutoMigrate 执行数据库迁移（并发安全版本）
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	
	return AutoMigrateWithContext(context.Background())
}

// AutoMigrateWithContext 带上下文的数据库迁移
func AutoMigrateWithContext(ctx context.Context) error {
	// 检查迁移是否已完成
	migrationMutex.RLock()
	if migrationComplete {
		migrationMutex.RUnlock()
		log.Println("Database migration already completed")
		return nil
	}
	migrationMutex.RUnlock()

	// 获取写锁进行迁移
	migrationMutex.Lock()
	defer migrationMutex.Unlock()

	// 双重检查，避免重复迁移
	if migrationComplete {
		log.Println("Database migration already completed (double check)")
		return nil
	}

	// 创建迁移锁文件
	_, err := createMigrationLock()
	if err != nil {
		log.Printf("Warning: Could not create migration lock file: %v", err)
		// 继续执行，不阻塞迁移
	}
	defer removeMigrationLock()

	// 执行迁移前验证
	if err := preMigrationValidation(ctx); err != nil {
		return fmt.Errorf("pre-migration validation failed: %w", err)
	}

	log.Println("Starting database migration...")
	
	// 执行基础表结构迁移
	err = DB.WithContext(ctx).AutoMigrate(
		&WalletAddress{}, 
		&TradeOrders{}, 
		&NotifyRecord{},
	)
	if err != nil {
		return fmt.Errorf("auto migration failed: %w", err)
	}

	// 执行索引和约束优化
	if err := executeIndexOptimizations(ctx); err != nil {
		log.Printf("Warning: Index optimizations failed: %v", err)
		// 不阻塞主要迁移流程
	}

	// 执行PostgreSQL特定的优化索引
	if err := createPostgreSQLOptimizedIndexes(ctx); err != nil {
		log.Printf("Warning: PostgreSQL optimized indexes creation failed: %v", err)
		// 不阻塞主要迁移流程
	}

	// 执行迁移后验证
	if err := postMigrationValidation(ctx); err != nil {
		return fmt.Errorf("post-migration validation failed: %w", err)
	}

	// 标记迁移完成
	migrationComplete = true
	log.Println("Database migration completed successfully")
	return nil
}

// preMigrationValidation 迁移前验证
func preMigrationValidation(ctx context.Context) error {
	// 检查数据库连接
	if err := Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// 检查数据库权限
	var result int
	err := DB.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error
	if err != nil {
		return fmt.Errorf("database access test failed: %w", err)
	}

	log.Println("Pre-migration validation passed")
	return nil
}

// postMigrationValidation 迁移后验证
func postMigrationValidation(ctx context.Context) error {
	// 验证表是否存在
	tables := []string{"trade_orders", "wallet_address", "notify_record"}
	for _, table := range tables {
		var exists bool
		err := DB.WithContext(ctx).Raw(
			"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = ?)", 
			table,
		).Scan(&exists).Error
		if err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("table %s was not created", table)
		}
	}

	// 验证关键字段是否存在
	if err := validateTradeOrdersSchema(ctx); err != nil {
		return fmt.Errorf("TradeOrders schema validation failed: %w", err)
	}

	log.Println("Post-migration validation passed")
	return nil
}

// validateTradeOrdersSchema 验证TradeOrders表结构
func validateTradeOrdersSchema(ctx context.Context) error {
	requiredColumns := []string{"id", "order_id", "trade_id", "version", "status", "amount", "chain", "address"}
	
	for _, column := range requiredColumns {
		var exists bool
		err := DB.WithContext(ctx).Raw(`
			SELECT EXISTS (
				SELECT FROM information_schema.columns 
				WHERE table_name = 'trade_orders' AND column_name = ?
			)
		`, column).Scan(&exists).Error
		
		if err != nil {
			return fmt.Errorf("failed to check column %s: %w", column, err)
		}
		if !exists {
			return fmt.Errorf("required column %s not found in trade_orders table", column)
		}
	}
	
	return nil
}

// executeIndexOptimizations 执行索引优化
func executeIndexOptimizations(ctx context.Context) error {
	// 读取并执行索引优化SQL
	migrationFile := "migrations/002_optimize_indexes.sql"
	if _, err := os.Stat(migrationFile); os.IsNotExist(err) {
		log.Printf("Index optimization file %s not found, skipping", migrationFile)
		return nil
	}

	content, err := os.ReadFile(migrationFile)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// 执行SQL语句
	err = DB.WithContext(ctx).Exec(string(content)).Error
	if err != nil {
		return fmt.Errorf("failed to execute index optimizations: %w", err)
	}

	log.Println("Index optimizations completed")
	return nil
}

// createMigrationLock 创建迁移锁文件
func createMigrationLock() (*os.File, error) {
	// 创建锁文件目录
	lockDir := filepath.Dir(migrationLockFile)
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create lock directory: %w", err)
	}

	// 检查锁文件是否已存在
	if _, err := os.Stat(migrationLockFile); err == nil {
		return nil, fmt.Errorf("migration lock file already exists, another migration may be in progress")
	}

	// 创建锁文件
	lockFile, err := os.Create(migrationLockFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}

	// 写入进程信息
	pid := os.Getpid()
	timestamp := time.Now().Format(time.RFC3339)
	lockContent := fmt.Sprintf("PID: %d\nTimestamp: %s\n", pid, timestamp)
	
	if _, err := lockFile.WriteString(lockContent); err != nil {
		lockFile.Close()
		os.Remove(migrationLockFile)
		return nil, fmt.Errorf("failed to write lock file content: %w", err)
	}

	return lockFile, nil
}

// removeMigrationLock 移除迁移锁文件
func removeMigrationLock() {
	if err := os.Remove(migrationLockFile); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: Failed to remove migration lock file: %v", err)
	}
}

// IsMigrationInProgress 检查是否有迁移正在进行
func IsMigrationInProgress() bool {
	migrationMutex.RLock()
	defer migrationMutex.RUnlock()
	return !migrationComplete
}

// ResetMigrationState 重置迁移状态（仅用于测试）
func ResetMigrationState() {
	migrationMutex.Lock()
	defer migrationMutex.Unlock()
	migrationComplete = false
	removeMigrationLock()
}

// startConnectionPoolMonitoring 启动连接池监控
func startConnectionPoolMonitoring(sqlDB *sql.DB) {
	if !config.IsDbMonitoringEnabled() {
		log.Println("Database connection monitoring is disabled")
		return
	}

	interval := config.GetDbMonitoringInterval()
	log.Printf("Starting connection pool monitoring with interval: %v", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			monitorConnectionPool(sqlDB)
		}
	}
}

// monitorConnectionPool 监控连接池状态
func monitorConnectionPool(sqlDB *sql.DB) {
	stats := sqlDB.Stats()
	
	// 记录连接池统计信息
	log.Printf("Connection Pool Stats: OpenConnections=%d, InUse=%d, Idle=%d, MaxOpenConns=%d, MaxLifetimeClosed=%d, MaxIdleTimeClosed=%d",
		stats.OpenConnections, stats.InUse, stats.Idle, stats.MaxOpenConnections,
		stats.MaxLifetimeClosed, stats.MaxIdleTimeClosed)

	// 连接池健康检查
	if err := performHealthCheck(sqlDB); err != nil {
		log.Printf("Connection pool health check failed: %v", err)
		// 尝试重连
		if err := attemptReconnection(); err != nil {
			log.Printf("Failed to reconnect to database: %v", err)
		}
	}

	// 检查连接使用率
	if stats.MaxOpenConnections > 0 {
		usageRate := float64(stats.InUse) / float64(stats.MaxOpenConnections) * 100
		if usageRate > 80 {
			log.Printf("Warning: High connection usage rate: %.2f%%", usageRate)
		}
	}

	// 检查等待连接的情况
	if stats.WaitCount > 0 {
		avgWaitTime := stats.WaitDuration / time.Duration(stats.WaitCount)
		log.Printf("Connection wait stats: Count=%d, TotalWaitDuration=%v, AvgWaitTime=%v",
			stats.WaitCount, stats.WaitDuration, avgWaitTime)
		
		if avgWaitTime > time.Millisecond*100 {
			log.Printf("Warning: High average connection wait time: %v", avgWaitTime)
		}
	}
}

// performHealthCheck 执行数据库健康检查
func performHealthCheck(sqlDB *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 简单的ping检查
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// 执行简单查询检查
	var result int
	err := sqlDB.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("query test failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("unexpected query result: %d", result)
	}

	return nil
}

// attemptReconnection 尝试重新连接数据库
func attemptReconnection() error {
	log.Println("Attempting to reconnect to database...")
	
	// 关闭当前连接
	if DB != nil {
		if sqlDB, err := DB.DB(); err == nil {
			sqlDB.Close()
		}
	}

	// 重新初始化连接
	newDB, err := initPostgreSQLWithRetry()
	if err != nil {
		return fmt.Errorf("reconnection failed: %w", err)
	}

	DB = newDB
	log.Println("Database reconnection successful")
	return nil
}

// GetConnectionPoolStats 获取连接池统计信息
func GetConnectionPoolStats() (*sql.DBStats, error) {
	if DB == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	stats := sqlDB.Stats()
	return &stats, nil
}

// CheckConnectionPoolHealth 检查连接池健康状态
func CheckConnectionPoolHealth() error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	return performHealthCheck(sqlDB)
}

// OptimizeConnectionPool 根据当前负载动态优化连接池
func OptimizeConnectionPool() error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	stats := sqlDB.Stats()
	
	// 根据使用模式调整连接池参数
	if stats.MaxOpenConnections > 0 {
		usageRate := float64(stats.InUse) / float64(stats.MaxOpenConnections)
		
		// 如果使用率持续较高，考虑增加连接数
		if usageRate > 0.8 && stats.MaxOpenConnections < 200 {
			newMaxOpen := int(float64(stats.MaxOpenConnections) * 1.2)
			if newMaxOpen > 200 {
				newMaxOpen = 200
			}
			sqlDB.SetMaxOpenConns(newMaxOpen)
			log.Printf("Increased MaxOpenConns to %d due to high usage rate", newMaxOpen)
		}
		
		// 如果使用率持续较低，考虑减少连接数
		if usageRate < 0.2 && stats.MaxOpenConnections > 20 {
			newMaxOpen := int(float64(stats.MaxOpenConnections) * 0.8)
			if newMaxOpen < 20 {
				newMaxOpen = 20
			}
			sqlDB.SetMaxOpenConns(newMaxOpen)
			log.Printf("Decreased MaxOpenConns to %d due to low usage rate", newMaxOpen)
		}
	}

	return nil
}

// Close 关闭数据库连接
func Close() error {
	if DB == nil {
		return nil
	}
	
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %v", err)
	}
	
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %v", err)
	}
	
	log.Println("Database connection closed")
	return nil
}

// Ping 测试数据库连接
func Ping() error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %v", err)
	}
	
	return sqlDB.Ping()
}

// Stats 获取数据库连接统计信息
func Stats() (interface{}, error) {
	if DB == nil {
		return nil, fmt.Errorf("database connection is not initialized")
	}
	
	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %v", err)
	}
	
	return sqlDB.Stats(), nil
}

// createPostgreSQLOptimizedIndexes 创建PostgreSQL优化的复合索引
func createPostgreSQLOptimizedIndexes(ctx context.Context) error {
	log.Println("Creating PostgreSQL optimized indexes...")

	// 定义复合索引SQL
	indexes := []string{
		// TradeOrders 核心查询索引
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_status_chain_address 
		 ON trade_orders (status, chain, address) WHERE status IN (1, 2)`,

		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_status_amount 
		 ON trade_orders (status, amount) WHERE status = 1`,

		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_chain_address_amount 
		 ON trade_orders (chain, address, amount) WHERE status = 1`,

		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_expired_status 
		 ON trade_orders (expired_at, status) WHERE status = 1`,

		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_notify_failed 
		 ON trade_orders (status, notify_num, notify_state) 
		 WHERE status = 2 AND notify_num > 0 AND notify_state = 0`,

		// 时间范围查询优化 - 使用BRIN索引适合时序数据
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_created_at_brin 
		 ON trade_orders USING BRIN (created_at)`,

		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_confirmed_at_brin 
		 ON trade_orders USING BRIN (confirmed_at) WHERE confirmed_at IS NOT NULL`,

		// WalletAddress 查询索引
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_wallet_address_chain_status 
		 ON wallet_address (chain, status) WHERE status = 1`,

		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_wallet_address_status_other_notify 
		 ON wallet_address (status, other_notify, chain, address) WHERE status = 1`,

		// 部分索引优化 - 只索引活跃记录
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_active_orders 
		 ON trade_orders (created_at, chain, address) 
		 WHERE status = 1 AND expired_at > NOW()`,

		// 覆盖索引 - 包含常用查询字段
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_covering 
		 ON trade_orders (status, chain, address) 
		 INCLUDE (amount, trade_id, created_at) WHERE status IN (1, 2)`,
	}

	// 逐个执行索引创建
	for i, indexSQL := range indexes {
		log.Printf("Creating index %d/%d", i+1, len(indexes))
		
		// 使用较短的上下文超时来避免长时间阻塞
		indexCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		err := DB.WithContext(indexCtx).Exec(indexSQL).Error
		cancel()

		if err != nil {
			// 记录错误但不阻塞整个流程
			log.Printf("Warning: Failed to create index %d: %v", i+1, err)
			continue
		}
		
		log.Printf("Successfully created index %d/%d", i+1, len(indexes))
	}

	log.Println("PostgreSQL optimized indexes creation completed")
	return nil
}
