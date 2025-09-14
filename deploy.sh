#!/bin/bash

# USDTMore 一键部署脚本
# 支持 Docker 和手动部署两种方式

set -e  # 遇到错误立即退出

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 命令未找到，请先安装 $1"
        return 1
    fi
    return 0
}

# 检查系统要求
check_system() {
    log_step "检查系统环境..."
    
    # 检查操作系统
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        log_info "检测到 Linux 系统"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        log_info "检测到 macOS 系统"
    else
        log_warn "未知操作系统: $OSTYPE"
    fi
    
    # 检查架构
    ARCH=$(uname -m)
    log_info "系统架构: $ARCH"
    
    # 检查内存
    if command -v free &> /dev/null; then
        MEMORY=$(free -m | awk 'NR==2{printf "%.1fGB", $2/1024}')
        log_info "系统内存: $MEMORY"
    fi
}

# 安装 Docker 和 Docker Compose
install_docker() {
    log_step "安装 Docker 和 Docker Compose..."
    
    if command -v docker &> /dev/null && command -v docker-compose &> /dev/null; then
        log_info "Docker 和 Docker Compose 已安装"
        return 0
    fi
    
    # 检测系统类型并安装 Docker
    if [[ -f /etc/debian_version ]]; then
        # Debian/Ubuntu
        log_info "检测到 Debian/Ubuntu 系统，安装 Docker..."
        sudo apt update
        sudo apt install -y apt-transport-https ca-certificates curl gnupg lsb-release
        
        # 添加 Docker 官方 GPG 密钥
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
        
        # 添加 Docker 仓库
        echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
        
        # 安装 Docker
        sudo apt update
        sudo apt install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
        
        # 安装 Docker Compose
        sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
        sudo chmod +x /usr/local/bin/docker-compose
        
    elif [[ -f /etc/redhat-release ]]; then
        # CentOS/RHEL/Fedora
        log_info "检测到 RedHat 系列系统，安装 Docker..."
        sudo yum install -y yum-utils
        sudo yum-config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
        sudo yum install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
        
        # 安装 Docker Compose
        sudo curl -L "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
        sudo chmod +x /usr/local/bin/docker-compose
        
    else
        log_error "不支持的系统类型，请手动安装 Docker"
        exit 1
    fi
    
    # 启动 Docker 服务
    sudo systemctl start docker
    sudo systemctl enable docker
    
    # 添加当前用户到 docker 组
    sudo usermod -aG docker $USER
    
    log_info "Docker 安装完成，请重新登录以使用 Docker"
}

# 安装 PostgreSQL
install_postgresql() {
    log_step "安装 PostgreSQL..."
    
    if command -v psql &> /dev/null; then
        log_info "PostgreSQL 已安装"
        return 0
    fi
    
    if [[ -f /etc/debian_version ]]; then
        # Debian/Ubuntu
        sudo apt update
        sudo apt install -y postgresql postgresql-contrib
    elif [[ -f /etc/redhat-release ]]; then
        # CentOS/RHEL/Fedora
        sudo yum install -y postgresql-server postgresql-contrib
        sudo postgresql-setup initdb
    else
        log_error "不支持的系统类型，请手动安装 PostgreSQL"
        exit 1
    fi
    
    # 启动 PostgreSQL 服务
    sudo systemctl start postgresql
    sudo systemctl enable postgresql
    
    log_info "PostgreSQL 安装完成"
}

# 配置数据库
setup_database() {
    log_step "配置数据库..."
    
    # 读取配置
    DB_NAME=${DB_NAME:-usdtmore}
    DB_USER=${DB_USER:-usdtmore}
    DB_PASSWORD=${DB_PASSWORD:-$(openssl rand -base64 32)}
    
    log_info "数据库名称: $DB_NAME"
    log_info "数据库用户: $DB_USER"
    log_info "数据库密码: $DB_PASSWORD"
    
    # 创建数据库和用户
    sudo -u postgres psql << EOF
CREATE DATABASE $DB_NAME;
CREATE USER $DB_USER WITH ENCRYPTED PASSWORD '$DB_PASSWORD';
GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO $DB_USER;
ALTER USER $DB_USER CREATEDB;
\q
EOF
    
    log_info "数据库配置完成"
}

# 生成配置文件
generate_config() {
    log_step "生成配置文件..."
    
    if [[ ! -f .env ]]; then
        if [[ -f .env.example ]]; then
            cp .env.example .env
            log_info "已从 .env.example 复制配置文件"
        else
            log_error ".env.example 文件不存在"
            exit 1
        fi
    else
        log_info "配置文件 .env 已存在"
    fi
    
    # 生成随机令牌
    if [[ -z "$AUTH_TOKEN" ]]; then
        AUTH_TOKEN=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
        log_info "生成认证令牌: $AUTH_TOKEN"
    fi
    
    # 更新配置文件中的占位符
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        sed -i '' "s/your_32_char_secure_token_here_change_me/$AUTH_TOKEN/g" .env
        sed -i '' "s/your_secure_db_password_change_me/$DB_PASSWORD/g" .env
    else
        # Linux
        sed -i "s/your_32_char_secure_token_here_change_me/$AUTH_TOKEN/g" .env
        sed -i "s/your_secure_db_password_change_me/$DB_PASSWORD/g" .env
    fi
    
    log_warn "请编辑 .env 文件，设置以下必需参数："
    echo -e "${CYAN}  - TG_BOT_TOKEN${NC}: Telegram Bot 令牌"
    echo -e "${CYAN}  - TG_BOT_ADMIN_ID${NC}: Telegram 管理员 ID"
    echo -e "${CYAN}  - ETHERSCAN_API_KEY${NC}: EVM 链 API 密钥"
    echo -e "${CYAN}  - TRON_GRID_API_KEY${NC}: TRON Grid API 密钥"
    echo ""
    
    read -p "是否现在编辑配置文件？(y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        ${EDITOR:-nano} .env
    fi
}

