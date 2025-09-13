package tests

import (
	"USDTMore/app/model"
	"USDTMore/app/service"
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestCalcTradeAmountLogic(t *testing.T) {
	// 测试新的无锁CalcTradeAmount逻辑
	wallets := []model.WalletAddress{
		{Chain: "TRON", Address: "TAddress1234567890123456789012345"},
		{Chain: "TRON", Address: "TAddress2345678901234567890123456"},
	}

	rate := 7.20
	money := 100.0

	// 这个测试验证基本的金额计算逻辑，不需要数据库
	expectedAmount := decimal.NewFromFloat(money / rate).StringFixed(2)
	
	// 调用原始函数（向后兼容）
	address, amount := model.CalcTradeAmount(wallets, rate, money)
	
	// 验证返回了一个有效的地址
	assert.NotEmpty(t, address.Address)
	assert.NotEmpty(t, address.Chain)
	
	// 验证金额格式正确
	assert.NotEmpty(t, amount)
	
	// 金额应该接近期望值（可能有原子增量）
	actualAmount, _ := decimal.NewFromString(amount)
	expectedAmountDec, _ := decimal.NewFromString(expectedAmount)
	
	// 差值应该小于1 USDT
	diff := actualAmount.Sub(expectedAmountDec).Abs()
	assert.True(t, diff.LessThan(decimal.NewFromInt(1)), "Amount difference should be less than 1 USDT")
}

func TestRetryManagerBasicLogic(t *testing.T) {
	// 测试重试管理器的基本逻辑
	rm := service.NewRetryManager()
	
	ctx := context.Background()
	
	// 测试成功的操作
	callCount := 0
	err := rm.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		callCount++
		return nil
	})
	
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestRetryManagerRetryLogic(t *testing.T) {
	// 测试重试逻辑
	rm := service.NewRetryManager()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// 测试重试后成功的操作
	callCount := 0
	err := rm.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		callCount++
		if callCount < 3 {
			return service.NewRetryableError("temporary failure")
		}
		return nil
	})
	
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}