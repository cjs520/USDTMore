package tests

import (
	"USDTMore/app/help"
	"USDTMore/app/model"
	"USDTMore/tests/new_compatibility/testutils"
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
)

// DecimalPrecisionTestSuite decimal精度验证测试套件
type DecimalPrecisionTestSuite struct {
	suite.Suite
	ctx       context.Context
	container testcontainers.Container
}

func (suite *DecimalPrecisionTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	var err error
	suite.container, testutils.TestDB = testutils.SetupTestDB(suite.ctx, suite.T())
	require.NoError(suite.T(), err)
}

func (suite *DecimalPrecisionTestSuite) TearDownSuite() {
	testutils.TeardownTestDB(suite.ctx, suite.container)
}

func (suite *DecimalPrecisionTestSuite) SetupTest() {
	testutils.CleanDatabase(testutils.TestDB)
}

// TestDecimalPrecisionMaintenance 测试decimal精度保持
func (suite *DecimalPrecisionTestSuite) TestDecimalPrecisionMaintenance() {
	t := suite.T()

	// 1. 测试金额计算的精度保持
	suite.Run("AmountCalculationPrecision", func() {
		testCases := []struct {
			name     string
			money    string
			rate     string
			expected string
		}{
			{
				name:     "标准计算",
				money:    "100.00",
				rate:     "6.9000",
				expected: "14.49275362", // 100.00 / 6.9000
			},
			{
				name:     "高精度计算",
				money:    "1000.99",
				rate:     "6.87654321",
				expected: "145.53874642", // 1000.99 / 6.87654321
			},
			{
				name:     "小额计算",
				money:    "0.01",
				rate:     "7.0000",
				expected: "0.00142857", // 0.01 / 7.0000
			},
			{
				name:     "大额计算",
				money:    "999999.99",
				rate:     "6.5000",
				expected: "153846.15230769", // 999999.99 / 6.5000
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				money, err := decimal.NewFromString(tc.money)
				require.NoError(t, err)
				
				rate, err := decimal.NewFromString(tc.rate)
				require.NoError(t, err)

				// 使用help包的计算函数
				result, err := help.CalculateUSDTAmount(money, rate)
				require.NoError(t, err)

				expected, err := decimal.NewFromString(tc.expected)
				require.NoError(t, err)

				// 验证计算精度（允许微小的精度差异）
				diff := result.Sub(expected).Abs()
				maxDiff := func() decimal.Decimal { d, _ := decimal.NewFromString("0.00000001"); return d }() // 1e-8的精度差异
				assert.True(t, diff.LessThanOrEqual(maxDiff), 
					"计算结果精度差异过大: 期望=%s, 实际=%s, 差异=%s", 
					expected.String(), result.String(), diff.String())
			})
		}
	})

	// 2. 验证decimal到string的转换准确性
	suite.Run("DecimalToStringConversion", func() {
		testValues := []struct {
			decimal  string
			expected string
		}{
			{"0", "0"},
			{"0.1", "0.1"},
			{"0.12345678", "0.12345678"},
			{"1234567890.12345678", "1234567890.12345678"},
			{"0.00000001", "0.00000001"},
			{"999999999.99999999", "999999999.99999999"},
		}

		for _, tv := range testValues {
			d, err := decimal.NewFromString(tv.decimal)
			require.NoError(t, err)

			result := d.String()
			assert.Equal(t, tv.expected, result, 
				"decimal转string结果不匹配: 输入=%s, 期望=%s, 实际=%s", 
				tv.decimal, tv.expected, result)

			// 测试格式化输出
			formatted := help.FormatCryptoFixed(d)
			assert.NotEmpty(t, formatted, "格式化结果不应为空")
			
			// 验证格式化后的字符串可以转回decimal
			parsed, err := decimal.NewFromString(formatted)
			require.NoError(t, err)
			assert.True(t, d.Equal(parsed), "格式化后应该能正确解析回原值")
		}
	})

	// 3. 测试浮点数到decimal的安全转换
	suite.Run("SafeFloatToDecimalConversion", func() {
		testFloats := []struct {
			input    float64
			expected string
			hasError bool
		}{
			{100.0, "100", false},
			{100.5, "100.5", false},
			{0.1, "0.1", false},
			{0.12345678, "0.12345678", false},
			{math.NaN(), "", true},
			{math.Inf(1), "", true},
			{math.Inf(-1), "", true},
			{-100.5, "-100.5", false},
			{1e-8, "0.00000001", false},
			{1e20, "100000000000000000000", false},
		}

		for _, tf := range testFloats {
			result, err := help.SafeDecimalFromFloat(tf.input)
			
			if tf.hasError {
				assert.Error(t, err, "输入 %f 应该产生错误", tf.input)
			} else {
				require.NoError(t, err, "输入 %f 不应该产生错误", tf.input)
				
				// 对于浮点精度问题，使用合理的比较方式
				expected, parseErr := decimal.NewFromString(tf.expected)
				require.NoError(t, parseErr)
				
				// 允许微小的浮点精度差异
				diff := result.Sub(expected).Abs()
				maxDiff := func() decimal.Decimal { d, _ := decimal.NewFromString("0.000000001"); return d }() // 1e-9
				assert.True(t, diff.LessThanOrEqual(maxDiff),
					"浮点转decimal结果差异过大: 输入=%.15f, 期望=%s, 实际=%s, 差异=%s",
					tf.input, expected.String(), result.String(), diff.String())
			}
		}
	})

	// 4. 测试数据库往返的精度保持
	suite.Run("DatabaseRoundTripPrecision", func() {
		testAmounts := []string{
			"0.00000001",
			"0.12345678",
			"100.12345678",
			"1234567890.12345678",
			"999999999.99999999",
		}

		for i, amountStr := range testAmounts {
			amount, err := decimal.NewFromString(amountStr)
			require.NoError(t, err)

			// 创建订单
			usdtRate, _ := decimal.NewFromString("7.12345678")
			money, _ := decimal.NewFromString("100.50")
			order := &model.TradeOrders{
				OrderId:   fmt.Sprintf("precision_test_%d", i),
				TradeId:   fmt.Sprintf("precision_trade_%d", i),
				UsdtRate:  usdtRate,
				Amount:    amount,
				Money:     money,
				Chain:     "TRON",
				Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
				Status:    1,
				Version:   0,
				ExpiredAt: time.Now().Add(time.Hour),
			}

			// 保存到数据库
			err = testutils.TestDB.Create(order).Error
			require.NoError(t, err)

			// 从数据库读取
			var retrieved model.TradeOrders
			err = testutils.TestDB.Where("order_id = ?", order.OrderId).First(&retrieved).Error
			require.NoError(t, err)

			// 验证精度保持
			assert.True(t, amount.Equal(retrieved.Amount),
				"数据库往返后精度丢失: 原始=%s, 读取=%s",
				amount.String(), retrieved.Amount.String())

			// 验证字符串表示一致
			assert.Equal(t, amountStr, retrieved.Amount.String(),
				"数据库往返后字符串表示不一致")
		}
	})

	// 5. 测试原子精度操作
	suite.Run("AtomicPrecisionOperations", func() {
		atom := decimal.NewFromFloat(model.Atomicity) // 0.01
		baseAmount, _ := decimal.NewFromString("100.00")

		// 测试原子增量
		for i := 0; i < 10; i++ {
			increment := help.AddAtomicIncrement(baseAmount, i, atom)
			
			// 验证增量结果
			expected := baseAmount.Add(atom.Mul(decimal.NewFromInt(int64(i))))
			tolerance, _ := decimal.NewFromString("0.001")
			assert.True(t, increment.Equal(expected) || increment.Sub(expected).Abs().LessThan(tolerance),
				"原子增量计算错误: 步骤=%d, 期望=%s, 实际=%s",
				i, expected.String(), increment.String())
		}
	})

	// 6. 测试边界值精度处理
	suite.Run("BoundaryValuePrecision", func() {
		boundaryValues := []string{
			"0.00000001",          // 最小精度
			"0.99999999",          // 接近1
			"1.00000000",          // 整数
			"1.00000001",          // 略大于整数
			"999999999.99999999",  // 最大值
			"1000000000.00000000", // 超出numeric(18,8)整数部分的值
		}

		for _, valueStr := range boundaryValues {
			value, err := decimal.NewFromString(valueStr)
			require.NoError(t, err)

			// 测试数据库存储
			usdtRate, _ := decimal.NewFromString("7.0")
			money, _ := decimal.NewFromString("100.00")
			order := &model.TradeOrders{
				OrderId:   fmt.Sprintf("boundary_%s", strings.ReplaceAll(valueStr, ".", "_")),
				TradeId:   fmt.Sprintf("boundary_trade_%s", strings.ReplaceAll(valueStr, ".", "_")),
				UsdtRate:  usdtRate,
				Amount:    value,
				Money:     money,
				Chain:     "TRON",
				Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
				Status:    1,
				Version:   0,
				ExpiredAt: time.Now().Add(time.Hour),
			}

			err = testutils.TestDB.Create(order).Error
			
			// 某些边界值可能会导致错误（如超出精度范围）
			if valueStr == "1000000000.00000000" {
				// 这个值超出了numeric(18,8)的整数部分范围，应该失败
				assert.Error(t, err, "超出精度范围的值应该导致错误")
			} else {
				require.NoError(t, err, "边界值 %s 应该能正常存储", valueStr)
				
				// 验证读取的精度
				var retrieved model.TradeOrders
				err = testutils.TestDB.Where("order_id = ?", order.OrderId).First(&retrieved).Error
				require.NoError(t, err)
				
				assert.True(t, value.Equal(retrieved.Amount),
					"边界值精度丢失: 原始=%s, 读取=%s",
					value.String(), retrieved.Amount.String())
			}
		}
	})
}

