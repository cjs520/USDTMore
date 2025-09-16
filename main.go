package main

import (
	"USDTMore/app/config"
	"USDTMore/app/log"
	"USDTMore/app/model"
	"USDTMore/app/monitor"
	"USDTMore/app/web"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

var (
	healthCheck = flag.Bool("health-check", false, "执行健康检查")
	version     = flag.Bool("version", false, "显示版本信息")
)

const AppVersion = "v2.1.0"

func main() {
	flag.Parse()

	// 处理命令行参数
	if *version {
		fmt.Printf("USDTMore %s\n", AppVersion)
		fmt.Println("一款多链路支持更易用的USDT收款网关")
		os.Exit(0)
	}

	if *healthCheck {
		performHealthCheck()
		return
	}

	// 日志系统会自动初始化
	log.Info("🚀 USDTMore 启动中...")
	log.Info("📋 版本:", AppVersion)

	// 显示API Key配置信息
	apiKeyCount := config.GetEtherscanApiKeyCount()
	if apiKeyCount > 0 {
		log.Info(fmt.Sprintf("🔑 Etherscan API Key配置: %d个密钥轮询使用", apiKeyCount))
	} else {
		log.Warn("⚠️  未配置Etherscan API Key，EVM链交易查询将失败")
	}

	// 显示配置警告
	warnings := config.ValidateSecurityConfig()
	if len(warnings) > 0 {
		log.Warn("⚠️  配置警告:")
		for _, warning := range warnings {
			log.Warn("   - " + warning)
		}
	}

	// 验证BSC Web3配置
	if err := config.ValidateBscWeb3Config(); err != nil {
		log.Error("❌ BSC配置错误:", err.Error())
		log.Info("💡 BSC配置建议:")
		suggestions := config.GetBscConfigSuggestions()
		for _, suggestion := range suggestions {
			log.Info("   " + suggestion)
		}
		log.Warn("⚠️  BSC链交易监控将无法正常工作")
	} else {
		log.Info("✅ BSC Web3 API配置验证通过")
	}

	// 初始化数据库
	log.Info("🗄️  初始化数据库连接...")
	model.Init()

	// 显示钱包地址统计
	addressStats := model.GetWalletAddressStats()
	if len(addressStats) > 0 {
		log.Info("💳 钱包地址配置统计:")
		for chain, count := range addressStats {
			log.Info(fmt.Sprintf("   - %s: %d个地址", chain, count))
		}
	} else {
		log.Warn("⚠️  未配置任何钱包地址，请通过Telegram机器人添加收款地址")
	}

	// 启动监控服务
	log.Info("📊 启动交易监控服务...")
	go monitor.TradeStart()

	// 启动通知服务
	log.Info("📢 启动通知服务...")
	go monitor.NotifyStart()

	// 启动汇率监控服务
	log.Info("💱 启动汇率监控服务...")
	go monitor.OkxUsdtRateStart()

	// 启动Telegram机器人
	if config.GetTGBotToken() != "" {
		log.Info("🤖 启动Telegram机器人...")
		go monitor.BotStart(AppVersion)
	}

	// 启动Web服务
	log.Info("🌐 启动Web服务...")
	web.Start()
}

// performHealthCheck 执行健康检查
func performHealthCheck() {
	// 检查数据库连接
	if err := model.HealthCheck(); err != nil {
		fmt.Printf("数据库健康检查失败: %v\n", err)
		os.Exit(1)
	}

	// 检查Web服务
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:6080/api/health")
	if err != nil {
		fmt.Printf("Web服务健康检查失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Web服务返回错误状态码: %d\n", resp.StatusCode)
		os.Exit(1)
	}

	fmt.Println("健康检查通过")
	os.Exit(0)
}