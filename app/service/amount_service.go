package service

import (
	"USDTMore/app/help"
	"USDTMore/app/model"
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// AmountService 定义金额计算服务接口
type AmountService interface {
	// CalcTradeAmount 计算可用的交易金额（使用decimal）
	CalcTradeAmountDecimal(ctx context.Context, wallets []model.WalletAddress, rate, money decimal.Decimal) (model.WalletAddress, decimal.Decimal, error)
	
	// CalcTradeAmount 计算可用的交易金额（向后兼容）
	CalcTradeAmount(ctx context.Context, wallets []model.WalletAddress, rate, money float64) (model.WalletAddress, string, error)
	
	// CalcTradeAmountWithTimeout 带超时的计算交易金额
	CalcTradeAmountWithTimeout(wallets []model.WalletAddress, rate, money float64, timeout time.Duration) (model.WalletAddress, string, error)
	
	// ValidateAmount 验证金额是否有效（使用decimal）
	ValidateAmountDecimal(amount decimal.Decimal) error
	
	// ValidateAmount 验证金额是否有效（字符串版本）
	ValidateAmount(amount string) error
	
	// CheckAmountConflict 检查金额冲突
	CheckAmountConflict(ctx context.Context, chain, address string, amount decimal.Decimal) error
}

// amountServiceImpl AmountService的实现
type amountServiceImpl struct {
	repo OrderRepository
}

// NewAmountService 创建新的AmountService实例
func NewAmountService(repo OrderRepository) AmountService {
	return &amountServiceImpl{
		repo: repo,
	}
}

// CalcTradeAmountDecimal 计算可用的交易金额（使用decimal）
func (s *amountServiceImpl) CalcTradeAmountDecimal(ctx context.Context, wallets []model.WalletAddress, rate, money decimal.Decimal) (model.WalletAddress, decimal.Decimal, error) {
	// 验证输入参数
	if err := help.ValidateMoneyAmount(money); err != nil {
		return model.WalletAddress{}, decimal.Zero, err
	}
	
	// 验证汇率
	if rate.IsZero() {
		return model.WalletAddress{}, decimal.Zero, errors.New("rate cannot be zero")
	}
	
	// 调用底层计算逻辑
	rateFloat, _ := help.ToFloat64Safe(rate)
	moneyFloat, _ := help.ToFloat64Safe(money)
	result := model.CalcTradeAmountWithContext(ctx, wallets, rateFloat, moneyFloat)
	if result.Error != nil {
		return model.WalletAddress{}, decimal.Zero, result.Error
	}
	
	// 转换返回的字符串金额为decimal
	resultAmount, err := help.SafeDecimalFromString(result.Amount)
	if err != nil {
		return model.WalletAddress{}, decimal.Zero, err
	}
	
	return result.Address, resultAmount, nil
}

// CalcTradeAmount 计算可用的交易金额（向后兼容）
func (s *amountServiceImpl) CalcTradeAmount(ctx context.Context, wallets []model.WalletAddress, rate, money float64) (model.WalletAddress, string, error) {
	result := model.CalcTradeAmountWithContext(ctx, wallets, rate, money)
	if result.Error != nil {
		return model.WalletAddress{}, "", result.Error
	}
	return result.Address, result.Amount, nil
}

// CalcTradeAmountWithTimeout 带超时的计算交易金额
func (s *amountServiceImpl) CalcTradeAmountWithTimeout(wallets []model.WalletAddress, rate, money float64, timeout time.Duration) (model.WalletAddress, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	return s.CalcTradeAmount(ctx, wallets, rate, money)
}

// ValidateAmountDecimal 验证decimal金额是否有效
func (s *amountServiceImpl) ValidateAmountDecimal(amount decimal.Decimal) error {
	return help.ValidateCryptoAmount(amount)
}

// ValidateAmount 验证字符串金额是否有效
func (s *amountServiceImpl) ValidateAmount(amount string) error {
	amountDecimal, err := help.SafeDecimalFromString(amount)
	if err != nil {
		return err
	}
	return s.ValidateAmountDecimal(amountDecimal)
}

// CheckAmountConflict 检查金额冲突
func (s *amountServiceImpl) CheckAmountConflict(ctx context.Context, chain, address string, amount decimal.Decimal) error {
	amountStr := help.FormatCryptoFixed(amount)
	return s.repo.CheckAmountConflict(ctx, chain, address, amountStr)
}