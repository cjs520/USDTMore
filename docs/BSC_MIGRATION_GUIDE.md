# BSC API 迁移指南

## 重要通知

BSCScan API 已被废弃，不再支持免费的API访问。为了确保BSC链的交易监控正常工作，您需要迁移到专用的Web3 API提供商。

## 迁移步骤

### 1. 选择Web3 API提供商

推荐的提供商（按推荐程度排序）：

#### Moralis（推荐）
- ✅ 免费额度充足
- ✅ 易于配置
- ✅ 稳定可靠
- 🔗 注册地址：https://moralis.io

#### Alchemy
- ✅ 免费额度可用
- ✅ 性能优秀
- 🔗 注册地址：https://alchemy.com

#### QuickNode
- ⚠️ 付费服务
- ✅ 企业级稳定性
- 🔗 注册地址：https://quicknode.com

### 2. 配置环境变量

#### 使用Moralis（推荐）
```bash
# 设置BSC Web3提供商
BSC_WEB3_PROVIDER=MORALIS

# 设置Moralis API密钥
MORALIS_API_KEY=your_moralis_api_key_here

# 可选：设置监控模式（默认为RECENT）
BSC_MONITOR_MODE=RECENT
BSC_RECENT_BLOCK_RANGE=100
```

#### 使用Alchemy
```bash
# 设置BSC Web3提供商
BSC_WEB3_PROVIDER=ALCHEMY

# 设置Alchemy API密钥
ALCHEMY_API_KEY=your_alchemy_api_key_here
```

#### 使用QuickNode
```bash
# 设置BSC Web3提供商
BSC_WEB3_PROVIDER=QUICKNODE

# 设置QuickNode配置
QUICKNODE_API_KEY=your_quicknode_api_key_here
QUICKNODE_ENDPOINT=https://your-endpoint.quicknode.pro/xxx
```

### 3. 移除旧配置

删除或注释掉以下环境变量：
```bash
# BSC_SCAN_API_KEY=deprecated  # 已弃用，可以删除
```

### 4. 重启应用

配置完成后重启USDTMore应用，系统会自动验证BSC配置。

## 获取API密钥详细步骤

### Moralis API密钥获取

1. 访问 https://moralis.io 并注册账户
2. 登录后进入控制台
3. 创建新项目或选择现有项目
4. 在项目设置中找到"API Keys"
5. 复制"Web3 API Key"
6. 将密钥设置为`MORALIS_API_KEY`环境变量

### Alchemy API密钥获取

1. 访问 https://alchemy.com 并注册账户
2. 创建新的App，选择BSC网络
3. 在App详情页面找到API Key
4. 将密钥设置为`ALCHEMY_API_KEY`环境变量

### QuickNode配置获取

1. 访问 https://quicknode.com 并注册账户
2. 创建BSC Mainnet端点
3. 获取HTTP Provider URL和API Key
4. 分别设置为`QUICKNODE_ENDPOINT`和`QUICKNODE_API_KEY`

## 验证配置

启动应用后，检查日志输出：

✅ **配置正确**：
```
✅ BSC Web3 API配置验证通过
```

❌ **配置错误**：
```
❌ BSC配置错误: 选择了MORALIS提供商但未设置MORALIS_API_KEY
💡 BSC配置建议:
   ✅ 当前使用: MORALIS (推荐)
   ❌ 缺少配置: MORALIS_API_KEY
   获取地址: https://moralis.io
```

## 常见问题

### Q: 为什么BSCScan API被废弃了？
A: BSCScan官方限制了免费API的访问，导致"Invalid API Key"错误频繁出现。

### Q: 哪个提供商最好？
A: 推荐Moralis，因为它提供充足的免费额度，配置简单，且专门为Web3应用设计。

### Q: 配置后还是报错怎么办？
A: 检查API密钥是否正确，网络连接是否正常，以及提供商的服务状态。

### Q: 可以同时配置多个提供商吗？
A: 目前只支持配置一个提供商，但可以通过修改`BSC_WEB3_PROVIDER`环境变量来切换。

### Q: 旧的BSC_SCAN_API_KEY还有用吗？
A: 不再有用，可以安全删除该配置。

## 技术细节

### 监控模式说明

- `BSC_MONITOR_MODE=RECENT`：仅监控最新区块（推荐，性能更好）
- `BSC_MONITOR_MODE=FULL`：监控完整历史记录（资源消耗大）

### API调用优化

- Moralis：使用ERC-20 token transfers API
- Alchemy/QuickNode：使用eth_getLogs JSON-RPC调用
- 自动重试机制和错误处理
- 智能区块范围管理

## 支持

如果在迁移过程中遇到问题，请：

1. 检查日志输出中的详细错误信息
2. 验证API密钥的有效性
3. 确认网络连接正常
4. 查看提供商的服务状态页面

迁移完成后，BSC链的交易监控将更加稳定可靠。