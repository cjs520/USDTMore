# USDTMore - USDT支付网关系统

USDTMore 是一个支持多链USDT支付的网关系统，支持TRON、Polygon、Optimism、BSC、Arbitrum、X-Layer、Solana、Aptos等多个区块链网络。

## 功能特性

- 🌐 **多链支持**：支持8个主流区块链网络的USDT支付
- 🔒 **安全可靠**：使用HMAC-SHA256签名算法，确保交易安全
- 📱 **Telegram集成**：支持Telegram机器人通知和管理
- 🔄 **实时监控**：自动监控交易状态，实时更新订单
- 🎯 **精确匹配**：智能金额匹配算法，避免重复支付
- 📊 **完整日志**：详细的交易日志和错误追踪

## 系统修复说明

本系统已修复以下9个关键问题：

### ✅ 已修复的问题

1. **Docker构建问题** - 修正构建路径和模板复制
2. **数据库连接问题** - 配置正确的SSL模式 (DB_SSLMODE=disable)
3. **模板路径问题** - 正确设置模板目录
4. **签名算法不匹配** - 使用HMAC-SHA256算法
5. **数据库字段长度限制** - 扩展地址字段到64位
6. **数据库缓存计划错误** - 重启应用清除缓存
7. **静态文件访问问题** - 正确配置静态文件路径
8. **订单金额计算失败** - 增加最大尝试次数到100,000
9. **数据库唯一约束冲突** - 清理重复数据

## 快速开始

### Docker 部署（推荐）

1. **克隆项目并配置**
```bash
git clone <repository-url>
cd USDTMore
cp .env.example .env
# 编辑 .env 文件，配置必要的参数
```

2. **启动服务**
```bash
docker-compose up -d
```

3. **初始化数据库**
```bash
docker exec -i usdtmore_postgres psql -U usdtmore -d usdtmore < init.sql
```

### 关键配置

#### 数据库配置
```bash
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5432
DB_NAME=usdtmore
DB_USER=usdtmore
DB_PASSWORD=your_password_here
# 关键配置：禁用SSL以避免TLS错误
DB_SSLMODE=disable
DB_TIMEZONE=Asia/Shanghai
```

#### API密钥配置
```bash
# TRON网络API密钥（至少需要一个）
TRON_SCAN_API_KEY=your_tronscan_api_key
TRON_GRID_API_KEY=your_trongrid_api_key

# EVM兼容链统一API密钥（Etherscan V2 API）
ETHERSCAN_API_KEY=your_etherscan_api_key

# Solana和Aptos网络API密钥（可选）
SOLANA_API_KEY=your_solana_api_key
APTOS_API_KEY=your_aptos_api_key
```

#### 安全配置
```bash
# 授权令牌（必须修改默认值）
AUTH_TOKEN=your_secure_random_token_here_at_least_16_chars

# HTTPS配置（生产环境推荐）
FORCE_HTTPS=true
REWRITE_HTTPS=true
ENVIRONMENT=production
```

## Docker 配置优化

### Dockerfile
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

# 复制二进制文件
COPY --from=builder /build/main .

# 复制模板和静态文件
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/static ./static

# 设置环境变量
ENV HTML_DIR=/app

EXPOSE 6080
CMD ["./main"]
```

### Docker Compose
```yaml
version: '3.8'
services:
  usdtmore:
    build: .
    ports:
      - "6080:6080"
    environment:
      - DB_HOST=postgres
      - DB_SSLMODE=disable
    env_file:
      - .env
    depends_on:
      - postgres
    volumes:
      - ./logs:/app/logs

  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: usdtmore
      POSTGRES_USER: usdtmore
      POSTGRES_PASSWORD: your_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

volumes:
  postgres_data:
```

## 支持的区块链网络

| 网络 | 代码 | USDT合约地址 | 浏览器 |
|------|------|-------------|--------|
| TRON | TRON | TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t | tronscan.org |
| Polygon | POLY | 0xc2132D05D31c914a87C6611C10748AEb04B58e8F | polygonscan.com |
| Optimism | OP | 0x94b008aA00579c1307B0EF2c499aD98a8ce58e58 | optimistic.etherscan.io |
| BSC | BSC | 0x55d398326f99059fF775485246999027B3197955 | bscscan.com |
| Arbitrum | ARB | 0xFd086bC7CD5C481DCC9C85ebE478A1C0b69FCbb9 | arbiscan.io |
| X-Layer | XLAYER | 0x1e4a5963abfd975d8c9021ce480b42188849d41d | oklink.com |
| Solana | SOL | Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB | solscan.io |
| Aptos | APT | 0xf22bede237a07e121b56d91a491eb7bcdfd1f5907926a9e58338f964a01b17fa::asset::USDT | explorer.aptoslabs.com |

## 系统修复步骤

### 1. 停止服务
```bash
docker-compose down
```

### 2. 数据库修复
```bash
psql -h localhost -U usdtmore -d usdtmore -f init.sql
```

### 3. 环境配置修复
```bash
# 备份现有配置
cp .env .env.backup

