package monitor_test

import (
	"USDTMore/app/model"
	"USDTMore/tests/testutils"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestMultichainPaymentProcessing 测试多链支付处理
func TestMultichainPaymentProcessing(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 定义支持的区块链及其配置
	chainConfigs := map[string]struct {
		testAddress     string
		contractAddr    string
		decimals        int32
		explorerURL     string
	}{
		"TRON": {
			testAddress:  "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
			contractAddr: "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
			decimals:     6,
			explorerURL:  "https://tronscan.org/#/transaction/",
		},
		"POLY": {
			testAddress:  "0x1234567890123456789012345678901234567890",
			contractAddr: "0xc2132D05D31c914a87C6611C10748AEb04B58e8F",
			decimals:     6,
			explorerURL:  "https://polygonscan.com/tx/",
		},
		"OP": {
			testAddress:  "0x2234567890123456789012345678901234567890",
			contractAddr: "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58",
			decimals:     6,
			explorerURL:  "https://optimistic.etherscan.io/tx/",
		},
		"BSC": {
			testAddress:  "0x3234567890123456789012345678901234567890",
			contractAddr: "0x55d398326f99059fF775485246999027B3197955",
			decimals:     18,
			explorerURL:  "https://bscscan.com/tx/",
		},
		"ARB": {
			testAddress:  "0x4234567890123456789012345678901234567890",
			contractAddr: "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9",
			decimals:     6,
			explorerURL:  "https://arbiscan.io/tx/",
		},
	}

	for chainName, config := range chainConfigs {
		t.Run(fmt.Sprintf("%s chain payment processing", chainName), func(t *testing.T) {
			testutils.CleanDatabase(db)

			// 创建钱包地址
			walletAddr := testutils.CreateTestWalletAddress(chainName, config.testAddress)
			err := db.Create(walletAddr).Error
			require.NoError(t, err)

			// 创建测试订单
			testAmount := "99.50"
			order := testutils.CreateTestOrder(map[string]interface{}{
				"chain":   chainName,
				"address": config.testAddress,
				"amount":  testAmount,
				"status":  model.OrderStatusWaiting,
			})
			err = db.Create(order).Error
			require.NoError(t, err)

			// 创建订单锁映射
			orderLock := make(map[string]model.TradeOrders)
			orderLock[chainName+config.testAddress+testAmount] = *order

			// 根据链类型创建不同的模拟数据
			var mockData map[string]interface{}
			if chainName == "TRON" {
				mockData = createMockTronTransactionData(config.testAddress, testAmount, fmt.Sprintf("%s_tx_hash_123", chainName), time.Now())
			} else {
				mockData = createMockETHTransactionData(config.testAddress, testAmount, fmt.Sprintf("0x%s_tx_hash_123", chainName), time.Now(), config.contractAddr, config.decimals)
			}

			jsonData, _ := json.Marshal(mockData)
			result := gjson.ParseBytes(jsonData)

			// 测试支付处理
			var processedPayments int
			if chainName == "TRON" {
				processedPayments = processTestPaymentForTronScan(orderLock, config.testAddress, result)
			} else {
				processedPayments = processTestPaymentForETH(orderLock, chainName, config.testAddress, result)
			}

			// 验证支付被正确处理
			assert.Equal(t, 1, processedPayments, "Should process 1 payment for %s chain", chainName)

			// 验证订单状态更新
			var updatedOrder model.TradeOrders
			err = db.First(&updatedOrder, order.Id).Error
			require.NoError(t, err)
			assert.Equal(t, model.OrderStatusSuccess, updatedOrder.Status)
			assert.Contains(t, updatedOrder.TradeHash, chainName)
			assert.NotEmpty(t, updatedOrder.FromAddress)
		})
	}
}

// TestCrossChainOrderConflicts 测试跨链订单冲突处理
func TestCrossChainOrderConflicts(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Same amount different chains should not conflict", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testAmount := "55.75"
		chains := []struct {
			name    string
			address string
		}{
			{"TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"},
			{"POLY", "0x1111111111111111111111111111111111111111"},
			{"BSC", "0x2222222222222222222222222222222222222222"},
		}

		// 创建不同链上的钱包地址
		wallets := make([]model.WalletAddress, len(chains))
		for i, chain := range chains {
			wallet := testutils.CreateTestWalletAddress(chain.name, chain.address)
			err := db.Create(wallet).Error
			require.NoError(t, err)
			wallets[i] = *wallet
		}

		// 测试金额计算是否会冲突
		rate := 7.20
		money := 401.40 // 对应55.75 USDT

		address1, amount1 := model.CalcTradeAmount([]model.WalletAddress{wallets[0]}, rate, money)
		address2, amount2 := model.CalcTradeAmount([]model.WalletAddress{wallets[1]}, rate, money)
		address3, amount3 := model.CalcTradeAmount([]model.WalletAddress{wallets[2]}, rate, money)

		// 验证每个链都可以使用相同的金额
		assert.Equal(t, testAmount, amount1)
		assert.Equal(t, testAmount, amount2)
		assert.Equal(t, testAmount, amount3)

		// 验证选择了正确的地址
		assert.Equal(t, chains[0].address, address1.Address)
		assert.Equal(t, chains[1].address, address2.Address)
		assert.Equal(t, chains[2].address, address3.Address)
	})

	t.Run("Same chain same amount should conflict and increment", func(t *testing.T) {
		testutils.CleanDatabase(db)

		chainName := "TRON"
		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "13.89"

		// 创建钱包地址
		wallet := testutils.CreateTestWalletAddress(chainName, testAddress)
		err := db.Create(wallet).Error
		require.NoError(t, err)

		// 创建第一个订单占用该地址和金额
		order1 := testutils.CreateTestOrder(map[string]interface{}{
			"chain":   chainName,
			"address": testAddress,
			"amount":  testAmount,
			"status":  model.OrderStatusWaiting,
		})
		err = db.Create(order1).Error
		require.NoError(t, err)

		// 尝试创建第二个订单，应该得到递增的金额
		wallets := []model.WalletAddress{*wallet}
		rate := 7.20
		money := 100.00

		address, amount := model.CalcTradeAmount(wallets, rate, money)

		// 应该使用相同地址但不同金额
		assert.Equal(t, testAddress, address.Address)
		assert.Equal(t, chainName, address.Chain)

		// 金额应该递增
		originalAmount, _ := decimal.NewFromString(testAmount)
		newAmount, _ := decimal.NewFromString(amount)
		assert.True(t, newAmount.GreaterThan(originalAmount))

		// 差值应该是原子精度
		diff := newAmount.Sub(originalAmount)
		atomicity := decimal.NewFromFloat(model.Atomicity)
		assert.Equal(t, atomicity, diff)
	})
}

