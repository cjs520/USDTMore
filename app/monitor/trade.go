package monitor

import (
	"USDTMore/app/config"
	"USDTMore/app/help"
	httpClient "USDTMore/app/http"
	"USDTMore/app/log"
	"USDTMore/app/model"
	"USDTMore/app/notify"
	"USDTMore/app/telegram"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// okX的智能合约地址
const usdtToken = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

func TradeStart() {
	log.Info("交易监控启动.")

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for range ticker.C {
		var recentTransferTotal float64
		var _lock, err = getAllPendingOrders()
		if err != nil {
			log.Error("获取待支付订单失败: " + err.Error())
			continue
		}

		// 这里是TRON网络的监控
		for _, _row := range model.GetAvailableAddress("TRON") {
			var result gjson.Result
			var err error

			if config.IsTronScanApi() {
				result, err = getUsdtTrc20TransByTronScan(_row.Address)
			} else {
				result, err = getUsdtTrc20TransByTronGrid(_row.Address)
			}
			if err != nil {
				log.Error(fmt.Sprintf("[TRON] 查询交易失败 %s: %v", _row.Address, err))
				continue
			}

			if config.IsTronScanApi() {
				recentTransferTotal = result.Get("total").Num
			} else {
				recentTransferTotal = result.Get("meta.page_size").Num
			}
			log.Info(fmt.Sprintf("[%s] recent transfer total: %s(%v)", config.GetTronServerApi(), _row.Address, recentTransferTotal))
			if recentTransferTotal <= 0 { // 没有交易记录
				continue
			}

			if config.IsTronScanApi() {
				handlePaymentTransactionForTronScan(_lock, _row.Address, result)
				handleOtherNotifyForTronScan(_row.Address, result)
			} else {
				handlePaymentTransactionForTronGrid(_lock, _row.Address, result)
				handleOtherNotifyForTronGrid(_row.Address, result)
			}
		}

		// 这里是POLYGON网络的监控
		for _, _row := range model.GetAvailableAddress("POLY") {
			var result gjson.Result
			var err error

			result, err = getUsdtPolygonTransByPolygonScan(_row.Address)
			if err != nil {
				log.Error(fmt.Sprintf("[POLY] 查询交易失败 %s: %v", _row.Address, err))
				continue
			}

			handlePaymentTransactionForPolygonScan(_lock, _row.Address, result)
			handleOtherNotifyForPolygonScan(_row.Address, result)
		}

		// 这里是OPTIMISM网络的监控
		for _, _row := range model.GetAvailableAddress("OP") {
			var result gjson.Result
			var err error

			result, err = getUsdtOptimismTransByOptimismExplorer(_row.Address)
			if err != nil {
				log.Error(fmt.Sprintf("[OP] 查询交易失败 %s: %v", _row.Address, err))
				continue
			}

			handlePaymentTransactionForOptimismExplorer(_lock, _row.Address, result)
			handleOtherNotifyForOptimismExplorer(_row.Address, result)
		}

		// 这里是BSC网络的监控
		for _, _row := range model.GetAvailableAddress("BSC") {
			var result gjson.Result
			var err error

			result, err = getUsdtBscTransByBscScan(_row.Address)
			if err != nil {
				log.Error(fmt.Sprintf("[BSC] 查询交易失败 %s: %v", _row.Address, err))
				continue
			}

			handlePaymentTransactionForBscScan(_lock, _row.Address, result)
			handleOtherNotifyForBscScan(_row.Address, result)
		}

		// 这里是Arbitrum One网络的监控
		for _, _row := range model.GetAvailableAddress("ARB") {
			var result gjson.Result
			var err error

			result, err = getUsdtArbitrumTransByArbitrumScan(_row.Address)
			if err != nil {
				log.Error(fmt.Sprintf("[ARB] 查询交易失败 %s: %v", _row.Address, err))
				continue
			}

			handlePaymentTransactionForArbitrumScan(_lock, _row.Address, result)
			handleOtherNotifyForArbitrumScan(_row.Address, result)
		}

		// 这里是X-Layer网络的监控
		for _, _row := range model.GetAvailableAddress("XLAYER") {
			var result gjson.Result
			var err error

			result, err = getUsdtXLayerTransByXLayerScan(_row.Address)
			if err != nil {
				log.Error(fmt.Sprintf("[XLAYER] 查询交易失败 %s: %v", _row.Address, err))
				continue
			}

			handlePaymentTransactionForXLayerScan(_lock, _row.Address, result)
			handleOtherNotifyForXLayerScan(_row.Address, result)
		}

		// 这里是Solana网络的监控
		for _, _row := range model.GetAvailableAddress("SOL") {
			var result gjson.Result
			var err error

			result, err = getUsdtSolanaTransBySolscan(_row.Address)
			if err != nil {
				log.Error(fmt.Sprintf("[SOL] 查询交易失败 %s: %v", _row.Address, err))
				continue
			}

			handlePaymentTransactionForSolana(_lock, _row.Address, result)
			handleOtherNotifyForSolana(_row.Address, result)
		}

		// 这里是Aptos网络的监控
		for _, _row := range model.GetAvailableAddress("APT") {
			var result gjson.Result
			var err error

			result, err = getUsdtAptosTransByAptosLabs(_row.Address)
			if err != nil {
				log.Error(fmt.Sprintf("[APT] 查询交易失败 %s: %v", _row.Address, err))
				continue
			}

			handlePaymentTransactionForAptos(_lock, _row.Address, result)
			handleOtherNotifyForAptos(_row.Address, result)
		}
	}
}

/*
列出所有等待支付的交易订单
*/
func getAllPendingOrders() (map[string]model.TradeOrders, error) {
	tradeOrders, err := model.GetTradeOrderByStatus(model.OrderStatusWaiting)
	if err != nil {
		return nil, fmt.Errorf("待支付订单获取失败: %w", err)
	}

	var _lock = make(map[string]model.TradeOrders) // 当前所有正在等待支付的订单 Lock Key
	for _, order := range tradeOrders {
		if time.Now().Unix() >= order.ExpiredAt.Unix() { // 订单过期
			err := order.OrderSetExpired()
			if err != nil {
				log.Error("订单过期标记失败：", err, order.OrderId)
			} else {
				log.Info("订单过期：", order.OrderId)
			}
			continue
		}
		// 标准化订单金额格式，确保与交易匹配时的Key一致
		amount, _ := decimal.NewFromString(order.Amount)
		standardAmount := amount.StringFixed(2)
		_lock[order.Chain+order.Address+standardAmount] = order
	}
	return _lock, nil
}

// 处理支付交易 TronScan
func handlePaymentTransactionForTronScan(_lock map[string]model.TradeOrders, _toAddress string, _data gjson.Result) {
	for _, transfer := range _data.Get("token_transfers").Array() {
		if !strings.EqualFold(transfer.Get("to_address").String(), _toAddress) {
			// 不是接收地址
			continue
		}

		// 计算交易金额
		var _rawQuant, _quant = parseTransAmount(transfer.Get("quant").Float())
		if !inPaymentAmountRange(_rawQuant) {
			continue
		}

		// 订单锁信息
		_order, ok := _lock["TRON"+_toAddress+_quant]
		if !ok || transfer.Get("contractRet").String() != "SUCCESS" {
			// 订单不存在或交易失败
			continue
		}

		// 判断时间是否有效
		var _createdAt = time.UnixMilli(transfer.Get("block_ts").Int())
		if _createdAt.Unix() < _order.CreatedAt.Unix() || _createdAt.Unix() > _order.ExpiredAt.Unix() {
			// 失效交易
			continue
		}

		var _transId = transfer.Get("transaction_id").String()
		var _fromAddress = transfer.Get("from_address").String()

		log.Info(fmt.Sprintf("[TRON] 处理订单支付: order_id=%s, txid=%s, from=%s, amount=%s",
			_order.TradeId, _transId, _fromAddress, _quant))

		if err := _order.OrderSetSucc(_fromAddress, _transId, _createdAt); err != nil {
			log.Error(fmt.Sprintf("[TRON] 订单设置成功状态失败: order_id=%s, txid=%s, error=%v",
				_order.TradeId, _transId, err))
		} else {
			log.Info(fmt.Sprintf("[TRON] 订单支付成功，发送回调: order_id=%s, txid=%s",
				_order.TradeId, _transId))
			// 通知订单支付成功
			go notify.OrderNotify(_order)
			// TG发送订单信息
			go telegram.SendTradeSuccMsg(_order)
		}
	}
}

// 处理支付交易 TronGrid
func handlePaymentTransactionForTronGrid(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	for _, transfer := range result.Get("data").Array() {
		if !strings.EqualFold(transfer.Get("to").String(), _toAddress) {
			// 不是接收地址
			continue
		}

		// 计算交易金额
		var _rawQuant, _quant = parseTransAmount(transfer.Get("value").Float())
		if !inPaymentAmountRange(_rawQuant) {
			continue
		}

		_order, ok := _lock["TRON"+_toAddress+_quant]
		if !ok || transfer.Get("type").String() != "Transfer" {
			// 订单不存在或交易失败
			continue
		}

		// 判断时间是否有效
		var _createdAt = time.UnixMilli(transfer.Get("block_timestamp").Int())
		if _createdAt.Unix() < _order.CreatedAt.Unix() || _createdAt.Unix() > _order.ExpiredAt.Unix() {
			// 失效交易
			continue
		}

		var _transId = transfer.Get("transaction_id").String()
		var _fromAddress = transfer.Get("from").String()

		log.Info(fmt.Sprintf("[TRON] 处理订单支付: order_id=%s, txid=%s, from=%s, amount=%s",
			_order.TradeId, _transId, _fromAddress, _quant))

		if err := _order.OrderSetSucc(_fromAddress, _transId, _createdAt); err != nil {
			log.Error(fmt.Sprintf("[TRON] 订单设置成功状态失败: order_id=%s, txid=%s, error=%v",
				_order.TradeId, _transId, err))
		} else {
			log.Info(fmt.Sprintf("[TRON] 订单支付成功，发送回调: order_id=%s, txid=%s",
				_order.TradeId, _transId))
			// 通知订单支付成功
			go notify.OrderNotify(_order)
			// TG发送订单信息
			go telegram.SendTradeSuccMsg(_order)
		}
	}
}

// 处理支付交易 ETH兼容网络
func handlePaymentTransactionForETH(_lock map[string]model.TradeOrders, _toChain string, _toAddress string, result gjson.Result) {
	for _, transfer := range result.Get("result").Array() {
		if !strings.EqualFold(transfer.Get("to").String(), _toAddress) {
			// 不是接收地址
			continue
		}

		tokenSymbol := transfer.Get("tokenSymbol").String()
		contractAddress := transfer.Get("contractAddress").String()

		// 根据链类型验证USDT合约地址和token symbol
		var isValidUSDT bool
		switch _toChain {
		case "POLY":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetPolygonScanContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		case "OP":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetOptimismExplorerContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		case "BSC":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetBscExplorerContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT") || tokenSymbol == "BSC-USD"
		case "ARB":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetArbitrumContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		case "XLAYER":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetXLayerContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		default:
			isValidUSDT = strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		}

		if !isValidUSDT {
			continue
		}

		// 计算交易金额
		value, ok := new(big.Int).SetString(transfer.Get("value").String(), 10)
		if !ok || value == nil {
			log.Error("Failed to parse transfer value:", transfer.Get("value").String())
			value = big.NewInt(0)
		}
		tokenDecimals, _ := strconv.ParseInt(transfer.Get("tokenDecimal").String(), 10, 32)
		decimalFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(tokenDecimals), nil)
		valueUSDT := new(big.Float).Quo(new(big.Float).SetInt(value), new(big.Float).SetInt(decimalFactor))
		bigFloatStr := valueUSDT.Text('f', -1)
		decimalUSDT, _ := decimal.NewFromString(bigFloatStr)
		// 如果不在范围直接跳出
		if !inPaymentAmountRange(decimalUSDT) {
			continue
		}

		// 使用标准化的金额格式进行订单匹配
		amountStr := decimalUSDT.StringFixed(2) // 统一使用2位小数格式
		orderKey := _toChain + _toAddress + amountStr
		_order, ok := _lock[orderKey]
		if !ok {
			// 尝试使用原始字符串格式匹配
			orderKeyAlt := _toChain + _toAddress + decimalUSDT.String()
			_order, ok = _lock[orderKeyAlt]
			if !ok {
				// 订单不存在，记录调试信息
				log.Info(fmt.Sprintf("[%s] 未找到匹配订单: key1=%s, key2=%s, amount=%s, txid=%s",
					_toChain, orderKey, orderKeyAlt, decimalUSDT.String(), transfer.Get("hash").String()))
				continue
			}
		}

		// 判断时间是否有效
		var _createdAt = time.Unix(transfer.Get("timeStamp").Int(), 0)
		if _createdAt.Unix() < _order.CreatedAt.Unix() || _createdAt.Unix() > _order.ExpiredAt.Unix() {
			// 失效交易，记录调试信息
			log.Info(fmt.Sprintf("[%s] 交易时间无效: txid=%s, 交易时间=%s, 订单创建时间=%s, 订单过期时间=%s",
				_toChain, transfer.Get("hash").String(),
				_createdAt.Format(time.DateTime),
				_order.CreatedAt.Format(time.DateTime),
				_order.ExpiredAt.Format(time.DateTime)))
			continue
		}

		var _transId = transfer.Get("hash").String()
		var _fromAddress = transfer.Get("from").String()

		log.Info(fmt.Sprintf("[%s] 处理订单支付: txid=%s, from=%s, to=%s, amount=%s",
			_toChain, _transId, _fromAddress, _toAddress, decimalUSDT.String()))

		if _order.OrderSetSucc(_fromAddress, _transId, _createdAt) == nil {
			// 通知订单支付成功
			log.Info(fmt.Sprintf("[%s] 订单支付成功，发送回调: order_id=%s, txid=%s", _toChain, _order.TradeId, _transId))
			go notify.OrderNotify(_order)
			// TG发送订单信息
			go telegram.SendTradeSuccMsg(_order)
		} else {
			log.Error("[" + _toChain + "] 订单设置成功状态失败: order_id=" + _order.TradeId + ", txid=" + _transId)
		}
	}
}

