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

		DB, _err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if _err != nil {
			return fmt.Errorf("failed to connect to PostgreSQL: %w", _err)
		}
	default:
		return fmt.Errorf("unsupported database type: %s. Only 'postgres' is supported in this version", dbType)
	}
	if _err = AutoMigrate(); _err != nil {

		return _err
	}

	addStartWalletAddress()

	return nil
}

func AutoMigrate() error {
	return DB.AutoMigrate(&WalletAddress{}, &TradeOrders{}, &NotifyRecord{})
}
