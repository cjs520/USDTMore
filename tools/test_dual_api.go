package main

import (
	"fmt"
	"time"
)

// 模拟双重API查询逻辑
func testDualAPIQuery() {
	fmt.Println("=== 双重API查询机制测试 ===")
	fmt.Println()
	
	// 模拟tokentx查询
	fmt.Println("1. 尝试tokentx查询 (ERC-20代币转账)")
	fmt.Println("   API: action=tokentx&contractaddress=USDT&address=TARGET")
	
	// 模拟查询失败的情况
	tokentxSuccess := false // 假设tokentx查询失败
	
	if tokentxSuccess {
		fmt.Println("   ✅ tokentx查询成功 - 找到USDT转账记录")
	} else {
		fmt.Println("   ❌ tokentx查询失败 - 可能原因：")
		fmt.Println("      - API Key问题")
		fmt.Println("      - 参数格式错误")
		fmt.Println("      - 区块范围问题")
		fmt.Println("      - USDT转账通过智能合约进行")
		fmt.Println()
		
		fmt.Println("2. 启用txlistinternal备用查询 (内部交易)")
		fmt.Println("   API: action=txlistinternal&address=TARGET")
		
		// 模拟备用查询
		internalSuccess := true // 假设内部交易查询成功
		
		if internalSuccess {
			fmt.Println("   ✅ txlistinternal查询成功 - 找到内部交易记录")
			fmt.Println("   📋 可能包含的交易类型：")
			fmt.Println("      - 智能合约调用产生的USDT转账")
			fmt.Println("      - DeFi协议交互")
			fmt.Println("      - 多签钱包转账")
			fmt.Println("      - 代理合约转账")
		} else {
			fmt.Println("   ❌ txlistinternal查询也失败")
		}
	}
	
	fmt.Println()
	fmt.Println("=== 查询策略总结 ===")
	fmt.Println("✅ 优势：")
	fmt.Println("   - 双重保障，提高交易发现率")
	fmt.Println("   - tokentx专门查询代币转账")
	fmt.Println("   - txlistinternal捕获智能合约转账")
	fmt.Println("   - 自动降级，提高可靠性")
	fmt.Println()
	fmt.Println("⚠️  注意事项：")
	fmt.Println("   - 消耗更多API调用配额")
	fmt.Println("   - 需要处理不同API的响应格式")
	fmt.Println("   - 可能有重复交易需要去重")
}

func main() {
	testDualAPIQuery()
	
	fmt.Println()
	fmt.Println("=== 实际应用建议 ===")
	fmt.Printf("当前时间: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("1. 监控日志中查看 'tokentx查询失败，尝试txlistinternal备用查询' 消息")
	fmt.Println("2. 如果经常需要备用查询，考虑调整API参数或Key")
	fmt.Println("3. 监控API调用频率，避免超出限制")
	fmt.Println("4. 定期检查两种API的响应格式是否一致")
}