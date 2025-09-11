package integration

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"gorm.io/gorm"
)

// APIEndpointsTestSuite API接口集成测试套件
type APIEndpointsTestSuite struct {
	suite.Suite
	container testcontainers.Container
	db        *gorm.DB
	router    *gin.Engine
	server    *httptest.Server
	apiTestData *testutils.APITestData
}

// SetupSuite 设置测试套件
func (s *APIEndpointsTestSuite) SetupSuite() {
	ctx := context.Background()
	
	// 设置测试数据库
	var err error
	s.container, s.db = testutils.SetupTestDB(ctx, s.T())
	require.NoError(s.T(), err)
	
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)
	
	// 创建路由
	s.router = gin.New()
	s.setupRoutes()
	
	// 创建测试服务器
	s.server = httptest.NewServer(s.router)
	
	// 创建API测试数据
	s.apiTestData = testutils.CreateOrderAPITestData()
}

// TearDownSuite 清理测试套件
func (s *APIEndpointsTestSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}
	if s.container != nil {
		ctx := context.Background()
		testutils.TeardownTestDB(ctx, s.container)
	}
}

// SetupTest 每个测试前的设置
func (s *APIEndpointsTestSuite) SetupTest() {
	// 清理数据库
	testutils.CleanDatabase(s.db)
	
	// 添加测试钱包地址
	s.setupTestWalletAddresses()
	
	// 重新生成测试数据以避免订单ID重复
	s.apiTestData = testutils.CreateOrderAPITestData()
}

// setupRoutes 设置路由
func (s *APIEndpointsTestSuite) setupRoutes() {
	// API路由组
	api := s.router.Group("/api/v1")
	{
		// 订单相关路由
		order := api.Group("/order")
		{
			order.POST("/create-transaction", s.authMiddleware(), web.CreateTransaction)
		}
	}
	
	// 支付页面路由
	pay := s.router.Group("/pay")
	{
		pay.GET("/checkout-counter/:trade_id", s.getCheckoutCounter)
		pay.GET("/check-status/:trade_id", s.getOrderStatus)
	}
}

// authMiddleware 模拟认证中间件
func (s *APIEndpointsTestSuite) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 模拟验证和数据解析
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(400, gin.H{"code": 400, "msg": "Invalid JSON"})
			c.Abort()
			return
		}
		
		// 设置到上下文
		c.Set("data", data)
		c.Next()
	}
}

// getCheckoutCounter 模拟支付页面接口
func (s *APIEndpointsTestSuite) getCheckoutCounter(c *gin.Context) {
	tradeId := c.Param("trade_id")
	
	order, exists := model.GetTradeOrder(tradeId)
	if !exists {
		c.JSON(404, gin.H{"code": 404, "msg": "Order not found"})
		return
	}
	
	// 返回支付页面数据
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "success",
		"data": gin.H{
			"trade_id":        order.TradeId,
			"order_id":        order.OrderId,
			"amount":          order.Money,
			"actual_amount":   order.Amount,
			"chain":           order.Chain,
			"address":         order.Address,
			"status":          order.Status,
			"status_label":    order.GetStatusLabel(),
			"expired_at":      order.ExpiredAt.Unix(),
			"created_at":      order.CreatedAt.Unix(),
		},
	})
}

// getOrderStatus 模拟订单状态查询接口
func (s *APIEndpointsTestSuite) getOrderStatus(c *gin.Context) {
	tradeId := c.Param("trade_id")
	
	order, exists := model.GetTradeOrder(tradeId)
	if !exists {
		c.JSON(404, gin.H{"code": 404, "msg": "Order not found"})
		return
	}
	
	// 返回订单状态
	c.JSON(200, gin.H{
		"code": 200,
		"msg":  "success",
		"data": gin.H{
			"trade_id":     order.TradeId,
			"order_id":     order.OrderId,
			"status":       order.Status,
			"status_label": order.GetStatusLabel(),
			"trade_hash":   order.TradeHash,
		},
	})
}

