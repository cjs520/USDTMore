package model

import (
	"USDTMore/app/config"
	"fmt"
	"log"
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
	
	// 设置连接池参数
	sqlDB.SetMaxIdleConns(config.GetDbMaxIdleConns())
	sqlDB.SetMaxOpenConns(config.GetDbMaxOpenConns())
	sqlDB.SetConnMaxLifetime(config.GetDbConnMaxLifetime())
	sqlDB.SetConnMaxIdleTime(config.GetDbConnMaxIdleTime())
	
	log.Printf("PostgreSQL connection pool configured: MaxIdleConns=%d, MaxOpenConns=%d, ConnMaxLifetime=%v, ConnMaxIdleTime=%v",
		config.GetDbMaxIdleConns(), config.GetDbMaxOpenConns(),
		config.GetDbConnMaxLifetime(), config.GetDbConnMaxIdleTime())
	
	return db, nil
}


// AutoMigrate 执行数据库迁移
func AutoMigrate() error {
	if DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}
	
	err := DB.AutoMigrate(&WalletAddress{}, &TradeOrders{}, &NotifyRecord{})
	if err != nil {
		return fmt.Errorf("auto migration failed: %v", err)
	}
	
	log.Println("Database migration completed successfully")
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
