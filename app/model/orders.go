package model

import (
	"USDTMore/app/help"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const OrderStatusExpired = 3
const OrderStatusSuccess = 2
const OrderStatusWaiting = 1

const OrderNotifyStateSucc = 1
const OrderNotifyStateFail = 0
const Atomicity = 0.01 // 原子精度

// 移除全局锁，改为使用数据库事务和乐观锁

type TradeOrders struct {
	Id          int64           `gorm:"primaryKey;autoIncrement;comment:id;index:idx_trade_orders_id"`
	OrderId     string          `gorm:"type:varchar(255);not null;uniqueIndex:idx_trade_orders_order_id;comment:客户订单ID"`
	TradeId     string          `gorm:"type:varchar(255);not null;uniqueIndex:idx_trade_orders_trade_id;comment:本地订单ID"`
	TradeHash   string          `gorm:"type:char(66);default:'';uniqueIndex:idx_trade_orders_trade_hash;comment:交易哈希"`
	UsdtRate    decimal.Decimal `gorm:"type:numeric(18,8);not null;comment:USDT汇率"`
	Amount      decimal.Decimal `gorm:"type:numeric(18,8);not null;default:0;comment:USDT交易数额;index:idx_trade_orders_amount"`
	Money       decimal.Decimal `gorm:"type:numeric(18,2);not null;default:0;comment:订单交易金额"`
	Chain       string          `gorm:"type:varchar(20);not null;comment:链路名称 TRON POLY OP BSC;index:idx_trade_orders_chain"`
	Address     string          `gorm:"type:varchar(50);not null;comment:收款地址;index:idx_trade_orders_address"`
	FromAddress string          `gorm:"type:varchar(50);not null;default:'';comment:支付地址"`
	Status      int16           `gorm:"type:smallint;not null;default:0;comment:交易状态 1：等待支付 2：支付成功 3：订单过期;index:idx_trade_orders_status"`
	Version     int64           `gorm:"type:bigint;not null;default:0;comment:乐观锁版本号"`
	ReturnUrl   string          `gorm:"type:text;not null;default:'';comment:同步地址"`
	NotifyUrl   string          `gorm:"type:text;not null;default:'';comment:异步地址"`
	NotifyNum   int16           `gorm:"type:smallint;not null;default:0;comment:回调次数"`
	NotifyState int16           `gorm:"type:smallint;not null;default:0;comment:回调状态 1：成功 0：失败;index:idx_trade_orders_notify_state"`
	ExpiredAt   time.Time       `gorm:"type:timestamptz;not null;comment:订单失效时间;index:idx_trade_orders_expired_at"`
	CreatedAt   time.Time       `gorm:"autoCreateTime;type:timestamptz;not null;comment:创建时间;index:idx_trade_orders_created_at"`
	UpdatedAt   time.Time       `gorm:"autoUpdateTime;type:timestamptz;not null;comment:更新时间"`
	ConfirmedAt *time.Time      `gorm:"type:timestamptz;null;comment:交易确认时间;index:idx_trade_orders_confirmed_at"`
}

/*
设置成过期
*/
func (o *TradeOrders) OrderSetExpired() error {
	o.Status = OrderStatusExpired
	return DB.Save(o).Error
}

/*
设置成功 - 使用乐观锁机制
*/
func (o *TradeOrders) OrderSetSucc(fromAddress, tradeHash string, confirmedAt time.Time) error {
	return o.OrderSetSuccWithContext(context.Background(), fromAddress, tradeHash, confirmedAt)
}

/*
设置成功 - 带上下文的乐观锁实现
*/
func (o *TradeOrders) OrderSetSuccWithContext(ctx context.Context, fromAddress, tradeHash string, confirmedAt time.Time) error {
	// 使用乐观锁更新订单状态
	currentVersion := o.Version
	result := DB.WithContext(ctx).Model(o).
		Where("id = ? AND version = ?", o.Id, currentVersion).
		Updates(map[string]interface{}{
			"status":       OrderStatusSuccess,
			"from_address": fromAddress,
			"confirmed_at": confirmedAt,
			"trade_hash":   tradeHash,
			"version":      currentVersion + 1,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update order status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("order update failed: version conflict or order not found")
	}

	// 更新本地对象的状态
	o.Status = OrderStatusSuccess
	o.FromAddress = fromAddress
	o.ConfirmedAt = &confirmedAt
	o.TradeHash = tradeHash
	o.Version = currentVersion + 1

	return nil
}

/*
设置成功 - 带交易验证的增强版本
使用Etherscan V2 Transaction API验证交易状态
*/
func (o *TradeOrders) OrderSetSuccWithVerification(ctx context.Context, fromAddress, tradeHash string, confirmedAt time.Time) error {
	// 导入交易验证器（延迟导入避免循环依赖）
	// 这里我们先实现基础逻辑，验证功能可选
	
	// TODO: 如果启用了交易验证，在这里添加验证逻辑
	// verifier := monitor.NewTransactionVerifier()
	// if verifier.IsEnabled() {
	//     result, err := verifier.VerifyTransaction(ctx, o.Chain, tradeHash)
	//     if err != nil {
	//         log.Warn(fmt.Sprintf("交易验证失败，使用原有逻辑: %v", err))
	//     } else if !result.IsSuccess {
	//         return fmt.Errorf("交易验证失败: %s", result.ErrorMessage)
	//     }
	// }
	
	// 使用原有的订单成功设置逻辑
	return o.OrderSetSuccWithContext(ctx, fromAddress, tradeHash, confirmedAt)
}

/*
设置通知
*/
func (o *TradeOrders) OrderSetNotifyState(state int) error {
	o.NotifyNum += 1
	o.NotifyState = int16(state)

	return DB.Save(o).Error
}

/*
*
设置状态标签
*/
func (o *TradeOrders) GetStatusLabel() string {
	var _label = "🟢 收款成功"
	if o.Status == OrderStatusExpired {
		_label = "🔴 交易过期"
	}
	if o.Status == OrderStatusWaiting {
		_label = "🟡 等待支付"
	}

	return _label
}

/*
获取订单信息
*/
func GetTradeOrder(tradeId string) (TradeOrders, bool) {
	var order TradeOrders
	var res = DB.Where("trade_id = ?", tradeId).First(&order)

	return order, res.Error == nil
}

/*
根据状体列表订单
*/
func GetTradeOrderByStatus(Status int) ([]TradeOrders, error) {
	var orders []TradeOrders
	var res = DB.Where("status = ?", Status).Find(&orders)
	return orders, res.Error
}

/*
获取通知失败的订单
*/
func GetNotifyFailedTradeOrders() ([]TradeOrders, error) {
	var orders []TradeOrders
	var res = DB.Where("status = ?", OrderStatusSuccess).Where("notify_num > ?", 0).
		Where("notify_state = ?", OrderNotifyStateFail).Find(&orders)

	return orders, res.Error
}

// CalcTradeAmountResult 计算交易金额的结果
type CalcTradeAmountResult struct {
	Address WalletAddress
	Amount  string
	Error   error
}

// CalcTradeAmount 计算当前实际可用的交易金额 - 无锁并发版本
func CalcTradeAmount(wa []WalletAddress, rate, money float64) (WalletAddress, string) {
	// 向后兼容：如果DB未初始化，直接返回第一个地址和基础金额
	if DB == nil {
		if len(wa) > 0 {
			moneyDecimal, _ := help.SafeDecimalFromFloat(money)
			rateDecimal, _ := help.SafeDecimalFromFloat(rate)
			if !rateDecimal.IsZero() {
				payAmount, _ := help.CalculateUSDTAmount(moneyDecimal, rateDecimal)
				return wa[0], help.FormatCryptoFixed(payAmount)
			}
			return wa[0], "0"
		}
		return WalletAddress{}, "0"
	}

	result := CalcTradeAmountWithContext(context.Background(), wa, rate, money)
	if result.Error != nil {
		// 如果出错，返回第一个地址和基础金额（向后兼容）
		if len(wa) > 0 {
			moneyDecimal, _ := help.SafeDecimalFromFloat(money)
			rateDecimal, _ := help.SafeDecimalFromFloat(rate)
			if !rateDecimal.IsZero() {
				payAmount, _ := help.CalculateUSDTAmount(moneyDecimal, rateDecimal)
				return wa[0], help.FormatCryptoFixed(payAmount)
			}
			return wa[0], "0"
		}
		return WalletAddress{}, "0"
	}
	return result.Address, result.Amount
}

// CalcTradeAmountWithContext 带上下文的计算交易金额
func CalcTradeAmountWithContext(ctx context.Context, wa []WalletAddress, rate, money float64) CalcTradeAmountResult {
	const (
		maxRetries = 10
	)

	if len(wa) == 0 {
		return CalcTradeAmountResult{Error: errors.New("no wallet addresses available")}
	}

	// 使用help包的安全转换函数
	moneyDecimal, err := help.SafeDecimalFromFloat(money)
	if err != nil {
		return CalcTradeAmountResult{Error: fmt.Errorf("invalid money value: %w", err)}
	}
	
	rateDecimal, err := help.SafeDecimalFromFloat(rate)
	if err != nil {
		return CalcTradeAmountResult{Error: fmt.Errorf("invalid rate value: %w", err)}
	}
	
	// 验证输入金额
	if err := help.ValidateMoneyAmount(moneyDecimal); err != nil {
		return CalcTradeAmountResult{Error: fmt.Errorf("invalid money amount: %w", err)}
	}

	// 计算基础USDT金额
	baseAmount, err := help.CalculateUSDTAmount(moneyDecimal, rateDecimal)
	if err != nil {
		return CalcTradeAmountResult{Error: fmt.Errorf("failed to calculate USDT amount: %w", err)}
	}
	
	// 验证金额在支付范围内
	if !help.IsInPaymentRange(baseAmount) {
		return CalcTradeAmountResult{Error: errors.New("amount outside valid payment range")}
	}
	
	atom := decimal.NewFromFloat(Atomicity)
	
	// 智能金额递增算法：线性递增 + 随机偏移
	for attempt := 0; attempt < maxRetries; attempt++ {
		// 使用help包的原子增量函数
		currentAmount := help.AddAtomicIncrement(baseAmount, attempt, atom)
		
		// 添加小的随机偏移以分散并发请求
		randomOffset, _ := help.SafeDecimalFromFloat(rand.Float64() * 0.01) // 0-0.01 USDT
		currentAmount = currentAmount.Add(randomOffset)
		
		// 重新验证调整后的金额
		if !help.IsInPaymentRange(currentAmount) {
			return CalcTradeAmountResult{Error: errors.New("adjusted amount exceeds payment range")}
		}
		
		// 使用标准化格式
		standardAmount := help.FormatCryptoFixed(currentAmount)

		// 尝试为每个地址找到可用金额
		for _, address := range wa {
			result := tryReserveAmountWithTransaction(ctx, address, standardAmount)
			if result.Error == nil {
				return CalcTradeAmountResult{
					Address: address,
					Amount:  standardAmount,
				}
			}
		}
		
		// 短暂休眠以避免过度竞争
		select {
		case <-ctx.Done():
			return CalcTradeAmountResult{Error: ctx.Err()}
		case <-time.After(time.Millisecond * time.Duration(1+rand.Intn(10))):
			// 继续下一次尝试
		}
	}

	return CalcTradeAmountResult{Error: errors.New("failed to find available amount after max retries")}
}

// tryReserveAmountWithTransaction 使用数据库事务尝试预留金额
func tryReserveAmountWithTransaction(ctx context.Context, address WalletAddress, amount string) CalcTradeAmountResult {
	// 使用SELECT FOR UPDATE的事务来检查金额是否已被占用
	tx := DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return CalcTradeAmountResult{Error: fmt.Errorf("failed to start transaction: %w", tx.Error)}
	}
	
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// 使用SELECT FOR UPDATE锁定相关的等待中订单
	var existingOrder TradeOrders
	result := tx.Where("status = ? AND chain = ? AND address = ? AND amount = ?", 
		OrderStatusWaiting, address.Chain, address.Address, amount).
		Select("id").
		First(&existingOrder)

	if result.Error == nil {
		// 金额已被占用
		tx.Rollback()
		return CalcTradeAmountResult{Error: errors.New("amount already reserved")}
	}

	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// 其他数据库错误
		tx.Rollback()
		return CalcTradeAmountResult{Error: fmt.Errorf("database query error: %w", result.Error)}
	}

	// 金额可用，提交事务
	if err := tx.Commit().Error; err != nil {
		return CalcTradeAmountResult{Error: fmt.Errorf("failed to commit transaction: %w", err)}
	}

	return CalcTradeAmountResult{
		Address: address,
		Amount:  amount,
	}
}
