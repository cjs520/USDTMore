package web_test

import (
	"USDTMore/app/config"
	"USDTMore/app/model"
	"USDTMore/app/usdt"
	"USDTMore/app/web"
	"USDTMore/tests/testutils"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTransaction_Success(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	// 创建测试钱包地址
	wallet := testutils.CreateTestWalletAddress("TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
	err := db.Create(wallet).Error
	require.NoError(t, err)

	// 模拟配置函数
	originalGetLatestRate := usdt.GetLatestRate
	usdt.GetLatestRate = func() float64 { return 7.20 }
	defer func() { usdt.GetLatestRate = originalGetLatestRate }()

	originalGetExpireTime := config.GetExpireTime
	config.GetExpireTime = func() time.Duration { return 30 * time.Minute }
	defer func() { config.GetExpireTime = originalGetExpireTime }()

	originalGetAppUri := config.GetAppUri
	config.GetAppUri = func(host string) string { return host }
	defer func() { config.GetAppUri = originalGetAppUri }()

	// 创建测试请求数据
	requestData := map[string]interface{}{
		"code":         "TRON",
		"order_id":     "TEST_ORDER_123",
		"amount":       100.0,
		"notify_url":   "https://example.com/notify",
		"redirect_url": "https://example.com/redirect",
	}

	// 创建HTTP请求
	jsonData, _ := json.Marshal(requestData)
	req, _ := http.NewRequest("POST", "/api/create_transaction", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// 创建响应记录器
	w := httptest.NewRecorder()

	// 创建Gin路由
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("data", requestData)
		c.Next()
	})
	router.POST("/api/create_transaction", web.CreateTransaction)

	// 执行请求
	router.ServeHTTP(w, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// 验证响应数据结构
	assert.Contains(t, response, "data")
	data := response["data"].(map[string]interface{})
	
	assert.Contains(t, data, "trade_id")
	assert.Contains(t, data, "order_id")
	assert.Contains(t, data, "amount")
	assert.Contains(t, data, "actual_amount")
	assert.Contains(t, data, "token")
	assert.Contains(t, data, "payment_url")

	assert.Equal(t, "TEST_ORDER_123", data["order_id"])
	assert.Equal(t, 100.0, data["amount"])
	assert.Equal(t, wallet.Address, data["token"])

	// 验证数据库中创建了订单
	tradeId := data["trade_id"].(string)
	order, exists := model.GetTradeOrder(tradeId)
	assert.True(t, exists)
	assert.Equal(t, "TEST_ORDER_123", order.OrderId)
	assert.Equal(t, model.OrderStatusWaiting, order.Status)
	assert.Equal(t, "TRON", order.Chain)
}

func TestCreateTransaction_MissingParameters(t *testing.T) {
	ctx := context.Background()
	container, _ := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		requestData map[string]interface{}
		expectError bool
	}{
		{
			name: "Missing order_id",
			requestData: map[string]interface{}{
				"code":         "TRON",
				"amount":       100.0,
				"notify_url":   "https://example.com/notify",
				"redirect_url": "https://example.com/redirect",
			},
			expectError: true,
		},
		{
			name: "Missing amount",
			requestData: map[string]interface{}{
				"code":         "TRON",
				"order_id":     "TEST_ORDER_123",
				"notify_url":   "https://example.com/notify",
				"redirect_url": "https://example.com/redirect",
			},
			expectError: true,
		},
		{
			name: "Missing notify_url",
			requestData: map[string]interface{}{
				"code":         "TRON",
				"order_id":     "TEST_ORDER_123",
				"amount":       100.0,
				"redirect_url": "https://example.com/redirect",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("data", tt.requestData)
				c.Next()
			})
			router.POST("/api/create_transaction", web.CreateTransaction)

			req, _ := http.NewRequest("POST", "/api/create_transaction", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)

			if tt.expectError {
				assert.Equal(t, "error", response["status"])
			}
		})
	}
}

func TestCreateTransaction_NoAvailableWallet(t *testing.T) {
	ctx := context.Background()
	container, _ := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	gin.SetMode(gin.TestMode)

	// 不创建钱包地址，模拟没有可用钱包的情况

	// 模拟配置函数
	originalGetLatestRate := usdt.GetLatestRate
	usdt.GetLatestRate = func() float64 { return 7.20 }
	defer func() { usdt.GetLatestRate = originalGetLatestRate }()

	requestData := map[string]interface{}{
		"code":         "TRON",
		"order_id":     "TEST_ORDER_123",
		"amount":       100.0,
		"notify_url":   "https://example.com/notify",
		"redirect_url": "https://example.com/redirect",
	}

	w := httptest.NewRecorder()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("data", requestData)
		c.Next()
	})
	router.POST("/api/create_transaction", web.CreateTransaction)

	req, _ := http.NewRequest("POST", "/api/create_transaction", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["message"].(string), "还没有配置收款地址")
}

func TestCreateTransaction_InvalidChain(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	gin.SetMode(gin.TestMode)

	// 创建TRON钱包，但请求使用其他链
	wallet := testutils.CreateTestWalletAddress("TRON", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t")
	err := db.Create(wallet).Error
	require.NoError(t, err)

	originalGetLatestRate := usdt.GetLatestRate
	usdt.GetLatestRate = func() float64 { return 7.20 }
	defer func() { usdt.GetLatestRate = originalGetLatestRate }()

	requestData := map[string]interface{}{
		"code":         "INVALID_CHAIN",
		"order_id":     "TEST_ORDER_123",
		"amount":       100.0,
		"notify_url":   "https://example.com/notify",
		"redirect_url": "https://example.com/redirect",
	}

	w := httptest.NewRecorder()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("data", requestData)
		c.Next()
	})
	router.POST("/api/create_transaction", web.CreateTransaction)

	req, _ := http.NewRequest("POST", "/api/create_transaction", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, "error", response["status"])
}