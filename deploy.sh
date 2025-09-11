#!/bin/bash

# USDTMore PostgreSQL Deployment Script
# This script provides simplified deployment commands for the PostgreSQL-only configuration

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        print_error "Docker is not running. Please start Docker first."
        exit 1
    fi
}

# Function to check if docker-compose is available
check_compose() {
    if ! command -v docker-compose &> /dev/null; then
        if ! docker compose version &> /dev/null; then
            print_error "Docker Compose is not available. Please install Docker Compose."
            exit 1
        else
            COMPOSE_CMD="docker compose"
        fi
    else
        COMPOSE_CMD="docker-compose"
    fi
}

# Function to check if .env file exists
check_env() {
    if [[ ! -f .env ]]; then
        print_warning ".env file not found. Creating from .env.example..."
        if [[ -f .env.example ]]; then
            cp .env.example .env
            print_info "Please edit .env file with your configuration before deployment."
            print_info "Required fields to update:"
            echo "  - DB_PASSWORD"
            echo "  - TG_BOT_TOKEN"
            echo "  - TG_BOT_ADMIN_ID"
            echo "  - ETHERSCAN_API_KEY"
            echo "  - TRON_SCAN_API_KEY"
            echo "  - SOLANA_API_KEY"
            echo "  - AUTH_TOKEN"
            echo "  - APP_URI"
            read -p "Press Enter after updating .env file to continue..."
        else
            print_error ".env.example file not found. Cannot create .env file."
            exit 1
        fi
    fi
}

# Function to deploy the application
deploy() {
    print_info "Starting USDTMore deployment with PostgreSQL..."
    
    check_docker
    check_compose
    check_env
    
    print_info "Building and starting containers..."
    $COMPOSE_CMD up -d
    
    print_info "Waiting for services to be healthy..."
    sleep 10
    
    # Check service status
    if $COMPOSE_CMD ps | grep -q "healthy"; then
        print_success "Deployment completed successfully!"
        print_info "Services:"
        $COMPOSE_CMD ps
        echo ""
        print_info "USDTMore is now available at: http://localhost:6080"
        print_info "PostgreSQL is available at: localhost:5432"
        print_info "Redis is available at: localhost:6379"
    else
        print_error "Some services may not be healthy. Check logs:"
        $COMPOSE_CMD logs
    fi
}

# Function to stop the application
stop() {
    print_info "Stopping USDTMore services..."
    check_compose
    $COMPOSE_CMD down
    print_success "Services stopped successfully."
}

# Function to restart the application
restart() {
    print_info "Restarting USDTMore services..."
    stop
    sleep 2
    deploy
}

# Function to show logs
logs() {
    check_compose
    if [[ -n $1 ]]; then
        $COMPOSE_CMD logs -f $1
    else
        $COMPOSE_CMD logs -f
    fi
}

# Function to show status
status() {
    check_compose
    print_info "Service status:"
    $COMPOSE_CMD ps
    echo ""
    print_info "Docker containers:"
    docker ps --filter "name=usdtmore"
}

# Function to backup database
backup() {
    check_compose
    print_info "Creating PostgreSQL backup..."
    BACKUP_FILE="usdtmore_backup_$(date +%Y%m%d_%H%M%S).sql"
    $COMPOSE_CMD exec postgres pg_dump -U usdtmore_user usdtmore > $BACKUP_FILE
    print_success "Database backup created: $BACKUP_FILE"
}

# Function to clean up
cleanup() {
    print_warning "This will remove all containers, volumes, and data. Are you sure? (y/N)"
    read -r response
    if [[ $response =~ ^[Yy]$ ]]; then
        check_compose
        print_info "Cleaning up containers and volumes..."
        $COMPOSE_CMD down -v --remove-orphans
        docker system prune -f
        print_success "Cleanup completed."
    else
        print_info "Cleanup cancelled."
    fi
}

# Function to show help
show_help() {
    echo "USDTMore PostgreSQL Deployment Script"
    echo ""
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  deploy     Deploy the application (default)"
    echo "  stop       Stop all services"
    echo "  restart    Restart all services"
    echo "  status     Show service status"
    echo "  logs       Show logs (optionally specify service name)"
    echo "  backup     Create PostgreSQL database backup"
    echo "  cleanup    Remove all containers and volumes"
    echo "  help       Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 deploy"
    echo "  $0 logs postgres"
    echo "  $0 logs usdtmore"
    echo "  $0 backup"
}

# Main script logic
case "${1:-deploy}" in
    deploy)
        deploy
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    status)
        status
        ;;
    logs)
        logs $2
        ;;
    backup)
        backup
        ;;
    cleanup)
        cleanup
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        print_error "Unknown command: $1"
        show_help
        exit 1
        ;;
esac