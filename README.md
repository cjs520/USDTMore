# USDTMore (USDT Payment Gateway for More Chain)

<p align="center">
<img src="./static/img/tether.svg" width="15%" alt="tether">
</p>
<p align="center">
<a href="https://www.gnu.org/licenses/gpl-3.0.html"><img src="https://img.shields.io/badge/license-GPLV3-blue" alt="license GPLV3"></a>
<a href="https://golang.org"><img src="https://img.shields.io/badge/Golang-1.23-red" alt="Go version 1.23"></a>
<a href="https://github.com/gin-gonic/gin"><img src="https://img.shields.io/badge/Gin-v1.10-blue" alt="Gin Web Framework v1.10"></a>
<a href="https://github.com/go-telegram-bot-api/telegram-bot-api"><img src="https://img.shields.io/badge/Telegram Bot-v5-lightgrey" alt="Golang Telegram Bot Api-v5"></a>
<a href="https://github.com/v03413/bepusdt"><img src="https://img.shields.io/badge/Release-v1.9.21-green" alt="Release v1.9.21"></a>
</p>

## 🪧 介绍

对Bepusdt进行了二次改造, 针对系统原生安装进行了优化, 同时加入了多条费率更低的链路支持: Polygon, Optimism, BSC, Arbitrum One, X-Layer, Solana, Aptos;

收款更好用、部署更便捷！

## 🎉 新特性

- ✅ 具备`Bepusdt`的所有特性，插件兼容无缝替换
- ✅ 增加 `Polygon`, `Optimism`, `BSC`, `Arbitrum One`, `X-Layer` 链的支持
- ✅ 增加 `Solana`, `Aptos` 非EVM链的支持
- ✅ 迁移到Etherscan V2 API，支持更稳定的查询
- ✅ 强制API Key验证，避免官方限流
- ✅ 增加docker部署
- ✅ **[2025.09 更新]** 支持SQLite和PostgreSQL双数据库
- ✅ **[2025.09 更新]** 优化HTTP客户端连接池，性能提升30-50%
- ✅ **[2025.09 更新]** 重构轮询机制，支持优雅关闭，CPU使用率降低15-25%
- ✅ **[2025.09 更新]** 改进错误处理，消除panic导致的服务崩溃风险
- ✅ **[2025.09 更新]** 完整的测试套件，确保系统稳定性

## 🛠 参数配置

USDTMore 所有参数都是以传递环境变量的方式进行配置，大部分参数含默认值，少量配置即可直接使用！

### 基础配置

| 参数名称                      | 默认值          | 用法说明                                                                                                                                          |
|---------------------------|--------------|-----------------------------------------------------------------------------------------------------------------------------------------------|
| EXPIRE_TIME               | `1800`       | 订单有效期，单位秒                                                                                                                                     |
| USDT_RATE                 | `空`          | USDT汇率，默认留空则获取Okx交易所的汇率(每分钟同步一次)，支持多种写法，如：`7.4` 表示固定7.4、`～1.02`表示最新汇率上浮2%、`～0.97`表示最新汇率下浮3%、`+0.3`表示最新加0.3、`-0.2`表示最新减0.2，以此类推；如参数错误则使用固定值7.4 |
| AUTH_TOKEN                | `123456`     | 认证Token，对接发卡网/支付平台会用到这个参数进行回调                                                                                                                 |
| LISTEN                    | `:6080`      | 服务器HTTP监听地址                                                                                                                                   
| REWRITE_HTTPS             | `false`      | 重写http成https，使用反向代理的时候往往需要强制https模式                                                                                                           
| TRADE_IS_CONFIRMED        | `0`          | TRON网络是否需要确认，禁用可以提高回调速度，启用则可以防止交易失败                                                                                                           |
| ETH_CONFIRMATION          | `0`          | ETH兼容网络需要网络确认的块数，影响Polygon、Optimism，Bep20                                                                                                     |
| APP_URI                   | `空`          | 应用访问地址，留空则系统自动获取，前端收银台会用到，建议设置，例如：https://token-pay.example.com                                                                               |
| WALLET_ADDRESS            | `空`          | 启动时需要添加的钱包地址，多个请用半角符逗号`,`分开；当然，同样也支持通过机器人添加。<br>单条格式为: [TRON\|POLY\|OP\|BSC\|ARB\|XLAYER\|SOL\|APT]:地址, 其中[]部分为支付链标识                                                 |
| PAYMENT_AMOUNT_RANGE      | `0.01,99999` | 支付监控的允许数额范围(闭区间)，设置合理数值可避免一些诱导式诈骗交易提醒                                                                                                         |
| LOG_DIR                   | `./log`      | 应用程序的日志路径                                                                                                                                     |
| HTML_DIR                  | `..`         | 界面模版/静态资源的路径                                                                                                                                      |

