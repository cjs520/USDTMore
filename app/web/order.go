package web

import (
	"USDTMore/app/config"
	"USDTMore/app/help"
	"USDTMore/app/log"
	"USDTMore/app/model"
	"USDTMore/app/service"
	"USDTMore/app/usdt"
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

// CreateTransaction 创建订单
func CreateTransaction(ctx *gin.Context) {
	_data, _ := ctx.Get("data")
	data := _data.(map[string]any)
	_chain, ok1 := data["code"].(string)
	if !ok1 {
		_chain = "TRC20"
	}
	_orderId, ok2 := data["order_id"].(string)
	_money, ok3 := data["amount"].(float64)
	_notifyUrl, ok4 := data["notify_url"].(string)
	_redirectUrl, ok5 := data["redirect_url"].(string)
	// ---
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 {
		log.Warn("参数错误", data)
		ctx.JSON(200, RespFailJson(fmt.Errorf("参数错误")))
		return
	}

	// 获取兑换汇率并转换为decimal
	rate := usdt.GetLatestRate()
	rateDecimal, err := help.SafeDecimalFromFloat(rate)
	if err != nil {
		log.Error("汇率转换失败：", err.Error())
		ctx.JSON(200, RespFailJson(fmt.Errorf("汇率数据异常")))
		return
	}

	// 转换金额为decimal
	moneyDecimal, err := help.SafeDecimalFromFloat(_money)
	if err != nil {
		log.Error("金额转换失败：", err.Error())
		ctx.JSON(200, RespFailJson(fmt.Errorf("金额格式错误")))
		return
	}

	// 验证金额
	if err := help.ValidateMoneyAmount(moneyDecimal); err != nil {
		log.Error("金额验证失败：", err.Error())
		ctx.JSON(200, RespFailJson(fmt.Errorf("金额无效：%v", err)))
		return
	}

	// 获取钱包地址
	var wallet = model.GetAvailableAddress(_chain)
	if len(wallet) == 0 {
		log.Error("订单创建失败：还没有配置收款地址")
		ctx.JSON(200, RespFailJson(fmt.Errorf("还没有配置收款地址")))
		return
	}

	// 使用服务层计算交易金额
	reqCtx := context.WithValue(ctx.Request.Context(), "request_id", help.GenerateTradeId())
	
	// 创建服务实例
	repo := service.NewOrderRepository(model.DB)
	amountSvc := service.NewAmountService(repo)
	
	address, _amount, err := amountSvc.CalcTradeAmountWithTimeout(wallet, rate, _money, 5*time.Second)
	if err != nil {
		log.Error("计算交易金额失败：", err.Error())
		ctx.JSON(200, RespFailJson(fmt.Errorf("系统繁忙，请稍后重试")))
		return
	}

	// 解析请求地址
	var _host = "http://" + ctx.Request.Host
	if ctx.Request.TLS != nil || config.IsReWriteHttps() {
		_host = "https://" + ctx.Request.Host
	}

	// 使用服务层创建交易订单
	var _tradeId = help.GenerateTradeId()
	var _expiredAt = time.Now().Add(config.GetExpireTime() * time.Second)
	
	orderSvc := service.NewOrderService(repo, amountSvc)
	
	// 转换_amount字符串为decimal
	amountDecimal, err := help.SafeDecimalFromString(_amount)
	if err != nil {
		log.Error("交易金额转换失败：", err.Error())
		ctx.JSON(200, RespFailJson(fmt.Errorf("交易金额计算错误")))
		return
	}
	
	createReq := service.CreateOrderRequest{
		OrderID:   _orderId,
		TradeID:   _tradeId,
		Chain:     address.Chain,
		Address:   address.Address,
		Amount:    amountDecimal,
		Money:     moneyDecimal,
		UsdtRate:  rateDecimal,
		ReturnURL: _redirectUrl,
		NotifyURL: _notifyUrl,
		ExpiredAt: _expiredAt,
	}
	
	order, err := orderSvc.CreateOrder(reqCtx, createReq)
	if err != nil {
		log.Error("订单创建失败：", err.Error())
		ctx.JSON(200, RespFailJson(fmt.Errorf("订单创建失败")))
		return
	}

	// 返回响应数据
	ctx.JSON(200, RespSuccJson(gin.H{
		"trade_id":        order.TradeId,
		"order_id":        order.OrderId,
		"amount":          help.FormatMoney(order.Money), // 确保货币金额格式化为2位小数
		"actual_amount":   help.FormatCryptoFixed(order.Amount), // 确保加密货币金额格式化为2位小数（用于显示）
		"token":           order.Address,
		"expiration_time": order.ExpiredAt.Unix(),
		"payment_url":     fmt.Sprintf("%s/pay/checkout-counter/%s", config.GetAppUri(_host), order.TradeId),
	}))
	log.Info(fmt.Sprintf("订单创建成功，商户订单号：%s，交易ID：%s", order.OrderId, order.TradeId))
}
