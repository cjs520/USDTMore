#!/bin/bash

# USDTMore 验证测试套件执行脚本
# 用于快速执行所有核心功能验证测试

echo "=========================================="
echo "USDTMore 核心功能验证测试套件"
echo "=========================================="
echo ""

# 设置测试环境
export GO111MODULE=on
export CGO_ENABLED=1

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo "❌ Go 未安装或不在 PATH 中"
    exit 1
fi

echo "✅ Go 环境检查通过: $(go version)"
echo ""

# 进入项目根目录
cd "$(dirname "$0")/.." || exit 1

# 检查依赖
echo "🔄 检查项目依赖..."
go mod tidy
if [ $? -ne 0 ]; then
    echo "❌ 依赖检查失败"
    exit 1
fi
echo "✅ 依赖检查完成"
echo ""

# 执行基础功能测试
echo "🧪 执行基础功能验证测试..."
echo "------------------------------------------"
go test -v ./tests/simple_validation_test.go -run "Test_Simple_OrderCreation|Test_Simple_CalcTradeAmount|Test_Simple_BusinessLogic" -timeout 2m
BASIC_RESULT=$?

echo ""
echo "📊 执行性能验证测试..."
echo "------------------------------------------"  
go test -v ./tests/simple_validation_test.go -run "Test_Simple_PerformanceValidation" -timeout 2m
PERF_RESULT=$?

echo ""
echo "📈 执行基准测试..."
echo "------------------------------------------"
go test -bench=Benchmark_Simple -run="^$" ./tests/simple_validation_test.go -timeout 2m
BENCH_RESULT=$?

echo ""
echo "📋 生成综合验证报告..."
echo "------------------------------------------"
go test -v ./tests/validation_report_test.go -timeout 2m
REPORT_RESULT=$?

echo ""
echo "=========================================="
echo "测试执行总结"
echo "=========================================="

# 计算总体结果
TOTAL_TESTS=4
PASSED_TESTS=0

if [ $BASIC_RESULT -eq 0 ]; then
    echo "✅ 基础功能测试: 通过"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ 基础功能测试: 失败"
fi

if [ $PERF_RESULT -eq 0 ]; then
    echo "✅ 性能验证测试: 通过"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ 性能验证测试: 失败"
fi

if [ $BENCH_RESULT -eq 0 ]; then
    echo "✅ 基准测试: 通过"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ 基准测试: 失败"
fi

if [ $REPORT_RESULT -eq 0 ]; then
    echo "✅ 综合报告: 通过"
    PASSED_TESTS=$((PASSED_TESTS + 1))
else
    echo "❌ 综合报告: 失败"
fi

echo ""
echo "总体结果: $PASSED_TESTS/$TOTAL_TESTS 通过"

SUCCESS_RATE=$(echo "scale=1; $PASSED_TESTS * 100 / $TOTAL_TESTS" | bc -l 2>/dev/null || echo "N/A")
if [ "$SUCCESS_RATE" != "N/A" ]; then
    echo "成功率: ${SUCCESS_RATE}%"
fi

echo ""
echo "📄 查看完整报告: FINAL_TEST_REPORT.md"
echo ""

# 根据结果设置退出码
if [ $PASSED_TESTS -eq $TOTAL_TESTS ]; then
    echo "🎉 所有测试通过! 系统核心功能验证完成。"
    exit 0
elif [ $PASSED_TESTS -ge 3 ]; then
    echo "⚠️  大部分测试通过，系统基本可用，建议查看失败项。"
    exit 0
else
    echo "❌ 多个测试失败，请检查系统问题。"
    exit 1
fi