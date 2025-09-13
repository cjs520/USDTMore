package testutils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/shopspring/decimal"
)

// ChainType 区块链类型 (using string constants from factories.go)

// MockTransaction 模拟区块链交易数据
type MockTransaction struct {
	Hash        string          `json:"hash"`
	From        string          `json:"from"`
	To          string          `json:"to"`
	Amount      decimal.Decimal `json:"amount"`
	Chain       string          `json:"chain"`
	Timestamp   time.Time       `json:"timestamp"`
	BlockHeight int64           `json:"block_height"`
	Status      string          `json:"status"` // "success", "pending", "failed"
	GasUsed     int64           `json:"gas_used"`
	GasPrice    decimal.Decimal `json:"gas_price"`
}

// MockBlockchainData 区块链数据生成器
type MockBlockchainData struct {
	transactions []MockTransaction
	addresses    map[string][]string
}

// NewMockBlockchainData 创建新的区块链数据生成器
func NewMockBlockchainData() *MockBlockchainData {
	return &MockBlockchainData{
		transactions: make([]MockTransaction, 0),
		addresses: map[string][]string{
			ChainTRON:     generateTRONAddresses(10),
			ChainBSC:      generateBSCAddresses(10),
			ChainPOLY:  generatePolygonAddresses(10),
			ChainOP:    generateOptimismAddresses(10),
		},
	}
}

// GenerateTransaction 生成模拟交易数据
func (m *MockBlockchainData) GenerateTransaction(chain string, to string, amount decimal.Decimal) MockTransaction {
	tx := MockTransaction{
		Hash:        GenerateTransactionHash(chain),
		From:        m.getRandomAddress(chain),
		To:          to,
		Amount:      amount,
		Chain:       chain,
		Timestamp:   time.Now(),
		BlockHeight: generateBlockHeight(),
		Status:      "success",
		GasUsed:     generateGasUsed(chain),
		GasPrice:    generateGasPrice(chain),
	}
	
	m.transactions = append(m.transactions, tx)
	return tx
}

// GeneratePendingTransaction 生成待确认的交易
func (m *MockBlockchainData) GeneratePendingTransaction(chain string, to string, amount decimal.Decimal) MockTransaction {
	tx := m.GenerateTransaction(chain, to, amount)
	tx.Status = "pending"
	tx.BlockHeight = 0 // 待确认交易没有区块高度
	return tx
}

// GenerateFailedTransaction 生成失败的交易
func (m *MockBlockchainData) GenerateFailedTransaction(chain string, to string, amount decimal.Decimal) MockTransaction {
	tx := m.GenerateTransaction(chain, to, amount)
	tx.Status = "failed"
	return tx
}

// ConfirmTransaction 确认待确认的交易
func (m *MockBlockchainData) ConfirmTransaction(hash string) bool {
	for i, tx := range m.transactions {
		if tx.Hash == hash && tx.Status == "pending" {
			m.transactions[i].Status = "success"
			m.transactions[i].BlockHeight = generateBlockHeight()
			m.transactions[i].Timestamp = time.Now()
			return true
		}
	}
	return false
}

// GetTransactionByHash 根据哈希获取交易
func (m *MockBlockchainData) GetTransactionByHash(hash string) (MockTransaction, bool) {
	for _, tx := range m.transactions {
		if tx.Hash == hash {
			return tx, true
		}
	}
	return MockTransaction{}, false
}

// GetTransactionsByAddress 获取地址的所有交易
func (m *MockBlockchainData) GetTransactionsByAddress(address string, chain string) []MockTransaction {
	var result []MockTransaction
	for _, tx := range m.transactions {
		if (tx.From == address || tx.To == address) && tx.Chain == chain {
			result = append(result, tx)
		}
	}
	return result
}

// GetRandomAddress 获取随机地址
func (m *MockBlockchainData) getRandomAddress(chain string) string {
	addresses := m.addresses[chain]
	if len(addresses) == 0 {
		return GenerateRandomAddress(chain)
	}
	
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(addresses))))
	return addresses[n.Int64()]
}

