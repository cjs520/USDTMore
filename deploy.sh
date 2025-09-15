#!/bin/bash

# USDTMore 统一部署和管理脚本
# 整合了所有功能：部署、修复、测试、构建等

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

log_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# 生成随机字符串函数
generate_random_string() {
    openssl rand -hex 16
}

# 等待数据库函数
wait_for_database() {
    local host=${1:-localhost}
    local port=${2:-5432}
    local timeout=${3:-60}
    
    log_info "等待数据库 $host:$port 启动..."
    
    # 如果是Docker环境，使用docker-compose检查服务状态
    if command -v docker-compose &> /dev/null && [ -f "docker-compose.yml" ]; then
        log_info "检测到Docker环境，使用docker-compose检查服务状态..."
        
        for i in $(seq 1 $timeout); do
            if docker-compose ps postgres | grep -q "Up.*healthy"; then
                log_success "数据库服务已就绪！"
                return 0
            fi
            
            if [ $i -eq $timeout ]; then
                log_error "等待数据库服务超时 ($timeout 秒)"
                return 1
            fi
            
            echo "等待数据库服务启动... ($i/$timeout)"
            sleep 1
        done
    else
        # 传统的端口检查方式
        for i in $(seq 1 $timeout); do
            if nc -z "$host" "$port" > /dev/null 2>&1; then
                log_success "数据库 $host:$port 已就绪！"
                return 0
            fi
            
            if [ $i -eq $timeout ]; then
                log_error "等待数据库 $host:$port 超时 ($timeout 秒)"
                return 1
            fi
            
            echo "等待数据库启动... ($i/$timeout)"
            sleep 1
        done
    fi
}

# 检查Docker环境
check_docker() {
    log_info "检查Docker环境..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装，请先安装 Docker"
        exit 1
    fi

    if ! command -v docker-compose &> /dev/null; then
        log_error "Docker Compose 未安装，请先安装 Docker Compose"
        exit 1
    fi

    log_success "Docker 环境检查通过"
}

# 生成配置文件
generate_config() {
    if [ -f ".env" ]; then
        log_warning ".env 文件已存在，跳过配置文件生成"
        log_info "如需重新生成配置，请删除现有 .env 文件"
        return 0
    fi
    
    log_info "生成配置文件..."

    cat > .env << EOF
# 数据库配置
DB_HOST=postgres
DB_PORT=5432
DB_USER=usdtmore
DB_PASSWORD=$(generate_random_string)
DB_NAME=usdtmore
DB_SSLMODE=disable

# 应用配置
APP_PORT=6080
APP_SECRET=$(generate_random_string)

# JWT配置
JWT_SECRET=$(generate_random_string)

# 管理员认证令牌
ADMIN_TOKEN=$(generate_random_string)
API_TOKEN=$(generate_random_string)

# EVM API配置 (统一使用Etherscan API)
ETHERSCAN_API_KEY=YourEtherscanApiKey

# 支付配置
PAYMENT_TIMEOUT=1800
MIN_AMOUNT=1.0
MAX_AMOUNT=10000.0

# 监控配置
MONITOR_INTERVAL=30
CLEANUP_INTERVAL=3600
EOF

    log_success "配置文件已生成"
}

# 测试API配置
test_api_config() {
    log_info "测试 EVM API 配置..."
    
    # 加载环境变量
    if [ -f ".env" ]; then
        source .env
    fi
    
    # 检查环境变量
    if [ -z "$ETHERSCAN_API_KEY" ] || [ "$ETHERSCAN_API_KEY" = "YourEtherscanApiKey" ]; then
        log_error "ETHERSCAN_API_KEY 未设置或使用默认值"
        log_info "请在 .env 文件中设置: ETHERSCAN_API_KEY=your_api_key"
        return 1
    else
        log_success "ETHERSCAN_API_KEY 已设置"
    fi

    # 测试API密钥有效性
    log_info "测试 Etherscan API..."
    response=$(curl -s "https://api.etherscan.io/api?module=stats&action=ethsupply&apikey=$ETHERSCAN_API_KEY")

    if echo "$response" | grep -q "OK"; then
        log_success "Etherscan API 密钥有效"
    else
        log_error "Etherscan API 密钥无效或有问题"
        echo "响应: $response"
        return 1
    fi

    # 测试 Etherscan V2 API (多链支持)
    log_info "测试 Etherscan V2 API (BSC)..."
    v2_response=$(curl -s "https://api.etherscan.io/v2/api?chainid=56&module=stats&action=bnbsupply&apikey=$ETHERSCAN_API_KEY")

    if echo "$v2_response" | grep -q "OK"; then
        log_success "Etherscan V2 API (多链) 密钥有效"
    else
        log_warning "Etherscan V2 API 可能有问题"
        echo "响应: $v2_response"
    fi

    log_success "API 配置测试完成"
}