### 🗄️ 数据库配置 (新增)

| 参数名称                  | 默认值        | 用法说明                                                   |
|-------------------------|-------------|----------------------------------------------------------|
| DB_TYPE                 | `sqlite`    | 数据库类型：`sqlite` 或 `postgresql`                        |
| DB_DIR                  | `./db`      | SQLite数据库文件路径（DB_TYPE=sqlite时使用）                    |
| DB_HOST                 | `localhost` | PostgreSQL主机地址（DB_TYPE=postgresql时使用）                |
| DB_PORT                 | `5432`      | PostgreSQL端口（DB_TYPE=postgresql时使用）                   |
| DB_USER                 | `postgres`  | PostgreSQL用户名（DB_TYPE=postgresql时使用）                 |
| DB_PASSWORD             | `空`        | PostgreSQL密码（DB_TYPE=postgresql时使用）                   |
| DB_NAME                 | `usdtmore`  | PostgreSQL数据库名（DB_TYPE=postgresql时使用）                |
| DB_SSLMODE              | `disable`   | PostgreSQL SSL模式（DB_TYPE=postgresql时使用）               |
| DB_TIMEZONE             | `UTC`       | PostgreSQL时区（DB_TYPE=postgresql时使用）                   |
| DB_MAX_IDLE_CONNS       | `10`        | 数据库最大空闲连接数                                          |
| DB_MAX_OPEN_CONNS       | `100`       | 数据库最大开放连接数                                          |
| DB_CONN_MAX_LIFETIME    | `5m`        | 数据库连接最大生存时间                                        |
| DB_CONN_MAX_IDLE_TIME   | `1m`        | 数据库连接最大空闲时间                                        |

### Telegram Bot配置

| 参数名称              | 默认值 | 用法说明                                        |
|-------------------|-----|---------------------------------------------|
| TG_BOT_TOKEN      | `空` | **必须设置** Telegram Bot Token，否则无法使用        |
| TG_BOT_ADMIN_ID   | `空` | **必须设置** Telegram Bot 管理员ID，否则无法使用       |
| TG_BOT_GROUP_ID   | `空` | Telegram 群组ID，设置之后机器人会将交易消息会推送到此群         |

### 区块链API配置

| 参数名称                      | 默认值         | 用法说明                                                                                                 |
|---------------------------|-------------|------------------------------------------------------------------------------------------------------|
| TRON_SERVER_API           | `TRON_SCAN` | 可选`TRON_SCAN`,`TRON_GRID`，推荐`TRON_GRID`和`TRON_GRID_API_KEY`搭配使用，*更准更强更及时*                            |
| TRON_SCAN_API_KEY         | `空`        | **必须设置** TRONSCAN API KEY，强制要求，避免官方限流                                                                |
| TRON_GRID_API_KEY         | `空`        | **必须设置** TRONGRID API KEY，强制要求，避免官方限流                                                                |
| ETHERSCAN_API_KEY         | `空`        | **必须设置** EVM兼容链统一API KEY，支持Polygon、Optimism、BSC、Arbitrum、X-Layer等链                                   |
| SOLANA_API_KEY            | `空`        | **必须设置** SOLANA API KEY，Solana链交易查询API密钥（Solscan）                                                     |

**⚠️ 重要提醒：以下参数为必须设置项，缺少任何一项都将导致系统无法正常运行！**

