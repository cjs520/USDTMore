# BSC Web3 API配置指南

## 重要更新说明

由于BscScan API已被弃用，Etherscan V2 API对BSC链需要付费访问，USDTMore系统现在支持三种专业Web3 API提供商来解决BSC监控问题。

## 支持的API提供商

### 1. Moralis Web3 API（推荐）

**注册地址**: https://moralis.io

**优势**:
- 免费层提供充足的API调用
- 专门的区块链数据API
- API格式: GET /api/v2/{address}/erc20
- 支持BSC链USDT交易查询

**配置示例**:
```bash
BSC_WEB3_PROVIDER=MORALIS
MORALIS_API_KEY=your_moralis_api_key
```

### 2. QuickNode API

**注册地址**: https://quicknode.com

**优势**:
- 专业的区块链节点服务
- 有免费层可用
- JSON-RPC接口支持
- 高性能区块链访问

**配置示例**:
```bash
BSC_WEB3_PROVIDER=QUICKNODE
QUICKNODE_API_KEY=your_quicknode_api_key
QUICKNODE_ENDPOINT=https://your-quicknode-endpoint.com
```

### 3. Alchemy API

**注册地址**: https://alchemy.com

**优势**:
- 强大的Web3基础设施
- 免费层支持BSC
- 企业级API服务
- 完整的区块链数据访问

**配置示例**:
```bash
BSC_WEB3_PROVIDER=ALCHEMY
ALCHEMY_API_KEY=your_alchemy_api_key
```

## BSC监控模式配置

### 监控模式选择

系统支持两种监控模式：

1. **RECENT模式**（推荐）：仅监控最新区块，性能更好
2. **FULL模式**：完整历史记录监控，资源消耗大

```bash
BSC_MONITOR_MODE=RECENT    # 推荐值
```

### 最新区块监控范围

在RECENT模式下，可以配置监控的区块范围：

```bash
BSC_RECENT_BLOCK_RANGE=100 # 监控最近100个区块的交易
```

## 完整配置示例

### 使用Moralis API（推荐配置）

```bash
# BSC Web3 API提供商选择
BSC_WEB3_PROVIDER=MORALIS

# Moralis API密钥
MORALIS_API_KEY=your_moralis_api_key_here

# BSC监控模式（仅监控最新区块以提高性能）
BSC_MONITOR_MODE=RECENT

# 监控最近100个区块的交易
BSC_RECENT_BLOCK_RANGE=100
```

### 使用QuickNode API

```bash
# BSC Web3 API提供商选择
BSC_WEB3_PROVIDER=QUICKNODE

# QuickNode API配置
QUICKNODE_API_KEY=your_quicknode_api_key
QUICKNODE_ENDPOINT=https://your-quicknode-endpoint.com

# BSC监控模式
BSC_MONITOR_MODE=RECENT
BSC_RECENT_BLOCK_RANGE=100
```

### 使用Alchemy API

```bash
# BSC Web3 API提供商选择
BSC_WEB3_PROVIDER=ALCHEMY

# Alchemy API密钥
ALCHEMY_API_KEY=your_alchemy_api_key

# BSC监控模式
BSC_MONITOR_MODE=RECENT
BSC_RECENT_BLOCK_RANGE=100
```

## 向后兼容性

如果不配置BSC_WEB3_PROVIDER，系统将默认使用ETHERSCAN模式以保持向后兼容：

```bash
# 默认配置（向后兼容）
BSC_WEB3_PROVIDER=ETHERSCAN  # 或不设置此参数
ETHERSCAN_API_KEY=your_etherscan_api_key
```

## 性能优化建议

1. **使用RECENT模式**：避免查询大量历史数据
2. **合理设置区块范围**：根据交易频率调整BSC_RECENT_BLOCK_RANGE
3. **API密钥轮询**：配置多个ETHERSCAN_API_KEY以分散请求压力
4. **监控日志**：启用REQUEST_LOG_ENABLED=true来调试API调用

## 故障排除

### 常见问题

1. **API密钥未配置**
   - 错误信息：`MORALIS_API_KEY未配置`
   - 解决方案：检查.env文件中的API密钥配置

2. **API调用失败**
   - 错误信息：`API请求失败`
   - 解决方案：检查网络连接和API密钥有效性

3. **区块高度获取失败**
   - 错误信息：`无法获取BSC当前区块高度`
   - 解决方案：系统会自动降级到完整历史查询

### 日志监控

启用详细日志来监控API调用：

```bash
REQUEST_LOG_ENABLED=true
LOG_DIR=/app/logs
```

查看日志：
```bash
tail -f /app/logs/usdtmore.log | grep "BSC"
```

## 迁移指南

### 从旧版BSC配置迁移

如果您之前使用的是BSC_SCAN_API_KEY，请按照以下步骤迁移：

1. **注册新的Web3 API服务**（推荐Moralis）
2. **更新.env配置文件**：
   ```bash
   # 旧配置（可以保留作为备用）
   # BSC_SCAN_API_KEY=your_old_bsc_scan_key
   
   # 新配置
   BSC_WEB3_PROVIDER=MORALIS
   MORALIS_API_KEY=your_new_moralis_key
   BSC_MONITOR_MODE=RECENT
   BSC_RECENT_BLOCK_RANGE=100
   ```
3. **重启服务**：
   ```bash
   docker-compose down
   docker-compose up -d
   ```
4. **验证配置**：检查日志确认BSC监控正常工作

## 技术实现细节

### API调用流程

1. **Moralis API**: 使用REST API查询ERC-20交易
2. **QuickNode/Alchemy**: 使用JSON-RPC eth_getLogs方法
3. **响应处理**: 自动解析不同API的响应格式
4. **交易匹配**: 智能匹配订单金额和地址

### 区块监控策略

- **RECENT模式**: 从(当前区块 - BSC_RECENT_BLOCK_RANGE)开始查询
- **FULL模式**: 从钱包地址的起始区块开始查询
- **区块高度获取**: 使用多个公共RPC端点获取当前区块高度

### 错误处理机制

- **API失败重试**: 使用配置的MAX_RETRIES参数
- **多端点支持**: QuickNode和Alchemy支持多个备用端点
- **降级处理**: API失败时自动降级到备用方案