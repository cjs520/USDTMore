package monitor

import (
	"USDTMore/app/log"
	"USDTMore/app/model"
	"USDTMore/app/notify"
	"fmt"
	"math"
	"time"
)

func NotifyStart() {
	log.Info("回调监控启动.")
	for range time.Tick(time.Second * 5) {
		tradeOrders, err := model.GetNotifyFailedTradeOrders()
		if err != nil {
			log.Error("待回调订单获取失败", err)
			continue
		}

		for _, order := range tradeOrders {
			// 设置最大重试次数，防止无限重试
			const maxRetryAttempts = 10
			if order.NotifyNum >= maxRetryAttempts {
				log.Warn(fmt.Sprintf("订单 %s 已达到最大重试次数 %d，停止重试", order.OrderId, maxRetryAttempts))
				continue
			}

			// 判断是否到达下次回调时间
			// 下次回调时间等于 3的失败次数次方 * 1分钟 + 交易确认时间
			// 限制最大延迟时间为24小时
			retryDelay := math.Min(math.Pow(3, float64(order.NotifyNum)), 24*60) // 最大24小时
			var _nextNotifyTime = order.ConfirmedAt.Add(time.Minute * time.Duration(retryDelay))
			if time.Now().Unix() >= _nextNotifyTime.Unix() {
				// 到达下次回调时间
				go notify.OrderNotify(order)
			}
		}
	}
}
