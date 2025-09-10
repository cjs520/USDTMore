#!/bin/bash

# Crontab 自动化任务设置脚本
# 作者: Database Administrator
# 创建时间: 2025-09-10

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "🔧 设置 PostgreSQL 自动化任务..."
echo "脚本目录: $SCRIPT_DIR"
echo "项目目录: $PROJECT_DIR"

# 检查必要的环境变量
if [ -z "$DB_PASSWORD" ]; then
    echo "⚠️  警告: 请设置 DB_PASSWORD 环境变量"
fi

# 创建临时crontab文件
TEMP_CRON=$(mktemp)

# 获取当前用户的crontab
crontab -l > "$TEMP_CRON" 2>/dev/null || true

# 添加备份任务 (每天凌晨2点)
echo "# USDTMore PostgreSQL 自动备份 - 每天凌晨2点" >> "$TEMP_CRON"
echo "0 2 * * * cd $PROJECT_DIR && $SCRIPT_DIR/backup.sh >> /var/log/usdtmore/backup.log 2>&1" >> "$TEMP_CRON"

# 添加健康检查任务 (每5分钟)
echo "" >> "$TEMP_CRON"
echo "# USDTMore PostgreSQL 健康检查 - 每5分钟" >> "$TEMP_CRON" 
echo "*/5 * * * * cd $PROJECT_DIR && $SCRIPT_DIR/health_check.sh >> /var/log/usdtmore/health.log 2>&1" >> "$TEMP_CRON"

# 添加性能监控任务 (每小时)
echo "" >> "$TEMP_CRON"
echo "# USDTMore PostgreSQL 性能监控 - 每小时" >> "$TEMP_CRON"
echo "0 * * * * cd $PROJECT_DIR && $SCRIPT_DIR/monitor.sh >> /var/log/usdtmore/monitor.log 2>&1" >> "$TEMP_CRON"

# 添加日志清理任务 (每周清理30天前的日志)
echo "" >> "$TEMP_CRON"
echo "# USDTMore 日志清理 - 每周日凌晨3点" >> "$TEMP_CRON"
echo "0 3 * * 0 find /var/log/usdtmore -name '*.log' -mtime +30 -delete 2>/dev/null" >> "$TEMP_CRON"

# 应用新的crontab
crontab "$TEMP_CRON"

# 清理临时文件
rm "$TEMP_CRON"

echo "✅ Crontab 任务设置完成!"
echo ""
echo "已添加的定时任务:"
echo "- 数据库备份: 每天凌晨2点"
echo "- 健康检查: 每5分钟"
echo "- 性能监控: 每小时"
echo "- 日志清理: 每周日凌晨3点"
echo ""
echo "查看当前crontab:"
echo "crontab -l"
echo ""
echo "查看日志:"
echo "tail -f /var/log/usdtmore/backup.log"
echo "tail -f /var/log/usdtmore/health.log"
echo "tail -f /var/log/usdtmore/monitor.log"

# 创建日志目录
sudo mkdir -p /var/log/usdtmore
sudo chown $(whoami):$(whoami) /var/log/usdtmore

echo ""
echo "✅ 日志目录已创建: /var/log/usdtmore"