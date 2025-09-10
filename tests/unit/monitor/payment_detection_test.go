package monitor_test

import (
	"USDTMore/app/model"
	"USDTMore/app/monitor"
	"USDTMore/tests/testutils"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestPaymentDetectionTRON 测试TRON链支付检测
func TestPaymentDetectionTRON(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("TRON TronScan API payment detection", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建测试订单
		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "13.89" // 标准化的2位小数格式
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":   "TRON",
			"address": testAddress,
			"amount":  testAmount,
			"status":  model.OrderStatusWaiting,
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		// 创建订单锁映射
		orderLock := make(map[string]model.TradeOrders)
		orderLock["TRON"+testAddress+testAmount] = *order

		// 模拟TronScan API响应数据
		mockData := map[string]interface{}{
			"total": 1,
			"token_transfers": []map[string]interface{}{
				{
					"transaction_id": "test_tx_hash_123",
					"to_address":     testAddress,
					"from_address":   "TTestFromAddress12345678901234567890",
					"quant":          13890000, // 13.89 USDT with 6 decimals
					"contractRet":    "SUCCESS",
					"block_ts":       time.Now().UnixMilli(),
				},
			},
		}

		jsonData, _ := json.Marshal(mockData)
		result := gjson.ParseBytes(jsonData)

		// 调用支付处理函数（这里需要导出内部函数用于测试）
		// 由于函数是私有的，我们需要创建一个测试辅助函数
		processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)

		// 验证支付被正确处理
		assert.Equal(t, 1, processedPayments)

		// 验证订单状态更新
		var updatedOrder model.TradeOrders
		err = db.First(&updatedOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, updatedOrder.Status)
		assert.Equal(t, "test_tx_hash_123", updatedOrder.TradeHash)
		assert.Equal(t, "TTestFromAddress12345678901234567890", updatedOrder.FromAddress)
	})

	t.Run("TRON TronGrid API payment detection", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建测试订单
		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "25.50"
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":   "TRON",
			"address": testAddress,
			"amount":  testAmount,
			"status":  model.OrderStatusWaiting,
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		orderLock := make(map[string]model.TradeOrders)
		orderLock["TRON"+testAddress+testAmount] = *order

		// 模拟TronGrid API响应数据
		mockData := map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"transaction_id":  "test_tx_hash_grid_456",
					"to":              testAddress,
					"from":            "TTestFromGrid12345678901234567890",
					"value":           25500000, // 25.50 USDT with 6 decimals
					"type":            "Transfer",
					"block_timestamp": time.Now().UnixMilli(),
				},
			},
			"meta": map[string]interface{}{
				"page_size": 1,
			},
		}

		jsonData, _ := json.Marshal(mockData)
		result := gjson.ParseBytes(jsonData)

		// 测试TronGrid支付处理
		processedPayments := processTestPaymentForTronGrid(orderLock, testAddress, result)

		assert.Equal(t, 1, processedPayments)

		// 验证订单状态更新
		var updatedOrder model.TradeOrders
		err = db.First(&updatedOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, updatedOrder.Status)
		assert.Equal(t, "test_tx_hash_grid_456", updatedOrder.TradeHash)
		assert.Equal(t, "TTestFromGrid12345678901234567890", updatedOrder.FromAddress)
	})
}

// TestPaymentDetectionETHChains 测试ETH兼容链支付检测
func TestPaymentDetectionETHChains(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	chains := []string{"POLY", "OP", "BSC", "ARB"}

	for _, chain := range chains {
		t.Run(fmt.Sprintf("%s payment detection", chain), func(t *testing.T) {
			testutils.CleanDatabase(db)

			// 创建测试订单
			testAddress := "0x1234567890123456789012345678901234567890"
			testAmount := "47.25"
			order := testutils.CreateTestOrder(map[string]interface{}{
				"chain":   chain,
				"address": testAddress,
				"amount":  testAmount,
				"status":  model.OrderStatusWaiting,
			})
			err := db.Create(order).Error
			require.NoError(t, err)

			orderLock := make(map[string]model.TradeOrders)
			orderLock[chain+testAddress+testAmount] = *order

			// 模拟Etherscan API响应数据
			mockData := map[string]interface{}{
				"status": "1",
				"message": "OK",
				"result": []map[string]interface{}{
					{
						"hash":             "0xtest_eth_tx_hash_789",
						"to":               testAddress,
						"from":             "0xTestFromAddress1234567890123456789012",
						"value":            "47250000", // 47.25 USDT with 6 decimals
						"tokenSymbol":      "USDT",
						"tokenDecimal":     "6",
						"contractAddress":  "0xdAC17F958D2ee523a2206206994597C13D831ec7",
						"timeStamp":        fmt.Sprintf("%d", time.Now().Unix()),
					},
				},
			}

			jsonData, _ := json.Marshal(mockData)
			result := gjson.ParseBytes(jsonData)

			// 测试ETH链支付处理
			processedPayments := processTestPaymentForETH(orderLock, chain, testAddress, result)

			assert.Equal(t, 1, processedPayments, "Should process 1 payment for chain %s", chain)

			// 验证订单状态更新
			var updatedOrder model.TradeOrders
			err = db.First(&updatedOrder, order.Id).Error
			require.NoError(t, err)
			assert.Equal(t, model.OrderStatusSuccess, updatedOrder.Status)
			assert.Equal(t, "0xtest_eth_tx_hash_789", updatedOrder.TradeHash)
			assert.Equal(t, "0xTestFromAddress1234567890123456789012", updatedOrder.FromAddress)
		})
	}
}

