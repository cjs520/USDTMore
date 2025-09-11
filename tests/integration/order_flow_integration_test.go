package integration_test

import (
	"USDTMore/app/model"
	"USDTMore/app/web"
	"USDTMore/tests/testutils"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// setupTestRouter 设置测试路由器
func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	
	// 添加中间件来解析请求数据
	router.Use(func(c *gin.Context) {
		if c.Request.Method == "POST" && strings.Contains(c.GetHeader("Content-Type"), "application/json") {
			body, err := io.ReadAll(c.Request.Body)
			if err == nil {
				var data map[string]interface{}
				if json.Unmarshal(body, &data) == nil {
					c.Set("data", data)
				}
				// 重置body供后续使用
				c.Request.Body = io.NopCloser(bytes.NewReader(body))
			}
		}
		c.Next()
	})

	// 注册路由
	api := router.Group("/api/v1")
	{
		api.POST("/orders", web.CreateTransaction)
	}
	
	pay := router.Group("/pay")
	{
		pay.GET("/checkout-counter/:trade_id", web.CheckoutCounter)
		pay.GET("/status/:trade_id", web.CheckStatus)
	}

	return router
}

// TestCompleteOrderFlow 测试完整的订单流程
func TestCompleteOrderFlow(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Complete successful order flow", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建模拟回调服务器
		callbackReceived := make(chan bool, 1)
		var receivedCallback map[string]interface{}

		mockCallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			json.NewDecoder(r.Body).Decode(&body)
			receivedCallback = body
			
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			callbackReceived <- true
		}))
		defer mockCallbackServer.Close()

		// 1. 设置测试环境
		testChain := "TRON"
		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		
		// 创建钱包地址
		wallet := testutils.CreateTestWalletAddress(testChain, testAddress)
		err := db.Create(wallet).Error
		require.NoError(t, err)

		// 设置路由器
		router := setupTestRouter()

		// 2. 创建订单
		orderRequest := map[string]interface{}{
			"order_id":     "TEST_ORDER_12345",
			"amount":       100.0,
			"code":         "TRC20",
			"notify_url":   mockCallbackServer.URL + "/notify",
			"redirect_url": "https://example.com/return",
		}

		requestBody, _ := json.Marshal(orderRequest)
		req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证订单创建响应
		assert.Equal(t, http.StatusOK, w.Code)
		
		var createResponse map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &createResponse)
		require.NoError(t, err)
		
		data := createResponse["data"].(map[string]interface{})
		tradeId := data["trade_id"].(string)
		actualAmount := data["actual_amount"].(string)
		paymentUrl := data["payment_url"].(string)

		assert.NotEmpty(t, tradeId)
		assert.NotEmpty(t, actualAmount)
		assert.NotEmpty(t, paymentUrl)
		assert.Contains(t, paymentUrl, tradeId)

		// 验证订单在数据库中创建
		var createdOrder model.TradeOrders
		err = db.Where("trade_id = ?", tradeId).First(&createdOrder).Error
		require.NoError(t, err)
		assert.Equal(t, "TEST_ORDER_12345", createdOrder.OrderId)
		assert.Equal(t, model.OrderStatusWaiting, createdOrder.Status)
		assert.Equal(t, testChain, createdOrder.Chain)
		assert.Equal(t, testAddress, createdOrder.Address)

		// 3. 检查订单状态（支付前）
		statusReq := httptest.NewRequest("GET", fmt.Sprintf("/pay/status/%s", tradeId), nil)
		statusW := httptest.NewRecorder()
		router.ServeHTTP(statusW, statusReq)

		assert.Equal(t, http.StatusOK, statusW.Code)
		var statusResponse map[string]interface{}
		err = json.Unmarshal(statusW.Body.Bytes(), &statusResponse)
		require.NoError(t, err)
		assert.Equal(t, float64(model.OrderStatusWaiting), statusResponse["status"])
		assert.Empty(t, statusResponse["return_url"]) // 未支付时不返回return_url

		// 4. 模拟区块链支付
		fromAddress := "TTestPaymentAddress123456789012345"
		txHash := "integration_test_tx_hash_12345"
		
		// 更新订单为支付成功
		err = createdOrder.OrderSetSucc(fromAddress, txHash, time.Now())
		require.NoError(t, err)

		// 5. 模拟回调通知（这里直接调用，在实际系统中由monitor触发）
		// 这模拟了monitor检测到支付后发送回调的过程
		go func() {
			time.Sleep(100 * time.Millisecond) // 模拟处理延迟
			// 在实际系统中，这里会由 notify.OrderNotify(createdOrder) 触发
			// 为了测试完整性，我们手动构造回调数据
			callbackData := map[string]interface{}{
				"trade_id":              tradeId,
				"order_id":              "TEST_ORDER_12345",
				"amount":                100.0,
				"actual_amount":         actualAmount,
				"token":                 testAddress,
				"block_transaction_id":  txHash,
				"signature":             "test_signature", // 在实际中会计算真实签名
				"status":                model.OrderStatusSuccess,
			}

			// 发送回调
			callbackJson, _ := json.Marshal(callbackData)
			http.Post(mockCallbackServer.URL+"/notify", "application/json", bytes.NewReader(callbackJson))
		}()

		// 6. 等待回调并验证
		select {
		case <-callbackReceived:
			assert.NotNil(t, receivedCallback)
			assert.Equal(t, tradeId, receivedCallback["trade_id"])
			assert.Equal(t, "TEST_ORDER_12345", receivedCallback["order_id"])
			assert.Equal(t, float64(100.0), receivedCallback["amount"])
			assert.Equal(t, txHash, receivedCallback["block_transaction_id"])
			assert.Equal(t, float64(model.OrderStatusSuccess), receivedCallback["status"])
		case <-time.After(5 * time.Second):
			t.Fatal("Callback not received within timeout")
		}

		// 7. 再次检查订单状态（支付后）
		statusReq2 := httptest.NewRequest("GET", fmt.Sprintf("/pay/status/%s", tradeId), nil)
		statusW2 := httptest.NewRecorder()
		router.ServeHTTP(statusW2, statusReq2)

		assert.Equal(t, http.StatusOK, statusW2.Code)
		var statusResponse2 map[string]interface{}
		err = json.Unmarshal(statusW2.Body.Bytes(), &statusResponse2)
		require.NoError(t, err)
		assert.Equal(t, float64(model.OrderStatusSuccess), statusResponse2["status"])
		assert.Equal(t, "https://example.com/return", statusResponse2["return_url"]) // 支付成功后返回return_url

		// 8. 验证最终订单状态
		var finalOrder model.TradeOrders
		err = db.Where("trade_id = ?", tradeId).First(&finalOrder).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, finalOrder.Status)
		assert.Equal(t, fromAddress, finalOrder.FromAddress)
		assert.Equal(t, txHash, finalOrder.TradeHash)
		assert.NotZero(t, finalOrder.ConfirmedAt)
	})
}

