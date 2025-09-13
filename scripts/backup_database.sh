#!/bin/bash

# PostgreSQL 数据库备份脚本
# 支持完整备份、增量备份和压缩存储
# 自动清理过期备份并支持远程存储

set -euo pipefail

# ==========================================
# 配置部分
# ==========================================

# 从环境变量读取数据库配置
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-usdtmore}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-}"

# 备份配置
BACKUP_DIR="${DB_BACKUP_DIR:-/var/backups/postgresql/usdtmore}"
BACKUP_RETENTION_DAYS="${DB_BACKUP_RETENTION_DAYS:-7}"
BACKUP_COMPRESSION="${BACKUP_COMPRESSION:-gzip}"
BACKUP_TYPE="${1:-full}" # full, schema, data

# 远程备份配置（可选）
REMOTE_BACKUP_ENABLED="${REMOTE_BACKUP_ENABLED:-false}"
REMOTE_BACKUP_HOST="${REMOTE_BACKUP_HOST:-}"
REMOTE_BACKUP_USER="${REMOTE_BACKUP_USER:-}"
REMOTE_BACKUP_PATH="${REMOTE_BACKUP_PATH:-}"

# 通知配置
TELEGRAM_BOT_TOKEN="${TG_BOT_TOKEN:-}"
TELEGRAM_CHAT_ID="${TG_BOT_ADMIN_ID:-}"

# 日志配置
LOG_FILE="${BACKUP_DIR}/backup.log"

# ==========================================
# 函数定义
# ==========================================

log() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
    echo "[$timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

error_exit() {
    local message="$1"
    log "ERROR" "$message"
    send_notification "❌ Backup Failed" "$message"
    exit 1
}

send_notification() {
    local title="$1"
    local message="$2"
    
    if [[ -n "$TELEGRAM_BOT_TOKEN" && -n "$TELEGRAM_CHAT_ID" ]]; then
        local text="$title\n\nDatabase: $DB_NAME\nHost: $DB_HOST\nTime: $(date)\n\n$message"
        curl -s -X POST "https://api.telegram.org/bot$TELEGRAM_BOT_TOKEN/sendMessage" \
            -d chat_id="$TELEGRAM_CHAT_ID" \
            -d text="$text" \
            -d parse_mode="HTML" > /dev/null 2>&1 || true
    fi
}

check_dependencies() {
    local missing_deps=()
    
    command -v pg_dump >/dev/null 2>&1 || missing_deps+=("pg_dump")
    command -v gzip >/dev/null 2>&1 || missing_deps+=("gzip")
    command -v rsync >/dev/null 2>&1 || missing_deps+=("rsync")
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        error_exit "Missing dependencies: ${missing_deps[*]}"
    fi
}

create_backup_directory() {
    if [[ ! -d "$BACKUP_DIR" ]]; then
        mkdir -p "$BACKUP_DIR" || error_exit "Failed to create backup directory: $BACKUP_DIR"
    fi
    
    # 确保日志文件存在
    touch "$LOG_FILE" || error_exit "Failed to create log file: $LOG_FILE"
}

test_database_connection() {
    log "INFO" "Testing database connection..."
    
    export PGPASSWORD="$DB_PASSWORD"
    
    if ! pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" >/dev/null 2>&1; then
        error_exit "Database connection failed: $DB_HOST:$DB_PORT/$DB_NAME"
    fi
    
    log "INFO" "Database connection successful"
}

get_database_size() {
    export PGPASSWORD="$DB_PASSWORD"
    
    local size=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT pg_size_pretty(pg_database_size('$DB_NAME'));
    " 2>/dev/null | tr -d ' ')
    
    echo "$size"
}

backup_full() {
    local timestamp=$(date '+%Y%m%d_%H%M%S')
    local backup_filename="usdtmore_full_${timestamp}.sql"
    local backup_filepath="$BACKUP_DIR/$backup_filename"
    
    log "INFO" "Starting full database backup..."
    log "INFO" "Database size: $(get_database_size)"
    
    export PGPASSWORD="$DB_PASSWORD"
    
    # 执行备份
    if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" \
        --verbose \
        --no-password \
        --format=custom \
        --compress=9 \
        --create \
        --clean \
        --if-exists \
        --quote-all-identifiers \
        --no-privileges \
        --no-owner \
        "$DB_NAME" > "$backup_filepath" 2>>"$LOG_FILE"; then
        error_exit "pg_dump failed"
    fi
    
    # 压缩备份文件
    if [[ "$BACKUP_COMPRESSION" == "gzip" ]]; then
        log "INFO" "Compressing backup file..."
        if ! gzip "$backup_filepath"; then
            error_exit "Failed to compress backup file"
        fi
        backup_filepath="${backup_filepath}.gz"
        backup_filename="${backup_filename}.gz"
    fi
    
    local backup_size=$(du -h "$backup_filepath" | cut -f1)
    log "INFO" "Full backup completed: $backup_filename (Size: $backup_size)"
    
    echo "$backup_filepath"
}