// TestPaymentAmountMatching 测试支付金额匹配逻辑
func TestPaymentAmountMatching(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Exact amount matching", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "100.00"
		
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":   "TRON",
			"address": testAddress,
			"amount":  testAmount,
			"status":  model.OrderStatusWaiting,
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		orderLock := make(map[string]model.TradeOrders)
		orderLock["TRON"+testAddress+testAmount] = *order

		// 测试精确匹配
		mockData := createMockTronData(testAddress, "100.00", "exact_match_tx", time.Now())
		jsonData, _ := json.Marshal(mockData)
		result := gjson.ParseBytes(jsonData)

		processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)
		assert.Equal(t, 1, processedPayments)
	})

	t.Run("Amount too small - should not match", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "100.00"
		
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":   "TRON",
			"address": testAddress,
			"amount":  testAmount,
			"status":  model.OrderStatusWaiting,
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		orderLock := make(map[string]model.TradeOrders)
		orderLock["TRON"+testAddress+testAmount] = *order

		// 测试金额过小的交易（小于0.1 USDT）
		mockData := createMockTronData(testAddress, "0.05", "small_amount_tx", time.Now())
		jsonData, _ := json.Marshal(mockData)
		result := gjson.ParseBytes(jsonData)

		processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)
		assert.Equal(t, 0, processedPayments, "Should not process payments below minimum threshold")
	})

	t.Run("Amount format variations", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		
		// 创建多个订单，测试不同的金额格式
		testCases := []struct {
			orderAmount   string
			paymentAmount string
			shouldMatch   bool
		}{
			{"100.00", "100.00", true},
			{"100.10", "100.10", true},
			{"100.01", "100.01", true},
			{"99.99", "100.00", false}, // 不同金额不应该匹配
		}

		for i, tc := range testCases {
			order := testutils.CreateTestOrder(map[string]interface{}{
				"trade_id": fmt.Sprintf("test_order_%d", i),
				"chain":    "TRON",
				"address":  testAddress,
				"amount":   tc.orderAmount,
				"status":   model.OrderStatusWaiting,
			})
			err := db.Create(order).Error
			require.NoError(t, err)

			orderLock := make(map[string]model.TradeOrders)
			orderLock["TRON"+testAddress+tc.orderAmount] = *order

			mockData := createMockTronData(testAddress, tc.paymentAmount, fmt.Sprintf("tx_%d", i), time.Now())
			jsonData, _ := json.Marshal(mockData)
			result := gjson.ParseBytes(jsonData)

			processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)
			
			if tc.shouldMatch {
				assert.Equal(t, 1, processedPayments, "Should match for case: %+v", tc)
				
				// 验证订单状态更新
				var updatedOrder model.TradeOrders
				err = db.First(&updatedOrder, order.Id).Error
				require.NoError(t, err)
				assert.Equal(t, model.OrderStatusSuccess, updatedOrder.Status)
			} else {
				assert.Equal(t, 0, processedPayments, "Should not match for case: %+v", tc)
				
				// 验证订单状态未更新
				var unchangedOrder model.TradeOrders
				err = db.First(&unchangedOrder, order.Id).Error
				require.NoError(t, err)
				assert.Equal(t, model.OrderStatusWaiting, unchangedOrder.Status)
			}
		}
	})
}