// TestChainSpecificFeatures 测试链特定功能
func TestChainSpecificFeatures(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("TRON energy and bandwidth considerations", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// TRON链特有的交易验证
		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		testAmount := "25.00"

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

		// 测试不同的TRON交易状态
		testCases := []struct {
			name         string
			contractRet  string
			shouldProcess bool
		}{
			{"Successful transaction", "SUCCESS", true},
			{"Failed transaction", "FAILED", false},
			{"Reverted transaction", "REVERT", false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				mockData := map[string]interface{}{
					"total": 1,
					"token_transfers": []map[string]interface{}{
						{
							"transaction_id": fmt.Sprintf("test_tron_%s", tc.contractRet),
							"to_address":     testAddress,
							"from_address":   "TTestFromAddress12345678901234567890",
							"quant":          25000000, // 25 USDT with 6 decimals
							"contractRet":    tc.contractRet,
							"block_ts":       time.Now().UnixMilli(),
						},
					},
				}

				jsonData, _ := json.Marshal(mockData)
				result := gjson.ParseBytes(jsonData)

				processedPayments := processTestPaymentForTronScan(orderLock, testAddress, result)

				if tc.shouldProcess {
					assert.Equal(t, 1, processedPayments, "Should process successful TRON transaction")
				} else {
					assert.Equal(t, 0, processedPayments, "Should not process failed TRON transaction")
				}
			})
		}
	})

	t.Run("BSC BNB gas fee handling", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// BSC链可能有特殊的代币符号处理
		testAddress := "0x1234567890123456789012345678901234567890"
		testAmount := "45.25"

		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":   "BSC",
			"address": testAddress,
			"amount":  testAmount,
			"status":  model.OrderStatusWaiting,
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		orderLock := make(map[string]model.TradeOrders)
		orderLock["BSC"+testAddress+testAmount] = *order

		// 测试不同的代币符号格式
		testCases := []struct {
			tokenSymbol   string
			shouldProcess bool
		}{
			{"USDT", true},
			{"usdt", true},
			{"BSC-USD", true}, // BSC特有格式
			{"USDC", false},   // 不是USDT
			{"BNB", false},    // 不是USDT
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("Token symbol: %s", tc.tokenSymbol), func(t *testing.T) {
				mockData := createMockETHTransactionData(
					testAddress,
					testAmount,
					"0xtest_bsc_tx_hash",
					time.Now(),
					"0x55d398326f99059fF775485246999027B3197955",
					18, // BSC USDT通常是18位小数
				)
				
				// 修改token symbol
				result := mockData["result"].([]map[string]interface{})
				result[0]["tokenSymbol"] = tc.tokenSymbol

				jsonData, _ := json.Marshal(mockData)
				gjsonResult := gjson.ParseBytes(jsonData)

				processedPayments := processTestPaymentForETH(orderLock, "BSC", testAddress, gjsonResult)

				if tc.shouldProcess {
					assert.Equal(t, 1, processedPayments, "Should process BSC transaction with token symbol: %s", tc.tokenSymbol)
				} else {
					assert.Equal(t, 0, processedPayments, "Should not process BSC transaction with token symbol: %s", tc.tokenSymbol)
				}
			})
		}
	})
}