func handlePaymentTransactionForPolygonScan(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	handlePaymentTransactionForETH(_lock, "POLY", _toAddress, result)
}
func handlePaymentTransactionForOptimismExplorer(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	handlePaymentTransactionForETH(_lock, "OP", _toAddress, result)
}
func handlePaymentTransactionForBscScan(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	handlePaymentTransactionForETH(_lock, "BSC", _toAddress, result)
}

// 非订单交易通知
func handleOtherNotifyForTronScan(_toAddress string, result gjson.Result) {
	for _, transfer := range result.Get("token_transfers").Array() {
		if !model.GetOtherNotify("TRON", _toAddress) {
			break
		}

		var _rawAmount, _amount = parseTransAmount(transfer.Get("quant").Float())
		if !inPaymentAmountRange(_rawAmount) {
			continue
		}

		var _created = time.UnixMilli(transfer.Get("block_ts").Int())
		var _txid = transfer.Get("transaction_id").String()
		var _detailUrl = "https://tronscan.org/#/transaction/" + _txid
		if !model.IsNeedNotifyByTxid(_txid) {
			// 不需要额外通知
			continue
		}

		var title = "收入"
		if !strings.EqualFold(transfer.Get("to_address").String(), _toAddress) {
			title = "支出"
		}

		var text = fmt.Sprintf(
			"#账户%s #非订单交易\n---\n```\n💲交易数额：%v USDT\n⏱️交易时间：%v\n✅接收地址：%v\n🅾️发送地址：%v```\n",
			title,
			_amount,
			_created.Format(time.DateTime),
			help.MaskAddress(transfer.Get("to_address").String()),
			help.MaskAddress(transfer.Get("from_address").String()),
		)

		var chatId, err = strconv.ParseInt(config.GetTgBotNotifyTarget(), 10, 64)
		if err != nil {
			continue
		}

		var msg = tgbotapi.NewMessage(chatId, text)
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonURL("📝查看交易明细", _detailUrl),
				},
			},
		}

		var _record = model.NotifyRecord{Txid: _txid}
		model.DB.Create(&_record)

		go telegram.SendMsg(msg)
	}
}

