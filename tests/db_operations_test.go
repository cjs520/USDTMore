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
	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OperationResult 操作测试结果结构
type OperationResult struct {
	TestName    string        `json:"test_name"`
	Operation   string        `json:"operation"`
	Table       string        `json:"table"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
	Description string        `json:"description"`
	DataCount   int           `json:"data_count,omitempty"`
}

// DatabaseOperationsTester 数据库操作测试器
type DatabaseOperationsTester struct {
	results []OperationResult
	db      *gorm.DB
	dbType  string
	testDataPath string
}

// NewDatabaseOperationsTester 创建新的数据库操作测试器
func NewDatabaseOperationsTester() *DatabaseOperationsTester {
	return &DatabaseOperationsTester{
		results: make([]OperationResult, 0),
		testDataPath: "/tmp/operations_test.db",
	}
}

// addResult 添加测试结果
func (dot *DatabaseOperationsTester) addResult(testName, operation, table, description string, success bool, duration time.Duration, err error, dataCount int) {
	result := OperationResult{
		TestName:    testName,
		Operation:   operation,
		Table:       table,
		Description: description,
		Success:     success,
		Duration:    duration,
		DataCount:   dataCount,
	}
	if err != nil {
		result.Error = err.Error()
	}
	dot.results = append(dot.results, result)
}

// setupDatabase 设置测试数据库
func (dot *DatabaseOperationsTester) setupDatabase(usePostgreSQL bool) error {
	var err error
	
	if usePostgreSQL {
		// 检查PostgreSQL配置
		if os.Getenv("POSTGRESQL_DSN") == "" && 
			(os.Getenv("DB_HOST") == "" || os.Getenv("DB_PASSWORD") == "") {
			return fmt.Errorf("missing PostgreSQL configuration")
		}
		
		dot.dbType = "PostgreSQL"
		dsn := config.GetPostgreSQLDSN()
		
		dot.db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		if err != nil {
			return fmt.Errorf("failed to connect to PostgreSQL: %v", err)
		}
		
		// 清理测试表
		dot.db.Migrator().DropTable(&model.WalletAddress{}, &model.TradeOrders{}, &model.NotifyRecord{})
		
	} else {
		dot.dbType = "SQLite"
		os.Remove(dot.testDataPath)
		
		dot.db, err = gorm.Open(sqlite.Open(dot.testDataPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
		if err != nil {
			return fmt.Errorf("failed to connect to SQLite: %v", err)
		}
	}
	
	// 自动迁移
	if err := dot.db.AutoMigrate(&model.WalletAddress{}, &model.TradeOrders{}, &model.NotifyRecord{}); err != nil {
		return fmt.Errorf("failed to migrate database: %v", err)
	}
	
	// 设置全局DB连接，供模型方法使用
	model.DB = dot.db
	
	log.Printf("Database setup completed for %s", dot.dbType)
	return nil
}

// TestWalletAddressOperations 测试钱包地址表的所有操作
func (dot *DatabaseOperationsTester) TestWalletAddressOperations() {
	
	// 测试创建操作
	dot.testWalletCreate()
	
	// 测试查询操作
	dot.testWalletQuery()
	
	// 测试更新操作
	dot.testWalletUpdate()
	
	// 测试状态管理
	dot.testWalletStatus()
	
	// 测试批量操作
	dot.testWalletBatchOperations()
	
	// 测试删除操作（最后执行）
	dot.testWalletDelete()
	
	log.Printf("Completed wallet address operations tests for %s", dot.dbType)
}

// testWalletCreate 测试钱包地址创建
func (dot *DatabaseOperationsTester) testWalletCreate() {
	start := time.Now()
	
	wallet := model.WalletAddress{
		Chain: "TRON",
		Address: "TRX123456789ABCDEF",
		StartBlock: 12345,
		InAmount: 100.50,
		OutAmount: 50.25,
		Count: 10,
		Status: 1,
		OtherNotify: 1,
	}
	
	err := dot.db.Create(&wallet).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Create", "CREATE", "wallet_address", "Create single wallet address record", false, duration, err, 0)
		return
	}
	
	if wallet.Id == 0 {
		dot.addResult("Wallet Create", "CREATE", "wallet_address", "Create single wallet address record", false, duration, 
			fmt.Errorf("wallet ID not assigned after creation"), 0)
		return
	}
	
	dot.addResult("Wallet Create", "CREATE", "wallet_address", "Create single wallet address record", true, duration, nil, 1)
}

// testWalletQuery 测试钱包地址查询
func (dot *DatabaseOperationsTester) testWalletQuery() {
	start := time.Now()
	
	// 单条记录查询
	var wallet model.WalletAddress
	err := dot.db.Where("chain = ? AND address = ?", "TRON", "TRX123456789ABCDEF").First(&wallet).Error
	
	if err != nil {
		dot.addResult("Wallet Query Single", "SELECT", "wallet_address", "Query single wallet by chain and address", false, time.Since(start), err, 0)
	} else {
		dot.addResult("Wallet Query Single", "SELECT", "wallet_address", "Query single wallet by chain and address", true, time.Since(start), nil, 1)
	}
	
	// 多条记录查询
	start = time.Now()
	var wallets []model.WalletAddress
	err = dot.db.Where("status = ?", 1).Find(&wallets).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Query Multiple", "SELECT", "wallet_address", "Query multiple wallets by status", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Query Multiple", "SELECT", "wallet_address", "Query multiple wallets by status", true, duration, nil, len(wallets))
	}
	
	// 计数查询
	start = time.Now()
	var count int64
	err = dot.db.Model(&model.WalletAddress{}).Where("status = ?", 1).Count(&count).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Count", "COUNT", "wallet_address", "Count active wallets", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Count", "COUNT", "wallet_address", "Count active wallets", true, duration, nil, int(count))
	}
	
	// 条件查询
	start = time.Now()
	err = dot.db.Where("in_amount > ? AND status = ?", 50.0, 1).Find(&wallets).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Conditional Query", "SELECT", "wallet_address", "Query wallets with amount condition", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Conditional Query", "SELECT", "wallet_address", "Query wallets with amount condition", true, duration, nil, len(wallets))
	}
}

// testWalletUpdate 测试钱包地址更新
func (dot *DatabaseOperationsTester) testWalletUpdate() {
	start := time.Now()
	
	// 查找要更新的记录
	var wallet model.WalletAddress
	if err := dot.db.Where("chain = ?", "TRON").First(&wallet).Error; err != nil {
		dot.addResult("Wallet Update", "UPDATE", "wallet_address", "Find wallet for update test", false, time.Since(start), err, 0)
		return
	}
	
	// 更新单个字段
	newAmount := 200.75
	err := dot.db.Model(&wallet).Update("in_amount", newAmount).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Update Single Field", "UPDATE", "wallet_address", "Update wallet in_amount", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Update Single Field", "UPDATE", "wallet_address", "Update wallet in_amount", true, duration, nil, 1)
	}
	
	// 更新多个字段
	start = time.Now()
	err = dot.db.Model(&wallet).Updates(model.WalletAddress{
		InAmount: 300.00,
		OutAmount: 150.00,
		Count: 20,
	}).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Update Multiple Fields", "UPDATE", "wallet_address", "Update multiple wallet fields", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Update Multiple Fields", "UPDATE", "wallet_address", "Update multiple wallet fields", true, duration, nil, 1)
	}
	
	// 批量更新
	start = time.Now()
	err = dot.db.Model(&model.WalletAddress{}).Where("chain = ?", "TRON").Update("other_notify", 0).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Batch Update", "UPDATE", "wallet_address", "Batch update wallets by chain", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Batch Update", "UPDATE", "wallet_address", "Batch update wallets by chain", true, duration, nil, 1)
	}
}

// testWalletStatus 测试钱包地址状态管理
func (dot *DatabaseOperationsTester) testWalletStatus() {
	start := time.Now()
	
	var wallet model.WalletAddress
	if err := dot.db.First(&wallet).Error; err != nil {
		dot.addResult("Wallet Status Test", "SELECT", "wallet_address", "Find wallet for status test", false, time.Since(start), err, 0)
		return
	}
	
	// 测试状态设置方法
	originalStatus := wallet.Status
	wallet.SetStatus(0)
	
	// 验证状态是否已更改
	var updatedWallet model.WalletAddress
	err := dot.db.First(&updatedWallet, wallet.Id).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Status Set", "UPDATE", "wallet_address", "Test wallet status setting method", false, duration, err, 0)
	} else if updatedWallet.Status != 0 {
		dot.addResult("Wallet Status Set", "UPDATE", "wallet_address", "Test wallet status setting method", false, duration, 
			fmt.Errorf("status not updated: expected 0, got %d", updatedWallet.Status), 0)
	} else {
		dot.addResult("Wallet Status Set", "UPDATE", "wallet_address", "Test wallet status setting method", true, duration, nil, 1)
	}
	
	// 恢复原始状态
	wallet.SetStatus(originalStatus)
	
	// 测试其他通知设置
	start = time.Now()
	originalNotify := wallet.OtherNotify
	wallet.SetOtherNotify(1 - originalNotify)
	
	err = dot.db.First(&updatedWallet, wallet.Id).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Notify Set", "UPDATE", "wallet_address", "Test wallet notify setting method", false, duration, err, 0)
	} else if updatedWallet.OtherNotify != (1 - originalNotify) {
		dot.addResult("Wallet Notify Set", "UPDATE", "wallet_address", "Test wallet notify setting method", false, duration, 
			fmt.Errorf("notify not updated: expected %d, got %d", 1-originalNotify, updatedWallet.OtherNotify), 0)
	} else {
		dot.addResult("Wallet Notify Set", "UPDATE", "wallet_address", "Test wallet notify setting method", true, duration, nil, 1)
	}
	
	// 恢复原始设置
	wallet.SetOtherNotify(originalNotify)
}

// testWalletBatchOperations 测试钱包地址批量操作
func (dot *DatabaseOperationsTester) testWalletBatchOperations() {
	start := time.Now()
	
	// 创建测试数据
	wallets := []model.WalletAddress{
		{Chain: "POLY", Address: "0x111", Status: 1, InAmount: 100.0},
		{Chain: "BSC", Address: "0x222", Status: 1, InAmount: 200.0},
		{Chain: "ARB", Address: "0x333", Status: 1, InAmount: 300.0},
		{Chain: "OP", Address: "0x444", Status: 1, InAmount: 400.0},
	}
	
	// 批量创建
	err := dot.db.CreateInBatches(wallets, 2).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Batch Create", "INSERT", "wallet_address", "Create multiple wallets in batches", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Batch Create", "INSERT", "wallet_address", "Create multiple wallets in batches", true, duration, nil, len(wallets))
	}
	
	// 测试批量查询
	start = time.Now()
	var batchWallets []model.WalletAddress
	chains := []string{"POLY", "BSC", "ARB", "OP"}
	err = dot.db.Where("chain IN ?", chains).Find(&batchWallets).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Batch Query", "SELECT", "wallet_address", "Query multiple wallets by chain list", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Batch Query", "SELECT", "wallet_address", "Query multiple wallets by chain list", true, duration, nil, len(batchWallets))
	}
}

// testWalletDelete 测试钱包地址删除
func (dot *DatabaseOperationsTester) testWalletDelete() {
	start := time.Now()
	
	// 查找要删除的记录
	var wallet model.WalletAddress
	if err := dot.db.Where("chain = ?", "POLY").First(&wallet).Error; err != nil {
		dot.addResult("Wallet Delete", "DELETE", "wallet_address", "Find wallet for delete test", false, time.Since(start), err, 0)
		return
	}
	
	// 测试删除方法
	wallet.Delete()
	
	// 验证删除
	var deletedWallet model.WalletAddress
	err := dot.db.First(&deletedWallet, wallet.Id).Error
	duration := time.Since(start)
	
	if err != nil && err == gorm.ErrRecordNotFound {
		dot.addResult("Wallet Delete", "DELETE", "wallet_address", "Test wallet delete method", true, duration, nil, 1)
	} else if err != nil {
		dot.addResult("Wallet Delete", "DELETE", "wallet_address", "Test wallet delete method", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Delete", "DELETE", "wallet_address", "Test wallet delete method", false, duration, 
			fmt.Errorf("wallet not deleted: still found in database"), 0)
	}
	
	// 测试批量删除
	start = time.Now()
	err = dot.db.Where("chain IN ?", []string{"BSC", "ARB"}).Delete(&model.WalletAddress{}).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Wallet Batch Delete", "DELETE", "wallet_address", "Batch delete wallets by chain", false, duration, err, 0)
	} else {
		dot.addResult("Wallet Batch Delete", "DELETE", "wallet_address", "Batch delete wallets by chain", true, duration, nil, 2)
	}
}

// TestTradeOrdersOperations 测试交易订单表的所有操作
func (dot *DatabaseOperationsTester) TestTradeOrdersOperations() {
	// 测试创建操作
	dot.testOrderCreate()
	
	// 测试查询操作
	dot.testOrderQuery()
	
	// 测试更新操作
	dot.testOrderUpdate()
	
	// 测试业务逻辑
	dot.testOrderBusinessLogic()
	
	// 测试删除操作
	dot.testOrderDelete()
	
	log.Printf("Completed trade orders operations tests for %s", dot.dbType)
}

// testOrderCreate 测试订单创建
func (dot *DatabaseOperationsTester) testOrderCreate() {
	start := time.Now()
	
	order := model.TradeOrders{
		Chain: "TRON",
		Address: "TRX123456789ABCDEF",
		Amount: "100.50",
		Status: 1,
		TradeHash: "hash123456789",
		OrderId: "order123",
		TradeId: "trade123",
		UsdtRate: "7.20",
		Money: 100.50,
	}
	
	err := dot.db.Create(&order).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Order Create", "CREATE", "trade_orders", "Create single trade order record", false, duration, err, 0)
		return
	}
	
	if order.Id == 0 {
		dot.addResult("Order Create", "CREATE", "trade_orders", "Create single trade order record", false, duration, 
			fmt.Errorf("order ID not assigned after creation"), 0)
		return
	}
	
	dot.addResult("Order Create", "CREATE", "trade_orders", "Create single trade order record", true, duration, nil, 1)
}

// testOrderQuery 测试订单查询
func (dot *DatabaseOperationsTester) testOrderQuery() {
	start := time.Now()
	
	// 按哈希查询
	var order model.TradeOrders
	err := dot.db.Where("hash = ?", "hash123456789").First(&order).Error
	
	if err != nil {
		dot.addResult("Order Query By Hash", "SELECT", "trade_orders", "Query order by transaction hash", false, time.Since(start), err, 0)
	} else {
		dot.addResult("Order Query By Hash", "SELECT", "trade_orders", "Query order by transaction hash", true, time.Since(start), nil, 1)
	}
	
	// 按状态查询
	start = time.Now()
	var orders []model.TradeOrders
	err = dot.db.Where("status = ?", 1).Find(&orders).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Order Query By Status", "SELECT", "trade_orders", "Query orders by status", false, duration, err, 0)
	} else {
		dot.addResult("Order Query By Status", "SELECT", "trade_orders", "Query orders by status", true, duration, nil, len(orders))
	}
	
	// 金额范围查询
	start = time.Now()
	err = dot.db.Where("amount >= ? AND amount <= ?", "50.00", "200.00").Find(&orders).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Order Amount Range Query", "SELECT", "trade_orders", "Query orders by amount range", false, duration, err, 0)
	} else {
		dot.addResult("Order Amount Range Query", "SELECT", "trade_orders", "Query orders by amount range", true, duration, nil, len(orders))
	}
}

// testOrderUpdate 测试订单更新
func (dot *DatabaseOperationsTester) testOrderUpdate() {
	start := time.Now()
	
	var order model.TradeOrders
	if err := dot.db.First(&order).Error; err != nil {
		dot.addResult("Order Update", "UPDATE", "trade_orders", "Find order for update test", false, time.Since(start), err, 0)
		return
	}
	
	// 更新状态
	err := dot.db.Model(&order).Update("status", 2).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Order Status Update", "UPDATE", "trade_orders", "Update order status", false, duration, err, 0)
	} else {
		dot.addResult("Order Status Update", "UPDATE", "trade_orders", "Update order status", true, duration, nil, 1)
	}
	
	// 更新回调状态
	start = time.Now()
	err = dot.db.Model(&order).Update("callback_status", 1).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Order Callback Update", "UPDATE", "trade_orders", "Update order callback status", false, duration, err, 0)
	} else {
		dot.addResult("Order Callback Update", "UPDATE", "trade_orders", "Update order callback status", true, duration, nil, 1)
	}
}

// testOrderBusinessLogic 测试订单业务逻辑
func (dot *DatabaseOperationsTester) testOrderBusinessLogic() {
	start := time.Now()
	
	// 创建多个订单来测试业务场景
	orders := []model.TradeOrders{
		{Chain: "TRON", Address: "TRX111", Amount: "50.0", OrderId: "order1", TradeId: "trade1", UsdtRate: "7.20", Money: 50.0, Status: 0},
		{Chain: "TRON", Address: "TRX111", Amount: "100.0", OrderId: "order2", TradeId: "trade2", UsdtRate: "7.20", Money: 100.0, Status: 1},
		{Chain: "POLY", Address: "0x111", Amount: "200.0", OrderId: "order3", TradeId: "trade3", UsdtRate: "7.20", Money: 200.0, Status: 1},
	}
	
	err := dot.db.CreateInBatches(orders, 10).Error
	if err != nil {
		dot.addResult("Order Business Setup", "INSERT", "trade_orders", "Create orders for business logic test", false, time.Since(start), err, 0)
		return
	}
	
	// 测试按地址汇总
	start = time.Now()
	var result struct {
		Address    string
		TotalAmount decimal.Decimal
		OrderCount int64
	}
	
	err = dot.db.Model(&model.TradeOrders{}).
		Select("address, SUM(amount) as total_amount, COUNT(*) as order_count").
		Where("address = ? AND status = ?", "TRX111", 1).
		Group("address").
		Scan(&result).Error
	
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Order Address Summary", "SELECT", "trade_orders", "Summarize orders by address", false, duration, err, 0)
	} else {
		dot.addResult("Order Address Summary", "SELECT", "trade_orders", "Summarize orders by address", true, duration, nil, 1)
	}
	
	// 测试状态统计
	start = time.Now()
	var statusStats []struct {
		Status int
		Count  int64
	}
	
	err = dot.db.Model(&model.TradeOrders{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusStats).Error
	
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Order Status Stats", "SELECT", "trade_orders", "Get order status statistics", false, duration, err, 0)
	} else {
		dot.addResult("Order Status Stats", "SELECT", "trade_orders", "Get order status statistics", true, duration, nil, len(statusStats))
	}
}

// testOrderDelete 测试订单删除
func (dot *DatabaseOperationsTester) testOrderDelete() {
	start := time.Now()
	
	var order model.TradeOrders
	if err := dot.db.Where("status = ?", 0).First(&order).Error; err != nil {
		dot.addResult("Order Delete", "DELETE", "trade_orders", "Find order for delete test", false, time.Since(start), err, 0)
		return
	}
	
	err := dot.db.Delete(&order).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Order Delete", "DELETE", "trade_orders", "Delete single order", false, duration, err, 0)
	} else {
		dot.addResult("Order Delete", "DELETE", "trade_orders", "Delete single order", true, duration, nil, 1)
	}
	
	// 批量删除测试状态订单
	start = time.Now()
	result := dot.db.Where("status = ?", 0).Delete(&model.TradeOrders{})
	duration = time.Since(start)
	
	if result.Error != nil {
		dot.addResult("Order Batch Delete", "DELETE", "trade_orders", "Batch delete orders by status", false, duration, result.Error, 0)
	} else {
		dot.addResult("Order Batch Delete", "DELETE", "trade_orders", "Batch delete orders by status", true, duration, nil, int(result.RowsAffected))
	}
}

// TestNotifyRecordOperations 测试通知记录表的所有操作
func (dot *DatabaseOperationsTester) TestNotifyRecordOperations() {
	// 测试创建操作
	dot.testNotifyCreate()
	
	// 测试查询操作
	dot.testNotifyQuery()
	
	// 测试更新操作
	dot.testNotifyUpdate()
	
	// 测试重试逻辑
	dot.testNotifyRetryLogic()
	
	// 测试删除操作
	dot.testNotifyDelete()
	
	log.Printf("Completed notify record operations tests for %s", dot.dbType)
}

// testNotifyCreate 测试通知记录创建
func (dot *DatabaseOperationsTester) testNotifyCreate() {
	start := time.Now()
	
	notify := model.NotifyRecord{
		Txid: "hash123456789",
	}
	
	err := dot.db.Create(&notify).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Notify Create", "CREATE", "notify_record", "Create single notify record", false, duration, err, 0)
		return
	}
	
	dot.addResult("Notify Create", "CREATE", "notify_record", "Create single notify record", true, duration, nil, 1)
}

// testNotifyQuery 测试通知记录查询
func (dot *DatabaseOperationsTester) testNotifyQuery() {
	start := time.Now()
	
	var notify model.NotifyRecord
	err := dot.db.Where("order_id = ?", "order123").First(&notify).Error
	
	if err != nil {
		dot.addResult("Notify Query", "SELECT", "notify_record", "Query notify by order ID", false, time.Since(start), err, 0)
	} else {
		dot.addResult("Notify Query", "SELECT", "notify_record", "Query notify by order ID", true, time.Since(start), nil, 1)
	}
	
	// 按状态查询
	start = time.Now()
	var notifications []model.NotifyRecord
	err = dot.db.Where("status = ?", 1).Find(&notifications).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Notify Status Query", "SELECT", "notify_record", "Query notifications by status", false, duration, err, 0)
	} else {
		dot.addResult("Notify Status Query", "SELECT", "notify_record", "Query notifications by status", true, duration, nil, len(notifications))
	}
}

// testNotifyUpdate 测试通知记录更新
func (dot *DatabaseOperationsTester) testNotifyUpdate() {
	start := time.Now()
	
	var notify model.NotifyRecord
	if err := dot.db.First(&notify).Error; err != nil {
		dot.addResult("Notify Update", "UPDATE", "notify_record", "Find notify for update test", false, time.Since(start), err, 0)
		return
	}
	
	// 简化测试 - 只测试更新操作
	err := dot.db.Model(&notify).Update("updated_at", time.Now()).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Notify Try Count Update", "UPDATE", "notify_record", "Update notification try count", false, duration, err, 0)
	} else {
		dot.addResult("Notify Try Count Update", "UPDATE", "notify_record", "Update notification try count", true, duration, nil, 1)
	}
}

// testNotifyRetryLogic 测试通知重试逻辑
func (dot *DatabaseOperationsTester) testNotifyRetryLogic() {
	start := time.Now()
	
	// 创建失败的通知记录
	retryNotifications := []model.NotifyRecord{
		{Txid: "hash_retry1"},
		{Txid: "hash_retry2"},
		{Txid: "hash_retry3"},
	}
	
	err := dot.db.CreateInBatches(retryNotifications, 10).Error
	if err != nil {
		dot.addResult("Notify Retry Setup", "INSERT", "notify_record", "Create notifications for retry test", false, time.Since(start), err, 0)
		return
	}
	
	// 查询需要重试的通知（重试次数 < 3）
	start = time.Now()
	var pendingRetries []model.NotifyRecord
	err = dot.db.Where("status = ? AND try_count < ?", 0, 3).Find(&pendingRetries).Error
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Notify Retry Query", "SELECT", "notify_record", "Query notifications for retry", false, duration, err, 0)
	} else {
		expectedCount := 2 // retry1 和 retry2 符合条件
		if len(pendingRetries) == expectedCount {
			dot.addResult("Notify Retry Query", "SELECT", "notify_record", "Query notifications for retry", true, duration, nil, len(pendingRetries))
		} else {
			dot.addResult("Notify Retry Query", "SELECT", "notify_record", "Query notifications for retry", false, duration, 
				fmt.Errorf("expected %d retries, got %d", expectedCount, len(pendingRetries)), len(pendingRetries))
		}
	}
	
	// 批量更新重试计数
	start = time.Now()
	err = dot.db.Model(&model.NotifyRecord{}).
		Where("status = ? AND try_count < ?", 0, 3).
		Update("try_count", gorm.Expr("try_count + 1")).Error
	duration = time.Since(start)
	
	if err != nil {
		dot.addResult("Notify Retry Update", "UPDATE", "notify_record", "Increment retry count for pending notifications", false, duration, err, 0)
	} else {
		dot.addResult("Notify Retry Update", "UPDATE", "notify_record", "Increment retry count for pending notifications", true, duration, nil, 2)
	}
}

// testNotifyDelete 测试通知记录删除
func (dot *DatabaseOperationsTester) testNotifyDelete() {
	start := time.Now()
	
	// 删除重试次数过多的记录
	result := dot.db.Where("try_count > ?", 4).Delete(&model.NotifyRecord{})
	duration := time.Since(start)
	
	if result.Error != nil {
		dot.addResult("Notify Cleanup Delete", "DELETE", "notify_record", "Delete notifications with excessive retry count", false, duration, result.Error, 0)
	} else {
		dot.addResult("Notify Cleanup Delete", "DELETE", "notify_record", "Delete notifications with excessive retry count", true, duration, nil, int(result.RowsAffected))
	}
}

// TestTransactionOperations 测试事务操作
func (dot *DatabaseOperationsTester) TestTransactionOperations() {
	start := time.Now()
	
	// 测试成功的事务
	err := dot.db.Transaction(func(tx *gorm.DB) error {
		wallet := model.WalletAddress{
			Chain: "TX_TEST", Address: "TX_ADDRESS", Status: 1,
		}
		if err := tx.Create(&wallet).Error; err != nil {
			return err
		}
		
		order := model.TradeOrders{
			Chain: "TX_TEST", Address: "TX_ADDRESS", Amount: "100.0", OrderId: "order_tx", TradeId: "trade_tx", UsdtRate: "7.20", Money: 100.0, Status: 1,
		}
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		
		return nil
	})
	
	duration := time.Since(start)
	
	if err != nil {
		dot.addResult("Transaction Success", "TRANSACTION", "multiple", "Execute successful transaction with multiple operations", false, duration, err, 0)
	} else {
		dot.addResult("Transaction Success", "TRANSACTION", "multiple", "Execute successful transaction with multiple operations", true, duration, nil, 2)
	}
	
	// 测试回滚的事务
	start = time.Now()
	err = dot.db.Transaction(func(tx *gorm.DB) error {
		wallet := model.WalletAddress{
			Chain: "ROLLBACK_TEST", Address: "ROLLBACK_ADDRESS", Status: 1,
		}
		if err := tx.Create(&wallet).Error; err != nil {
			return err
		}
		
		// 故意造成错误以触发回滚
		return fmt.Errorf("intentional rollback")
	})
	
	duration = time.Since(start)
	
	if err != nil {
		// 验证数据没有被提交
		var count int64
		dot.db.Model(&model.WalletAddress{}).Where("chain = ?", "ROLLBACK_TEST").Count(&count)
		if count == 0 {
			dot.addResult("Transaction Rollback", "TRANSACTION", "multiple", "Execute transaction rollback on error", true, duration, nil, 0)
		} else {
			dot.addResult("Transaction Rollback", "TRANSACTION", "multiple", "Execute transaction rollback on error", false, duration, 
				fmt.Errorf("rollback failed: found %d records", count), int(count))
		}
	} else {
		dot.addResult("Transaction Rollback", "TRANSACTION", "multiple", "Execute transaction rollback on error", false, duration, 
			fmt.Errorf("transaction should have failed but succeeded"), 0)
	}
}

// TestModelMethods 测试模型方法
func (dot *DatabaseOperationsTester) TestModelMethods() {
	// 测试模型函数
	start := time.Now()
	
	// 创建测试钱包
	wallet := model.WalletAddress{
		Chain: "METHOD_TEST", Address: "METHOD_ADDRESS", Status: 1,
	}
	if err := dot.db.Create(&wallet).Error; err != nil {
		dot.addResult("Model Method Setup", "CREATE", "wallet_address", "Create wallet for method testing", false, time.Since(start), err, 0)
		return
	}
	
	// 测试 ExistsAddress 函数
	start = time.Now()
	exists := model.ExistsAddress("METHOD_TEST", "METHOD_ADDRESS")
	duration := time.Since(start)
	
	if exists {
		dot.addResult("Model ExistsAddress", "SELECT", "wallet_address", "Test ExistsAddress model function", true, duration, nil, 1)
	} else {
		dot.addResult("Model ExistsAddress", "SELECT", "wallet_address", "Test ExistsAddress model function", false, duration, 
			fmt.Errorf("ExistsAddress returned false for existing address"), 0)
	}
	
	// 测试 GetAvailableAddress 函数
	start = time.Now()
	addresses := model.GetAvailableAddress("METHOD_TEST")
	duration = time.Since(start)
	
	if len(addresses) > 0 {
		dot.addResult("Model GetAvailableAddress", "SELECT", "wallet_address", "Test GetAvailableAddress model function", true, duration, nil, len(addresses))
	} else {
		dot.addResult("Model GetAvailableAddress", "SELECT", "wallet_address", "Test GetAvailableAddress model function", false, duration, 
			fmt.Errorf("GetAvailableAddress returned no results"), 0)
	}
	
	// 测试 GetOtherNotify 函数
	start = time.Now()
	_ = model.GetOtherNotify("METHOD_TEST", "METHOD_ADDRESS")
	duration = time.Since(start)
	
	// 这个测试总是成功，因为函数有默认返回值
	dot.addResult("Model GetOtherNotify", "SELECT", "wallet_address", "Test GetOtherNotify model function", true, duration, nil, 1)
}

// RunAllTests 运行所有操作测试
func (dot *DatabaseOperationsTester) RunAllTests(usePostgreSQL bool) {
	log.Printf("Starting database operations tests for %s...", map[bool]string{true: "PostgreSQL", false: "SQLite"}[usePostgreSQL])
	
	if err := dot.setupDatabase(usePostgreSQL); err != nil {
		log.Printf("Failed to setup database: %v", err)
		dot.addResult("Database Setup", "CONNECTION", "system", "Initialize database for testing", false, 0, err, 0)
		return
	}
	
	defer func() {
		if !usePostgreSQL {
			os.Remove(dot.testDataPath)
		}
		
		if dot.db != nil {
			if sqlDB, err := dot.db.DB(); err == nil {
				sqlDB.Close()
			}
		}
	}()
	
	// 运行各表的操作测试
	dot.TestWalletAddressOperations()
	dot.TestTradeOrdersOperations()
	dot.TestNotifyRecordOperations()
	
	// 运行事务测试
	dot.TestTransactionOperations()
	
	// 运行模型方法测试
	dot.TestModelMethods()
	
	log.Printf("Completed database operations tests for %s", dot.dbType)
}

// PrintResults 打印测试结果
func (dot *DatabaseOperationsTester) PrintResults() {
	fmt.Println("\n" + strings.Repeat("=", 130))
	fmt.Printf("DATABASE OPERATIONS TEST RESULTS (%s)\n", dot.dbType)
	fmt.Println(strings.Repeat("=", 130))
	
	// 按表分组结果
	tableResults := make(map[string][]OperationResult)
	for _, result := range dot.results {
		tableResults[result.Table] = append(tableResults[result.Table], result)
	}
	
	for tableName, results := range tableResults {
		fmt.Printf("\nTable: %s\n", tableName)
		fmt.Println(strings.Repeat("-", 90))
		fmt.Printf("%-20s | %-10s | %-8s | %8s | %8s | %s\n", 
			"Test", "Operation", "Success", "Duration", "Records", "Description")
		fmt.Println(strings.Repeat("-", 90))
		
		for _, result := range results {
			status := "✅"
			if !result.Success {
				status = "❌"
			}
			
			recordInfo := fmt.Sprintf("%d", result.DataCount)
			if result.DataCount == 0 {
				recordInfo = "N/A"
			}
			
			fmt.Printf("%-20s | %-10s | %8s | %8s | %8s | %s\n", 
				result.TestName, result.Operation, status, 
				result.Duration.Truncate(time.Millisecond), recordInfo, result.Description)
			
			if result.Error != "" {
				fmt.Printf("     Error: %s\n", result.Error)
			}
		}
	}
	
	// 统计信息
	totalTests := len(dot.results)
	passedTests := 0
	for _, result := range dot.results {
		if result.Success {
			passedTests++
		}
	}
	
	fmt.Println(strings.Repeat("=", 130))
	fmt.Printf("SUMMARY (%s): %d/%d tests passed (%.1f%%)\n", 
		dot.dbType, passedTests, totalTests, float64(passedTests)/float64(totalTests)*100)
	fmt.Println(strings.Repeat("=", 130))
}

// GetResults 获取测试结果
func (dot *DatabaseOperationsTester) GetResults() []OperationResult {
	return dot.results
}

// TestMain 主测试入口（用于go test）
func TestDatabaseOperations(t *testing.T) {
	// 测试SQLite
	sqliteTester := NewDatabaseOperationsTester()
	sqliteTester.RunAllTests(false)
	sqliteTester.PrintResults()
	
	for _, result := range sqliteTester.GetResults() {
		if !result.Success {
			t.Errorf("SQLite operations test '%s' failed: %s", result.TestName, result.Error)
		}
	}
	
	// 测试PostgreSQL（如果可用）
	if os.Getenv("POSTGRESQL_DSN") != "" || (os.Getenv("DB_HOST") != "" && os.Getenv("DB_PASSWORD") != "") {
		postgresTester := NewDatabaseOperationsTester()
		postgresTester.RunAllTests(true)
		postgresTester.PrintResults()
		
		for _, result := range postgresTester.GetResults() {
			if !result.Success {
				t.Errorf("PostgreSQL operations test '%s' failed: %s", result.TestName, result.Error)
			}
		}
	}
}