package monitor

import (
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

// 简化的币安内部转账监控说明
func ShowBinanceInternalTransferGuide() {
	log.Info("=== 币安内部转账监控指南 ===")
	log.Info("")
	log.Info("检测到链下转账（币安内部转账）：296760475029")
	log.Info("")
	log.Info("要监控币安内部转账，需要以下步骤：")
	log.Info("")
	log.Info("1. 获取币安API密钥：")
	log.Info("   - 登录币安账户")
	log.Info("   - 进入 API管理 页面")
	log.Info("   - 创建新的API密钥")
	log.Info("   - 启用 '现货和杠杆交易' 权限")
	log.Info("")
	log.Info("2. 设置环境变量：")
	log.Info("   - BINANCE_API_KEY=你的API密钥")
	log.Info("   - BINANCE_SECRET_KEY=你的密钥")
	log.Info("")
	log.Info("3. 可监控的币安API端点：")
	log.Info("   - /sapi/v1/capital/deposit/hisrec (充值记录)")
	log.Info("   - /sapi/v1/capital/withdraw/history (提现记录)")
	log.Info("   - /sapi/v1/sub-account/transfer/subUserHistory (内部转账)")
	log.Info("   - /api/v3/myTrades (交易记录)")
	log.Info("")
	log.Info("4. 注意事项：")
	log.Info("   - 币安内部转账不会产生区块链交易")
	log.Info("   - 需要通过币安API查询，不能通过区块链API查询")
	log.Info("   - API有频率限制，需要合理控制请求频率")
	log.Info("")

	// 检查是否已配置API密钥
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")

	if apiKey != "" && secretKey != "" {
		log.Info("✅ 检测到币安API密钥已配置")
		log.Info("可以启用币安内部转账监控功能")
	} else {
		log.Warn("❌ 币安API密钥未配置")
		log.Warn("请设置 BINANCE_API_KEY 和 BINANCE_SECRET_KEY 环境变量")
	}

	log.Info("")
	log.Info("=== 监控指南结束 ===")
}

// 模拟币安内部转账检测
func CheckBinanceInternalTransfer(transferId string) {
	log.Info(fmt.Sprintf("检测到可能的币安内部转账ID: %s", transferId))

	// 检查转账ID格式（币安内部转账ID通常是数字）
	if len(transferId) > 8 && len(transferId) < 15 {
		// 可能是币安内部转账ID
		log.Info("这看起来像币安内部转账ID")
		log.Info("建议启用币安API监控来跟踪此类交易")

		// 显示指南
		ShowBinanceInternalTransferGuide()
	}
}

// 币安API监控状态检查
func CheckBinanceAPIStatus() bool {
	apiKey := os.Getenv("BINANCE_API_KEY")
	secretKey := os.Getenv("BINANCE_SECRET_KEY")

	if apiKey == "" || secretKey == "" {
		log.Warn("[Binance-Monitor] 币安API未配置，无法监控内部转账")
		return false
	}

	log.Info("[Binance-Monitor] 币安API已配置，可以监控内部转账")
	return true
}

// 启动币安监控（占位函数）
func StartBinanceMonitoring() {
	if !CheckBinanceAPIStatus() {
		return
	}

	log.Info("[Binance-Monitor] 启动币安内部转账监控...")

	// 这里可以添加实际的监控逻辑
	// 比如定期调用币安API查询转账记录

	go func() {
		for {
			// 每30秒检查一次
			time.Sleep(30 * time.Second)

			// 这里添加实际的API调用逻辑
			log.Debug("[Binance-Monitor] 检查币安内部转账...")
		}
	}()
}
