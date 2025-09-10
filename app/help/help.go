package help

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"github.com/google/uuid"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
)

// IsExist 判断文件是否存在
func IsExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {

		return true
	}

	if os.IsExist(err) {

		return true
	}

	return false
}

/*
获取环境变量
*/
func GetEnv(key string) string {
	return os.Getenv(key)
}

/*
*
生成签名 - 使用HMAC-SHA256算法
*/
func GenerateSignature(data map[string]interface{}, token string) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		if k == "signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sign strings.Builder
	for _, k := range keys {
		v := data[k]
		if v == nil || v == "" {
			continue
		}
		sign.WriteString(k)
		sign.WriteString("=")
		sign.WriteString(fmt.Sprintf("%v", v))
		sign.WriteString("&")
	}

	signString := strings.TrimRight(sign.String(), "&")
	return HmacSha256String(signString, token)
}

/*
*
生成签名 - 兼容旧版MD5签名（已弃用，仅用于向后兼容）
*/
func GenerateSignatureMD5(data map[string]interface{}, token string) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		if k == "signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sign strings.Builder
	for _, k := range keys {
		v := data[k]
		if v == nil || v == "" {
			continue
		}
		sign.WriteString(k)
		sign.WriteString("=")
		sign.WriteString(fmt.Sprintf("%v", v))
		sign.WriteString("&")
	}

	signString := strings.TrimRight(sign.String(), "&")
	return Md5String(signString + token)
}

/*
生成订单号
*/
func GenerateTradeId() string {
	return uuid.New().String()
}

/*
计算HMAC-SHA256值
*/
func HmacSha256String(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return fmt.Sprintf("%x", h.Sum(nil))
}

/*
计算MD5值（已弃用，仅用于向后兼容）
*/
func Md5String(text string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(text)))
}

/*
过滤字符串
*/
func Ec(str string) string {
	escapeChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}

	for _, char := range escapeChars {
		str = strings.ReplaceAll(str, char, "\\"+char)
	}

	return str
}

/*
是否是数字
*/
func IsNumber(s string) bool {
	match, err := regexp.MatchString(`^\d+\.?\d*$`, s)

	return match && err == nil
}

/*
是否是TRON的地址
*/
func IsValidTRONWalletAddress(address string) bool {
	match, err := regexp.MatchString(`^TRON:T[a-zA-Z0-9]{33}$`, address)
	return match && err == nil
}

/*
死否是Polygon的地址
*/
func IsValidPOLWalletAddress(address string) bool {
	match, err := regexp.MatchString(`^POLY:0x[a-zA-Z0-9]{40}$`, address)
	return match && err == nil
}

/*
是否是Optimism链的地址
*/
func IsValidOPTWalletAddress(address string) bool {
	match, err := regexp.MatchString(`^OP:0x[a-zA-Z0-9]{40}$`, address)
	return match && err == nil
}

/*
是否是BSC的地址
*/
func IsValidBSCWalletAddress(address string) bool {
	match, err := regexp.MatchString(`^BSC:0x[a-zA-Z0-9]{40}$`, address)
	return match && err == nil
}

/*
是否是Arbitrum One的地址
*/
func IsValidARBWalletAddress(address string) bool {
	match, err := regexp.MatchString(`^ARB:0x[a-zA-Z0-9]{40}$`, address)
	return match && err == nil
}

/*
是否是X-Layer的地址
*/
func IsValidXLAYERWalletAddress(address string) bool {
	match, err := regexp.MatchString(`^XLAYER:0x[a-zA-Z0-9]{40}$`, address)
	return match && err == nil
}

/*
是否是Solana的地址
*/
func IsValidSOLWalletAddress(address string) bool {
	match, err := regexp.MatchString(`^SOL:[1-9A-HJ-NP-Za-km-z]{32,44}$`, address)
	return match && err == nil
}

/*
是否是Aptos的地址
*/
func IsValidAPTWalletAddress(address string) bool {
	match, err := regexp.MatchString(`^APT:0x[a-fA-F0-9]{64}$`, address)
	return match && err == nil
}

/*
验证回调URL安全性
*/
func IsValidCallbackURL(callbackURL string) bool {
	if callbackURL == "" {
		return false
	}

	// 解析URL
	parsedURL, err := url.Parse(callbackURL)
	if err != nil {
		return false
	}

	// 只允许HTTPS协议（生产环境）或HTTP（开发环境）
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return false
	}

	// 禁止内网地址和本地地址
	host := parsedURL.Hostname()
	if host == "" {
		return false
	}

	// 禁止的主机名/IP地址模式
	forbiddenPatterns := []string{
		"127.0.0.1", "localhost", "::1",
		"10.", "172.16.", "172.17.", "172.18.", "172.19.",
		"172.20.", "172.21.", "172.22.", "172.23.", "172.24.",
		"172.25.", "172.26.", "172.27.", "172.28.", "172.29.",
		"172.30.", "172.31.", "192.168.",
	}

	for _, pattern := range forbiddenPatterns {
		if strings.HasPrefix(host, pattern) {
			return false
		}
	}

	return true
}

/*
掩码功能
*/
func MaskAddress(address string) string {
	if len(address) <= 20 {
		return address
	}
	return address[:8] + " ***** " + address[len(address)-10:]
}

/*
安全日志过滤 - 移除敏感信息
*/
func FilterSensitiveData(data map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})
	sensitiveFields := map[string]bool{
		"signature":            true,
		"token":                true,
		"auth_token":           true,
		"block_transaction_id": false, // 区块链交易哈希可以记录
		"trade_id":             false, // 订单ID可以记录
		"order_id":             false, // 客户订单ID可以记录
	}

	for k, v := range data {
		if sensitive, exists := sensitiveFields[strings.ToLower(k)]; exists && sensitive {
			filtered[k] = "***"
		} else {
			filtered[k] = v
		}
	}

	return filtered
}