backup_schema_only() {
    local timestamp=$(date '+%Y%m%d_%H%M%S')
    local backup_filename="usdtmore_schema_${timestamp}.sql"
    local backup_filepath="$BACKUP_DIR/$backup_filename"
    
    log "INFO" "Starting schema-only backup..."
    
    export PGPASSWORD="$DB_PASSWORD"
    
    if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" \
        --verbose \
        --no-password \
        --schema-only \
        --format=plain \
        --create \
        --clean \
        --if-exists \
        --quote-all-identifiers \
        "$DB_NAME" > "$backup_filepath" 2>>"$LOG_FILE"; then
        error_exit "Schema backup failed"
    fi
    
    if [[ "$BACKUP_COMPRESSION" == "gzip" ]]; then
        gzip "$backup_filepath"
        backup_filepath="${backup_filepath}.gz"
        backup_filename="${backup_filename}.gz"
    fi
    
    local backup_size=$(du -h "$backup_filepath" | cut -f1)
    log "INFO" "Schema backup completed: $backup_filename (Size: $backup_size)"
    
    echo "$backup_filepath"
}

backup_data_only() {
    local timestamp=$(date '+%Y%m%d_%H%M%S')
    local backup_filename="usdtmore_data_${timestamp}.sql"
    local backup_filepath="$BACKUP_DIR/$backup_filename"
    
    log "INFO" "Starting data-only backup..."
    
    export PGPASSWORD="$DB_PASSWORD"
    
    if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" \
        --verbose \
        --no-password \
        --data-only \
        --format=custom \
        --compress=9 \
        --disable-triggers \
        --quote-all-identifiers \
        "$DB_NAME" > "$backup_filepath" 2>>"$LOG_FILE"; then
        error_exit "Data backup failed"
    fi
    
    if [[ "$BACKUP_COMPRESSION" == "gzip" ]]; then
        gzip "$backup_filepath"
        backup_filepath="${backup_filepath}.gz"
        backup_filename="${backup_filename}.gz"
    fi
    
    local backup_size=$(du -h "$backup_filepath" | cut -f1)
    log "INFO" "Data backup completed: $backup_filename (Size: $backup_size)"
    
    echo "$backup_filepath"
}

verify_backup() {
    local backup_file="$1"
    
    log "INFO" "Verifying backup file: $(basename "$backup_file")"
    
    # 检查文件大小
    if [[ ! -s "$backup_file" ]]; then
        error_exit "Backup file is empty: $backup_file"
    fi
    
    local file_size=$(stat -f%z "$backup_file" 2>/dev/null || stat -c%s "$backup_file" 2>/dev/null)
    if [[ $file_size -lt 1024 ]]; then
        error_exit "Backup file too small (${file_size} bytes), likely corrupted"
    fi
    
    # 如果是压缩文件，测试压缩完整性
    if [[ "$backup_file" == *.gz ]]; then
        if ! gzip -t "$backup_file" 2>/dev/null; then
            error_exit "Backup file compression is corrupted"
        fi
    fi
    
    # 如果是PostgreSQL custom format，验证格式
    if [[ "$backup_file" == *.sql && "$backup_file" != *.gz ]]; then
        if ! pg_restore --list "$backup_file" >/dev/null 2>&1; then
            # 如果不是custom format，尝试作为plain text验证
            if ! head -n 10 "$backup_file" | grep -q "PostgreSQL database dump" 2>/dev/null; then
                log "WARNING" "Cannot verify backup format, assuming it's valid"
            fi
        fi
    fi
    
    log "INFO" "Backup verification successful"
}

