package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type OrderStatus struct {
	TradeId   string `json:"trade_id"`
	Status    int    `json:"status"`
	ReturnUrl string `json:"return_url"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run query_order.go <trade_id>")
		fmt.Println("示例: go run query_order.go a9bc4dc8-6704-45b3-8ab9-424f8afb198b")
		os.Exit(1)
	}

	tradeId := os.Args[1]
	url := fmt.Sprintf("https://api.wkk.su/pay/check-status/%s", tradeId)

	fmt.Printf("查询订单: %s\n", tradeId)
	fmt.Printf("请求URL: %s\n", url)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ 读取响应失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("HTTP状态: %d\n", resp.StatusCode)
	fmt.Printf("原始响应: %s\n", string(body))

	var orderStatus OrderStatus
	if err := json.Unmarshal(body, &orderStatus); err != nil {
		fmt.Printf("❌ 解析JSON失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n📊 订单状态详情:")
	fmt.Printf("订单ID: %s\n", orderStatus.TradeId)
	
	statusText := ""
	statusIcon := ""
	switch orderStatus.Status {
	case 1:
		statusText = "等待支付"
		statusIcon = "⏳"
	case 2:
		statusText = "支付成功"
		statusIcon = "✅"
	case 3:
		statusText = "订单过期"
		statusIcon = "❌"
	default:
		statusText = "未知状态"
		statusIcon = "❓"
	}
	
	fmt.Printf("订单状态: %s %s (%d)\n", statusIcon, statusText, orderStatus.Status)
	fmt.Printf("回调URL: %s\n", orderStatus.ReturnUrl)

	if orderStatus.Status == 1 {
		fmt.Println("\n💡 订单处于等待支付状态，这是正常的")
		fmt.Println("ℹ️  用户需要按照页面显示的金额和地址进行USDT转账")
		fmt.Println("ℹ️  系统会自动检测到账并更新订单状态")
	}
}