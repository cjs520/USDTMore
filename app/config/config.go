package config

import (
	"USDTMore/app/help"
	"fmt"
	"github.com/shopspring/decimal"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultExpireTime = 1800        // 订单默认有效期 10分钟
const defaultUsdtRate = 7.4           // 默认汇率
const defaultAuthToken = "123234"     // 默认授权码
const defaultListen = ":6080"         // 默认监听地址
const TronServerApiScan = "TRON_SCAN" //
const TronServerApiGrid = "TRON_GRID" //
const defaultPaymentMinAmount = 0.01  //
const defaultPaymentMaxAmount = 99999 //

// 当前路径
var runPath string

func init() {
	// 获取应用程序当前的路径
	execPath, err := os.Executable()
	if err != nil {
		// 如果无法获取可执行文件路径，使用当前工作目录
		pwd, pwdErr := os.Getwd()
		if pwdErr != nil {
			// 最后的备选方案
			runPath = "."
		} else {
			runPath = pwd
		}
	} else {
		runPath = filepath.Dir(execPath)
	}
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
	return "c0634c05-b4db-4fa4-a14a-93f2c2d5b65e"
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
	return "YourSolanaApiKey"
}

/*
获得Aptos接口的API密钥
*/
func GetAptosApiKey() string {
	if data := help.GetEnv("APTOS_API_KEY"); data != "" {
		return strings.TrimSpace(data)
	}
	return "YourAptosApiKey"
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
			// 如果转换失败，处理错误
			fmt.Println("转换错误:", err)
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
获取PostgreSQL连接DSN
*/
func GetPostgreSQLDSN() string {
	if dsn := help.GetEnv("POSTGRESQL_DSN"); dsn != "" {
		return strings.TrimSpace(dsn)
	}
	
	// 从单独的环境变量构建DSN
	host := GetPostgreSQLHost()
	port := GetPostgreSQLPort()
	user := GetPostgreSQLUser()
	password := GetPostgreSQLPassword()
	dbname := GetPostgreSQLDatabase()
	sslmode := GetPostgreSQLSSLMode()
	timezone := GetPostgreSQLTimeZone()
	
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s", 
		host, port, user, password, dbname, sslmode, timezone)
}

/*
获取环境变量，如果不存在则返回默认值
*/
func getEnvWithDefault(key, defaultValue string) string {
	if value := help.GetEnv(key); value != "" {
		return strings.TrimSpace(value)
	}
	return defaultValue
}

/*
获取数据库类型
*/
func GetDatabaseType() string {
	return "postgresql"
}


/*
获取PostgreSQL主机地址
*/
func GetPostgreSQLHost() string {
	return getEnvWithDefault("DB_HOST", "localhost")
}

/*
获取PostgreSQL端口
*/
func GetPostgreSQLPort() string {
	return getEnvWithDefault("DB_PORT", "5432")
}

/*
获取PostgreSQL用户名
*/
func GetPostgreSQLUser() string {
	return getEnvWithDefault("DB_USER", "postgres")
}

/*
获取PostgreSQL密码
*/
func GetPostgreSQLPassword() string {
	return help.GetEnv("DB_PASSWORD")
}

/*
获取PostgreSQL数据库名
*/
func GetPostgreSQLDatabase() string {
	return getEnvWithDefault("DB_NAME", "usdtmore")
}

/*
获取PostgreSQL SSL模式
*/
func GetPostgreSQLSSLMode() string {
	return getEnvWithDefault("DB_SSLMODE", "disable")
}

/*
获取PostgreSQL时区
*/
func GetPostgreSQLTimeZone() string {
	return getEnvWithDefault("DB_TIMEZONE", "Asia/Shanghai")
}

/*
获取PostgreSQL连接超时时间
*/
func GetPostgreSQLConnectTimeout() string {
	return getEnvWithDefault("DB_CONNECT_TIMEOUT", "10")
}

/*
获取PostgreSQL应用程序名称
*/
func GetPostgreSQLAppName() string {
	return getEnvWithDefault("DB_APP_NAME", "usdtmore")
}

/*
获取完整的PostgreSQL连接DSN（包含所有可选参数）
*/
func GetPostgreSQLFullDSN() string {
	if dsn := help.GetEnv("POSTGRESQL_DSN"); dsn != "" {
		return strings.TrimSpace(dsn)
	}
	
	// 构建完整DSN
	host := GetPostgreSQLHost()
	port := GetPostgreSQLPort()
	user := GetPostgreSQLUser()
	password := GetPostgreSQLPassword()
	dbname := GetPostgreSQLDatabase()
	sslmode := GetPostgreSQLSSLMode()
	timezone := GetPostgreSQLTimeZone()
	connectTimeout := GetPostgreSQLConnectTimeout()
	appName := GetPostgreSQLAppName()
	
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s connect_timeout=%s application_name=%s", 
		host, port, user, password, dbname, sslmode, timezone, connectTimeout, appName)
}

