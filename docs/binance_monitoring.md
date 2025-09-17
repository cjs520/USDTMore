# 币安内部转账监控指南

## 问题背景

当你在币安交易所内进行转账时（如充值、提现、内部转账），这些操作可能是**链下交易**，不会在区块链上产生实际的交易记录。

例如：
- 转账ID: `296760475029`
- 金额: 0.14 USDT
- 时间: 2025-09-17 11:20:14
- 类型: 币安内部转账（链下）

这类交易无法通过区块链API（如Moralis、Etherscan）查询到，需要使用币安官方API。

## 解决方案

### 1. 获取币安API密钥

1. 登录币安账户
2. 进入 **API管理** 页面
3. 创建新的API密钥
4. 启用以下权限：
   - ✅ 现货和杠杆交易
   - ✅ 钱包
   - ❌ 期货（可选）

### 2. 配置环境变量

在系统中设置以下环境变量：

```bash
# Windows PowerShell
$env:BINANCE_API_KEY="你的API密钥"
$env:BINANCE_SECRET_KEY="你的密钥"

# Linux/Mac
export BINANCE_API_KEY="你的API密钥"
export BINANCE_SECRET_KEY="你的密钥"
```

### 3. 可监控的API端点

| API端点 | 功能 | 说明 |
|---------|------|------|
| `/sapi/v1/capital/deposit/hisrec` | 充值记录 | 查询USDT充值历史 |
| `/sapi/v1/capital/withdraw/history` | 提现记录 | 查询USDT提现历史 |
| `/sapi/v1/sub-account/transfer/subUserHistory` | 内部转账 | 子账户间转账记录 |
| `/api/v3/myTrades` | 交易记录 | 现货交易记录 |

### 4. 使用方法

启动应用后，系统会自动检测币安API配置：

```
✅ 检测到币安API密钥已配置
可以启用币安内部转账监控功能
```

或者：

```
❌ 币安API密钥未配置
请设置 BINANCE_API_KEY 和 BINANCE_SECRET_KEY 环境变量
```

## 技术实现

### 代码结构

```
app/monitor/
├── binance.go          # 完整的币安API集成
├── binance_simple.go   # 简化版监控和指南
└── trade.go           # 主监控逻辑
```

### 关键函数

1. **getBinanceConfig()** - 获取API配置
2. **getUSDTTransfersByBinanceAPI()** - 查询USDT转账记录
3. **handleBinanceInternalTransfers()** - 处理内部转账
4. **CheckBinanceInternalTransfer()** - 检测内部转账ID

### API签名

币安API需要HMAC-SHA256签名：

```go
func (bc *BinanceConfig) generateSignature(queryString string) string {
    h := hmac.New(sha256.New, []byte(bc.SecretKey))
    h.Write([]byte(queryString))
    return hex.EncodeToString(h.Sum(nil))
}
```

## 注意事项

### 1. API限制
- 币安API有频率限制
- 建议控制请求频率（每分钟不超过1200次）
- 使用合理的时间间隔

### 2. 安全性
- API密钥具有敏感权限，请妥善保管
- 建议只启用必要的权限
- 定期更换API密钥

### 3. 数据同步
- 币安内部转账是即时的
- API数据可能有几秒钟的延迟
- 建议设置合理的轮询间隔

## 示例响应

### 充值记录响应
```json
[
  {
    "amount": "0.14",
    "coin": "USDT",
    "network": "BSC",
    "status": 1,
    "address": "0xb5bc9cf309320abe924f38b13ec5c519ae04dbc5",
    "txId": "296760475029",
    "insertTime": 1726549214000,
    "transferType": 0,
    "confirmTimes": "1/1"
  }
]
```

### 内部转账响应
```json
[
  {
    "asset": "USDT",
    "qty": "0.14",
    "time": 1726549214000,
    "tranId": "296760475029",
    "status": "SUCCESS"
  }
]
```

## 集成到现有系统

币安监控可以与现有的区块链监控并行运行：

1. **区块链监控** - 监控真实的链上交易
2. **币安监控** - 监控交易所内部转账
3. **统一处理** - 将两种类型的交易统一处理

这样可以确保无论是链上交易还是链下交易，都能被正确监控和处理。