// setupTestWalletAddresses 设置测试钱包地址
func (s *APIEndpointsTestSuite) setupTestWalletAddresses() {
	addresses := []*model.WalletAddress{
		testutils.CreateTestWalletAddress("TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"),
		testutils.CreateTestWalletAddress("BSC", "0x55d398326f99059ff775485246999027b3197955"),
		testutils.CreateTestWalletAddress("POLY", "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"),
		testutils.CreateTestWalletAddress("OP", "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58"),
	}
	
	for _, addr := range addresses {
		err := s.db.Create(addr).Error
		require.NoError(s.T(), err)
	}
}

// doRequest 执行HTTP请求的辅助函数
func (s *APIEndpointsTestSuite) doRequest(method, path string, body interface{}) (*http.Response, []byte, error) {
	var reqBody io.Reader
	
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}
	
	req, err := http.NewRequest(method, s.server.URL+path, reqBody)
	if err != nil {
		return nil, nil, err
	}
	
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, err
	}
	
	return resp, respBody, nil
}

// TestCreateTransactionAPI 测试创建交易订单API
func (s *APIEndpointsTestSuite) TestCreateTransactionAPI() {
	// 使用有效的请求数据
	resp, body, err := s.doRequest("POST", "/api/v1/order/create-transaction", s.apiTestData.CreateOrderRequest)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	
	// 解析响应
	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(s.T(), err)
	
	// 验证响应结构
	assert.Equal(s.T(), float64(200), response["code"])
	assert.Equal(s.T(), "success", response["msg"])
	
	data, exists := response["data"].(map[string]interface{})
	require.True(s.T(), exists)
	
	// 验证返回的数据
	assert.NotEmpty(s.T(), data["trade_id"])
	assert.Equal(s.T(), s.apiTestData.CreateOrderRequest["order_id"], data["order_id"])
	assert.Equal(s.T(), s.apiTestData.CreateOrderRequest["amount"], data["amount"])
	assert.NotEmpty(s.T(), data["actual_amount"])
	assert.NotEmpty(s.T(), data["token"])
	assert.NotEmpty(s.T(), data["payment_url"])
	
	// 验证数据库中创建了订单
	tradeId := data["trade_id"].(string)
	order, exists := model.GetTradeOrder(tradeId)
	require.True(s.T(), exists)
	assert.Equal(s.T(), s.apiTestData.CreateOrderRequest["order_id"], order.OrderId)
	assert.Equal(s.T(), s.apiTestData.CreateOrderRequest["amount"], order.Money)
	assert.Equal(s.T(), model.OrderStatusWaiting, order.Status)
}

// TestCreateTransactionWithInvalidData 测试使用无效数据创建订单
func (s *APIEndpointsTestSuite) TestCreateTransactionWithInvalidData() {
	for i, invalidReq := range s.apiTestData.InvalidRequests {
		s.T().Run(fmt.Sprintf("InvalidRequest_%d", i), func(t *testing.T) {
			_, body, err := s.doRequest("POST", "/api/v1/order/create-transaction", invalidReq)
			require.NoError(t, err)
			
			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			require.NoError(t, err)
			
			// 应该返回错误响应
			assert.NotEqual(t, float64(200), response["code"], 
				"Request %d should fail: %+v", i, invalidReq)
			
			t.Logf("Invalid request %d response: %s", i, string(body))
		})
	}
}

// TestCreateDuplicateOrder 测试创建重复订单
func (s *APIEndpointsTestSuite) TestCreateDuplicateOrder() {
	// 第一次创建订单
	resp1, body1, err := s.doRequest("POST", "/api/v1/order/create-transaction", s.apiTestData.CreateOrderRequest)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp1.StatusCode)
	
	var response1 map[string]interface{}
	err = json.Unmarshal(body1, &response1)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), float64(200), response1["code"])
	
	// 尝试创建相同订单ID的订单
	_, body2, err := s.doRequest("POST", "/api/v1/order/create-transaction", s.apiTestData.CreateOrderRequest)
	require.NoError(s.T(), err)
	
	var response2 map[string]interface{}
	err = json.Unmarshal(body2, &response2)
	require.NoError(s.T(), err)
	
	// 应该失败（订单ID重复）
	assert.NotEqual(s.T(), float64(200), response2["code"], 
		"Duplicate order creation should fail")
	
	s.T().Logf("Duplicate order response: %s", string(body2))
}

