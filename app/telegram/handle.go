package telegram

import (
	"USDTMore/app/config"
	"USDTMore/app/help"
	"USDTMore/app/model"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"strings"
)

func HandleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		botCommandHandle(msg)
		return
	}

	if msg.ReplyToMessage != nil && msg.ReplyToMessage.Text == replayAddressText {
		addWalletAddress(msg)
	}

	// 如果发送的是地址，则会查询TRON地址的金额
	if msg.Text != "" && help.IsValidTRONWalletAddress(msg.Text) {
		_addresses := strings.Split(strings.TrimSpace(msg.Text), ":")
		if len(_addresses) >= 2 {
			go queryAnyTrc20AddressInfo(msg, _addresses[1])
		}
	}

	// 如果发送的是地址，则会查询地址的金额
	if msg.Text != "" && help.IsValidPOLWalletAddress(msg.Text) {
		_addresses := strings.Split(strings.TrimSpace(msg.Text), ":")
		if len(_addresses) >= 2 {
			go queryAnyPOLAddressInfo(msg, _addresses[1])
		}
	}

	// 如果发送的是地址，则会查询地址的金额
	if msg.Text != "" && help.IsValidOPTWalletAddress(msg.Text) {
		_addresses := strings.Split(strings.TrimSpace(msg.Text), ":")
		if len(_addresses) >= 2 {
			go queryAnyOPTAddressInfo(msg, _addresses[1])
		}
	}

	// 如果发送的是地址，则会查询地址的金额
	if msg.Text != "" && help.IsValidBSCWalletAddress(msg.Text) {
		_addresses := strings.Split(strings.TrimSpace(msg.Text), ":")
		if len(_addresses) >= 2 {
			go queryAnyBSCAddressInfo(msg, _addresses[1])
		}
	}
}

func HandleCallback(query *tgbotapi.CallbackQuery) {
	if fmt.Sprintf("%v", query.From.ID) != config.GetTGBotAdminId() {

		return
	}

	var args []string
	var act = query.Data
	if strings.Contains(query.Data, "|") {
		args = strings.Split(query.Data, "|")
		act = args[0]
	}

	switch act {
	case cbWallet:
		go cbWalletAction(query, args[1])
	case cbAddressAdd:
		go cbAddressAddHandle(query)
	case cbAddress:
		go cbAddressAction(query, args[1])
	case cbAddressEnable:
		go cbAddressEnableAction(query, args[1])
	case cbAddressDisable:
		go cbAddressDisableAction(query, args[1])
	case cbAddressDelete:
		go cbAddressDeleteAction(query, args[1])
	case cbAddressOtherNotify:
		go cbAddressOtherNotifyAction(query, args[1])
	case cbOrderDetail:
		go cbOrderDetailAction(args[1])
	}
}

/*
添加钱包地址
*/
func addWalletAddress(msg *tgbotapi.Message) {
	var address = strings.TrimSpace(msg.Text)

	// 简单检测地址是否合法
	if !help.IsValidTRONWalletAddress(address) && !help.IsValidPOLWalletAddress(address) && !help.IsValidOPTWalletAddress(address) && !help.IsValidBSCWalletAddress(address) {
		SendMsg(tgbotapi.NewMessage(msg.Chat.ID, "钱包地址不合法"))
		return
	}

	_addresses := strings.Split(address, ":")
	if len(_addresses) < 2 {
		SendMsg(tgbotapi.NewMessage(msg.Chat.ID, "钱包地址格式不正确，应为: CHAIN:ADDRESS"))
		return
	}

	_exists := model.ExistsAddress(_addresses[0], _addresses[1])
	if !_exists {
		var wa = model.WalletAddress{Chain: _addresses[0], Address: _addresses[1], Status: model.StatusEnable}
		var r = model.DB.Create(&wa)
		if r.Error != nil {
			if r.Error.Error() == "UNIQUE constraint failed: wallet_address.address" {
				SendMsg(tgbotapi.NewMessage(msg.Chat.ID, "❌地址添加失败，地址重复！"))
				return
			}

			SendMsg(tgbotapi.NewMessage(msg.Chat.ID, "❌地址添加失败，错误信息："+r.Error.Error()))
			return
		}

		SendMsg(tgbotapi.NewMessage(msg.Chat.ID, "✅添加且成功启用"))
	}
	if _exists {
		SendMsg(tgbotapi.NewMessage(msg.Chat.ID, "❌地址请勿重复添加"))
	}
	cmdStartHandle()
}

func botCommandHandle(_msg *tgbotapi.Message) {
	// 命令 /id
	if _msg.Command() == cmdGetId {
		go cmdGetIdHandle(_msg)
	}

	// 非管理员退出
	if fmt.Sprintf("%v", _msg.Chat.ID) != config.GetTGBotAdminId() {
		return
	}

	// 相关指令
	switch _msg.Command() {
	case cmdStart:
		go cmdStartHandle()
	case cmdUsdt:
		go cmdUsdtHandle()
	case cmdWallet:
		go cmdWalletHandle()
	case cmdOrder:
		go cmdOrderHandle()
	}
}

func queryAnyTrc20AddressInfo(msg *tgbotapi.Message, address string) {
	var info = getWalletInfoByTRONAddress(address)
	var reply = tgbotapi.NewMessage(msg.Chat.ID, "❌查询失败")
	if info != "" {
		reply.ReplyToMessageID = msg.MessageID
		reply.Text = info
		reply.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonURL("📝查看详细信息", "https://tronscan.org/#/address/"+address),
				},
			},
		}
	}

	_, _ = botApi.Send(reply)
}

func queryAnyPOLAddressInfo(msg *tgbotapi.Message, address string) {
	var info = getWalletInfoByPOLAddress(address)
	var reply = tgbotapi.NewMessage(msg.Chat.ID, "❌查询失败")
	if info != "" {
		reply.ReplyToMessageID = msg.MessageID
		reply.Text = info
		reply.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonURL("📝查看详细信息", "https://polygonscan.com/address/"+address),
				},
			},
		}
	}

	_, _ = botApi.Send(reply)
}

func queryAnyOPTAddressInfo(msg *tgbotapi.Message, address string) {
	var info = getWalletInfoByOPTAddress(address)
	var reply = tgbotapi.NewMessage(msg.Chat.ID, "❌查询失败")
	if info != "" {
		reply.ReplyToMessageID = msg.MessageID
		reply.Text = info
		reply.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonURL("📝查看详细信息", "https://optimistic.etherscan.io/address/"+address),
				},
			},
		}
	}

	_, _ = botApi.Send(reply)
}

func queryAnyBSCAddressInfo(msg *tgbotapi.Message, address string) {
	var info = getWalletInfoByBSCAddress(address)
	var reply = tgbotapi.NewMessage(msg.Chat.ID, "❌查询失败")
	if info != "" {
		reply.ReplyToMessageID = msg.MessageID
		reply.Text = info
		reply.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonURL("📝查看详细信息", "https://bscscan.com/address/"+address),
				},
			},
		}
	}

	_, _ = botApi.Send(reply)
}
