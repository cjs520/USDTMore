package config

import (
	"USDTMore/app/help"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const defaultExpireTime = 600         // 订单默认有效期 10分钟
const defaultUsdtRate = 7.4           // 默认汇率
const defaultAuthToken = "123234"     // 默认授权码
const defaultListen = ":6080"         // 默认监听地址
const TronServerApiScan = "TRON_SCAN" //
const TronServerApiGrid = "TRON_GRID" //
const defaultPaymentMinAmount = 0.01  //
const defaultPaymentMaxAmount = 99999 //

// 网络请求配置常量
const defaultHttpTimeout = 30 // HTTP请求默认超时时间（秒）
const defaultMaxRetries = 3   // 默认最大重试次数
const defaultRetryDelay = 1   // 默认重试延迟（秒）

// 当前路径
var runPath string

func init() {
	// 获取应用程序当前的路径
	execPath, err := os.Executable()
	if err != nil {
		panic(err)
	}
	runPath = filepath.Dir(execPath)
}

/*
获取最小支付金额
*/
func GetPaymentMinAmount() decimal.Decimal {
	var _default = decimal.NewFromFloat(defaultPaymentMinAmount)
	// 取得配置数据
	var _min, _ = getPaymentRangeAmount()
	if _min == "" {
		return _default
	}

	// 使用配置数据返回
	_result, err := decimal.NewFromString(_min)
	if err == nil {
		return _result
	}

	// 默认返回数据
	return _default
}

/*
获取最大支付金额
*/
func GetPaymentMaxAmount() decimal.Decimal {
	var _default = decimal.NewFromFloat(defaultPaymentMaxAmount)
	// 取得配置数据
	var _, _max = getPaymentRangeAmount()
	if _max == "" {
		return _default
	}

	// 使用配置数据返回
	_result, err := decimal.NewFromString(_max)
	if err == nil {

		return _result
	}

	// 默认返回数据
	return _default
}

/*
读取支付金额范围
*/
func getPaymentRangeAmount() (string, string) {
	var _rangeVar string
	if _rangeVar = strings.TrimSpace(help.GetEnv("PAYMENT_AMOUNT_RANGE")); _rangeVar == "" {
		return "", ""
	}

	var _payRange = strings.Split(_rangeVar, ",")
	if len(_payRange) < 2 {
		return "", ""
	}

	return _payRange[0], _payRange[1]
}

/*
获得过期时间
*/
func GetExpireTime() time.Duration {
	if ret := help.GetEnv("EXPIRE_TIME"); ret != "" {
		sec, err := strconv.Atoi(ret)
		if err == nil && sec > 0 {
			return time.Duration(sec)
		}
	}

	return defaultExpireTime
}

/*
获取固定汇率设置
*/
func GetUsdtRateRaw() string {
	if data := help.GetEnv("USDT_RATE"); data != "" {
		return strings.TrimSpace(data)
	}
	return ""
}

/*
获得TRON服务类型，可选`TRON_SCAN`,`TRON_GRID`
只适用于TRON链路
*/
func GetTronServerApi() string {
	if data := help.GetEnv("TRON_SERVER_API"); data != "" {
		return strings.TrimSpace(data)
	}

	return ""
}

/*
获得TronScan接口的API密钥
*/
func GetTronScanApiKey() string {
	if data := help.GetEnv("TRON_SCAN_API_KEY"); data != "" {
		return strings.TrimSpace(data)
	}
	// 移除硬编码的默认API密钥，强制用户设置
	return ""
}

/*
获得TronGrid接口的API密钥
*/
func GetTronGridApiKey() string {
	if data := help.GetEnv("TRON_GRID_API_KEY"); data != "" {
		return strings.TrimSpace(data)
	}
	return ""
}

/*
判断是否使用了Tron Scan接口
*/
func IsTronScanApi() bool {
	if GetTronServerApi() == TronServerApiScan {
		return true
	}

	return GetTronServerApi() != TronServerApiGrid
}

/*
获得Etherscan V2 API密钥（EVM兼容链统一使用）
支持的链：Polygon, Optimism, BSC, Arbitrum, X-Layer
*/
func GetEtherscanApiKey() string {
	// 优先使用统一的ETHERSCAN_API_KEY
	if data := help.GetEnv("ETHERSCAN_API_KEY"); data != "" {
		return strings.TrimSpace(data)
	}

	// 向后兼容：依次尝试旧的各链专用API Key
	keys := []string{
		"POLYGON_SCAN_API_KEY",
		"OPTIMISM_EXPLORER_API_KEY",
		"BSC_SCAN_API_KEY",
		"ARBITRUM_SCAN_API_KEY",
		"XLAYER_SCAN_API_KEY",
	}

	for _, key := range keys {
		if data := help.GetEnv(key); data != "" {
			return strings.TrimSpace(data)
		}
	}

	return ""
}

/*
获得Polygon接口的API密钥（已弃用，建议使用GetEtherscanApiKey）
*/
func GetPolygonScanApiKey() string {
	return GetEtherscanApiKey()
}

/*
获得OptimismExplorer接口的API密钥（已弃用，建议使用GetEtherscanApiKey）
*/
func GetOptimismExplorerApiKey() string {
	return GetEtherscanApiKey()
}

/*
获得BSC接口的API密钥（已弃用，建议使用GetEtherscanApiKey）
*/
func GetBscExplorerApiKey() string {
	return GetEtherscanApiKey()
}

/*
获得Arbitrum Scan接口的API密钥（已弃用，建议使用GetEtherscanApiKey）
*/
func GetArbitrumScanApiKey() string {
	return GetEtherscanApiKey()
}

/*
获得X-Layer接口的API密钥（已弃用，建议使用GetEtherscanApiKey）
*/
func GetXLayerApiKey() string {
	return GetEtherscanApiKey()
}

/*
获得Solana接口的API密钥
*/
func GetSolanaApiKey() string {
	if data := help.GetEnv("SOLANA_API_KEY"); data != "" {
		return strings.TrimSpace(data)
	}
	// Solscan API可以不需要密钥，但建议设置以提高限流
	return ""
}

/*
获得Aptos接口的API密钥
*/
func GetAptosApiKey() string {
	if data := help.GetEnv("APTOS_API_KEY"); data != "" {
		return strings.TrimSpace(data)
	}
	// Aptos官方API是公开的，不需要API密钥
	return ""
}

// ERC-20 合约地址 (Polygon 主网上的 USDT)
const tokenPolygonContractAddress = "0xc2132D05D31c914a87C6611C10748AEb04B58e8F"

// ERC-20 合约地址 (Optimism 主网上的 USDT)
const tokenOptimismContractAddress = "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58"

// ERC-20 合约地址 (Bsc 主网上的 USDT)
const tokenBscContractAddress = "0x55d398326f99059fF775485246999027B3197955"

// ERC-20 合约地址 (Arbitrum One 主网上的 USDT)
const tokenArbitrumContractAddress = "0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9"

// ERC-20 合约地址 (X-Layer 主网上的 USDT)
const tokenXLayerContractAddress = "0x1e4a5963abfd975d8c9021ce480b42188849d41d"

// SPL Token 合约地址 (Solana 主网上的 USDT)
const tokenSolanaContractAddress = "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"

// Aptos 主网上的 USDT 合约地址
const tokenAptosContractAddress = "0xf22bede237a07e121b56d91a491eb7bcdfd1f5907926a9e58338f964a01b17fa::asset::USDT"

/*
获得PolygonScan接口的API密钥
*/
func GetPolygonScanContractAddress() string {
	return tokenPolygonContractAddress
}

/*
获得OptimismExplorer接口的API密钥
*/
func GetOptimismExplorerContractAddress() string {
	return tokenOptimismContractAddress
}

/*
获得BSC合约地址
*/
func GetBscExplorerContractAddress() string {
	return tokenBscContractAddress
}

/*
获得Arbitrum One合约地址
*/
func GetArbitrumContractAddress() string {
	return tokenArbitrumContractAddress
}

/*
获得X-Layer合约地址
*/
func GetXLayerContractAddress() string {
	return tokenXLayerContractAddress
}

/*
获得Solana合约地址
*/
func GetSolanaContractAddress() string {
	return tokenSolanaContractAddress
}

/*
获得Aptos合约地址
*/
func GetAptosContractAddress() string {
	return tokenAptosContractAddress
}

/*
通过okX交易所获得最新的汇率
*/
func GetUsdtRate() (string, decimal.Decimal, float64) {
	// 只有设置了汇率才自动使用动态汇率
	if data := help.GetEnv("USDT_RATE"); data != "" {
		data = strings.TrimSpace(data)
		// 纯数字，固定汇率
		if help.IsNumber(data) {
			if _res, err := strconv.ParseFloat(data, 64); err == nil {
				return "", decimal.Decimal{}, _res
			}
		}

		// 动态交易所汇率，有波动
		if len(data) >= 2 {
			if match, err2 := regexp.MatchString(`^[~+-]\d+(\.\d+)?$`, data); match && err2 == nil {
				_value, err3 := strconv.ParseFloat(data[1:], 64)
				if err3 == nil {
					return string(data[0]), decimal.NewFromFloat(_value), defaultUsdtRate
				}
			}
		}
	}

	// 动态交易所汇率，无波动
	return "=", decimal.Decimal{}, defaultUsdtRate
}

/*
获取回调密钥
*/
func GetAuthToken() string {
	if data := help.GetEnv("AUTH_TOKEN"); data != "" {
		return strings.TrimSpace(data)
	}
	return defaultAuthToken
}

/*
获取监听服务器
*/
func GetListen() string {
	if data := help.GetEnv("LISTEN"); data != "" {
		return strings.TrimSpace(data)
	}
	return defaultListen
}

/*
是否要等待区块链确认完成， 最好不要
*/
func GetTradeConfirmed() bool {
	if data := help.GetEnv("TRADE_IS_CONFIRMED"); data != "" {
		if data == "1" || data == "true" {
			return true
		}
	}
	return false
}

/*
获得Polygon Confirmation确认次数
*/
func GetPolygonConfirmation() int {
	if data := help.GetEnv("ETH_CONFIRMATION"); data != "" {
		num, err := strconv.Atoi(data)
		if err != nil {
			// 如果转换失败，记录错误并返回默认值
			fmt.Printf("ETH_CONFIRMATION配置错误，使用默认值50: %v\n", err)
			return 50
		}
		// 验证配置值的合理性
		if num < 0 || num > 1000 {
			fmt.Printf("ETH_CONFIRMATION配置值超出合理范围(0-1000)，使用默认值50: %d\n", num)
			return 50
		}
		return num
	}

	return 50
}

/*
获取应用的地址
*/
func GetAppUri(host string) string {
	if data := help.GetEnv("APP_URI"); data != "" {
		return strings.TrimSpace(data)
	}
	return host
}

/*
机器人Token
*/
func GetTGBotToken() string {
	if data := help.GetEnv("TG_BOT_TOKEN"); data != "" {
		return strings.TrimSpace(data)
	}
	return ""
}

/*
管理员UID
*/
func GetTGBotAdminId() string {
	if data := help.GetEnv("TG_BOT_ADMIN_ID"); data != "" {
		return strings.TrimSpace(data)
	}
	return ""
}

/*
通知组GID
*/
func GetTgBotGroupId() string {
	if data := help.GetEnv("TG_BOT_GROUP_ID"); data != "" {
		return strings.TrimSpace(data)
	}
	return ""
}

/*
通知的Telegram群组
*/
func GetTgBotNotifyTarget() string {
	var groupId = GetTgBotGroupId()
	if groupId != "" {
		return groupId
	}
	return GetTGBotAdminId()
}

/*
日志路径
*/
func GetOutputLog() string {
	if data := help.GetEnv("LOG_DIR"); data != "" {
		return strings.TrimSpace(data) + "/usdtmore.log"
	}
	return runPath + "/usdtmore.log"
}

/*
模版路径
*/
func GetTemplatePath() string {
	if data := help.GetEnv("HTML_DIR"); data != "" {
		return strings.TrimSpace(data) + "/templates/*"
	}
	return runPath + "/templates/*"
}

/*
静态路径
*/
func GetStaticPath() string {
	if data := help.GetEnv("HTML_DIR"); data != "" {
		return strings.TrimSpace(data) + "/static/"
	}
	return runPath + "/static/"
}

/*
钱包地址，启动以后会自动增加，当然也可以机器人自动添加
*/
func GetInitWalletAddress() []string {
	if data := help.GetEnv("WALLET_ADDRESS"); data != "" {
		return strings.Split(strings.TrimSpace(data), ",")
	}
	return []string{}
}

/*
是否开启反向代理中的https覆写
*/
func IsReWriteHttps() bool {
	if data := help.GetEnv("REWRITE_HTTPS"); data != "" {
		if data == "true" || data == "yes" || data == "1" {
			return true
		}
	}
	return false
}

/*
获取数据库类型
*/
func GetDBType() string {
	if data := help.GetEnv("DB_TYPE"); data != "" {
		return strings.TrimSpace(data)
	}
	return "postgres" // 默认使用PostgreSQL
}

/*
获取数据库主机地址
*/
func GetDBHost() string {
	if data := help.GetEnv("DB_HOST"); data != "" {
		return strings.TrimSpace(data)
	}
	return "localhost"
}

/*
获取数据库端口
*/
func GetDBPort() string {
	if data := help.GetEnv("DB_PORT"); data != "" {
		return strings.TrimSpace(data)
	}
	return "5432"
}

/*
获取数据库名称
*/
func GetDBName() string {
	if data := help.GetEnv("DB_NAME"); data != "" {
		return strings.TrimSpace(data)
	}
	return "usdtmore"
}

/*
获取数据库用户名
*/
func GetDBUser() string {
	if data := help.GetEnv("DB_USER"); data != "" {
		return strings.TrimSpace(data)
	}
	return "usdtmore"
}

/*
获取数据库密码
*/
func GetDBPassword() string {
	if data := help.GetEnv("DB_PASSWORD"); data != "" {
		return strings.TrimSpace(data)
	}
	return ""
}

/*
获取数据库SSL模式
*/
func GetDBSSLMode() string {
	if data := help.GetEnv("DB_SSLMODE"); data != "" {
		return strings.TrimSpace(data)
	}
	return "disable"
}

/*
获取数据库时区
*/
func GetDBTimezone() string {
	if data := help.GetEnv("DB_TIMEZONE"); data != "" {
		return strings.TrimSpace(data)
	}
	return "Asia/Shanghai"
}

/*
获取HTTP请求超时时间（秒）
*/
func GetHttpTimeout() int {
	if data := help.GetEnv("HTTP_TIMEOUT"); data != "" {
		if timeout, err := strconv.Atoi(data); err == nil && timeout > 0 {
			return timeout
		}
	}
	return defaultHttpTimeout
}

/*
获取最大重试次数
*/
func GetMaxRetries() int {
	if data := help.GetEnv("MAX_RETRIES"); data != "" {
		if retries, err := strconv.Atoi(data); err == nil && retries >= 0 {
			return retries
		}
	}
	return defaultMaxRetries
}

/*
获取重试延迟时间（秒）
*/
func GetRetryDelay() int {
	if data := help.GetEnv("RETRY_DELAY"); data != "" {
		if delay, err := strconv.Atoi(data); err == nil && delay > 0 {
			return delay
		}
	}
	return defaultRetryDelay
}

/*
是否启用请求日志
*/
func IsRequestLogEnabled() bool {
	if data := help.GetEnv("REQUEST_LOG_ENABLED"); data != "" {
		return data == "true" || data == "1"
	}
	return false
}

/*
验证安全配置
*/
func ValidateSecurityConfig() []string {
	var warnings []string

	// 检查AUTH_TOKEN强度
	authToken := GetAuthToken()
	if authToken == "" || authToken == "123234" {
		warnings = append(warnings, "AUTH_TOKEN未设置或使用默认值，存在严重安全风险")
	} else if len(authToken) < 16 {
		warnings = append(warnings, "AUTH_TOKEN长度过短，建议至少16位")
	}

	// 检查是否在生产环境使用HTTP
	if strings.ToLower(help.GetEnv("ENVIRONMENT")) == "production" {
		if help.GetEnv("FORCE_HTTPS") != "true" {
			warnings = append(warnings, "生产环境建议启用FORCE_HTTPS=true")
		}
	}

	// 检查必需的API密钥配置（移除未使用的变量）

	// 检查TRON API密钥（至少需要一个）
	if GetTronScanApiKey() == "" && GetTronGridApiKey() == "" {
		warnings = append(warnings, "TRON_SCAN_API_KEY和TRON_GRID_API_KEY至少需要设置一个")
	}

	// 检查EVM兼容链API密钥
	if GetEtherscanApiKey() == "" {
		warnings = append(warnings, "ETHERSCAN_API_KEY是必需的，用于EVM兼容链交易查询")
	}

	// 检查数据库配置
	if GetDBType() == "postgres" {
		if GetDBPassword() == "" {
			warnings = append(warnings, "PostgreSQL数据库密码未设置，存在安全风险")
		}
		if GetDBHost() == "localhost" && strings.ToLower(help.GetEnv("ENVIRONMENT")) == "production" {
			warnings = append(warnings, "生产环境建议使用专用数据库服务器")
		}
	}

	// 检查支付金额范围配置
	minAmount := GetPaymentMinAmount()
	maxAmount := GetPaymentMaxAmount()
	if minAmount.GreaterThanOrEqual(maxAmount) {
		warnings = append(warnings, "支付金额范围配置错误：最小金额应小于最大金额")
	}

	return warnings
}
