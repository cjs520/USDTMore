package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func makeRequest(description, url string) {
	fmt.Printf("\n🔍 %s\n", description)
	fmt.Printf("URL: %s\n", url)
	fmt.Println(strings.Repeat("━", 60))
	
	client := &http.Client{Timeout: 30 * time.Second}
	
	start := time.Now()
	resp, err := client.Get(url)
	duration := time.Since(start)
	
	if err != nil {
		fmt.Printf("❌ 请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("❌ 读取响应失败: %v\n", err)
		return
	}
	
	fmt.Printf("⏱️  耗时: %v\n", duration)
	fmt.Printf("📊 状态: %d\n", resp.StatusCode)
	fmt.Printf("📏 长度: %d bytes\n", len(body))
	fmt.Printf("📄 响应:\n%s\n", string(body))
}

func main() {
	apiKey := os.Getenv("ETHERSCAN_API_KEY")
	if len(os.Args) > 1 {
		apiKey = os.Args[1]
	}
	if apiKey == "" {
		fmt.Println("请设置API Key")
		return
	}
	
	address := "0xb5bc9cf309320abe924f38b13ec5c519ae04dbc5"
	usdtContract := "0x55d398326f99059fF775485246999027B3197955"
	
	fmt.Println("🌐 BscScan API 测试")
	fmt.Printf("🎯 地址: %s\n", address)
	fmt.Printf("💰 USDT: %s\n", usdtContract)
	
	// 1. 使用BscScan官方API (不需要chainid参数)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("1️⃣  BscScan TOKENTX API")
	fmt.Println(strings.Repeat("=", 60))
	bscTokenURL := fmt.Sprintf("https://api.bscscan.com/api?module=account&action=tokentx&contractaddress=%s&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		usdtContract, address, apiKey)
	makeRequest("BscScan代币转账查询", bscTokenURL)
	
	// 2. 使用BscScan普通交易查询
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("2️⃣  BscScan TXLIST API")
	fmt.Println(strings.Repeat("=", 60))
	bscTxURL := fmt.Sprintf("https://api.bscscan.com/api?module=account&action=txlist&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		address, apiKey)
	makeRequest("BscScan普通交易查询", bscTxURL)
	
	// 3. 使用BscScan内部交易查询
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("3️⃣  BscScan TXLISTINTERNAL API")
	fmt.Println(strings.Repeat("=", 60))
	bscInternalURL := fmt.Sprintf("https://api.bscscan.com/api?module=account&action=txlistinternal&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		address, apiKey)
	makeRequest("BscScan内部交易查询", bscInternalURL)
	
	// 4. 余额确认
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("4️⃣  BscScan 余额查询")
	fmt.Println(strings.Repeat("=", 60))
	bscBalanceURL := fmt.Sprintf("https://api.bscscan.com/api?module=account&action=tokenbalance&contractaddress=%s&address=%s&tag=latest&apikey=%s",
		usdtContract, address, apiKey)
	makeRequest("BscScan USDT余额查询", bscBalanceURL)
	
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✅ BscScan API 测试完成")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("💡 关键发现:")
	fmt.Println("   - Etherscan V2 API对BSC的免费访问有限制")
	fmt.Println("   - BscScan.com是BSC官方的区块浏览器API")
	fmt.Println("   - 应该使用api.bscscan.com而不是api.etherscan.io/v2")
	fmt.Println("   - BscScan API不需要chainid参数")
}