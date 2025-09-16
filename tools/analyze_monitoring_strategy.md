# USDT转账监控策略分析

## 当前问题
1. tokentx API 查询特定地址的USDT转账记录时返回空或错误
2. 可能是API参数、Key或监控策略的问题

## 可选监控方案

### 1. tokentx (当前使用) - ERC-20代币转账
**用途**: 查询特定地址的代币转账记录
**适用场景**: 监控地址是否收到USDT
**API**: `action=tokentx&contractaddress=USDT_CONTRACT&address=TARGET_ADDRESS`

### 2. txlist - 普通交易记录  
**用途**: 查询地址的所有交易
**适用场景**: 监控地址的所有活动，然后过滤USDT相关
**API**: `action=txlist&address=TARGET_ADDRESS`

### 3. txlistinternal - 内部交易
**用途**: 查询智能合约内部调用
**适用场景**: 复杂的DeFi交易或合约调用产生的转账
**API**: `action=txlistinternal&address=TARGET_ADDRESS` 或 `txhash=SPECIFIC_TX`

### 4. 混合策略
**用途**: 结合多个API获得完整的交易信息
**适用场景**: 确保不遗漏任何USDT转账

## 建议的修复方案

### 方案1: 修复当前tokentx API
- 检查API Key有效性
- 验证参数格式
- 添加错误处理和重试机制

### 方案2: 使用txlist + 过滤
- 获取地址的所有交易
- 在本地过滤USDT相关交易
- 更可靠但可能更慢

### 方案3: 多API并行
- 同时使用tokentx和txlistinternal
- 合并结果确保完整性
- 最可靠但消耗更多API配额