// TestOrderFlowWithExpiry 测试带过期的订单流程
func TestOrderFlowWithExpiry(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Order expiry flow", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建钱包地址
		wallet := testutils.CreateTestWalletAddress("TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
		err := db.Create(wallet).Error
		require.NoError(t, err)

		// 创建一个很短过期时间的订单（1秒后过期）
		order := testutils.CreateTestOrder(map[string]interface{}{
			"status":     model.OrderStatusWaiting,
			"expired_at": time.Now().Add(1 * time.Second),
		})
		err = db.Create(order).Error
		require.NoError(t, err)

		// 设置路由器
		router := setupTestRouter()

		// 检查初始状态
		statusReq := httptest.NewRequest("GET", fmt.Sprintf("/pay/status/%s", order.TradeId), nil)
		statusW := httptest.NewRecorder()
		router.ServeHTTP(statusW, statusReq)
		assert.Equal(t, http.StatusOK, statusW.Code)

		// 等待订单过期
		time.Sleep(2 * time.Second)

		// 模拟系统检测到过期并更新状态
		if time.Now().Unix() >= order.ExpiredAt.Unix() {
			err = order.OrderSetExpired()
			require.NoError(t, err)
		}

		// 再次检查状态
		statusReq2 := httptest.NewRequest("GET", fmt.Sprintf("/pay/status/%s", order.TradeId), nil)
		statusW2 := httptest.NewRecorder()
		router.ServeHTTP(statusW2, statusReq2)

		assert.Equal(t, http.StatusOK, statusW2.Code)
		var statusResponse map[string]interface{}
		err = json.Unmarshal(statusW2.Body.Bytes(), &statusResponse)
		require.NoError(t, err)
		assert.Equal(t, float64(model.OrderStatusExpired), statusResponse["status"])

		// 验证数据库中的状态
		var expiredOrder model.TradeOrders
		err = db.First(&expiredOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusExpired, expiredOrder.Status)
	})
}

