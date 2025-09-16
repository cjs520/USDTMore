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
	// 限制显示长度
	if len(body) > 1000 {
		fmt.Printf("📄 响应 (前1000字符):\n%s...\n", string(body[:1000]))
	} else {
		fmt.Printf("📄 响应:\n%s\n", string(body))
	}
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
	
	fmt.Println("🔄 Etherscan V1 API 测试 (无chainid)")
	fmt.Printf("🎯 地址: %s\n", address)
	fmt.Printf("💰 USDT: %s\n", usdtContract)
	
	// 1. 测试Etherscan V1 API (不带chainid，可能默认支持多链)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("1️⃣  Etherscan V1 TOKENTX API")
	fmt.Println(strings.Repeat("=", 60))
	v1TokenURL := fmt.Sprintf("https://api.etherscan.io/api?module=account&action=tokentx&contractaddress=%s&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		usdtContract, address, apiKey)
	makeRequest("Etherscan V1 代币转账查询", v1TokenURL)
	
	// 2. 测试一些知名的BSC地址，看看能不能查到数据
	knownBscAddress := "0x8894e0a0c962cb723c1976a4421c95949be2d4e3" // Binance Hot Wallet
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("2️⃣  测试知名BSC地址")
	fmt.Println(strings.Repeat("=", 60))
	knownURL := fmt.Sprintf("https://api.etherscan.io/api?module=account&action=tokentx&contractaddress=%s&address=%s&page=1&offset=5&sort=desc&apikey=%s",
		usdtContract, knownBscAddress, apiKey)
	makeRequest("测试Binance热钱包地址", knownURL)
	
	// 3. 尝试不指定合约地址，查看所有代币转账
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("3️⃣  查询所有代币转账")
	fmt.Println(strings.Repeat("=", 60))
	allTokenURL := fmt.Sprintf("https://api.etherscan.io/api?module=account&action=tokentx&address=%s&page=1&offset=10&sort=desc&apikey=%s",
		address, apiKey)
	makeRequest("查询地址所有代币转账", allTokenURL)
	
	// 4. 测试普通交易
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("4️⃣  测试普通交易查询")
	fmt.Println(strings.Repeat("=", 60))
	normalTxURL := fmt.Sprintf("https://api.etherscan.io/api?module=account&action=txlist&address=%s&page=1&offset=10&sort=desc&apikey=%s",
		address, apiKey)
	makeRequest("普通交易查询", normalTxURL)
	
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✅ Etherscan V1 API 测试完成")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("💡 分析:")
	fmt.Println("   - V1 API默认查询以太坊主网")
	fmt.Println("   - BSC需要专门的API端点")
	fmt.Println("   - 可能需要配置不同链的专用API")
}