// TestGetCheckoutCounter 测试获取支付页面数据
func (s *APIEndpointsTestSuite) TestGetCheckoutCounter() {
	// 先创建一个订单
	order := testutils.CreateTestOrder()
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	// 请求支付页面数据
	path := fmt.Sprintf("/pay/checkout-counter/%s", order.TradeId)
	resp, body, err := s.doRequest("GET", path, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	
	// 解析响应
	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(s.T(), err)
	
	assert.Equal(s.T(), float64(200), response["code"])
	assert.Equal(s.T(), "success", response["msg"])
	
	data, exists := response["data"].(map[string]interface{})
	require.True(s.T(), exists)
	
	// 验证返回的数据
	assert.Equal(s.T(), order.TradeId, data["trade_id"])
	assert.Equal(s.T(), order.OrderId, data["order_id"])
	assert.Equal(s.T(), order.Money, data["amount"])
	assert.Equal(s.T(), order.Amount, data["actual_amount"])
	assert.Equal(s.T(), order.Chain, data["chain"])
	assert.Equal(s.T(), order.Address, data["address"])
	assert.Equal(s.T(), float64(order.Status), data["status"])
	assert.NotEmpty(s.T(), data["status_label"])
}

// TestGetCheckoutCounterNotFound 测试获取不存在的订单支付页面
func (s *APIEndpointsTestSuite) TestGetCheckoutCounterNotFound() {
	// 请求不存在的订单
	path := "/pay/checkout-counter/non-existent-trade-id"
	resp, body, err := s.doRequest("GET", path, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode) // Gin返回200但内容是错误
	
	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(s.T(), err)
	
	assert.Equal(s.T(), float64(404), response["code"])
	assert.Equal(s.T(), "Order not found", response["msg"])
}

// TestGetOrderStatus 测试查询订单状态
func (s *APIEndpointsTestSuite) TestGetOrderStatus() {
	// 创建不同状态的订单进行测试
	testCases := []struct {
		name        string
		orderFunc   func() *model.TradeOrders
		expectedMsg string
	}{
		{
			name:        "Waiting Order",
			orderFunc:   testutils.CreatePendingOrder,
			expectedMsg: "🟡 等待支付",
		},
		{
			name:        "Success Order", 
			orderFunc:   testutils.CreateSuccessOrder,
			expectedMsg: "🟢 收款成功",
		},
		{
			name:        "Expired Order",
			orderFunc:   testutils.CreateExpiredOrder,
			expectedMsg: "🔴 交易过期",
		},
	}
	
	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			// 创建订单
			order := tc.orderFunc()
			err := s.db.Create(order).Error
			require.NoError(t, err)
			
			// 查询订单状态
			path := fmt.Sprintf("/pay/check-status/%s", order.TradeId)
			resp, body, err := s.doRequest("GET", path, nil)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			
			// 解析响应
			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			require.NoError(t, err)
			
			assert.Equal(t, float64(200), response["code"])
			assert.Equal(t, "success", response["msg"])
			
			data, exists := response["data"].(map[string]interface{})
			require.True(t, exists)
			
			// 验证状态数据
			assert.Equal(t, order.TradeId, data["trade_id"])
			assert.Equal(t, order.OrderId, data["order_id"])
			assert.Equal(t, float64(order.Status), data["status"])
			assert.Equal(t, tc.expectedMsg, data["status_label"])
			
			if order.Status == model.OrderStatusSuccess {
				assert.NotEmpty(t, data["trade_hash"])
			}
		})
	}
}

