package notify

import (
	"USDTMore/app/config"
	"USDTMore/app/help"
	httpClient "USDTMore/app/http"
	"USDTMore/app/log"
	"USDTMore/app/model"
	"encoding/json"
	"fmt"
	"strings"
)

func OrderNotify(order model.TradeOrders) {
	var data = make(map[string]interface{})
	var body = struct {
		TradeId            string  `json:"trade_id"`             //  本地订单号
		OrderId            string  `json:"order_id"`             //  客户交易id
		Amount             float64 `json:"amount"`               //  订单金额 CNY
		ActualAmount       string  `json:"actual_amount"`        //  USDT 交易数额
		Token              string  `json:"token"`                //  收款钱包地址
		BlockTransactionId string  `json:"block_transaction_id"` // 区块id
		Signature          string  `json:"signature"`            // 签名
		Status             int     `json:"status"`               //  1：等待支付，2：支付成功，3：已过期
	}{
		TradeId:            order.TradeId,
		OrderId:            order.OrderId,
		Amount:             order.Money,
		ActualAmount:       order.Amount,
		Token:              order.Address,
		BlockTransactionId: order.TradeHash,
		Status:             order.Status,
	}
	var jsonBody, err = json.Marshal(body)
	if err != nil {
		log.Error("Notify Json Marshal Error：", err)

		return
	}

	if err = json.Unmarshal(jsonBody, &data); err != nil {
		log.Error("Notify JSON Unmarshal Error：", err)

		return
	}

	// 签名
	body.Signature = help.GenerateSignature(data, config.GetAuthToken())

	// 再次序列化
	jsonBody, err = json.Marshal(body)
	if err != nil {
		log.Error("Notify JSON序列化失败：", err)
		return
	}

	// 设置请求头
	headers := map[string]string{
		"Content-Type": "application/json",
		"Powered-By":   "https://ovsea.net",
		"User-Agent":   "USDTMore/1.0",
	}

	// 使用统一的HTTP客户端发送POST请求，包含重试机制
	resp, err := httpClient.DefaultClient.Post(order.NotifyUrl, strings.NewReader(string(jsonBody)), headers, config.GetMaxRetries())
	if err != nil {
		log.Error("订单回调请求失败：", err)
		order.OrderSetNotifyState(model.OrderNotifyStateFail)
		return
	}

	// 获取响应内容
	responseBody, err := httpClient.GetResponseBody(resp)
	if err != nil {
		log.Warn(fmt.Sprintf("订单回调失败(%v)：读取响应失败", order.OrderId), err)
		order.OrderSetNotifyState(model.OrderNotifyStateFail)
		return
	}

	// 检查响应内容
	responseBodyStr := string(responseBody)
	if responseBodyStr != "ok" {
		log.Warn(fmt.Sprintf("订单回调失败(%v)：响应内容不正确 (%s)", order.OrderId, responseBodyStr))
		order.OrderSetNotifyState(model.OrderNotifyStateFail)
		return
	}

	err = order.OrderSetNotifyState(model.OrderNotifyStateSucc)
	if err != nil {
		log.Error("订单标记通知成功错误：", err, order.OrderId)
	} else {
		log.Info("订单通知成功：", order.OrderId)
	}
}
