package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
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

type LogEntry struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	BlockHash        string   `json:"blockHash"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
}

func makeJSONRPCCall(endpoint, method string, params []interface{}) (interface{}, error) {
	request := JSONRPCRequest{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("JSON编码失败: %v", err)
	}
	
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}
	
	var rpcResp JSONRPCResponse
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %v", err)
	}
	
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("RPC错误 [%d]: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}
	
	return rpcResp.Result, nil
}

func hexToInt(hexStr string) int64 {
	if hexStr == "" {
		return 0
	}
	// 移除0x前缀
	hexStr = strings.TrimPrefix(hexStr, "0x")
	if hexStr == "" {
		return 0
	}
	
	result, err := strconv.ParseInt(hexStr, 16, 64)
	if err != nil {
		return 0
	}
	return result
}

func intToHex(num int64) string {
	return fmt.Sprintf("0x%x", num)
}

func getUSDTTransfers(endpoint, usdtContract, address string, fromBlock int64) ([]LogEntry, error) {
	fmt.Printf("🔍 查询USDT转账记录...\n")
	fmt.Printf("📍 端点: %s\n", endpoint)
	fmt.Printf("💰 USDT合约: %s\n", usdtContract)
	fmt.Printf("🎯 目标地址: %s\n", address)
	fmt.Printf("📦 起始区块: %d\n", fromBlock)
	
	// Transfer事件签名 
	transferTopic := "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
	// 目标地址作为接收方 (第三个参数)
	addressTopic := "0x000000000000000000000000" + strings.TrimPrefix(strings.ToLower(address), "0x")
	
	var allLogs []LogEntry
	currentBlock := fromBlock
	batchSize := int64(10000) // 每次查询10000个区块
	
	// 获取最新区块号
	latestResult, err := makeJSONRPCCall(endpoint, "eth_blockNumber", []interface{}{})
	if err != nil {
		return nil, fmt.Errorf("获取最新区块失败: %v", err)
	}
	
	latestBlock := hexToInt(latestResult.(string))
	fmt.Printf("📊 最新区块: %d\n", latestBlock)
	
	for currentBlock <= latestBlock {
		toBlock := currentBlock + batchSize
		if toBlock > latestBlock {
			toBlock = latestBlock
		}
		
		fmt.Printf("🔄 查询区块范围: %d - %d\n", currentBlock, toBlock)
		
		// 构造日志查询参数
		logFilter := map[string]interface{}{
			"address":   usdtContract,
			"fromBlock": intToHex(currentBlock),
			"toBlock":   intToHex(toBlock),
			"topics": []interface{}{
				transferTopic,    // Transfer事件
				nil,              // from地址 (任意)
				addressTopic,     // to地址 (我们的目标地址)
			},
		}
		
		result, err := makeJSONRPCCall(endpoint, "eth_getLogs", []interface{}{logFilter})
		if err != nil {
			fmt.Printf("⚠️  区块 %d-%d 查询失败: %v\n", currentBlock, toBlock, err)
			
			// 如果批次太大，尝试减小批次
			if strings.Contains(err.Error(), "limit exceeded") || strings.Contains(err.Error(), "exceed maximum") {
				batchSize = batchSize / 2
				if batchSize < 1000 {
					fmt.Printf("❌ 批次大小已降至最低，跳过此范围\n")
					currentBlock = toBlock + 1
					continue
				}
				fmt.Printf("🔄 减小批次大小至: %d\n", batchSize)
				continue
			}
			
			currentBlock = toBlock + 1
			continue
		}
		
		// 解析日志
		if result != nil {
			logsData, _ := json.Marshal(result)
			var logs []LogEntry
			if err := json.Unmarshal(logsData, &logs); err == nil {
				allLogs = append(allLogs, logs...)
				if len(logs) > 0 {
					fmt.Printf("✅ 找到 %d 条转账记录\n", len(logs))
				}
			}
		}
		
		currentBlock = toBlock + 1
		
		// 添加延迟避免请求过快
		time.Sleep(100 * time.Millisecond)
	}
	
	fmt.Printf("🎉 总计找到 %d 条USDT转账记录\n", len(allLogs))
	return allLogs, nil
}

func parseUSDTTransfer(log LogEntry) map[string]interface{} {
	// 解析Transfer事件数据
	// Transfer(address indexed from, address indexed to, uint256 value)
	// topics[0]: 事件签名
	// topics[1]: from地址
	// topics[2]: to地址  
	// data: 转账金额
	
	if len(log.Topics) < 3 {
		return nil
	}
	
	fromAddress := "0x" + strings.TrimPrefix(log.Topics[1], "0x000000000000000000000000")
	toAddress := "0x" + strings.TrimPrefix(log.Topics[2], "0x000000000000000000000000")
	
	// 解析转账金额 (USDT有6位小数)
	valueHex := strings.TrimPrefix(log.Data, "0x")
	if valueHex == "" {
		valueHex = "0"
	}
	
	valueInt := hexToInt("0x" + valueHex)
	valueFinal := float64(valueInt) / 1000000 // USDT有6位小数
	
	blockNumber := hexToInt(log.BlockNumber)
	
	return map[string]interface{}{
		"from":             fromAddress,
		"to":               toAddress,
		"value":            fmt.Sprintf("%.6f", valueFinal),
		"blockNumber":      blockNumber,
		"transactionHash":  log.TransactionHash,
		"contractAddress":  log.Address,
	}
}

func main() {
	// BSC配置
	endpoint := "https://bsc-dataseed.binance.org/"
	usdtContract := "0x55d398326f99059fF775485246999027B3197955"
	address := "0xb5bc9cf309320abe924f38b13ec5c519ae04dbc5"
	fromBlock := int64(46962026)
	
	fmt.Println("🚀 BSC USDT转账查询实现")
	fmt.Println(strings.Repeat("=", 60))
	
	// 查询USDT转账记录
	transfers, err := getUSDTTransfers(endpoint, usdtContract, address, fromBlock)
	if err != nil {
		fmt.Printf("❌ 查询失败: %v\n", err)
		return
	}
	
	if len(transfers) == 0 {
		fmt.Println("ℹ️  未找到USDT转账记录")
		return
	}
	
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("📋 转账记录详情:")
	fmt.Println(strings.Repeat("=", 60))
	
	for i, transfer := range transfers {
		parsed := parseUSDTTransfer(transfer)
		if parsed != nil {
			fmt.Printf("\n🔸 转账 %d:\n", i+1)
			fmt.Printf("   📤 发送方: %s\n", parsed["from"])
			fmt.Printf("   📥 接收方: %s\n", parsed["to"])
			fmt.Printf("   💰 金额: %s USDT\n", parsed["value"])
			fmt.Printf("   📦 区块: %v\n", parsed["blockNumber"])
			fmt.Printf("   🔗 交易哈希: %s\n", parsed["transactionHash"])
		}
	}
	
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("✅ 查询完成")
	fmt.Println("💡 这个实现可以替代Etherscan API用于BSC USDT监控")
}