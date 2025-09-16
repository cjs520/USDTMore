package main

import (
	"USDTMore/app/help"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("用法: go run validate_address.go <地址>")
		fmt.Println("示例:")
		fmt.Println("  go run validate_address.go TRON:TLBaRhANQoJFTqre9Nf1mjuwNWjCJeYqUL")
		fmt.Println("  go run validate_address.go BSC:0x1234567890123456789012345678901234567890")
		os.Exit(1)
	}

	address := os.Args[1]
	fmt.Printf("验证地址: %s\n", address)

	// 测试各种地址格式
	tests := []struct {
		name string
		fn   func(string) bool
	}{
		{"TRON", help.IsValidTRONWalletAddress},
		{"Polygon", help.IsValidPOLWalletAddress},
		{"Optimism", help.IsValidOPTWalletAddress},
		{"BSC", help.IsValidBSCWalletAddress},
		{"Arbitrum", help.IsValidARBWalletAddress},
		{"X-Layer", help.IsValidXLAYERWalletAddress},
		{"Solana", help.IsValidSOLWalletAddress},
		{"Aptos", help.IsValidAPTWalletAddress},
	}

	valid := false
	for _, test := range tests {
		if test.fn(address) {
			fmt.Printf("✅ 有效的 %s 地址\n", test.name)
			valid = true
			break
		}
	}

	if !valid {
		fmt.Println("❌ 无效的地址格式")
		fmt.Println("\n支持的格式:")
		fmt.Println("- TRON:TLBaRhANQoJFTqre9Nf1mjuwNWjCJeYqUL")
		fmt.Println("- POLY:0x1234567890123456789012345678901234567890")
		fmt.Println("- BSC:0x1234567890123456789012345678901234567890")
		fmt.Println("- OP:0x1234567890123456789012345678901234567890")
		fmt.Println("- ARB:0x1234567890123456789012345678901234567890")
		fmt.Println("- XLAYER:0x1234567890123456789012345678901234567890")
		fmt.Println("- SOL:11111111111111111111111111111111")
		fmt.Println("- APT:0x1234567890123456789012345678901234567890123456789012345678901234")
		os.Exit(1)
	}
}