package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
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
	if len(body) > 800 {
		fmt.Printf("📄 响应 (前800字符):\n%s...\n", string(body[:800]))
	} else {
		fmt.Printf("📄 响应:\n%s\n", string(body))
	}
}

func main() {
	address := "0xb5bc9cf309320abe924f38b13ec5c519ae04dbc5"
	usdtContract := "0x55d398326f99059fF775485246999027B3197955"
	
	fmt.Println("🔍 BSC API 替代方案测试")
	fmt.Printf("🎯 地址: %s\n", address)
	fmt.Printf("💰 USDT: %s\n", usdtContract)
	fmt.Println()
	
	// 1. 测试 BSCTrace API (NodeReal 提供)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("1️⃣  BSCTrace API 测试 (NodeReal)")
	fmt.Println(strings.Repeat("=", 60))
	
	// BSCTrace 可能使用标准的以太坊JSON-RPC格式
	bsctraceURL := fmt.Sprintf("https://bsctrace.com/api?module=account&action=tokentx&contractaddress=%s&address=%s&page=1&offset=10&sort=desc",
		usdtContract, address)
	makeRequest("BSCTrace 代币转账查询", bsctraceURL)
	
	// 2. 测试 Moralis API (免费层)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("2️⃣  Moralis API 测试 (需要API Key)")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("⚠️  Moralis需要注册获取API Key，跳过测试")
	fmt.Println("📋 Moralis API格式示例:")
	fmt.Println("   GET https://deep-index.moralis.io/api/v2/{address}/erc20")
	fmt.Println("   Headers: X-API-Key: YOUR_API_KEY")
	
	// 3. 测试 QuickNode (免费层)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("3️⃣  QuickNode 测试")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("⚠️  QuickNode需要注册获取端点，跳过直接测试")
	fmt.Println("📋 QuickNode使用标准JSON-RPC格式")
	
	// 4. 测试 GetBlock (免费层)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("4️⃣  GetBlock API 测试")
	fmt.Println(strings.Repeat("=", 60))
	
	// GetBlock通常提供JSON-RPC端点
	getblockURL := "https://bsc.getblock.io/mainnet/"
	fmt.Printf("📋 GetBlock BSC端点: %s\n", getblockURL)
	fmt.Println("⚠️  需要POST请求和API Key，这里只测试连通性")
	makeRequest("GetBlock连通性测试", getblockURL)
	
	// 5. 测试 Ankr (免费层)
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("5️⃣  Ankr 公共端点测试")
	fmt.Println(strings.Repeat("=", 60))
	
	ankrURL := "https://rpc.ankr.com/bsc"
	fmt.Printf("📋 Ankr BSC端点: %s\n", ankrURL)
	fmt.Println("⚠️  这是JSON-RPC端点，需要POST请求")
	makeRequest("Ankr连通性测试", ankrURL)
	
	// 6. 测试一些公共的BSC RPC端点
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("6️⃣  公共BSC RPC端点测试")
	fmt.Println(strings.Repeat("=", 60))
	
	publicRPCs := []string{
		"https://bsc-dataseed.binance.org/",
		"https://bsc-dataseed1.defibit.io/",
		"https://bsc-dataseed1.ninicoin.io/",
	}
	
	for i, rpc := range publicRPCs {
		fmt.Printf("\n📡 测试公共RPC %d: %s\n", i+1, rpc)
		makeRequest(fmt.Sprintf("公共RPC %d", i+1), rpc)
	}
	
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("✅ 测试完成")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("💡 推荐方案:")
	fmt.Println("   1. 🥇 Moralis - 强大的Web3 API，有免费层")
	fmt.Println("   2. 🥈 QuickNode - 专业的区块链基础设施")
	fmt.Println("   3. 🥉 Ankr - 免费的公共RPC端点")
	fmt.Println("   4. 🔧 GetBlock - 可靠的节点服务")
	fmt.Println("   5. 🆓 公共RPC - 直接使用JSON-RPC调用")
	fmt.Println()
	fmt.Println("🚀 下一步:")
	fmt.Println("   - 注册Moralis免费账户获取API Key")
	fmt.Println("   - 或者实现JSON-RPC调用方式")
	fmt.Println("   - 修改代码支持新的API格式")
}