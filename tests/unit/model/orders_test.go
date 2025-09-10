package model_test

import (
	"USDTMore/app/model"
	"USDTMore/tests/testutils"
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTradeOrders_OrderSetExpired(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建测试订单
	order := testutils.CreateTestOrder()
	err := db.Create(order).Error
	require.NoError(t, err)

	// 测试设置过期状态
	err = order.OrderSetExpired()
	assert.NoError(t, err)
	assert.Equal(t, model.OrderStatusExpired, order.Status)

	// 验证数据库中的状态
	var dbOrder model.TradeOrders
	err = db.First(&dbOrder, order.Id).Error
	require.NoError(t, err)
	assert.Equal(t, model.OrderStatusExpired, dbOrder.Status)
}

func TestTradeOrders_OrderSetSucc(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建测试订单
	order := testutils.CreateTestOrder()
	err := db.Create(order).Error
	require.NoError(t, err)

	// 测试设置成功状态
	fromAddress := "TTestFromAddress12345678901234567890"
	tradeHash := "test_tx_hash_123"
	confirmedAt := time.Now()

	err = order.OrderSetSucc(fromAddress, tradeHash, confirmedAt)
	assert.NoError(t, err)
	assert.Equal(t, model.OrderStatusSuccess, order.Status)
	assert.Equal(t, fromAddress, order.FromAddress)
	assert.Equal(t, tradeHash, order.TradeHash)
	assert.WithinDuration(t, confirmedAt, order.ConfirmedAt, time.Second)

	// 验证数据库中的状态
	var dbOrder model.TradeOrders
	err = db.First(&dbOrder, order.Id).Error
	require.NoError(t, err)
	assert.Equal(t, model.OrderStatusSuccess, dbOrder.Status)
	assert.Equal(t, fromAddress, dbOrder.FromAddress)
	assert.Equal(t, tradeHash, dbOrder.TradeHash)
}

func TestTradeOrders_OrderSetNotifyState(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建测试订单
	order := testutils.CreateTestOrder()
	err := db.Create(order).Error
	require.NoError(t, err)

	initialNotifyNum := order.NotifyNum

	// 测试设置通知状态
	err = order.OrderSetNotifyState(model.OrderNotifyStateSucc)
	assert.NoError(t, err)
	assert.Equal(t, model.OrderNotifyStateSucc, order.NotifyState)
	assert.Equal(t, initialNotifyNum+1, order.NotifyNum)

	// 再次设置失败状态
	err = order.OrderSetNotifyState(model.OrderNotifyStateFail)
	assert.NoError(t, err)
	assert.Equal(t, model.OrderNotifyStateFail, order.NotifyState)
	assert.Equal(t, initialNotifyNum+2, order.NotifyNum)
}

func TestTradeOrders_GetStatusLabel(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		expectedLabel  string
	}{
		{
			name:          "Waiting status",
			status:        model.OrderStatusWaiting,
			expectedLabel: "🟡 等待支付",
		},
		{
			name:          "Success status",
			status:        model.OrderStatusSuccess,
			expectedLabel: "🟢 收款成功",
		},
		{
			name:          "Expired status",
			status:        model.OrderStatusExpired,
			expectedLabel: "🔴 交易过期",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := testutils.CreateTestOrder(map[string]interface{}{
				"status": tt.status,
			})
			label := order.GetStatusLabel()
			assert.Equal(t, tt.expectedLabel, label)
		})
	}
}

func TestGetTradeOrder(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建测试订单
	order := testutils.CreateTestOrder()
	err := db.Create(order).Error
	require.NoError(t, err)

	// 测试获取存在的订单
	foundOrder, exists := model.GetTradeOrder(order.TradeId)
	assert.True(t, exists)
	assert.Equal(t, order.TradeId, foundOrder.TradeId)
	assert.Equal(t, order.OrderId, foundOrder.OrderId)

	// 测试获取不存在的订单
	_, exists = model.GetTradeOrder("non_existent_id")
	assert.False(t, exists)
}

