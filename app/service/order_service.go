package service

import (
	"USDTMore/app/model"
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// OrderService 定义订单服务接口
type OrderService interface {
	// CreateOrder 创建新订单
	CreateOrder(ctx context.Context, req CreateOrderRequest) (*model.TradeOrders, error)
	
	// UpdateOrderStatus 更新订单状态（使用乐观锁）
	UpdateOrderStatus(ctx context.Context, orderID int64, status int, version int64) error
	
	// SetOrderSuccess 设置订单为成功状态
	SetOrderSuccess(ctx context.Context, orderID int64, fromAddress, tradeHash string, confirmedAt time.Time) error
	
	// GetOrderByTradeID 根据交易ID获取订单
	GetOrderByTradeID(ctx context.Context, tradeID string) (*model.TradeOrders, error)
	
	// GetPendingOrders 获取所有待支付订单
	GetPendingOrders(ctx context.Context) ([]model.TradeOrders, error)
	
	// ExpireOrders 过期订单处理
	ExpireOrders(ctx context.Context) error
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	OrderID     string
	TradeID     string
	Chain       string
	Address     string
	Amount      decimal.Decimal
	Money       decimal.Decimal
	UsdtRate    decimal.Decimal
	ReturnURL   string
	NotifyURL   string
	ExpiredAt   time.Time
}

// orderServiceImpl OrderService的实现
type orderServiceImpl struct {
	repo         OrderRepository
	amountSvc    AmountService
	retryManager *RetryManager
}

// NewOrderService 创建新的OrderService实例
func NewOrderService(repo OrderRepository, amountSvc AmountService) OrderService {
	return &orderServiceImpl{
		repo:         repo,
		amountSvc:    amountSvc,
		retryManager: NewRetryManager(),
	}
}

// CreateOrder 创建新订单
func (s *orderServiceImpl) CreateOrder(ctx context.Context, req CreateOrderRequest) (*model.TradeOrders, error) {
	order := &model.TradeOrders{
		OrderId:     req.OrderID,
		TradeId:     req.TradeID,
		Chain:       req.Chain,
		Address:     req.Address,
		Amount:      req.Amount,
		Money:       req.Money,
		UsdtRate:    req.UsdtRate,
		Status:      model.OrderStatusWaiting,
		ReturnUrl:   req.ReturnURL,
		NotifyUrl:   req.NotifyURL,
		ExpiredAt:   req.ExpiredAt,
		Version:     0, // 初始版本
	}

	// 使用事务创建订单
	err := s.retryManager.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		return s.repo.CreateOrder(ctx, order)
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	return order, nil
}

// UpdateOrderStatus 更新订单状态（使用乐观锁）
func (s *orderServiceImpl) UpdateOrderStatus(ctx context.Context, orderID int64, status int, version int64) error {
	return s.retryManager.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		return s.repo.UpdateOrderStatus(ctx, orderID, status, version)
	})
}

// SetOrderSuccess 设置订单为成功状态
func (s *orderServiceImpl) SetOrderSuccess(ctx context.Context, orderID int64, fromAddress, tradeHash string, confirmedAt time.Time) error {
	return s.retryManager.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		order, err := s.repo.GetOrderByID(ctx, orderID)
		if err != nil {
			return fmt.Errorf("failed to get order: %w", err)
		}

		return order.OrderSetSuccWithContext(ctx, fromAddress, tradeHash, confirmedAt)
	})
}

// GetOrderByTradeID 根据交易ID获取订单
func (s *orderServiceImpl) GetOrderByTradeID(ctx context.Context, tradeID string) (*model.TradeOrders, error) {
	return s.repo.GetOrderByTradeID(ctx, tradeID)
}

// GetPendingOrders 获取所有待支付订单
func (s *orderServiceImpl) GetPendingOrders(ctx context.Context) ([]model.TradeOrders, error) {
	return s.repo.GetOrdersByStatus(ctx, model.OrderStatusWaiting)
}

// ExpireOrders 过期订单处理
func (s *orderServiceImpl) ExpireOrders(ctx context.Context) error {
	return s.retryManager.ExecuteWithRetry(ctx, func(ctx context.Context) error {
		return s.repo.ExpireOrders(ctx, time.Now())
	})
}