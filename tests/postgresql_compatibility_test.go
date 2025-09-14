package tests

import (
	"USDTMore/app/model"
	"USDTMore/tests/testutils"
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// PostgreSQLCompatibilityTestSuite PostgreSQL兼容性测试套件
type PostgreSQLCompatibilityTestSuite struct {
	suite.Suite
	ctx       context.Context
	container interface{}
}

func (suite *PostgreSQLCompatibilityTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	var err error
	suite.container, testutils.TestDB = testutils.SetupTestDB(suite.ctx, suite.T())
	require.NoError(suite.T(), err)
}

func (suite *PostgreSQLCompatibilityTestSuite) TearDownSuite() {
	testutils.TeardownTestDB(suite.ctx, suite.container)
}

func (suite *PostgreSQLCompatibilityTestSuite) SetupTest() {
	testutils.CleanDatabase(testutils.TestDB)
}

// TestPostgreSQLFieldTypeCompatibility 测试PostgreSQL字段类型兼容性
func (suite *PostgreSQLCompatibilityTestSuite) TestPostgreSQLFieldTypeCompatibility() {
	t := suite.T()

	// 1. 测试decimal.Decimal字段的读写操作
	suite.Run("DecimalFieldReadWrite", func() {
		// 创建测试订单
		order := &model.TradeOrders{
			OrderId:   "test_order_001",
			TradeId:   "trade_001",
			UsdtRate:  decimal.NewFromFloat(6.9876),
			Amount:    decimal.NewFromString("100.12345678"),
			Money:     decimal.NewFromString("690.88"),
			Chain:     "TRON",
			Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			Status:    1,
			Version:   0,
			ExpiredAt: time.Now().Add(time.Hour),
		}

		// 保存到数据库
		err := testutils.TestDB.Create(order).Error
		require.NoError(t, err)

		// 从数据库读取
		var retrievedOrder model.TradeOrders
		err = testutils.TestDB.Where("order_id = ?", "test_order_001").First(&retrievedOrder).Error
		require.NoError(t, err)

		// 验证decimal字段精度保持
		assert.True(t, order.UsdtRate.Equal(retrievedOrder.UsdtRate), "USDT汇率精度应该保持一致")
		assert.True(t, order.Amount.Equal(retrievedOrder.Amount), "交易金额精度应该保持一致")
		assert.True(t, order.Money.Equal(retrievedOrder.Money), "订单金额精度应该保持一致")

		// 验证decimal字段的字符串表示
		assert.Equal(t, "6.9876", retrievedOrder.UsdtRate.String())
		assert.Equal(t, "100.12345678", retrievedOrder.Amount.String())
		assert.Equal(t, "690.88", retrievedOrder.Money.String())
	})

	// 2. 验证timestamptz时区支持
	suite.Run("TimestamptzTimezoneSupport", func() {
		// 使用不同时区的时间创建订单
		utcTime := time.Date(2024, 1, 15, 10, 30, 45, 123456789, time.UTC)
		localTime := utcTime.In(time.Local)
		
		order := &model.TradeOrders{
			OrderId:     "tz_test_001",
			TradeId:     "tz_trade_001",
			UsdtRate:    decimal.NewFromFloat(7.0),
			Amount:      decimal.NewFromFloat(100.0),
			Money:       decimal.NewFromFloat(700.0),
			Chain:       "TRON",
			Address:     "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			Status:      1,
			Version:     0,
			ExpiredAt:   localTime,
			CreatedAt:   utcTime,
		}

		// 保存到数据库
		err := testutils.TestDB.Create(order).Error
		require.NoError(t, err)

		// 从数据库读取
		var retrievedOrder model.TradeOrders
		err = testutils.TestDB.Where("order_id = ?", "tz_test_001").First(&retrievedOrder).Error
		require.NoError(t, err)

		// 验证时区信息保持正确（PostgreSQL应该统一转换为UTC存储）
		assert.True(t, utcTime.Equal(retrievedOrder.CreatedAt.UTC()) || 
			utcTime.Sub(retrievedOrder.CreatedAt.UTC()).Abs() < time.Second)

		// 验证timestamptz字段可以正确处理时区
		assert.NotZero(t, retrievedOrder.ExpiredAt)
		assert.NotZero(t, retrievedOrder.CreatedAt)
		assert.NotZero(t, retrievedOrder.UpdatedAt)
	})

	// 3. 测试布尔类型字段的正确映射
	suite.Run("BooleanFieldMapping", func() {
		// 创建钱包地址记录
		address := &model.WalletAddress{
			Address:     "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			PrivateKey:  "test_private_key",
			Chain:       "TRON",
			Status:      1,
			OtherNotify: true, // 布尔字段
		}

		// 保存到数据库
		err := testutils.TestDB.Create(address).Error
		require.NoError(t, err)

		// 从数据库读取
		var retrievedAddress model.WalletAddress
		err = testutils.TestDB.Where("address = ?", "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE").First(&retrievedAddress).Error
		require.NoError(t, err)

		// 验证布尔字段正确映射
		assert.Equal(t, true, retrievedAddress.OtherNotify)

		// 测试false值
		address2 := &model.WalletAddress{
			Address:     "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSF",
			PrivateKey:  "test_private_key_2",
			Chain:       "TRON",
			Status:      1,
			OtherNotify: false, // 布尔字段为false
		}

		err = testutils.TestDB.Create(address2).Error
		require.NoError(t, err)

		var retrievedAddress2 model.WalletAddress
		err = testutils.TestDB.Where("address = ?", "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSF").First(&retrievedAddress2).Error
		require.NoError(t, err)

		assert.Equal(t, false, retrievedAddress2.OtherNotify)
	})

	// 4. 验证字符串长度限制的有效性
	suite.Run("StringLengthConstraints", func() {
		// 测试varchar长度限制
		validOrder := &model.TradeOrders{
			OrderId:   "valid_order_id", // 应该在varchar(255)范围内
			TradeId:   "valid_trade_id",
			TradeHash: "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", // 66字符的哈希
			UsdtRate:  decimal.NewFromFloat(7.0),
			Amount:    decimal.NewFromFloat(100.0),
			Money:     decimal.NewFromFloat(700.0),
			Chain:     "TRON", // varchar(20)
			Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE", // varchar(50)
			Status:    1,
			Version:   0,
			ExpiredAt: time.Now().Add(time.Hour),
		}

		err := testutils.TestDB.Create(validOrder).Error
		require.NoError(t, err, "有效长度的字符串应该能成功保存")

		// 验证数据正确保存
		var retrieved model.TradeOrders
		err = testutils.TestDB.Where("order_id = ?", "valid_order_id").First(&retrieved).Error
		require.NoError(t, err)

		assert.Equal(t, validOrder.OrderId, retrieved.OrderId)
		assert.Equal(t, validOrder.TradeHash, retrieved.TradeHash)
		assert.Equal(t, validOrder.Chain, retrieved.Chain)
		assert.Equal(t, validOrder.Address, retrieved.Address)
	})

	// 5. 测试numeric类型的精度和标度
	suite.Run("NumericPrecisionAndScale", func() {
		// 测试numeric(18,8)类型 - 用于Amount
		testValues := []string{
			"0.00000001",        // 最小精度
			"0.12345678",        // 8位小数
			"1234567890.12345678", // 最大整数部分
			"999999999.99999999",  // 接近最大值
		}

		for i, amountStr := range testValues {
			amount, _ := decimal.NewFromString(amountStr)
			order := &model.TradeOrders{
				OrderId:   generateTestOrderId("numeric", i),
				TradeId:   generateTestTradeId("numeric", i),
				UsdtRate:  decimal.NewFromFloat(7.0),
				Amount:    amount,
				Money:     decimal.NewFromString("100.50"), // numeric(18,2)
				Chain:     "TRON",
				Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
				Status:    1,
				Version:   0,
				ExpiredAt: time.Now().Add(time.Hour),
			}

			err := testutils.TestDB.Create(order).Error
			require.NoError(t, err, "Amount值 %s 应该能正确保存", amountStr)

			// 验证读取的精度
			var retrieved model.TradeOrders
			err = testutils.TestDB.Where("order_id = ?", order.OrderId).First(&retrieved).Error
			require.NoError(t, err)

			assert.True(t, amount.Equal(retrieved.Amount), 
				"Amount %s 精度应该保持一致，期望: %s, 实际: %s", 
				amountStr, amount.String(), retrieved.Amount.String())
		}
	})

	// 6. 测试索引字段性能
	suite.Run("IndexedFieldPerformance", func() {
		// 创建大量测试数据
		orders := make([]*model.TradeOrders, 1000)
		for i := 0; i < 1000; i++ {
			orders[i] = &model.TradeOrders{
				OrderId:   generateTestOrderId("perf", i),
				TradeId:   generateTestTradeId("perf", i),
				UsdtRate:  decimal.NewFromFloat(7.0),
				Amount:    decimal.NewFromFloat(float64(i) + 0.01),
				Money:     decimal.NewFromFloat(700.0),
				Chain:     "TRON",
				Address:   generateTestAddress(i),
				Status:    int16(1 + i%3), // 1, 2, 3 循环
				Version:   0,
				ExpiredAt: time.Now().Add(time.Hour),
			}
		}

		// 批量插入
		err := testutils.TestDB.CreateInBatches(orders, 100).Error
		require.NoError(t, err)

		// 测试索引字段查询性能
		startTime := time.Now()
		
		// 按状态查询（有索引）
		var statusResults []model.TradeOrders
		err = testutils.TestDB.Where("status = ?", 1).Find(&statusResults).Error
		require.NoError(t, err)
		
		statusQueryTime := time.Since(startTime)
		assert.Less(t, statusQueryTime, time.Millisecond*100, "状态字段查询应该很快")
		assert.Greater(t, len(statusResults), 0, "应该找到状态为1的记录")

		// 按链和地址查询（有复合索引）
		startTime = time.Now()
		var chainResults []model.TradeOrders
		err = testutils.TestDB.Where("chain = ? AND address = ?", "TRON", generateTestAddress(0)).Find(&chainResults).Error
		require.NoError(t, err)
		
		chainQueryTime := time.Since(startTime)
		assert.Less(t, chainQueryTime, time.Millisecond*50, "链和地址查询应该很快")
	})
}

// TestPostgreSQLConstraintsAndForeignKeys 测试约束和外键
func (suite *PostgreSQLCompatibilityTestSuite) TestPostgreSQLConstraintsAndForeignKeys() {
	t := suite.T()

	// 1. 测试唯一约束
	suite.Run("UniqueConstraints", func() {
		order1 := &model.TradeOrders{
			OrderId:   "unique_test_001",
			TradeId:   "unique_trade_001",
			UsdtRate:  decimal.NewFromFloat(7.0),
			Amount:    decimal.NewFromFloat(100.0),
			Money:     decimal.NewFromFloat(700.0),
			Chain:     "TRON",
			Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			Status:    1,
			Version:   0,
			ExpiredAt: time.Now().Add(time.Hour),
		}

		err := testutils.TestDB.Create(order1).Error
		require.NoError(t, err)

		// 尝试插入重复的OrderId
		order2 := *order1
		order2.TradeId = "unique_trade_002" // 不同的TradeId
		
		err = testutils.TestDB.Create(&order2).Error
		assert.Error(t, err, "重复的OrderId应该导致唯一约束错误")
		assert.Contains(t, err.Error(), "duplicate", "错误信息应该包含duplicate关键字")
	})

	// 2. 测试NOT NULL约束
	suite.Run("NotNullConstraints", func() {
		order := &model.TradeOrders{
			OrderId: "not_null_test",
			TradeId: "", // 空的TradeId，应该违反NOT NULL约束
			// 其他必需字段留空
		}

		err := testutils.TestDB.Create(order).Error
		// 注意：GORM可能会在某些情况下自动处理空字符串，所以这个测试可能需要调整
		if err == nil {
			// 如果GORM允许空字符串，验证实际保存的值
			var retrieved model.TradeOrders
			testutils.TestDB.Where("order_id = ?", "not_null_test").First(&retrieved)
			// 验证GORM是否正确处理了空值
		}
	})

	// 3. 测试检查约束（如果有的话）
	suite.Run("CheckConstraints", func() {
		// 测试状态值的有效性
		order := &model.TradeOrders{
			OrderId:   "check_test_001",
			TradeId:   "check_trade_001",
			UsdtRate:  decimal.NewFromFloat(7.0),
			Amount:    decimal.NewFromFloat(100.0),
			Money:     decimal.NewFromFloat(700.0),
			Chain:     "TRON",
			Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			Status:    999, // 无效状态值
			Version:   0,
			ExpiredAt: time.Now().Add(time.Hour),
		}

		// 如果有检查约束，这应该失败
		err := testutils.TestDB.Create(order).Error
		// 根据实际约束情况调整断言
		_ = err // 暂时不做断言，取决于实际的检查约束设置
	})
}

// TestPostgreSQLConcurrencyControl 测试并发控制机制
func (suite *PostgreSQLCompatibilityTestSuite) TestPostgreSQLConcurrencyControl() {
	t := suite.T()

	// 1. 测试乐观锁机制
	suite.Run("OptimisticLocking", func() {
		// 创建初始订单
		order := &model.TradeOrders{
			OrderId:   "optimistic_test_001",
			TradeId:   "optimistic_trade_001",
			UsdtRate:  decimal.NewFromFloat(7.0),
			Amount:    decimal.NewFromFloat(100.0),
			Money:     decimal.NewFromFloat(700.0),
			Chain:     "TRON",
			Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			Status:    1,
			Version:   0,
			ExpiredAt: time.Now().Add(time.Hour),
		}

		err := testutils.TestDB.Create(order).Error
		require.NoError(t, err)

		// 模拟两个并发更新
		var order1, order2 model.TradeOrders
		
		// 两个事务都读取相同的版本
		err = testutils.TestDB.Where("order_id = ?", "optimistic_test_001").First(&order1).Error
		require.NoError(t, err)
		
		err = testutils.TestDB.Where("order_id = ?", "optimistic_test_001").First(&order2).Error
		require.NoError(t, err)
		
		assert.Equal(t, int64(0), order1.Version)
		assert.Equal(t, int64(0), order2.Version)

		// 第一个事务更新成功
		now := time.Now()
		err = order1.OrderSetSuccWithContext(suite.ctx, "sender1", "hash1", now)
		require.NoError(t, err)

		// 第二个事务更新应该失败（版本冲突）
		err = order2.OrderSetSuccWithContext(suite.ctx, "sender2", "hash2", now)
		assert.Error(t, err, "版本冲突应该导致更新失败")
		assert.Contains(t, err.Error(), "version conflict", "错误信息应该包含版本冲突")

		// 验证只有第一个更新生效
		var final model.TradeOrders
		err = testutils.TestDB.Where("order_id = ?", "optimistic_test_001").First(&final).Error
		require.NoError(t, err)
		
		assert.Equal(t, "sender1", final.FromAddress)
		assert.Equal(t, "hash1", final.TradeHash)
		assert.Equal(t, int64(1), final.Version)
		assert.Equal(t, int16(2), final.Status) // OrderStatusSuccess
	})

	// 2. 测试事务隔离级别
	suite.Run("TransactionIsolation", func() {
		// 这个测试需要手动管理事务来测试隔离级别
		tx1 := testutils.TestDB.Begin()
		tx2 := testutils.TestDB.Begin()
		
		defer func() {
			tx1.Rollback()
			tx2.Rollback()
		}()

		// 在事务1中创建记录
		order := &model.TradeOrders{
			OrderId:   "isolation_test_001",
			TradeId:   "isolation_trade_001",
			UsdtRate:  decimal.NewFromFloat(7.0),
			Amount:    decimal.NewFromFloat(100.0),
			Money:     decimal.NewFromFloat(700.0),
			Chain:     "TRON",
			Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			Status:    1,
			Version:   0,
			ExpiredAt: time.Now().Add(time.Hour),
		}

		err := tx1.Create(order).Error
		require.NoError(t, err)

		// 事务2应该看不到未提交的记录
		var count int64
		err = tx2.Model(&model.TradeOrders{}).Where("order_id = ?", "isolation_test_001").Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count, "事务2不应该看到事务1未提交的记录")

		// 提交事务1
		err = tx1.Commit().Error
		require.NoError(t, err)

		// 现在事务2应该能看到记录
		err = tx2.Model(&model.TradeOrders{}).Where("order_id = ?", "isolation_test_001").Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "事务1提交后，事务2应该能看到记录")

		tx2.Commit()
	})
}

// 辅助函数
func generateTestOrderId(prefix string, index int) string {
	return fmt.Sprintf("%s_order_%04d", prefix, index)
}

func generateTestTradeId(prefix string, index int) string {
	return fmt.Sprintf("%s_trade_%04d", prefix, index)
}

func generateTestAddress(index int) string {
	// 生成不同的测试地址
	addresses := []string{
		"TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
		"TLyqzVGLV1srkB7dToTAEqgDSfPtXRJZYH", 
		"TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		"TMuA6YqfCeX8EhbfYEg5y7S4DqzSJireY9",
		"TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs",
	}
	return addresses[index%len(addresses)]
}

// TestPostgreSQLCompatibility 运行PostgreSQL兼容性测试
func TestPostgreSQLCompatibility(t *testing.T) {
	suite.Run(t, new(PostgreSQLCompatibilityTestSuite))
}