# Docker 部署
deploy_docker() {
    log_step "使用 Docker 部署..."
    
    # 检查 Docker
    if ! check_command docker || ! check_command docker-compose; then
        log_warn "Docker 未安装，正在安装..."
        install_docker
        log_warn "请重新登录后再次运行此脚本"
        exit 0
    fi
    
    # 生成配置文件
    generate_config
    
    # 构建和启动服务
    log_info "构建 Docker 镜像..."
    docker-compose build
    
    log_info "启动服务..."
    docker-compose up -d
    
    # 等待服务启动
    log_info "等待服务启动..."
    sleep 10
    
    # 检查服务状态
    log_info "检查服务状态..."
    docker-compose ps
    
    # 显示日志
    log_info "显示服务日志..."
    docker-compose logs --tail=50
    
    log_info "Docker 部署完成！"
    log_info "访问地址: http://localhost:6080"
}

# 手动部署
deploy_manual() {
    log_step "手动部署..."
    
    # 安装 PostgreSQL
    install_postgresql
    
    # 配置数据库
    setup_database
    
    # 生成配置文件
    generate_config
    
    # 下载应用程序
    log_info "下载应用程序..."
    ARCH=$(uname -m)
    case $ARCH in
        x86_64)
            BINARY_ARCH="amd64"
            ;;
        aarch64|arm64)
            BINARY_ARCH="arm64"
            ;;
        *)
            log_error "不支持的架构: $ARCH"
            exit 1
            ;;
    esac
    
    BINARY_NAME="usdtmore-linux-$BINARY_ARCH"
    DOWNLOAD_URL="https://github.com/cjs520/USDTMore/releases/latest/download/$BINARY_NAME"
    
    if [[ ! -f $BINARY_NAME ]]; then
        log_info "从 $DOWNLOAD_URL 下载..."
        curl -L -o $BINARY_NAME $DOWNLOAD_URL
        chmod +x $BINARY_NAME
    else
        log_info "应用程序已存在: $BINARY_NAME"
    fi
    
    # 创建系统服务
    if [[ -f docs/usdtmore.service ]]; then
        log_info "安装系统服务..."
        sudo cp docs/usdtmore.service /etc/systemd/system/
        
        # 更新服务文件中的路径
        sudo sed -i "s|/path/to/usdtmore|$(pwd)/$BINARY_NAME|g" /etc/systemd/system/usdtmore.service
        sudo sed -i "s|/path/to/workdir|$(pwd)|g" /etc/systemd/system/usdtmore.service
        
        # 重新加载 systemd
        sudo systemctl daemon-reload
        sudo systemctl enable usdtmore
        sudo systemctl start usdtmore
        
        log_info "系统服务已启动"
        sudo systemctl status usdtmore --no-pager
    else
        log_warn "未找到服务文件，手动启动应用..."
        ./$BINARY_NAME &
        log_info "应用已在后台启动"
    fi
    
    log_info "手动部署完成！"
    log_info "访问地址: http://localhost:6080"
}

# 显示帮助信息
show_help() {
    echo -e "${PURPLE}USDTMore 一键部署脚本${NC}"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -d, --docker     使用 Docker 部署（推荐）"
    echo "  -m, --manual     手动部署"
    echo "  -c, --check      仅检查系统环境"
    echo "  -h, --help       显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 --docker      # Docker 部署"
    echo "  $0 --manual      # 手动部署"
    echo "  $0 --check       # 检查环境"
    echo ""
}

# 主函数
main() {
    echo -e "${PURPLE}"
    echo "=================================================="
    echo "         USDTMore 一键部署脚本 v2.1.0"
    echo "=================================================="
    echo -e "${NC}"
    
    # 检查参数
    case "${1:-}" in
        -d|--docker)
            check_system
            deploy_docker
            ;;
        -m|--manual)
            check_system
            deploy_manual
            ;;
        -c|--check)
            check_system
            log_info "系统检查完成"
            ;;
        -h|--help)
            show_help
            ;;
        "")
            # 无参数时显示交互式菜单
            echo "请选择部署方式:"
            echo "1) Docker 部署（推荐）"
            echo "2) 手动部署"
            echo "3) 仅检查系统环境"
            echo "4) 显示帮助"
            echo ""
            read -p "请输入选择 (1-4): " choice
            
            case $choice in
                1)
                    check_system
                    deploy_docker
                    ;;
                2)
                    check_system
                    deploy_manual
                    ;;
                3)
                    check_system
                    log_info "系统检查完成"
                    ;;
                4)
                    show_help
                    ;;
                *)
                    log_error "无效选择"
                    exit 1
                    ;;
            esac
            ;;
        *)
            log_error "未知参数: $1"
            show_help
            exit 1
            ;;
    esac
    
    echo ""
    echo -e "${GREEN}=================================================="
    echo "                 部署完成！"
    echo "=================================================="
    echo -e "${NC}"
    echo "🎉 USDTMore 已成功部署！"
    echo ""
    echo "📋 接下来的步骤："
    echo "1. 编辑 .env 文件，设置必需的 API 密钥"
    echo "2. 重启服务以应用配置更改"
    echo "3. 访问 http://localhost:6080 开始使用"
    echo ""
    echo "📚 更多信息请查看 README.md"
    echo "🆘 如需帮助，请访问: https://github.com/cjs520/USDTMore"
}

# 执行主函数
main "$@"