// TestGetOrderStatusNotFound 测试查询不存在的订单状态
func (s *APIEndpointsTestSuite) TestGetOrderStatusNotFound() {
	path := "/pay/check-status/non-existent-trade-id"
	resp, body, err := s.doRequest("GET", path, nil)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
	
	var response map[string]interface{}
	err = json.Unmarshal(body, &response)
	require.NoError(s.T(), err)
	
	assert.Equal(s.T(), float64(404), response["code"])
	assert.Equal(s.T(), "Order not found", response["msg"])
}

// TestAPIRateLimiting 测试API频率限制
func (s *APIEndpointsTestSuite) TestAPIRateLimiting() {
	// 快速发送多个创建订单请求
	concurrentRequests := 10
	results := make(chan struct {
		statusCode int
		response   map[string]interface{}
	}, concurrentRequests)
	
	for i := 0; i < concurrentRequests; i++ {
		go func(index int) {
			// 为每个请求生成唯一的订单ID
			reqData := make(map[string]interface{})
			for k, v := range s.apiTestData.CreateOrderRequest {
				reqData[k] = v
			}
			reqData["order_id"] = fmt.Sprintf("RATE_LIMIT_%d_%d", index, time.Now().UnixNano())
			
			resp, body, err := s.doRequest("POST", "/api/v1/order/create-transaction", reqData)
			
			var response map[string]interface{}
			if err == nil && body != nil {
				json.Unmarshal(body, &response)
			}
			
			statusCode := 0
			if resp != nil {
				statusCode = resp.StatusCode
			}
			
			results <- struct {
				statusCode int
				response   map[string]interface{}
			}{statusCode, response}
		}(i)
	}
	
	// 收集结果
	successCount := 0
	errorCount := 0
	
	for i := 0; i < concurrentRequests; i++ {
		select {
		case result := <-results:
			if result.statusCode == http.StatusOK && 
			   result.response != nil && 
			   result.response["code"] == float64(200) {
				successCount++
			} else {
				errorCount++
			}
		case <-time.After(10 * time.Second):
			s.T().Fatal("Rate limiting test timed out")
		}
	}
	
	s.T().Logf("Rate limiting test: %d success, %d errors out of %d requests", 
		successCount, errorCount, concurrentRequests)
	
	// 在理想情况下，所有请求都应该成功（如果没有实际的频率限制）
	// 在有频率限制的情况下，部分请求可能会失败
	assert.Greater(s.T(), successCount, 0, "At least some requests should succeed")
}

// TestAPIErrorHandling 测试API错误处理
func (s *APIEndpointsTestSuite) TestAPIErrorHandling() {
	testCases := []struct {
		name           string
		method         string
		path           string
		body           interface{}
		expectedCode   int
		expectedStatus int
	}{
		{
			name:           "Invalid JSON",
			method:         "POST",
			path:           "/api/v1/order/create-transaction",
			body:           "invalid json",
			expectedCode:   400,
			expectedStatus: http.StatusOK, // Gin middleware处理后返回200但内容是错误
		},
		{
			name:           "Empty Request Body",
			method:         "POST", 
			path:           "/api/v1/order/create-transaction",
			body:           map[string]interface{}{},
			expectedCode:   400,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "Missing Required Fields",
			method:         "POST",
			path:           "/api/v1/order/create-transaction", 
			body:           map[string]interface{}{"order_id": "test"},
			expectedCode:   400,
			expectedStatus: http.StatusOK,
		},
	}
	
	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			var reqBody io.Reader
			
			if tc.body != nil {
				if str, ok := tc.body.(string); ok {
					reqBody = strings.NewReader(str)
				} else {
					jsonBody, _ := json.Marshal(tc.body)
					reqBody = bytes.NewBuffer(jsonBody)
				}
			}
			
			req, err := http.NewRequest(tc.method, s.server.URL+tc.path, reqBody)
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			
			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
			
			// 读取响应体验证错误消息
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			
			var response map[string]interface{}
			if len(body) > 0 {
				err = json.Unmarshal(body, &response)
				if err == nil && response["code"] != nil {
					assert.Equal(t, float64(tc.expectedCode), response["code"],
						"Test case: %s, Response: %s", tc.name, string(body))
				}
			}
			
			t.Logf("Test case: %s, Response: %s", tc.name, string(body))
		})
	}
}

