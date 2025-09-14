# USDTMore (USDT Payment Gateway for More Chain)

<p align="center">
<img src="./static/img/tether.svg" width="15%" alt="tether">
</p>
<p align="center">
<a href="https://www.gnu.org/licenses/gpl-3.0.html"><img src="https://img.shields.io/badge/license-GPLV3-blue" alt="license GPLV3"></a>
<a href="https://golang.org"><img src="https://img.shields.io/badge/Golang-1.22-red" alt="Go version 1.21"></a>
<a href="https://github.com/gin-gonic/gin"><img src="https://img.shields.io/badge/Gin-v1.9-blue" alt="Gin Web Framework v1.9"></a>
<a href="https://github.com/go-telegram-bot-api/telegram-bot-api"><img src="https://img.shields.io/badge/Telegram Bot-v5-lightgrey" alt="Golang Telegram Bot Api-v5"></a>
<a href="https://github.com/v03413/bepusdt"><img src="https://img.shields.io/badge/Release-v2.1.0-green" alt="Release v2.1.0"></a>
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
- 🔥 **全新安全性增强**：HMAC-SHA256签名算法，回调URL安全验证
- 🔥 **可靠性改进**：数据库事务保护，并发安全处理，智能重试机制
- 🔥 **统一HTTP客户端**：支持重试、超时控制、连接池管理
- 🔥 **数据库迁移**：完全移除SQLite支持，仅支持PostgreSQL
- 🔥 **数据类型修复**：修复MySQL特有数据类型兼容性问题

## 🚀 快速部署

### 方法一：Docker 部署（推荐）

```bash
# 1. 克隆项目
git clone https://github.com/cjs520/USDTMore.git
cd USDTMore

# 2. 配置环境变量
cp docs/usdtmore.conf .env
# 编辑 .env 文件，修改必要配置

# 3. 启动服务
docker-compose up -d

# 4. 查看状态
docker-compose ps
```

### 方法二：手动部署

```bash
# 1. 安装PostgreSQL
sudo apt install postgresql postgresql-contrib

# 2. 创建数据库
sudo -u postgres createdb usdtmore
sudo -u postgres createuser usdtmore

# 3. 下载应用
wget https://github.com/cjs520/USDTMore/releases/latest/download/usdtmore-linux-amd64
chmod +x usdtmore-linux-amd64

# 4. 配置并启动
cp docs/usdtmore.conf /etc/usdtmore/
# 编辑配置文件后启动
./usdtmore-linux-amd64
```

## 🛠 参数配置

USDTMore 所有参数都是以传递环境变量的方式进行配置，大部分参数含默认值，少量配置即可直接使用！

### 必需配置项

| 参数名称 | 说明 | 示例值 |
|---------|------|--------|
| `AUTH_TOKEN` | 🔒 **认证Token**，**强烈建议设置32位以上强密钥** | `your_32_char_secure_token_here` |
| `TG_BOT_TOKEN` | Telegram Bot Token（**必需**） | `6123456789:AAEhBOweik6ad6PsLMuhl3oifns...` |
| `TG_BOT_ADMIN_ID` | Telegram Bot 管理员ID（**必需**） | `123456789` |
| `ETHERSCAN_API_KEY` | EVM链统一API密钥（**必需**） | `ABCD1234EFGH5678` |
| `TRON_GRID_API_KEY` | TRON Grid API密钥（**必需**） | `xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx` |
| `DB_PASSWORD` | PostgreSQL数据库密码（**必需**） | `your_secure_db_password` |

### 数据库配置（PostgreSQL）

| 参数名称 | 默认值 | 说明 |
|---------|--------|------|
| `DB_TYPE` | `postgres` | 数据库类型（仅支持PostgreSQL） |
| `DB_HOST` | `localhost` | 数据库主机地址 |
| `DB_PORT` | `5432` | 数据库端口 |
| `DB_NAME` | `usdtmore` | 数据库名称 |
| `DB_USER` | `usdtmore` | 数据库用户名 |
| `DB_SSLMODE` | `disable` | SSL模式 |
| `DB_TIMEZONE` | `Asia/Shanghai` | 数据库时区 |

### 应用配置

| 参数名称 | 默认值 | 说明 |
|---------|--------|------|
| `LISTEN` | `:6080` | 服务器HTTP监听地址 |
| `EXPIRE_TIME` | `1800` | 订单有效期，单位秒（默认30分钟） |
| `USDT_RATE` | `空` | USDT汇率，默认留空则获取Okx交易所的汇率 |
| `REWRITE_HTTPS` | `false` | 重写http成https，使用反向代理时需要 |
| `TRADE_IS_CONFIRMED` | `0` | TRON网络是否需要确认 |
| `ETH_CONFIRMATION` | `0` | ETH兼容网络需要网络确认的块数 |
| `APP_URI` | `空` | 应用访问地址，建议设置 |
| `WALLET_ADDRESS` | `空` | 启动时需要添加的钱包地址 |

### API密钥配置