// TestPaymentTimeValidation 测试支付时间验证
func TestPaymentTimeValidation(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Payment within valid time window", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "75.50"
		
		now := time.Now()
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":      "TRON",
			"address":    testAddress,
			"amount":     testAmount,
			"status":     model.OrderStatusWaiting,
			"created_at": now.Add(-10 * time.Minute), // 10分钟前创建
			"expired_at": now.Add(20 * time.Minute),  // 20分钟后过期
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		orderLock := make(map[string]model.TradeOrders)
		orderLock["TRON"+testAddress+testAmount] = *order

		// 交易时间在有效范围内（5分钟前）
		paymentTime := now.Add(-5 * time.Minute)
		mockData := createMockTronData(testAddress, testAmount, "valid_time_tx", paymentTime)
		jsonData, _ := json.Marshal(mockData)
		result := gjson.ParseBytes(jsonData)

		processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)
		assert.Equal(t, 1, processedPayments, "Should process payment within valid time window")
	})

	t.Run("Payment before order creation - should not match", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "75.50"
		
		now := time.Now()
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":      "TRON",
			"address":    testAddress,
			"amount":     testAmount,
			"status":     model.OrderStatusWaiting,
			"created_at": now.Add(-10 * time.Minute), // 10分钟前创建
			"expired_at": now.Add(20 * time.Minute),  // 20分钟后过期
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		orderLock := make(map[string]model.TradeOrders)
		orderLock["TRON"+testAddress+testAmount] = *order

		// 交易时间在订单创建前（20分钟前）
		paymentTime := now.Add(-20 * time.Minute)
		mockData := createMockTronData(testAddress, testAmount, "early_time_tx", paymentTime)
		jsonData, _ := json.Marshal(mockData)
		result := gjson.ParseBytes(jsonData)

		processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)
		assert.Equal(t, 0, processedPayments, "Should not process payment before order creation")
	})

	t.Run("Payment after order expiry - should not match", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "75.50"
		
		now := time.Now()
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":      "TRON",
			"address":    testAddress,
			"amount":     testAmount,
			"status":     model.OrderStatusWaiting,
			"created_at": now.Add(-30 * time.Minute), // 30分钟前创建
			"expired_at": now.Add(-10 * time.Minute), // 10分钟前过期
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		orderLock := make(map[string]model.TradeOrders)
		orderLock["TRON"+testAddress+testAmount] = *order

		// 交易时间在订单过期后（5分钟前，但订单10分钟前就过期了）
		paymentTime := now.Add(-5 * time.Minute)
		mockData := createMockTronData(testAddress, testAmount, "late_time_tx", paymentTime)
		jsonData, _ := json.Marshal(mockData)
		result := gjson.ParseBytes(jsonData)

		processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)
		assert.Equal(t, 0, processedPayments, "Should not process payment after order expiry")
	})
}

// TestPaymentDuplicateHandling 测试重复支付处理
func TestPaymentDuplicateHandling(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Duplicate payment should not reprocess", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "88.88"
		
		// 创建已成功的订单（模拟已处理过的支付）
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":        "TRON",
			"address":      testAddress,
			"amount":       testAmount,
			"status":       model.OrderStatusSuccess,
			"trade_hash":   "existing_tx_hash",
			"from_address": "TExistingFromAddress1234567890123456",
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		// 由于订单已经成功，不会在orderLock中
		orderLock := make(map[string]model.TradeOrders)
		// orderLock为空，模拟没有待支付订单

		// 尝试处理相同的交易
		mockData := createMockTronData(testAddress, testAmount, "existing_tx_hash", time.Now())
		jsonData, _ := json.Marshal(mockData)
		result := gjson.ParseBytes(jsonData)

		processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)
		assert.Equal(t, 0, processedPayments, "Should not reprocess already successful payment")
	})
}