// 非订单交易通知
func handleOtherNotifyForTronGrid(_toAddress string, result gjson.Result) {
	for _, transfer := range result.Get("data").Array() {
		if !model.GetOtherNotify("TRON", _toAddress) {
			break
		}

		var _rawQuant, _amount = parseTransAmount(transfer.Get("value").Float())
		if !inPaymentAmountRange(_rawQuant) {
			continue
		}

		var _created = time.UnixMilli(transfer.Get("block_timestamp").Int())
		var _txid = transfer.Get("transaction_id").String()
		var _detailUrl = "https://tronscan.org/#/transaction/" + _txid
		if !model.IsNeedNotifyByTxid(_txid) {
			// 不需要额外通知
			continue
		}

		var title = "收入"
		if !strings.EqualFold(transfer.Get("to").String(), _toAddress) {
			title = "支出"
		}

		var text = fmt.Sprintf(
			"#账户%s #非订单交易\n---\n```\n💲交易数额：%v USDT\n⏱️交易时间：%v\n✅接收地址：%v\n🅾️发送地址：%v```\n",
			title,
			_amount,
			_created.Format(time.DateTime),
			help.MaskAddress(transfer.Get("to").String()),
			help.MaskAddress(transfer.Get("from").String()),
		)

		var chatId, err = strconv.ParseInt(config.GetTgBotNotifyTarget(), 10, 64)
		if err != nil {
			continue
		}

		var msg = tgbotapi.NewMessage(chatId, text)
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonURL("📝查看交易明细", _detailUrl),
				},
			},
		}

		var _record = model.NotifyRecord{Txid: _txid}
		model.DB.Create(&_record)

		go telegram.SendMsg(msg)
	}
}

