# 数据库迁移测试 Makefile
# Database Migration Tests Makefile

.PHONY: all test test-sqlite test-postgres test-connection test-migration test-performance test-operations help clean setup build docker-test

# 变量定义
GO_VERSION := $(shell go version | cut -d' ' -f3)
PROJECT_ROOT := $(shell pwd)/..
TEST_DIR := $(shell pwd)
BIN_DIR := $(TEST_DIR)/bin
LOG_DIR := $(TEST_DIR)/test_logs
DATA_DIR := $(TEST_DIR)/test_data
DOCKER_COMPOSE_FILE := $(TEST_DIR)/docker-compose.test.yml

# 默认目标
all: setup test

# 帮助信息
help:
	@echo "数据库迁移测试 Makefile"
	@echo "========================"
	@echo ""
	@echo "可用目标:"
	@echo "  all              - 设置环境并运行所有测试"
	@echo "  setup            - 创建必要的目录和文件"
	@echo "  build            - 构建测试二进制文件"
	@echo "  test             - 运行所有测试"
	@echo "  test-sqlite      - 仅运行 SQLite 测试"
	@echo "  test-postgres    - 仅运行 PostgreSQL 测试"
	@echo "  test-connection  - 仅运行连接测试"
	@echo "  test-migration   - 仅运行迁移测试"
	@echo "  test-performance - 仅运行性能测试"
	@echo "  test-operations  - 仅运行操作测试"
	@echo "  docker-test      - 使用 Docker 运行测试"
	@echo "  docker-up        - 启动测试 Docker 环境"
	@echo "  docker-down      - 停止测试 Docker 环境"
	@echo "  clean            - 清理生成的文件"
	@echo "  clean-docker     - 清理 Docker 资源"
	@echo ""
	@echo "环境变量:"
	@echo "  DB_TYPE          - 数据库类型 (sqlite/postgresql)"
	@echo "  DB_HOST          - PostgreSQL 主机"
	@echo "  DB_PASSWORD      - PostgreSQL 密码"
	@echo ""

# 设置环境
setup:
	@echo "设置测试环境..."
	@mkdir -p $(BIN_DIR) $(LOG_DIR) $(DATA_DIR)
	@echo "✅ 目录创建完成"
	@if [ ! -f "$(TEST_DIR)/.env" ]; then \
		echo "创建默认环境配置文件..."; \
		cp $(TEST_DIR)/test_config.env $(TEST_DIR)/.env; \
		echo "✅ 环境配置文件已创建: $(TEST_DIR)/.env"; \
	fi

# 构建测试二进制文件
build: setup
	@echo "构建测试二进制文件..."
	@cd $(TEST_DIR) && go build -o $(BIN_DIR)/db-connection-test ./db_connection_test.go
	@cd $(TEST_DIR) && go build -o $(BIN_DIR)/db-migration-test ./db_migration_test.go
	@cd $(TEST_DIR) && go build -o $(BIN_DIR)/db-performance-test ./db_performance_test.go
	@cd $(TEST_DIR) && go build -o $(BIN_DIR)/db-operations-test ./db_operations_test.go
	@echo "✅ 测试二进制文件构建完成"

# 运行所有测试
test: setup
	@echo "运行所有数据库测试..."
	@$(TEST_DIR)/run_tests.sh --all

# 仅运行 SQLite 测试
test-sqlite: setup
	@echo "运行 SQLite 测试..."
	@DB_TYPE=sqlite $(TEST_DIR)/run_tests.sh --all

# 仅运行 PostgreSQL 测试
test-postgres: setup
	@echo "运行 PostgreSQL 测试..."
	@DB_TYPE=postgresql $(TEST_DIR)/run_tests.sh --all

# 仅运行连接测试
test-connection: setup
	@echo "运行数据库连接测试..."
	@$(TEST_DIR)/run_tests.sh --connection

# 仅运行迁移测试
test-migration: setup
	@echo "运行数据库迁移测试..."
	@$(TEST_DIR)/run_tests.sh --migration

# 仅运行性能测试
test-performance: setup
	@echo "运行数据库性能测试..."
	@$(TEST_DIR)/run_tests.sh --performance

# 仅运行操作测试
test-operations: setup
	@echo "运行数据库操作测试..."
	@$(TEST_DIR)/run_tests.sh --operations

# Go 测试套件
test-go: setup
	@echo "运行 Go 测试套件..."
	@cd $(TEST_DIR) && go test -v ./... -timeout=30m

# Go 测试带覆盖率
test-coverage: setup
	@echo "运行 Go 测试并生成覆盖率报告..."
	@cd $(TEST_DIR) && go test -v -coverprofile=coverage.out ./...
	@cd $(TEST_DIR) && go tool cover -html=coverage.out -o coverage.html
	@echo "✅ 覆盖率报告生成: $(TEST_DIR)/coverage.html"

# Go 基准测试
test-benchmark: setup
	@echo "运行 Go 基准测试..."
	@cd $(TEST_DIR) && go test -bench=. -benchmem ./...

# Docker 测试
docker-test: docker-up
	@echo "使用 Docker 运行测试..."
	@$(TEST_DIR)/run_tests.sh --docker
	@$(MAKE) docker-down

# 启动 Docker 测试环境
docker-up:
	@echo "启动 Docker 测试环境..."
	@cd $(PROJECT_ROOT) && docker-compose -f $(DOCKER_COMPOSE_FILE) --profile test up -d
	@echo "等待服务启动..."
	@sleep 10
	@echo "✅ Docker 测试环境已启动"

# 停止 Docker 测试环境
docker-down:
	@echo "停止 Docker 测试环境..."
	@cd $(PROJECT_ROOT) && docker-compose -f $(DOCKER_COMPOSE_FILE) down
	@echo "✅ Docker 测试环境已停止"

