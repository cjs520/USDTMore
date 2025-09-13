# 数据库运维指南

本文档提供了 USDT 支付系统数据库的运维指导，包括日常管理、监控、备份恢复、性能优化和故障处理。

## 目录

1. [数据库架构概览](#数据库架构概览)
2. [日常运维任务](#日常运维任务)
3. [监控和告警](#监控和告警)
4. [备份和恢复](#备份和恢复)
5. [性能优化](#性能优化)
6. [故障处理](#故障处理)
7. [安全管理](#安全管理)
8. [应急响应](#应急响应)

## 数据库架构概览

### 技术栈
- **数据库**: PostgreSQL 13+
- **连接池**: Go database/sql 原生连接池
- **ORM**: GORM v2
- **监控**: 内置性能监控 + pg_stat_statements

### 核心表结构

#### trade_orders (交易订单表)
```sql
-- 主要字段
id          BIGSERIAL PRIMARY KEY
order_id    VARCHAR(255) UNIQUE NOT NULL    -- 客户订单ID
trade_id    VARCHAR(255) UNIQUE NOT NULL    -- 内部订单ID
trade_hash  VARCHAR(64) UNIQUE             -- 区块链交易哈希
amount      DECIMAL(10,2) NOT NULL         -- USDT金额
money       DECIMAL(10,2) NOT NULL         -- 法币金额
status      SMALLINT NOT NULL              -- 状态: 1等待 2成功 3过期
version     BIGINT NOT NULL DEFAULT 0       -- 乐观锁版本号
chain       VARCHAR(255) NOT NULL          -- 区块链名称
address     VARCHAR(34) NOT NULL           -- 收款地址
expired_at  TIMESTAMP NOT NULL             -- 过期时间
created_at  TIMESTAMP NOT NULL             -- 创建时间
updated_at  TIMESTAMP NOT NULL             -- 更新时间
```

#### wallet_address (钱包地址表)
```sql
-- 主要字段
id           BIGSERIAL PRIMARY KEY
chain        VARCHAR(255) NOT NULL         -- 区块链名称
address      VARCHAR(255) NOT NULL         -- 钱包地址
status       SMALLINT DEFAULT 1            -- 状态: 1启用 0禁用
start_block  BIGINT DEFAULT 0              -- 起始区块
in_amount    DECIMAL(18,8) DEFAULT 0       -- 累计收入
out_amount   DECIMAL(18,8) DEFAULT 0       -- 累计支出
count        BIGINT DEFAULT 0              -- 交易次数

-- 唯一约束
UNIQUE(chain, address)
```

#### notify_record (通知记录表)
```sql
-- 主要字段
txid         VARCHAR(64) PRIMARY KEY       -- 交易哈希
created_at   TIMESTAMP NOT NULL           -- 创建时间
updated_at   TIMESTAMP NOT NULL           -- 更新时间
```

### 索引策略

```sql
-- 高频查询索引
CREATE INDEX idx_trade_orders_status_created ON trade_orders(status, created_at);
CREATE INDEX idx_trade_orders_address_amount_status ON trade_orders(chain, address, amount, status);
CREATE INDEX idx_trade_orders_expired_at ON trade_orders(expired_at) WHERE status = 1;
CREATE INDEX idx_trade_orders_trade_hash ON trade_orders(trade_hash) WHERE trade_hash != '';
CREATE INDEX idx_trade_orders_notify ON trade_orders(status, notify_num, notify_state) WHERE status = 2 AND notify_state = 0;

-- 钱包地址索引
CREATE UNIQUE INDEX idx_wallet_address_chain_address ON wallet_address(chain, address);
CREATE INDEX idx_wallet_address_status ON wallet_address(status);
```

## 日常运维任务

### 1. 连接池监控

```bash
# 检查连接池状态
curl -s http://localhost:6080/admin/db/stats | jq .

# 或使用数据库查询
psql -h localhost -U postgres -d usdtmore -c "
SELECT 
    application_name,
    state,
    COUNT(*) as connection_count
FROM pg_stat_activity 
WHERE datname = 'usdtmore'
GROUP BY application_name, state;
"
```

**正常指标范围:**
- 连接使用率: < 80%
- 平均等待时间: < 100ms
- 空闲连接数: 10-30个

### 2. 性能监控

```bash
# 查看慢查询
psql -h localhost -U postgres -d usdtmore -c "
SELECT 
    query,
    calls,
    total_exec_time,
    mean_exec_time,
    stddev_exec_time
FROM pg_stat_statements 
WHERE mean_exec_time > 1000  -- 超过1秒的查询
ORDER BY mean_exec_time DESC 
LIMIT 10;
"

# 查看表统计信息
psql -h localhost -U postgres -d usdtmore -c "
SELECT 
    schemaname,
    tablename,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    n_live_tup as live_tuples,
    n_dead_tup as dead_tuples
FROM pg_stat_user_tables;
"
```

### 3. 数据完整性检查

```bash
# 每日运行数据完整性检查
go run cmd/integrity_check.go

# 或使用SQL检查
psql -h localhost -U postgres -d usdtmore -c "
-- 检查重复订单ID
SELECT order_id, COUNT(*) 
FROM trade_orders 
GROUP BY order_id 
HAVING COUNT(*) > 1;

-- 检查状态不一致的订单
SELECT COUNT(*) as expired_not_marked
FROM trade_orders 
WHERE status = 1 AND expired_at < NOW();

-- 检查成功订单缺少交易哈希
SELECT COUNT(*) as missing_hash
FROM trade_orders 
WHERE status = 2 AND (trade_hash = '' OR trade_hash IS NULL);
"
```

### 4. 自动清理任务

```bash
# 清理过期订单 (每小时运行)
psql -h localhost -U postgres -d usdtmore -c "
UPDATE trade_orders 
SET status = 3, updated_at = NOW()
WHERE status = 1 AND expired_at < NOW();
"

# 清理老旧的通知记录 (每周运行)
psql -h localhost -U postgres -d usdtmore -c "
DELETE FROM notify_record 
WHERE created_at < NOW() - INTERVAL '30 days';
"

# 重新计算表统计信息 (每日运行)
psql -h localhost -U postgres -d usdtmore -c "
ANALYZE trade_orders;
ANALYZE wallet_address;
ANALYZE notify_record;
"
```

## 监控和告警

### 1. 关键指标监控

```bash
# 创建监控脚本 /usr/local/bin/monitor_db.sh
#!/bin/bash

DB_NAME="usdtmore"
ALERT_THRESHOLD_CONNECTION=80  # 连接使用率阈值
ALERT_THRESHOLD_SLOW_QUERY=1000  # 慢查询阈值(ms)

# 检查连接使用率
CONNECTION_USAGE=$(psql -h localhost -U postgres -d $DB_NAME -t -c "
SELECT ROUND(
    (SELECT COUNT(*) FROM pg_stat_activity WHERE datname = '$DB_NAME') * 100.0 / 
    (SELECT setting::int FROM pg_settings WHERE name = 'max_connections')
, 2);
")

if (( $(echo "$CONNECTION_USAGE > $ALERT_THRESHOLD_CONNECTION" | bc -l) )); then
    echo "ALERT: High connection usage: ${CONNECTION_USAGE}%"
    # 发送告警通知
fi

# 检查慢查询数量
SLOW_QUERIES=$(psql -h localhost -U postgres -d $DB_NAME -t -c "
SELECT COUNT(*) FROM pg_stat_statements 
WHERE mean_exec_time > $ALERT_THRESHOLD_SLOW_QUERY;
")

if [ "$SLOW_QUERIES" -gt 10 ]; then
    echo "ALERT: Too many slow queries: $SLOW_QUERIES"
fi
```

### 2. 业务指标监控

```sql
-- 订单成功率监控 (过去1小时)
SELECT 
    COUNT(CASE WHEN status = 2 THEN 1 END) * 100.0 / COUNT(*) as success_rate
FROM trade_orders 
WHERE created_at > NOW() - INTERVAL '1 hour';

-- 通知失败率监控
SELECT 
    COUNT(CASE WHEN notify_state = 0 THEN 1 END) * 100.0 / COUNT(*) as failure_rate
FROM trade_orders 
WHERE status = 2 AND notify_num > 0 
  AND created_at > NOW() - INTERVAL '1 hour';

-- 平均订单处理时间
SELECT 
    AVG(EXTRACT(EPOCH FROM (confirmed_at - created_at))) as avg_processing_time
FROM trade_orders 
WHERE status = 2 AND confirmed_at IS NOT NULL 
  AND created_at > NOW() - INTERVAL '1 hour';
```

### 3. 告警配置

```bash
# /etc/cron.d/db_monitoring
# 每5分钟检查一次
*/5 * * * * postgres /usr/local/bin/monitor_db.sh

# 每小时生成性能报告
0 * * * * postgres /usr/local/bin/generate_db_report.sh
```

## 备份和恢复

### 1. 自动备份

```bash
# 设置自动备份 (每日凌晨2点)
0 2 * * * postgres /path/to/scripts/backup_database.sh full

# 设置环境变量
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=usdtmore
export DB_USER=postgres
export DB_PASSWORD=your_password
export DB_BACKUP_DIR=/var/backups/postgresql/usdtmore
export DB_BACKUP_RETENTION_DAYS=7

# 执行备份
./scripts/backup_database.sh full
```

**备份策略:**
- 完整备份: 每日一次
- 增量备份: 每6小时一次 (WAL归档)
- 保留期: 本地7天，远程30天

### 2. 备份验证

```bash
# 验证备份文件完整性
./scripts/backup_database.sh --verify /path/to/backup.sql.gz

# 测试恢复 (在测试环境)
export DB_NAME=usdtmore_test
./scripts/restore_database.sh full /path/to/backup.sql.gz
```

### 3. 灾难恢复

```bash
# 紧急恢复步骤
1. 停止应用服务
   systemctl stop usdtmore

2. 创建安全备份
   ./scripts/backup_database.sh full

3. 恢复数据
   export FORCE_RESTORE=true
   ./scripts/restore_database.sh full /path/to/backup.sql.gz

4. 验证数据完整性
   go run cmd/integrity_check.go

5. 重启应用服务
   systemctl start usdtmore
```

### 4. 时间点恢复 (PITR)

```bash
# 配置 WAL 归档
archive_mode = on
archive_command = 'cp %p /var/lib/postgresql/wal_archive/%f'
wal_level = replica

# 恢复到指定时间点
pg_basebackup -h localhost -U postgres -D /var/lib/postgresql/recovery
# 在 recovery.conf 中设置
restore_command = 'cp /var/lib/postgresql/wal_archive/%f %p'
recovery_target_time = '2024-01-15 14:30:00'
```

## 性能优化

### 1. 连接池优化

```go
// 基于负载动态调整
func optimizeConnectionPool() {
    stats := db.Stats()
    usageRate := float64(stats.InUse) / float64(stats.MaxOpenConnections)
    
    if usageRate > 0.8 {
        // 增加连接数
        newMax := int(float64(stats.MaxOpenConnections) * 1.2)
        if newMax <= 200 {
            db.SetMaxOpenConns(newMax)
        }
    } else if usageRate < 0.2 {
        // 减少连接数
        newMax := int(float64(stats.MaxOpenConnections) * 0.8)
        if newMax >= 20 {
            db.SetMaxOpenConns(newMax)
        }
    }
}
```

### 2. 查询优化

```sql
-- 分析慢查询
EXPLAIN (ANALYZE, BUFFERS) 
SELECT * FROM trade_orders 
WHERE status = 1 AND expired_at < NOW();

-- 优化查询计划
SET enable_seqscan = off;
SET work_mem = '256MB';

-- 更新表统计信息
ANALYZE trade_orders;
```

### 3. 索引维护

```sql
-- 检查索引使用情况
SELECT 
    indexrelname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch
FROM pg_stat_user_indexes;

-- 重建索引 (如果碎片化严重)
REINDEX INDEX CONCURRENTLY idx_trade_orders_status_created;

-- 检查索引膨胀
SELECT 
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_total_relation_size(indexrelid)) as index_size
FROM pg_stat_user_indexes 
ORDER BY pg_total_relation_size(indexrelid) DESC;
```

### 4. 自动清理优化

```sql
-- 调整 autovacuum 参数
ALTER TABLE trade_orders SET (
    autovacuum_vacuum_scale_factor = 0.1,
    autovacuum_analyze_scale_factor = 0.05,
    autovacuum_vacuum_cost_delay = 10
);

-- 手动清理
VACUUM (ANALYZE, VERBOSE) trade_orders;
```

## 故障处理

### 1. 常见问题诊断

#### 连接数过多
```bash
# 查看当前连接
SELECT 
    application_name,
    client_addr,
    state,
    query_start,
    query
FROM pg_stat_activity 
WHERE datname = 'usdtmore'
ORDER BY query_start;

# 终止空闲连接
SELECT pg_terminate_backend(pid) 
FROM pg_stat_activity 
WHERE datname = 'usdtmore' 
  AND state = 'idle' 
  AND query_start < NOW() - INTERVAL '1 hour';
```

#### 死锁问题
```sql
-- 查看死锁信息
SELECT 
    blocked_locks.pid AS blocked_pid,
    blocked_activity.usename AS blocked_user,
    blocking_locks.pid AS blocking_pid,
    blocking_activity.usename AS blocking_user,
    blocked_activity.query AS blocked_statement,
    blocking_activity.query AS current_statement_in_blocking_process
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks 
    ON blocking_locks.locktype = blocked_locks.locktype
    AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;
```

#### 磁盘空间不足
```bash
# 检查数据库大小
SELECT 
    pg_database.datname,
    pg_size_pretty(pg_database_size(pg_database.datname)) AS size
FROM pg_database;

# 清理日志
TRUNCATE TABLE pg_log;

# 清理临时文件
SELECT pg_ls_dir('base/pgsql_tmp');
```

### 2. 紧急处理流程

#### 数据库无法连接
```bash
1. 检查进程状态
   systemctl status postgresql

2. 查看日志
   tail -f /var/log/postgresql/postgresql-*.log

3. 检查配置文件
   postgresql -D /var/lib/postgresql/data -C config_file

4. 重启服务
   systemctl restart postgresql
```

#### 性能严重下降
```bash
1. 查看当前活跃查询
   SELECT pid, query_start, state, query 
   FROM pg_stat_activity 
   WHERE state != 'idle' 
   ORDER BY query_start;

2. 终止长时间运行的查询
   SELECT pg_cancel_backend(pid) FROM pg_stat_activity 
   WHERE query_start < NOW() - INTERVAL '10 minutes' 
   AND state != 'idle';

3. 检查锁等待
   SELECT * FROM pg_locks WHERE NOT granted;

4. 手动 VACUUM
   VACUUM ANALYZE;
```

#### 数据不一致
```bash
1. 运行完整性检查
   go run cmd/integrity_check.go

2. 修复重复数据
   DELETE FROM trade_orders a USING trade_orders b 
   WHERE a.id < b.id AND a.order_id = b.order_id;

3. 修复状态不一致
   UPDATE trade_orders SET status = 3 
   WHERE status = 1 AND expired_at < NOW();

4. 重新计算统计
   ANALYZE;
```

## 安全管理

### 1. 用户和权限

```sql
-- 创建应用用户
CREATE USER usdtmore_app WITH PASSWORD 'strong_password';

-- 授予必要权限
GRANT CONNECT ON DATABASE usdtmore TO usdtmore_app;
GRANT USAGE ON SCHEMA public TO usdtmore_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO usdtmore_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO usdtmore_app;

-- 创建只读用户
CREATE USER usdtmore_readonly WITH PASSWORD 'readonly_password';
GRANT CONNECT ON DATABASE usdtmore TO usdtmore_readonly;
GRANT USAGE ON SCHEMA public TO usdtmore_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO usdtmore_readonly;
```

### 2. 连接安全

```bash
# postgresql.conf
ssl = on
ssl_cert_file = 'server.crt'
ssl_key_file = 'server.key'
ssl_min_protocol_version = 'TLSv1.2'

# pg_hba.conf
hostssl  usdtmore  usdtmore_app  0.0.0.0/0  scram-sha-256
hostssl  usdtmore  usdtmore_readonly  0.0.0.0/0  scram-sha-256
```

### 3. 审计日志

```sql
-- 启用日志记录
log_statement = 'mod'  # DDL, DML
log_line_prefix = '%t [%p]: [%l-1] user=%u,db=%d,app=%a,client=%h '
log_connections = on
log_disconnections = on
log_lock_waits = on
```

## 应急响应

### 1. 故障响应流程

```bash
# 故障发现
1. 监控告警触发
2. 用户报告问题
3. 日常巡检发现

# 初步评估 (5分钟内)
1. 确认故障范围和影响
2. 评估紧急程度
3. 决定是否需要紧急处理

# 紧急处理 (15分钟内)
1. 如果是性能问题：重启应用，不重启数据库
2. 如果是数据问题：停止写入，评估数据损坏程度
3. 如果是硬件问题：切换到备用环境

# 根本原因分析
1. 保存现场信息
2. 分析日志和监控数据
3. 制定修复计划
4. 实施修复并验证
```

### 2. 紧急联系人

```
数据库管理员: admin@company.com
系统架构师: architect@company.com
运维负责人: ops@company.com
值班电话: +86-XXX-XXXX-XXXX
```

### 3. 应急脚本

```bash
# /usr/local/bin/db_emergency.sh
#!/bin/bash

case "$1" in
    "restart")
        systemctl restart postgresql
        systemctl restart usdtmore
        ;;
    "readonly")
        # 切换到只读模式
        psql -c "ALTER SYSTEM SET default_transaction_read_only TO on;"
        psql -c "SELECT pg_reload_conf();"
        ;;
    "maintenance")
        # 进入维护模式
        systemctl stop usdtmore
        echo "System is in maintenance mode"
        ;;
    "recover")
        # 从备份恢复
        ./scripts/restore_database.sh full
        ;;
esac
```

### 4. 故障记录模板

```markdown
## 故障报告

### 基本信息
- 故障时间: 
- 发现时间: 
- 影响范围: 
- 紧急程度: 

### 故障现象
- 用户反馈: 
- 监控告警: 
- 系统日志: 

### 处理过程
- 初步诊断: 
- 应急处理: 
- 根本原因: 
- 最终解决: 

### 改进措施
- 预防措施: 
- 监控改进: 
- 流程优化: 
```

---

## 总结

本运维指南涵盖了 PostgreSQL 数据库的日常管理、监控、备份、性能优化和故障处理的各个方面。建议运维团队：

1. **建立定期检查机制**: 每日检查关键指标，每周进行深度分析
2. **完善监控体系**: 设置合理的告警阈值，确保及时发现问题
3. **定期演练**: 定期进行故障演练和恢复测试
4. **持续优化**: 根据业务发展和负载变化，持续优化配置和策略
5. **文档更新**: 及时更新运维文档，分享经验和最佳实践

通过规范的运维管理，可以确保数据库系统的高可用性、高性能和数据安全。