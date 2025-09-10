#!/bin/bash

# PostgreSQL 数据库恢复脚本
# 作者: Database Administrator
# 创建时间: 2025-09-10

set -e

# 配置变量
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-usdtmore}"
DB_USER="${DB_USER:-postgres}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"

# 显示用法
show_usage() {
    echo "用法: $0 <backup_file>"
    echo "示例: $0 ./backups/usdtmore_backup_20250910_143000.sql.gz"
    echo ""
    echo "可用的备份文件:"
    find "$BACKUP_DIR" -name "usdtmore_backup_*.sql.gz" -printf "%T@ %Tc %p\n" | sort -nr | head -10 | while read timestamp date_time file; do
        echo "  $file ($date_time)"
    done
}

# 检查参数
if [ $# -eq 0 ]; then
    echo "❌ 错误: 请指定备份文件"
    show_usage
    exit 1
fi

BACKUP_FILE="$1"

# 检查备份文件是否存在
if [ ! -f "$BACKUP_FILE" ]; then
    echo "❌ 错误: 备份文件不存在: $BACKUP_FILE"
    show_usage
    exit 1
fi

echo "准备恢复数据库: $DB_NAME"
echo "恢复时间: $(date)"
echo "备份文件: $BACKUP_FILE"

# 确认操作
read -p "⚠️  警告: 此操作将完全替换现有数据库! 是否继续? (输入 'YES' 确认): " confirm
if [ "$confirm" != "YES" ]; then
    echo "恢复操作已取消"
    exit 0
fi

# 创建临时解压文件
TEMP_SQL_FILE=$(mktemp)

# 解压备份文件
echo "解压备份文件..."
if [[ "$BACKUP_FILE" == *.gz ]]; then
    gunzip -c "$BACKUP_FILE" > "$TEMP_SQL_FILE"
else
    cp "$BACKUP_FILE" "$TEMP_SQL_FILE"
fi

# 执行恢复
echo "开始恢复数据库..."
psql \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="postgres" \
    --quiet \
    --file="$TEMP_SQL_FILE"

# 清理临时文件
rm -f "$TEMP_SQL_FILE"

# 验证恢复结果
echo "验证恢复结果..."
WALLET_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM wallet_address;" | xargs)
ORDER_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM trade_orders;" | xargs)
NOTIFY_COUNT=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT COUNT(*) FROM notify_record;" | xargs)

echo "✅ 数据库恢复完成!"
echo ""
echo "=== 恢复统计 ==="
echo "钱包地址记录数: $WALLET_COUNT"
echo "交易订单记录数: $ORDER_COUNT"
echo "通知记录数: $NOTIFY_COUNT"
echo "恢复完成时间: $(date)"

# 重建统计信息
echo "重建数据库统计信息..."
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "ANALYZE;"

echo "所有恢复操作完成!"