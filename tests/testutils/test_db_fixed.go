package testutils

import (
	"USDTMore/app/model"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	postgresModule "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupTestDBFixed 设置测试数据库容器(修复版)
func SetupTestDBFixed(ctx context.Context, t *testing.T) (testcontainers.Container, *gorm.DB) {
	// 创建PostgreSQL测试容器
	postgresContainer, err := postgresModule.RunContainer(ctx,
		testcontainers.WithImage("postgres:15-alpine"),
		postgresModule.WithDatabase("usdtmore_test"),
		postgresModule.WithUsername("test"),
		postgresModule.WithPassword("test123"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err)

	// 获取数据库连接信息
	host, err := postgresContainer.Host(ctx)
	require.NoError(t, err)

	port, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	// 连接数据库
	dsn := fmt.Sprintf("host=%s port=%s user=test password=test123 dbname=usdtmore_test sslmode=disable TimeZone=UTC",
		host, port.Port())

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// 手动创建表结构，避免AutoMigrate的兼容性问题
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS trade_orders (
		id BIGSERIAL PRIMARY KEY,
		order_id VARCHAR(255) NOT NULL UNIQUE,
		trade_id VARCHAR(255) NOT NULL UNIQUE,
		trade_hash VARCHAR(64) DEFAULT '' UNIQUE,
		usdt_rate VARCHAR(10) NOT NULL,
		amount NUMERIC(10,2) NOT NULL DEFAULT 0,
		money NUMERIC(10,2) NOT NULL DEFAULT 0,
		chain VARCHAR(255) NOT NULL,
		address VARCHAR(34) NOT NULL,
		from_address VARCHAR(34) NOT NULL DEFAULT '',
		status SMALLINT NOT NULL DEFAULT 0,
		version BIGINT NOT NULL DEFAULT 0,
		return_url VARCHAR(255) NOT NULL DEFAULT '',
		notify_url VARCHAR(255) NOT NULL DEFAULT '',
		notify_num INTEGER NOT NULL DEFAULT 0,
		notify_state SMALLINT NOT NULL DEFAULT 0,
		expired_at TIMESTAMP NOT NULL,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		confirmed_at TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS wallet_addresses (
		id BIGSERIAL PRIMARY KEY,
		chain VARCHAR(10) NOT NULL,
		address VARCHAR(42) NOT NULL,
		balance NUMERIC(20,6) DEFAULT 0,
		is_active BOOLEAN DEFAULT true,
		last_used_at TIMESTAMP,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(chain, address)
	);

	CREATE TABLE IF NOT EXISTS notify_records (
		id BIGSERIAL PRIMARY KEY,
		order_id VARCHAR(255) NOT NULL,
		url VARCHAR(500) NOT NULL,
		params TEXT,
		response TEXT,
		status_code INTEGER,
		attempt_count INTEGER DEFAULT 1,
		success BOOLEAN DEFAULT false,
		created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- 创建索引
	CREATE INDEX IF NOT EXISTS idx_trade_orders_status ON trade_orders(status);
	CREATE INDEX IF NOT EXISTS idx_trade_orders_expired_at ON trade_orders(expired_at);
	CREATE INDEX IF NOT EXISTS idx_trade_orders_created_at ON trade_orders(created_at);
	CREATE INDEX IF NOT EXISTS idx_wallet_addresses_chain_active ON wallet_addresses(chain, is_active);
	CREATE INDEX IF NOT EXISTS idx_notify_records_order_id ON notify_records(order_id);
	`

	err = db.Exec(createTableSQL).Error
	require.NoError(t, err)

	TestDB = db
	model.DB = db

	return postgresContainer, db
}

// CleanDatabaseFixed 清理数据库数据
func CleanDatabaseFixed(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE trade_orders CASCADE")
	db.Exec("TRUNCATE TABLE wallet_addresses CASCADE")
	db.Exec("TRUNCATE TABLE notify_records CASCADE")
}