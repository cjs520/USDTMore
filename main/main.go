package main

import (
	"USDTMore/app/config"
	"USDTMore/app/help"
	"USDTMore/app/log"
	"USDTMore/app/model"
	"USDTMore/app/monitor"
	"USDTMore/app/web"
	"context"
	"fmt"
	"os"
	"time"
)

// Version 版本号说明 1.0.0 代表主版本号.功能版本号.修订号
const Version = "1.0.0"

func main() {
	// 添加全局panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			log.Error("应用程序发生panic:", r)
			// 执行清理工作
			if err := model.Close(); err != nil {
				log.Error("关闭数据库连接失败:", err)
			}
			help.CleanupClients()
			os.Exit(1)
		}
	}()

	// 数据库初始化
	if err := model.Init(); err != nil {
		log.Error("数据库初始化失败：", err)
		os.Exit(1)
	}

	// 检查必要配置
	if config.GetTGBotToken() == "" || config.GetTGBotAdminId() == "" {
		log.Error("请配置参数 TG_BOT_TOKEN 和 TG_BOT_ADMIN_ID")
		os.Exit(1)
	}

	// 获取上下文管理器
	contextManager := help.GetContextManager()

	// 启动各个服务
	contextManager.RunWithContext("BotStart", func(ctx context.Context) {
		monitor.BotStart(ctx, Version)
	})

	contextManager.RunWithContext("RateMonitor", func(ctx context.Context) {
		monitor.OkxUsdtRateStart(ctx)
	})

	contextManager.RunWithContext("TradeMonitor", func(ctx context.Context) {
		monitor.TradeStart(ctx)
	})

	contextManager.RunWithContext("NotifyMonitor", func(ctx context.Context) {
		monitor.NotifyStart(ctx)
	})

	contextManager.RunWithContext("WebServer", func(ctx context.Context) {
		web.Start(ctx)
	})

	fmt.Printf("USDTMore 启动成功，当前版本：%s\n", Version)
	fmt.Println("按 Ctrl+C 停止服务")

	// 等待优雅关闭
	contextManager.WaitForShutdown(30 * time.Second)

	// 关闭数据库连接
	if err := model.Close(); err != nil {
		log.Error("关闭数据库连接失败:", err)
	}

	fmt.Println("服务已停止")
}