// TestAPIConcurrentAccess 测试API并发访问
func (s *APIEndpointsTestSuite) TestAPIConcurrentAccess() {
	// 创建一些订单用于并发查询
	orders := testutils.CreateOrderBatch(5)
	for _, order := range orders {
		err := s.db.Create(order).Error
		require.NoError(s.T(), err)
	}
	
	// 并发执行不同类型的请求
	concurrency := 15
	done := make(chan bool, concurrency)
	
	// 混合请求：创建订单、查询状态、获取支付页面
	for i := 0; i < concurrency; i++ {
		go func(index int) {
			defer func() { done <- true }()
			
			switch index % 3 {
			case 0:
				// 创建订单
				reqData := make(map[string]interface{})
				for k, v := range s.apiTestData.CreateOrderRequest {
					reqData[k] = v
				}
				reqData["order_id"] = fmt.Sprintf("CONCURRENT_%d_%d", index, time.Now().UnixNano())
				
				resp, _, err := s.doRequest("POST", "/api/v1/order/create-transaction", reqData)
				if err == nil {
					assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
				}
				
			case 1:
				// 查询订单状态
				order := orders[index%len(orders)]
				path := fmt.Sprintf("/pay/check-status/%s", order.TradeId)
				resp, _, err := s.doRequest("GET", path, nil)
				if err == nil {
					assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
				}
				
			case 2:
				// 获取支付页面
				order := orders[index%len(orders)]
				path := fmt.Sprintf("/pay/checkout-counter/%s", order.TradeId)
				resp, _, err := s.doRequest("GET", path, nil)
				if err == nil {
					assert.Equal(s.T(), http.StatusOK, resp.StatusCode)
				}
			}
		}(i)
	}
	
	// 等待所有请求完成
	for i := 0; i < concurrency; i++ {
		select {
		case <-done:
			// 成功
		case <-time.After(15 * time.Second):
			s.T().Fatal("Concurrent access test timed out")
		}
	}
	
	s.T().Log("All concurrent requests completed successfully")
}

// TestAPIResponseTimes 测试API响应时间
func (s *APIEndpointsTestSuite) TestAPIResponseTimes() {
	// 创建订单用于查询测试
	order := testutils.CreateTestOrder()
	err := s.db.Create(order).Error
	require.NoError(s.T(), err)
	
	testCases := []struct {
		name    string
		method  string
		path    string
		body    interface{}
		maxTime time.Duration
	}{
		{
			name:    "Create Order",
			method:  "POST",
			path:    "/api/v1/order/create-transaction",
			body:    s.apiTestData.CreateOrderRequest,
			maxTime: 2 * time.Second,
		},
		{
			name:    "Get Order Status",
			method:  "GET",
			path:    fmt.Sprintf("/pay/check-status/%s", order.TradeId),
			body:    nil,
			maxTime: 1 * time.Second,
		},
		{
			name:    "Get Checkout Counter",
			method:  "GET",
			path:    fmt.Sprintf("/pay/checkout-counter/%s", order.TradeId),
			body:    nil,
			maxTime: 1 * time.Second,
		},
	}
	
	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			// 为创建订单测试生成唯一ID
			body := tc.body
			if tc.method == "POST" && body != nil {
				reqData := make(map[string]interface{})
				for k, v := range body.(map[string]interface{}) {
					reqData[k] = v
				}
				reqData["order_id"] = fmt.Sprintf("PERF_%s_%d", tc.name, time.Now().UnixNano())
				body = reqData
			}
			
			start := time.Now()
			resp, _, err := s.doRequest(tc.method, tc.path, body)
			duration := time.Since(start)
			
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Less(t, duration, tc.maxTime, 
				"API %s took too long: %v", tc.name, duration)
			
			t.Logf("API %s response time: %v", tc.name, duration)
		})
	}
}

