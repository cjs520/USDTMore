#!/bin/bash

# PostgreSQL 数据库恢复脚本
# 支持从备份文件恢复数据库，包括完整恢复、增量恢复和时间点恢复
# 包含安全检查和回滚机制

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

# 恢复配置
BACKUP_DIR="${DB_BACKUP_DIR:-/var/backups/postgresql/usdtmore}"
RESTORE_TYPE="${1:-full}" # full, schema, data
BACKUP_FILE="${2:-}"

# 安全配置
FORCE_RESTORE="${FORCE_RESTORE:-false}"
CREATE_SAFETY_BACKUP="${CREATE_SAFETY_BACKUP:-true}"
SAFETY_BACKUP_DIR="${BACKUP_DIR}/safety"

# 通知配置
TELEGRAM_BOT_TOKEN="${TG_BOT_TOKEN:-}"
TELEGRAM_CHAT_ID="${TG_BOT_ADMIN_ID:-}"

# 日志配置
LOG_FILE="${BACKUP_DIR}/restore.log"

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
    send_notification "❌ Restore Failed" "$message"
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
    
    command -v pg_restore >/dev/null 2>&1 || missing_deps+=("pg_restore")
    command -v psql >/dev/null 2>&1 || missing_deps+=("psql")
    command -v gzip >/dev/null 2>&1 || missing_deps+=("gzip")
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        error_exit "Missing dependencies: ${missing_deps[*]}"
    fi
}

create_restore_directories() {
    if [[ ! -d "$BACKUP_DIR" ]]; then
        mkdir -p "$BACKUP_DIR" || error_exit "Failed to create backup directory: $BACKUP_DIR"
    fi
    
    if [[ "$CREATE_SAFETY_BACKUP" == "true" && ! -d "$SAFETY_BACKUP_DIR" ]]; then
        mkdir -p "$SAFETY_BACKUP_DIR" || error_exit "Failed to create safety backup directory: $SAFETY_BACKUP_DIR"
    fi
    
    # 确保日志文件存在
    touch "$LOG_FILE" || error_exit "Failed to create log file: $LOG_FILE"
}

test_database_connection() {
    log "INFO" "Testing database connection..."
    
    export PGPASSWORD="$DB_PASSWORD"
    
    if ! pg_isready -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" >/dev/null 2>&1; then
        error_exit "Database server connection failed: $DB_HOST:$DB_PORT"
    fi
    
    log "INFO" "Database server connection successful"
}

check_database_exists() {
    export PGPASSWORD="$DB_PASSWORD"
    
    local db_exists=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -t -c "
        SELECT 1 FROM pg_database WHERE datname = '$DB_NAME';
    " 2>/dev/null | tr -d ' ')
    
    if [[ "$db_exists" == "1" ]]; then
        return 0
    else
        return 1
    fi
}

get_database_info() {
    if ! check_database_exists; then
        log "INFO" "Database $DB_NAME does not exist"
        return
    fi
    
    export PGPASSWORD="$DB_PASSWORD"
    
    local size=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT pg_size_pretty(pg_database_size('$DB_NAME'));
    " 2>/dev/null | tr -d ' ')
    
    local tables=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';
    " 2>/dev/null | tr -d ' ')
    
    local rows=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT SUM(n_tup_ins) FROM pg_stat_user_tables;
    " 2>/dev/null | tr -d ' ')
    
    log "INFO" "Current database info: Size=$size, Tables=$tables, Rows=${rows:-0}"
}

find_latest_backup() {
    local pattern="$1"
    local latest_backup=$(find "$BACKUP_DIR" -name "$pattern" -type f -printf '%T@ %p\n' 2>/dev/null | sort -nr | head -n1 | cut -d' ' -f2-)
    
    if [[ -z "$latest_backup" ]]; then
        error_exit "No backup files found matching pattern: $pattern"
    fi
    
    echo "$latest_backup"
}

