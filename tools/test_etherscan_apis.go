package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

func testAPI(description, url string) {
	fmt.Printf("\n=== %s ===\n", description)
	fmt.Printf("URL: %s\n", url)
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		return
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("读取响应错误: %v\n", err)
		return
	}
	
	fmt.Printf("状态码: %d\n", resp.StatusCode)
	fmt.Printf("响应长度: %d bytes\n", len(body))
	
	// 显示前500个字符
	if len(body) > 500 {
		fmt.Printf("响应内容 (前500字符): %s...\n", string(body[:500]))
	} else {
		fmt.Printf("响应内容: %s\n", string(body))
	}
}

func main() {
	apiKey := os.Getenv("ETHERSCAN_API_KEY")
	if apiKey == "" {
		apiKey = "YourApiKeyToken" // 使用默认值进行测试
	}
	
	address := "0xb5bc9cf309320abe924f38b13ec5c519ae04dbc5"
	usdtContract := "0x55d398326f99059fF775485246999027B3197955" // BSC USDT
	
	// 测试1: 当前使用的tokentx API (代币转账)
	tokentxURL := fmt.Sprintf("https://api.etherscan.io/v2/api?chainid=56&module=account&action=tokentx&contractaddress=%s&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		usdtContract, address, apiKey)
	testAPI("代币转账查询 (tokentx)", tokentxURL)
	
	// 测试2: 普通交易查询 (txlist)
	txlistURL := fmt.Sprintf("https://api.etherscan.io/v2/api?chainid=56&module=account&action=txlist&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		address, apiKey)
	testAPI("普通交易查询 (txlist)", txlistURL)
	
	// 测试3: 内部交易查询 (txlistinternal)
	internalURL := fmt.Sprintf("https://api.etherscan.io/v2/api?chainid=56&module=account&action=txlistinternal&address=%s&page=1&offset=10&startblock=46962026&endblock=latest&sort=desc&apikey=%s",
		address, apiKey)
	testAPI("内部交易查询 (txlistinternal)", internalURL)
	
	// 测试4: 余额查询
	balanceURL := fmt.Sprintf("https://api.etherscan.io/v2/api?chainid=56&module=account&action=tokenbalance&contractaddress=%s&address=%s&tag=latest&apikey=%s",
		usdtContract, address, apiKey)
	testAPI("代币余额查询 (tokenbalance)", balanceURL)
}