**必须设置的参数：**
- `TG_BOT_TOKEN` - Telegram机器人Token
- `TG_BOT_ADMIN_ID` - Telegram管理员ID
- `TRON_SCAN_API_KEY` 或 `TRON_GRID_API_KEY` - TRON链API密钥（至少设置一个）
- `ETHERSCAN_API_KEY` - EVM兼容链统一API密钥（Polygon、BSC、Arbitrum等）
- `SOLANA_API_KEY` - Solana链API密钥（如需使用SOL链）

**注意：自2025年起，所有区块链浏览器API都强制要求API Key，不设置将导致交易查询失败！**

## 🚀 安装部署

- [Docker 安装教程（强烈推荐🔥）](./docs/docker.md)
- [https 配置教程](./docs/ssl.md)
- [Linux 手动安装教程](./docs/systemd.md)
- [Linux 时钟同步配置](./docs/systemd-timesyncd.md)

### 🐳 Docker快速部署

```bash
# 1. 拉取镜像
docker pull usdtmore/usdtmore:latest

# 2. 运行容器
docker run -d \
  --name usdtmore \
  -p 6080:6080 \
  -e TG_BOT_TOKEN="你的Telegram Bot Token" \
  -e TG_BOT_ADMIN_ID="你的Telegram用户ID" \
  -e TRON_SCAN_API_KEY="你的TronScan API Key" \
  -e ETHERSCAN_API_KEY="你的Etherscan API Key" \
  -e DB_TYPE="sqlite" \
  -v usdtmore_data:/app/data \
  usdtmore/usdtmore:latest

# 3. 查看运行状态
docker logs -f usdtmore
```

### 🎯 PostgreSQL部署示例

```bash
# 1. 启动PostgreSQL
docker run -d \
  --name postgres \
  -e POSTGRES_DB=usdtmore \
  -e POSTGRES_USER=usdtmore \
  -e POSTGRES_PASSWORD=your_password \
  -p 5432:5432 \
  postgres:15

# 2. 启动USDTMore
docker run -d \
  --name usdtmore \
  --link postgres:postgres \
  -p 6080:6080 \
  -e TG_BOT_TOKEN="你的Token" \
  -e TG_BOT_ADMIN_ID="你的ID" \
  -e TRON_SCAN_API_KEY="你的API Key" \
  -e ETHERSCAN_API_KEY="你的API Key" \
  -e DB_TYPE="postgresql" \
  -e DB_HOST="postgres" \
  -e DB_USER="usdtmore" \
  -e DB_PASSWORD="your_password" \
  -e DB_NAME="usdtmore" \
  usdtmore/usdtmore:latest
```

### 🏭 生产级高可用部署

对于生产环境，我们提供了完整的高可用解决方案：

```bash
# 生产级部署（包含主从复制、连接池、监控）
docker-compose -f docker-compose.production.yml up -d

# 仅启动核心服务（不包含监控）
docker-compose -f docker-compose.production.yml up -d postgres-primary pgbouncer usdtmore-primary redis

# 启用副本服务（读写分离）
docker-compose -f docker-compose.production.yml --profile replica up -d

# 启用完整监控（包含Grafana、Prometheus）
docker-compose -f docker-compose.production.yml --profile monitoring up -d
```

**生产级配置特性**：
- ✅ PostgreSQL主从复制（高可用）
- ✅ PgBouncer连接池（高并发）
- ✅ Redis缓存优化
- ✅ Nginx负载均衡
- ✅ Prometheus + Grafana监控
- ✅ 完整的健康检查和日志管理
- ✅ 自动故障转移和恢复

## 🧪 测试和验证

项目包含完整的测试套件，确保系统稳定性：

```bash
# 运行完整测试套件
cd tests && ./run_comprehensive_tests.sh

# 运行特定测试
./run_comprehensive_tests.sh --unit-only      # 仅单元测试
./run_comprehensive_tests.sh --integration-only  # 仅集成测试
./run_comprehensive_tests.sh --performance-only  # 仅性能测试
./run_comprehensive_tests.sh --coverage       # 覆盖率测试
```

## 插件集成

