package testutils

import (
	"USDTMore/app/model"
	"context"
	"fmt"
	"log"
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

var TestDB *gorm.DB

// SetupTestDB 设置测试数据库容器
func SetupTestDB(ctx context.Context, t *testing.T) (testcontainers.Container, *gorm.DB) {
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
		Logger: logger.Default.LogMode(logger.Silent), // 静默模式，避免测试日志混乱
	})
	require.NoError(t, err)

	// 运行数据库迁移
	err = db.AutoMigrate(
		&model.TradeOrders{},
		&model.WalletAddress{},
		&model.NotifyRecord{},
	)
	require.NoError(t, err)

	TestDB = db
	model.DB = db // 设置全局数据库连接

	return postgresContainer, db
}

// TeardownTestDB 清理测试数据库
func TeardownTestDB(ctx context.Context, container testcontainers.Container) {
	if container != nil {
		if err := container.Terminate(ctx); err != nil {
			log.Printf("Failed to terminate container: %v", err)
		}
	}
}

// CleanDatabase 清理数据库数据
func CleanDatabase(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE trade_orders CASCADE")
	db.Exec("TRUNCATE TABLE wallet_addresses CASCADE")
	db.Exec("TRUNCATE TABLE notify_records CASCADE")
}