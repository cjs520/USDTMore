package monitor

import (
	"USDTMore/app/config"
	httpClient "USDTMore/app/http"
	"USDTMore/app/log"
	"USDTMore/app/usdt"
	"errors"
	"fmt"
	"github.com/shopspring/decimal"
	"github.com/tidwall/gjson"
	"strconv"
	"time"
)

// OkxUsdtRateStart Okx USDT 汇率监控，避免频繁请求
func OkxUsdtRateStart() {
	var _act, _value, _defaultRate = config.GetUsdtRate()
	for {
		if _act == "" {
			usdt.SetLatestRate(_defaultRate)

			log.Info("固定汇率", usdt.GetLatestRate())
		} else {
			_okxRate, _okxErr := getOkxUsdtCnySellPrice()
			if _okxErr == nil { // 获取成功
				usdt.SetOkxLatestRate(_okxRate.InexactFloat64())

				switch _act {
				case "~":
					usdt.SetLatestRate(_okxRate.Mul(_value).InexactFloat64())
				case "+":
					usdt.SetLatestRate(_okxRate.Add(_value).InexactFloat64())
				case "-":
					usdt.SetLatestRate(_okxRate.Sub(_value).InexactFloat64())
				default:
					usdt.SetLatestRate(_okxRate.InexactFloat64())
				}

				log.Info(fmt.Sprintf("okx rate: %v act(%v) value(%v) 最终实际汇率：%v", _okxRate, _act, _value, usdt.GetLatestRate()))
			}
		}

		time.Sleep(time.Minute)
	}
}

// getOkxUsdtCnySellPrice  Okx  C2C快捷交易 USDT出售 实时汇率
func getOkxUsdtCnySellPrice() (decimal.Decimal, error) {
	var _zero = decimal.NewFromInt(0)
	var t = strconv.Itoa(int(time.Now().Unix()))
	var requestURL = "https://www.okx.com/v4/c2c/express/price?crypto=USDT&fiat=CNY&side=sell&t=" + t

	// 设置请求头，模拟真实浏览器
	headers := map[string]string{
		"User-Agent":      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/117.0.0.0 Safari/537.36",
		"Accept":          "application/json, text/plain, */*",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Referer":         "https://www.okx.com/",
	}

	// 使用统一的HTTP客户端发送请求，包含重试机制
	resp, err := httpClient.DefaultClient.Get(requestURL, headers, config.GetMaxRetries())
	if err != nil {
		return _zero, fmt.Errorf("OKX API请求失败: %w", err)
	}

	// 获取响应内容
	body, err := httpClient.GetResponseBody(resp)
	if err != nil {
		return _zero, fmt.Errorf("读取OKX响应失败: %w", err)
	}

	result := gjson.ParseBytes(body)

	// 检查API响应错误
	if result.Get("error_code").Int() != 0 {
		return _zero, fmt.Errorf("OKX API错误: %s", result.Get("error_message").String())
	}

	// 检查价格数据
	if result.Get("data.price").Exists() {
		var _ret = result.Get("data.price").Float()
		if _ret <= 0 {
			return _zero, errors.New("OKX返回的价格数据无效: price <= 0")
		}

		return decimal.NewFromFloat(_ret), nil
	}

	return _zero, errors.New("OKX响应中未找到价格数据")
}
