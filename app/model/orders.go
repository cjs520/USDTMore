package model

import (
	"fmt"
	"github.com/shopspring/decimal"
	"strconv"
	"sync"
	"time"
)

const OrderStatusExpired = 3
const OrderStatusSuccess = 2
const OrderStatusWaiting = 1

const OrderNotifyStateSucc = 1
const OrderNotifyStateFail = 0
const Atomicity = 0.01 // 原子精度

var _calcMutex sync.Mutex

type TradeOrders struct {
	Id          int64     `gorm:"primary_key;AUTO_INCREMENT;comment:id"`
	OrderId     string    `gorm:"type:varchar(255);not null;unique;color:blue;comment:客户订单ID"`
	TradeId     string    `gorm:"type:varchar(255);not null;unique;color:blue;comment:本地订单ID"`
	TradeHash   string    `gorm:"type:varchar(64);default:'';unique;comment:交易哈希"`
	UsdtRate    string    `gorm:"type:varchar(10);not null;comment:USDT汇率"`
	Amount      string    `gorm:"type:decimal(10,2);not null;default:0;comment:USDT交易数额"`
	Money       float64   `gorm:"type:decimal(10,2);not null;default:0;comment:订单交易金额"`
	Chain       string    `gorm:"type:varchar(255);not null;comment:链路名称 TRON POLY OP BSC"`
	Address     string    `gorm:"type:varchar(34);not null;comment:收款地址"`
	FromAddress string    `gorm:"type:varchar(34);not null;default:'';comment:支付地址"`
	Status      int       `gorm:"type:tinyint(1);not null;default:0;comment:交易状态 1：等待支付 2：支付成功 3：订单过期"`
	ReturnUrl   string    `gorm:"type:varchar(255);not null;default:'';comment:同步地址"`
	NotifyUrl   string    `gorm:"type:varchar(255);not null;default:'';comment:异步地址"`
	NotifyNum   int       `gorm:"type:int(11);not null;default:0;comment:回调次数"`
	NotifyState int       `gorm:"type:tinyint(1);not null;default:0;comment:回调状态 1：成功 0：失败"`
	ExpiredAt   time.Time `gorm:"type:timestamp;not null;comment:订单失效时间"`
	CreatedAt   time.Time `gorm:"autoCreateTime;type:timestamp;not null;comment:创建时间"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime;type:timestamp;not null;comment:更新时间"`
	ConfirmedAt time.Time `gorm:"type:timestamp;null;comment:交易确认时间"`
}

/*
设置成过期
*/
func (o *TradeOrders) OrderSetExpired() error {
	o.Status = OrderStatusExpired
	return DB.Save(o).Error
}

/*
设置成工程
*/
func (o *TradeOrders) OrderSetSucc(fromAddress, tradeHash string, confirmedAt time.Time) error {
	// 检查订单状态，防止重复更新
	if o.Status != OrderStatusWaiting {
		return fmt.Errorf("订单状态不正确，当前状态: %d", o.Status)
	}

	// 订单标记交易成功
	o.Status = OrderStatusSuccess
	o.FromAddress = fromAddress
	o.ConfirmedAt = confirmedAt
	o.TradeHash = tradeHash
	r := DB.Save(o)

	return r.Error
}

/*
设置通知
*/
func (o *TradeOrders) OrderSetNotifyState(state int) error {
	o.NotifyNum += 1
	o.NotifyState = state

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

// CalcTradeAmount 计算当前实际可用的交易金额
func CalcTradeAmount(wa []WalletAddress, rate, money float64) (WalletAddress, string) {
	_calcMutex.Lock()
	defer _calcMutex.Unlock()

	// 检查钱包地址是否为空
	if len(wa) == 0 {
		return WalletAddress{}, ""
	}

	var _orders []TradeOrders
	var _lock = make(map[string]bool)
	DB.Where("status = ?", OrderStatusWaiting).Find(&_orders)
	for _, _order := range _orders {
		// 标准化订单金额格式，确保与其他地方的Key一致
		amount, err := decimal.NewFromString(_order.Amount)
		if err != nil {
			continue // 跳过无效的金额
		}
		standardAmount := amount.StringFixed(2)
		_lock[_order.Chain+_order.Address+standardAmount] = true
	}

	var _atom = decimal.NewFromFloat(Atomicity)
	var payAmount = strconv.FormatFloat(money/rate, 'f', 2, 64)
	var _payAmount, err = decimal.NewFromString(payAmount)
	if err != nil {
		return WalletAddress{}, ""
	}

	// 设置最大尝试次数，防止无限循环
	const maxAttempts = 10000
	for attempt := 0; attempt < maxAttempts; attempt++ {
		for _, address := range wa {
			// 使用标准化的金额格式进行Key匹配
			standardPayAmount := _payAmount.StringFixed(2)
			_key := address.Chain + address.Address + standardPayAmount
			if _, ok := _lock[_key]; ok {
				continue
			}

			return address, _payAmount.String()
		}

		// 已经被占用，每次递增一个原子精度
		_payAmount = _payAmount.Add(_atom)
	}

	// 如果达到最大尝试次数，返回空值
	return WalletAddress{}, ""
}
