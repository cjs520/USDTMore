# Docker 部署教程 🐳

## 前置要求

确保你的服务器已安装：
- Docker (版本 20.10+)
- Docker Compose (版本 2.0+)

如果未安装，请参考 [Docker官方文档](https://docs.docker.com/get-docker/) 进行安装。

## 🚀 快速部署

### 方法一：PostgreSQL部署

```bash
# 1. 启动PostgreSQL数据库
docker run -d \
  --name postgres \
  -e POSTGRES_DB=usdtmore \
  -e POSTGRES_USER=usdtmore \
  -e POSTGRES_PASSWORD=your_password \
  -p 5432:5432 \
  postgres:15

# 2. 启动USDTMore
docker run -d --restart=always --name usdtmore -p 6080:6080 \
  -e TG_BOT_TOKEN=你的机器人Token \
  -e TG_BOT_ADMIN_ID=你的管理员ID \
  -e AUTH_TOKEN=你的验证密钥 \
  -e APP_URI=https://你的域名.com \
  -e REWRITE_HTTPS=true \
  -e DB_HOST=postgres \
  -e DB_USER=usdtmore \
  -e DB_PASSWORD=your_password \
  -e DB_NAME=usdtmore \
  -e ETHERSCAN_API_KEY=你的Etherscan_API_Key \
  -e TRON_SCAN_API_KEY=你的TRON_SCAN_API_KEY \
  -e SOLANA_API_KEY=你的Solana_API_Key \
  --link postgres:postgres \
  zxzx412/usdtmore:latest

# 3. 查看运行状态
docker logs -f usdtmore
```

### 方法二：Docker Compose（推荐）

#### PostgreSQL版本 (docker-compose.yml)

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
      DB_HOST: "postgres"
      DB_PORT: "5432"
      DB_USER: "usdtmore"
      DB_PASSWORD: "your_password"
      DB_NAME: "usdtmore"
      
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
      - usdtmore_logs:/app/logs             # 日志持久化
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD-SHELL", "curl -f http://localhost:6080/ || exit 1"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    networks:
      - usdtmore_network

  postgres:
    image: postgres:15-alpine
    container_name: usdtmore_postgres
    restart: always
    environment:
      POSTGRES_DB: usdtmore
      POSTGRES_USER: usdtmore
      POSTGRES_PASSWORD: your_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U usdtmore"]
      interval: 10s
      timeout: 5s
      retries: 5
    networks:
      - usdtmore_network

volumes:
  postgres_data:
  usdtmore_logs:

networks:
  usdtmore_network:
    driver: bridge
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

## 🗄️ PostgreSQL数据库选择指南 (2025.09更新)

### PostgreSQL（标准推荐）
**适用场景**：中小型生产环境
- ✅ **高并发**，支持50+同时连接
- ✅ **数据完整性**，ACID事务保证
- ✅ **备份恢复**，完善的数据保护
- ⚠️ 配置相对复杂，需要独立数据库服务

### 生产级高可用（企业推荐）
**适用场景**：大型生产环境、关键业务
- ✅ **超高并发**，支持200+同时连接
- ✅ **99.9%可用性**，主从复制保障
- ✅ **完整监控**，实时性能分析
- ✅ **自动故障转移**，无人值守运维
- ✅ **负载均衡**，多实例分流
- ⚠️ 资源占用较大，配置复杂度高

#### 📊 PostgreSQL配置对比表

| 特性 | PostgreSQL标准 | 生产级高可用 |
|------|---------------|-------------|
| **部署复杂度** | ⭐⭐ | ⭐⭐⭐⭐ |
| **资源占用** | 200-500MB | 1-2GB |
| **并发支持** | 50+ | 200+ |
| **可用性** | 99% | 99.9% |
| **监控能力** | 标准 | 企业级 |
| **故障恢复** | 半自动 | 全自动 |
| **适用订单量** | < 5000/日 | 无限制 |

## 🚀 部署命令

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

### 🏭 生产级高可用部署

对于大型生产环境，我们提供了完整的企业级解决方案：

#### 生产级配置特性
- ✅ **PostgreSQL主从复制** - 99.9%高可用性
- ✅ **PgBouncer连接池** - 支持200+并发连接
- ✅ **Redis高性能缓存** - 提升响应速度
- ✅ **Nginx负载均衡** - 多实例负载分担
- ✅ **Prometheus + Grafana** - 完整监控体系
- ✅ **自动健康检查** - 故障自动恢复
- ✅ **完整日志管理** - 便于问题排查

#### 部署命令

```bash
# 1. 创建必要的配置目录
mkdir -p config monitoring ssl scripts

# 2. 创建环境变量文件
cat > .env << EOF
# 数据库密码
DB_PASSWORD=your_very_secure_password_here
REPLICA_PASSWORD=replica_secure_password_here

# Telegram配置
TG_BOT_TOKEN=your_telegram_bot_token
TG_BOT_ADMIN_ID=your_telegram_admin_id

# API Keys
ETHERSCAN_API_KEY=your_etherscan_api_key
TRON_SCAN_API_KEY=your_tron_scan_api_key
SOLANA_API_KEY=your_solana_api_key

# 应用配置
AUTH_TOKEN=your_auth_token
APP_URI=https://your-domain.com

# 监控配置
GRAFANA_PASSWORD=secure_grafana_password
REDIS_PASSWORD=redis_secure_password

# 钱包地址
WALLET_ADDRESS=TRON:your_tron_address,BSC:your_bsc_address
TG_BOT_GROUP_ID=your_group_id
EOF

# 3. 启动完整生产环境
docker-compose -f docker-compose.production.yml up -d

# 4. 仅启动核心服务（推荐首次部署）
docker-compose -f docker-compose.production.yml up -d postgres-primary pgbouncer usdtmore-primary redis

# 5. 启用读写分离（可选）
docker-compose -f docker-compose.production.yml --profile replica up -d

# 6. 启用完整监控系统
docker-compose -f docker-compose.production.yml --profile monitoring up -d

# 7. 查看所有服务状态
docker-compose -f docker-compose.production.yml ps

# 8. 查看服务日志
docker-compose -f docker-compose.production.yml logs -f usdtmore-primary

# 9. 停止服务
docker-compose -f docker-compose.production.yml down
```

#### 服务访问地址

部署完成后，可通过以下地址访问各项服务：

| 服务 | 访问地址 | 用途 |
|------|---------|------|
| **USDTMore主服务** | `http://localhost:6080` | 支付网关主界面 |
| **USDTMore副本** | `http://localhost:6081` | 只读副本（可选） |
| **Grafana监控** | `http://localhost:3000` | 数据可视化监控面板 |
| **Prometheus** | `http://localhost:9090` | 指标收集和查询 |
| **AlertManager** | `http://localhost:9093` | 告警管理 |
| **PgBouncer** | `localhost:6432` | 数据库连接池 |
| **PostgreSQL主库** | `localhost:5432` | 主数据库 |
| **PostgreSQL副本** | `localhost:5433` | 只读副本 |
| **Redis缓存** | `localhost:6379` | 缓存服务 |

#### 监控指标说明

**核心业务指标**：
- 订单处理量（TPS）
- 支付成功率
- 平均响应时间
- 区块链确认时间

**系统性能指标**：
- CPU使用率
- 内存使用率
- 磁盘I/O
- 网络流量

**数据库指标**：
- 连接数
- 查询性能
- 缓存命中率
- 慢查询统计

#### 故障转移说明

**自动故障转移场景**：
1. **主数据库故障** → 自动切换到只读副本
2. **应用实例故障** → Nginx自动路由到健康实例
3. **Redis缓存故障** → 直接访问数据库（性能降级）
4. **网络连接异常** → 自动重连机制

**手动故障处理**：
```bash
# 检查服务健康状态
docker-compose -f docker-compose.production.yml exec usdtmore-primary curl http://localhost:6080/health

# 重启指定服务
docker-compose -f docker-compose.production.yml restart usdtmore-primary

# 查看详细日志
docker-compose -f docker-compose.production.yml logs --tail=100 -f postgres-primary

# 手动切换到副本实例
docker-compose -f docker-compose.production.yml stop usdtmore-primary
docker-compose -f docker-compose.production.yml scale usdtmore-replica=1
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