package tests

import (
	"USDTMore/app/model"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// TestSimpleDecimalPerformance 简单的decimal性能测试
func TestSimpleDecimalPerformance(t *testing.T) {
	// 使用环境变量或默认值
	dsn := "host=localhost user=postgres password=postgres dbname=usdtmore port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("跳过测试：无法连接数据库 - %v", err)
		return
	}

	// 确保表结构已创建
	err = database.AutoMigrate(&model.TradeOrders{})
	require.NoError(t, err, "数据库迁移失败")

	// 测试1: 基本CRUD操作性能
	t.Run("BasicCRUDPerformance", func(t *testing.T) {
		// 创建测试订单
		amount, _ := decimal.NewFromString("123.45678901")
		money, _ := decimal.NewFromString("888.88")
		usdtRate, _ := decimal.NewFromString("7.2")
		
		order := &model.TradeOrders{
			OrderId:   fmt.Sprintf("simple_test_%d", time.Now().Unix()),
			TradeId:   fmt.Sprintf("trade_%d", time.Now().Unix()),
			Amount:    amount,
			Money:     money,
			UsdtRate:  usdtRate,
			Chain:     "TRON",
			Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
			Status:    1,
			ExpiredAt: time.Now().Add(time.Hour),
		}

		// 插入性能
		start := time.Now()
		err := database.Create(order).Error
		insertTime := time.Since(start)
		require.NoError(t, err, "插入失败")
		t.Logf("插入单条记录耗时: %v", insertTime)

		// 查询性能
		start = time.Now()
		var found model.TradeOrders
		err = database.Where("order_id = ?", order.OrderId).First(&found).Error
		queryTime := time.Since(start)
		require.NoError(t, err, "查询失败")
		t.Logf("查询单条记录耗时: %v", queryTime)

		// 验证decimal精度
		assert.True(t, found.Amount.Equal(amount), 
			"Amount精度不匹配: 期望=%s, 实际=%s", 
			amount.String(), found.Amount.String())
		assert.True(t, found.Money.Equal(money),
			"Money精度不匹配: 期望=%s, 实际=%s",
			money.String(), found.Money.String())
		assert.True(t, found.UsdtRate.Equal(usdtRate),
			"UsdtRate精度不匹配: 期望=%s, 实际=%s",
			usdtRate.String(), found.UsdtRate.String())

		// 更新性能
		newAmount, _ := decimal.NewFromString("999.99999999")
		start = time.Now()
		err = database.Model(&model.TradeOrders{}).
			Where("order_id = ?", order.OrderId).
			Update("amount", newAmount).Error
		updateTime := time.Since(start)
		require.NoError(t, err, "更新失败")
		t.Logf("更新单条记录耗时: %v", updateTime)

		// 验证更新后的精度
		err = database.Where("order_id = ?", order.OrderId).First(&found).Error
		require.NoError(t, err)
		assert.True(t, found.Amount.Equal(newAmount),
			"更新后Amount精度不匹配: 期望=%s, 实际=%s",
			newAmount.String(), found.Amount.String())

		// 删除性能
		start = time.Now()
		err = database.Where("order_id = ?", order.OrderId).Delete(&model.TradeOrders{}).Error
		deleteTime := time.Since(start)
		require.NoError(t, err, "删除失败")
		t.Logf("删除单条记录耗时: %v", deleteTime)

		// 性能总结
		t.Logf("\n性能总结:\n  插入: %v\n  查询: %v\n  更新: %v\n  删除: %v",
			insertTime, queryTime, updateTime, deleteTime)
	})

	// 测试2: 批量操作性能
	t.Run("BatchOperationsPerformance", func(t *testing.T) {
		batchSize := 100
		orders := make([]*model.TradeOrders, batchSize)
		
		for i := 0; i < batchSize; i++ {
			amount, _ := decimal.NewFromString(fmt.Sprintf("%.8f", 100.0+float64(i)*0.01))
			money, _ := decimal.NewFromString(fmt.Sprintf("%.2f", 720.0+float64(i)*0.1))
			usdtRate, _ := decimal.NewFromString("7.2")
			
			orders[i] = &model.TradeOrders{
				OrderId:   fmt.Sprintf("batch_%d_%d", time.Now().Unix(), i),
				TradeId:   fmt.Sprintf("trade_%d_%d", time.Now().Unix(), i),
				Amount:    amount,
				Money:     money,
				UsdtRate:  usdtRate,
				Chain:     "TRON",
				Address:   "TQn9Y2khEsLJW1ChVWFMSMeRDow5KcbLSE",
				Status:    1,
				ExpiredAt: time.Now().Add(time.Hour),
			}
		}

		// 批量插入
		start := time.Now()
		err := database.CreateInBatches(orders, 20).Error
		batchInsertTime := time.Since(start)
		require.NoError(t, err, "批量插入失败")
		t.Logf("批量插入 %d 条记录耗时: %v (%.2f records/s)", 
			batchSize, batchInsertTime, float64(batchSize)/batchInsertTime.Seconds())

		// 批量查询
		start = time.Now()
		var results []model.TradeOrders
		err = database.Where("order_id LIKE ?", fmt.Sprintf("batch_%d%%", time.Now().Unix())).
			Find(&results).Error
		batchQueryTime := time.Since(start)
		require.NoError(t, err, "批量查询失败")
		t.Logf("批量查询 %d 条记录耗时: %v", len(results), batchQueryTime)

		// 清理测试数据
		err = database.Where("order_id LIKE ?", fmt.Sprintf("batch_%d%%", time.Now().Unix())).
			Delete(&model.TradeOrders{}).Error
		require.NoError(t, err, "清理测试数据失败")
	})

	// 测试3: Decimal计算性能
	t.Run("DecimalCalculationPerformance", func(t *testing.T) {
		iterations := 10000
		
		// 创建性能
		start := time.Now()
		for i := 0; i < iterations; i++ {
			_, _ = decimal.NewFromString(fmt.Sprintf("%.8f", float64(i)*0.00000001))
		}
		createTime := time.Since(start)
		t.Logf("创建 %d 个decimal对象耗时: %v (%.0f ops/s)",
			iterations, createTime, float64(iterations)/createTime.Seconds())

		// 加法性能
		value1, _ := decimal.NewFromString("123.45678901")
		value2, _ := decimal.NewFromString("987.65432109")
		start = time.Now()
		for i := 0; i < iterations; i++ {
			_ = value1.Add(value2)
		}
		addTime := time.Since(start)
		t.Logf("执行 %d 次加法耗时: %v (%.0f ops/s)",
			iterations, addTime, float64(iterations)/addTime.Seconds())

		// 乘法性能
		multiplier, _ := decimal.NewFromString("7.2")
		start = time.Now()
		for i := 0; i < iterations; i++ {
			_ = value1.Mul(multiplier)
		}
		mulTime := time.Since(start)
		t.Logf("执行 %d 次乘法耗时: %v (%.0f ops/s)",
			iterations, mulTime, float64(iterations)/mulTime.Seconds())

		// 比较性能
		start = time.Now()
		for i := 0; i < iterations; i++ {
			_ = value1.LessThan(value2)
		}
		compareTime := time.Since(start)
		t.Logf("执行 %d 次比较耗时: %v (%.0f ops/s)",
			iterations, compareTime, float64(iterations)/compareTime.Seconds())
	})
}

