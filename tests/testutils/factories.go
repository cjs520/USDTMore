package testutils

import (
	"USDTMore/app/model"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CreateTestOrder 创建测试订单
func CreateTestOrder(customFields ...map[string]interface{}) *model.TradeOrders {
	order := &model.TradeOrders{
		OrderId:     fmt.Sprintf("TEST_%s", uuid.New().String()[:8]),
		TradeId:     fmt.Sprintf("TID_%s", uuid.New().String()[:8]),
		TradeHash:   "",
		UsdtRate:    "7.20",
		Amount:      "100.00",
		Money:       720.00,
		Chain:       "TRON",
		Address:     "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		FromAddress: "",
		Status:      model.OrderStatusWaiting,
		ReturnUrl:   "https://example.com/return",
		NotifyUrl:   "https://example.com/notify",
		NotifyNum:   0,
		NotifyState: model.OrderNotifyStateFail,
		ExpiredAt:   time.Now().Add(30 * time.Minute),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 应用自定义字段
	if len(customFields) > 0 {
		fields := customFields[0]
		if v, ok := fields["order_id"].(string); ok {
			order.OrderId = v
		}
		if v, ok := fields["trade_id"].(string); ok {
			order.TradeId = v
		}
		if v, ok := fields["trade_hash"].(string); ok {
			order.TradeHash = v
		}
		if v, ok := fields["amount"].(string); ok {
			order.Amount = v
		}
		if v, ok := fields["money"].(float64); ok {
			order.Money = v
		}
		if v, ok := fields["chain"].(string); ok {
			order.Chain = v
		}
		if v, ok := fields["address"].(string); ok {
			order.Address = v
		}
		if v, ok := fields["from_address"].(string); ok {
			order.FromAddress = v
		}
		if v, ok := fields["status"].(int); ok {
			order.Status = v
		}
		if v, ok := fields["notify_url"].(string); ok {
			order.NotifyUrl = v
		}
		if v, ok := fields["return_url"].(string); ok {
			order.ReturnUrl = v
		}
		if v, ok := fields["notify_num"].(int); ok {
			order.NotifyNum = v
		}
		if v, ok := fields["notify_state"].(int); ok {
			order.NotifyState = v
		}
		if v, ok := fields["expired_at"].(time.Time); ok {
			order.ExpiredAt = v
		}
		if v, ok := fields["created_at"].(time.Time); ok {
			order.CreatedAt = v
		}
		if v, ok := fields["confirmed_at"].(time.Time); ok {
			order.ConfirmedAt = v
		}
	}

	return order
}

// CreateTestWalletAddress 创建测试钱包地址
func CreateTestWalletAddress(chain string, address string) *model.WalletAddress {
	return &model.WalletAddress{
		Chain:      chain,
		Address:    address,
		StartBlock: 0,
		OtherNotify: 0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// CreateTestTransactionData 创建模拟交易数据
func CreateTestTransactionData(chain string, toAddress string, amount decimal.Decimal, txHash string, timestamp time.Time) map[string]interface{} {
	switch chain {
	case "TRON":
		return map[string]interface{}{
			"token_transfers": []map[string]interface{}{
				{
					"transaction_id": txHash,
					"to_address":     toAddress,
					"from_address":   "TTestFromAddress12345678901234567890",
					"quant":          amount.Mul(decimal.NewFromFloat(1000000)).IntPart(), // TRC20 USDT has 6 decimals
					"contractRet":    "SUCCESS",
					"block_ts":       timestamp.UnixMilli(),
				},
			},
			"total": 1,
		}
	case "POLY", "OP", "BSC", "ARB":
		return map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"hash":             txHash,
					"to":               toAddress,
					"from":             "0x1234567890123456789012345678901234567890",
					"value":            amount.Mul(decimal.NewFromFloat(1000000)).String(), // 6 decimals
					"tokenSymbol":      "USDT",
					"tokenDecimal":     "6",
					"contractAddress":  "0xdAC17F958D2ee523a2206206994597C13D831ec7",
					"timeStamp":        fmt.Sprintf("%d", timestamp.Unix()),
				},
			},
		}
	default:
		return map[string]interface{}{}
	}
}

// CreateExpiredOrder 创建过期订单
func CreateExpiredOrder() *model.TradeOrders {
	return CreateTestOrder(map[string]interface{}{
		"expired_at": time.Now().Add(-1 * time.Hour), // 1小时前过期
		"created_at": time.Now().Add(-2 * time.Hour), // 2小时前创建
	})
}

// CreateSuccessOrder 创建成功订单
func CreateSuccessOrder() *model.TradeOrders {
	return CreateTestOrder(map[string]interface{}{
		"status":       model.OrderStatusSuccess,
		"trade_hash":   "0x123456789abcdef",
		"from_address": "TTestFromAddress12345678901234567890",
	})
}

// CreatePendingOrder 创建待支付订单
func CreatePendingOrder() *model.TradeOrders {
	return CreateTestOrder(map[string]interface{}{
		"status": model.OrderStatusWaiting,
	})
}

// CreateFailedNotifyOrder 创建回调失败的订单
func CreateFailedNotifyOrder() *model.TradeOrders {
	return CreateTestOrder(map[string]interface{}{
		"status":       model.OrderStatusSuccess,
		"notify_num":   3,
		"notify_state": model.OrderNotifyStateFail,
		"trade_hash":   "0x123456789abcdef",
	})
}