// 非订单交易通知
func handleOtherNotifyForETH(_toChain string, _toAddress string, result gjson.Result) {
	transfers := result.Get("result").Array()
	for _, transfer := range transfers {
		// 不需要通知
		if !model.GetOtherNotify(_toChain, _toAddress) {
			break
		}

		tokenSymbol := transfer.Get("tokenSymbol").String()
		contractAddress := transfer.Get("contractAddress").String()

		// 根据链类型验证USDT合约地址和token symbol
		var isValidUSDT bool
		switch _toChain {
		case "POLY":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetPolygonScanContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		case "OP":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetOptimismExplorerContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		case "BSC":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetBscExplorerContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT") || tokenSymbol == "BSC-USD"
		case "ARB":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetArbitrumContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		case "XLAYER":
			isValidUSDT = strings.EqualFold(contractAddress, config.GetXLayerContractAddress()) ||
				strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		default:
			isValidUSDT = strings.Contains(strings.ToUpper(tokenSymbol), "USDT")
		}

		if !isValidUSDT {
			continue
		}

		// 计算交易金额
		value, ok := new(big.Int).SetString(transfer.Get("value").String(), 10)
		if !ok || value == nil {
			log.Error("Failed to parse transfer value:", transfer.Get("value").String())
			value = big.NewInt(0)
		}
		tokenDecimals, _ := strconv.ParseInt(transfer.Get("tokenDecimal").String(), 10, 32)
		decimalFactor := new(big.Int).Exp(big.NewInt(10), big.NewInt(tokenDecimals), nil)
		valueUSDT := new(big.Float).Quo(new(big.Float).SetInt(value), new(big.Float).SetInt(decimalFactor))
		bigFloatStr := valueUSDT.Text('f', -1)
		decimalUSDT, _ := decimal.NewFromString(bigFloatStr)
		// 如果不在范围直接跳出
		if !inPaymentAmountRange(decimalUSDT) {
			continue
		}

		var _created = time.Unix(transfer.Get("timeStamp").Int(), 0)
		var _txid = transfer.Get("hash").String()
		var _detailUrl = "https://polygonscan.com/tx/" + _txid
		if _toChain == "OP" {
			_detailUrl = "https://optimistic.etherscan.io/tx/" + _txid
		}
		if _toChain == "BSC" {
			_detailUrl = "https://bscscan.com/tx/" + _txid
		}

		if !model.IsNeedNotifyByTxid(_txid) {
			// 不需要额外通知
			continue
		}

		var title = "收入"
		if !strings.EqualFold(transfer.Get("to").String(), _toAddress) {
			title = "支出"
		}

		var text = fmt.Sprintf(
			"#账户%s #非订单交易\n---\n```\n💲交易数额：%v USDT\n⏱️交易时间：%v\n✅接收地址：%v\n🅾️发送地址：%v```\n",
			title,
			decimalUSDT,
			_created.Format(time.DateTime),
			help.MaskAddress(transfer.Get("to").String()),
			help.MaskAddress(transfer.Get("from").String()),
		)

		var chatId, err = strconv.ParseInt(config.GetTgBotNotifyTarget(), 10, 64)
		if err != nil {
			continue
		}

		var msg = tgbotapi.NewMessage(chatId, text)
		msg.ParseMode = tgbotapi.ModeMarkdown
		msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{
			InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
				{
					tgbotapi.NewInlineKeyboardButtonURL("📝查看交易明细", _detailUrl),
				},
			},
		}

		var _record = model.NotifyRecord{Txid: _txid}
		model.DB.Create(&_record)

		go telegram.SendMsg(msg)
	}
}
func handleOtherNotifyForPolygonScan(_toAddress string, result gjson.Result) {
	handleOtherNotifyForETH("POLY", _toAddress, result)
}
func handleOtherNotifyForOptimismExplorer(_toAddress string, result gjson.Result) {
	handleOtherNotifyForETH("OP", _toAddress, result)
}
func handleOtherNotifyForBscScan(_toAddress string, result gjson.Result) {
	handleOtherNotifyForETH("BSC", _toAddress, result)
}