// TestDecimalAPIResponseFormat 测试API响应格式的正确性
func (suite *DecimalPrecisionTestSuite) TestDecimalAPIResponseFormat() {
	t := suite.T()

	// 1. 确保API响应格式的正确性
	suite.Run("APIResponseFormatConsistency", func() {
		// 创建测试订单
		usdtRate, _ := decimal.NewFromString("6.87654321")
		amount, _ := decimal.NewFromString("123.12345678")
		money, _ := decimal.NewFromString("846.88")
		order := &model.TradeOrders{
			OrderId:   "api_format_test",
			TradeId:   "api_format_trade",
			UsdtRate:  usdtRate,
			Amount:    amount,
			Money:     money,
			Chain:     "TRON",
			Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			Status:    1,
			Version:   0,
			ExpiredAt: time.Now().Add(time.Hour),
		}

		err := testutils.TestDB.Create(order).Error
		require.NoError(t, err)

		// 读取订单并格式化为API响应
		var retrieved model.TradeOrders
		err = testutils.TestDB.Where("order_id = ?", "api_format_test").First(&retrieved).Error
		require.NoError(t, err)

		// 测试各种格式化输出
		usdtRateStr := help.FormatCryptoFixed(retrieved.UsdtRate)
		amountStr := help.FormatCryptoFixed(retrieved.Amount)
		moneyStr := help.FormatMoney(retrieved.Money)

		// 验证格式化结果
		assert.Equal(t, "6.87654321", usdtRateStr, "USDT汇率格式化错误")
		assert.Equal(t, "123.12345678", amountStr, "交易金额格式化错误")
		assert.Equal(t, "846.88", moneyStr, "订单金额格式化错误")

		// 验证格式化结果可以转回decimal
		parsedRate, err := decimal.NewFromString(usdtRateStr)
		require.NoError(t, err)
		assert.True(t, retrieved.UsdtRate.Equal(parsedRate), "汇率格式化后应该能正确解析")

		parsedAmount, err := decimal.NewFromString(amountStr)
		require.NoError(t, err)
		assert.True(t, retrieved.Amount.Equal(parsedAmount), "金额格式化后应该能正确解析")

		parsedMoney, err := decimal.NewFromString(moneyStr)
		require.NoError(t, err)
		assert.True(t, retrieved.Money.Equal(parsedMoney), "订单金额格式化后应该能正确解析")
	})

	// 2. 测试金额显示的一致性
	suite.Run("AmountDisplayConsistency", func() {
		testCases := []struct {
			amount   string
			expected string
		}{
			{"100.00000000", "100"},
			{"100.10000000", "100.1"},
			{"100.12345678", "100.12345678"},
			{"0.00000001", "0.00000001"},
		}

		for _, tc := range testCases {
			amount, err := decimal.NewFromString(tc.amount)
			require.NoError(t, err)

			formatted := help.FormatCryptoFixed(amount)
			assert.Equal(t, tc.expected, formatted,
				"金额 %s 的格式化结果应该是 %s，实际是 %s",
				tc.amount, tc.expected, formatted)
		}
	})

	// 3. 测试货币格式化一致性
	suite.Run("CurrencyFormatConsistency", func() {
		testCases := []struct {
			money    string
			expected string
		}{
			{"100.00", "100.00"},
			{"100.50", "100.50"},
			{"1000.99", "1000.99"},
			{"0.01", "0.01"},
		}

		for _, tc := range testCases {
			money, err := decimal.NewFromString(tc.money)
			require.NoError(t, err)

			formatted := help.FormatMoney(money)
			assert.Equal(t, tc.expected, formatted,
				"货币 %s 的格式化结果应该是 %s，实际是 %s",
				tc.money, tc.expected, formatted)
		}
	})
}

