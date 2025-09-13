#!/bin/bash

# 并发安全性验证测试执行脚本
# 执行基于前面完成的并发安全性修复工作的验证测试

set -e

echo "====== 并发安全性修复验证测试套件 ======"
echo "测试时间: $(date)"
echo "测试环境: $(go version)"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 创建测试结果目录
RESULT_DIR="./test_results/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULT_DIR"

echo -e "${BLUE}创建测试结果目录: $RESULT_DIR${NC}"
echo ""

# 设置测试环境变量
export TEST_TIMEOUT="30m"
export TEST_DEBUG="false"
export TEST_CLEANUP_ON_FAIL="true"

# 测试函数
run_test() {
    local test_name="$1"
    local test_file="$2"
    local description="$3"
    
    echo -e "${YELLOW}正在执行: $test_name${NC}"
    echo "描述: $description"
    echo "测试文件: $test_file"
    echo "开始时间: $(date)"
    
    local log_file="$RESULT_DIR/${test_name}.log"
    local result_file="$RESULT_DIR/${test_name}_result.txt"
    
    if go test -v -timeout="$TEST_TIMEOUT" "$test_file" > "$log_file" 2>&1; then
        echo -e "${GREEN}✅ $test_name - 通过${NC}"
        echo "PASS" > "$result_file"
    else
        echo -e "${RED}❌ $test_name - 失败${NC}"
        echo "FAIL" > "$result_file"
        echo "失败日志保存在: $log_file"
    fi
    
    echo "完成时间: $(date)"
    echo ""
}

# 执行并发安全性验证测试
echo -e "${BLUE}=== 1. 并发安全性验证测试 ===${NC}"
run_test "concurrency_fix" \
         "./concurrency_fix_test.go" \
         "测试新的CalcTradeAmount函数的并发性能和金额唯一性，验证乐观锁机制的冲突处理"

echo -e "${BLUE}=== 2. 性能回归测试 ===${NC}"
run_test "performance_regression" \
         "./performance_regression_test.go" \
         "对比修复前后的性能数据，验证API响应时间改善和系统吞吐量提升"

echo -e "${BLUE}=== 3. 极端并发场景测试 ===${NC}"
run_test "extreme_concurrent" \
         "./extreme_concurrent_test.go" \
         "模拟1000+并发订单创建，测试系统降级处理和长时间高负载稳定性"

echo -e "${BLUE}=== 4. 功能兼容性测试 ===${NC}"
run_test "compatibility" \
         "./compatibility_test.go" \
         "验证现有API接口兼容性，确保订单生命周期完整性和支付监控功能正常"

# 运行所有测试（可选）
echo -e "${BLUE}=== 5. 执行完整测试套件 ===${NC}"
echo "正在执行所有测试..."

if go test -v -timeout="$TEST_TIMEOUT" ./concurrency_fix_test.go ./performance_regression_test.go ./extreme_concurrent_test.go ./compatibility_test.go > "$RESULT_DIR/full_suite.log" 2>&1; then
    echo -e "${GREEN}✅ 完整测试套件 - 通过${NC}"
    echo "PASS" > "$RESULT_DIR/full_suite_result.txt"
else
    echo -e "${RED}❌ 完整测试套件 - 部分失败${NC}"
    echo "PARTIAL_FAIL" > "$RESULT_DIR/full_suite_result.txt"
fi

# 生成测试总结报告
echo -e "${BLUE}=== 生成测试总结报告 ===${NC}"

SUMMARY_FILE="$RESULT_DIR/test_summary.md"

cat > "$SUMMARY_FILE" << EOF
# 并发安全性修复验证测试总结报告

**测试时间**: $(date)  
**测试环境**: $(go version)  
**结果目录**: $RESULT_DIR

## 测试概览

本次测试验证了并发安全性修复的效果，包括以下几个方面：

1. **并发安全性验证**: 测试新的CalcTradeAmount函数并发性能
2. **性能回归测试**: 对比修复前后的性能改进
3. **极端并发场景**: 测试系统在高负载下的稳定性
4. **功能兼容性**: 确保现有功能正常工作

## 测试结果

| 测试项目 | 状态 | 描述 |
|---------|------|------|
EOF

# 检查各个测试结果
for test in concurrency_fix performance_regression extreme_concurrent compatibility full_suite; do
    if [[ -f "$RESULT_DIR/${test}_result.txt" ]]; then
        result=$(cat "$RESULT_DIR/${test}_result.txt")
        case $result in
            "PASS")
                status="✅ 通过"
                ;;
            "FAIL")
                status="❌ 失败"
                ;;
            "PARTIAL_FAIL")
                status="⚠️ 部分失败"
                ;;
            *)
                status="❓ 未知"
                ;;
        esac
        
        case $test in
            "concurrency_fix")
                desc="并发安全性验证测试"
                ;;
            "performance_regression")
                desc="性能回归测试"
                ;;
            "extreme_concurrent")
                desc="极端并发场景测试"
                ;;
            "compatibility")
                desc="功能兼容性测试"
                ;;
            "full_suite")
                desc="完整测试套件"
                ;;
        esac
        
        echo "| $desc | $status | 详见 ${test}.log |" >> "$SUMMARY_FILE"
    fi
done

cat >> "$SUMMARY_FILE" << EOF

## 核心验证目标

### 1. 金额计算唯一性
- **目标**: ≥ 98%
- **验证**: 新的原子精度机制确保金额唯一性

### 2. 订单创建成功率
- **目标**: ≥ 95%
- **验证**: 高并发下订单创建的成功率

### 3. 乐观锁机制
- **目标**: 正确处理并发冲突
- **验证**: 版本冲突检测和重试机制

### 4. 系统性能改进
- **目标**: API响应时间改善，吞吐量提升
- **验证**: 对比修复前后的性能数据

### 5. 系统稳定性
- **目标**: 长时间高负载下稳定运行
- **验证**: 内存使用、连接池管理、错误处理

## 测试文件说明

- **concurrency_fix_test.go**: 核心并发修复验证
- **performance_regression_test.go**: 性能对比分析
- **extreme_concurrent_test.go**: 极限场景压测
- **compatibility_test.go**: 功能兼容性检查

## 使用建议

1. 重点关注并发安全性验证测试的结果
2. 检查性能回归测试的改进数据
3. 验证极端场景下的系统表现
4. 确保所有现有功能正常工作

## 问题排查

如果测试失败，请检查：

1. 测试日志文件 (*.log)
2. 数据库连接和配置
3. 系统资源使用情况
4. 并发参数设置

EOF

echo -e "${GREEN}测试总结报告已生成: $SUMMARY_FILE${NC}"

# 统计测试结果
echo ""
echo -e "${BLUE}=== 测试结果统计 ===${NC}"

total_tests=0
passed_tests=0
failed_tests=0

for result_file in "$RESULT_DIR"/*_result.txt; do
    if [[ -f "$result_file" ]]; then
        result=$(cat "$result_file")
        total_tests=$((total_tests + 1))
        
        if [[ "$result" == "PASS" ]]; then
            passed_tests=$((passed_tests + 1))
        else
            failed_tests=$((failed_tests + 1))
        fi
    fi
done

echo "总测试数: $total_tests"
echo -e "通过测试: ${GREEN}$passed_tests${NC}"
echo -e "失败测试: ${RED}$failed_tests${NC}"

if [[ $failed_tests -eq 0 ]]; then
    echo -e "${GREEN}🎉 所有测试通过！并发安全性修复验证成功！${NC}"
    exit 0
else
    echo -e "${YELLOW}⚠️ 部分测试失败，请检查详细日志${NC}"
    exit 1
fi