| API服务 | 环境变量 | 获取地址 | 用途 |
|---------|----------|----------|------|
| Etherscan | `ETHERSCAN_API_KEY` | [Etherscan](https://etherscan.io/apis) | EVM链统一API（BSC、Polygon、Optimism等） |
| TRON Grid | `TRON_GRID_API_KEY` | [TronGrid](https://www.trongrid.io/) | TRON链API密钥 |
| TRON Scan | `TRON_SCAN_API_KEY` | [TronScan](https://tronscan.org/) | TRON链扫描API |
| Solana | `SOLANA_API_KEY` | [Solana Docs](https://docs.solana.com/api) | Solana链API（可选） |

### 支持的区块链网络

| 链标识 | 网络名称 | 代币类型 | 所需API密钥 |
|--------|----------|----------|-------------|
| `TRON` | TRON | USDT-TRC20 | `TRON_GRID_API_KEY` |
| `POLY` | Polygon | USDT-ERC20 | `ETHERSCAN_API_KEY` |
| `OP` | Optimism | USDT-ERC20 | `ETHERSCAN_API_KEY` |
| `BSC` | BSC | USDT-BEP20 | `ETHERSCAN_API_KEY` |
| `ARB` | Arbitrum One | USDT-ERC20 | `ETHERSCAN_API_KEY` |
| `XLAYER` | X-Layer | USDT | `ETHERSCAN_API_KEY` |
| `SOL` | Solana | USDT-SPL | `SOLANA_API_KEY` |
| `APT` | Aptos | USDT | **无需API Key** |

## 🔒 安全配置

### 生产环境必备

1. **强密钥设置**：
   ```bash
   AUTH_TOKEN=your_very_secure_32_char_token_here
   DB_PASSWORD=your_very_secure_database_password
   ```

2. **HTTPS配置**：
   ```bash
   REWRITE_HTTPS=true
   APP_URI=https://your-domain.com
   ```

3. **数据库安全**：
   ```bash
   DB_SSLMODE=require  # 生产环境启用SSL
   ```

### SSL证书配置

#### 使用Cloudflare（推荐）
1. 设置DNS解析到你的服务器
2. 在Cloudflare中设置SSL/TLS模式为"灵活"
3. 开启代理（小云朵）

#### 使用Let's Encrypt
```bash
# 安装certbot
sudo apt install certbot python3-certbot-nginx

# 获取证书
sudo certbot --nginx -d your-domain.com
```

### 防火墙配置
```bash
# 配置UFW防火墙
sudo ufw allow ssh
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw deny 6080/tcp  # 不直接暴露应用端口
sudo ufw enable
```

## 📊 监控和维护

### 健康检查
```bash
# 检查应用状态
curl -H "Authorization: Bearer your_auth_token" \
     http://localhost:6080/api/health
```

### 日志查看
```bash
# 应用日志
tail -f /var/log/usdtmore/usdtmore.log

# Docker日志
docker-compose logs -f usdtmore
```

### 数据库备份
```bash
# 备份数据库
pg_dump -h localhost -U usdtmore -d usdtmore > backup.sql

# 恢复数据库
psql -h localhost -U usdtmore -d usdtmore < backup.sql
```

### 时间同步（重要）
```bash
# 安装时间同步服务
sudo apt install systemd-timesyncd -y
sudo systemctl enable systemd-timesyncd.service
sudo systemctl start systemd-timesyncd.service

# 检查同步状态
timedatectl
```

## 🔧 故障排除

### 常见问题

1. **数据库连接失败**
   ```bash
   # 检查PostgreSQL状态
   sudo systemctl status postgresql
   
   # 检查连接
   psql -h localhost -U usdtmore -d usdtmore
   ```

2. **API调用失败**
   - 检查API密钥是否正确
   - 验证网络连接
   - 查看应用日志

3. **端口访问问题**
   ```bash
   # 检查端口监听
   sudo netstat -tlnp | grep 6080
   
   # 检查防火墙
   sudo ufw status
   ```

## 📋 更新日志

### v2.1.0 (2025-01-14)

#### 🔥 重大变更
- **完全移除SQLite支持**：项目现在仅支持PostgreSQL数据库
- **数据库字段优化**：修复MySQL特有类型，完全兼容PostgreSQL
- **交易哈希长度修复**：支持以太坊完整交易哈希（66字符）
- **除零错误防护**：修复汇率计算中的潜在除零错误

#### ✨ 新功能
- 🗄️ **PostgreSQL专用优化**：针对PostgreSQL进行性能和安全优化
- 🔒 **增强安全配置**：更严格的数据库权限和连接安全
- 📊 **完善监控支持**：增加数据库性能监控和健康检查
- 🐳 **Docker优化**：修复容器端口配置，使用distroless基础镜像

#### ⚠️ 破坏性变更
- **不再支持SQLite**：现有SQLite用户需要迁移到PostgreSQL
- **环境变量变更**：移除 `DB_DIR` 配置项
- **默认数据库类型**：`DB_TYPE` 默认值从 `sqlite` 改为 `postgres`

## 🆘 获取帮助

### 技术支持
- **GitHub Issues**: [提交问题](https://github.com/cjs520/USDTMore/issues)
- **Telegram群组**: [USDTMore交流群](https://t.me/usdt_more)

### 报告问题时请提供
1. **系统信息**: 操作系统版本、Docker版本等
2. **错误日志**: 完整的错误信息和日志
3. **配置信息**: 相关配置（隐藏敏感信息）
4. **复现步骤**: 详细的问题复现步骤

## 🤝 贡献指南

欢迎提交Issue和Pull Request来帮助改进项目！

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用 GPL-3.0 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

---

**最后更新**: 2025年1月14日  
**版本**: v2.1.0  
**维护者**: USDTMore 开发团队