// GenerateMultipleTransactions 生成多笔交易用于压力测试
func (m *MockBlockchainData) GenerateMultipleTransactions(count int, chain string, toAddress string, baseAmount decimal.Decimal) []MockTransaction {
	var transactions []MockTransaction
	
	for i := 0; i < count; i++ {
		// 在基础金额基础上随机变动
		variation := decimal.NewFromFloat(float64(i) * 0.01) // 每笔增加0.01
		amount := baseAmount.Add(variation)
		
		tx := m.GenerateTransaction(chain, toAddress, amount)
		transactions = append(transactions, tx)
		
		// 模拟交易时间间隔
		time.Sleep(time.Millisecond * 10)
	}
	
	return transactions
}

// SimulateNetworkDelay 模拟网络延迟
func (m *MockBlockchainData) SimulateNetworkDelay() {
	// 随机延迟 100-500ms
	delay := 100 + (time.Now().UnixNano()%400)*int64(time.Millisecond)
	time.Sleep(time.Duration(delay))
}

// 地址生成函数
func generateTRONAddresses(count int) []string {
	addresses := make([]string, count)
	for i := 0; i < count; i++ {
		addresses[i] = fmt.Sprintf("TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t") // USDT TRC20 地址格式
	}
	return addresses
}

func generateBSCAddresses(count int) []string {
	addresses := make([]string, count)
	for i := 0; i < count; i++ {
		addresses[i] = fmt.Sprintf("0x55d398326f99059ff775485246999027b3197955") // USDT BEP20 地址格式
	}
	return addresses
}

func generatePolygonAddresses(count int) []string {
	addresses := make([]string, count)
	for i := 0; i < count; i++ {
		addresses[i] = fmt.Sprintf("0xc2132D05D31c914a87C6611C10748AEb04B58e8F") // USDT Polygon 地址格式
	}
	return addresses
}

func generateOptimismAddresses(count int) []string {
	addresses := make([]string, count)
	for i := 0; i < count; i++ {
		addresses[i] = fmt.Sprintf("0x94b008aA00579c1307B0EF2c499aD98a8ce58e58") // USDT Optimism 地址格式
	}
	return addresses
}

// These functions are now defined in factories.go to avoid duplication

// generateBlockHeight 生成区块高度
func generateBlockHeight() int64 {
	base := int64(50000000) // 基础区块高度
	variation, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return base + variation.Int64()
}

// generateGasUsed 生成Gas使用量
func generateGasUsed(chain string) int64 {
	switch chain {
	case ChainTRON:
		return 15000 + (time.Now().UnixNano() % 5000) // TRON 能量消耗
	case ChainBSC:
		return 21000 + (time.Now().UnixNano() % 10000) // BSC Gas
	case ChainPOLY:
		return 21000 + (time.Now().UnixNano() % 8000) // Polygon Gas
	case ChainOP:
		return 21000 + (time.Now().UnixNano() % 12000) // Optimism Gas
	default:
		return 21000
	}
}

// generateGasPrice 生成Gas价格
func generateGasPrice(chain string) decimal.Decimal {
	switch chain {
	case ChainTRON:
		return decimal.NewFromInt(0) // TRON 不使用Gas价格概念
	case ChainBSC:
		base := 5.0 + float64(time.Now().UnixNano()%10) // 5-15 Gwei
		return decimal.NewFromFloat(base)
	case ChainPOLY:
		base := 30.0 + float64(time.Now().UnixNano()%20) // 30-50 Gwei
		return decimal.NewFromFloat(base)
	case ChainOP:
		base := 0.1 + float64(time.Now().UnixNano()%5)/10.0 // 0.1-0.6 Gwei
		return decimal.NewFromFloat(base)
	default:
		return decimal.NewFromFloat(10.0)
	}
}

// TestBlockchainScenarios 测试场景生成器
type TestBlockchainScenarios struct {
	blockchain *MockBlockchainData
}

// NewTestBlockchainScenarios 创建测试场景生成器
func NewTestBlockchainScenarios() *TestBlockchainScenarios {
	return &TestBlockchainScenarios{
		blockchain: NewMockBlockchainData(),
	}
}

// ScenarioNormalPayment 正常支付场景
func (s *TestBlockchainScenarios) ScenarioNormalPayment(chain string, toAddress string, amount decimal.Decimal) MockTransaction {
	return s.blockchain.GenerateTransaction(chain, toAddress, amount)
}