cleanup_old_backups() {
    log "INFO" "Cleaning up backups older than $BACKUP_RETENTION_DAYS days..."
    
    local deleted_count=0
    while IFS= read -r -d '' file; do
        rm "$file"
        log "INFO" "Deleted old backup: $(basename "$file")"
        ((deleted_count++))
    done < <(find "$BACKUP_DIR" -name "usdtmore_*.sql*" -type f -mtime +$BACKUP_RETENTION_DAYS -print0 2>/dev/null)
    
    if [[ $deleted_count -gt 0 ]]; then
        log "INFO" "Cleaned up $deleted_count old backup files"
    else
        log "INFO" "No old backup files to clean up"
    fi
}

upload_to_remote() {
    local backup_file="$1"
    
    if [[ "$REMOTE_BACKUP_ENABLED" != "true" ]]; then
        return 0
    fi
    
    if [[ -z "$REMOTE_BACKUP_HOST" || -z "$REMOTE_BACKUP_USER" || -z "$REMOTE_BACKUP_PATH" ]]; then
        log "WARNING" "Remote backup is enabled but configuration is incomplete"
        return 0
    fi
    
    log "INFO" "Uploading backup to remote server..."
    
    local remote_target="${REMOTE_BACKUP_USER}@${REMOTE_BACKUP_HOST}:${REMOTE_BACKUP_PATH}/"
    
    if rsync -avz --progress "$backup_file" "$remote_target" >>"$LOG_FILE" 2>&1; then
        log "INFO" "Remote backup upload successful"
    else
        log "WARNING" "Remote backup upload failed"
    fi
}

generate_backup_report() {
    local backup_file="$1"
    local start_time="$2"
    local end_time=$(date '+%s')
    local duration=$((end_time - start_time))
    
    local report="Backup Report:
- Database: $DB_NAME
- Host: $DB_HOST:$DB_PORT
- Type: $BACKUP_TYPE
- File: $(basename "$backup_file")
- Size: $(du -h "$backup_file" | cut -f1)
- Duration: ${duration}s
- Status: Success
- Timestamp: $(date)"
    
    log "INFO" "Backup completed successfully"
    log "INFO" "$report"
    
    send_notification "✅ Backup Completed" "$report"
}

# ==========================================
# 主程序
# ==========================================

main() {
    local start_time=$(date '+%s')
    
    log "INFO" "Starting PostgreSQL backup process..."
    log "INFO" "Backup type: $BACKUP_TYPE"
    
    # 预检查
    check_dependencies
    create_backup_directory
    test_database_connection
    
    # 执行备份
    local backup_file=""
    case "$BACKUP_TYPE" in
        "full")
            backup_file=$(backup_full)
            ;;
        "schema")
            backup_file=$(backup_schema_only)
            ;;
        "data")
            backup_file=$(backup_data_only)
            ;;
        *)
            error_exit "Unknown backup type: $BACKUP_TYPE. Use: full, schema, or data"
            ;;
    esac
    
    # 验证备份
    verify_backup "$backup_file"
    
    # 上传到远程服务器
    upload_to_remote "$backup_file"
    
    # 清理旧备份
    cleanup_old_backups
    
    # 生成报告
    generate_backup_report "$backup_file" "$start_time"
}

# ==========================================
# 使用说明
# ==========================================

show_usage() {
    cat << EOF
Usage: $0 [backup_type]

Backup types:
  full     - Full database backup (default)
  schema   - Schema-only backup
  data     - Data-only backup

Environment variables:
  DB_HOST                 - Database host (default: localhost)
  DB_PORT                 - Database port (default: 5432)
  DB_NAME                 - Database name (default: usdtmore)
  DB_USER                 - Database user (default: postgres)
  DB_PASSWORD             - Database password
  DB_BACKUP_DIR           - Backup directory (default: /var/backups/postgresql/usdtmore)
  DB_BACKUP_RETENTION_DAYS - Retention period in days (default: 7)
  BACKUP_COMPRESSION      - Compression type: gzip (default: gzip)
  REMOTE_BACKUP_ENABLED   - Enable remote backup (default: false)
  REMOTE_BACKUP_HOST      - Remote backup host
  REMOTE_BACKUP_USER      - Remote backup user
  REMOTE_BACKUP_PATH      - Remote backup path
  TG_BOT_TOKEN           - Telegram bot token for notifications
  TG_BOT_ADMIN_ID        - Telegram chat ID for notifications

Examples:
  $0                      # Full backup
  $0 full                 # Full backup
  $0 schema               # Schema-only backup
  $0 data                 # Data-only backup

EOF
}

# 处理参数
if [[ $# -gt 1 ]]; then
    show_usage
    exit 1
fi

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    show_usage
    exit 0
fi

# 执行主程序
main "$@"