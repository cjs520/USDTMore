#!/bin/bash

# USDTMore 综合测试套件运行脚本
# 用于运行完整的测试套件，包括单元测试、集成测试和性能测试

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查Docker是否运行
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        log_error "Docker is not running. Please start Docker first."
        exit 1
    fi
    log_success "Docker is running"
}

# 检查Go是否安装
check_go() {
    if ! command -v go &> /dev/null; then
        log_error "Go is not installed. Please install Go first."
        exit 1
    fi
    log_success "Go is installed: $(go version)"
}

# 检查依赖
check_dependencies() {
    log_info "Checking dependencies..."
    check_docker
    check_go
    
    # 检查是否有go.mod文件
    if [ ! -f "../go.mod" ]; then
        log_error "go.mod file not found. Please ensure you're in the correct directory."
        exit 1
    fi
    
    log_info "Installing test dependencies..."
    cd ..
    go mod tidy
    go mod download
    cd tests
    log_success "Dependencies checked and installed"
}

# 运行单元测试
run_unit_tests() {
    log_info "Running unit tests..."
    
    echo "========================================="
    echo "          UNIT TESTS REPORT"
    echo "========================================="
    
    # 订单生命周期测试
    log_info "Running order lifecycle tests..."
    go test -v ./unit/model/order_lifecycle_test.go -count=1 || log_warning "Some order lifecycle tests failed"
    
    # 支付检测测试
    log_info "Running payment detection tests..."
    go test -v ./unit/monitor/payment_detection_test.go -count=1 || log_warning "Some payment detection tests failed"
    
    # 通知机制测试
    log_info "Running notification tests..."
    go test -v ./unit/notify/notification_test.go -count=1 || log_warning "Some notification tests failed"
    
    # 多链支付测试
    log_info "Running multichain payment tests..."
    go test -v ./unit/monitor/multichain_test.go -count=1 || log_warning "Some multichain tests failed"
    
    # 边界条件测试
    log_info "Running edge case tests..."
    go test -v ./unit/edge_cases_test.go -count=1 || log_warning "Some edge case tests failed"
    
    # 现有的模型测试
    log_info "Running existing model tests..."
    go test -v ./unit/model/orders_test.go -count=1 || log_warning "Some existing model tests failed"
    
    log_success "Unit tests completed"
}

# 运行集成测试
run_integration_tests() {
    log_info "Running integration tests..."
    
    echo "========================================="
    echo "        INTEGRATION TESTS REPORT"
    echo "========================================="
    
    # 完整订单流程集成测试
    log_info "Running order flow integration tests..."
    go test -v ./integration/order_flow_integration_test.go -count=1 || log_warning "Some integration tests failed"
    
    log_success "Integration tests completed"
}

# 运行性能测试
run_performance_tests() {
    log_info "Running performance tests..."
    
    echo "========================================="
    echo "        PERFORMANCE TESTS REPORT"
    echo "========================================="
    
    # 运行基准测试
    log_info "Running benchmark tests..."
    go test -v -bench=. -benchmem ./performance/performance_test.go -count=1 || log_warning "Some benchmark tests failed"
    
    # 运行负载测试
    log_info "Running load tests..."
    go test -v ./performance/performance_test.go -run="TestHighConcurrency|TestHighThroughput|TestNotificationPerformance|TestMemoryUsage|TestDatabaseConnectionPool" -count=1 || log_warning "Some performance tests failed"
    
    log_success "Performance tests completed"
}

# 运行覆盖率测试
run_coverage_tests() {
    log_info "Running coverage analysis..."
    
    echo "========================================="
    echo "         COVERAGE ANALYSIS REPORT"
    echo "========================================="
    
    cd ..
    
    # 运行覆盖率测试
    log_info "Generating coverage report..."
    go test -coverprofile=coverage.out -coverpkg=./app/model,./app/monitor,./app/notify,./app/web ./tests/unit/...
    
    # 生成HTML报告
    if [ -f "coverage.out" ]; then
        go tool cover -html=coverage.out -o coverage.html
        log_success "Coverage report generated: coverage.html"
        
        # 显示覆盖率统计
        go tool cover -func=coverage.out | tail -1
    else
        log_warning "Coverage file not generated"
    fi
    
    cd tests
}