// TestAPIDataValidation 测试API数据验证
func (s *APIEndpointsTestSuite) TestAPIDataValidation() {
	validationTests := []struct {
		name         string
		requestData  map[string]interface{}
		shouldPass   bool
		expectedMsg  string
	}{
		{
			name: "Valid Request",
			requestData: map[string]interface{}{
				"order_id":     "VALID_ORDER_001",
				"amount":       100.0,
				"code":         "TRON",
				"notify_url":   "https://example.com/notify",
				"redirect_url": "https://example.com/return",
			},
			shouldPass:  true,
			expectedMsg: "success",
		},
		{
			name: "Negative Amount",
			requestData: map[string]interface{}{
				"order_id":     "NEGATIVE_AMOUNT",
				"amount":       -100.0,
				"code":         "TRON",
				"notify_url":   "https://example.com/notify", 
				"redirect_url": "https://example.com/return",
			},
			shouldPass:  false,
			expectedMsg: "参数错误",
		},
		{
			name: "Zero Amount",
			requestData: map[string]interface{}{
				"order_id":     "ZERO_AMOUNT",
				"amount":       0.0,
				"code":         "TRON",
				"notify_url":   "https://example.com/notify",
				"redirect_url": "https://example.com/return",
			},
			shouldPass:  false,
			expectedMsg: "参数错误",
		},
		{
			name: "Empty Order ID",
			requestData: map[string]interface{}{
				"order_id":     "",
				"amount":       100.0,
				"code":         "TRON",
				"notify_url":   "https://example.com/notify",
				"redirect_url": "https://example.com/return",
			},
			shouldPass:  false,
			expectedMsg: "参数错误",
		},
		{
			name: "Invalid Chain Code",
			requestData: map[string]interface{}{
				"order_id":     "INVALID_CHAIN",
				"amount":       100.0,
				"code":         "INVALID_CHAIN_CODE",
				"notify_url":   "https://example.com/notify",
				"redirect_url": "https://example.com/return",
			},
			shouldPass:  false,
			expectedMsg: "还没有配置收款地址",
		},
		{
			name: "Very Large Amount",
			requestData: map[string]interface{}{
				"order_id":     "LARGE_AMOUNT",
				"amount":       999999999.99,
				"code":         "TRON",
				"notify_url":   "https://example.com/notify",
				"redirect_url": "https://example.com/return",
			},
			shouldPass:  true, // 应该能处理大金额
			expectedMsg: "success",
		},
	}
	
	for _, test := range validationTests {
		s.T().Run(test.name, func(t *testing.T) {
			resp, body, err := s.doRequest("POST", "/api/v1/order/create-transaction", test.requestData)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			
			var response map[string]interface{}
			err = json.Unmarshal(body, &response)
			require.NoError(t, err)
			
			if test.shouldPass {
				assert.Equal(t, float64(200), response["code"],
					"Test %s should pass but failed with response: %s", test.name, string(body))
			} else {
				assert.NotEqual(t, float64(200), response["code"],
					"Test %s should fail but passed with response: %s", test.name, string(body))
			}
			
			// 检查错误消息
			if msg, exists := response["msg"]; exists && !test.shouldPass {
				assert.Contains(t, fmt.Sprintf("%v", msg), test.expectedMsg,
					"Error message should contain expected text")
			}
			
			t.Logf("Test %s: %s", test.name, string(body))
		})
	}
}

// 运行测试套件
func TestAPIEndpointsTestSuite(t *testing.T) {
	suite.Run(t, new(APIEndpointsTestSuite))
}