func TestGetTradeOrderByStatus(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建不同状态的订单
	waitingOrder := testutils.CreateTestOrder(map[string]interface{}{
		"status": model.OrderStatusWaiting,
	})
	successOrder := testutils.CreateTestOrder(map[string]interface{}{
		"status": model.OrderStatusSuccess,
	})
	expiredOrder := testutils.CreateTestOrder(map[string]interface{}{
		"status": model.OrderStatusExpired,
	})

	err := db.Create(waitingOrder).Error
	require.NoError(t, err)
	err = db.Create(successOrder).Error
	require.NoError(t, err)
	err = db.Create(expiredOrder).Error
	require.NoError(t, err)

	// 测试获取等待状态的订单
	waitingOrders, err := model.GetTradeOrderByStatus(model.OrderStatusWaiting)
	assert.NoError(t, err)
	assert.Len(t, waitingOrders, 1)
	assert.Equal(t, waitingOrder.TradeId, waitingOrders[0].TradeId)

	// 测试获取成功状态的订单
	successOrders, err := model.GetTradeOrderByStatus(model.OrderStatusSuccess)
	assert.NoError(t, err)
	assert.Len(t, successOrders, 1)
	assert.Equal(t, successOrder.TradeId, successOrders[0].TradeId)
}

func TestGetNotifyFailedTradeOrders(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建回调失败的订单
	failedOrder := testutils.CreateFailedNotifyOrder()
	err := db.Create(failedOrder).Error
	require.NoError(t, err)

	// 创建回调成功的订单
	successOrder := testutils.CreateTestOrder(map[string]interface{}{
		"status":       model.OrderStatusSuccess,
		"notify_num":   1,
		"notify_state": model.OrderNotifyStateSucc,
	})
	err = db.Create(successOrder).Error
	require.NoError(t, err)

	// 测试获取回调失败的订单
	failedOrders, err := model.GetNotifyFailedTradeOrders()
	assert.NoError(t, err)
	assert.Len(t, failedOrders, 1)
	assert.Equal(t, failedOrder.TradeId, failedOrders[0].TradeId)
}

func TestCalcTradeAmount(t *testing.T) {
	ctx := context.Background()
	container, db := testutils.SetupTestDB(ctx, t)
	defer testutils.TeardownTestDB(ctx, container)

	// 创建钱包地址
	wallet1 := testutils.CreateTestWalletAddress("TRON", "TAddress1234567890123456789012345")
	wallet2 := testutils.CreateTestWalletAddress("TRON", "TAddress2345678901234567890123456")
	err := db.Create(wallet1).Error
	require.NoError(t, err)
	err = db.Create(wallet2).Error
	require.NoError(t, err)

	wallets := []model.WalletAddress{*wallet1, *wallet2}

	t.Run("No existing orders", func(t *testing.T) {
		testutils.CleanDatabase(db)
		db.Create(wallet1)
		db.Create(wallet2)
		
		wallets := []model.WalletAddress{*wallet1, *wallet2}
		rate := 7.20
		money := 100.0

		address, amount := model.CalcTradeAmount(wallets, rate, money)
		
		assert.Equal(t, wallet1.Address, address.Address)
		assert.Equal(t, wallet1.Chain, address.Chain)
		
		expectedAmount := decimal.NewFromFloat(money / rate).StringFixed(2)
		assert.Equal(t, expectedAmount, amount)
	})

	t.Run("With existing conflicting order", func(t *testing.T) {
		testutils.CleanDatabase(db)
		db.Create(wallet1)
		db.Create(wallet2)

		rate := 7.20
		money := 100.0
		expectedAmount := decimal.NewFromFloat(money / rate).StringFixed(2)

		// 创建一个占用第一个地址和金额的订单
		conflictOrder := testutils.CreateTestOrder(map[string]interface{}{
			"chain":   "TRON",
			"address": wallet1.Address,
			"amount":  expectedAmount,
			"status":  model.OrderStatusWaiting,
		})
		err := db.Create(conflictOrder).Error
		require.NoError(t, err)

		wallets := []model.WalletAddress{*wallet1, *wallet2}
		address, amount := model.CalcTradeAmount(wallets, rate, money)

		// 应该使用第二个地址或者递增金额
		if address.Address == wallet1.Address {
			// 如果使用了同一个地址，金额应该递增
			expectedIncrement := decimal.NewFromFloat(money / rate).Add(decimal.NewFromFloat(model.Atomicity))
			assert.Equal(t, expectedIncrement.StringFixed(2), amount)
		} else {
			// 如果使用了不同地址，可以使用原金额
			assert.Equal(t, wallet2.Address, address.Address)
		}
	})
}