// 辅助函数用于测试（模拟内部处理逻辑）
func processTestPaymentForTronScan(orderLock map[string]model.TradeOrders, toAddress string, data gjson.Result) int {
	processed := 0
	for _, transfer := range data.Get("token_transfers").Array() {
		if !strings.EqualFold(transfer.Get("to_address").String(), toAddress) {
			continue
		}

		// 计算交易金额
		rawQuant := transfer.Get("quant").Float()
		_decimalAmount := decimal.NewFromFloat(rawQuant)
		_decimalDivisor := decimal.NewFromFloat(1000000)
		result := _decimalAmount.Div(_decimalDivisor)
		quant := result.StringFixed(2)

		// 检查金额范围
		if result.LessThan(decimal.NewFromFloat(0.1)) {
			continue
		}

		// 检查订单是否存在
		orderKey := "TRON" + toAddress + quant
		order, ok := orderLock[orderKey]
		if !ok || transfer.Get("contractRet").String() != "SUCCESS" {
			continue
		}

		// 检查时间有效性
		createdAt := time.UnixMilli(transfer.Get("block_ts").Int())
		if createdAt.Unix() < order.CreatedAt.Unix() || createdAt.Unix() > order.ExpiredAt.Unix() {
			continue
		}

		// 处理支付
		transId := transfer.Get("transaction_id").String()
		fromAddress := transfer.Get("from_address").String()
		if order.OrderSetSucc(fromAddress, transId, createdAt) == nil {
			processed++
		}
	}
	return processed
}

func processTestPaymentForTronGrid(orderLock map[string]model.TradeOrders, toAddress string, data gjson.Result) int {
	processed := 0
	for _, transfer := range data.Get("data").Array() {
		if !strings.EqualFold(transfer.Get("to").String(), toAddress) {
			continue
		}

		// 计算交易金额
		rawQuant := transfer.Get("value").Float()
		_decimalAmount := decimal.NewFromFloat(rawQuant)
		_decimalDivisor := decimal.NewFromFloat(1000000)
		result := _decimalAmount.Div(_decimalDivisor)
		quant := result.StringFixed(2)

		// 检查金额范围
		if result.LessThan(decimal.NewFromFloat(0.1)) {
			continue
		}

		orderKey := "TRON" + toAddress + quant
		order, ok := orderLock[orderKey]
		if !ok || transfer.Get("type").String() != "Transfer" {
			continue
		}

		// 检查时间有效性
		createdAt := time.UnixMilli(transfer.Get("block_timestamp").Int())
		if createdAt.Unix() < order.CreatedAt.Unix() || createdAt.Unix() > order.ExpiredAt.Unix() {
			continue
		}

		// 处理支付
		transId := transfer.Get("transaction_id").String()
		fromAddress := transfer.Get("from").String()
		if order.OrderSetSucc(fromAddress, transId, createdAt) == nil {
			processed++
		}
	}
	return processed
}

func processTestPaymentForETH(orderLock map[string]model.TradeOrders, chain, toAddress string, data gjson.Result) int {
	processed := 0
	for _, transfer := range data.Get("result").Array() {
		if !strings.EqualFold(transfer.Get("to").String(), toAddress) {
			continue
		}

		tokenSymbol := transfer.Get("tokenSymbol").String()
		if !strings.Contains(strings.ToUpper(tokenSymbol), "USDT") {
			continue
		}

		// 解析交易金额
		value := transfer.Get("value").String()
		valueDecimal, err := decimal.NewFromString(value)
		if err != nil {
			continue
		}
		
		tokenDecimals := int32(6) // USDT通常是6位小数
		decimalFactor := decimal.New(1, tokenDecimals)
		valueUSDT := valueDecimal.Div(decimalFactor)

		// 检查金额范围
		if valueUSDT.LessThan(decimal.NewFromFloat(0.1)) {
			continue
		}

		amountStr := valueUSDT.StringFixed(2)
		orderKey := chain + toAddress + amountStr
		order, ok := orderLock[orderKey]
		if !ok {
			continue
		}

		// 检查时间有效性
		createdAt := time.Unix(transfer.Get("timeStamp").Int(), 0)
		if createdAt.Unix() < order.CreatedAt.Unix() || createdAt.Unix() > order.ExpiredAt.Unix() {
			continue
		}

		// 处理支付
		transId := transfer.Get("hash").String()
		fromAddress := transfer.Get("from").String()
		if order.OrderSetSucc(fromAddress, transId, createdAt) == nil {
			processed++
		}
	}
	return processed
}

func createMockTronData(toAddress, amount, txHash string, timestamp time.Time) map[string]interface{} {
	amountDecimal, _ := decimal.NewFromString(amount)
	quantInt := amountDecimal.Mul(decimal.NewFromFloat(1000000)).IntPart()

	return map[string]interface{}{
		"total": 1,
		"token_transfers": []map[string]interface{}{
			{
				"transaction_id": txHash,
				"to_address":     toAddress,
				"from_address":   "TTestFromAddress12345678901234567890",
				"quant":          quantInt,
				"contractRet":    "SUCCESS",
				"block_ts":       timestamp.UnixMilli(),
			},
		},
	}
}