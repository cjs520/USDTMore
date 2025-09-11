# USDTMore PostgreSQL数据库部署指南

## 概述

本指南详细说明了如何在USDTMore项目中部署和配置PostgreSQL数据库，以获得更好的性能、可靠性和扩展性。

## 部署前准备

### 1. 环境要求

- PostgreSQL 13+ 
- Go 1.22+
- 足够的磁盘空间（推荐最小 500MB）
- 管理员权限

### 2. 准备数据迁移（如适用）

```bash
# 如果从旧系统迁移数据，建议先备份原始数据
mkdir -p /path/to/backup
pg_dump -U old_user -d old_database > database_backup_$(date +%Y%m%d_%H%M%S).sql
```

## 安装步骤

### 第一步：安装PostgreSQL

#### Ubuntu/Debian
```bash
sudo apt update
sudo apt install postgresql postgresql-contrib
```

#### CentOS/RHEL
```bash
sudo yum install postgresql-server postgresql-contrib
sudo postgresql-setup initdb
sudo systemctl enable postgresql
sudo systemctl start postgresql
```

#### Docker方式
```bash
# 使用提供的docker-compose配置
docker-compose -f docker-compose.postgresql.yml up -d postgres-primary
```

### 第二步：创建数据库和用户

```bash
# 切换到postgres用户
sudo -u postgres psql

# 创建数据库和用户
CREATE DATABASE usdtmore;
CREATE USER usdtmore_user WITH ENCRYPTED PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE usdtmore TO usdtmore_user;
GRANT ALL ON SCHEMA public TO usdtmore_user;
ALTER DATABASE usdtmore OWNER TO usdtmore_user;
\q
```

### 第三步：运行数据库初始化脚本

```bash
# 执行初始化脚本
psql -h localhost -p 5432 -U usdtmore_user -d usdtmore -f migrations/001_initial_schema.sql
```

### 第四步：配置环境变量

复制并修改环境变量配置：

```bash
cp .env.example .env
```

编辑 `.env` 文件：

```bash
# 数据库类型
DB_TYPE=postgresql

# PostgreSQL连接配置
DB_HOST=localhost
DB_PORT=5432
DB_USER=usdtmore_user
DB_PASSWORD=your_secure_password
DB_NAME=usdtmore
DB_SSLMODE=disable

# 连接池配置
DB_MAX_IDLE_CONNS=10
DB_MAX_OPEN_CONNS=100
DB_CONN_MAX_LIFETIME=5m
DB_CONN_MAX_IDLE_TIME=1m

# 调试模式（生产环境设置为false）
DB_DEBUG=false
```

### 第五步：更新Go依赖

```bash
go mod tidy
```

### 第六步：数据迁移

#### 方法1：使用自动迁移工具

```bash
# 导出SQLite数据
./scripts/export_sqlite_data.sh

# 导入到PostgreSQL
./scripts/migrate_to_postgresql.sql
```

#### 方法2：手动迁移

1. **导出SQLite数据为CSV：**

```bash
# 导出钱包地址表
sqlite3 -header -csv usdtmore.db "SELECT * FROM wallet_address;" > wallet_address.csv

# 导出交易订单表
sqlite3 -header -csv usdtmore.db "SELECT * FROM trade_orders;" > trade_orders.csv

# 导出通知记录表  
sqlite3 -header -csv usdtmore.db "SELECT * FROM notify_record;" > notify_record.csv
```

2. **导入到PostgreSQL：**

```sql
-- 连接到PostgreSQL数据库
\c usdtmore

-- 导入数据
COPY wallet_address FROM '/path/to/wallet_address.csv' WITH (FORMAT csv, HEADER true);
COPY trade_orders FROM '/path/to/trade_orders.csv' WITH (FORMAT csv, HEADER true);
COPY notify_record FROM '/path/to/notify_record.csv' WITH (FORMAT csv, HEADER true);

-- 重置序列
SELECT setval('wallet_address_id_seq', (SELECT MAX(id) FROM wallet_address));
SELECT setval('trade_orders_id_seq', (SELECT MAX(id) FROM trade_orders));
```

### 第七步：验证迁移

```bash
# 运行健康检查
./scripts/health_check.sh

# 验证数据完整性
psql -h localhost -p 5432 -U usdtmore_user -d usdtmore -c "
SELECT 'wallet_address' as table_name, COUNT(*) as record_count FROM wallet_address
UNION ALL
SELECT 'trade_orders' as table_name, COUNT(*) as record_count FROM trade_orders  
UNION ALL
SELECT 'notify_record' as table_name, COUNT(*) as record_count FROM notify_record;
"
```

### 第八步：启动应用

```bash
# 测试应用启动
go run main.go

# 检查日志确认PostgreSQL连接成功
tail -f /var/log/usdtmore/app.log
```

## 自动化运维设置

### 1. 设置自动备份

```bash
# 设置定时任务
./scripts/setup_crontab.sh

# 手动执行备份测试
./scripts/backup.sh
```

### 2. 设置监控

```bash
# 启动完整监控栈（可选）
docker-compose -f docker-compose.postgresql.yml --profile monitoring up -d

# 访问Grafana监控面板
# http://localhost:3000 (admin/admin123)
```

### 3. 性能优化

编辑PostgreSQL配置文件 `config/postgresql.conf` 根据服务器硬件调整：

```bash
# 重新加载配置
sudo systemctl reload postgresql
# 或Docker环境
docker-compose -f docker-compose.postgresql.yml restart postgres-primary
```

数据类型优化说明n
### PostgreSQL数据类型特性n
| 字段类型 | PostgreSQL类型 | 说明 |n
|----------|----------------|------|n
| 主键 | BIGSERIAL PRIMARY KEY | 大规模自增主键 |n
| 金额 | DECIMAL(18,6) | 高精度小数存储 |n
| 字符串 | VARCHAR(255) | 灵活字符串处理 |n
| 布尔值 | SMALLINT | 使用小整数表示布尔值 |n
| 时间戳 | TIMESTAMP WITH TIME ZONE | 带时区的时间戳 |n
### 数据类型优化建议n
1. **金额精度**：使用DECIMAL(18,6)确保财务数据精确计算n
2. **时间戳一致性**：使用带时区的时间戳，支持全球业务n
3. **布尔值处理**：通过CHECK约束增强数据验证n
4. **ID空间**：BIGSERIAL支持大规模数据增长n