// TestDecimalPrecisionValidation 验证decimal精度保持
func TestDecimalPrecisionValidation(t *testing.T) {
	// 测试各种精度的decimal值
	testCases := []struct {
		name     string
		value    string
		expected string
	}{
		{"整数", "100", "100"},
		{"两位小数", "100.50", "100.50"},
		{"八位小数", "123.45678901", "123.45678901"},
		{"科学计数法小数", "0.00000001", "0.00000001"},
		{"大数值", "999999999.99999999", "999999999.99999999"},
		{"负数", "-123.45678901", "-123.45678901"},
		{"零值", "0", "0"},
		{"小于1的正数", "0.12345678", "0.12345678"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建decimal值
			value, err := decimal.NewFromString(tc.value)
			require.NoError(t, err, "创建decimal失败: %s", tc.value)
			
			// 验证字符串表示
			assert.Equal(t, tc.expected, value.String(),
				"字符串表示不匹配: 输入=%s, 期望=%s, 实际=%s",
				tc.value, tc.expected, value.String())
			
			// 验证往返转换
			str := value.String()
			roundTrip, err := decimal.NewFromString(str)
			require.NoError(t, err, "往返转换失败")
			assert.True(t, value.Equal(roundTrip),
				"往返转换后值不相等: 原始=%s, 往返=%s",
				value.String(), roundTrip.String())
		})
	}
}

// BenchmarkDecimalVsFloat 对比decimal和float64的性能
func BenchmarkDecimalVsFloat(b *testing.B) {
	b.Run("DecimalAddition", func(b *testing.B) {
		value1, _ := decimal.NewFromString("123.45678901")
		value2, _ := decimal.NewFromString("987.65432109")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = value1.Add(value2)
		}
	})

	b.Run("Float64Addition", func(b *testing.B) {
		value1 := 123.45678901
		value2 := 987.65432109
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = value1 + value2
		}
	})

	b.Run("DecimalMultiplication", func(b *testing.B) {
		value1, _ := decimal.NewFromString("123.45678901")
		value2, _ := decimal.NewFromString("7.2")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = value1.Mul(value2)
		}
	})

	b.Run("Float64Multiplication", func(b *testing.B) {
		value1 := 123.45678901
		value2 := 7.2
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = value1 * value2
		}
	})
}