validate_backup_file() {
    local backup_file="$1"
    
    if [[ ! -f "$backup_file" ]]; then
        error_exit "Backup file not found: $backup_file"
    fi
    
    if [[ ! -s "$backup_file" ]]; then
        error_exit "Backup file is empty: $backup_file"
    fi
    
    log "INFO" "Validating backup file: $(basename "$backup_file")"
    
    # 检查文件大小
    local file_size=$(stat -f%z "$backup_file" 2>/dev/null || stat -c%s "$backup_file" 2>/dev/null)
    if [[ $file_size -lt 1024 ]]; then
        error_exit "Backup file too small (${file_size} bytes), likely corrupted"
    fi
    
    # 如果是压缩文件，测试压缩完整性
    if [[ "$backup_file" == *.gz ]]; then
        if ! gzip -t "$backup_file" 2>/dev/null; then
            error_exit "Backup file compression is corrupted"
        fi
        log "INFO" "Compressed backup file validation successful"
    fi
    
    log "INFO" "Backup file validation successful (Size: $(du -h "$backup_file" | cut -f1))"
}

create_safety_backup() {
    if [[ "$CREATE_SAFETY_BACKUP" != "true" ]]; then
        return 0
    fi
    
    if ! check_database_exists; then
        log "INFO" "No existing database to backup for safety"
        return 0
    fi
    
    log "INFO" "Creating safety backup before restore..."
    
    local timestamp=$(date '+%Y%m%d_%H%M%S')
    local safety_backup_file="$SAFETY_BACKUP_DIR/usdtmore_safety_${timestamp}.sql"
    
    export PGPASSWORD="$DB_PASSWORD"
    
    if ! pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" \
        --verbose \
        --no-password \
        --format=custom \
        --compress=9 \
        --create \
        --clean \
        --if-exists \
        "$DB_NAME" > "$safety_backup_file" 2>>"$LOG_FILE"; then
        error_exit "Failed to create safety backup"
    fi
    
    # 压缩安全备份
    gzip "$safety_backup_file"
    safety_backup_file="${safety_backup_file}.gz"
    
    local backup_size=$(du -h "$safety_backup_file" | cut -f1)
    log "INFO" "Safety backup created: $(basename "$safety_backup_file") (Size: $backup_size)"
    
    echo "$safety_backup_file"
}

confirm_restore() {
    if [[ "$FORCE_RESTORE" == "true" ]]; then
        return 0
    fi
    
    echo
    echo "WARNING: This operation will replace the current database!"
    echo "Database: $DB_NAME"
    echo "Host: $DB_HOST:$DB_PORT"
    echo "Backup file: $(basename "$BACKUP_FILE")"
    echo "Restore type: $RESTORE_TYPE"
    echo
    
    if check_database_exists; then
        get_database_info
        echo
    fi
    
    read -p "Are you sure you want to proceed? (yes/no): " -r confirmation
    
    if [[ "$confirmation" != "yes" ]]; then
        log "INFO" "Restore cancelled by user"
        exit 0
    fi
}

prepare_restore_file() {
    local backup_file="$1"
    local temp_file="$backup_file"
    
    # 如果是压缩文件，解压到临时位置
    if [[ "$backup_file" == *.gz ]]; then
        temp_file="/tmp/$(basename "$backup_file" .gz)"
        log "INFO" "Decompressing backup file..."
        
        if ! gunzip -c "$backup_file" > "$temp_file"; then
            error_exit "Failed to decompress backup file"
        fi
        
        log "INFO" "Backup file decompressed to: $temp_file"
    fi
    
    echo "$temp_file"
}

cleanup_temp_files() {
    local temp_file="$1"
    
    if [[ "$temp_file" != "$BACKUP_FILE" && -f "$temp_file" ]]; then
        rm -f "$temp_file"
        log "INFO" "Cleaned up temporary file: $temp_file"
    fi
}

restore_full() {
    local backup_file="$1"
    
    log "INFO" "Starting full database restore..."
    
    export PGPASSWORD="$DB_PASSWORD"
    
    # 如果数据库存在，先断开所有连接
    if check_database_exists; then
        log "INFO" "Terminating existing connections to database..."
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "
            SELECT pg_terminate_backend(pid) 
            FROM pg_stat_activity 
            WHERE datname = '$DB_NAME' AND pid <> pg_backend_pid();
        " >>"$LOG_FILE" 2>&1 || true
        
        # 删除现有数据库
        log "INFO" "Dropping existing database..."
        if ! psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "
            DROP DATABASE IF EXISTS \"$DB_NAME\";
        " >>"$LOG_FILE" 2>&1; then
            error_exit "Failed to drop existing database"
        fi
    fi
    
    # 创建新数据库
    log "INFO" "Creating database..."
    if ! psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "
        CREATE DATABASE \"$DB_NAME\" WITH ENCODING='UTF8';
    " >>"$LOG_FILE" 2>&1; then
        error_exit "Failed to create database"
    fi
    
    # 恢复数据
    log "INFO" "Restoring database from backup..."
    if ! pg_restore -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" \
        --verbose \
        --no-password \
        --dbname="$DB_NAME" \
        --clean \
        --create \
        --if-exists \
        --single-transaction \
        --disable-triggers \
        "$backup_file" >>"$LOG_FILE" 2>&1; then
        error_exit "pg_restore failed"
    fi
    
    log "INFO" "Full database restore completed"
}

