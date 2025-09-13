package tests

import (
	"USDTMore/app/model"
	"sync"
	"testing"
)

// 模拟旧的全局锁版本的CalcTradeAmount（用于对比）
var oldCalcMutex sync.Mutex

func calcTradeAmountOld(wa []model.WalletAddress, rate, money float64) (model.WalletAddress, string) {
	oldCalcMutex.Lock()
	defer oldCalcMutex.Unlock()
	
	// 简化的旧版本逻辑
	if len(wa) > 0 {
		return wa[0], "13.89" // 固定返回值用于测试
	}
	return model.WalletAddress{}, "0"
}

// BenchmarkCalcTradeAmountOld 测试旧版本（全局锁）的性能
func BenchmarkCalcTradeAmountOld(b *testing.B) {
	wallets := []model.WalletAddress{
		{Chain: "TRON", Address: "TAddress1234567890123456789012345"},
		{Chain: "TRON", Address: "TAddress2345678901234567890123456"},
	}
	rate := 7.20
	money := 100.0

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			calcTradeAmountOld(wallets, rate, money)
		}
	})
}

// BenchmarkCalcTradeAmountNew 测试新版本（无锁）的性能
func BenchmarkCalcTradeAmountNew(b *testing.B) {
	wallets := []model.WalletAddress{
		{Chain: "TRON", Address: "TAddress1234567890123456789012345"},
		{Chain: "TRON", Address: "TAddress2345678901234567890123456"},
	}
	rate := 7.20
	money := 100.0

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			model.CalcTradeAmount(wallets, rate, money)
		}
	})
}

// BenchmarkConcurrentOrderProcessing 测试并发订单处理性能
func BenchmarkConcurrentOrderProcessing(b *testing.B) {
	wallets := []model.WalletAddress{
		{Chain: "TRON", Address: "TAddress1234567890123456789012345"},
		{Chain: "TRON", Address: "TAddress2345678901234567890123456"},
		{Chain: "BSC", Address: "0x1234567890123456789012345678901234567890"},
		{Chain: "POLY", Address: "0x2345678901234567890123456789012345678901"},
	}
	
	rates := []float64{7.20, 7.25, 7.30}
	amounts := []float64{50.0, 100.0, 200.0, 500.0}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			rate := rates[i%len(rates)]
			amount := amounts[i%len(amounts)]
			model.CalcTradeAmount(wallets, rate, amount)
			i++
		}
	})
}