package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func makeRequest(url string) {
	fmt.Printf("\n🔍 查询URL: %s\n", url)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
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
	
	fmt.Printf("⏱️  请求耗时: %v\n", duration)
	fmt.Printf("📊 状态码: %d\n", resp.StatusCode)
	fmt.Printf("📏 响应长度: %d bytes\n", len(body))
	fmt.Printf("📄 响应内容:\n%s\n", string(body))
}

func main() {
	// 从环境变量或命令行参数获取API Key
	apiKey := os.Getenv("ETHERSCAN_API_KEY")
	if len(os.Args) > 1 {
		apiKey = os.Args[1]
	}
	if apiKey == "" {
		fmt.Println("请设置ETHERSCAN_API_KEY环境变量或作为命令行参数传入")
		fmt.Println("用法: go run manual_query.go [API_KEY]")
		return
	}
	
	// 目标地址和合约地址
	address := "0xb5bc9cf309320abe924f38b13ec5c519ae04dbc5"
	usdtContract := "0x55d398326f99059fF775485246999027B3197955" // BSC USDT
	chainId := "56" // BSC
	
	fmt.Println("🚀 Etherscan API 手动查询工具")
	fmt.Printf("🎯 目标地址: %s\n", address)
	fmt.Printf("💰 USDT合约: %s\n", usdtContract)
	fmt.Printf("⛓️  链ID: %s (BSC)\n", chainId)
	fmt.Printf("🔑 API Key: %s...%s\n", apiKey[:4], apiKey[len(apiKey)-4:])
	
	// 1. tokentx - ERC20代币转账查询
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("1️⃣  TOKENTX 查询 (ERC-20 代币转账)")
	fmt.Println(strings.Repeat("=", 60))
	tokentxURL := fmt.Sprintf("https://api.etherscan.io/v2/api?chainid=%s&module=account&action=tokentx&contractaddress=%s&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		chainId, usdtContract, address, apiKey)
	makeRequest(tokentxURL)
	
	// 2. txlist - 普通交易查询
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("2️⃣  TXLIST 查询 (普通交易)")
	fmt.Println(strings.Repeat("=", 60))
	txlistURL := fmt.Sprintf("https://api.etherscan.io/v2/api?chainid=%s&module=account&action=txlist&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		chainId, address, apiKey)
	makeRequest(txlistURL)
	
	// 3. txlistinternal - 内部交易查询
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("3️⃣  TXLISTINTERNAL 查询 (内部交易)")
	fmt.Println(strings.Repeat("=", 60))
	internalURL := fmt.Sprintf("https://api.etherscan.io/v2/api?chainid=%s&module=account&action=txlistinternal&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		chainId, address, apiKey)
	makeRequest(internalURL)
	
	// 4. tokenbalance - 代币余额查询
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("4️⃣  TOKENBALANCE 查询 (代币余额)")
	fmt.Println(strings.Repeat("=", 60))
	balanceURL := fmt.Sprintf("https://api.etherscan.io/v2/api?chainid=%s&module=account&action=tokenbalance&contractaddress=%s&address=%s&tag=latest&apikey=%s",
		chainId, usdtContract, address, apiKey)
	makeRequest(balanceURL)
	
	// 5. balance - ETH余额查询
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("5️⃣  BALANCE 查询 (ETH余额)")
	fmt.Println(strings.Repeat("=", 60))
	ethBalanceURL := fmt.Sprintf("https://api.etherscan.io/v2/api?chainid=%s&module=account&action=balance&address=%s&tag=latest&apikey=%s",
		chainId, address, apiKey)
	makeRequest(ethBalanceURL)
	
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✅ 查询完成")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("💡 分析建议:")
	fmt.Println("   - 如果tokentx返回空结果但txlistinternal有数据，说明USDT转账通过智能合约进行")
	fmt.Println("   - 如果所有查询都失败，检查API Key是否有效")
	fmt.Println("   - 如果返回NOTOK，查看具体错误信息")
	fmt.Printf("   - 当前时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
}