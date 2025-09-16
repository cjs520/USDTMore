package monitor

import (
	"USDTMore/app/config"
	"USDTMore/app/help"
	"USDTMore/app/log"
	"USDTMore/app/model"
	"USDTMore/app/telegram"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
)

// BSC Moralis API通知处理函数
func handleOtherNotifyForBscMoralis(_toAddress string, result gjson.Result) {
	for _, transfer := range result.Get("result").Array() {
		if !model.GetOtherNotify("BSC", _toAddress) {
			break
		}

		// 检查是否为目标地址的相关交易
		toAddr := transfer.Get("to_address").String()
		fromAddr := transfer.Get("from_address").String()
		if !strings.EqualFold(toAddr, _toAddress) && !strings.EqualFold(fromAddr, _toAddress) {
			continue
		}

		// 解析交易金额 (BSC USDT有18位小数)
		valueStr := transfer.Get("value").String()
		if valueStr == "" {
			continue
		}
		
		// 转换wei到USDT (18位小数)
		value := new(big.Int)
		value.SetString(valueStr, 10)
		
		// USDT有18位小数，所以除以10^18
		divisor := new(big.Int)
		divisor.Exp(big.NewInt(10), big.NewInt(18), nil)
		
		amount := new(big.Float).SetInt(value)
		divisorFloat := new(big.Float).SetInt(divisor)
		resultFloat := new(big.Float).Quo(amount, divisorFloat)
		
		amountFloat, _ := resultFloat.Float64()
		_rawAmount := decimal.NewFromFloat(amountFloat)

		if !inPaymentAmountRange(_rawAmount) {
			continue
		}

		// 解析交易时间
		blockTimestamp := transfer.Get("block_timestamp").String()
		_created, err := time.Parse("2006-01-02T15:04:05.000Z", blockTimestamp)
		if err != nil {
			log.Warn(fmt.Sprintf("[BSC-Moralis] 时间解析失败: %s", blockTimestamp))
			_created = time.Now()
		}

		_txid := transfer.Get("transaction_hash").String()
		_detailUrl := "https://bscscan.com/tx/" + _txid

		if !model.IsNeedNotifyByTxid(_txid) {
			continue
		}

		title := "收入"
		if !strings.EqualFold(toAddr, _toAddress) {
			title = "支出"
		}

		text := fmt.Sprintf(
			"#账户%s #非订单交易\n---\n```\n💲交易数额：%v USDT\n⏱️交易时间：%v\n✅接收地址：%v\n🅾️发送地址：%v```\n",
			title,
			_rawAmount,
			_created.Format(time.DateTime),
			help.MaskAddress(toAddr),
			help.MaskAddress(fromAddr),
		)

		chatId, err := strconv.ParseInt(config.GetTgBotNotifyTarget(), 10, 64)
		if err != nil {
			continue
		}

		msg := tgbotapi.NewMessage(chatId, text)
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{tgbotapi.NewInlineKeyboardButtonURL("📝查看交易明细", _detailUrl)},
			},
		}

		_record := model.NotifyRecord{Txid: _txid}
		model.DB.Create(&_record)
		go telegram.SendMsg(msg)
	}
}

// BSC JSON-RPC API通知处理函数 (适用于QuickNode和Alchemy)
func handleOtherNotifyForBscJsonRpc(_toAddress string, result gjson.Result) {
	for _, logEntry := range result.Get("result").Array() {
		if !model.GetOtherNotify("BSC", _toAddress) {
			break
		}

		// 解析ERC-20 Transfer事件日志
		topics := logEntry.Get("topics").Array()
		if len(topics) < 3 {
			continue
		}

		// 验证是否为Transfer事件 (topic[0])
		if topics[0].String() != "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" {
			continue
		}

		// 解析发送和接收地址
		fromAddressHex := topics[1].String()
		toAddressHex := topics[2].String()
		fromAddress := "0x" + fromAddressHex[26:] // 去掉前面的0填充
		toAddress := "0x" + toAddressHex[26:]     // 去掉前面的0填充
		
		// 检查是否与目标地址相关
		if !strings.EqualFold(toAddress, _toAddress) && !strings.EqualFold(fromAddress, _toAddress) {
			continue
		}

		// 解析交易金额 (data字段)
		dataHex := logEntry.Get("data").String()
		if dataHex == "" || dataHex == "0x" {
			continue
		}

		// 转换十六进制数据到big.Int
		value := new(big.Int)
		value.SetString(strings.TrimPrefix(dataHex, "0x"), 16)
		
		// USDT有18位小数，所以除以10^18
		divisor := new(big.Int)
		divisor.Exp(big.NewInt(10), big.NewInt(18), nil)
		
		amount := new(big.Float).SetInt(value)
		divisorFloat := new(big.Float).SetInt(divisor)
		resultFloat := new(big.Float).Quo(amount, divisorFloat)
		
		amountFloat, _ := resultFloat.Float64()
		_rawAmount := decimal.NewFromFloat(amountFloat)

		if !inPaymentAmountRange(_rawAmount) {
			continue
		}

		// 使用当前时间作为交易时间（简化处理）
		_created := time.Now()

		_txid := logEntry.Get("transactionHash").String()
		_detailUrl := "https://bscscan.com/tx/" + _txid

		if !model.IsNeedNotifyByTxid(_txid) {
			continue
		}

		title := "收入"
		if !strings.EqualFold(toAddress, _toAddress) {
			title = "支出"
		}

		text := fmt.Sprintf(
			"#账户%s #非订单交易\n---\n```\n💲交易数额：%v USDT\n⏱️交易时间：%v\n✅接收地址：%v\n🅾️发送地址：%v```\n",
			title,
			_rawAmount,
			_created.Format(time.DateTime),
			help.MaskAddress(toAddress),
			help.MaskAddress(fromAddress),
		)

		chatId, err := strconv.ParseInt(config.GetTgBotNotifyTarget(), 10, 64)
		if err != nil {
			continue
		}

		msg := tgbotapi.NewMessage(chatId, text)
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{tgbotapi.NewInlineKeyboardButtonURL("📝查看交易明细", _detailUrl)},
			},
		}

		_record := model.NotifyRecord{Txid: _txid}
		model.DB.Create(&_record)
		go telegram.SendMsg(msg)
	}
}