// Arbitrum One交易处理函数
func handlePaymentTransactionForArbitrumScan(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	handlePaymentTransactionForETH(_lock, "ARB", _toAddress, result)
}
func handleOtherNotifyForArbitrumScan(_toAddress string, result gjson.Result) {
	handleOtherNotifyForETH("ARB", _toAddress, result)
}

// X-Layer交易处理函数
func handlePaymentTransactionForXLayerScan(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	handlePaymentTransactionForETH(_lock, "XLAYER", _toAddress, result)
}
func handleOtherNotifyForXLayerScan(_toAddress string, result gjson.Result) {
	handleOtherNotifyForETH("XLAYER", _toAddress, result)
}

// 搜索交易记录 TronScan
func getUsdtTrc20TransByTronScan(_toAddress string) (gjson.Result, error) {
	var now = time.Now()

	// 构建请求URL
	baseURL := "https://apilist.tronscanapi.com/api/new/token_trc20/transfers"
	var params = url.Values{}
	params.Add("start", "0")
	params.Add("limit", "30")
	params.Add("contract_address", usdtToken)
	params.Add("start_timestamp", strconv.FormatInt(now.Add(-time.Hour).UnixMilli(), 10)) // 当前时间向前推 1 小时
	params.Add("end_timestamp", strconv.FormatInt(now.Add(time.Hour).UnixMilli(), 10))    // 当前时间向后推 1 小时
	params.Add("relatedAddress", _toAddress)
	if config.GetTradeConfirmed() {
		params.Add("confirm", "true")
	} else {
		params.Add("confirm", "false")
	}

	requestURL := baseURL + "?" + params.Encode()

	// 根据TRONSCAN 2025年8月公告，API Key现在是强制要求的
	apiKey := config.GetTronScanApiKey()
	if apiKey == "" {
		return gjson.Result{}, fmt.Errorf("TRON_SCAN_API_KEY是必需的，请设置环境变量")
	}

	// 设置请求头
	headers := map[string]string{
		"TRON-PRO-API-KEY": apiKey,
		"User-Agent":       "USDTMore/1.0",
	}

	// 使用统一的HTTP客户端发送请求，包含重试机制
	resp, err := httpClient.DefaultClient.Get(requestURL, headers, config.GetMaxRetries())
	if err != nil {
		return gjson.Result{}, fmt.Errorf("TronScan API请求失败: %w", err)
	}

	// 获取响应内容
	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("读取TronScan响应失败: %w", err)
	}

	// 解析响应记录
	result := gjson.ParseBytes(body)

	// 检查API响应是否包含错误
	if result.Get("success").Exists() && !result.Get("success").Bool() {
		return gjson.Result{}, fmt.Errorf("TronScan API错误: %s", result.Get("error").String())
	}

	return result, nil
}

// 搜索交易记录 TronGrid
func getUsdtTrc20TransByTronGrid(_toAddress string) (gjson.Result, error) {
	var now = time.Now()

	// 构建请求URL
	baseURL := fmt.Sprintf("https://api.trongrid.io/v1/accounts/%s/transactions/trc20", _toAddress)
	var params = url.Values{}
	params.Add("limit", "30")
	params.Add("contract_address", usdtToken)
	params.Add("min_timestamp", strconv.FormatInt(now.Add(-time.Hour).UnixMilli(), 10)) // 当前时间向前推 1 小时
	params.Add("max_timestamp", strconv.FormatInt(now.Add(time.Hour).UnixMilli(), 10))  // 当前时间向后推 1 小时
	params.Add("order_by", "block_timestamp,desc")
	if config.GetTradeConfirmed() {
		params.Add("only_confirmed", "true")
	} else {
		params.Add("only_confirmed", "false")
	}

	requestURL := baseURL + "?" + params.Encode()

	// 根据TRONSCAN 2025年8月公告，TronGrid API Key也是强制要求的
	gridApiKey := config.GetTronGridApiKey()
	if gridApiKey == "" {
		return gjson.Result{}, fmt.Errorf("TRON_GRID_API_KEY是必需的，请设置环境变量")
	}

	// 设置请求头
	headers := map[string]string{
		"TRON-PRO-API-KEY": gridApiKey,
		"User-Agent":       "USDTMore/1.0",
	}

	// 使用统一的HTTP客户端发送请求，包含重试机制
	resp, err := httpClient.DefaultClient.Get(requestURL, headers, config.GetMaxRetries())
	if err != nil {
		return gjson.Result{}, fmt.Errorf("TronGrid API请求失败: %w", err)
	}

	// 获取响应内容
	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("读取TronGrid响应失败: %w", err)
	}

	// 解析响应记录
	result := gjson.ParseBytes(body)

	// 检查API响应是否包含错误
	if result.Get("success").Exists() && !result.Get("success").Bool() {
		return gjson.Result{}, fmt.Errorf("TronGrid API错误: %s", result.Get("Error").String())
	}

	return result, nil
}