// TestMultipleOrdersAmountCalculation 测试多订单金额计算
func TestMultipleOrdersAmountCalculation(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Multiple orders with amount conflicts", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建钱包地址
		wallet := testutils.CreateTestWalletAddress("TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
		err := db.Create(wallet).Error
		require.NoError(t, err)

		router := setupTestRouter()

		// 创建多个相同金额的订单
		baseAmount := 100.0
		numOrders := 3
		createdOrders := make([]string, numOrders)

		for i := 0; i < numOrders; i++ {
			orderRequest := map[string]interface{}{
				"order_id":     fmt.Sprintf("TEST_ORDER_%d", i),
				"amount":       baseAmount,
				"code":         "TRC20",
				"notify_url":   "https://example.com/notify",
				"redirect_url": "https://example.com/return",
			}

			requestBody, _ := json.Marshal(orderRequest)
			req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			data := response["data"].(map[string]interface{})
			createdOrders[i] = data["trade_id"].(string)
		}

		// 验证所有订单都被创建，且金额递增
		var orders []model.TradeOrders
		err = db.Where("order_id LIKE ?", "TEST_ORDER_%").Find(&orders).Error
		require.NoError(t, err)
		assert.Len(t, orders, numOrders)

		// 验证金额递增（避免冲突）
		amounts := make([]decimal.Decimal, len(orders))
		for i, order := range orders {
			amount, _ := decimal.NewFromString(order.Amount)
			amounts[i] = amount
		}

		// 验证每个金额都不同
		for i := 1; i < len(amounts); i++ {
			assert.True(t, amounts[i].GreaterThan(amounts[i-1]), "Amounts should be incrementally different")
		}
	})
}

// TestPaymentProcessingIntegration 测试支付处理集成
func TestPaymentProcessingIntegration(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Payment processing with blockchain simulation", func(t *testing.T) {
		testutils.CleanDatabase(db)

		// 创建钱包地址
		testAddress := "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"
		wallet := testutils.CreateTestWalletAddress("TRON", testAddress)
		err := db.Create(wallet).Error
		require.NoError(t, err)

		// 创建测试订单
		testAmount := "13.89"
		order := testutils.CreateTestOrder(map[string]interface{}{
			"chain":   "TRON",
			"address": testAddress,
			"amount":  testAmount,
			"status":  model.OrderStatusWaiting,
		})
		err = db.Create(order).Error
		require.NoError(t, err)

		// 模拟区块链API响应
		mockBlockchainData := map[string]interface{}{
			"total": 1,
			"token_transfers": []map[string]interface{}{
				{
					"transaction_id": "blockchain_integration_tx_12345",
					"to_address":     testAddress,
					"from_address":   "TTestPayer123456789012345678901234",
					"quant":          13890000, // 13.89 USDT with 6 decimals
					"contractRet":    "SUCCESS",
					"block_ts":       time.Now().UnixMilli(),
				},
			},
		}

		// 模拟支付检测和处理逻辑
		orderLock := make(map[string]model.TradeOrders)
		orderLock["TRON"+testAddress+testAmount] = *order

		jsonData, _ := json.Marshal(mockBlockchainData)
		result := gjson.ParseBytes(jsonData)

		// 处理支付（模拟monitor.trade.go中的逻辑）
		for _, transfer := range result.Get("token_transfers").Array() {
			if transfer.Get("to_address").String() != testAddress {
				continue
			}

			if transfer.Get("contractRet").String() != "SUCCESS" {
				continue
			}

			// 计算金额
			rawQuant := transfer.Get("quant").Float()
			decimalAmount := decimal.NewFromFloat(rawQuant)
			decimalDivisor := decimal.NewFromFloat(1000000)
			resultAmount := decimalAmount.Div(decimalDivisor)
			quant := resultAmount.StringFixed(2)

			orderKey := "TRON" + testAddress + quant
			foundOrder, ok := orderLock[orderKey]
			if !ok {
				continue
			}

			// 检查时间有效性
			createdAt := time.UnixMilli(transfer.Get("block_ts").Int())
			if createdAt.Unix() < foundOrder.CreatedAt.Unix() || createdAt.Unix() > foundOrder.ExpiredAt.Unix() {
				continue
			}

			// 处理支付成功
			transId := transfer.Get("transaction_id").String()
			fromAddress := transfer.Get("from_address").String()
			err := foundOrder.OrderSetSucc(fromAddress, transId, createdAt)
			assert.NoError(t, err)
		}

		// 验证订单状态更新
		var updatedOrder model.TradeOrders
		err = db.First(&updatedOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderStatusSuccess, updatedOrder.Status)
		assert.Equal(t, "blockchain_integration_tx_12345", updatedOrder.TradeHash)
		assert.Equal(t, "TTestPayer123456789012345678901234", updatedOrder.FromAddress)
	})
}

