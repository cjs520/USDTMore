package service

import (
	"USDTMore/app/model"
	"context"
	"time"
)

// AmountService 定义金额计算服务接口
type AmountService interface {
	// CalcTradeAmount 计算可用的交易金额
	CalcTradeAmount(ctx context.Context, wallets []model.WalletAddress, rate, money float64) (model.WalletAddress, string, error)
	
	// CalcTradeAmountWithTimeout 带超时的计算交易金额
	CalcTradeAmountWithTimeout(wallets []model.WalletAddress, rate, money float64, timeout time.Duration) (model.WalletAddress, string, error)
	
	// ValidateAmount 验证金额是否有效
	ValidateAmount(amount string) error
	
	// CheckAmountConflict 检查金额冲突
	CheckAmountConflict(ctx context.Context, chain, address, amount string) error
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

// CalcTradeAmount 计算可用的交易金额
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

// ValidateAmount 验证金额是否有效
func (s *amountServiceImpl) ValidateAmount(amount string) error {
	// TODO: 实现金额验证逻辑
	return nil
}

// CheckAmountConflict 检查金额冲突
func (s *amountServiceImpl) CheckAmountConflict(ctx context.Context, chain, address, amount string) error {
	return s.repo.CheckAmountConflict(ctx, chain, address, amount)
}