// TestChainAmountPrecision 测试不同链的金额精度处理
func TestChainAmountPrecision(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Different decimal precision handling", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testCases := []struct {
			chain     string
			address   string
			decimals  int32
			rawAmount string
			expected  string
		}{
			{"TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", 6, "100000000", "100.00"},    // 100 USDT
			{"POLY", "0x1111111111111111111111111111111111111111", 6, "50000000", "50.00"},   // 50 USDT
			{"BSC", "0x2222222222222222222222222222222222222222", 18, "75000000000000000000", "75.00"}, // 75 USDT (18 decimals)
			{"OP", "0x3333333333333333333333333333333333333333", 6, "25500000", "25.50"},     // 25.5 USDT
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s precision test", tc.chain), func(t *testing.T) {
				order := testutils.CreateTestOrder(map[string]interface{}{
					"chain":   tc.chain,
					"address": tc.address,
					"amount":  tc.expected,
					"status":  model.OrderStatusWaiting,
				})
				err := db.Create(order).Error
				require.NoError(t, err)

				orderLock := make(map[string]model.TradeOrders)
				orderLock[tc.chain+tc.address+tc.expected] = *order

				var mockData map[string]interface{}
				if tc.chain == "TRON" {
					mockData = map[string]interface{}{
						"total": 1,
						"token_transfers": []map[string]interface{}{
							{
								"transaction_id": fmt.Sprintf("test_%s_precision_tx", tc.chain),
								"to_address":     tc.address,
								"from_address":   "TTestFromAddress12345678901234567890",
								"quant":          mustParseInt(tc.rawAmount),
								"contractRet":    "SUCCESS",
								"block_ts":       time.Now().UnixMilli(),
							},
						},
					}
				} else {
					mockData = map[string]interface{}{
						"status": "1",
						"result": []map[string]interface{}{
							{
								"hash":             fmt.Sprintf("0xtest_%s_precision_tx", tc.chain),
								"to":               tc.address,
								"from":             "0x1234567890123456789012345678901234567890",
								"value":            tc.rawAmount,
								"tokenSymbol":      "USDT",
								"tokenDecimal":     fmt.Sprintf("%d", tc.decimals),
								"contractAddress":  "0xdAC17F958D2ee523a2206206994597C13D831ec7",
								"timeStamp":        fmt.Sprintf("%d", time.Now().Unix()),
							},
						},
					}
				}

				jsonData, _ := json.Marshal(mockData)
				result := gjson.ParseBytes(jsonData)

				var processedPayments int
				if tc.chain == "TRON" {
					processedPayments = processTestPaymentForTronScan(orderLock, tc.address, result)
				} else {
					processedPayments = processTestPaymentForETH(orderLock, tc.chain, tc.address, result)
				}

				assert.Equal(t, 1, processedPayments, "Should correctly handle %d decimal precision for %s", tc.decimals, tc.chain)

				// 验证订单状态更新
				var updatedOrder model.TradeOrders
				err = db.First(&updatedOrder, order.Id).Error
				require.NoError(t, err)
				assert.Equal(t, model.OrderStatusSuccess, updatedOrder.Status)
			})
		}
	})
}

