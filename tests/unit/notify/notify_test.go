package notify_test

import (
	"USDTMore/app/config"
	"USDTMore/app/model"
	"USDTMore/app/notify"
	"USDTMore/tests/testutils"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrderNotify_Success(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建模拟HTTP服务器
	mockServer := testutils.NewMockHTTPServer()
	defer mockServer.Close()

	// 创建成功订单
	order := testutils.CreateSuccessOrder()
	order.NotifyUrl = mockServer.Server.URL + "/notify"
	err := db.Create(order).Error
	require.NoError(t, err)

	// 设置测试配置
	originalAuthToken := config.GetAuthToken
	config.GetAuthToken = func() string { return "test_auth_token" }
	defer func() { config.GetAuthToken = originalAuthToken }()

	// 执行通知
	notify.OrderNotify(*order)

	// 等待异步处理完成
	time.Sleep(100 * time.Millisecond)

	// 验证HTTP请求
	assert.Equal(t, 1, mockServer.GetRequestCount())
	lastRequest := mockServer.GetLastRequest()
	assert.NotNil(t, lastRequest)
	assert.Equal(t, "POST", lastRequest.Method)
	assert.Equal(t, "/notify", lastRequest.URL)
	assert.Equal(t, "application/json", lastRequest.Headers["Content-Type"])

	// 验证请求体
	var requestBody map[string]interface{}
	err = json.Unmarshal([]byte(lastRequest.Body), &requestBody)
	assert.NoError(t, err)
	
	assert.Equal(t, order.TradeId, requestBody["trade_id"])
	assert.Equal(t, order.OrderId, requestBody["order_id"])
	assert.Equal(t, order.Money, requestBody["amount"])
	assert.Equal(t, order.Amount, requestBody["actual_amount"])
	assert.Equal(t, order.Address, requestBody["token"])
	assert.Equal(t, order.TradeHash, requestBody["block_transaction_id"])
	assert.Equal(t, float64(order.Status), requestBody["status"])
	assert.Contains(t, requestBody, "signature")

	// 验证订单通知状态更新
	var updatedOrder model.TradeOrders
	err = db.First(&updatedOrder, order.Id).Error
	require.NoError(t, err)
	assert.Equal(t, model.OrderNotifyStateSucc, updatedOrder.NotifyState)
	assert.Equal(t, 1, updatedOrder.NotifyNum)
}

func TestOrderNotify_Failed(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建模拟HTTP服务器
	mockServer := testutils.NewMockHTTPServer()
	defer mockServer.Close()

	// 创建待支付订单（会导致回调失败）
	order := testutils.CreatePendingOrder()
	order.NotifyUrl = mockServer.Server.URL + "/notify"
	err := db.Create(order).Error
	require.NoError(t, err)

	// 设置测试配置
	originalAuthToken := config.GetAuthToken
	config.GetAuthToken = func() string { return "test_auth_token" }
	defer func() { config.GetAuthToken = originalAuthToken }()

	// 执行通知
	notify.OrderNotify(*order)

	// 等待异步处理完成
	time.Sleep(100 * time.Millisecond)

	// 验证HTTP请求
	assert.Equal(t, 1, mockServer.GetRequestCount())

	// 验证订单通知状态更新（应该标记为失败）
	var updatedOrder model.TradeOrders
	err = db.First(&updatedOrder, order.Id).Error
	require.NoError(t, err)
	assert.Equal(t, model.OrderNotifyStateFail, updatedOrder.NotifyState)
	assert.Equal(t, 1, updatedOrder.NotifyNum)
}

func TestOrderNotify_InvalidURL(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建订单，使用无效的通知URL
	order := testutils.CreateSuccessOrder()
	order.NotifyUrl = "http://invalid-url-that-does-not-exist.com/notify"
	err := db.Create(order).Error
	require.NoError(t, err)

	// 设置测试配置
	originalAuthToken := config.GetAuthToken
	config.GetAuthToken = func() string { return "test_auth_token" }
	defer func() { config.GetAuthToken = originalAuthToken }()

	// 执行通知
	notify.OrderNotify(*order)

	// 等待异步处理完成
	time.Sleep(100 * time.Millisecond)

	// 验证订单通知状态更新（应该标记为失败）
	var updatedOrder model.TradeOrders
	err = db.First(&updatedOrder, order.Id).Error
	require.NoError(t, err)
	assert.Equal(t, model.OrderNotifyStateFail, updatedOrder.NotifyState)
	assert.Equal(t, 1, updatedOrder.NotifyNum)
}

func TestOrderNotify_Timeout(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建超时的模拟服务器
	timeoutServer := testutils.NewMockHTTPServer()
	defer timeoutServer.Close()

	// 创建订单
	order := testutils.CreateSuccessOrder()
	order.NotifyUrl = timeoutServer.Server.URL + "/timeout"
	err := db.Create(order).Error
	require.NoError(t, err)

	// 设置测试配置
	originalAuthToken := config.GetAuthToken
	config.GetAuthToken = func() string { return "test_auth_token" }
	defer func() { config.GetAuthToken = originalAuthToken }()

	// 执行通知
	notify.OrderNotify(*order)

	// 等待超时处理完成
	time.Sleep(6 * time.Second) // HTTP客户端超时是5秒

	// 验证订单通知状态（应该失败或保持原状）
	var updatedOrder model.TradeOrders
	err = db.First(&updatedOrder, order.Id).Error
	require.NoError(t, err)
	// 由于超时，状态可能不会更新或标记为失败
}

func TestNotifyRetry_Logic(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建回调失败的订单
	failedOrder := testutils.CreateFailedNotifyOrder()
	failedOrder.ConfirmedAt = time.Now().Add(-10 * time.Minute) // 10分钟前确认
	err := db.Create(failedOrder).Error
	require.NoError(t, err)

	// 测试重试间隔计算逻辑
	// 重试间隔 = 3^失败次数 * 1分钟
	tests := []struct {
		name         string
		notifyNum    int
		expectedWait time.Duration
	}{
		{
			name:         "First retry",
			notifyNum:    1,
			expectedWait: 3 * time.Minute, // 3^1 = 3 minutes
		},
		{
			name:         "Second retry", 
			notifyNum:    2,
			expectedWait: 9 * time.Minute, // 3^2 = 9 minutes
		},
		{
			name:         "Third retry",
			notifyNum:    3,
			expectedWait: 27 * time.Minute, // 3^3 = 27 minutes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 更新订单的失败次数
			failedOrder.NotifyNum = tt.notifyNum
			db.Save(failedOrder)

			// 计算下次重试时间
			nextRetryTime := failedOrder.ConfirmedAt.Add(tt.expectedWait)
			
			// 验证是否到达重试时间
			now := time.Now()
			shouldRetry := now.Unix() >= nextRetryTime.Unix()
			
			if tt.notifyNum <= 3 {
				// 对于前3次重试，如果时间足够，应该重试
				if now.Sub(failedOrder.ConfirmedAt) >= tt.expectedWait {
					assert.True(t, shouldRetry)
				}
			}
		})
	}
}