/*
请求ETH兼容的链
*/
func requestAddress(baseUrl string, query string) []byte {
	requestURL := baseUrl + "?" + query

	// 设置请求头
	headers := map[string]string{
		"User-Agent": "USDTMore/1.0",
	}

	// 使用统一的HTTP客户端发送请求，包含重试机制
	resp, err := httpClient.DefaultClient.Get(requestURL, headers, config.GetMaxRetries())
	if err != nil {
		log.Error("ETH兼容链API请求失败:", err)
		return nil
	}

	// 获取响应内容
	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		log.Error("读取ETH兼容链响应失败:", err)
		return nil
	}

	return body
}

/*
所有ETH兼容链路的到账监控，使用Etherscan V2 API避免服务中断
*/
func getUsdtTransByETH(chain string, address string) (gjson.Result, error) {
	// 累计所有交易的 Value 来计算总交易量
	var wa model.WalletAddress

	// 根据不同链类型使用对应的API端点
	var host string
	var chainId string
	var apiKey string
	var contractAddress string

	// 根据链类型设置API端点、chainid和相关配置，并强制验证API Key
	switch chain {
	case "POLY":
		host = "https://api.etherscan.io/v2/api" // Polygon使用Etherscan V2 API
		chainId = "137"                          // Polygon chainid
		apiKey = config.GetPolygonScanApiKey()
		if apiKey == "" {
			return gjson.Result{}, fmt.Errorf("POLYGON_SCAN_API_KEY是必需的，请设置环境变量")
		}
		contractAddress = config.GetPolygonScanContractAddress()
	case "OP":
		host = "https://api.etherscan.io/v2/api" // Optimism使用Etherscan V2 API
		chainId = "10"                           // Optimism chainid
		apiKey = config.GetOptimismExplorerApiKey()
		if apiKey == "" {
			return gjson.Result{}, fmt.Errorf("OPTIMISM_EXPLORER_API_KEY是必需的，请设置环境变量")
		}
		contractAddress = config.GetOptimismExplorerContractAddress()
	case "BSC":
		host = "https://api.etherscan.io/v2/api" // BSC使用Etherscan V2 Multichain API
		chainId = "56"                           // BSC chainid
		apiKey = config.GetBscExplorerApiKey()
		if apiKey == "" {
			return gjson.Result{}, fmt.Errorf("BSC_SCAN_API_KEY是必需的，请设置环境变量")
		}
		contractAddress = config.GetBscExplorerContractAddress()
	case "ARB":
		host = "https://api.etherscan.io/v2/api" // Arbitrum使用Etherscan V2 API
		chainId = "42161"                        // Arbitrum One chainid
		apiKey = config.GetArbitrumScanApiKey()
		if apiKey == "" {
			return gjson.Result{}, fmt.Errorf("ARBITRUM_SCAN_API_KEY是必需的，请设置环境变量")
		}
		contractAddress = config.GetArbitrumContractAddress()
	case "XLAYER":
		host = "https://api.etherscan.io/v2/api" // X-Layer使用Etherscan V2 API
		chainId = "196"                          // X-Layer chainid
		apiKey = config.GetXLayerApiKey()
		if apiKey == "" {
			return gjson.Result{}, fmt.Errorf("XLAYER_SCAN_API_KEY是必需的，请设置环境变量")
		}
		contractAddress = config.GetXLayerContractAddress()
	default:
		return gjson.Result{}, fmt.Errorf("不支持的链类型: %s", chain)
	}

	if model.DB.Where("chain = ? and address = ?", chain, address).First(&wa).Error == nil {
		// 统一使用Etherscan V2 API格式（所有链都需要chainid参数）
		var queryTx = "chainid=" + chainId + "&module=account&action=tokentx&contractaddress=" + contractAddress + "&address=" + address + "&page=1&offset=100&startblock=" + strconv.FormatInt(wa.StartBlock+1, 10) + "&endblock=" + strconv.FormatInt(wa.StartBlock+999999999999, 10) + "&sort=asc&apikey=" + apiKey
		allTx := requestAddress(host, queryTx)
		resultTx := gjson.ParseBytes(allTx)

		// 更新StartBlock - 处理最新的区块号，避免重复查询
		if resultTx.Get("result").IsArray() && len(resultTx.Get("result").Array()) > 0 {
			latestBlockNumber := int64(0)
			threeHoursAgo := time.Now().Add(-3 * time.Hour)

			for _, tx := range resultTx.Get("result").Array() {
				txTime := time.Unix(tx.Get("timeStamp").Int(), 0)
				blockNumber := tx.Get("blockNumber").Int()

				// 只更新3小时前的区块，确保交易已确认
				if txTime.Before(threeHoursAgo) && blockNumber > latestBlockNumber {
					latestBlockNumber = blockNumber
				}
			}

			if latestBlockNumber > wa.StartBlock {
				wa.StartBlock = latestBlockNumber
				model.DB.Save(&wa)
				log.Info(fmt.Sprintf("[%s] 更新StartBlock: address=%s, block=%d", chain, address, latestBlockNumber))
			}
		}

		return resultTx, nil
	}

	return gjson.Result{}, nil
}
func getUsdtPolygonTransByPolygonScan(_toAddress string) (gjson.Result, error) {
	return getUsdtTransByETH("POLY", _toAddress)
}
func getUsdtOptimismTransByOptimismExplorer(_toAddress string) (gjson.Result, error) {
	return getUsdtTransByETH("OP", _toAddress)
}
func getUsdtBscTransByBscScan(_toAddress string) (gjson.Result, error) {
	return getUsdtTransByETH("BSC", _toAddress)
}
func getUsdtArbitrumTransByArbitrumScan(_toAddress string) (gjson.Result, error) {
	return getUsdtTransByETH("ARB", _toAddress)
}
func getUsdtXLayerTransByXLayerScan(_toAddress string) (gjson.Result, error) {
	return getUsdtTransByETH("XLAYER", _toAddress)
}

