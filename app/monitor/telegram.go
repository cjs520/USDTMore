package monitor

import (
	"USDTMore/app/log"
	"USDTMore/app/telegram"
	"context"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var err error

func BotStart(ctx context.Context, version string) {
	log.Info("Telegram Bot启动.")
	var botApi = telegram.GetBotApi()
	if botApi == nil {
		log.Error("Telegram Bot API初始化失败")
		return
	}

	_, err = botApi.MakeRequest("deleteWebhook", tgbotapi.Params{})
	if err != nil {
		log.Error("TG Bot deleteWebhook Error:", err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := botApi.GetUpdatesChan(u)
	if err != nil {
		log.Error("TG Bot GetUpdatesChan Error:", err)
		return
	}

	telegram.SendWelcome(version)

	// 监听消息
	for {
		select {
		case <-ctx.Done():
			log.Info("Telegram Bot收到关闭信号，正在退出...")
			botApi.StopReceivingUpdates()
			return
		case _u := <-updates:
			if _u.Message != nil {
				if !_u.FromChat().IsPrivate() {
					continue
				}
				telegram.HandleMessage(_u.Message)
			}
			if _u.CallbackQuery != nil {
				telegram.HandleCallback(_u.CallbackQuery)
			}
		}
	}
}
