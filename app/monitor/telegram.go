package monitor

import (
	"USDTMore/app/log"
	"USDTMore/app/telegram"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var err error

func BotStart(version string) {
	// 先初始化Bot
	if err := telegram.InitBot(); err != nil {
		log.Error("初始化Telegram Bot失败:", err)
		return
	}

	var botApi = telegram.GetBotApi()
	if botApi == nil {
		log.Info("Telegram Bot未配置，跳过启动")
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
	for _u := range updates {
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