# 更新关键配置
# DB_SSLMODE=disable
# AUTH_TOKEN=your_secure_token
# 添加必需的API密钥
```

### 4. 重新构建和启动
```bash
docker-compose build --no-cache
docker-compose up -d
```

### 5. 验证修复结果
```bash
# 检查服务状态
curl -I http://localhost:6080

# 检查静态文件访问
curl -I http://localhost:6080/static/css/style.css

# 查看应用日志
docker-compose logs -f usdtmore
```

## 故障排查

### 常见问题解决

#### 数据库连接失败
```bash
# 检查数据库服务
docker-compose ps postgres

# 确保配置正确
DB_SSLMODE=disable
```

#### 静态文件404
```bash
# 检查环境变量
HTML_DIR=/app

# 检查文件存在
docker exec -it usdtmore_app ls -la /app/static/
```

#### API调用失败
```bash
# 检查API密钥配置
ETHERSCAN_API_KEY=your_key
TRON_SCAN_API_KEY=your_key
```

#### 订单创建失败
```bash
# 检查钱包地址
curl http://localhost:6080/api/addresses

# 清理过期订单
psql -c "UPDATE trade_orders SET status = 3 WHERE status = 1 AND created_at < NOW() - INTERVAL '24 hours';"
```

## 健康检查

```bash
# 创建健康检查脚本
cat > health-check.sh << 'EOF'
#!/bin/bash
echo "=== USDTMore Health Check ==="
curl -s http://localhost:6080/health || echo "Service DOWN"
psql -h localhost -U usdtmore -d usdtmore -c "SELECT COUNT(*) FROM trade_orders WHERE status = 1;" 2>/dev/null || echo "Database DOWN"
EOF

chmod +x health-check.sh
./health-check.sh
```

## 定期维护

```bash
# 创建维护脚本
cat > maintenance.sh << 'EOF'
#!/bin/bash
echo "=== USDTMore Maintenance ==="

# 清理过期订单
psql -h localhost -U usdtmore -d usdtmore -c "
UPDATE trade_orders SET status = 3, updated_at = NOW() 
WHERE status = 1 AND created_at < NOW() - INTERVAL '24 hours';"

# 清理旧通知记录
psql -h localhost -U usdtmore -d usdtmore -c "
DELETE FROM notify_records WHERE created_at < NOW() - INTERVAL '7 days';"

# 更新数据库统计
psql -h localhost -U usdtmore -d usdtmore -c "ANALYZE;"

echo "Maintenance completed at $(date)"
EOF

chmod +x maintenance.sh

# 添加到crontab（每天凌晨2点执行）
echo "0 2 * * * /path/to/maintenance.sh >> /var/log/usdtmore-maintenance.log 2>&1" | crontab -
```

## 安全建议

1. **更改默认密钥**：确保修改 `AUTH_TOKEN` 为强密码
2. **API密钥保护**：妥善保管各区块链网络的API密钥
3. **HTTPS部署**：生产环境启用HTTPS
4. **防火墙配置**：限制数据库端口访问
5. **定期备份**：定期备份数据库和配置文件
6. **日志监控**：监控异常访问和错误日志

## 更新日志

### v1.1.0 (2024-01-15)
- ✅ 修复签名算法不匹配问题
- ✅ 更新API调用格式为Etherscan V2
- ✅ 优化订单金额计算算法
- ✅ 扩展数据库地址字段长度
- ✅ 修复Docker构建和模板路径问题
- ✅ 改进错误处理和日志记录

### v1.0.0 (2024-01-01)
- 🎉 初始版本发布
- 🌐 支持8个区块链网络
- 🔒 HMAC-SHA256签名验证
- 📱 Telegram机器人集成
- 🔄 实时交易监控

## 许可证

本项目采用 MIT 许可证。