package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

type JSONRPCRequest struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type JSONRPCResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result"`
	Error   *RPCError   `json:"error"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func makeJSONRPCRequest(url, method string, params []interface{}) {
	fmt.Printf("\n🔍 JSON-RPC 调用: %s\n", method)
	fmt.Printf("🌐 端点: %s\n", url)
	fmt.Printf("📋 参数: %v\n", params)
	fmt.Println(strings.Repeat("━", 60))
	
	request := JSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		fmt.Printf("❌ JSON编码失败: %v\n", err)
		return
	}
	
	client := &http.Client{Timeout: 30 * time.Second}
	
	start := time.Now()
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
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
	
	// 解析JSON响应
	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		fmt.Printf("❌ JSON解析失败: %v\n", err)
		fmt.Printf("📄 原始响应: %s\n", string(body))
		return
	}
	
	if rpcResp.Error != nil {
		fmt.Printf("❌ RPC错误: [%d] %s\n", rpcResp.Error.Code, rpcResp.Error.Message)
		return
	}
	
	// 格式化输出结果
	resultJSON, _ := json.MarshalIndent(rpcResp.Result, "", "  ")
	if len(resultJSON) > 1000 {
		fmt.Printf("✅ 结果 (前1000字符): %s...\n", string(resultJSON[:1000]))
	} else {
		fmt.Printf("✅ 结果: %s\n", string(resultJSON))
	}
}

func main() {
	address := "0xb5bc9cf309320abe924f38b13ec5c519ae04dbc5"
	usdtContract := "0x55d398326f99059fF775485246999027B3197955"
	
	fmt.Println("🚀 BSC JSON-RPC API 测试")
	fmt.Printf("🎯 地址: %s\n", address)
	fmt.Printf("💰 USDT合约: %s\n", usdtContract)
	fmt.Println()
	
	// 测试不同的BSC JSON-RPC端点
	endpoints := []string{
		"https://rpc.ankr.com/bsc",
		"https://bsc-dataseed.binance.org/",
		"https://bsc-dataseed1.defibit.io/",
		"https://endpoints.omniatech.io/v1/bsc/mainnet/public",
	}
	
	for i, endpoint := range endpoints {
		fmt.Println(strings.Repeat("=", 60))
		fmt.Printf("🔗 测试端点 %d: %s\n", i+1, endpoint)
		fmt.Println(strings.Repeat("=", 60))
		
		// 1. 测试基础连接 - 获取最新区块号
		makeJSONRPCRequest(endpoint, "eth_blockNumber", []interface{}{})
		
		// 2. 测试获取BNB余额
		makeJSONRPCRequest(endpoint, "eth_getBalance", []interface{}{address, "latest"})
		
		// 3. 测试获取USDT余额 (ERC-20 balanceOf)
		// balanceOf(address) 的函数签名是 0x70a08231
		balanceOfData := "0x70a08231000000000000000000000000" + strings.TrimPrefix(address, "0x")
		makeJSONRPCRequest(endpoint, "eth_call", []interface{}{
			map[string]interface{}{
				"to":   usdtContract,
				"data": balanceOfData,
			},
			"latest",
		})
		
		// 4. 测试获取交易记录 - 这个比较复杂，需要事件过滤
		fmt.Printf("\n📋 注意: 获取ERC-20转账记录需要使用eth_getLogs方法\n")
		fmt.Printf("🔍 USDT Transfer事件签名: 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef\n")
		
		// Transfer事件过滤器
		transferTopic := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
		addressTopic := "0x000000000000000000000000" + strings.TrimPrefix(address, "0x")
		
		makeJSONRPCRequest(endpoint, "eth_getLogs", []interface{}{
			map[string]interface{}{
				"address":   usdtContract,
				"topics":    []interface{}{transferTopic, nil, addressTopic}, // Transfer to address
				"fromBlock": "0x2CCE05A", // 46962026 in hex
				"toBlock":   "latest",
			},
		})
		
		fmt.Println()
	}
	
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("✅ JSON-RPC 测试完成")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("💡 总结:")
	fmt.Println("   - JSON-RPC是访问BSC最直接的方式")
	fmt.Println("   - eth_getLogs可以查询ERC-20转账事件")
	fmt.Println("   - 需要解析Transfer事件日志获取转账信息")
	fmt.Println("   - 比Etherscan API更复杂但更可靠")
	fmt.Println()
	fmt.Println("🔧 实施建议:")
	fmt.Println("   1. 选择一个稳定的RPC端点")
	fmt.Println("   2. 实现eth_getLogs调用")
	fmt.Println("   3. 解析Transfer事件日志")
	fmt.Println("   4. 添加错误处理和重试机制")
}