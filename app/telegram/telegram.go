package telegram

import (
	"USDTMore/app/config"
	"USDTMore/app/log"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strconv"
)

var botApi *tgbotapi.BotAPI
var err error

func init() {
	var token = config.GetTGBotToken()
	if token == "" {

		return
	}

	botApi, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		panic("TG Bot NewBotAPI Error:" + err.Error())
	}

	// 注册命令
	_, err = botApi.Request(tgbotapi.NewSetMyCommands([]tgbotapi.BotCommand{
		{Command: "/" + cmdGetId, Description: "获取ID"},
		{Command: "/" + cmdStart, Description: "开始使用"},
		{Command: "/" + cmdUsdt, Description: "实时汇率"},
		{Command: "/" + cmdWallet, Description: "钱包信息"},
		{Command: "/" + cmdOrder, Description: "最近订单"},
	}...))
	if err != nil {
		panic("TG Bot Request Error:" + err.Error())
	}

	fmt.Println("Bot UserName: ", botApi.Self.UserName)
}

func GetBotApi() *tgbotapi.BotAPI {

	return botApi
}

func SendMsg(msg tgbotapi.MessageConfig) {
	if botApi == nil {
		log.Error("Bot API未初始化，无法发送消息")
		return
	}

	if msg.ChatID != 0 {
		_, err := botApi.Send(msg)
		if err != nil {
			log.Error("发送消息失败:", err)
		}
		return
	}

	var chatId, err = strconv.ParseInt(config.GetTGBotAdminId(), 10, 64)
	if err != nil {
		log.Error("解析管理员ID失败:", err)
		return
	}

	msg.ChatID = chatId
	_, err = botApi.Send(msg)
	if err != nil {
		log.Error("发送消息到管理员失败:", err)
	}
}

func DeleteMsg(msgId int) {
	if botApi == nil {
		log.Error("Bot API未初始化，无法删除消息")
		return
	}

	var chatId, err = strconv.ParseInt(config.GetTGBotAdminId(), 10, 64)
	if err != nil {
		log.Error("解析管理员ID失败:", err)
		return
	}

	_, err = botApi.Send(tgbotapi.NewDeleteMessage(chatId, msgId))
	if err != nil {
		log.Error("删除消息失败:", err)
	}
}

func EditAndSendMsg(msgId int, text string, replyMarkup tgbotapi.InlineKeyboardMarkup) {
	if botApi == nil {
		log.Error("Bot API未初始化，无法编辑消息")
		return
	}

	var chatId, err = strconv.ParseInt(config.GetTGBotAdminId(), 10, 64)
	if err != nil {
		log.Error("解析管理员ID失败:", err)
		return
	}

	_, err = botApi.Send(tgbotapi.NewEditMessageTextAndMarkup(chatId, msgId, text, replyMarkup))
	if err != nil {
		log.Error("编辑消息失败:", err)
	}
}
