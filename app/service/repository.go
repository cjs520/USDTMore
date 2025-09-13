package service

import (
	"USDTMore/app/model"
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// OrderRepository 订单数据访问接口
type OrderRepository interface {
	// CreateOrder 创建订单
	CreateOrder(ctx context.Context, order *model.TradeOrders) error
	
	// GetOrderByID 根据ID获取订单
	GetOrderByID(ctx context.Context, id int64) (*model.TradeOrders, error)
	
	// GetOrderByTradeID 根据交易ID获取订单
	GetOrderByTradeID(ctx context.Context, tradeID string) (*model.TradeOrders, error)
	
	// GetOrdersByStatus 根据状态获取订单列表
	GetOrdersByStatus(ctx context.Context, status int) ([]model.TradeOrders, error)
	
	// UpdateOrderStatus 更新订单状态（乐观锁）
	UpdateOrderStatus(ctx context.Context, orderID int64, status int, version int64) error
	
	// CheckAmountConflict 检查金额冲突
	CheckAmountConflict(ctx context.Context, chain, address, amount string) error
	
	// ExpireOrders 批量过期订单
	ExpireOrders(ctx context.Context, expireTime time.Time) error
}

// orderRepositoryImpl OrderRepository的GORM实现
type orderRepositoryImpl struct {
	db *gorm.DB
}

// NewOrderRepository 创建新的OrderRepository实例
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepositoryImpl{
		db: db,
	}
}

// CreateOrder 创建订单
func (r *orderRepositoryImpl) CreateOrder(ctx context.Context, order *model.TradeOrders) error {
	result := r.db.WithContext(ctx).Create(order)
	if result.Error != nil {
		return fmt.Errorf("failed to create order: %w", result.Error)
	}
	return nil
}

// GetOrderByID 根据ID获取订单
func (r *orderRepositoryImpl) GetOrderByID(ctx context.Context, id int64) (*model.TradeOrders, error) {
	var order model.TradeOrders
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&order)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("order not found")
		}
		return nil, fmt.Errorf("failed to get order: %w", result.Error)
	}
	return &order, nil
}

// GetOrderByTradeID 根据交易ID获取订单
func (r *orderRepositoryImpl) GetOrderByTradeID(ctx context.Context, tradeID string) (*model.TradeOrders, error) {
	var order model.TradeOrders
	result := r.db.WithContext(ctx).Where("trade_id = ?", tradeID).First(&order)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, errors.New("order not found")
		}
		return nil, fmt.Errorf("failed to get order by trade_id: %w", result.Error)
	}
	return &order, nil
}

// GetOrdersByStatus 根据状态获取订单列表
func (r *orderRepositoryImpl) GetOrdersByStatus(ctx context.Context, status int) ([]model.TradeOrders, error) {
	var orders []model.TradeOrders
	result := r.db.WithContext(ctx).Where("status = ?", status).Find(&orders)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get orders by status: %w", result.Error)
	}
	return orders, nil
}

// UpdateOrderStatus 更新订单状态（乐观锁）
func (r *orderRepositoryImpl) UpdateOrderStatus(ctx context.Context, orderID int64, status int, version int64) error {
	result := r.db.WithContext(ctx).Model(&model.TradeOrders{}).
		Where("id = ? AND version = ?", orderID, version).
		Updates(map[string]interface{}{
			"status":  status,
			"version": version + 1,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update order status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return errors.New("order update failed: version conflict or order not found")
	}

	return nil
}

// CheckAmountConflict 检查金额冲突
func (r *orderRepositoryImpl) CheckAmountConflict(ctx context.Context, chain, address, amount string) error {
	var count int64
	result := r.db.WithContext(ctx).Model(&model.TradeOrders{}).
		Where("status = ? AND chain = ? AND address = ? AND amount = ?", 
			model.OrderStatusWaiting, chain, address, amount).
		Count(&count)

	if result.Error != nil {
		return fmt.Errorf("failed to check amount conflict: %w", result.Error)
	}

	if count > 0 {
		return errors.New("amount already reserved")
	}

	return nil
}

// ExpireOrders 批量过期订单
func (r *orderRepositoryImpl) ExpireOrders(ctx context.Context, expireTime time.Time) error {
	result := r.db.WithContext(ctx).Model(&model.TradeOrders{}).
		Where("status = ? AND expired_at < ?", model.OrderStatusWaiting, expireTime).
		Updates(map[string]interface{}{
			"status": model.OrderStatusExpired,
			"version": gorm.Expr("version + 1"),
		})

	if result.Error != nil {
		return fmt.Errorf("failed to expire orders: %w", result.Error)
	}

	return nil
}