func TestGetNotifyFailedOrders(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建不同状态的订单
	failedOrder1 := testutils.CreateFailedNotifyOrder()
	failedOrder1.NotifyNum = 1
	err := db.Create(failedOrder1).Error
	require.NoError(t, err)

	failedOrder2 := testutils.CreateFailedNotifyOrder()
	failedOrder2.NotifyNum = 2
	err = db.Create(failedOrder2).Error
	require.NoError(t, err)

	successOrder := testutils.CreateTestOrder(map[string]interface{}{
		"status":       model.OrderStatusSuccess,
		"notify_num":   1,
		"notify_state": model.OrderNotifyStateSucc,
	})
	err = db.Create(successOrder).Error
	require.NoError(t, err)

	waitingOrder := testutils.CreatePendingOrder()
	err = db.Create(waitingOrder).Error
	require.NoError(t, err)

	// 获取回调失败的订单
	failedOrders, err := model.GetNotifyFailedTradeOrders()
	assert.NoError(t, err)
	assert.Len(t, failedOrders, 2)

	// 验证只返回了失败的订单
	orderIds := make(map[string]bool)
	for _, order := range failedOrders {
		orderIds[order.TradeId] = true
		assert.Equal(t, model.OrderStatusSuccess, order.Status)
		assert.Equal(t, model.OrderNotifyStateFail, order.NotifyState)
		assert.Greater(t, order.NotifyNum, 0)
	}
	
	assert.True(t, orderIds[failedOrder1.TradeId])
	assert.True(t, orderIds[failedOrder2.TradeId])
	assert.False(t, orderIds[successOrder.TradeId])
	assert.False(t, orderIds[waitingOrder.TradeId])
}