#!/bin/bash

# 数据库迁移测试运行脚本
# Database Migration Test Runner Script

set -e  # 遇到错误时退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 脚本配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TEST_LOG_DIR="${PROJECT_ROOT}/test_logs"
TEST_DATA_DIR="${PROJECT_ROOT}/test_data"

# 创建必要目录
mkdir -p "$TEST_LOG_DIR"
mkdir -p "$TEST_DATA_DIR"

# 日志文件
LOG_FILE="${TEST_LOG_DIR}/test_run_$(date +%Y%m%d_%H%M%S).log"
SUMMARY_FILE="${TEST_LOG_DIR}/test_summary_$(date +%Y%m%d_%H%M%S).json"

# 函数：打印带颜色的消息
print_message() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}" | tee -a "$LOG_FILE"
}

# 函数：打印标题
print_title() {
    local title=$1
    local length=${#title}
    local line=$(printf "%-${length}s" "=" | tr ' ' '=')
    
    print_message "$CYAN" ""
    print_message "$CYAN" "$line"
    print_message "$CYAN" "$title"
    print_message "$CYAN" "$line"
}

# 函数：检查命令是否存在
check_command() {
    local cmd=$1
    if ! command -v "$cmd" >/dev/null 2>&1; then
        print_message "$RED" "错误: 命令 '$cmd' 未找到，请确保它已安装"
        return 1
    fi
}

# 函数：检查环境变量
check_env_vars() {
    print_message "$BLUE" "检查环境变量..."
    
    if [ -f "$SCRIPT_DIR/test_config.env" ]; then
        print_message "$GREEN" "加载测试配置文件: test_config.env"
        set -a
        source "$SCRIPT_DIR/test_config.env"
        set +a
    fi
    
    # 检查必要的环境变量
    if [ -z "$DB_TYPE" ]; then
        print_message "$YELLOW" "警告: DB_TYPE 未设置，默认使用 sqlite"
        export DB_TYPE=sqlite
    fi
    
    print_message "$GREEN" "当前数据库类型: $DB_TYPE"
}

# 函数：检查数据库连接
check_database_connection() {
    print_message "$BLUE" "检查数据库连接..."
    
    if [ "$DB_TYPE" = "postgresql" ]; then
        if [ -z "$DB_HOST" ] || [ -z "$DB_PASSWORD" ]; then
            print_message "$RED" "错误: PostgreSQL 配置不完整"
            print_message "$RED" "请设置 DB_HOST, DB_PASSWORD 等环境变量"
            return 1
        fi
        
        # 检查 PostgreSQL 连接
        print_message "$YELLOW" "测试 PostgreSQL 连接..."
        if command -v pg_isready >/dev/null 2>&1; then
            if pg_isready -h "${DB_HOST:-localhost}" -p "${DB_PORT:-5432}" -U "${DB_USER:-postgres}" >/dev/null 2>&1; then
                print_message "$GREEN" "PostgreSQL 连接正常"
            else
                print_message "$RED" "PostgreSQL 连接失败"
                return 1
            fi
        else
            print_message "$YELLOW" "pg_isready 不可用，跳过连接检查"
        fi
    fi
    
    print_message "$GREEN" "数据库连接检查完成"
}

# 函数：运行单个测试
run_single_test() {
    local test_name=$1
    local test_binary=$2
    local test_desc=$3
    
    print_message "$PURPLE" "开始运行: $test_name"
    print_message "$BLUE" "描述: $test_desc"
    
    local start_time=$(date +%s)
    local test_log="${TEST_LOG_DIR}/${test_name}_$(date +%Y%m%d_%H%M%S).log"
    
    if [ -f "$test_binary" ]; then
        # 运行测试二进制文件
        if "$test_binary" 2>&1 | tee "$test_log"; then
            local end_time=$(date +%s)
            local duration=$((end_time - start_time))
            print_message "$GREEN" "✅ $test_name 完成 (耗时: ${duration}s)"
            echo "{\"test\": \"$test_name\", \"status\": \"PASS\", \"duration\": $duration}" >> "$SUMMARY_FILE.tmp"
            return 0
        else
            local end_time=$(date +%s)
            local duration=$((end_time - start_time))
            print_message "$RED" "❌ $test_name 失败 (耗时: ${duration}s)"
            echo "{\"test\": \"$test_name\", \"status\": \"FAIL\", \"duration\": $duration}" >> "$SUMMARY_FILE.tmp"
            return 1
        fi
    else
        # 使用 go test 运行
        local test_file="${SCRIPT_DIR}/${test_binary}.go"
        if [ -f "$test_file" ]; then
            cd "$SCRIPT_DIR"
            if go run "$test_file" 2>&1 | tee "$test_log"; then
                local end_time=$(date +%s)
                local duration=$((end_time - start_time))
                print_message "$GREEN" "✅ $test_name 完成 (耗时: ${duration}s)"
                echo "{\"test\": \"$test_name\", \"status\": \"PASS\", \"duration\": $duration}" >> "$SUMMARY_FILE.tmp"
                return 0
            else
                local end_time=$(date +%s)
                local duration=$((end_time - start_time))
                print_message "$RED" "❌ $test_name 失败 (耗时: ${duration}s)"
                echo "{\"test\": \"$test_name\", \"status\": \"FAIL\", \"duration\": $duration}" >> "$SUMMARY_FILE.tmp"
                return 1
            fi
        else
            print_message "$RED" "测试文件不存在: $test_file"
            echo "{\"test\": \"$test_name\", \"status\": \"SKIP\", \"duration\": 0}" >> "$SUMMARY_FILE.tmp"
            return 1
        fi
    fi
}

# 函数：运行 Go 测试
run_go_tests() {
    print_message "$PURPLE" "运行 Go 测试套件"
    
    cd "$SCRIPT_DIR"
    local start_time=$(date +%s)
    
    if go test -v ./... -timeout=30m 2>&1 | tee "${TEST_LOG_DIR}/go_tests_$(date +%Y%m%d_%H%M%S).log"; then
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        print_message "$GREEN" "✅ Go 测试套件完成 (耗时: ${duration}s)"
        echo "{\"test\": \"Go Test Suite\", \"status\": \"PASS\", \"duration\": $duration}" >> "$SUMMARY_FILE.tmp"
        return 0
    else
        local end_time=$(date +%s)
        local duration=$((end_time - start_time))
        print_message "$RED" "❌ Go 测试套件失败 (耗时: ${duration}s)"
        echo "{\"test\": \"Go Test Suite\", \"status\": \"FAIL\", \"duration\": $duration}" >> "$SUMMARY_FILE.tmp"
        return 1
    fi
}

# 函数：生成测试报告
generate_report() {
    print_message "$BLUE" "生成测试报告..."
    
    # 创建 JSON 报告
    local total_tests=0
    local passed_tests=0
    local failed_tests=0
    local total_duration=0
    
    {
        echo "{"
        echo "  \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
        echo "  \"environment\": {"
        echo "    \"db_type\": \"$DB_TYPE\","
        echo "    \"go_version\": \"$(go version | cut -d' ' -f3)\","
        echo "    \"os\": \"$(uname -s)\","
        echo "    \"arch\": \"$(uname -m)\""
        echo "  },"
        echo "  \"tests\": ["
    } > "$SUMMARY_FILE"
    
    if [ -f "$SUMMARY_FILE.tmp" ]; then
        local first_line=true
        while IFS= read -r line; do
            if [ "$first_line" = true ]; then
                first_line=false
                echo "    $line" >> "$SUMMARY_FILE"
            else
                echo "    ,$line" >> "$SUMMARY_FILE"
            fi
            
            total_tests=$((total_tests + 1))
            if echo "$line" | grep -q '"status": "PASS"'; then
                passed_tests=$((passed_tests + 1))
            elif echo "$line" | grep -q '"status": "FAIL"'; then
                failed_tests=$((failed_tests + 1))
            fi
            
            local duration=$(echo "$line" | grep -o '"duration": [0-9]*' | cut -d' ' -f2)
            total_duration=$((total_duration + duration))
        done < "$SUMMARY_FILE.tmp"
    fi
    
    {
        echo "  ],"
        echo "  \"summary\": {"
        echo "    \"total\": $total_tests,"
        echo "    \"passed\": $passed_tests,"
        echo "    \"failed\": $failed_tests,"
        echo "    \"skipped\": $((total_tests - passed_tests - failed_tests)),"
        echo "    \"success_rate\": $(echo "scale=2; $passed_tests * 100 / $total_tests" | bc -l 2>/dev/null || echo "0"),"
        echo "    \"total_duration\": $total_duration"
        echo "  }"
        echo "}"
    } >> "$SUMMARY_FILE"
    
    rm -f "$SUMMARY_FILE.tmp"
    
    # 打印摘要
    print_title "测试运行摘要 / Test Run Summary"
    print_message "$BLUE" "总测试数 / Total Tests: $total_tests"
    print_message "$GREEN" "通过 / Passed: $passed_tests"
    print_message "$RED" "失败 / Failed: $failed_tests"
    print_message "$YELLOW" "跳过 / Skipped: $((total_tests - passed_tests - failed_tests))"
    print_message "$BLUE" "总耗时 / Total Duration: ${total_duration}s"
    
    if [ $failed_tests -eq 0 ]; then
        print_message "$GREEN" "🎉 所有测试通过！/ All tests passed!"
    else
        print_message "$RED" "⚠️  有 $failed_tests 个测试失败 / $failed_tests tests failed"
    fi
    
    print_message "$BLUE" "详细报告保存在 / Detailed report saved to: $SUMMARY_FILE"
    print_message "$BLUE" "日志文件保存在 / Log files saved in: $TEST_LOG_DIR"
}

# 函数：清理临时文件
cleanup() {
    print_message "$BLUE" "清理临时文件..."
    
    # 清理测试数据库文件
    find "$TEST_DATA_DIR" -name "*.db" -type f -mtime +7 -delete 2>/dev/null || true
    find "$TEST_DATA_DIR" -name "*test*.db" -type f -delete 2>/dev/null || true
    
    # 清理临时目录
    rm -rf /tmp/usdtmore_tests 2>/dev/null || true
    rm -rf /tmp/*test*.db 2>/dev/null || true
    
    print_message "$GREEN" "清理完成"
}

# 函数：显示帮助信息
show_help() {
    cat << EOF
数据库迁移测试运行器 / Database Migration Test Runner

用法 / Usage:
    $0 [选项] [测试名称]

选项 / Options:
    -h, --help              显示此帮助信息 / Show this help
    -c, --connection        仅运行连接测试 / Run connection tests only
    -m, --migration         仅运行迁移测试 / Run migration tests only
    -p, --performance       仅运行性能测试 / Run performance tests only
    -o, --operations        仅运行操作测试 / Run operations tests only
    -g, --go-test          运行 Go 测试套件 / Run Go test suite
    -a, --all              运行所有测试 (默认) / Run all tests (default)
    --skip-cleanup         跳过清理 / Skip cleanup
    --skip-go-test         跳过 Go 测试 / Skip Go tests
    --docker               使用 Docker 环境 / Use Docker environment

环境变量 / Environment Variables:
    DB_TYPE                数据库类型 (sqlite/postgresql)
    DB_HOST                数据库主机 (PostgreSQL)
    DB_PASSWORD           数据库密码 (PostgreSQL)
    TEST_ENV              测试环境标识 (true/false)

示例 / Examples:
    $0                     # 运行所有测试
    $0 --connection        # 仅运行连接测试
    $0 --docker           # 使用 Docker 环境运行测试

EOF
}

# 主函数
main() {
    local run_all=true
    local run_connection=false
    local run_migration=false
    local run_performance=false
    local run_operations=false
    local run_go_test=true
    local skip_cleanup=false
    local use_docker=false
    
    # 解析命令行参数
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -c|--connection)
                run_all=false
                run_connection=true
                shift
                ;;
            -m|--migration)
                run_all=false
                run_migration=true
                shift
                ;;
            -p|--performance)
                run_all=false
                run_performance=true
                shift
                ;;
            -o|--operations)
                run_all=false
                run_operations=true
                shift
                ;;
            -g|--go-test)
                run_all=false
                run_go_test=true
                shift
                ;;
            -a|--all)
                run_all=true
                shift
                ;;
            --skip-cleanup)
                skip_cleanup=true
                shift
                ;;
            --skip-go-test)
                run_go_test=false
                shift
                ;;
            --docker)
                use_docker=true
                shift
                ;;
            *)
                print_message "$RED" "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 开始测试
    print_title "数据库迁移测试开始 / Database Migration Tests Start"
    print_message "$BLUE" "测试开始时间: $(date)"
    print_message "$BLUE" "日志文件: $LOG_FILE"
    
    # 初始化摘要文件
    touch "$SUMMARY_FILE.tmp"
    
    # 检查环境
    check_env_vars
    
    if [ "$use_docker" = true ]; then
        print_message "$BLUE" "使用 Docker 环境运行测试..."
        cd "$PROJECT_ROOT"
        docker-compose -f tests/docker-compose.test.yml --profile test up -d
        # 等待服务启动
        sleep 10
    fi
    
    # 检查必要命令
    check_command "go"
    
    # 检查数据库连接
    check_database_connection
    
    local overall_result=0
    
    # 运行测试
    if [ "$run_all" = true ] || [ "$run_connection" = true ]; then
        run_single_test "连接测试" "db_connection_test" "数据库连接和基本功能测试"
        if [ $? -ne 0 ]; then overall_result=1; fi
    fi
    
    if [ "$run_all" = true ] || [ "$run_migration" = true ]; then
        run_single_test "迁移测试" "db_migration_test" "数据库迁移和数据一致性测试"
        if [ $? -ne 0 ]; then overall_result=1; fi
    fi
    
    if [ "$run_all" = true ] || [ "$run_performance" = true ]; then
        run_single_test "性能测试" "db_performance_test" "数据库性能对比测试"
        if [ $? -ne 0 ]; then overall_result=1; fi
    fi
    
    if [ "$run_all" = true ] || [ "$run_operations" = true ]; then
        run_single_test "操作测试" "db_operations_test" "数据库操作完整性测试"
        if [ $? -ne 0 ]; then overall_result=1; fi
    fi
    
    if [ "$run_go_test" = true ]; then
        run_go_tests
        if [ $? -ne 0 ]; then overall_result=1; fi
    fi
    
    # 生成报告
    generate_report
    
    # 清理
    if [ "$skip_cleanup" = false ]; then
        cleanup
    fi
    
    # 停止 Docker 服务
    if [ "$use_docker" = true ]; then
        print_message "$BLUE" "停止 Docker 服务..."
        cd "$PROJECT_ROOT"
        docker-compose -f tests/docker-compose.test.yml down
    fi
    
    print_message "$BLUE" "测试完成时间: $(date)"
    
    if [ $overall_result -eq 0 ]; then
        print_message "$GREEN" "🎉 所有测试成功完成！"
        exit 0
    else
        print_message "$RED" "❌ 部分测试失败，请检查日志"
        exit 1
    fi
}

# 捕获 Ctrl+C
trap 'print_message "$YELLOW" "测试被中断"; cleanup; exit 130' INT TERM

# 运行主函数
main "$@"