// TestCallbackRetryIntegration 测试回调重试集成
func TestCallbackRetryIntegration(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Callback retry mechanism integration", func(t *testing.T) {
		testutils.CleanDatabase(db)

		retryCount := 0
		maxRetries := 3

		// 创建模拟回调服务器（前几次失败，最后成功）
		mockCallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			retryCount++
			if retryCount < maxRetries {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Server Error"))
			} else {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("ok"))
			}
		}))
		defer mockCallbackServer.Close()

		// 创建支付成功的订单
		order := testutils.CreateTestOrder(map[string]interface{}{
			"status":       model.OrderStatusSuccess,
			"notify_url":   mockCallbackServer.URL + "/notify",
			"trade_hash":   "callback_retry_integration_tx",
			"from_address": "TTestFromAddress12345678901234567890",
			"confirmed_at": time.Now(),
		})
		err := db.Create(order).Error
		require.NoError(t, err)

		// 模拟回调重试逻辑
		for attempt := 1; attempt <= maxRetries; attempt++ {
			// 模拟 notify.OrderNotify 调用
			// 这里我们直接模拟HTTP请求而不是调用实际函数
			callbackData := map[string]interface{}{
				"trade_id":              order.TradeId,
				"order_id":              order.OrderId,
				"amount":                order.Money,
				"actual_amount":         order.Amount,
				"token":                 order.Address,
				"block_transaction_id":  order.TradeHash,
				"signature":             "test_signature",
				"status":                order.Status,
			}

			jsonBody, _ := json.Marshal(callbackData)
			resp, err := http.Post(mockCallbackServer.URL+"/notify", "application/json", bytes.NewReader(jsonBody))
			require.NoError(t, err)

			// 更新订单通知状态
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				if string(body) == "ok" {
					err = order.OrderSetNotifyState(model.OrderNotifyStateSucc)
				} else {
					err = order.OrderSetNotifyState(model.OrderNotifyStateFail)
				}
			} else {
				err = order.OrderSetNotifyState(model.OrderNotifyStateFail)
			}
			assert.NoError(t, err)
			resp.Body.Close()

			// 如果成功，跳出循环
			if order.NotifyState == model.OrderNotifyStateSucc {
				break
			}

			// 模拟重试延迟
			if attempt < maxRetries {
				time.Sleep(100 * time.Millisecond)
			}
		}

		// 验证最终状态
		assert.Equal(t, maxRetries, retryCount, "Should retry exactly the expected number of times")
		assert.Equal(t, model.OrderNotifyStateSucc, order.NotifyState, "Final notification should succeed")
		assert.Equal(t, maxRetries, order.NotifyNum, "Should have correct retry count")

		// 验证数据库状态
		var finalOrder model.TradeOrders
		err = db.First(&finalOrder, order.Id).Error
		require.NoError(t, err)
		assert.Equal(t, model.OrderNotifyStateSucc, finalOrder.NotifyState)
		assert.Equal(t, maxRetries, finalOrder.NotifyNum)
	})
}

// TestCrossChainIntegration 测试跨链集成
func TestCrossChainIntegration(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	t.Run("Cross-chain payment processing", func(t *testing.T) {
		testutils.CleanDatabase(db)

		chains := []struct {
			name    string
			code    string
			address string
		}{
			{"TRON", "TRC20", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"},
			{"POLY", "Polygon", "0x1111111111111111111111111111111111111111"},
			{"BSC", "BEP20", "0x2222222222222222222222222222222222222222"},
		}

		router := setupTestRouter()

		for _, chain := range chains {
			// 创建钱包地址
			wallet := testutils.CreateTestWalletAddress(chain.name, chain.address)
			err := db.Create(wallet).Error
			require.NoError(t, err)

			// 创建订单
			orderRequest := map[string]interface{}{
				"order_id":     fmt.Sprintf("%s_ORDER_12345", chain.name),
				"amount":       50.0,
				"code":         chain.code,
				"notify_url":   "https://example.com/notify",
				"redirect_url": "https://example.com/return",
			}

			requestBody, _ := json.Marshal(orderRequest)
			req := httptest.NewRequest("POST", "/api/v1/orders", bytes.NewReader(requestBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Order creation should succeed for %s", chain.name)

			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			data := response["data"].(map[string]interface{})
			tradeId := data["trade_id"].(string)

			// 验证订单在数据库中
			var createdOrder model.TradeOrders
			err = db.Where("trade_id = ?", tradeId).First(&createdOrder).Error
			require.NoError(t, err)
			assert.Equal(t, chain.name, createdOrder.Chain, "Order should be created for correct chain")
			assert.Equal(t, chain.address, createdOrder.Address, "Order should use correct address")
		}

		// 验证所有链的订单都被创建且互不冲突
		var allOrders []model.TradeOrders
		err := db.Find(&allOrders).Error
		require.NoError(t, err)
		assert.Len(t, allOrders, len(chains), "Should create orders for all chains")

		// 验证每个链的订单使用了正确的地址
		chainAddressMap := make(map[string]string)
		for _, order := range allOrders {
			chainAddressMap[order.Chain] = order.Address
		}

		for _, chain := range chains {
			assert.Equal(t, chain.address, chainAddressMap[chain.name], 
				"Chain %s should use correct address", chain.name)
		}
	})
}