/*
获取数据库连接字符串
*/
func GetDatabaseConnectionString() string {
	return GetPostgreSQLDSN()
}

/*
数据库调试模式
*/
func GetDbDebug() bool {
	if data := help.GetEnv("DB_DEBUG"); data != "" {
		return data == "1" || data == "true"
	}
	return false
}

/*
数据库最大空闲连接数
*/
func GetDbMaxIdleConns() int {
	if data := help.GetEnv("DB_MAX_IDLE_CONNS"); data != "" {
		if num, err := strconv.Atoi(data); err == nil && num > 0 {
			return num
		}
	}
	return 10
}

/*
数据库最大开放连接数
*/
func GetDbMaxOpenConns() int {
	if data := help.GetEnv("DB_MAX_OPEN_CONNS"); data != "" {
		if num, err := strconv.Atoi(data); err == nil && num > 0 {
			return num
		}
	}
	return 100
}

/*
数据库连接最大生存时间
*/
func GetDbConnMaxLifetime() time.Duration {
	if data := help.GetEnv("DB_CONN_MAX_LIFETIME"); data != "" {
		if duration, err := time.ParseDuration(data); err == nil {
			return duration
		}
	}
	return 5 * time.Minute
}

/*
数据库连接最大空闲时间
*/
func GetDbConnMaxIdleTime() time.Duration {
	if data := help.GetEnv("DB_CONN_MAX_IDLE_TIME"); data != "" {
		if duration, err := time.ParseDuration(data); err == nil {
			return duration
		}
	}
	return 1 * time.Minute
}

/*
数据库备份路径
*/
func GetDbBackupPath() string {
	if data := help.GetEnv("DB_BACKUP_DIR"); data != "" {
		return strings.TrimSpace(data)
	}
	return runPath + "/backups"
}

/*
数据库备份保留天数
*/
func GetDbBackupRetentionDays() int {
	if data := help.GetEnv("DB_BACKUP_RETENTION_DAYS"); data != "" {
		if num, err := strconv.Atoi(data); err == nil && num > 0 {
			return num
		}
	}
	return 7
}

/*
是否启用数据库监控
*/
func IsDbMonitoringEnabled() bool {
	if data := help.GetEnv("DB_MONITORING_ENABLED"); data != "" {
		return data == "1" || data == "true"
	}
	return false
}

/*
数据库监控间隔
*/
func GetDbMonitoringInterval() time.Duration {
	if data := help.GetEnv("DB_MONITORING_INTERVAL"); data != "" {
		if duration, err := time.ParseDuration(data); err == nil {
			return duration
		}
	}
	return 30 * time.Second
}

/*
数据库慢查询阈值
*/
func GetDbSlowQueryThreshold() time.Duration {
	if data := help.GetEnv("DB_SLOW_QUERY_THRESHOLD"); data != "" {
		if duration, err := time.ParseDuration(data); err == nil {
			return duration
		}
	}
	return 1 * time.Second
}

/*
模版路径
*/
func GetTemplatePath() string {
	if data := help.GetEnv("HTML_DIR"); data != "" {
		return strings.TrimSpace(data) + "/templates/*"
	}
	return runPath + "/../templates/*"
}

/*
静态路径
*/
func GetStaticPath() string {
	if data := help.GetEnv("HTML_DIR"); data != "" {
		return strings.TrimSpace(data) + "/static/"
	}
	return runPath + "/../static/"
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