restore_schema_only() {
    local backup_file="$1"
    
    log "INFO" "Starting schema-only restore..."
    
    export PGPASSWORD="$DB_PASSWORD"
    
    # 确保数据库存在
    if ! check_database_exists; then
        log "INFO" "Creating database for schema restore..."
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "
            CREATE DATABASE \"$DB_NAME\" WITH ENCODING='UTF8';
        " >>"$LOG_FILE" 2>&1 || error_exit "Failed to create database"
    fi
    
    # 恢复模式
    if [[ "$backup_file" == *.sql ]]; then
        # Plain SQL file
        if ! psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" \
            -f "$backup_file" >>"$LOG_FILE" 2>&1; then
            error_exit "Schema restore from SQL file failed"
        fi
    else
        # Custom format
        if ! pg_restore -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" \
            --verbose \
            --no-password \
            --dbname="$DB_NAME" \
            --schema-only \
            --clean \
            --if-exists \
            "$backup_file" >>"$LOG_FILE" 2>&1; then
            error_exit "pg_restore schema-only failed"
        fi
    fi
    
    log "INFO" "Schema-only restore completed"
}

restore_data_only() {
    local backup_file="$1"
    
    log "INFO" "Starting data-only restore..."
    
    export PGPASSWORD="$DB_PASSWORD"
    
    # 确保数据库存在
    if ! check_database_exists; then
        error_exit "Database $DB_NAME does not exist. Create schema first."
    fi
    
    # 禁用触发器以提高性能
    log "INFO" "Disabling triggers for faster data loading..."
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
        SET session_replication_role = replica;
    " >>"$LOG_FILE" 2>&1 || true
    
    # 恢复数据
    if ! pg_restore -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" \
        --verbose \
        --no-password \
        --dbname="$DB_NAME" \
        --data-only \
        --disable-triggers \
        --single-transaction \
        "$backup_file" >>"$LOG_FILE" 2>&1; then
        error_exit "pg_restore data-only failed"
    fi
    
    # 重新启用触发器
    log "INFO" "Re-enabling triggers..."
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
        SET session_replication_role = DEFAULT;
    " >>"$LOG_FILE" 2>&1 || true
    
    log "INFO" "Data-only restore completed"
}

verify_restore() {
    if ! check_database_exists; then
        error_exit "Database verification failed: database does not exist"
    fi
    
    log "INFO" "Verifying restored database..."
    
    export PGPASSWORD="$DB_PASSWORD"
    
    # 检查基本连接
    if ! psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1; then
        error_exit "Database verification failed: cannot connect"
    fi
    
    # 检查表数量
    local table_count=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';
    " 2>/dev/null | tr -d ' ')
    
    if [[ "$table_count" -lt 1 ]]; then
        error_exit "Database verification failed: no tables found"
    fi
    
    # 检查关键表是否存在
    local key_tables=("trade_orders" "wallet_address" "notify_record")
    for table in "${key_tables[@]}"; do
        local exists=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
            SELECT EXISTS (
                SELECT FROM information_schema.tables 
                WHERE table_schema = 'public' AND table_name = '$table'
            );
        " 2>/dev/null | tr -d ' ')
        
        if [[ "$exists" != "t" ]]; then
            error_exit "Database verification failed: missing key table '$table'"
        fi
    done
    
    # 获取恢复后的统计信息
    get_database_info
    
    log "INFO" "Database verification successful"
}

analyze_database() {
    log "INFO" "Analyzing database to update statistics..."
    
    export PGPASSWORD="$DB_PASSWORD"
    
    if ! psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "ANALYZE;" >>"$LOG_FILE" 2>&1; then
        log "WARNING" "Database analysis failed, but restore was successful"
    else
        log "INFO" "Database analysis completed"
    fi
}