# 构建项目
build_project() {
    log_info "构建项目..."
    
    # 清理文件
    find . -name ".DS_Store" -exec rm -f {} \; 2>/dev/null || true
    rm -rf out 2>/dev/null || true
    
    # 如果是Debian包构建
    if [ -d "debian" ]; then
        chmod 755 debian/*.ex 2>/dev/null || true
        cp debian/postinst.ex debian/postinst 2>/dev/null || true
        cp debian/postrm.ex debian/postrm 2>/dev/null || true
        chmod 755 debian/postinst 2>/dev/null || true
        chmod 755 debian/postrm 2>/dev/null || true
        
        if command -v debuild &> /dev/null; then
            debuild -us -uc
        else
            log_warning "debuild 未安装，跳过Debian包构建"
        fi
    fi
    
    # Docker构建
    log_info "构建 Docker 镜像..."
    docker-compose build
    
    log_success "项目构建完成"
}

# 初始化数据库
init_database() {
    log_info "初始化数据库..."
    
    if [ -f "init.sql" ]; then
        if docker-compose exec -T postgres psql -U usdtmore -d usdtmore < init.sql; then
            log_success "数据库初始化完成"
        else
            log_error "数据库初始化失败"
            return 1
        fi
    else
        log_warning "init.sql 文件不存在，跳过数据库初始化"
    fi
}

# 快速修复
quick_fix() {
    log_info "执行快速修复..."
    
    # 检查服务状态
    if ! docker-compose ps | grep -q "Up"; then
        log_info "启动Docker服务..."
        docker-compose up -d
        sleep 10
    fi
    
    # 重启应用
    log_info "重启应用服务..."
    docker-compose restart usdtmore
    
    # 等待服务启动
    sleep 5
    
    # 检查日志中的错误
    log_info "检查应用日志..."
    if docker-compose logs --tail=10 usdtmore | grep -E "(ERROR|ERRO|error|panic)"; then
        log_warning "发现应用错误，请检查日志"
    else
        log_success "应用运行正常"
    fi
    
    log_success "快速修复完成"
}

# 并发订单修复
fix_concurrent_orders() {
    log_info "开始修复并发订单创建问题..."

    # 检查Docker服务状态
    if ! docker-compose ps | grep -q "Up"; then
        log_warning "Docker服务未运行，启动服务..."
        docker-compose up -d
        sleep 10
    fi

    # 备份数据库（可选但推荐）
    log_info "创建数据库备份..."
    if docker-compose exec -T postgres pg_dump -U usdtmore -d usdtmore > backup_before_concurrent_fix_$(date +%Y%m%d_%H%M%S).sql; then
        log_success "数据库备份完成"
    else
        log_warning "数据库备份失败，继续执行修复..."
    fi

    # 执行数据库修复
    log_info "执行数据库结构修复..."
    if [ -f "fix_concurrent_final.sql" ]; then
        if docker-compose exec -T postgres psql -U usdtmore -d usdtmore < fix_concurrent_final.sql; then
            log_success "数据库修复完成"
        else
            log_error "数据库修复失败"
            return 1
        fi
    else
        log_warning "fix_concurrent_final.sql 文件不存在，跳过数据库修复"
    fi

    # 重启应用以应用代码更改
    log_info "重启应用服务..."
    docker-compose restart usdtmore

    # 等待应用启动
    log_info "等待应用启动..."
    sleep 15

    # 测试并发订单创建
    log_info "测试并发订单创建..."
    test_concurrent_orders

    log_success "并发订单修复完成！"
}

# 测试并发订单
test_concurrent_orders() {
    log_info "开始并发测试..."
    
    # 并发测试函数
    test_order_creation() {
        local order_id="test_order_$(date +%s)_$1"
        local response=$(curl -s -X POST http://localhost:6080/api/create_order \
            -H "Content-Type: application/json" \
            -d "{
                \"order_id\": \"$order_id\",
                \"money\": 10.00,
                \"notify_url\": \"http://example.com/notify\",
                \"return_url\": \"http://example.com/return\"
            }")
        
        if echo "$response" | grep -q "success"; then
            log_success "订单 $order_id 创建成功"
            return 0
        else
            log_error "订单 $order_id 创建失败: $response"
            return 1
        fi
    }

    # 并发创建5个订单
    for i in {1..5}; do
        test_order_creation $i &
    done

    # 等待所有后台任务完成
    wait

    log_success "并发测试完成"
}

# 重启和修复服务
restart_and_fix() {
    log_info "重启和修复服务..."
    
    # 停止所有服务
    log_info "停止现有服务..."
    docker-compose down
    
    # 清理Docker资源
    log_info "清理Docker资源..."
    docker system prune -f
    
    # 重新构建和启动
    log_info "重新构建和启动服务..."
    docker-compose build --no-cache
    docker-compose up -d
    
    # 等待服务启动
    sleep 15
    wait_for_database postgres 5432
    
    # 初始化数据库
    init_database
    
    # 检查服务状态
    log_info "检查服务状态..."
    docker-compose ps
    
    log_success "重启和修复完成"
}

# 完整部署
deploy() {
    log_info "🚀 开始部署 USDTMore..."
    
    check_docker
    generate_config
    build_project
    
    log_info "启动服务..."
    docker-compose up -d
    
    # 等待数据库启动
    log_info "等待服务启动..."
    sleep 15
    
    # 检查服务健康状态
    if docker-compose ps | grep -q "Up.*healthy"; then
        log_success "所有服务已启动并健康"
    else
        log_warning "服务可能还在启动中，请稍后检查状态"
    fi
    
    init_database
    
    # 检查服务状态
    log_info "检查服务状态..."
    docker-compose ps
    
    # 显示访问信息
    display_info
    
    log_success "🎉 USDTMore 部署完成！"
}

# 显示访问信息
display_info() {
    echo ""
    log_success "🎉 USDTMore 服务信息："
    echo ""
    echo "📋 服务信息："
    echo "- 应用地址: http://localhost:6080"
    
    if [ -f ".env" ]; then
        echo "- 管理员令牌: $(grep ADMIN_TOKEN .env | cut -d'=' -f2)"
        echo "- API令牌: $(grep API_TOKEN .env | cut -d'=' -f2)"
    fi
    
    echo ""
    echo "📝 重要提醒："
    echo "1. 请修改 .env 文件中的 ETHERSCAN_API_KEY"
    echo "2. 请妥善保管管理员令牌和API令牌"
    echo "3. 生产环境请修改默认密码"
    echo ""
    echo "🔗 常用命令："
    echo "- 查看日志: docker-compose logs -f"
    echo "- 重启服务: docker-compose restart"
    echo "- 停止服务: docker-compose down"
    echo ""
    echo "🛠️  管理命令："
    echo "- 快速修复: $0 fix"
    echo "- 测试API: $0 test-api"
    echo "- 并发修复: $0 fix-concurrent"
    echo "- 重启修复: $0 restart-fix"
    echo "- 重新部署: $0 deploy"
}

# 显示帮助信息
show_help() {
    echo "USDTMore 统一管理脚本"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "命令:"
    echo "  deploy          完整部署 (默认)"
    echo "  build           构建项目"
    echo "  fix             快速修复"
    echo "  fix-concurrent  修复并发订单问题"
    echo "  restart-fix     重启和修复服务"
    echo "  test-api        测试API配置"
    echo "  test-concurrent 测试并发订单"
    echo "  init-db         初始化数据库"
    echo "  info            显示服务信息"
    echo "  help            显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0              # 完整部署"
    echo "  $0 deploy       # 完整部署"
    echo "  $0 fix          # 快速修复"
    echo "  $0 test-api     # 测试API配置"
    echo "  $0 restart-fix  # 重启和修复"
}

# 主函数
main() {
    case "${1:-deploy}" in
        "deploy")
            deploy
            ;;
        "build")
            build_project
            ;;
        "fix")
            quick_fix
            ;;
        "fix-concurrent")
            fix_concurrent_orders
            ;;
        "restart-fix")
            restart_and_fix
            ;;
        "test-api")
            test_api_config
            ;;
        "test-concurrent")
            test_concurrent_orders
            ;;
        "init-db")
            init_database
            ;;
        "info")
            display_info
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            log_error "未知命令: $1"
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"