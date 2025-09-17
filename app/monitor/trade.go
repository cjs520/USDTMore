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
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
)

// okX的智能合约地址
const usdtToken = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

func TradeStart() {
	log.Info("交易监控启动.")

	ticker := time.NewTicker(time.Second * 15)
	defer ticker.Stop()

	for range ticker.C {
		var recentTransferTotal float64
		var _lock, err = getAllPendingOrders()
		if err != nil {
			log.Error("获取待支付订单失败: " + err.Error())
			continue
		}

		// 如果没有待支付订单，跳过API查询以节省资源
		if len(_lock) == 0 {
			log.Debug("当前无待支付订单，跳过交易监控")
			continue
		}

		log.Info(fmt.Sprintf("当前有 %d 个待支付订单，开始监控交易", len(_lock)))

		// 统计需要监控的链
		chainsToMonitor := make(map[string]bool)
		for _, order := range _lock {
			chainsToMonitor[order.Chain] = true
		}

		// 这里是TRON网络的监控
		if chainsToMonitor["TRON"] {
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
		}

		// 这里是POLYGON网络的监控
		if chainsToMonitor["POLY"] {
			// 获取有待支付订单的POLY地址
			addressesWithOrders := getAddressesWithPendingOrders(_lock, "POLY")
			log.Info(fmt.Sprintf("[POLY] 需要监控的地址数量: %d", len(addressesWithOrders)))

			for _, address := range addressesWithOrders {
				var result gjson.Result
				var err error

				result, err = getUsdtPolygonTransByPolygonScan(address)
				if err != nil {
					log.Error(fmt.Sprintf("[POLY] 查询交易失败 %s: %v", address, err))
					continue
				}

				handlePaymentTransactionForPolygonScan(_lock, address, result)
				handleOtherNotifyForPolygonScan(address, result)
			}
		}

		// 这里是OPTIMISM网络的监控
		if chainsToMonitor["OP"] {
			// 获取有待支付订单的OP地址
			addressesWithOrders := getAddressesWithPendingOrders(_lock, "OP")
			log.Info(fmt.Sprintf("[OP] 需要监控的地址数量: %d", len(addressesWithOrders)))

			for _, address := range addressesWithOrders {
				var result gjson.Result
				var err error

				result, err = getUsdtOptimismTransByOptimismExplorer(address)
				if err != nil {
					log.Error(fmt.Sprintf("[OP] 查询交易失败 %s: %v", address, err))
					continue
				}

				handlePaymentTransactionForOptimismExplorer(_lock, address, result)
				handleOtherNotifyForOptimismExplorer(address, result)
			}
		}

		// 这里是BSC网络的监控
		if chainsToMonitor["BSC"] {
			// 获取有待支付订单的BSC地址
			addressesWithOrders := getAddressesWithPendingOrders(_lock, "BSC")
			log.Info(fmt.Sprintf("[BSC] 需要监控的地址数量: %d", len(addressesWithOrders)))

			for _, address := range addressesWithOrders {
				var result gjson.Result
				var err error

				result, err = getUsdtBscTransByBscScan(address)
				if err != nil {
					log.Error(fmt.Sprintf("[BSC] 查询交易失败 %s: %v", address, err))
					continue
				}

				handlePaymentTransactionForBscScan(_lock, address, result)
				handleOtherNotifyForBscScan(address, result)
			}
		}

		// 这里是Arbitrum One网络的监控
		if chainsToMonitor["ARB"] {
			// 获取有待支付订单的ARB地址
			addressesWithOrders := getAddressesWithPendingOrders(_lock, "ARB")
			log.Info(fmt.Sprintf("[ARB] 需要监控的地址数量: %d", len(addressesWithOrders)))

			for _, address := range addressesWithOrders {
				var result gjson.Result
				var err error

				result, err = getUsdtArbitrumTransByArbitrumScan(address)
				if err != nil {
					log.Error(fmt.Sprintf("[ARB] 查询交易失败 %s: %v", address, err))
					continue
				}

				handlePaymentTransactionForArbitrumScan(_lock, address, result)
				handleOtherNotifyForArbitrumScan(address, result)
			}
		}

		// 这里是X-Layer网络的监控
		if chainsToMonitor["XLAYER"] {
			// 获取有待支付订单的XLAYER地址
			addressesWithOrders := getAddressesWithPendingOrders(_lock, "XLAYER")
			log.Info(fmt.Sprintf("[XLAYER] 需要监控的地址数量: %d", len(addressesWithOrders)))

			for _, address := range addressesWithOrders {
				var result gjson.Result
				var err error

				result, err = getUsdtXLayerTransByXLayerScan(address)
				if err != nil {
					log.Error(fmt.Sprintf("[XLAYER] 查询交易失败 %s: %v", address, err))
					continue
				}

				handlePaymentTransactionForXLayerScan(_lock, address, result)
				handleOtherNotifyForXLayerScan(address, result)
			}
		}

		// 这里是Solana网络的监控
		if chainsToMonitor["SOL"] {
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
		}

		// 这里是Aptos网络的监控
		if chainsToMonitor["APT"] {
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

/*
获取指定链上有待支付订单的地址列表
*/
func getAddressesWithPendingOrders(orders map[string]model.TradeOrders, chain string) []string {
	addressSet := make(map[string]bool)

	// 从待支付订单中提取该链的地址
	for _, order := range orders {
		if order.Chain == chain {
			addressSet[order.Address] = true
		}
	}

	// 转换为切片
	addresses := make([]string, 0, len(addressSet))
	for address := range addressSet {
		addresses = append(addresses, address)
	}

	return addresses
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

			// 重新查询订单以获取最新状态
			var updatedOrder model.TradeOrders
			if err := model.DB.Where("id = ?", _order.Id).First(&updatedOrder).Error; err != nil {
				log.Error(fmt.Sprintf("[TRON] 重新查询订单失败: order_id=%s, error=%v", _order.TradeId, err))
			} else {
				// 通知订单支付成功
				go notify.OrderNotify(updatedOrder)
				// TG发送订单信息
				go telegram.SendTradeSuccMsg(updatedOrder)
			}
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

			// 重新查询订单以获取最新状态
			var updatedOrder model.TradeOrders
			if err := model.DB.Where("id = ?", _order.Id).First(&updatedOrder).Error; err != nil {
				log.Error(fmt.Sprintf("[TRON] 重新查询订单失败: order_id=%s, error=%v", _order.TradeId, err))
			} else {
				// 通知订单支付成功
				go notify.OrderNotify(updatedOrder)
				// TG发送订单信息
				go telegram.SendTradeSuccMsg(updatedOrder)
			}
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
			// BSC不再使用通用ETH处理函数
			log.Warn("[BSC] 不应该使用通用ETH处理函数，请使用专用的BSC Web3 API")
			continue
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

		if err := _order.OrderSetSucc(_fromAddress, _transId, _createdAt); err != nil {
			log.Error(fmt.Sprintf("[%s] 订单设置成功状态失败: order_id=%s, txid=%s, error=%v",
				_toChain, _order.TradeId, _transId, err))
		} else {
			log.Info(fmt.Sprintf("[%s] 订单支付成功，发送回调: order_id=%s, txid=%s", _toChain, _order.TradeId, _transId))

			// 重新查询订单以获取最新状态
			var updatedOrder model.TradeOrders
			if err := model.DB.Where("id = ?", _order.Id).First(&updatedOrder).Error; err != nil {
				log.Error(fmt.Sprintf("[%s] 重新查询订单失败: order_id=%s, error=%v", _toChain, _order.TradeId, err))
			} else {
				// 通知订单支付成功
				go notify.OrderNotify(updatedOrder)
				// TG发送订单信息
				go telegram.SendTradeSuccMsg(updatedOrder)
			}
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
	provider := config.GetBscWeb3Provider()

	switch provider {
	case config.WEB3_PROVIDER_MORALIS:
		handlePaymentTransactionForBscMoralis(_lock, _toAddress, result)
	case config.WEB3_PROVIDER_QUICKNODE, config.WEB3_PROVIDER_ALCHEMY:
		handlePaymentTransactionForBscJsonRpc(_lock, _toAddress, result)
	default:
		log.Error(fmt.Sprintf("[BSC] 不支持的Web3提供商: %s，请设置 BSC_WEB3_PROVIDER 为 MORALIS、QUICKNODE 或 ALCHEMY", provider))
	}
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
			// BSC不再使用通用ETH处理函数
			log.Warn("[BSC] 不应该使用通用ETH处理函数，请使用专用的BSC Web3 API")
			continue
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
	// BSC通知处理根据Web3提供商类型进行
	provider := config.GetBscWeb3Provider()

	switch provider {
	case config.WEB3_PROVIDER_MORALIS:
		handleOtherNotifyForBscMoralis(_toAddress, result)
	case config.WEB3_PROVIDER_QUICKNODE, config.WEB3_PROVIDER_ALCHEMY:
		handleOtherNotifyForBscJsonRpc(_toAddress, result)
	default:
		log.Warn(fmt.Sprintf("[BSC] 不支持的Web3提供商用于通知: %s", provider))
	}
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
请求ETH兼容的链 - 使用正确的Etherscan V2 API格式
*/
func requestAddress(baseUrl string, query string) []byte {
	requestURL := baseUrl + "?" + query

	// 设置请求头，模拟浏览器请求
	headers := map[string]string{
		"User-Agent":                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 Edg/140.0.0.0",
		"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language":           "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6",
		"Cache-Control":             "max-age=0",
		"DNT":                       "1",
		"Sec-CH-UA":                 `"Chromium";v="140", "Not=A?Brand";v="24", "Microsoft Edge";v="140"`,
		"Sec-CH-UA-Mobile":          "?0",
		"Sec-CH-UA-Platform":        `"Windows"`,
		"Sec-Fetch-Dest":            "document",
		"Sec-Fetch-Mode":            "navigate",
		"Sec-Fetch-Site":            "none",
		"Sec-Fetch-User":            "?1",
		"Upgrade-Insecure-Requests": "1",
	}

	// 如果是Etherscan API，应用限流
	if strings.Contains(requestURL, "api.etherscan.io") {
		log.Debug("应用Etherscan API限流...")
		httpClient.WaitForEtherscan()
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
所有ETH兼容链路的到账监控，使用正确的专用API端点
*/
func getUsdtTransByETH(chain string, address string) (gjson.Result, error) {
	// 累计所有交易的 Value 来计算总交易量
	var wa model.WalletAddress

	// 获取API端点配置
	apiConfig := config.GetEVMChainAPIEndpoints(chain)
	if apiConfig == nil {
		return gjson.Result{}, fmt.Errorf("不支持的链类型: %s", chain)
	}

	// BSC链不使用此函数，应该使用专用的Web3 API
	if chain == "BSC" {
		return gjson.Result{}, fmt.Errorf("BSC链不支持通用ETH查询，请使用专用的BSC Web3 API")
	}

	// 获取合约地址和API Key
	var contractAddress string
	switch chain {
	case "POLY":
		contractAddress = config.GetPolygonScanContractAddress()
	case "OP":
		contractAddress = config.GetOptimismExplorerContractAddress()
	case "ARB":
		contractAddress = config.GetArbitrumContractAddress()
	case "XLAYER":
		contractAddress = config.GetXLayerContractAddress()
	default:
		return gjson.Result{}, fmt.Errorf("不支持的链类型: %s", chain)
	}

	apiKey := config.GetEtherscanApiKey()

	// 验证API Key配置
	if apiKey == "" {
		return gjson.Result{}, fmt.Errorf("[%s] ETHERSCAN_API_KEY未配置", chain)
	}

	// 简单验证API Key格式（应该是20位以上字符串）
	if len(apiKey) < 20 {
		log.Warn(fmt.Sprintf("[%s] API Key长度异常: %d位，可能无效", chain, len(apiKey)))
	}

	if model.DB.Where("chain = ? and address = ?", chain, address).First(&wa).Error == nil {
		// 使用合理的endblock值，避免超出区块范围
		endBlock := "latest"

		// 获取所有可用的API端点（主要+备用）
		endpoints := apiConfig.GetAllEndpoints()
		if len(endpoints) == 0 {
			return gjson.Result{}, fmt.Errorf("[%s] 没有可用的API端点", chain)
		}

		var resultTx gjson.Result
		var querySuccess bool = false

		// 尝试所有可用的API端点
		for i, endpoint := range endpoints {
			log.Info(fmt.Sprintf("尝试使用 %s 查询 %s 链交易 (尝试 %d/%d)",
				apiConfig.GetDisplayName(endpoint), chain, i+1, len(endpoints)))

			// 构建查询参数 - 根据API类型调整格式
			extraParams := map[string]string{
				"contractaddress": contractAddress,
				"page":            "1",
				"offset":          "100",
				"startblock":      strconv.FormatInt(wa.StartBlock+1, 10),
				"endblock":        endBlock,
				"sort":            "asc",
			}

			// Etherscan V2 API需要chainid参数，专用API不需要
			// chainid参数会在BuildQueryURL中自动添加，这里不需要手动添加

			queryTx := apiConfig.BuildQueryURL(endpoint, "account", "tokentx", address, apiKey, extraParams)

			// 记录API请求详情（隐藏API Key）
			maskedQuery := strings.Replace(queryTx, apiKey, "***", 1)
			log.Info(fmt.Sprintf("[%s] API请求: %s", chain, maskedQuery))

			// 首先尝试tokentx查询
			allTx := requestAddress(endpoint, strings.Split(queryTx, "?")[1])
			if allTx != nil {
				resultTx = gjson.ParseBytes(allTx)

				// 记录原始响应（用于调试）
				if config.IsRequestLogEnabled() {
					log.Info(fmt.Sprintf("[%s] tokentx API响应: %s", chain, string(allTx)))
				}

				// 检查API响应状态
				status := resultTx.Get("status").String()
				message := resultTx.Get("message").String()

				if status == "1" {
					querySuccess = true
					log.Info(fmt.Sprintf("[%s] tokentx查询成功，使用端点: %s", chain, apiConfig.GetDisplayName(endpoint)))
					break
				} else if strings.Contains(strings.ToLower(message), "no transactions found") {
					querySuccess = true // 无交易记录也是成功的响应
					log.Debug(fmt.Sprintf("[%s] tokentx查询成功，暂无新交易，使用端点: %s", chain, apiConfig.GetDisplayName(endpoint)))
					break
				} else {
					log.Warn(fmt.Sprintf("[%s] 端点 %s 查询失败: %s", chain, apiConfig.GetDisplayName(endpoint), message))
				}
			} else {
				log.Warn(fmt.Sprintf("[%s] 端点 %s 请求失败", chain, apiConfig.GetDisplayName(endpoint)))
			}

			// 如果tokentx查询失败，尝试txlistinternal作为备用
			if !querySuccess && i == len(endpoints)-1 {
				log.Info(fmt.Sprintf("[%s] 所有tokentx查询都失败，尝试txlistinternal备用查询", chain))

				extraParams["action"] = "txlistinternal"
				delete(extraParams, "contractaddress") // txlistinternal不需要合约地址

				backupQuery := apiConfig.BuildQueryURL(endpoint, "account", "txlistinternal", address, apiKey, extraParams)
				maskedBackupQuery := strings.Replace(backupQuery, apiKey, "***", 1)
				log.Info(fmt.Sprintf("[%s] 备用API请求: %s", chain, maskedBackupQuery))

				backupTx := requestAddress(endpoint, strings.Split(backupQuery, "?")[1])
				if backupTx != nil {
					backupResult := gjson.ParseBytes(backupTx)
					if config.IsRequestLogEnabled() {
						log.Info(fmt.Sprintf("[%s] txlistinternal API响应: %s", chain, string(backupTx)))
					}

					backupStatus := backupResult.Get("status").String()
					if backupStatus == "1" {
						resultTx = backupResult
						querySuccess = true
						log.Info(fmt.Sprintf("[%s] txlistinternal备用查询成功", chain))
					}
				}
			}
		}

		if !querySuccess {
			log.Error(fmt.Sprintf("[%s] 所有API端点都失败了，无法查询链交易", chain))
			return gjson.Result{}, fmt.Errorf("所有API端点都不可用，链路：%s，地址：%s", chain, address)
		}

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
	// 根据配置的Web3提供商选择不同的API
	provider := config.GetBscWeb3Provider()

	switch provider {
	case config.WEB3_PROVIDER_MORALIS:
		return getUsdtBscTransByMoralis(_toAddress)
	case config.WEB3_PROVIDER_QUICKNODE:
		return getUsdtBscTransByQuickNode(_toAddress)
	case config.WEB3_PROVIDER_ALCHEMY:
		return getUsdtBscTransByAlchemy(_toAddress)
	default:
		return gjson.Result{}, fmt.Errorf("[BSC] 不支持的Web3提供商: %s，请设置 BSC_WEB3_PROVIDER 为 MORALIS、QUICKNODE 或 ALCHEMY", provider)
	}
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

// BSC Moralis Web3 API集成
func getUsdtBscTransByMoralis(_toAddress string) (gjson.Result, error) {
	apiKey := config.GetMoralisApiKey()
	if apiKey == "" {
		return gjson.Result{}, fmt.Errorf("[BSC-Moralis] MORALIS_API_KEY未配置")
	}

	// 构造Moralis API请求URL - 使用正确的transfers端点
	requestURL := fmt.Sprintf("https://deep-index.moralis.io/api/v2/%s/erc20/transfers", _toAddress)

	// 设置查询参数
	params := url.Values{}
	params.Add("chain", "bsc")
	params.Add("contract_addresses", config.GetBscExplorerContractAddress()) // 修正参数名
	params.Add("limit", "50")
	params.Add("order", "DESC") // 按时间倒序，获取最新交易

	// 扩大区块监控范围或移除限制以确保能查询到交易
	if config.GetBscMonitorMode() == "RECENT" {
		// 获取当前区块高度并计算起始区块 - 扩大范围到1000个区块
		currentBlock, err := getBscCurrentBlockNumber()
		if err == nil {
			// 扩大监控范围到1000个区块，确保不遗漏交易
			blockRange := config.GetBscRecentBlockRange()
			if blockRange < 1000 {
				blockRange = 1000 // 最小1000个区块
			}
			startBlock := currentBlock - int64(blockRange)
			if startBlock > 0 {
				params.Add("from_block", strconv.FormatInt(startBlock, 10))
				log.Info(fmt.Sprintf("[BSC-Moralis] 监控区块范围: %d - %d (共%d个区块)", startBlock, currentBlock, blockRange))
			}
		}
	} else {
		// 如果不是RECENT模式，不设置from_block，查询所有历史交易
		log.Info("[BSC-Moralis] 查询所有历史交易（无区块限制）")
	}

	finalURL := requestURL + "?" + params.Encode()

	// 设置请求头
	headers := map[string]string{
		"Content-Type": "application/json",
		"X-API-Key":    apiKey,
		"User-Agent":   "USDTMore/1.0",
	}

	// 记录API请求详情（隐藏API Key）
	maskedURL := strings.Replace(finalURL, apiKey, "***", -1)
	log.Info(fmt.Sprintf("[BSC-Moralis] API请求: %s", maskedURL))

	// 使用统一的HTTP客户端发送请求，包含重试机制
	resp, err := httpClient.DefaultClient.Get(finalURL, headers, config.GetMaxRetries())
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[BSC-Moralis] API请求失败: %w", err)
	}

	// 获取响应内容
	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[BSC-Moralis] 读取响应失败: %w", err)
	}

	// 解析响应记录
	result := gjson.ParseBytes(body)

	// 检查API响应是否包含错误
	if result.Get("message").Exists() {
		return gjson.Result{}, fmt.Errorf("[BSC-Moralis] API错误: %s", result.Get("message").String())
	}

	// 记录原始响应（用于调试）
	if config.IsRequestLogEnabled() {
		log.Info(fmt.Sprintf("[BSC-Moralis] API响应: %s", string(body)))
	}

	return result, nil
}

// BSC QuickNode Web3 API集成
func getUsdtBscTransByQuickNode(_toAddress string) (gjson.Result, error) {
	endpoint := config.GetQuickNodeEndpoint()
	if endpoint == "" {
		return gjson.Result{}, fmt.Errorf("[BSC-QuickNode] QUICKNODE_ENDPOINT未配置")
	}

	apiKey := config.GetQuickNodeApiKey()
	if apiKey == "" {
		return gjson.Result{}, fmt.Errorf("[BSC-QuickNode] QUICKNODE_API_KEY未配置")
	}

	// 构造QuickNode JSON-RPC请求
	var startBlock string = "earliest"

	// 如果配置为仅监控最新区块，设置起始区块
	if config.GetBscMonitorMode() == "RECENT" {
		currentBlock, err := getBscCurrentBlockNumber()
		if err == nil {
			recentStartBlock := currentBlock - int64(config.GetBscRecentBlockRange())
			if recentStartBlock > 0 {
				startBlock = fmt.Sprintf("0x%x", recentStartBlock)
			}
		}
	}

	// 构造eth_getLogs请求参数
	requestBody := fmt.Sprintf(`{
		"jsonrpc": "2.0",
		"method": "eth_getLogs",
		"params": [{
			"address": "%s",
			"topics": ["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef", null, "0x000000000000000000000000%s"],
			"fromBlock": "%s",
			"toBlock": "latest"
		}],
		"id": 1
	}`, config.GetBscExplorerContractAddress(), strings.TrimPrefix(_toAddress, "0x"), startBlock)

	// 设置请求头
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + apiKey,
		"User-Agent":    "USDTMore/1.0",
	}

	// 记录API请求详情（隐藏API Key）
	maskedEndpoint := strings.Replace(endpoint, apiKey, "***", -1)
	log.Info(fmt.Sprintf("[BSC-QuickNode] API请求: %s", maskedEndpoint))

	// 使用统一的HTTP客户端发送POST请求
	resp, err := httpClient.DefaultClient.Post(endpoint, strings.NewReader(requestBody), headers, config.GetMaxRetries())
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[BSC-QuickNode] API请求失败: %w", err)
	}

	// 获取响应内容
	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[BSC-QuickNode] 读取响应失败: %w", err)
	}

	// 解析响应记录
	result := gjson.ParseBytes(body)

	// 检查JSON-RPC错误
	if result.Get("error").Exists() {
		return gjson.Result{}, fmt.Errorf("[BSC-QuickNode] RPC错误: %s", result.Get("error.message").String())
	}

	// 记录原始响应（用于调试）
	if config.IsRequestLogEnabled() {
		log.Info(fmt.Sprintf("[BSC-QuickNode] API响应: %s", string(body)))
	}

	return result, nil
}

// BSC Alchemy Web3 API集成
func getUsdtBscTransByAlchemy(_toAddress string) (gjson.Result, error) {
	apiKey := config.GetAlchemyApiKey()
	if apiKey == "" {
		return gjson.Result{}, fmt.Errorf("[BSC-Alchemy] ALCHEMY_API_KEY未配置")
	}

	// 构造Alchemy API请求URL
	requestURL := fmt.Sprintf("https://bnb-mainnet.g.alchemy.com/v2/%s", apiKey)

	// 构造eth_getLogs请求参数
	var startBlock string = "earliest"

	// 如果配置为仅监控最新区块，设置起始区块
	if config.GetBscMonitorMode() == "RECENT" {
		currentBlock, err := getBscCurrentBlockNumber()
		if err == nil {
			recentStartBlock := currentBlock - int64(config.GetBscRecentBlockRange())
			if recentStartBlock > 0 {
				startBlock = fmt.Sprintf("0x%x", recentStartBlock)
			}
		}
	}

	requestBody := fmt.Sprintf(`{
		"jsonrpc": "2.0",
		"method": "eth_getLogs",
		"params": [{
			"address": "%s",
			"topics": ["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef", null, "0x000000000000000000000000%s"],
			"fromBlock": "%s",
			"toBlock": "latest"
		}],
		"id": 1
	}`, config.GetBscExplorerContractAddress(), strings.TrimPrefix(_toAddress, "0x"), startBlock)

	// 设置请求头
	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "USDTMore/1.0",
	}

	// 记录API请求详情（隐藏API Key）
	maskedURL := strings.Replace(requestURL, apiKey, "***", -1)
	log.Info(fmt.Sprintf("[BSC-Alchemy] API请求: %s", maskedURL))

	// 使用统一的HTTP客户端发送POST请求
	resp, err := httpClient.DefaultClient.Post(requestURL, strings.NewReader(requestBody), headers, config.GetMaxRetries())
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[BSC-Alchemy] API请求失败: %w", err)
	}

	// 获取响应内容
	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[BSC-Alchemy] 读取响应失败: %w", err)
	}

	// 解析响应记录
	result := gjson.ParseBytes(body)

	// 检查JSON-RPC错误
	if result.Get("error").Exists() {
		return gjson.Result{}, fmt.Errorf("[BSC-Alchemy] RPC错误: %s", result.Get("error.message").String())
	}

	// 记录原始响应（用于调试）
	if config.IsRequestLogEnabled() {
		log.Info(fmt.Sprintf("[BSC-Alchemy] API响应: %s", string(body)))
	}

	return result, nil
}

// 获取BSC当前区块高度（用于最新区块监控）
func getBscCurrentBlockNumber() (int64, error) {
	// 使用公共RPC端点获取当前区块高度
	requestBody := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`

	headers := map[string]string{
		"Content-Type": "application/json",
		"User-Agent":   "USDTMore/1.0",
	}

	// 尝试多个公共RPC端点
	endpoints := []string{
		"https://bsc-dataseed1.binance.org/",
		"https://bsc-dataseed2.binance.org/",
		"https://bsc-dataseed.binance.org/",
	}

	for _, endpoint := range endpoints {
		resp, err := httpClient.DefaultClient.Post(endpoint, strings.NewReader(requestBody), headers, 1)
		if err != nil {
			continue
		}

		body, err := httpClient.GetResponseBody(resp)
		if err != nil {
			continue
		}

		result := gjson.ParseBytes(body)
		if result.Get("error").Exists() {
			continue
		}

		blockHex := result.Get("result").String()
		if blockHex != "" {
			blockNum, err := strconv.ParseInt(strings.TrimPrefix(blockHex, "0x"), 16, 64)
			if err == nil {
				return blockNum, nil
			}
		}
	}

	return 0, fmt.Errorf("无法获取BSC当前区块高度")
}

// BSC Moralis API响应处理函数
func handlePaymentTransactionForBscMoralis(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	for _, transfer := range result.Get("result").Array() {
		// 检查是否为目标地址的接收交易
		if !strings.EqualFold(transfer.Get("to_address").String(), _toAddress) {
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
		result := new(big.Float).Quo(amount, divisorFloat)

		amountFloat, _ := result.Float64()
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
		_amount := _rawAmount.StringFixed(2)

		// 检查是否已处理过此交易
		if !model.IsNeedNotifyByTxid(_txid) {
			continue
		}

		// 查找匹配的订单 - 使用标准格式：链名+地址+金额
		orderKey := "BSC" + _toAddress + _amount
		_row, exists := _lock[orderKey]
		if !exists {
			log.Info(fmt.Sprintf("[BSC-Moralis] 未找到匹配订单: key=%s, amount=%s, txid=%s", orderKey, _amount, _txid))
			continue
		}

		// 判断交易时间是否有效
		if _created.Unix() < _row.CreatedAt.Unix() || _created.Unix() > _row.ExpiredAt.Unix() {
			log.Info(fmt.Sprintf("[BSC-Moralis] 交易时间无效: txid=%s, 交易时间=%s, 订单创建时间=%s, 订单过期时间=%s",
				_txid, _created.Format(time.DateTime), _row.CreatedAt.Format(time.DateTime), _row.ExpiredAt.Format(time.DateTime)))
			continue
		}

		var _fromAddress = transfer.Get("from_address").String()

		log.Info(fmt.Sprintf("[BSC-Moralis] 处理订单支付: order_id=%s, txid=%s, from=%s, amount=%s",
			_row.TradeId, _txid, _fromAddress, _amount))

		if err := _row.OrderSetSucc(_fromAddress, _txid, _created); err != nil {
			log.Error(fmt.Sprintf("[BSC-Moralis] 订单设置成功状态失败: order_id=%s, txid=%s, error=%v",
				_row.TradeId, _txid, err))
		} else {
			log.Info(fmt.Sprintf("[BSC-Moralis] 订单支付成功，发送回调: order_id=%s, txid=%s",
				_row.TradeId, _txid))

			// 重新查询订单以获取最新状态
			var updatedOrder model.TradeOrders
			if err := model.DB.Where("id = ?", _row.Id).First(&updatedOrder).Error; err != nil {
				log.Error(fmt.Sprintf("[BSC-Moralis] 重新查询订单失败: order_id=%s, error=%v", _row.TradeId, err))
			} else {
				// 通知订单支付成功
				go notify.OrderNotify(updatedOrder)
				// TG发送订单信息
				go telegram.SendTradeSuccMsg(updatedOrder)
			}
		}
	}
}

// BSC JSON-RPC API响应处理函数 (适用于QuickNode和Alchemy)
func handlePaymentTransactionForBscJsonRpc(_lock map[string]model.TradeOrders, _toAddress string, result gjson.Result) {
	for _, logEntry := range result.Get("result").Array() {
		// 解析ERC-20 Transfer事件日志
		topics := logEntry.Get("topics").Array()
		if len(topics) < 3 {
			continue
		}

		// 验证是否为Transfer事件 (topic[0])
		if topics[0].String() != "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef" {
			continue
		}

		// 解析接收地址 (topic[2])
		toAddressHex := topics[2].String()
		toAddress := "0x" + toAddressHex[26:] // 去掉前面的0填充

		if !strings.EqualFold(toAddress, _toAddress) {
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

		// 解析区块时间戳
		blockNumberHex := logEntry.Get("blockNumber").String()
		blockNumber, err := strconv.ParseInt(strings.TrimPrefix(blockNumberHex, "0x"), 16, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("[BSC-JsonRPC] 区块号解析失败: %s", blockNumberHex))
			continue
		}

		// 获取区块时间戳（这里简化处理，使用当前时间）
		_created := time.Now()

		_txid := logEntry.Get("transactionHash").String()
		_detailUrl := "https://bscscan.com/tx/" + _txid
		_amount := _rawAmount.StringFixed(2)

		// 检查是否已处理过此交易
		if !model.IsNeedNotifyByTxid(_txid) {
			continue
		}

		// 查找匹配的订单 - 使用标准格式：链名+地址+金额
		orderKey := "BSC" + _toAddress + _amount
		if _row, exists := _lock[orderKey]; exists {
			log.Info(fmt.Sprintf("[BSC-JsonRPC] 找到匹配订单: %s, 金额: %s USDT, 区块: %d", _row.TradeId, _amount, blockNumber))

			go func(row model.TradeOrders, txid, detailUrl string, created time.Time) {
				// 更新订单状态
				model.DB.Model(&model.TradeOrders{}).Where("trade_id = ?", row.TradeId).Updates(map[string]interface{}{
					"status":      2,
					"finish_time": created,
					"txid":        txid,
				})

				// 发送成功通知
				notify.OrderNotify(row)
			}(_row, _txid, _detailUrl, _created)
		}
	}
}