// Solana链USDT交易查询
func getUsdtSolanaTransBySolscan(_toAddress string) (gjson.Result, error) {
	// 使用Solscan API查询SPL Token交易
	requestURL := fmt.Sprintf("https://public-api.solscan.io/account/splTransfers?account=%s&limit=50", _toAddress)

	// 设置请求头
	headers := map[string]string{
		"User-Agent": "USDTMore/1.0",
	}

	// 添加API Key（如果有）
	apiKey := config.GetSolanaApiKey()
	if apiKey != "" && apiKey != "YourSolanaApiKey" {
		headers["token"] = apiKey
	}

	// 使用统一的HTTP客户端发送请求，包含重试机制
	resp, err := httpClient.DefaultClient.Get(requestURL, headers, config.GetMaxRetries())
	if err != nil {
		return gjson.Result{}, fmt.Errorf("Solana API请求失败: %w", err)
	}

	// 获取响应内容
	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("读取Solana响应失败: %w", err)
	}

	// 解析响应记录
	result := gjson.ParseBytes(body)

	// 检查API响应是否包含错误
	if result.Get("success").Exists() && !result.Get("success").Bool() {
		return gjson.Result{}, fmt.Errorf("Solscan API错误: %s", result.Get("message").String())
	}

	return result, nil
}

// Aptos链USDT交易查询
func getUsdtAptosTransByAptosLabs(_toAddress string) (gjson.Result, error) {
	// 使用Aptos官方API查询代币交易
	requestURL := fmt.Sprintf("https://fullnode.mainnet.aptoslabs.com/v1/accounts/%s/transactions?limit=50", _toAddress)

	// 设置请求头
	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "USDTMore/1.0",
	}

	// 使用统一的HTTP客户端发送请求，包含重试机制
	resp, err := httpClient.DefaultClient.Get(requestURL, headers, config.GetMaxRetries())
	if err != nil {
		return gjson.Result{}, fmt.Errorf("Aptos API请求失败: %w", err)
	}

	// 获取响应内容
	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("读取Aptos响应失败: %w", err)
	}

	// 解析响应记录
	result := gjson.ParseBytes(body)

	// 检查API响应是否包含错误
	if result.Get("error_code").Exists() {
		return gjson.Result{}, fmt.Errorf("Aptos API错误: %s", result.Get("message").String())
	}

	return result, nil
}

// Solana交易处理函数
func handlePaymentTransactionForSolana(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	for _, transfer := range result.Get("data").Array() {
		if transfer.Get("dst").String() != _toAddress {
			continue // 不是目标地址
		}

		// 验证是否为USDT代币
		if transfer.Get("token.tokenAddress").String() != config.GetSolanaContractAddress() {
			continue
		}

		// 解析交易金额 (Solana USDT有6位小数)
		amount := transfer.Get("amount").Float()
		_rawQuant := decimal.NewFromFloat(amount / 1000000) // USDT 6位小数
		if !inPaymentAmountRange(_rawQuant) {
			continue
		}

		// 查找匹配的订单 - 使用双重格式匹配
		amountStr := _rawQuant.StringFixed(2) // 标准化格式
		orderKey := "SOL" + _toAddress + amountStr
		_order, ok := _lock[orderKey]
		if !ok {
			// 尝试原始格式
			orderKeyAlt := "SOL" + _toAddress + _rawQuant.String()
			_order, ok = _lock[orderKeyAlt]
			if !ok {
				log.Info(fmt.Sprintf("[SOL] 未找到匹配订单: key1=%s, key2=%s, amount=%s", orderKey, orderKeyAlt, _rawQuant.String()))
				continue
			}
		}

		// 验证交易时间
		blockTime := transfer.Get("blockTime").Int()
		_createdAt := time.Unix(blockTime, 0)
		if _createdAt.Unix() < _order.CreatedAt.Unix() || _createdAt.Unix() > _order.ExpiredAt.Unix() {
			continue
		}

		// 处理成功的支付
		var _transId = transfer.Get("txHash").String()
		var _fromAddress = transfer.Get("src").String()

		log.Info(fmt.Sprintf("[SOL] 处理订单支付: order_id=%s, txid=%s, from=%s, amount=%s",
			_order.TradeId, _transId, _fromAddress, _rawQuant.String()))

		if err := _order.OrderSetSucc(_fromAddress, _transId, _createdAt); err != nil {
			log.Error(fmt.Sprintf("[SOL] 订单设置成功状态失败: order_id=%s, txid=%s, error=%v",
				_order.TradeId, _transId, err))
		} else {
			log.Info(fmt.Sprintf("[SOL] 订单支付成功，发送回调: order_id=%s, txid=%s",
				_order.TradeId, _transId))
			go notify.OrderNotify(_order)
			go telegram.SendTradeSuccMsg(_order)
		}
	}
}

