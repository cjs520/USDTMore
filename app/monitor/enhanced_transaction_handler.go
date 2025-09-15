package monitor

import (
	"USDTMore/app/config"
	"USDTMore/app/log"
	"USDTMore/app/model"
	"USDTMore/app/notify"
	"USDTMore/app/telegram"
	"context"
	"fmt"
	"time"
)

// EnhancedTransactionHandler 增强的交易处理器
// 集成了Etherscan V2 Transaction API验证功能
type EnhancedTransactionHandler struct {
	verifier *TransactionVerifier
	enabled  bool
}

// NewEnhancedTransactionHandler 创建增强交易处理器
func NewEnhancedTransactionHandler() *EnhancedTransactionHandler {
	return &EnhancedTransactionHandler{
		verifier: NewTransactionVerifier(),
		enabled:  config.GetTradeConfirmed(), // 使用现有配置
	}
}

// ProcessSuccessfulTransaction 处理成功的交易
// 这是对原有OrderSetSucc逻辑的增强版本
func (eth *EnhancedTransactionHandler) ProcessSuccessfulTransaction(ctx context.Context, order model.TradeOrders, fromAddress, transId string, createdAt time.Time) error {
	// 如果启用了增强验证，先验证交易
	if eth.enabled && eth.verifier.IsEnabled() {
		log.Info(fmt.Sprintf("开始验证交易: chain=%s, txhash=%s", order.Chain, transId))
		
		verifyResult, err := eth.verifier.VerifyTransaction(ctx, order.Chain, transId)
		if err != nil {
			log.Warn(fmt.Sprintf("交易验证失败，继续使用原有逻辑: %v", err))
		} else {
			log.Info(fmt.Sprintf("交易验证完成: method=%s, success=%t, error=%s", 
				verifyResult.Method, verifyResult.IsSuccess, verifyResult.ErrorMessage))
			
			// 如果验证明确显示交易失败，记录但不阻止处理（向后兼容）
			if !verifyResult.IsSuccess {
				log.Warn(fmt.Sprintf("交易验证显示失败，但继续处理以保持兼容性: %s", verifyResult.ErrorMessage))
			}
		}
	}

	// 执行订单成功设置
	if err := order.OrderSetSuccWithContext(ctx, fromAddress, transId, createdAt); err != nil {
		return fmt.Errorf("设置订单成功状态失败: %w", err)
	}

	// 发送通知
	go notify.OrderNotify(order)
	
	// 发送Telegram通知
	go telegram.SendTradeSuccMsg(order)
	
	// 记录成功日志
	log.Info(fmt.Sprintf("[%s] 订单处理成功: order_id=%s, trade_id=%s, txid=%s, amount=%s", 
		order.Chain, order.OrderId, order.TradeId, transId, order.Amount.String()))

	return nil
}

// ProcessTransactionBatch 批量处理交易
// 用于需要批量验证多个交易的场景
func (eth *EnhancedTransactionHandler) ProcessTransactionBatch(ctx context.Context, transactions []struct {
	Order       model.TradeOrders
	FromAddress string
	TransId     string
	CreatedAt   time.Time
}) error {
	if len(transactions) == 0 {
		return nil
	}

	// 如果启用了验证，先批量验证所有交易
	if eth.enabled && eth.verifier.IsEnabled() {
		txsToVerify := make([]struct {
			Chain  string
			TxHash string
		}, len(transactions))
		
		for i, tx := range transactions {
			txsToVerify[i] = struct {
				Chain  string
				TxHash string
			}{
				Chain:  tx.Order.Chain,
				TxHash: tx.TransId,
			}
		}
		
		verifyResults, err := eth.verifier.VerifyTransactionBatch(ctx, txsToVerify)
		if err != nil {
			log.Warn(fmt.Sprintf("批量交易验证失败: %v", err))
		} else {
for _, result := range verifyResults {
				if !result.IsSuccess {
					log.Warn(fmt.Sprintf("交易验证显示失败: chain=%s, txhash=%s, error=%s", 
						result.Chain, result.TxHash, result.ErrorMessage))
				}
			}
		}
	}

	// 处理每个交易
	successCount := 0
	for _, tx := range transactions {
		if err := eth.ProcessSuccessfulTransaction(ctx, tx.Order, tx.FromAddress, tx.TransId, tx.CreatedAt); err != nil {
			log.Error(fmt.Sprintf("处理交易失败: order_id=%s, error=%v", tx.Order.OrderId, err))
		} else {
			successCount++
		}
	}

	log.Info(fmt.Sprintf("批量处理完成: 总计=%d, 成功=%d", len(transactions), successCount))
	return nil
}

// GetVerificationStats 获取验证统计信息
func (eth *EnhancedTransactionHandler) GetVerificationStats() map[string]interface{} {
	return map[string]interface{}{
		"enhanced_handler_enabled": eth.enabled,
		"verifier_enabled":         eth.verifier.IsEnabled(),
		"api_key_configured":       config.GetEtherscanApiKey() != "",
	}
}

// 使用示例函数 - 展示如何在现有代码中集成
func ExampleUsage() {
	// 在trade.go中的使用示例：
	// handler := NewEnhancedTransactionHandler()
	// 
	// // 替换原来的直接调用：
	// // if _order.OrderSetSucc(_fromAddress, _transId, _createdAt) == nil {
	// //     go notify.OrderNotify(_order)
	// //     go telegram.SendTradeSuccMsg(_order)
	// // }
	//
	// // 改为使用增强处理器：
	// ctx := context.Background()
	// if err := handler.ProcessSuccessfulTransaction(ctx, _order, _fromAddress, _transId, _createdAt); err != nil {
	//     log.Error(fmt.Sprintf("订单处理失败: %v", err))
	// }
}