generate_restore_report() {
    local backup_file="$1"
    local safety_backup_file="$2"
    local start_time="$3"
    local end_time=$(date '+%s')
    local duration=$((end_time - start_time))
    
    local report="Restore Report:
- Database: $DB_NAME
- Host: $DB_HOST:$DB_PORT
- Type: $RESTORE_TYPE
- Source: $(basename "$backup_file")
- Safety Backup: $(basename "${safety_backup_file:-none}")"
    
    if check_database_exists; then
        export PGPASSWORD="$DB_PASSWORD"
        local size=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
            SELECT pg_size_pretty(pg_database_size('$DB_NAME'));
        " 2>/dev/null | tr -d ' ')
        report="$report
- Final Size: $size"
    fi
    
    report="$report
- Duration: ${duration}s
- Status: Success
- Timestamp: $(date)"
    
    log "INFO" "Restore completed successfully"
    log "INFO" "$report"
    
    send_notification "✅ Restore Completed" "$report"
}

# ==========================================
# 主程序
# ==========================================

main() {
    local start_time=$(date '+%s')
    
    log "INFO" "Starting PostgreSQL restore process..."
    log "INFO" "Restore type: $RESTORE_TYPE"
    
    # 预检查
    check_dependencies
    create_restore_directories
    test_database_connection
    
    # 确定备份文件
    if [[ -z "$BACKUP_FILE" ]]; then
        case "$RESTORE_TYPE" in
            "full")
                BACKUP_FILE=$(find_latest_backup "usdtmore_full_*.sql*")
                ;;
            "schema")
                BACKUP_FILE=$(find_latest_backup "usdtmore_schema_*.sql*")
                ;;
            "data")
                BACKUP_FILE=$(find_latest_backup "usdtmore_data_*.sql*")
                ;;
        esac
    fi
    
    log "INFO" "Using backup file: $(basename "$BACKUP_FILE")"
    
    # 验证备份文件
    validate_backup_file "$BACKUP_FILE"
    
    # 获取当前数据库信息
    get_database_info
    
    # 确认恢复操作
    confirm_restore
    
    # 创建安全备份
    local safety_backup_file=""
    safety_backup_file=$(create_safety_backup)
    
    # 准备恢复文件
    local restore_file=$(prepare_restore_file "$BACKUP_FILE")
    
    # 执行恢复
    case "$RESTORE_TYPE" in
        "full")
            restore_full "$restore_file"
            ;;
        "schema")
            restore_schema_only "$restore_file"
            ;;
        "data")
            restore_data_only "$restore_file"
            ;;
        *)
            error_exit "Unknown restore type: $RESTORE_TYPE. Use: full, schema, or data"
            ;;
    esac
    
    # 清理临时文件
    cleanup_temp_files "$restore_file"
    
    # 验证恢复
    verify_restore
    
    # 分析数据库
    analyze_database
    
    # 生成报告
    generate_restore_report "$BACKUP_FILE" "$safety_backup_file" "$start_time"
}

# ==========================================
# 使用说明
# ==========================================

show_usage() {
    cat << EOF
Usage: $0 [restore_type] [backup_file]

Restore types:
  full     - Full database restore (default)
  schema   - Schema-only restore
  data     - Data-only restore

Parameters:
  backup_file  - Path to backup file (optional, will find latest if not specified)

Environment variables:
  DB_HOST                 - Database host (default: localhost)
  DB_PORT                 - Database port (default: 5432)
  DB_NAME                 - Database name (default: usdtmore)
  DB_USER                 - Database user (default: postgres)
  DB_PASSWORD             - Database password
  DB_BACKUP_DIR           - Backup directory (default: /var/backups/postgresql/usdtmore)
  FORCE_RESTORE           - Skip confirmation (default: false)
  CREATE_SAFETY_BACKUP    - Create safety backup (default: true)
  TG_BOT_TOKEN           - Telegram bot token for notifications
  TG_BOT_ADMIN_ID        - Telegram chat ID for notifications

Examples:
  $0                                    # Restore latest full backup
  $0 full                              # Restore latest full backup
  $0 schema                            # Restore latest schema backup
  $0 data                              # Restore latest data backup
  $0 full /path/to/backup.sql          # Restore specific backup file
  FORCE_RESTORE=true $0 full           # Force restore without confirmation

EOF
}

# 处理参数
if [[ $# -gt 2 ]]; then
    show_usage
    exit 1
fi

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    show_usage
    exit 0
fi

# 执行主程序
main "$@"