package model

import (
	"USDTMore/app/config"
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB
var _err error

// HealthCheck 数据库健康检查
func HealthCheck() error {
	if DB == nil {
		return fmt.Errorf("数据库连接未初始化")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	return nil
}

func Init() error {
	dbType := config.GetDBType()

	switch dbType {
	case "postgres", "postgresql":
		// Build PostgreSQL connection DSN
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
			config.GetDBHost(),
			config.GetDBUser(),
			config.GetDBPassword(),
			config.GetDBName(),
			config.GetDBPort(),
			config.GetDBSSLMode(),
			config.GetDBTimezone())

		DB, _err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			PrepareStmt: false, // 禁用预编译语句缓存，避免cached plan错误
		})
		if _err != nil {
			return fmt.Errorf("failed to connect to PostgreSQL: %w", _err)
		}

		// 清除PostgreSQL查询缓存，解决cached plan错误
		sqlDB, err := DB.DB()
		if err == nil {
			// 执行DISCARD ALL清除所有缓存的查询计划
			sqlDB.Exec("DISCARD ALL")
		}

	default:
		return fmt.Errorf("unsupported database type: %s. Only 'postgres' is supported in this version", dbType)
	}
	// 跳过AutoMigrate，使用init.sql处理数据库结构
	// if _err = AutoMigrate(); _err != nil {
	//	return _err
	// }

	addStartWalletAddress()

	return nil
}

func AutoMigrate() error {
	// 禁用自动创建约束，避免与init.sql冲突
	return DB.Set("gorm:table_options", "").AutoMigrate(&WalletAddress{}, &TradeOrders{}, &NotifyRecord{})
}
