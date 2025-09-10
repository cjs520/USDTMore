#!/bin/bash

# PostgreSQL 数据库备份脚本
# 作者: Database Administrator
# 创建时间: 2025-09-10

set -e

# 配置变量
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-usdtmore}"
DB_USER="${DB_USER:-postgres}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 生成备份文件名
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
BACKUP_FILE="$BACKUP_DIR/usdtmore_backup_$TIMESTAMP.sql"
BACKUP_COMPRESSED="$BACKUP_FILE.gz"

echo "开始备份数据库: $DB_NAME"
echo "备份时间: $(date)"
echo "备份文件: $BACKUP_COMPRESSED"

# 执行备份
pg_dump \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="$DB_NAME" \
    --verbose \
    --clean \
    --if-exists \
    --create \
    --format=plain \
    --encoding=UTF8 \
    --no-password \
    --file="$BACKUP_FILE"

# 压缩备份文件
echo "压缩备份文件..."
gzip "$BACKUP_FILE"

# 验证备份文件
if [ -f "$BACKUP_COMPRESSED" ]; then
    BACKUP_SIZE=$(du -h "$BACKUP_COMPRESSED" | cut -f1)
    echo "✅ 备份完成! 文件大小: $BACKUP_SIZE"
    echo "备份文件路径: $BACKUP_COMPRESSED"
else
    echo "❌ 备份失败!"
    exit 1
fi

# 清理旧备份文件
echo "清理 $RETENTION_DAYS 天前的备份文件..."
find "$BACKUP_DIR" -name "usdtmore_backup_*.sql.gz" -mtime +$RETENTION_DAYS -delete
echo "清理完成"

# 显示备份统计信息
echo ""
echo "=== 备份统计 ==="
echo "当前备份文件数量: $(find "$BACKUP_DIR" -name "usdtmore_backup_*.sql.gz" | wc -l)"
echo "备份目录大小: $(du -sh "$BACKUP_DIR" | cut -f1)"
echo "最新备份: $BACKUP_COMPRESSED"

# 可选：上传到云存储
if [ -n "$CLOUD_BACKUP_ENABLED" ] && [ "$CLOUD_BACKUP_ENABLED" = "true" ]; then
    echo ""
    echo "上传备份到云存储..."
    # 这里可以添加云存储上传逻辑，例如 AWS S3, Google Cloud Storage 等
    # aws s3 cp "$BACKUP_COMPRESSED" "s3://your-backup-bucket/usdtmore/"
fi

echo "备份任务完成: $(date)"