// ScenarioDelayedPayment 延迟支付场景
func (s *TestBlockchainScenarios) ScenarioDelayedPayment(chain string, toAddress string, amount decimal.Decimal) MockTransaction {
	tx := s.blockchain.GeneratePendingTransaction(chain, toAddress, amount)
	
	// 模拟延迟确认
	go func() {
		time.Sleep(time.Second * 30) // 30秒后确认
		s.blockchain.ConfirmTransaction(tx.Hash)
	}()
	
	return tx
}

// ScenarioPartialPayment 部分支付场景
func (s *TestBlockchainScenarios) ScenarioPartialPayment(chain string, toAddress string, expectedAmount decimal.Decimal) MockTransaction {
	// 支付金额少于预期
	actualAmount := expectedAmount.Mul(decimal.NewFromFloat(0.8)) // 80%的金额
	return s.blockchain.GenerateTransaction(chain, toAddress, actualAmount)
}

// ScenarioOverpayment 超额支付场景
func (s *TestBlockchainScenarios) ScenarioOverpayment(chain string, toAddress string, expectedAmount decimal.Decimal) MockTransaction {
	// 支付金额多于预期
	actualAmount := expectedAmount.Mul(decimal.NewFromFloat(1.2)) // 120%的金额
	return s.blockchain.GenerateTransaction(chain, toAddress, actualAmount)
}

// ScenarioFailedPayment 支付失败场景
func (s *TestBlockchainScenarios) ScenarioFailedPayment(chain string, toAddress string, amount decimal.Decimal) MockTransaction {
	return s.blockchain.GenerateFailedTransaction(chain, toAddress, amount)
}

// ScenarioConcurrentPayments 并发支付场景
func (s *TestBlockchainScenarios) ScenarioConcurrentPayments(chain string, toAddress string, baseAmount decimal.Decimal, count int) []MockTransaction {
	return s.blockchain.GenerateMultipleTransactions(count, chain, toAddress, baseAmount)
}

// ScenarioNetworkCongestion 网络拥堵场景
func (s *TestBlockchainScenarios) ScenarioNetworkCongestion(chain string, toAddress string, amount decimal.Decimal) MockTransaction {
	// 模拟网络拥堵，高Gas价格，长确认时间
	tx := s.blockchain.GenerateTransaction(chain, toAddress, amount)
	tx.GasPrice = tx.GasPrice.Mul(decimal.NewFromFloat(3.0)) // 3倍Gas价格
	tx.Status = "pending"
	tx.BlockHeight = 0
	
	// 模拟长时间确认
	go func() {
		time.Sleep(time.Minute * 5) // 5分钟后确认
		s.blockchain.ConfirmTransaction(tx.Hash)
	}()
	
	return tx
}

// GetBlockchain 获取区块链数据生成器
func (s *TestBlockchainScenarios) GetBlockchain() *MockBlockchainData {
	return s.blockchain
}

// MockExchangeRate 模拟汇率数据
type MockExchangeRate struct {
	rate      float64
	variation float64 // 波动范围
}

// NewMockExchangeRate 创建模拟汇率
func NewMockExchangeRate(baseRate float64, variation float64) *MockExchangeRate {
	return &MockExchangeRate{
		rate:      baseRate,
		variation: variation,
	}
}

// GetCurrentRate 获取当前汇率
func (m *MockExchangeRate) GetCurrentRate() float64 {
	// 添加随机波动
	variationRange := m.rate * m.variation
	randomVariation := (float64(time.Now().UnixNano()%1000) / 1000.0 - 0.5) * 2 * variationRange
	return m.rate + randomVariation
}

// SimulateRateVolatility 模拟汇率波动
func (m *MockExchangeRate) SimulateRateVolatility(duration time.Duration) {
	ticker := time.NewTicker(time.Second)
	timeout := time.After(duration)
	
	for {
		select {
		case <-ticker.C:
			// 每秒更新汇率
			change := (float64(time.Now().UnixNano()%100) / 1000.0 - 0.05) * m.rate
			m.rate += change
			
			// 确保汇率不会过于偏离合理范围
			if m.rate < 6.0 {
				m.rate = 6.0
			}
			if m.rate > 8.0 {
				m.rate = 8.0
			}
			
		case <-timeout:
			ticker.Stop()
			return
		}
	}
}