# Docker 部署教程 🐳

## 前置要求

确保你的服务器已安装：
- Docker (版本 20.10+)
- Docker Compose (版本 2.0+)

如果未安装，请参考 [Docker官方文档](https://docs.docker.com/get-docker/) 进行安装。

## 🚀 快速部署

### 方法一：Docker Run（简单部署）

```bash
docker run -d --restart=always --name usdtmore -p 6080:6080 \
  -e TG_BOT_TOKEN=你的机器人Token \
  -e TG_BOT_ADMIN_ID=你的管理员ID \
  -e AUTH_TOKEN=你的验证密钥 \
  -e APP_URI=https://你的域名.com \
  -e REWRITE_HTTPS=true \
  -e ETHERSCAN_API_KEY=你的Etherscan_API_Key \
  -e TRON_SERVER_API="https://apilist.tronscanapi.com/api/block" \
  -e TRON_SCAN_API_KEY=你的TRON_SCAN_API_KEY \
  -e SOLANA_API_KEY=你的Solana_API_Key \
  zxzx412/usdtmore:latest
```

### 方法二：Docker Compose（推荐）

创建 `docker-compose.yml` 文件：

```yaml
version: '3.8'

services:
  usdtmore:
    image: zxzx412/usdtmore:latest
    container_name: usdtmore
    restart: always
    ports:
      - "6080:6080"
    environment:
      # 必需配置项
      TG_BOT_TOKEN: "你的机器人Token"
      TG_BOT_ADMIN_ID: "你的管理员ID" 
      
      # API Keys (必需)
      ETHERSCAN_API_KEY: "你的Etherscan_API_Key"    # EVM链统一API Key
      TRON_SCAN_API_KEY: "你的TRON_SCAN_API_KEY"     # TRON链API Key
      SOLANA_API_KEY: "你的Solana_API_Key"          # Solana链API Key (可选)
      
      # 应用配置
      AUTH_TOKEN: "你的验证密钥"
      APP_URI: "https://你的域名.com"
      REWRITE_HTTPS: "true"
      
      # 网络配置
      TRON_SERVER_API: "https://apilist.tronscanapi.com/api/block"        # 推荐使用TRON_GRID
      ETH_CONFIRMATION: "0"               # EVM链确认数
      TRADE_IS_CONFIRMED: "0"             # TRON链是否需要确认
      
      # 支付配置
      PAYMENT_AMOUNT_RANGE: "0.01,99999"  # 支付金额范围
      EXPIRE_TIME: "1800"                 # 订单过期时间(秒)
      
      # 可选：预设钱包地址
      WALLET_ADDRESS: "TRON:你的TRON地址,BSC:你的BSC地址,POLY:你的Polygon地址"
      
      # Telegram通知配置 (可选)
      TG_BOT_GROUP_ID: "你的群组ID"        # 交易通知群组
    volumes:
      - ./data:/app/db                    # 数据库持久化
      - ./logs:/app/log                   # 日志持久化
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:6080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

启动服务：

```bash
# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f

# 停止
docker-compose down
```

## 🔧 环境变量配置说明

### 必需配置项 ⚠️

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `TG_BOT_TOKEN` | Telegram机器人Token | `6123456789:AAEhBOweik6ad6PsLMuhl3oifns...` |
| `TG_BOT_ADMIN_ID` | Telegram管理员ID | `123456789` |
| `ETHERSCAN_API_KEY` | EVM链统一API密钥 | `ABCD1234EFGH5678` |
| `TRON_GRID_API_KEY` | TRON链API密钥 | `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` |

### 可选配置项

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `SOLANA_API_KEY` | - | Solana链API密钥（如需使用SOL链） |
| `APTOS_API_KEY` | - | Aptos链API密钥（如需使用APT链） |
| `AUTH_TOKEN` | `123456` | API认证Token |
| `APP_URI` | 自动检测 | 应用访问域名 |
| `EXPIRE_TIME` | `1800` | 订单过期时间(秒) |
| `PAYMENT_AMOUNT_RANGE` | `0.01,99999` | 支付金额范围 |

## 🔗 支持的区块链网络

系统支持以下8条区块链网络：

| 链标识 | 区块链网络 | 代币类型 | 所需API Key |
|--------|-----------|----------|-------------|
| `TRON` | TRON | USDT-TRC20 | `TRON_GRID_API_KEY` |
| `POLY` | Polygon | USDT-ERC20 | `ETHERSCAN_API_KEY` |
| `OP` | Optimism | USDT-ERC20 | `ETHERSCAN_API_KEY` |
| `BSC` | BSC | USDT-BEP20 | `ETHERSCAN_API_KEY` |
| `ARB` | Arbitrum One | USDT-ERC20 | `ETHERSCAN_API_KEY` |
| `XLAYER` | X-Layer | USDT | `ETHERSCAN_API_KEY` |
| `SOL` | Solana | USDT-SPL | `SOLANA_API_KEY` |
| `APT` | Aptos | USDT | `APTOS_API_KEY` |

## 🎯 钱包地址配置

可通过环境变量或机器人命令添加收款地址：

```bash
# 环境变量方式（多个地址用逗号分隔）
WALLET_ADDRESS="TRON:TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t,BSC:0x1234...abcd,POLY:0x5678...efgh"

# 或通过机器人命令添加
/add_address TRON TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t
/add_address BSC 0x1234567890abcdef1234567890abcdef12345678
```

## 📝 API Key申请指南

### EtherScan API Key（EVM链通用）
1. 访问 [EtherScan.io](https://etherscan.io/)
2. 注册账号并登录
3. 进入 [API Keys](https://etherscan.io/myapikey) 页面
4. 创建新的API Key
5. **一个Key支持所有EVM链**：Polygon、BSC、Arbitrum、Optimism、X-Layer

### Tronscan API Key
1. 访问 [Tronscan](https://tronscan.org/))
2. 注册账号并登录
3. 在用户中心创建API Key
4. 免费套餐：10万次/天

### Solana API Key
1. 访问 [Solscan.io](https://solscan.io/)
2. 注册开发者账号
3. 创建API Key

### Aptos API Key
1. 访问 [AptosLabs.com](https://aptoslabs.com/)
2. 注册开发者账号
3. 申请API访问权限

## 🚦 部署验证

部署完成后：

1. **检查服务状态**：
```bash
docker ps | grep usdtmore
```

2. **访问Web界面**：
```
http://你的服务器IP:6080
```

3. **查看运行日志**：
```bash
docker logs -f usdtmore
```

4. **测试机器人**：
向机器人发送 `/help` 命令

## ⚠️ 重要提醒

- **所有API Key都是必需的**，缺失将导致对应链无法工作
- **定期备份数据库**文件（`./data` 目录）
- **确保服务器时间准确**，避免订单时间异常
- **生产环境建议使用HTTPS**和反向代理
- **监控API使用量**，避免超出免费额度

## 🔧 故障排除

### 常见问题

1. **容器启动失败**：
   - 检查环境变量是否正确设置
   - 确认端口6080未被占用

2. **API调用失败**：
   - 验证API Key是否有效
   - 检查网络连接是否正常

3. **机器人无响应**：
   - 确认Token和Admin ID正确
   - 检查机器人是否被封禁

4. **订单回调失败**：
   - 查看日志中的具体错误信息
   - 验证AUTH_TOKEN设置

需要更多帮助？加入交流群：[USDTMore](https://t.me/usdt_more)