func handleOtherNotifyForSolana(_toAddress string, result gjson.Result) {
	for _, transfer := range result.Get("data").Array() {
		if !model.GetOtherNotify("SOL", _toAddress) {
			break
		}

		if transfer.Get("token.tokenAddress").String() != config.GetSolanaContractAddress() {
			continue
		}

		amount := transfer.Get("amount").Float()
		_rawAmount := decimal.NewFromFloat(amount / 1000000)
		if !inPaymentAmountRange(_rawAmount) {
			continue
		}

		blockTime := transfer.Get("blockTime").Int()
		_created := time.Unix(blockTime, 0)
		_txid := transfer.Get("txHash").String()
		_detailUrl := "https://solscan.io/tx/" + _txid

		if !model.IsNeedNotifyByTxid(_txid) {
			continue
		}

		title := "收入"
		if transfer.Get("dst").String() != _toAddress {
			title = "支出"
		}

		text := fmt.Sprintf(
			"#账户%s #非订单交易\n---\n```\n💲交易数额：%v USDT\n⏱️交易时间：%v\n✅接收地址：%v\n🅾️发送地址：%v```\n",
			title,
			_rawAmount,
			_created.Format(time.DateTime),
			help.MaskAddress(transfer.Get("dst").String()),
			help.MaskAddress(transfer.Get("src").String()),
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

// Aptos交易处理函数
func handlePaymentTransactionForAptos(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	for _, tx := range result.Array() {
		if tx.Get("type").String() != "user_transaction" {
			continue
		}

		// 查找USDT转账事件
		for _, event := range tx.Get("events").Array() {
			if !strings.Contains(event.Get("type").String(), "::coin::CoinStore") {
				continue
			}

			if event.Get("data.account").String() != _toAddress {
				continue // 不是目标地址
			}

			// 解析金额 (Aptos USDT通常有6位小数)
			amount := event.Get("data.amount").Float()
			_rawQuant := decimal.NewFromFloat(amount / 1000000)
			if !inPaymentAmountRange(_rawQuant) {
				continue
			}

			// 查找匹配订单 - 使用双重格式匹配
			amountStr := _rawQuant.StringFixed(2) // 标准化格式
			orderKey := "APT" + _toAddress + amountStr
			_order, ok := _lock[orderKey]
			if !ok {
				// 尝试原始格式
				orderKeyAlt := "APT" + _toAddress + _rawQuant.String()
				_order, ok = _lock[orderKeyAlt]
				if !ok {
					log.Info(fmt.Sprintf("[APT] 未找到匹配订单: key1=%s, key2=%s, amount=%s", orderKey, orderKeyAlt, _rawQuant.String()))
					continue
				}
			}

			// 验证交易时间
			timestamp := tx.Get("timestamp").Int() / 1000000 // 微秒转秒
			_createdAt := time.Unix(timestamp, 0)
			if _createdAt.Unix() < _order.CreatedAt.Unix() || _createdAt.Unix() > _order.ExpiredAt.Unix() {
				continue
			}

			// 处理成功支付
			_transId := tx.Get("hash").String()
			_fromAddress := tx.Get("sender").String()

			log.Info(fmt.Sprintf("[APT] 处理订单支付: order_id=%s, txid=%s, from=%s, amount=%s",
				_order.TradeId, _transId, _fromAddress, _rawQuant.String()))

			if err := _order.OrderSetSucc(_fromAddress, _transId, _createdAt); err != nil {
				log.Error(fmt.Sprintf("[APT] 订单设置成功状态失败: order_id=%s, txid=%s, error=%v",
					_order.TradeId, _transId, err))
			} else {
				log.Info(fmt.Sprintf("[APT] 订单支付成功，发送回调: order_id=%s, txid=%s",
					_order.TradeId, _transId))
				go notify.OrderNotify(_order)
				go telegram.SendTradeSuccMsg(_order)
			}
		}
	}
}

func handleOtherNotifyForAptos(_toAddress string, result gjson.Result) {
	for _, tx := range result.Array() {
		if !model.GetOtherNotify("APT", _toAddress) {
			break
		}

		if tx.Get("type").String() != "user_transaction" {
			continue
		}

		for _, event := range tx.Get("events").Array() {
			if !strings.Contains(event.Get("type").String(), "::coin::CoinStore") {
				continue
			}

			amount := event.Get("data.amount").Float()
			_rawAmount := decimal.NewFromFloat(amount / 1000000)
			if !inPaymentAmountRange(_rawAmount) {
				continue
			}

			timestamp := tx.Get("timestamp").Int() / 1000000
			_created := time.Unix(timestamp, 0)
			_txid := tx.Get("hash").String()
			_detailUrl := "https://explorer.aptoslabs.com/txn/" + _txid

			if !model.IsNeedNotifyByTxid(_txid) {
				continue
			}

			title := "收入"
			if event.Get("data.account").String() != _toAddress {
				title = "支出"
			}

			text := fmt.Sprintf(
				"#账户%s #非订单交易\n---\n```\n💲交易数额：%v USDT\n⏱️交易时间：%v\n✅接收地址：%v\n🅾️发送地址：%v```\n",
				title,
				_rawAmount,
				_created.Format(time.DateTime),
				help.MaskAddress(event.Get("data.account").String()),
				help.MaskAddress(tx.Get("sender").String()),
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
}

// 解析交易金额
func parseTransAmount(amount float64) (decimal.Decimal, string) {
	var _decimalAmount = decimal.NewFromFloat(amount)
	var _decimalDivisor = decimal.NewFromFloat(1000000)
	var result = _decimalAmount.Div(_decimalDivisor)

	// 返回标准化的2位小数格式，确保与订单Key匹配
	return result, result.StringFixed(2)
}