// TestChainAddressValidation 测试不同链的地址格式验证
func TestChainAddressValidation(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Address format validation", func(t *testing.T) {
		testutils.CleanDatabase(db)

		testCases := []struct {
			chain           string
			validAddress    string
			invalidAddress  string
		}{
			{
				"TRON",
				"TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
				"0x1234567890123456789012345678901234567890", // ETH格式地址
			},
			{
				"POLY",
				"0x1234567890123456789012345678901234567890",
				"TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", // TRON格式地址
			},
			{
				"BSC",
				"0x2234567890123456789012345678901234567890",
				"invalid_address_format",
			},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s address validation", tc.chain), func(t *testing.T) {
				testAmount := "33.33"

				// 测试有效地址
				validOrder := testutils.CreateTestOrder(map[string]interface{}{
					"chain":   tc.chain,
					"address": tc.validAddress,
					"amount":  testAmount,
					"status":  model.OrderStatusWaiting,
				})
				err := db.Create(validOrder).Error
				require.NoError(t, err)

				orderLock := make(map[string]model.TradeOrders)
				orderLock[tc.chain+tc.validAddress+testAmount] = *validOrder

				// 创建指向有效地址的交易数据
				var mockData map[string]interface{}
				if tc.chain == "TRON" {
					mockData = createMockTronTransactionData(tc.validAddress, testAmount, "valid_addr_tx", time.Now())
				} else {
					mockData = createMockETHTransactionData(tc.validAddress, testAmount, "0xvalid_addr_tx", time.Now(), "0xdAC17F958D2ee523a2206206994597C13D831ec7", 6)
				}

				jsonData, _ := json.Marshal(mockData)
				result := gjson.ParseBytes(jsonData)

				var processedPayments int
				if tc.chain == "TRON" {
					processedPayments = processTestPaymentForTronScan(orderLock, tc.validAddress, result)
				} else {
					processedPayments = processTestPaymentForETH(orderLock, tc.chain, tc.validAddress, result)
				}

				assert.Equal(t, 1, processedPayments, "Should process payment to valid %s address", tc.chain)

				// 测试无效地址不会被处理
				if tc.chain == "TRON" {
					invalidProcessed := processTestPaymentForTronScan(orderLock, tc.invalidAddress, result)
					assert.Equal(t, 0, invalidProcessed, "Should not process payment to invalid %s address format", tc.chain)
				} else {
					invalidProcessed := processTestPaymentForETH(orderLock, tc.chain, tc.invalidAddress, result)
					assert.Equal(t, 0, invalidProcessed, "Should not process payment to invalid %s address format", tc.chain)
				}
			})
		}
	})
}

// 辅助函数
func createMockTronTransactionData(toAddress, amount, txHash string, timestamp time.Time) map[string]interface{} {
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

func createMockETHTransactionData(toAddress, amount, txHash string, timestamp time.Time, contractAddr string, decimals int32) map[string]interface{} {
	amountDecimal, _ := decimal.NewFromString(amount)
	// 根据小数位数计算原始值
	multiplier := decimal.New(1, decimals)
	rawValue := amountDecimal.Mul(multiplier).String()

	return map[string]interface{}{
		"status": "1",
		"result": []map[string]interface{}{
			{
				"hash":             txHash,
				"to":               toAddress,
				"from":             "0x1234567890123456789012345678901234567890",
				"value":            rawValue,
				"tokenSymbol":      "USDT",
				"tokenDecimal":     fmt.Sprintf("%d", decimals),
				"contractAddress":  contractAddr,
				"timeStamp":        fmt.Sprintf("%d", timestamp.Unix()),
			},
		},
	}
}

func mustParseInt(s string) int64 {
	decimal, _ := decimal.NewFromString(s)
	return decimal.IntPart()
}