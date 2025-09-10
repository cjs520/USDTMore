# USDTMore SQLite到PostgreSQL迁移指南

## 概述

本指南详细说明了如何将USDTMore项目从SQLite数据库迁移到PostgreSQL数据库，以获得更好的性能、可靠性和扩展性。

## 迁移前准备

### 1. 环境要求

- PostgreSQL 13+ 
- Go 1.22+
- 足够的磁盘空间（至少是当前SQLite数据库的2倍）
- 管理员权限

### 2. 备份当前数据

```bash
# 备份当前SQLite数据库
cp /path/to/usdtmore.db /path/to/backup/usdtmore_backup_$(date +%Y%m%d_%H%M%S).db

# 导出SQLite数据为SQL格式
sqlite3 /path/to/usdtmore.db .dump > sqlite_backup.sql
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

## 数据类型转换说明

### 重要转换

| SQLite类型 | PostgreSQL类型 | 说明 |
|------------|----------------|------|
| INTEGER PRIMARY KEY | BIGSERIAL PRIMARY KEY | 自增主键 |
| REAL | DECIMAL(18,6) | 高精度小数 |
| VARCHAR(255) | VARCHAR(255) | 字符串 |
| TINYINT(1) | SMALLINT | 布尔值用小整数 |
| TIMESTAMP | TIMESTAMP WITH TIME ZONE | 带时区的时间戳 |

### 注意事项

1. **金额字段精度**：SQLite的REAL类型在PostgreSQL中使用DECIMAL(18,6)保证精度
2. **时间戳**：PostgreSQL使用带时区的时间戳，确保跨时区一致性
3. **布尔值**：使用SMALLINT替代TINYINT，配合CHECK约束
4. **自增ID**：使用BIGSERIAL确保足够的ID空间

## 性能对比

| 指标 | SQLite | PostgreSQL |
|------|--------|------------|
| 并发连接数 | 1个写入 | 100+个连接 |
| 事务处理能力 | 低 | 高 |
| 数据完整性 | 基础 | 强 |
| 备份恢复 | 文件拷贝 | 在线备份 |
| 扩展性 | 有限 | 优秀 |

## 故障排除

### 常见问题

1. **连接失败**
```bash
# 检查PostgreSQL服务状态
sudo systemctl status postgresql

# 检查网络连接
telnet localhost 5432
```

2. **权限问题**
```sql
-- 重新授权
GRANT ALL PRIVILEGES ON DATABASE usdtmore TO usdtmore_user;
GRANT ALL ON SCHEMA public TO usdtmore_user;
```

3. **数据类型错误**
```sql
-- 检查表结构
\d+ wallet_address
\d+ trade_orders
\d+ notify_record
```

4. **性能问题**
```bash
# 运行性能监控
./scripts/monitor.sh

# 检查慢查询日志
tail -f /var/log/postgresql/postgresql-*.log
```

### 回滚方案

如果迁移出现问题，可以快速回滚到SQLite：

```bash
# 修改环境变量
DB_TYPE=sqlite

# 恢复SQLite数据库
cp /path/to/backup/usdtmore_backup_*.db /path/to/usdtmore.db

# 重启应用
systemctl restart usdtmore
```

## 生产环境建议

### 高可用配置

1. **主从复制**：启用PostgreSQL流复制
2. **连接池**：使用PgBouncer优化连接管理
3. **监控告警**：集成Prometheus + Grafana
4. **自动备份**：每日全量 + 实时WAL备份

### 安全配置

1. **网络安全**：配置防火墙，限制数据库访问
2. **用户权限**：使用最小权限原则
3. **数据加密**：启用SSL连接和数据加密
4. **审计日志**：记录所有数据库操作

### 维护计划

- **每日**：自动备份、健康检查
- **每周**：性能报告、日志清理  
- **每月**：全面性能评估、配置优化
- **每季度**：容量规划、安全审计

## 联系支持

如果在迁移过程中遇到问题，请：

1. 查看日志文件：`/var/log/usdtmore/*.log`
2. 运行健康检查：`./scripts/health_check.sh`
3. 检查监控面板：http://localhost:3000
4. 提交Issue附带详细错误信息

---

*本迁移指南由 Database Administrator 编写，最后更新：2025-09-10*