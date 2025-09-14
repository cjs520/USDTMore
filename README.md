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

## 🛠️ 问题修复总结

本项目已修复了以下关键问题，确保系统稳定运行：

### ✅ 已修复的问题

#### 1. Docker构建问题
- **问题**: `no Go files in /build/main`
- **解决方案**: 修改Dockerfile构建路径从`./main`改为`.`

#### 2. 数据库连接问题
- **问题**: `tls error (server refused TLS connection)`
- **解决方案**: 将.env文件中的`DB_SSLMODE`设置为`disable`

#### 3. 模板路径问题
- **问题**: `panic: html/template: pattern matches no files`
- **解决方案**: 修复模板和静态文件路径配置

#### 4. 签名算法不匹配
- **问题**: ACG-FAKA插件与USDTMore签名验证失败
- **解决方案**: 修改签名算法从MD5改为HMAC-SHA256

#### 5. 数据库字段长度限制
- **问题**: `value too long for type character varying(34)`
- **解决方案**: 扩展地址字段到varchar(64)，哈希字段到varchar(128)

#### 6. 数据库缓存计划错误
- **问题**: `cached plan must not change result type`
- **解决方案**: 重启应用清除查询缓存

#### 7. 静态文件访问问题
- **问题**: CSS/JS文件返回404错误
- **解决方案**: 修复静态文件路径配置

#### 8. 订单金额计算失败
- **问题**: 无法计算可用的交易金额
- **解决方案**: 增加最大尝试次数到100,000

#### 9. 数据库唯一约束冲突
- **问题**: `could not create unique index "uni_trade_orders_trade_hash"`
- **解决方案**: 重新设计约束，支持并发订单创建

#### 10. EVM API统一配置问题
- **问题**: `Invalid API Key (#err2)|bsc5` - EVM链API密钥配置混乱
- **解决方案**: 统一使用`ETHERSCAN_API_KEY`支持所有EVM兼容链

#### 11. 并发订单创建问题
- **问题**: 多个订单同时创建时出现唯一约束冲突
- **解决方案**: 优化数据库约束设计，支持高并发场景

### 🔧 数据库初始化优化

所有数据库修复已整合到 `init.sql` 文件中，包括：

- **基础表结构创建** - 优化的字段长度和类型
- **智能数据修复** - 自动检测并修复现有数据
- **并发支持约束** - 支持高并发订单创建
- **性能优化索引** - 提升查询性能
- **自动化功能** - 触发器和权限设置

### 🚀 一键修复脚本

项目提供了多个自动化修复脚本：

```bash
# 完整系统修复
./restart_and_fix.sh

# 快速修复
./quick_fix.sh

# 并发问题专项修复
./concurrent_fix_complete.sh

# API配置测试
./test_api_config.sh
```

### 📊 验证修复效果

修复完成后，系统应该能够：

1. ✅ 正常启动和运行
2. ✅ 连接数据库无TLS错误
3. ✅ 正确显示页面样式
4. ✅ EVM链API调用正常（无"Invalid API Key"错误）
5. ✅ ACG-FAKA插件签名验证通过
6. ✅ 处理长钱包地址和交易哈希
7. ✅ 支持高并发订单创建
8. ✅ 数据库约束正常工作

## 📋 更新日志

### v2.1.0 (2025-01-15) - 稳定性大幅提升

#### 🔥 重大修复
- **完全解决并发问题**：重新设计数据库约束，支持高并发订单创建
- **EVM API统一配置**：所有EVM链使用统一的Etherscan V2 API
- **数据库字段优化**：支持长地址（64字符）和完整交易哈希（128字符）
- **签名算法升级**：ACG-FAKA插件升级到HMAC-SHA256算法

#### ✨ 新功能
- 🗄️ **数据库初始化优化**：所有修复整合到init.sql，自动应用
- 🔒 **增强安全配置**：更严格的数据库权限和连接安全
- 📊 **完善监控支持**：增加数据库性能监控和健康检查
- 🐳 **Docker优化**：修复容器配置，优化构建流程

#### ⚠️ 破坏性变更
- **不再支持SQLite**：现有SQLite用户需要迁移到PostgreSQL
- **环境变量变更**：移除 `DB_DIR` 配置项
- **默认数据库类型**：`DB_TYPE` 默认值从 `sqlite` 改为 `postgres`

#### 🛠️ 自动化工具
- **一键修复脚本**：提供多个自动化修复工具
- **API配置测试**：自动验证API密钥配置
- **数据库备份**：自动备份和恢复功能

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

**最后更新**: 2025年1月15日  
**版本**: v2.1.0  
**维护者**: USDTMore 开发团队