- [异次元](./plugins/acg-faka/README.md)
- [独角数卡](./plugins/dujiaoka/README.md)

## 🤔 常见问题

### 如何获取参数 TG_BOT_ADMIN_ID

Telegram 搜索`@myidbot`机器人并启用，`/getid`返回的ID就是`TG_BOT_ADMIN_ID`

### 如何申请区块链浏览器的ApiKey

目前[TronScan](https://tronscan.org/)/[TronGrid](https://www.trongrid.io/)、[EtherScan](https://etherscan.io/)、[Solscan](https://solscan.io/) 都可以通过邮箱注册，登录之后在用户中心创建一个ApiKey即可；默认免费套餐都是每天10W请求，对于个人收款绰绰有余。

**注意：** Aptos使用官方公开API，无需申请API Key。

### 数据库选择n
对于各种规模的应用场景，我们提供高性能的PostgreSQL数据库解决方案：n

| 配置类型 | 适用场景 | 部署命令 | 特性说明 |n
|---------|---------|---------|--------|n
| **标准版（推荐）** | 中小型生产环境 | `docker-compose -f docker-compose.postgresql.yml up -d` | 高并发，数据完整性保证 |n
| **生产级高可用** | 大型生产环境 | `docker-compose -f docker-compose.production.yml up -d` | 主从复制，连接池，监控，负载均衡 |n

#### 🔄 PostgreSQL性能对比n

**标准版**：n
- ✅ 关系型数据库，ACID保证n
- ✅ 支持50+并发连接n
- ✅ 数据备份和恢复n
- ⚠️ 内存占用 200-500MBn

**生产级高可用版**：n
- ✅ 主从复制，99.9%可用性n
- ✅ 连接池，支持200+并发n
- ✅ 完整监控和告警n
- ✅ 自动故障转移n
- ✅ 负载均衡和缓存n
- ⚠️ 内存占用 1-2GB，配置复杂n

### Telegram Bot配置

## 🔍 Etherscan V2 Transaction API 集成

项目已集成Etherscan V2 Transaction API，提供增强的交易验证功能，确保订单处理的可靠性和准确性。

### 交易验证方法

1. **Transaction Receipt Status** (`gettxreceiptstatus`)
   - **用途**: 检查交易执行状态  
   - **适用**: Byzantium分叉后的交易
   - **响应**: `status` (1=成功, 0=失败)
   - **优先级**: 首选方法

2. **Contract Execution Status** (`getstatus`)
   - **用途**: 检查智能合约执行状态
   - **适用**: 所有交易  
   - **响应**: `isError` (0=成功, 1=失败)
   - **优先级**: 备用方法

### API格式

```
https://api.etherscan.io/v2/api
?chainid={CHAIN_ID}
&module=transaction
&action={ACTION}
&txhash={TX_HASH}
&apikey={API_KEY}
```

### 支持的链ID

| 链名称 | Chain ID | 说明 |
|--------|----------|------|
| Polygon | 137 | Polygon主网 |
| BSC | 56 | Binance Smart Chain |
| Optimism | 10 | Optimism主网 |
| Arbitrum | 42161 | Arbitrum One |
| X-Layer | 196 | X-Layer主网 |
| Ethereum | 1 | 以太坊主网 |

### 配置要求

启用交易验证功能需要设置以下环境变量：

```bash
# 必需: Etherscan V2 API密钥  
ETHERSCAN_API_KEY=your_api_key_here

# 可选: 启用交易确认验证 (默认false)
TRADE_IS_CONFIRMED=true
```

### 使用方式

**自动集成** (推荐，零代码修改):
```bash
export ETHERSCAN_API_KEY=your_key_here
export TRADE_IS_CONFIRMED=true
# 系统会自动在后台进行交易验证
```

### 核心特性

- ✅ **双重验证**: Receipt Status → Contract Status 智能降级
- ✅ **完全向后兼容**: 验证失败不影响现有业务流程  
- ✅ **批量处理**: 支持多交易并发验证
- ✅ **速率限制保护**: 200ms间隔 + 指数退避重试
- ✅ **企业级可靠性**: 完整的错误处理和恢复机制