// TestDecimalCalculationAccuracy 测试decimal计算准确性
func (suite *DecimalPrecisionTestSuite) TestDecimalCalculationAccuracy() {
	t := suite.T()

	// 1. 测试复杂计算的精度保持
	suite.Run("ComplexCalculationPrecision", func() {
		// 模拟实际业务计算场景
		scenarios := []struct {
			name     string
			money    string
			rate     string
			expected string
		}{
			{
				name:     "标准订单计算",
				money:    "100.00",
				rate:     "6.9000",
				expected: "14.49275362",
			},
			{
				name:     "高精度汇率计算",
				money:    "500.50",
				rate:     "6.87654321",
				expected: "72.77937321",
			},
			{
				name:     "小额支付计算",
				money:    "1.00",
				rate:     "7.2500",
				expected: "0.13793103",
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				money, err := decimal.NewFromString(scenario.money)
				require.NoError(t, err)

				rate, err := decimal.NewFromString(scenario.rate)
				require.NoError(t, err)

				// 使用业务逻辑计算
				result, err := help.CalculateUSDTAmount(money, rate)
				require.NoError(t, err)

				expected, err := decimal.NewFromString(scenario.expected)
				require.NoError(t, err)

				// 验证计算精度
				diff := result.Sub(expected).Abs()
				maxDiff, _ := decimal.NewFromString("0.00000001")
				assert.True(t, diff.LessThanOrEqual(maxDiff),
					"计算结果精度差异: 场景=%s, 期望=%s, 实际=%s, 差异=%s",
					scenario.name, expected.String(), result.String(), diff.String())

				// 测试计算结果的数据库存储和检索
				order := &model.TradeOrders{
					OrderId:   fmt.Sprintf("calc_%s", strings.ReplaceAll(scenario.name, " ", "_")),
					TradeId:   fmt.Sprintf("calc_trade_%s", strings.ReplaceAll(scenario.name, " ", "_")),
					UsdtRate:  rate,
					Amount:    result,
					Money:     money,
					Chain:     "TRON",
					Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
					Status:    1,
					Version:   0,
					ExpiredAt: time.Now().Add(time.Hour),
				}

				err = testutils.TestDB.Create(order).Error
				require.NoError(t, err)

				var retrieved model.TradeOrders
				err = testutils.TestDB.Where("order_id = ?", order.OrderId).First(&retrieved).Error
				require.NoError(t, err)

				// 验证数据库往返精度
				assert.True(t, result.Equal(retrieved.Amount),
					"数据库往返后计算结果精度丢失: 原始=%s, 读取=%s",
					result.String(), retrieved.Amount.String())
			})
		}
	})

	// 2. 测试累积计算误差
	suite.Run("AccumulatedCalculationError", func() {
		// 测试多次小额计算是否会产生累积误差
		baseAmount, _ := decimal.NewFromString("0.00000001")
		atom := decimal.NewFromFloat(model.Atomicity)
		
		accumulated := baseAmount
		for i := 0; i < 1000; i++ {
			accumulated = accumulated.Add(atom)
		}

		// 预期结果
		expected := baseAmount.Add(atom.Mul(decimal.NewFromInt(1000)))
		
		// 验证累积计算没有误差
		assert.True(t, accumulated.Equal(expected),
			"累积计算产生误差: 期望=%s, 实际=%s",
			expected.String(), accumulated.String())

		// 验证累积结果可以正确存储和检索
		usdtRate, _ := decimal.NewFromString("7.0")
		money, _ := decimal.NewFromString("100.00")
		order := &model.TradeOrders{
			OrderId:   "accumulated_test",
			TradeId:   "accumulated_trade",
			UsdtRate:  usdtRate,
			Amount:    accumulated,
			Money:     money,
			Chain:     "TRON",
			Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			Status:    1,
			Version:   0,
			ExpiredAt: time.Now().Add(time.Hour),
		}

		err := testutils.TestDB.Create(order).Error
		require.NoError(t, err)

		var retrieved model.TradeOrders
		err = testutils.TestDB.Where("order_id = ?", "accumulated_test").First(&retrieved).Error
		require.NoError(t, err)

		assert.True(t, accumulated.Equal(retrieved.Amount),
			"累积计算结果数据库往返精度丢失: 原始=%s, 读取=%s",
			accumulated.String(), retrieved.Amount.String())
	})
}

// TestDecimalPrecision 运行decimal精度验证测试
func TestDecimalPrecision(t *testing.T) {
	suite.Run(t, new(DecimalPrecisionTestSuite))
}