# 启动带管理界面的 Docker 环境
docker-admin:
	@echo "启动带管理界面的 Docker 环境..."
	@cd $(PROJECT_ROOT) && docker-compose -f $(DOCKER_COMPOSE_FILE) --profile admin up -d
	@echo "✅ PgAdmin 可访问: http://localhost:8081"
	@echo "   用户名: admin@test.com"
	@echo "   密码: admin123"

# 查看 Docker 日志
docker-logs:
	@cd $(PROJECT_ROOT) && docker-compose -f $(DOCKER_COMPOSE_FILE) logs -f

# 连接到 PostgreSQL 容器
docker-psql:
	@echo "连接到 PostgreSQL 测试数据库..."
	@docker exec -it usdtmore-postgres-test psql -U test_user -d usdtmore_test

# 检查系统要求
check-requirements:
	@echo "检查系统要求..."
	@echo "Go 版本: $(GO_VERSION)"
	@go version
	@echo ""
	@if command -v docker >/dev/null 2>&1; then \
		echo "Docker 版本:"; \
		docker --version; \
	else \
		echo "❌ Docker 未安装"; \
	fi
	@echo ""
	@if command -v docker-compose >/dev/null 2>&1; then \
		echo "Docker Compose 版本:"; \
		docker-compose --version; \
	else \
		echo "❌ Docker Compose 未安装"; \
	fi
	@echo ""
	@if command -v psql >/dev/null 2>&1; then \
		echo "✅ PostgreSQL 客户端可用"; \
	else \
		echo "⚠️  PostgreSQL 客户端未安装 (不影响测试)"; \
	fi

# 快速测试（跳过性能测试）
test-quick: setup
	@echo "运行快速测试（跳过性能测试）..."
	@$(TEST_DIR)/run_tests.sh --connection
	@$(TEST_DIR)/run_tests.sh --migration
	@$(TEST_DIR)/run_tests.sh --operations

# 生成测试报告
report:
	@echo "生成测试报告..."
	@if [ -d "$(LOG_DIR)" ]; then \
		echo "最新测试日志:"; \
		ls -la $(LOG_DIR) | head -10; \
		echo ""; \
		if ls $(LOG_DIR)/test_summary_*.json 1> /dev/null 2>&1; then \
			echo "最新测试摘要:"; \
			cat $$(ls -t $(LOG_DIR)/test_summary_*.json | head -1) | jq .; \
		fi \
	else \
		echo "没有找到测试报告"; \
	fi

# 清理生成的文件
clean:
	@echo "清理测试文件..."
	@rm -rf $(BIN_DIR)
	@rm -rf $(DATA_DIR)/*.db
	@rm -rf /tmp/usdtmore_tests
	@rm -rf /tmp/*test*.db
	@find $(LOG_DIR) -name "*.log" -mtime +7 -delete 2>/dev/null || true
	@echo "✅ 清理完成"

# 深度清理（包括日志）
clean-all: clean
	@echo "深度清理..."
	@rm -rf $(LOG_DIR)
	@rm -rf $(DATA_DIR)
	@rm -f $(TEST_DIR)/coverage.out
	@rm -f $(TEST_DIR)/coverage.html
	@echo "✅ 深度清理完成"

# 清理 Docker 资源
clean-docker:
	@echo "清理 Docker 资源..."
	@cd $(PROJECT_ROOT) && docker-compose -f $(DOCKER_COMPOSE_FILE) down -v --remove-orphans
	@docker system prune -f
	@echo "✅ Docker 资源清理完成"

# 安装依赖
install-deps:
	@echo "安装测试依赖..."
	@cd $(PROJECT_ROOT) && go mod tidy
	@cd $(PROJECT_ROOT) && go mod download
	@echo "✅ 依赖安装完成"

# 更新依赖
update-deps:
	@echo "更新测试依赖..."
	@cd $(PROJECT_ROOT) && go get -u ./...
	@cd $(PROJECT_ROOT) && go mod tidy
	@echo "✅ 依赖更新完成"

# 验证模块
verify:
	@echo "验证 Go 模块..."
	@cd $(PROJECT_ROOT) && go mod verify
	@echo "✅ 模块验证完成"

# CI/CD 测试流水线
ci-test: install-deps check-requirements test-quick
	@echo "✅ CI/CD 测试流水线完成"

# 完整测试流水线
full-test: install-deps check-requirements docker-test
	@echo "✅ 完整测试流水线完成"

# 开发测试（快速反馈）
dev-test:
	@echo "开发测试..."
	@$(MAKE) test-connection
	@$(MAKE) test-operations

# 性能基准测试
perf-benchmark:
	@echo "运行性能基准测试..."
	@$(MAKE) test-performance
	@$(MAKE) test-benchmark

# 显示测试统计
stats:
	@echo "测试统计信息:"
	@echo "==============="
	@if [ -d "$(LOG_DIR)" ]; then \
		echo "日志文件数量: $$(ls $(LOG_DIR)/*.log 2>/dev/null | wc -l)"; \
		echo "测试报告数量: $$(ls $(LOG_DIR)/test_summary_*.json 2>/dev/null | wc -l)"; \
		echo "日志目录大小: $$(du -sh $(LOG_DIR) 2>/dev/null | cut -f1)"; \
		echo ""; \
		if ls $(LOG_DIR)/test_summary_*.json 1> /dev/null 2>&1; then \
			echo "最近的测试结果:"; \
			cat $$(ls -t $(LOG_DIR)/test_summary_*.json | head -1) | jq '.summary'; \
		fi \
	else \
		echo "还没有运行过测试"; \
	fi