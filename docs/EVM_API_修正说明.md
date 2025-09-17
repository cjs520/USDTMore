# EVM链API端点修正说明

## 修正概述

根据Etherscan V2 API文档，修正了除BSC链外其他EVM兼容链的API调用问题。

## 主要问题

### 修正前的问题
1. **错误的API端点使用**：所有EVM链都使用统一的 `https://api.etherscan.io/v2/api` 端点
2. **缺乏故障转移机制**：没有备用端点支持
3. **参数格式不当**：专用API不需要chainid参数，但代码中统一添加了

### 修正后的改进
1. **使用正确的专用API端点**：
   - Polygon: `https://api.polygonscan.com/api`
   - Optimism: `https://api-optimistic.etherscan.io/api`
   - Arbitrum: `https://api.arbiscan.io/api`
   - X-Layer: `https://www.oklink.com/api/explorer/v1/eth`

2. **实现多端点故障转移**：主要端点失败时自动切换到备用端点
3. **智能参数处理**：根据API类型动态调整请求参数

## 修正的文件

### 1. app/monitor/trade.go
**函数**: `getUsdtTransByETH`

**主要修改**:
- 移除硬编码的API端点
- 使用 `config.GetEVMChainAPIEndpoints(chain)` 获取正确配置
- 实现多端点故障转移机制
- 根据API类型动态添加chainid参数

**修正前**:
```go
switch chain {
case "POLY":
    host = "https://api.etherscan.io/v2/api" // 错误！
    chainId = "137"
    // ...
}
```

**修正后**:
```go
// 获取API端点配置
apiConfig := config.GetEVMChainAPIEndpoints(chain)
endpoints := apiConfig.GetAllEndpoints()

// 尝试所有可用的API端点
for i, endpoint := range endpoints {
    // 专用API不需要chainid参数，只有Etherscan V2 API需要
    if strings.Contains(endpoint, "api.etherscan.io") {
        extraParams["chainid"] = apiConfig.ChainID
    }
    // ...
}
```

### 2. app/telegram/callback.go
**函数**: `getWalletInfoByPOLAddress`, `getWalletInfoByOPTAddress`, `getWalletInfoByBSCAddress`, `getWalletInfoETH`

**主要修改**:
- 修正钱包查询函数使用正确的API端点
- BSC链改用专用的Web3提供商
- 在getWalletInfoETH中实现智能参数处理

**修正前**:
```go
func getWalletInfoByPOLAddress(address string) string {
    return getWalletInfoETH("Polygon", "MATIC", "POLY", "https://api.etherscan.io/v2/api", ...)
}
```

**修正后**:
```go
func getWalletInfoByPOLAddress(address string) string {
    return getWalletInfoETH("Polygon", "MATIC", "POLY", "", ...)
}

func getWalletInfoByBSCAddress(address string) string {
    // BSC不再使用Etherscan API，使用专用的Web3提供商
    return getBscWalletInfo(address)
}
```

### 3. app/config/api_endpoints.go
**修改**: 更新备用端点为V2 API

**修正前**:
```go
Fallbacks: []string{
    "https://api.etherscan.io/api", // V1 API
},
```

**修正后**:
```go
Fallbacks: []string{
    "https://api.etherscan.io/v2/api", // V2 API
},
```

## 技术改进

### 1. 多端点故障转移
- 主要端点失败时自动尝试备用端点
- 详细的日志记录，便于问题排查
- 智能端点选择和错误处理

### 2. 参数格式优化
- 专用API（如polygonscan.com）不需要chainid参数
- Etherscan V2 API需要chainid参数
- 动态参数构建，避免无效请求

### 3. BSC链专用处理
- BSC链不再使用Etherscan API
- 使用专用的Web3提供商（Moralis、QuickNode、Alchemy）
- 保持现有的Web3 API配置

## 预期效果

### 1. 提高API调用成功率
- 使用正确的专用API端点
- 多端点故障转移机制
- 减少API调用失败

### 2. 改善性能
- 专用API通常比通用API响应更快
- 减少无效的chainid参数传递
- 更好的错误处理和重试机制

### 3. 增强稳定性
- 备用端点确保服务连续性
- 详细的日志记录便于监控
- 智能的错误分析和处理

## 配置要求

### 环境变量
确保设置了以下环境变量：
- `ETHERSCAN_API_KEY`: 用于所有EVM兼容链的API调用
- BSC链的Web3提供商配置（根据选择的提供商）:
  - `MORALIS_API_KEY` (推荐)
  - `QUICKNODE_API_KEY` + `QUICKNODE_ENDPOINT`
  - `ALCHEMY_API_KEY`

### API Key轮询
- 支持最多3个Etherscan API Key轮询使用
- 环境变量: `ETHERSCAN_API_KEY`, `ETHERSCAN_API_KEY_2`, `ETHERSCAN_API_KEY_3`

## 测试建议

1. **功能测试**: 测试各链的交易查询和余额查询
2. **故障转移测试**: 模拟主要端点失败，验证备用端点是否正常工作
3. **性能测试**: 对比修正前后的API响应时间
4. **日志监控**: 检查日志中的API调用详情和错误信息

## 注意事项

1. **API限流**: 不同的API端点有不同的限流策略，需要合理控制调用频率
2. **API Key管理**: 建议为不同的专用API申请专门的API Key
3. **监控告警**: 建议设置API调用失败的监控告警
4. **定期检查**: 定期检查各API端点的可用性和响应时间

---

修正完成时间: 2025年1月17日
修正人员: CodeBuddy (Tencent AI Assistant)