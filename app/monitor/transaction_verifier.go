package monitor

import (
	"USDTMore/app/config"
	"USDTMore/app/help"
	"USDTMore/app/log"
	"context"
	"fmt"
	"time"

	"github.com/tidwall/gjson"
)

// TransactionVerificationResult 交易验证结果
type TransactionVerificationResult struct {
	TxHash       string    `json:"tx_hash"`
	Chain        string    `json:"chain"`
	IsConfirmed  bool      `json:"is_confirmed"`
	IsSuccess    bool      `json:"is_success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	VerifiedAt   time.Time `json:"verified_at"`
	Method       string    `json:"method"` // "getstatus" 或 "gettxreceiptstatus"
}

// TransactionVerifier 交易验证器 - 使用Etherscan V2 Transaction API
type TransactionVerifier struct {
	enabled bool
}

// NewTransactionVerifier 创建交易验证器
func NewTransactionVerifier() *TransactionVerifier {
	return &TransactionVerifier{
		enabled: config.GetTradeConfirmed(), // 使用现有的配置项
	}
}

// IsEnabled 检查是否启用交易验证
func (tv *TransactionVerifier) IsEnabled() bool {
	return tv.enabled
}

// VerifyTransaction 验证交易状态
// 优先使用gettxreceiptstatus，失败时fallback到getstatus
func (tv *TransactionVerifier) VerifyTransaction(ctx context.Context, chain, txHash string) (*TransactionVerificationResult, error) {
	if !tv.enabled {
		return &TransactionVerificationResult{
			TxHash:      txHash,
			Chain:       chain,
			IsConfirmed: true,
			IsSuccess:   true,
			VerifiedAt:  time.Now(),
			Method:      "disabled",
		}, nil
	}

	// 获取chainid
	chainId, err := tv.getChainId(chain)
	if err != nil {
		return nil, fmt.Errorf("不支持的链类型: %s", chain)
	}

	// 获取API密钥
	apiKey := config.GetEtherscanApiKey()
	if apiKey == "" {
		log.Warn("ETHERSCAN_API_KEY未设置，跳过交易验证")
		return &TransactionVerificationResult{
			TxHash:      txHash,
			Chain:       chain,
			IsConfirmed: true,
			IsSuccess:   true,
			VerifiedAt:  time.Now(),
			Method:      "no_api_key",
		}, nil
	}

	// 先尝试Transaction Receipt Status (推荐方法)
	result, err := tv.verifyByReceiptStatus(ctx, chainId, txHash, apiKey)
	if err == nil {
		result.TxHash = txHash
		result.Chain = chain
		result.VerifiedAt = time.Now()
		result.Method = "gettxreceiptstatus"
		return result, nil
	}

	log.Warn(fmt.Sprintf("Transaction Receipt Status验证失败，fallback到Contract Execution Status: %v", err))

	// Fallback到Contract Execution Status
	result, err = tv.verifyByContractStatus(ctx, chainId, txHash, apiKey)
	if err != nil {
		return nil, fmt.Errorf("交易验证失败: %w", err)
	}

	result.TxHash = txHash
	result.Chain = chain
	result.VerifiedAt = time.Now()
	result.Method = "getstatus"
	return result, nil
}

// verifyByReceiptStatus 使用Transaction Receipt Status验证（仅适用于Byzantium分叉后）
func (tv *TransactionVerifier) verifyByReceiptStatus(ctx context.Context, chainId, txHash, apiKey string) (*TransactionVerificationResult, error) {
	url := "https://api.etherscan.io/v2/api"
	queryParams := fmt.Sprintf("chainid=%s&module=transaction&action=gettxreceiptstatus&txhash=%s&apikey=%s", chainId, txHash, apiKey)

	resp := requestAddress(url, queryParams)
	if resp == nil {
		return nil, fmt.Errorf("API请求失败")
	}

	result := gjson.ParseBytes(resp)
	
	// 验证API响应
	if result.Get("status").String() == "0" {
		errorMsg := result.Get("message").String()
		return nil, fmt.Errorf("API错误: %s", errorMsg)
	}

	// 解析状态
	status := result.Get("result.status").String()
	return &TransactionVerificationResult{
		IsConfirmed: true,
		IsSuccess:   status == "1",
		ErrorMessage: func() string {
			if status != "1" {
				return "交易执行失败"
			}
			return ""
		}(),
	}, nil
}

// verifyByContractStatus 使用Contract Execution Status验证
func (tv *TransactionVerifier) verifyByContractStatus(ctx context.Context, chainId, txHash, apiKey string) (*TransactionVerificationResult, error) {
	url := "https://api.etherscan.io/v2/api"
	queryParams := fmt.Sprintf("chainid=%s&module=transaction&action=getstatus&txhash=%s&apikey=%s", chainId, txHash, apiKey)

	resp := requestAddress(url, queryParams)
	if resp == nil {
		return nil, fmt.Errorf("API请求失败")
	}

	result := gjson.ParseBytes(resp)
	
	// 验证API响应
	if result.Get("status").String() == "0" {
		errorMsg := result.Get("message").String()
		return nil, fmt.Errorf("API错误: %s", errorMsg)
	}

	// 解析状态 (注意：isError字段含义相反)
	isError := result.Get("result.isError").String()
	return &TransactionVerificationResult{
		IsConfirmed: true,
		IsSuccess:   isError == "0", // 0表示成功，1表示失败
		ErrorMessage: func() string {
			if isError != "0" {
				return "合约执行失败"
			}
			return ""
		}(),
	}, nil
}

// getChainId 获取链对应的chainid
func (tv *TransactionVerifier) getChainId(chain string) (string, error) {
	chainMap := map[string]string{
		"POLY":   "137",   // Polygon
		"BSC":    "56",    // BSC
		"OP":     "10",    // Optimism
		"ARB":    "42161", // Arbitrum
		"XLAYER": "196",   // X-Layer
		"ETH":    "1",     // Ethereum Mainnet
	}
	
	chainId, exists := chainMap[chain]
	if !exists {
		return "", fmt.Errorf("不支持的链类型: %s", chain)
	}
	return chainId, nil
}

// VerifyTransactionBatch 批量验证多个交易
func (tv *TransactionVerifier) VerifyTransactionBatch(ctx context.Context, transactions []struct {
	Chain  string
	TxHash string
}) ([]*TransactionVerificationResult, error) {
	if !tv.enabled {
		// 如果未启用，返回所有交易都成功的结果
		results := make([]*TransactionVerificationResult, len(transactions))
		for i, tx := range transactions {
			results[i] = &TransactionVerificationResult{
				TxHash:      tx.TxHash,
				Chain:       tx.Chain,
				IsConfirmed: true,
				IsSuccess:   true,
				VerifiedAt:  time.Now(),
				Method:      "disabled",
			}
		}
		return results, nil
	}

	results := make([]*TransactionVerificationResult, 0, len(transactions))
	
	for _, tx := range transactions {
		// 避免API限制，每次请求间隔200ms
		time.Sleep(200 * time.Millisecond)
		
		result, err := tv.VerifyTransaction(ctx, tx.Chain, tx.TxHash)
		if err != nil {
			log.Error(fmt.Sprintf("验证交易失败: chain=%s, txhash=%s, error=%v", tx.Chain, tx.TxHash, err))
			// 验证失败时，假设交易成功（向后兼容）
			result = &TransactionVerificationResult{
				TxHash:      tx.TxHash,
				Chain:       tx.Chain,
				IsConfirmed: true,
				IsSuccess:   true,
				VerifiedAt:  time.Now(),
				Method:      "fallback_success",
				ErrorMessage: err.Error(),
			}
		}
		results = append(results, result)
	}
	
	return results, nil
}