# Docker 部署教程 🐳

## 前置要求

确保你的服务器已安装：
- Docker (版本 20.10+)
- Docker Compose (版本 2.0+)

如果未安装，请参考 [Docker官方文档](https://docs.docker.com/get-docker/) 进行安装。

## 🚀 快速部署

### 方法一：SQLite版本（简单部署）

```bash
docker run -d --restart=always --name usdtmore -p 6080:6080 \
  -e TG_BOT_TOKEN=你的机器人Token \
  -e TG_BOT_ADMIN_ID=你的管理员ID \
  -e AUTH_TOKEN=你的验证密钥 \
  -e APP_URI=https://你的域名.com \
  -e REWRITE_HTTPS=true \
  -e DB_TYPE=sqlite \
  -e ETHERSCAN_API_KEY=你的Etherscan_API_Key \
  -e TRON_SCAN_API_KEY=你的TRON_SCAN_API_KEY \
  -e SOLANA_API_KEY=你的Solana_API_Key \
  -v usdtmore_data:/app/data \
  zxzx412/usdtmore:latest
```

### 方法二：Docker Compose（推荐）

#### SQLite版本 (docker-compose.yml)

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
      # ===== 数据库配置 =====
      DB_TYPE: "sqlite"                       # 数据库类型
      
      # ===== 必需配置项 =====
      TG_BOT_TOKEN: "你的机器人Token"
      TG_BOT_ADMIN_ID: "你的管理员ID" 
      
      # ===== API Keys (必需) =====
      ETHERSCAN_API_KEY: "你的Etherscan_API_Key"    # EVM链统一API Key
      TRON_SCAN_API_KEY: "你的TRON_SCAN_API_KEY"     # TRON链API Key
      SOLANA_API_KEY: "你的Solana_API_Key"          # Solana链API Key (可选)
      
      # ===== 应用配置 =====
      AUTH_TOKEN: "你的验证密钥"
      APP_URI: "https://你的域名.com"
      REWRITE_HTTPS: "true"
      
      # ===== 网络配置 =====
      TRON_SERVER_API: "TRON_SCAN"           # 推荐使用TRON_GRID
      ETH_CONFIRMATION: "0"                  # EVM链确认数
      TRADE_IS_CONFIRMED: "0"               # TRON链是否需要确认
      
      # ===== 支付配置 =====
      PAYMENT_AMOUNT_RANGE: "0.01,99999"   # 支付金额范围
      EXPIRE_TIME: "1800"                   # 订单过期时间(秒)
      
      # ===== 可选：预设钱包地址 =====
      WALLET_ADDRESS: "TRON:你的TRON地址,BSC:你的BSC地址,POLY:你的Polygon地址"
      
      # ===== Telegram通知配置 (可选) =====
      TG_BOT_GROUP_ID: "你的群组ID"        # 交易通知群组
    volumes:
      - usdtmore_data:/app/data             # 数据库持久化 (SQLite)
      - usdtmore_logs:/app/logs             # 日志持久化
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost:6080/ || exit 1"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  usdtmore_data:
  usdtmore_logs:
```

#### PostgreSQL高性能版本 (docker-compose.postgresql.yml)

```yaml
version: '3.8'

services:
  # PostgreSQL数据库
  postgres:
    image: postgres:15-alpine
    container_name: usdtmore_postgres
    restart: always
    environment:
      POSTGRES_DB: usdtmore
      POSTGRES_USER: usdtmore_user
      POSTGRES_PASSWORD: your_secure_password_here
      POSTGRES_INITDB_ARGS: "--encoding=UTF-8 --lc-collate=C --lc-ctype=C"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./config/postgresql.conf:/etc/postgresql/postgresql.conf:ro
      - ./config/pg_hba.conf:/etc/postgresql/pg_hba.conf:ro
    ports:
      - "5432:5432"
    command: ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U usdtmore_user -d usdtmore"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - usdtmore_network

  # USDTMore应用
  usdtmore:
    image: zxzx412/usdtmore:latest
    container_name: usdtmore
    restart: always
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "6080:6080"
    environment:
      # ===== 数据库配置 (PostgreSQL) =====
      DB_TYPE: "postgresql"
      DB_HOST: "postgres"
      DB_PORT: "5432"
      DB_USER: "usdtmore_user"
      DB_PASSWORD: "your_secure_password_here"
      DB_NAME: "usdtmore"
      DB_SSLMODE: "disable"
      DB_TIMEZONE: "UTC"
      
      # ===== 数据库连接池配置 =====
      DB_MAX_IDLE_CONNS: "10"               # 最大空闲连接
      DB_MAX_OPEN_CONNS: "100"              # 最大开放连接
      DB_CONN_MAX_LIFETIME: "5m"            # 连接最大生存时间
      DB_CONN_MAX_IDLE_TIME: "1m"           # 连接最大空闲时间
      
      # ===== 必需配置项 =====
      TG_BOT_TOKEN: "你的机器人Token"
      TG_BOT_ADMIN_ID: "你的管理员ID"
      
      # ===== API Keys (必需) =====
      ETHERSCAN_API_KEY: "你的Etherscan_API_Key"
      TRON_SCAN_API_KEY: "你的TRON_SCAN_API_KEY"
      SOLANA_API_KEY: "你的Solana_API_Key"
      
      # ===== 应用配置 =====
      AUTH_TOKEN: "你的验证密钥"
      APP_URI: "https://你的域名.com"
      REWRITE_HTTPS: "true"
      
      # ===== 网络配置 =====
      TRON_SERVER_API: "TRON_SCAN"
      ETH_CONFIRMATION: "0"
      TRADE_IS_CONFIRMED: "0"
      
      # ===== 支付配置 =====
      PAYMENT_AMOUNT_RANGE: "0.01,99999"
      EXPIRE_TIME: "1800"
      
      # ===== 可选配置 =====
      WALLET_ADDRESS: "TRON:你的TRON地址,BSC:你的BSC地址"
      TG_BOT_GROUP_ID: "你的群组ID"
    volumes:
      - usdtmore_logs:/app/logs
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost:6080/ || exit 1"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    networks:
      - usdtmore_network

  # Redis缓存 (可选，高性能配置)
  redis:
    image: redis:7-alpine
    container_name: usdtmore_redis
    restart: always
    command: redis-server --appendonly yes --maxmemory 256mb --maxmemory-policy allkeys-lru
    volumes:
      - redis_data:/data
    networks:
      - usdtmore_network

volumes:
  postgres_data:
  redis_data:
  usdtmore_logs:

networks:
  usdtmore_network:
    driver: bridge
```

## 🗄️ 数据库选择指南 (2025.09更新)

### SQLite（默认推荐）
**适用场景**：个人使用、小规模部署
- ✅ **零配置**，开箱即用
- ✅ **轻量级**，资源占用少
- ✅ **简单部署**，单容器搞定
- ⚠️ 并发限制，适合日订单量 < 1000

### PostgreSQL（高性能推荐）
**适用场景**：生产环境、高并发场景
- ✅ **高并发**，支持100+同时连接
- ✅ **数据完整性**，ACID事务保证
- ✅ **扩展性强**，支持分片和复制
- ✅ **监控完善**，详细的性能指标
- ⚠️ 配置相对复杂，需要独立数据库服务

## 🚀 部署命令

### SQLite版本部署
```bash
# 启动SQLite版本
docker-compose up -d

# 查看日志
docker-compose logs -f usdtmore

# 停止服务
docker-compose down
```

### PostgreSQL版本部署
```bash
# 创建配置目录
mkdir -p config

# 复制PostgreSQL配置文件（可选，使用默认配置也可以）
# cp docker-compose.postgresql.yml docker-compose.yml

# 启动PostgreSQL版本
docker-compose -f docker-compose.postgresql.yml up -d

# 查看服务状态
docker-compose -f docker-compose.postgresql.yml ps

# 查看日志
docker-compose -f docker-compose.postgresql.yml logs -f

# 停止服务
docker-compose -f docker-compose.postgresql.yml down
```

## 🔧 环境变量配置说明

### 必需配置项 ⚠️

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `TG_BOT_TOKEN` | Telegram机器人Token | `6123456789:AAEhBOweik6ad6PsLMuhl3oifns...` |
| `TG_BOT_ADMIN_ID` | Telegram管理员ID | `123456789` |
| `ETHERSCAN_API_KEY` | EVM链统一API密钥 | `ABCD1234EFGH5678` |
| `TRON_SCAN_API_KEY` | TRON链API密钥 | `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` |

### 数据库配置 (2025.09新增)

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `DB_TYPE` | `sqlite` | 数据库类型：`sqlite` 或 `postgresql` |
| `DB_HOST` | `localhost` | PostgreSQL主机地址 |
| `DB_PORT` | `5432` | PostgreSQL端口 |
| `DB_USER` | `postgres` | PostgreSQL用户名 |
| `DB_PASSWORD` | - | PostgreSQL密码 |
| `DB_NAME` | `usdtmore` | PostgreSQL数据库名 |
| `DB_MAX_IDLE_CONNS` | `10` | 最大空闲连接数 |
| `DB_MAX_OPEN_CONNS` | `100` | 最大开放连接数 |

### 可选配置项

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `SOLANA_API_KEY` | - | Solana链API密钥（如需使用SOL链） |
| ~~`APTOS_API_KEY`~~ | - | ~~Aptos链API密钥~~（**无需设置**，官方API公开免费） |
| `AUTH_TOKEN` | `123456` | API认证Token |
| `APP_URI` | 自动检测 | 应用访问域名 |
| `EXPIRE_TIME` | `1800` | 订单过期时间(秒) |
| `PAYMENT_AMOUNT_RANGE` | `0.01,99999` | 支付金额范围 |

## 🔗 支持的区块链网络

系统支持以下8条区块链网络，手续费对比：

| 链标识 | 区块链网络 | 代币类型 | 手续费估算 | 所需API Key |
|--------|-----------|----------|-----------|-------------|
| `TRON` | TRON | USDT-TRC20 | ~1-2 USDT | `TRON_SCAN_API_KEY` |
| `POLY` | Polygon | USDT-ERC20 | ~0.001-0.01 USDT | `ETHERSCAN_API_KEY` |
| `OP` | Optimism | USDT-ERC20 | ~0.0001-0.001 USDT | `ETHERSCAN_API_KEY` |
| `BSC` | BSC | USDT-BEP20 | ~0.1-0.3 USDT | `ETHERSCAN_API_KEY` |
| `ARB` | Arbitrum One | USDT-ERC20 | ~0.0001-0.001 USDT | `ETHERSCAN_API_KEY` |
| `XLAYER` | X-Layer | USDT | ~极低 | `ETHERSCAN_API_KEY` |
| `SOL` | Solana | USDT-SPL | ~0.000005 SOL | `SOLANA_API_KEY` |
| `APT` | Aptos | USDT | ~0.0001 APT | **无需API Key** |

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
1. 访问 [Tronscan](https://tronscan.org/)
2. 注册账号并登录
3. 在用户中心创建API Key
4. 免费套餐：10万次/天

### Solana API Key
1. 访问 [Solscan.io](https://solscan.io/)
2. 注册开发者账号
3. 创建API Key

### ~~Aptos API Key~~
**Aptos链使用官方公开API，无需申请API Key**
- API地址：`https://fullnode.mainnet.aptoslabs.com`
- 免费使用，无需认证

## 🚦 部署验证

部署完成后：

1. **检查服务状态**：
```bash
# SQLite版本
docker-compose ps

# PostgreSQL版本
docker-compose -f docker-compose.postgresql.yml ps
```

2. **访问Web界面**：
```
http://你的服务器IP:6080
```

3. **查看运行日志**：
```bash
# SQLite版本
docker-compose logs -f usdtmore

# PostgreSQL版本
docker-compose -f docker-compose.postgresql.yml logs -f usdtmore
```

4. **测试数据库连接**（PostgreSQL版本）：
```bash
# 进入PostgreSQL容器
docker exec -it usdtmore_postgres psql -U usdtmore_user -d usdtmore

# 查看表结构
\dt
```

5. **测试机器人**：
向机器人发送 `/help` 命令

## 📊 性能监控 (2025.09更新)

### 查看系统资源使用
```bash
# 查看容器资源使用情况
docker stats

# 查看PostgreSQL连接数
docker exec -it usdtmore_postgres psql -U usdtmore_user -d usdtmore -c "SELECT count(*) FROM pg_stat_activity;"

# 查看应用日志中的性能信息
docker logs usdtmore | grep -i "performance\|connection\|pool"
```

### 监控指标说明
- **HTTP客户端性能提升**: 30-50%
- **CPU使用率优化**: 降低15-25%
- **内存使用优化**: 减少20-30%
- **并发处理能力**: 提升50-100%

## ⚠️ 重要提醒

- **所有API Key都是必需的**，缺失将导致对应链无法工作
- **定期备份数据库**：
  - SQLite：备份 `usdtmore_data` volume
  - PostgreSQL：使用 `pg_dump` 备份
- **确保服务器时间准确**，避免订单时间异常
- **生产环境建议**：
  - 使用PostgreSQL数据库
  - 配置HTTPS和反向代理
  - 设置防火墙规则
  - 定期更新镜像
- **监控API使用量**，避免超出免费额度

## 🔧 故障排除

### 常见问题

1. **容器启动失败**：
```bash
# 检查容器状态
docker ps -a

# 查看启动日志
docker logs usdtmore

# 检查环境变量
docker exec usdtmore env | grep -E "TG_BOT_TOKEN|DB_TYPE"
```

2. **数据库连接失败**（PostgreSQL）：
```bash
# 检查PostgreSQL容器状态
docker logs usdtmore_postgres

# 测试数据库连接
docker exec -it usdtmore_postgres pg_isready -U usdtmore_user

# 检查网络连通性
docker exec usdtmore ping postgres
```

3. **API调用失败**：
```bash
# 检查API Key设置
docker exec usdtmore env | grep API_KEY

# 测试网络连接
docker exec usdtmore curl -I https://api.etherscan.io/api
```

4. **机器人无响应**：
```bash
# 检查Telegram配置
docker exec usdtmore env | grep TG_BOT

# 查看机器人相关日志
docker logs usdtmore | grep -i telegram
```

5. **性能问题**：
```bash
# 查看资源使用
docker stats usdtmore

# 检查数据库连接池状态（PostgreSQL）
docker exec usdtmore curl -s http://localhost:6080/api/health

# 查看慢查询日志
docker logs usdtmore | grep -i "slow\|timeout"
```

### 数据库迁移

如需从SQLite迁移到PostgreSQL：

```bash
# 1. 停止当前服务
docker-compose down

# 2. 备份SQLite数据
docker run --rm -v usdtmore_data:/source -v $(pwd):/backup alpine cp -r /source /backup/sqlite_backup

# 3. 启动PostgreSQL版本
docker-compose -f docker-compose.postgresql.yml up -d

# 4. 使用迁移脚本（如有）
# 详见 MIGRATION_GUIDE.md
```

## 🔄 升级指南

### 升级到最新版本
```bash
# 1. 备份数据
docker-compose exec usdtmore cp -r /app/data /backup/

# 2. 停止服务
docker-compose down

# 3. 更新镜像
docker-compose pull

# 4. 启动服务
docker-compose up -d

# 5. 验证升级
docker logs -f usdtmore
```

需要更多帮助？
- 📖 查看完整文档：[项目README](../README.md)
- 🆘 故障排除：[MIGRATION_GUIDE.md](../MIGRATION_GUIDE.md)  
- 💬 加入交流群：[USDTMore](https://t.me/usdt_more)
- 🐛 提交Issue：[GitHub Issues](https://github.com/cjs520/USDTMore/issues)