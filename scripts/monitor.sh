#!/bin/bash

# PostgreSQL 数据库监控脚本
# 作者: Database Administrator
# 创建时间: 2025-09-10

set -e

# 配置变量
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-usdtmore}"
DB_USER="${DB_USER:-postgres}"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo_color() {
    echo -e "${2}${1}${NC}"
}

echo_color "🔍 PostgreSQL 数据库监控报告" $BLUE
echo_color "监控时间: $(date)" $BLUE
echo_color "数据库: $DB_NAME@$DB_HOST:$DB_PORT" $BLUE
echo ""

# 1. 连接状态检查
echo_color "=== 连接状态检查 ===" $YELLOW
CONNECTION_CHECK=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT 1;" 2>/dev/null | xargs || echo "FAILED")
if [ "$CONNECTION_CHECK" = "1" ]; then
    echo_color "✅ 数据库连接正常" $GREEN
else
    echo_color "❌ 数据库连接失败" $RED
    exit 1
fi

# 2. 数据库基本信息
echo ""
echo_color "=== 数据库基本信息 ===" $YELLOW
DB_SIZE=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" | xargs)
echo "数据库大小: $DB_SIZE"

VERSION=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT version();" | head -1 | xargs)
echo "PostgreSQL版本: $VERSION"

# 3. 表统计信息
echo ""
echo_color "=== 表统计信息 ===" $YELLOW
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
SELECT 
    schemaname as "Schema",
    tablename as "Table", 
    n_tup_ins as "Inserts",
    n_tup_upd as "Updates", 
    n_tup_del as "Deletes",
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as "Size"
FROM pg_stat_user_tables 
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
EOF

# 4. 活跃连接统计
echo ""
echo_color "=== 活跃连接统计 ===" $YELLOW
ACTIVE_CONNECTIONS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'active';" | xargs)
IDLE_CONNECTIONS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'idle';" | xargs)
TOTAL_CONNECTIONS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM pg_stat_activity;" | xargs)

echo "活跃连接数: $ACTIVE_CONNECTIONS"
echo "空闲连接数: $IDLE_CONNECTIONS"
echo "总连接数: $TOTAL_CONNECTIONS"

# 连接数预警
MAX_CONNECTIONS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SHOW max_connections;" | xargs)
CONNECTION_USAGE=$(echo "scale=2; $TOTAL_CONNECTIONS * 100 / $MAX_CONNECTIONS" | bc)

if (( $(echo "$CONNECTION_USAGE > 80" | bc -l) )); then
    echo_color "⚠️  警告: 连接使用率过高 ${CONNECTION_USAGE}%" $RED
elif (( $(echo "$CONNECTION_USAGE > 60" | bc -l) )); then
    echo_color "⚠️  注意: 连接使用率 ${CONNECTION_USAGE}%" $YELLOW
else
    echo_color "✅ 连接使用率正常 ${CONNECTION_USAGE}%" $GREEN
fi

# 5. 长时间运行的查询
echo ""
echo_color "=== 长时间运行的查询 ===" $YELLOW
LONG_QUERIES=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'active' AND query_start < NOW() - INTERVAL '1 minute';" | xargs)

if [ "$LONG_QUERIES" -gt 0 ]; then
    echo_color "⚠️  发现 $LONG_QUERIES 个长时间运行的查询" $YELLOW
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
SELECT 
    pid,
    usename,
    application_name,
    client_addr,
    query_start,
    NOW() - query_start as duration,
    LEFT(query, 100) as query_preview
FROM pg_stat_activity 
WHERE state = 'active' 
AND query_start < NOW() - INTERVAL '1 minute'
ORDER BY query_start;
EOF
else
    echo_color "✅ 没有长时间运行的查询" $GREEN
fi

# 6. 锁等待检查
echo ""
echo_color "=== 锁等待检查 ===" $YELLOW
LOCK_WAITS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock';" | xargs)

if [ "$LOCK_WAITS" -gt 0 ]; then
    echo_color "⚠️  发现 $LOCK_WAITS 个锁等待" $RED
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
SELECT 
    pid,
    usename,
    wait_event_type,
    wait_event,
    query_start,
    LEFT(query, 80) as query_preview
FROM pg_stat_activity 
WHERE wait_event_type = 'Lock'
ORDER BY query_start;
EOF
else
    echo_color "✅ 没有锁等待" $GREEN
fi

# 7. 业务统计
echo ""
echo_color "=== 业务统计 ===" $YELLOW
WALLET_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM wallet_address;" | xargs)
ACTIVE_WALLET_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM wallet_address WHERE status = 1;" | xargs)
TOTAL_ORDERS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM trade_orders;" | xargs)
WAITING_ORDERS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM trade_orders WHERE status = 1;" | xargs)
SUCCESS_ORDERS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM trade_orders WHERE status = 2;" | xargs)

echo "钱包地址总数: $WALLET_COUNT"
echo "活跃钱包地址: $ACTIVE_WALLET_COUNT"
echo "订单总数: $TOTAL_ORDERS"
echo "等待支付订单: $WAITING_ORDERS"
echo "成功支付订单: $SUCCESS_ORDERS"

# 今日订单统计
TODAY_ORDERS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM trade_orders WHERE DATE(created_at) = CURRENT_DATE;" | xargs)
echo "今日新增订单: $TODAY_ORDERS"

# 8. 索引使用统计
echo ""
echo_color "=== 索引使用统计 ===" $YELLOW
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
SELECT 
    schemaname,
    tablename, 
    indexname,
    idx_tup_read,
    idx_tup_fetch,
    CASE 
        WHEN idx_tup_read = 0 THEN 'Never Used'
        WHEN idx_tup_read < 100 THEN 'Low Usage'
        ELSE 'Normal Usage'
    END as usage_level
FROM pg_stat_user_indexes 
ORDER BY idx_tup_read DESC;
EOF

echo ""
echo_color "监控报告完成: $(date)" $BLUE