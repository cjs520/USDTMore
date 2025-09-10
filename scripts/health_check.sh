#!/bin/bash

# PostgreSQL 健康检查脚本
# 作者: Database Administrator  
# 创建时间: 2025-09-10

set -e

# 配置变量
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-usdtmore}"
DB_USER="${DB_USER:-postgres}"
TIMEOUT="${HEALTH_CHECK_TIMEOUT:-10}"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo_color() {
    echo -e "${2}${1}${NC}"
}

# 检查PostgreSQL连接
check_postgresql() {
    echo "检查PostgreSQL连接..."
    
    if timeout $TIMEOUT psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" > /dev/null 2>&1; then
        echo_color "✅ PostgreSQL连接正常" $GREEN
        return 0
    else
        echo_color "❌ PostgreSQL连接失败" $RED
        return 1
    fi
}

# 检查表是否存在
check_tables() {
    echo "检查数据表..."
    
    TABLES=("wallet_address" "trade_orders" "notify_record")
    
    for table in "${TABLES[@]}"; do
        if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT to_regclass('$table');" 2>/dev/null | grep -q "$table"; then
            echo_color "✅ 表 $table 存在" $GREEN
        else
            echo_color "❌ 表 $table 不存在" $RED
            return 1
        fi
    done
    
    return 0
}

# 检查连接数
check_connections() {
    echo "检查数据库连接数..."
    
    ACTIVE_CONNS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM pg_stat_activity WHERE state = 'active';" 2>/dev/null | xargs)
    TOTAL_CONNS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM pg_stat_activity;" 2>/dev/null | xargs)
    MAX_CONNS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SHOW max_connections;" 2>/dev/null | xargs)
    
    echo "活跃连接: $ACTIVE_CONNS"
    echo "总连接数: $TOTAL_CONNS"
    echo "最大连接数: $MAX_CONNS"
    
    CONNECTION_USAGE=$(echo "scale=2; $TOTAL_CONNS * 100 / $MAX_CONNS" | bc)
    
    if (( $(echo "$CONNECTION_USAGE > 80" | bc -l) )); then
        echo_color "⚠️  警告: 连接使用率过高 ${CONNECTION_USAGE}%" $RED
        return 1
    elif (( $(echo "$CONNECTION_USAGE > 60" | bc -l) )); then
        echo_color "⚠️  注意: 连接使用率 ${CONNECTION_USAGE}%" $YELLOW
    else
        echo_color "✅ 连接使用率正常 ${CONNECTION_USAGE}%" $GREEN
    fi
    
    return 0
}

# 检查磁盘空间
check_disk_space() {
    echo "检查数据库磁盘使用..."
    
    DB_SIZE=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT pg_size_pretty(pg_database_size('$DB_NAME'));" 2>/dev/null | xargs)
    echo "数据库大小: $DB_SIZE"
    
    # 检查磁盘空间 (假设数据目录在 /var/lib/postgresql)
    if command -v df > /dev/null; then
        DISK_USAGE=$(df -h /var/lib/postgresql 2>/dev/null | tail -1 | awk '{print $5}' | sed 's/%//' || echo "N/A")
        if [[ "$DISK_USAGE" != "N/A" ]] && [[ $DISK_USAGE -gt 80 ]]; then
            echo_color "⚠️  警告: 磁盘使用率过高 ${DISK_USAGE}%" $RED
            return 1
        elif [[ "$DISK_USAGE" != "N/A" ]]; then
            echo_color "✅ 磁盘使用率正常 ${DISK_USAGE}%" $GREEN
        fi
    fi
    
    return 0
}

# 主函数
main() {
    echo_color "🔍 PostgreSQL 健康检查开始" $YELLOW
    echo "检查时间: $(date)"
    echo ""
    
    FAILED=0
    
    if ! check_postgresql; then
        FAILED=1
    fi
    
    echo ""
    
    if ! check_tables; then
        FAILED=1
    fi
    
    echo ""
    
    if ! check_connections; then
        FAILED=1
    fi
    
    echo ""
    
    if ! check_disk_space; then
        FAILED=1
    fi
    
    echo ""
    
    if [ $FAILED -eq 0 ]; then
        echo_color "✅ 所有健康检查通过" $GREEN
        exit 0
    else
        echo_color "❌ 健康检查失败" $RED
        exit 1
    fi
}

# 如果直接运行此脚本
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi