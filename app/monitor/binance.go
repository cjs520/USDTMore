package monitor

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"

	httpClient "USDTMore/app/http"
	"USDTMore/app/model"
)

// 币安API配置
type BinanceConfig struct {
	APIKey    string
	SecretKey string
	BaseURL   string
}

// 获取币安API配置
func getBinanceConfig() (*BinanceConfig, error) {
	// 从环境变量获取币安API密钥
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")

	if apiKey == "" || secretKey == "" {
		return nil, fmt.Errorf("币安API密钥未配置，请设置BINANCE_API_KEY和BINANCE_SECRET_KEY环境变量")
	}

	return &BinanceConfig{
		APIKey:    apiKey,
		SecretKey: secretKey,
		BaseURL:   "https://api.binance.com",
	}, nil
}

// 生成币安API签名
func (bc *BinanceConfig) generateSignature(queryString string) string {
	h := hmac.New(sha256.New, []byte(bc.SecretKey))
	h.Write([]byte(queryString))
	return hex.EncodeToString(h.Sum(nil))
}

// 币安内部转账记录查询
func getUSDTTransfersByBinanceAPI(address string) (gjson.Result, error) {
	binanceConfig, err := getBinanceConfig()
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[Binance-API] %v", err)
	}

	// 构建查询参数
	params := url.Values{}
	params.Add("asset", "USDT")
	params.Add("startTime", strconv.FormatInt(time.Now().AddDate(0, 0, -30).UnixMilli(), 10)) // 最近30天
	params.Add("endTime", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Add("limit", "100")
	params.Add("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	// 生成签名
	queryString := params.Encode()
	signature := binanceConfig.generateSignature(queryString)
	params.Add("signature", signature)

	// 构建请求URL
	requestURL := fmt.Sprintf("%s/sapi/v1/capital/deposit/hisrec?%s", binanceConfig.BaseURL, params.Encode())

	// 设置请求头
	headers := map[string]string{
		"X-MBX-APIKEY": binanceConfig.APIKey,
		"Content-Type": "application/json",
	}

	log.Info(fmt.Sprintf("[Binance-API] 查询USDT充值记录: %s", address))

	// 发送请求
	resp, err := httpClient.DefaultClient.Get(requestURL, headers, 1)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[Binance-API] 请求失败: %v", err)
	}

	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[Binance-API] 读取响应失败: %v", err)
	}

	result := gjson.ParseBytes(body)

	// 检查API错误
	if result.Get("code").Exists() && result.Get("code").Int() != 200 {
		return gjson.Result{}, fmt.Errorf("[Binance-API] API错误: %s", result.Get("msg").String())
	}

	log.Info(fmt.Sprintf("[Binance-API] 返回记录数量: %d", len(result.Array())))

	return result, nil
}

// 币安内部转账记录查询（用户间转账）
func getUSDTInternalTransfersByBinanceAPI() (gjson.Result, error) {
	binanceConfig, err := getBinanceConfig()
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[Binance-Internal] %v", err)
	}

	// 构建查询参数 - 查询内部转账记录
	params := url.Values{}
	params.Add("asset", "USDT")
	params.Add("startTime", strconv.FormatInt(time.Now().AddDate(0, 0, -7).UnixMilli(), 10)) // 最近7天
	params.Add("endTime", strconv.FormatInt(time.Now().UnixMilli(), 10))
	params.Add("limit", "100")
	params.Add("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	// 生成签名
	queryString := params.Encode()
	signature := binanceConfig.generateSignature(queryString)
	params.Add("signature", signature)

	// 构建请求URL - 使用内部转账API
	requestURL := fmt.Sprintf("%s/sapi/v1/sub-account/transfer/subUserHistory?%s", binanceConfig.BaseURL, params.Encode())

	// 设置请求头
	headers := map[string]string{
		"X-MBX-APIKEY": binanceConfig.APIKey,
		"Content-Type": "application/json",
	}

	log.Info("[Binance-Internal] 查询内部转账记录")

	// 发送请求
	resp, err := httpClient.DefaultClient.Get(requestURL, headers, 1)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[Binance-Internal] 请求失败: %v", err)
	}

	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return gjson.Result{}, fmt.Errorf("[Binance-Internal] 读取响应失败: %v", err)
	}

	result := gjson.ParseBytes(body)

	// 检查API错误
	if result.Get("code").Exists() && result.Get("code").Int() != 200 {
		return gjson.Result{}, fmt.Errorf("[Binance-Internal] API错误: %s", result.Get("msg").String())
	}

	// 添加调试信息
	if result.IsArray() {
		log.Info(fmt.Sprintf("[Binance-Internal] 返回内部转账记录数量: %d", len(result.Array())))

		// 显示最新的几笔转账
		for i, transfer := range result.Array() {
			if i >= 5 { // 显示前5笔
				break
			}
			asset := transfer.Get("asset").String()
			amount := transfer.Get("qty").String()
			timestamp := transfer.Get("time").Int()
			transferTime := time.UnixMilli(timestamp).Format("2006-01-02 15:04:05")
			transferId := transfer.Get("tranId").String()

			log.Info(fmt.Sprintf("[Binance-Internal] 转账%d: 时间=%s, 资产=%s, 金额=%s, ID=%s",
				i+1, transferTime, asset, amount, transferId))
		}
	}

	return result, nil
}

// 处理币安内部转账记录
func handleBinanceInternalTransfers(_lock map[string]model.TradeOrders, targetAddress string, result gjson.Result) {
	for _, transfer := range result.Array() {
		// 检查是否为USDT转账
		if !strings.EqualFold(transfer.Get("asset").String(), "USDT") {
			continue
		}

		// 获取转账信息
		transferId := transfer.Get("tranId").String()
		amount := transfer.Get("qty").String()
		timestamp := transfer.Get("time").Int()

		if transferId == "" || amount == "" {
			continue
		}

		// 检查是否已处理过此转账
		if _, exists := _lock[transferId]; exists {
			continue
		}

		// 转换金额
		amountFloat, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			log.Warn(fmt.Sprintf("[Binance-Internal] 无法解析金额: %s", amount))
			continue
		}

		// 创建交易记录
		transferTime := time.UnixMilli(timestamp)
		log.Info(fmt.Sprintf("[Binance-Internal] 发现新的内部转账: ID=%s, 金额=%s USDT, 时间=%s",
			transferId, amount, transferTime.Format("2006-01-02 15:04:05")))

		// 这里可以添加具体的业务逻辑处理
		// 比如更新订单状态、发送通知等

		// 标记为已处理
		_lock[transferId] = model.TradeOrders{
			TradeHash: transferId,
			Amount:    fmt.Sprintf("%.8f", amountFloat),
			CreatedAt: transferTime,
		}
	}
}