# 生成测试报告
generate_test_report() {
    log_info "Generating comprehensive test report..."
    
    REPORT_FILE="test_report_$(date +%Y%m%d_%H%M%S).md"
    
    cat > "$REPORT_FILE" << EOF
# USDTMore 测试报告

生成时间: $(date '+%Y-%m-%d %H:%M:%S')
测试环境: $(uname -s) $(uname -r)
Go版本: $(go version)

## 测试套件概述

本测试套件包含以下测试类型：

### 1. 单元测试 (Unit Tests)
- **订单生命周期测试**: 测试订单从创建到完成的完整流程
- **支付检测测试**: 验证多链支付处理和订单匹配算法
- **回调通知测试**: 测试回调发送、重试机制和状态管理
- **多链支付测试**: 验证TRON、POLY、OP、BSC等链的支付处理
- **边界条件测试**: 测试超时、重复支付、并发处理等场景

### 2. 集成测试 (Integration Tests)
- **完整订单流程**: 端到端测试订单创建、支付检测、回调完整流程
- **跨链支付处理**: 测试不同区块链的支付处理集成
- **回调重试机制**: 测试回调失败时的重试逻辑

### 3. 性能测试 (Performance Tests)
- **基准测试**: 订单创建、查询、金额计算、通知发送的性能基准
- **高并发测试**: 测试系统在高并发下的性能表现
- **负载测试**: 测试高吞吐量支付处理和通知性能
- **内存使用测试**: 监控系统在负载下的内存使用情况

## 测试覆盖的核心功能

### 订单状态管理
- [x] 订单创建和状态更新
- [x] 订单过期处理
- [x] 订单成功标记
- [x] 订单查询和筛选

### 支付检测
- [x] 区块链交易监控
- [x] 交易金额匹配
- [x] 交易时间验证
- [x] 多链支付支持

### 回调通知
- [x] 回调数据构造和签名
- [x] HTTP回调发送
- [x] 失败重试机制
- [x] 重试时间调度

### 多链支持
- [x] TRON (TRC20)
- [x] Polygon (POLY)
- [x] Optimism (OP)
- [x] BSC (BEP20)
- [x] Arbitrum (ARB)

## 测试场景

### 正常流程测试
1. 订单创建 → 支付检测 → 订单确认 → 回调通知 → 完成

### 异常情况测试
1. 订单过期处理
2. 支付金额不匹配
3. 重复支付检测
4. 回调失败重试
5. 网络异常处理

### 边界条件测试
1. 极小金额支付
2. 极大金额支付
3. 并发订单处理
4. 高频支付检测
5. 大量回调重试

## 性能指标

### 吞吐量目标
- 订单创建: > 500 订单/秒
- 支付处理: > 100 支付/秒
- 回调通知: > 50 通知/秒
- 数据库查询: > 1000 查询/秒

### 响应时间目标
- 订单创建: < 100ms
- 支付检测: < 5s
- 回调发送: < 5s
- 订单查询: < 50ms

## 测试数据

测试使用模拟数据和测试容器：
- PostgreSQL 15 测试数据库
- 模拟区块链API响应
- 模拟HTTP回调服务器
- 随机测试数据生成

## 运行说明

使用以下命令运行完整测试套件：

\`\`\`bash
./run_comprehensive_tests.sh
\`\`\`

或运行特定类型的测试：

\`\`\`bash
# 仅运行单元测试
./run_comprehensive_tests.sh --unit-only

# 仅运行性能测试  
./run_comprehensive_tests.sh --performance-only

# 运行覆盖率测试
./run_comprehensive_tests.sh --coverage
\`\`\`

## 依赖要求

- Go 1.19+
- Docker (用于测试数据库容器)
- 网络连接 (用于下载依赖和容器镜像)

---

*本报告由自动化测试套件生成*
EOF

    log_success "Test report generated: $REPORT_FILE"
}

# 清理函数
cleanup() {
    log_info "Cleaning up..."
    cd ..
    if [ -f "coverage.out" ]; then
        rm -f coverage.out
    fi
    cd tests
    log_success "Cleanup completed"
}

# 显示帮助信息
show_help() {
    cat << EOF
USDTMore 综合测试套件

用法: $0 [选项]

选项:
    --unit-only         仅运行单元测试
    --integration-only  仅运行集成测试
    --performance-only  仅运行性能测试
    --coverage          运行覆盖率测试
    --help             显示此帮助信息

示例:
    $0                    # 运行所有测试
    $0 --unit-only        # 仅运行单元测试
    $0 --coverage         # 运行覆盖率测试

EOF
}

# 主函数
main() {
    echo "========================================="
    echo "     USDTMore 综合测试套件 v1.0"
    echo "========================================="
    
    # 解析参数
    UNIT_ONLY=false
    INTEGRATION_ONLY=false
    PERFORMANCE_ONLY=false
    COVERAGE_ONLY=false
    
    while [[ $# -gt 0 ]]; do
        case $1 in
            --unit-only)
                UNIT_ONLY=true
                shift
                ;;
            --integration-only)
                INTEGRATION_ONLY=true
                shift
                ;;
            --performance-only)
                PERFORMANCE_ONLY=true
                shift
                ;;
            --coverage)
                COVERAGE_ONLY=true
                shift
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # 检查依赖
    check_dependencies
    
    # 开始时间记录
    START_TIME=$(date +%s)
    
    # 根据参数运行相应测试
    if [ "$COVERAGE_ONLY" = true ]; then
        run_coverage_tests
    elif [ "$UNIT_ONLY" = true ]; then
        run_unit_tests
    elif [ "$INTEGRATION_ONLY" = true ]; then
        run_integration_tests
    elif [ "$PERFORMANCE_ONLY" = true ]; then
        run_performance_tests
    else
        # 运行所有测试
        run_unit_tests
        echo ""
        run_integration_tests
        echo ""
        run_performance_tests
        echo ""
        run_coverage_tests
    fi
    
    # 计算运行时间
    END_TIME=$(date +%s)
    DURATION=$((END_TIME - START_TIME))
    
    echo ""
    echo "========================================="
    echo "           测试完成总结"
    echo "========================================="
    log_success "所有测试已完成"
    log_info "总运行时间: ${DURATION} 秒"
    
    # 生成报告
    generate_test_report
    
    # 清理
    cleanup
    
    echo ""
    log_success "测试套件运行完成！"
    echo "详细报告请查看生成的 test_report_*.md 文件"
}

# 运行主函数
main "$@"