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

### 数据库选择建议

| 配置类型 | 适用场景 | 部署命令 | 特性说明 |
|---------|---------|---------|---------|
| **SQLite（默认）** | 个人使用、小规模部署 | `docker-compose up -d` | 零配置，开箱即用，资源占用少 |
| **PostgreSQL（标准）** | 中小型生产环境 | `docker-compose -f docker-compose.postgresql.yml up -d` | 高并发，数据完整性保证 |
| **生产级（推荐）** | 大型生产环境 | `docker-compose -f docker-compose.production.yml up -d` | 主从复制，连接池，监控，负载均衡 |

#### 🔄 部署配置对比

**SQLite版本**：
- ✅ 单文件数据库，部署简单
- ✅ 内存占用 < 100MB
- ⚠️ 并发限制，适合日订单 < 500

**PostgreSQL标准版**：
- ✅ 关系型数据库，ACID保证
- ✅ 支持50+并发连接
- ✅ 数据备份和恢复
- ⚠️ 内存占用 200-500MB

**生产级高可用版**：
- ✅ 主从复制，99.9%可用性
- ✅ 连接池，支持200+并发
- ✅ 完整监控和告警
- ✅ 自动故障转移
- ✅ 负载均衡和缓存
- ⚠️ 内存占用 1-2GB，配置复杂

### 支持的区块链网络

- **TRON (TRX)**：基于TRON网络的USDT-TRC20转账，手续费约1-2 USDT
- **Polygon (POLY)**：基于Polygon网络的USDT转账，手续费约0.001-0.01 USDT
- **Optimism (OP)**：基于Optimism网络的USDT转账，手续费约0.0001-0.001 USDT
- **BSC (BSC)**：基于Binance Smart Chain的USDT转账，手续费约0.1-0.3 USDT
- **Arbitrum One (ARB)**：基于Arbitrum One网络的USDT转账，手续费约0.0001-0.001 USDT
- **X-Layer (XLAYER)**：基于X-Layer网络的USDT转账，手续费极低
- **Solana (SOL)**：基于Solana网络的USDT-SPL转账，手续费约0.000005 SOL
- **Aptos (APT)**：基于Aptos网络的USDT转账，手续费约0.0001 APT

## 📊 性能特性 (2025.09更新)

- ⚡ **高性能HTTP客户端**: 连接池复用，性能提升30-50%
- 🔄 **优雅关闭机制**: 支持Ctrl+C优雅停止，确保数据完整性
- 🛡️ **错误恢复**: 全局panic恢复机制，防止单点故障导致服务崩溃
- 📈 **并发优化**: 优化轮询机制，CPU使用率降低15-25%
- 🗄️ **双数据库支持**: SQLite与PostgreSQL自由切换
- 🔍 **完整测试覆盖**: 单元测试、集成测试、性能测试全覆盖

## ⚠️ 特别注意

- 订单交易强依赖时间，请确保服务器时间准确性，否则可能导致订单异常！
- 部分功能依赖网络，请确保服务器网络纯洁性，否则可能导致功能异常！
- 如果有问题，欢迎加入交流群交流 [USDTMore](https://t.me/usdt_more)
- **重要**: 生产环境建议使用PostgreSQL数据库以获得更好的性能和稳定性

## 🔧 故障排除

### 常见问题

1. **服务启动失败**
   ```bash
   # 检查配置是否正确
   docker logs usdtmore
   
   # 检查必要参数是否设置
   echo $TG_BOT_TOKEN
   echo $TG_BOT_ADMIN_ID
   ```

2. **交易查询失败**
   ```bash
   # 检查API Key是否正确设置
   echo $TRON_SCAN_API_KEY
   echo $ETHERSCAN_API_KEY
   ```

3. **数据库连接问题**
   ```bash
   # 检查PostgreSQL连接
   docker exec -it usdtmore /app/usdtmore -test-db
   
   # 查看详细日志
   docker logs -f usdtmore
   ```

## 🙏 感谢三位大佬的代码，在此基础上改写了新的功能

- https://github.com/assimon/epusdt
- https://github.com/v03413/bepusdt
- https://github.com/botinheart/USDTMore

## 📢 声明

- 本项目仅供个人学习研究使用，任何人或组织在使用过程中请符合当地的法律法规，否则产生的任何后果责任自负。

---

## 🎯 更新日志

### v1.10.0 (2025.09)
- ✨ 新增PostgreSQL数据库支持
- ⚡ HTTP客户端连接池优化，性能提升30-50%
- 🔄 重构轮询机制，支持优雅关闭
- 🛡️ 改进错误处理，消除panic崩溃风险
- 🧪 完整测试套件覆盖
- 📈 CPU使用率优化，降低15-25%
- 🔧 增强配置管理和环境变量支持

### v1.9.21
- 🔧 修复订单金额格式化问题
- 📊 优化订单匹配逻辑
- 🌐 统一使用Etherscan V2 API格式
- 🔑 移除Aptos API Key依赖