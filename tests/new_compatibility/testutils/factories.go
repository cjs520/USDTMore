package testutils

import (
	"USDTMore/app/model"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CreateTestOrder 创建测试订单
func CreateTestOrder(customFields ...map[string]interface{}) *model.TradeOrders {
	order := &model.TradeOrders{
		OrderId:     fmt.Sprintf("TEST_%s", uuid.New().String()[:8]),
		TradeId:     fmt.Sprintf("TID_%s", uuid.New().String()[:8]),
		TradeHash:   "",
		UsdtRate:    func() decimal.Decimal { d, _ := decimal.NewFromString("7.20"); return d }(),
		Amount:      func() decimal.Decimal { d, _ := decimal.NewFromString("100.00"); return d }(),
		Money:       decimal.NewFromFloat(720.00),
		Chain:       "TRON",
		Address:     "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		FromAddress: "",
		Status:      model.OrderStatusWaiting,
		ReturnUrl:   "https://example.com/return",
		NotifyUrl:   "https://example.com/notify",
		NotifyNum:   0,
		NotifyState: model.OrderNotifyStateFail,
		ExpiredAt:   time.Now().Add(30 * time.Minute),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 应用自定义字段
	if len(customFields) > 0 {
		fields := customFields[0]
		if v, ok := fields["order_id"].(string); ok {
			order.OrderId = v
		}
		if v, ok := fields["trade_id"].(string); ok {
			order.TradeId = v
		}
		if v, ok := fields["trade_hash"].(string); ok {
			order.TradeHash = v
		}
		if v, ok := fields["amount"].(string); ok {
			if amount, err := decimal.NewFromString(v); err == nil {
				order.Amount = amount
			}
		}
		if v, ok := fields["money"].(float64); ok {
			order.Money = decimal.NewFromFloat(v)
		}
		if v, ok := fields["chain"].(string); ok {
			order.Chain = v
		}
		if v, ok := fields["address"].(string); ok {
			order.Address = v
		}
		if v, ok := fields["from_address"].(string); ok {
			order.FromAddress = v
		}
		if v, ok := fields["status"].(int); ok {
			order.Status = int16(v)
		}
		if v, ok := fields["notify_url"].(string); ok {
			order.NotifyUrl = v
		}
		if v, ok := fields["return_url"].(string); ok {
			order.ReturnUrl = v
		}
		if v, ok := fields["notify_num"].(int); ok {
			order.NotifyNum = int16(v)
		}
		if v, ok := fields["notify_state"].(int); ok {
			order.NotifyState = int16(v)
		}
		if v, ok := fields["expired_at"].(time.Time); ok {
			order.ExpiredAt = v
		}
		if v, ok := fields["created_at"].(time.Time); ok {
			order.CreatedAt = v
		}
		if v, ok := fields["confirmed_at"].(time.Time); ok {
			order.ConfirmedAt = &v
		}
	}

	return order
}

// CreateTestWalletAddress 创建测试钱包地址
func CreateTestWalletAddress(chain string, address string) *model.WalletAddress {
	return &model.WalletAddress{
		Chain:       chain,
		Address:     address,
		StartBlock:  0,
		InAmount:    decimal.Zero,
		OutAmount:   decimal.Zero,
		Count:       0,
		Status:      1, // 启用状态
		OtherNotify: 1, // 启用通知
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// CreateTestTransactionData 创建模拟交易数据
func CreateTestTransactionData(chain string, toAddress string, amount decimal.Decimal, txHash string, timestamp time.Time) map[string]interface{} {
	switch chain {
	case "TRON":
		return map[string]interface{}{
			"token_transfers": []map[string]interface{}{
				{
					"transaction_id": txHash,
					"to_address":     toAddress,
					"from_address":   "TTestFromAddress12345678901234567890",
					"quant":          amount.Mul(decimal.NewFromFloat(1000000)).IntPart(), // TRC20 USDT has 6 decimals
					"contractRet":    "SUCCESS",
					"block_ts":       timestamp.UnixMilli(),
				},
			},
			"total": 1,
		}
	case "POLY", "OP", "BSC", "ARB":
		return map[string]interface{}{
			"result": []map[string]interface{}{
				{
					"hash":             txHash,
					"to":               toAddress,
					"from":             "0x1234567890123456789012345678901234567890",
					"value":            amount.Mul(decimal.NewFromFloat(1000000)).String(), // 6 decimals
					"tokenSymbol":      "USDT",
					"tokenDecimal":     "6",
					"contractAddress":  "0xdAC17F958D2ee523a2206206994597C13D831ec7",
					"timeStamp":        fmt.Sprintf("%d", timestamp.Unix()),
				},
			},
		}
	default:
		return map[string]interface{}{}
	}
}

// CreateExpiredOrder 创建过期订单
func CreateExpiredOrder() *model.TradeOrders {
	return CreateTestOrder(map[string]interface{}{
		"expired_at": time.Now().Add(-1 * time.Hour), // 1小时前过期
		"created_at": time.Now().Add(-2 * time.Hour), // 2小时前创建
	})
}

// CreateSuccessOrder 创建成功订单
func CreateSuccessOrder() *model.TradeOrders {
	return CreateTestOrder(map[string]interface{}{
		"status":       model.OrderStatusSuccess,
		"trade_hash":   "0x123456789abcdef",
		"from_address": "TTestFromAddress12345678901234567890",
	})
}

// CreatePendingOrder 创建待支付订单
func CreatePendingOrder() *model.TradeOrders {
	return CreateTestOrder(map[string]interface{}{
		"status": model.OrderStatusWaiting,
	})
}

// CreateFailedNotifyOrder 创建回调失败的订单
func CreateFailedNotifyOrder() *model.TradeOrders {
	return CreateTestOrder(map[string]interface{}{
		"status":       model.OrderStatusSuccess,
		"notify_num":   3,
		"notify_state": model.OrderNotifyStateFail,
		"trade_hash":   "0x123456789abcdef",
	})
}

// CreateOrderBatch 批量创建测试订单
func CreateOrderBatch(count int, customFields ...map[string]interface{}) []*model.TradeOrders {
	orders := make([]*model.TradeOrders, count)
	baseFields := make(map[string]interface{})
	
	if len(customFields) > 0 {
		baseFields = customFields[0]
	}
	
	for i := 0; i < count; i++ {
		// 为每个订单创建独立的字段副本
		fields := make(map[string]interface{})
		for k, v := range baseFields {
			fields[k] = v
		}
		
		// 确保每个订单有唯一的ID
		if _, exists := fields["order_id"]; !exists {
			fields["order_id"] = fmt.Sprintf("BATCH_%d_%s", i, uuid.New().String()[:8])
		}
		if _, exists := fields["trade_id"]; !exists {
			fields["trade_id"] = fmt.Sprintf("TID_%d_%s", i, uuid.New().String()[:8])
		}
		
		orders[i] = CreateTestOrder(fields)
	}
	
	return orders
}

// CreateMultiChainOrders 创建多链测试订单
func CreateMultiChainOrders() []*model.TradeOrders {
	chains := []string{"TRON", "BSC", "POLY", "OP"}
	addresses := map[string]string{
		"TRON": "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
		"BSC":  "0x55d398326f99059ff775485246999027b3197955",
		"POLY": "0xc2132D05D31c914a87C6611C10748AEb04B58e8F",
		"OP":   "0x94b008aA00579c1307B0EF2c499aD98a8ce58e58",
	}
	
	orders := make([]*model.TradeOrders, len(chains))
	
	for i, chain := range chains {
		orders[i] = CreateTestOrder(map[string]interface{}{
			"chain":   chain,
			"address": addresses[chain],
		})
	}
	
	return orders
}

// CreateOrderWithRandomAmount 创建随机金额的测试订单
func CreateOrderWithRandomAmount(minAmount, maxAmount float64) *model.TradeOrders {
	amount := minAmount + rand.Float64()*(maxAmount-minAmount)
	rate := 7.0 + rand.Float64()*2.0 // 7.0-9.0 的汇率
	usdtAmount := amount / rate
	
	return CreateTestOrder(map[string]interface{}{
		"money":  amount,
		"amount": fmt.Sprintf("%.2f", usdtAmount),
	})
}

// CreateConcurrentTestOrders 创建并发测试用的订单
func CreateConcurrentTestOrders(count int, baseAmount float64) []*model.TradeOrders {
	orders := make([]*model.TradeOrders, count)
	chains := []string{"TRON", "BSC", "POLY", "OP"}
	
	for i := 0; i < count; i++ {
		chain := chains[i%len(chains)]
		amount := baseAmount + float64(i)*0.01 // 每个订单金额略有不同，避免金额冲突
		
		orders[i] = CreateTestOrder(map[string]interface{}{
			"chain": chain,
			"money": amount,
			"order_id": fmt.Sprintf("CONCURRENT_%d_%d", i, time.Now().UnixNano()),
			"trade_id": fmt.Sprintf("TID_%d_%d", i, time.Now().UnixNano()),
		})
		
		// 微小延迟确保时间戳不同
		time.Sleep(time.Nanosecond)
	}
	
	return orders
}

// CreateOrderLifecycleTest 创建订单生命周期测试数据
func CreateOrderLifecycleTest() *OrderLifecycleTestData {
	baseOrder := CreateTestOrder()
	
	return &OrderLifecycleTestData{
		OrderData:     baseOrder,
		PaymentAmount: "100.00",
		TxHash:        GenerateTransactionHash(ChainTRON),
		FromAddress:   GenerateRandomAddress(ChainTRON),
		Chain:         baseOrder.Chain,
		ExpectedCallbacks: []string{baseOrder.NotifyUrl},
	}
}

// OrderLifecycleTestData 订单生命周期测试数据
type OrderLifecycleTestData struct {
	OrderData         *model.TradeOrders
	PaymentAmount     string
	TxHash            string
	FromAddress       string
	Chain             string
	ExpectedCallbacks []string
}

// CreateNotifyRecord 创建通知记录
func CreateNotifyRecord(txid string) *model.NotifyRecord {
	return &model.NotifyRecord{
		Txid:      txid,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// CreateFailedNotifyRecord 创建失败的通知记录
func CreateFailedNotifyRecord(txid string) *model.NotifyRecord {
	return &model.NotifyRecord{
		Txid:      txid,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// CreateStressTestOrders 创建压力测试订单
func CreateStressTestOrders(orderCount, batchSize int) [][]*model.TradeOrders {
	totalBatches := (orderCount + batchSize - 1) / batchSize
	batches := make([][]*model.TradeOrders, totalBatches)
	
	for i := 0; i < totalBatches; i++ {
		currentBatchSize := batchSize
		if i == totalBatches-1 {
			// 最后一批可能数量不足
			currentBatchSize = orderCount - i*batchSize
		}
		
		batches[i] = CreateConcurrentTestOrders(currentBatchSize, 100.0)
	}
	
	return batches
}

// CreateAPITestData 创建API测试数据
type APITestData struct {
	CreateOrderRequest map[string]interface{}
	ExpectedResponse   map[string]interface{}
	InvalidRequests    []map[string]interface{}
}

// CreateOrderAPITestData 创建订单API测试数据
func CreateOrderAPITestData() *APITestData {
	validRequest := map[string]interface{}{
		"order_id":     fmt.Sprintf("API_TEST_%s", uuid.New().String()[:8]),
		"amount":       100.0,
		"code":         "TRON",
		"notify_url":   "https://example.com/notify",
		"redirect_url": "https://example.com/return",
	}
	
	expectedResponse := map[string]interface{}{
		"code": 200,
		"msg":  "success",
		"data": map[string]interface{}{
			"trade_id":        "",  // 动态生成
			"order_id":        validRequest["order_id"],
			"amount":          validRequest["amount"],
			"actual_amount":   "", // 动态计算
			"token":           "", // 动态分配
			"expiration_time": 0,  // 动态计算
			"payment_url":     "", // 动态生成
		},
	}
	
	invalidRequests := []map[string]interface{}{
		// 缺少必要参数
		{"order_id": "TEST001", "amount": 100.0},
		// 金额无效
		{"order_id": "TEST002", "amount": -100.0, "code": "TRON", "notify_url": "https://example.com/notify", "redirect_url": "https://example.com/return"},
		// 订单ID重复
		{"order_id": "DUPLICATE", "amount": 100.0, "code": "TRON", "notify_url": "https://example.com/notify", "redirect_url": "https://example.com/return"},
		// 无效的链类型
		{"order_id": "TEST003", "amount": 100.0, "code": "INVALID_CHAIN", "notify_url": "https://example.com/notify", "redirect_url": "https://example.com/return"},
		// 无效的URL格式
		{"order_id": "TEST004", "amount": 100.0, "code": "TRON", "notify_url": "invalid-url", "redirect_url": "https://example.com/return"},
	}
	
	return &APITestData{
		CreateOrderRequest: validRequest,
		ExpectedResponse:   expectedResponse,
		InvalidRequests:    invalidRequests,
	}
}

// CreatePerformanceTestData 创建性能测试数据
type PerformanceTestData struct {
	OrderBatches     [][]*model.TradeOrders
	ConcurrentUsers  int
	RequestsPerUser  int
	TestDuration     time.Duration
	ExpectedTPS      int
}

// CreatePerformanceScenario 创建性能测试场景
func CreatePerformanceScenario(scenario string) *PerformanceTestData {
	switch scenario {
	case "light":
		return &PerformanceTestData{
			OrderBatches:    CreateStressTestOrders(100, 10),
			ConcurrentUsers: 5,
			RequestsPerUser: 20,
			TestDuration:    time.Minute * 1,
			ExpectedTPS:     10,
		}
	case "moderate":
		return &PerformanceTestData{
			OrderBatches:    CreateStressTestOrders(500, 25),
			ConcurrentUsers: 20,
			RequestsPerUser: 25,
			TestDuration:    time.Minute * 3,
			ExpectedTPS:     50,
		}
	case "heavy":
		return &PerformanceTestData{
			OrderBatches:    CreateStressTestOrders(1000, 50),
			ConcurrentUsers: 50,
			RequestsPerUser: 20,
			TestDuration:    time.Minute * 5,
			ExpectedTPS:     100,
		}
	default:
		return CreatePerformanceScenario("light")
	}
}

// CreateDataConsistencyTestSuite 创建数据一致性测试套件
type DataConsistencyTestSuite struct {
	BaseOrders          []*model.TradeOrders
	ConcurrentUpdates   []OrderUpdateOperation
	ExpectedFinalStates []OrderExpectedState
}

// OrderUpdateOperation 订单更新操作
type OrderUpdateOperation struct {
	OrderID   string
	Operation string // "pay", "expire", "callback", "cancel"
	Data      map[string]interface{}
	Delay     time.Duration
}

// OrderExpectedState 订单预期状态
type OrderExpectedState struct {
	OrderID         string
	ExpectedStatus  int
	ExpectedCallbacks int
	ShouldHaveTxHash bool
}

// CreateDataConsistencyTestSuite 创建数据一致性测试套件
func CreateDataConsistencyTests() *DataConsistencyTestSuite {
	baseOrders := CreateOrderBatch(10)
	
	operations := make([]OrderUpdateOperation, 0)
	expectedStates := make([]OrderExpectedState, 0)
	
	for i, order := range baseOrders {
		switch i % 4 {
		case 0: // 正常支付
			operations = append(operations, OrderUpdateOperation{
				OrderID:   order.OrderId,
				Operation: "pay",
				Data: map[string]interface{}{
					"tx_hash":      GenerateTransactionHash(ChainTRON),
					"from_address": GenerateRandomAddress(ChainTRON),
				},
				Delay: time.Duration(i) * time.Millisecond * 100,
			})
			expectedStates = append(expectedStates, OrderExpectedState{
				OrderID:           order.OrderId,
				ExpectedStatus:    model.OrderStatusSuccess,
				ExpectedCallbacks: 1,
				ShouldHaveTxHash:  true,
			})
			
		case 1: // 订单过期
			operations = append(operations, OrderUpdateOperation{
				OrderID:   order.OrderId,
				Operation: "expire",
				Data:      map[string]interface{}{},
				Delay:     time.Duration(i) * time.Millisecond * 50,
			})
			expectedStates = append(expectedStates, OrderExpectedState{
				OrderID:           order.OrderId,
				ExpectedStatus:    model.OrderStatusExpired,
				ExpectedCallbacks: 0,
				ShouldHaveTxHash:  false,
			})
			
		case 2: // 并发支付尝试
			operations = append(operations, 
				OrderUpdateOperation{
					OrderID:   order.OrderId,
					Operation: "pay",
					Data: map[string]interface{}{
						"tx_hash":      GenerateTransactionHash(ChainTRON),
						"from_address": GenerateRandomAddress(ChainTRON),
					},
					Delay: time.Duration(i) * time.Millisecond * 10,
				},
				OrderUpdateOperation{
					OrderID:   order.OrderId,
					Operation: "pay",
					Data: map[string]interface{}{
						"tx_hash":      GenerateTransactionHash(ChainTRON),
						"from_address": GenerateRandomAddress(ChainTRON),
					},
					Delay: time.Duration(i) * time.Millisecond * 15, // 稍后执行
				},
			)
			expectedStates = append(expectedStates, OrderExpectedState{
				OrderID:           order.OrderId,
				ExpectedStatus:    model.OrderStatusSuccess,
				ExpectedCallbacks: 1, // 只应该有一次成功回调
				ShouldHaveTxHash:  true,
			})
			
		case 3: // 等待支付（无操作）
			expectedStates = append(expectedStates, OrderExpectedState{
				OrderID:           order.OrderId,
				ExpectedStatus:    model.OrderStatusWaiting,
				ExpectedCallbacks: 0,
				ShouldHaveTxHash:  false,
			})
		}
	}
	
	return &DataConsistencyTestSuite{
		BaseOrders:          baseOrders,
		ConcurrentUpdates:   operations,
		ExpectedFinalStates: expectedStates,
	}
}

// 内存监控和性能测试辅助函数

// GetMemoryUsage 获取当前内存使用量（字节）
func GetMemoryUsage() int64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return int64(memStats.Alloc)
}

// GetGoroutineCount 获取当前goroutine数量
func GetGoroutineCount() int64 {
	return int64(runtime.NumGoroutine())
}

// ForceGC 强制执行垃圾回收
func ForceGC() {
	runtime.GC()
	runtime.GC() // 执行两次确保完全回收
}

// SaveTestReport 保存测试报告到文件
func SaveTestReport(filename, content string) error {
	return ioutil.WriteFile(filename, []byte(content), 0644)
}

// 链和地址生成常量
const (
	ChainTRON    = "TRON"
	ChainBSC     = "BSC"
	ChainPOLY    = "POLY"
	ChainOP      = "OP"
	ChainETH     = "ETH"
	ChainARB     = "ARB"
)

// GenerateTransactionHash 生成随机交易哈希
func GenerateTransactionHash(chain string) string {
	switch chain {
	case ChainTRON:
		return fmt.Sprintf("tron_%s", generateRandomHex(32))
	case ChainBSC, ChainPOLY, ChainOP, ChainETH, ChainARB:
		return fmt.Sprintf("0x%s", generateRandomHex(64))
	default:
		return fmt.Sprintf("0x%s", generateRandomHex(64))
	}
}

// GenerateRandomAddress 生成随机地址
func GenerateRandomAddress(chain string) string {
	switch chain {
	case ChainTRON:
		return fmt.Sprintf("T%s", generateRandomBase58(33))
	case ChainBSC, ChainPOLY, ChainOP, ChainETH, ChainARB:
		return fmt.Sprintf("0x%s", generateRandomHex(40))
	default:
		return fmt.Sprintf("0x%s", generateRandomHex(40))
	}
}

// generateRandomHex 生成随机十六进制字符串
func generateRandomHex(length int) string {
	chars := "0123456789abcdef"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

// generateRandomBase58 生成随机Base58字符串
func generateRandomBase58(length int) string {
	chars := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	result := make([]byte, length)
	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}
	return string(result)
}

// 性能测试辅助函数

// PerformanceTestResult 性能测试结果
type PerformanceTestResult struct {
	TotalRequests     int64
	SuccessfulRequests int64
	FailedRequests    int64
	AverageLatency    time.Duration
	ThroughputPerSec  float64
	ErrorRate         float64
}

// CalculatePerformanceMetrics 计算性能指标
func CalculatePerformanceMetrics(totalRequests, successfulRequests int64, totalDuration time.Duration) PerformanceTestResult {
	failedRequests := totalRequests - successfulRequests
	errorRate := 0.0
	if totalRequests > 0 {
		errorRate = float64(failedRequests) / float64(totalRequests) * 100
	}
	
	averageLatency := time.Duration(0)
	if successfulRequests > 0 {
		averageLatency = totalDuration / time.Duration(successfulRequests)
	}
	
	throughputPerSec := 0.0
	if totalDuration.Seconds() > 0 {
		throughputPerSec = float64(successfulRequests) / totalDuration.Seconds()
	}
	
	return PerformanceTestResult{
		TotalRequests:      totalRequests,
		SuccessfulRequests: successfulRequests,
		FailedRequests:     failedRequests,
		AverageLatency:     averageLatency,
		ThroughputPerSec:   throughputPerSec,
		ErrorRate:          errorRate,
	}
}

// 系统资源监控

// SystemResourceUsage 系统资源使用情况
type SystemResourceUsage struct {
	MemoryUsage    int64   // 内存使用量（字节）
	GoroutineCount int64   // Goroutine数量
	CPUUsage       float64 // CPU使用率（百分比）
	Timestamp      time.Time
}

// GetSystemResourceUsage 获取系统资源使用情况
func GetSystemResourceUsage() SystemResourceUsage {
	return SystemResourceUsage{
		MemoryUsage:    GetMemoryUsage(),
		GoroutineCount: GetGoroutineCount(),
		CPUUsage:       getCPUUsage(),
		Timestamp:      time.Now(),
	}
}

// getCPUUsage 获取CPU使用率（简化实现）
func getCPUUsage() float64 {
	// 简化的CPU使用率检测
	// 在实际环境中，应该使用更精确的方法
	if runtime.GOOS == "linux" {
		content, err := ioutil.ReadFile("/proc/loadavg")
		if err == nil {
			parts := strings.Fields(string(content))
			if len(parts) > 0 {
				if load, err := strconv.ParseFloat(parts[0], 64); err == nil {
					return load * 100 / float64(runtime.NumCPU())
				}
			}
		}
	}
	return 0.0 // 无法获取时返回0
}

// 数据验证辅助函数

// ValidateOrderData 验证订单数据完整性
func ValidateOrderData(order *model.TradeOrders) []string {
	var errors []string
	
	if order.OrderId == "" {
		errors = append(errors, "OrderId is empty")
	}
	if order.TradeId == "" {
		errors = append(errors, "TradeId is empty")
	}
	if order.Amount.IsZero() {
		errors = append(errors, "Amount is zero")
	}
	if order.Chain == "" {
		errors = append(errors, "Chain is empty")
	}
	if order.Address == "" {
		errors = append(errors, "Address is empty")
	}
	if order.Money.LessThanOrEqual(decimal.Zero) {
		errors = append(errors, "Money must be positive")
	}
	if order.ExpiredAt.Before(time.Now()) {
		errors = append(errors, "ExpiredAt is in the past")
	}
	
	return errors
}

// CompareOrders 比较两个订单是否相等
func CompareOrders(order1, order2 *model.TradeOrders) bool {
	return order1.OrderId == order2.OrderId &&
		order1.TradeId == order2.TradeId &&
		order1.Amount.Equal(order2.Amount) &&
		order1.Chain == order2.Chain &&
		order1.Address == order2.Address &&
		order1.Status == order2.Status
}

// 测试环境配置

// TestConfig 测试配置
type TestConfig struct {
	DatabaseURL     string
	TestTimeout     time.Duration
	EnableDebugLog  bool
	CleanupOnFail   bool
}

// GetTestConfig 获取测试配置
func GetTestConfig() *TestConfig {
	timeout := 30 * time.Second
	if timeoutStr := os.Getenv("TEST_TIMEOUT"); timeoutStr != "" {
		if duration, err := time.ParseDuration(timeoutStr); err == nil {
			timeout = duration
		}
	}
	
	return &TestConfig{
		DatabaseURL:    os.Getenv("TEST_DATABASE_URL"),
		TestTimeout:    timeout,
		EnableDebugLog: os.Getenv("TEST_DEBUG") == "true",
		CleanupOnFail:  os.Getenv("TEST_CLEANUP_ON_FAIL") != "false",
	}
}

// 并发测试辅助函数

// ConcurrentTestResult 并发测试结果
type ConcurrentTestResult struct {
	TotalOperations    int64
	SuccessfulOps      int64
	FailedOps          int64
	ConflictCount      int64
	AverageLatency     time.Duration
	MaxLatency         time.Duration
	MinLatency         time.Duration
	ThroughputPerSec   float64
}

// RunConcurrentTest 运行并发测试
func RunConcurrentTest(concurrency int, operation func() error) ConcurrentTestResult {
	startTime := time.Now()
	results := make(chan error, concurrency)
	latencies := make(chan time.Duration, concurrency)
	
	// 启动并发操作
	for i := 0; i < concurrency; i++ {
		go func() {
			opStart := time.Now()
			err := operation()
			latency := time.Since(opStart)
			
			results <- err
			latencies <- latency
		}()
	}
	
	// 收集结果
	var successfulOps, failedOps, conflictCount int64
	var totalLatency, maxLatency, minLatency time.Duration
	minLatency = time.Hour // 初始化为一个大值
	
	for i := 0; i < concurrency; i++ {
		err := <-results
		latency := <-latencies
		
		if err != nil {
			failedOps++
			if strings.Contains(err.Error(), "conflict") ||
			   strings.Contains(err.Error(), "version") ||
			   strings.Contains(err.Error(), "already reserved") {
				conflictCount++
			}
		} else {
			successfulOps++
		}
		
		totalLatency += latency
		if latency > maxLatency {
			maxLatency = latency
		}
		if latency < minLatency {
			minLatency = latency
		}
	}
	
	totalDuration := time.Since(startTime)
	averageLatency := totalLatency / time.Duration(concurrency)
	throughputPerSec := float64(successfulOps) / totalDuration.Seconds()
	
	return ConcurrentTestResult{
		TotalOperations:  int64(concurrency),
		SuccessfulOps:    successfulOps,
		FailedOps:        failedOps,
		ConflictCount:    conflictCount,
		AverageLatency:   averageLatency,
		MaxLatency:       maxLatency,
		MinLatency:       minLatency,
		ThroughputPerSec: throughputPerSec,
	}
}

