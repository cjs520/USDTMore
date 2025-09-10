# 数据库迁移测试工具 / Database Migration Testing Tools

这是一套完整的数据库迁移测试和验证工具，用于确保 SQLite 到 PostgreSQL 的迁移过程是成功和安全的。

## 目录结构 / Directory Structure

```
tests/
├── README.md                    # 本文件 / This file
├── Makefile                     # 测试构建和运行工具 / Test build and run utilities
├── run_tests.sh                 # 主测试运行脚本 / Main test runner script
├── test_config.env              # 测试配置文件 / Test configuration file
├── docker-compose.test.yml      # Docker 测试环境 / Docker test environment
├── Dockerfile.test              # 测试容器 / Test container
├── db_connection_test.go        # 数据库连接测试 / Database connection tests
├── db_migration_test.go         # 数据库迁移验证 / Database migration validation
├── db_performance_test.go       # 性能对比测试 / Performance comparison tests
├── db_operations_test.go        # 数据库操作测试 / Database operations tests
├── sql/
│   └── init/
│       └── 01-init-test-db.sql  # 测试数据库初始化脚本 / Test database initialization
├── bin/                         # 编译后的测试二进制文件 / Compiled test binaries
├── test_logs/                   # 测试日志文件 / Test log files
└── test_data/                   # 测试数据文件 / Test data files
```

## 测试组件 / Test Components

### 1. 数据库连接测试 (`db_connection_test.go`)

- ✅ SQLite 连接测试
- ✅ PostgreSQL 连接测试
- ✅ 连接池测试
- ✅ 连接超时测试
- ✅ 数据库类型切换测试

### 2. 数据库迁移验证 (`db_migration_test.go`)

- ✅ 模式结构验证
- ✅ 数据完整性检查
- ✅ 数据迁移验证
- ✅ 数据一致性比较
- ✅ 事务回滚测试

### 3. 性能对比测试 (`db_performance_test.go`)

- ✅ 批量插入性能
- ✅ 单条插入性能
- ✅ 查询性能测试
- ✅ 更新性能测试
- ✅ 删除性能测试
- ✅ 并发操作测试
- ✅ 事务性能测试

### 4. 数据库操作测试 (`db_operations_test.go`)

- ✅ CRUD 操作完整性
- ✅ 业务逻辑验证
- ✅ 模型方法测试
- ✅ 数据约束测试
- ✅ 索引效果验证

## 快速开始 / Quick Start

### 环境要求 / Requirements

- Go 1.23+
- SQLite (自动处理)
- PostgreSQL 15+ (如果测试 PostgreSQL)
- Docker & Docker Compose (可选，用于隔离测试环境)

### 1. 基本测试运行

```bash
# 进入测试目录
cd tests/

# 运行所有测试
make test

# 或者使用脚本
./run_tests.sh --all
```

### 2. SQLite 测试

```bash
# 仅测试 SQLite
make test-sqlite

# 或者
DB_TYPE=sqlite ./run_tests.sh --all
```

### 3. PostgreSQL 测试

首先配置 PostgreSQL 连接：

```bash
# 编辑配置文件
cp test_config.env .env
vim .env

# 设置 PostgreSQL 配置
export DB_TYPE=postgresql
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=your_password
export DB_NAME=usdtmore_test
```

然后运行测试：

```bash
# 仅测试 PostgreSQL
make test-postgres

# 或者
DB_TYPE=postgresql ./run_tests.sh --all
```

### 4. Docker 环境测试

```bash
# 启动 Docker 测试环境
make docker-up

# 运行测试
make docker-test

# 停止环境
make docker-down
```

## 详细使用说明 / Detailed Usage

### 测试脚本选项 / Test Script Options

```bash
./run_tests.sh [选项]

选项:
  -h, --help              显示帮助信息
  -c, --connection        仅运行连接测试
  -m, --migration         仅运行迁移测试
  -p, --performance       仅运行性能测试
  -o, --operations        仅运行操作测试
  -g, --go-test          运行 Go 测试套件
  -a, --all              运行所有测试 (默认)
  --skip-cleanup         跳过清理
  --skip-go-test         跳过 Go 测试
  --docker               使用 Docker 环境
```

### Makefile 目标 / Makefile Targets

```bash
make help              # 显示帮助
make setup             # 设置测试环境
make build             # 构建测试二进制文件
make test              # 运行所有测试
make test-connection   # 仅运行连接测试
make test-migration    # 仅运行迁移测试
make test-performance  # 仅运行性能测试
make test-operations   # 仅运行操作测试
make docker-test       # Docker 环境测试
make clean             # 清理生成文件
make report            # 生成测试报告
```

### 环境变量配置 / Environment Configuration

在 `test_config.env` 或 `.env` 文件中配置：

```bash
# 数据库类型
DB_TYPE=postgresql  # 或 sqlite

# PostgreSQL 连接配置
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=usdtmore_test
DB_SSLMODE=disable
DB_TIMEZONE=Asia/Shanghai

# 测试配置
DB_DEBUG=true
PERF_TEST_BATCH_SIZE=1000
MIGRATION_TEST_WALLET_COUNT=100
```

## 测试结果解读 / Test Results Interpretation

### 连接测试结果

- ✅ **PASS**: 数据库连接正常，可以进行后续操作
- ❌ **FAIL**: 连接失败，需要检查配置和网络

### 迁移测试结果

- ✅ **Data Matches**: 数据迁移完整，源和目标数据一致
- ❌ **Data Mismatch**: 数据不一致，需要检查迁移逻辑

### 性能测试结果

测试会显示：
- **Ops/Sec**: 每秒操作数
- **Avg Latency**: 平均延迟
- **Ratio**: PostgreSQL 相对于 SQLite 的性能比值

典型性能期望：
- PostgreSQL 在并发和大批量操作中通常优于 SQLite
- SQLite 在单连接和小数据量时可能更快

### 操作测试结果

- ✅ **Success**: 所有 CRUD 操作正常
- ❌ **Failure**: 发现数据操作问题，需要修复

## 故障排除 / Troubleshooting

### 常见问题

1. **PostgreSQL 连接失败**
   ```bash
   # 检查服务状态
   sudo systemctl status postgresql
   
   # 检查连接配置
   psql -h localhost -U postgres -d postgres -c "SELECT version();"
   ```

2. **权限错误**
   ```bash
   # 确保测试用户有权限创建数据库
   sudo -u postgres createuser --createdb --login test_user
   sudo -u postgres psql -c "ALTER USER test_user PASSWORD 'test123456';"
   ```

3. **端口冲突**
   ```bash
   # 检查端口占用
   netstat -tlnp | grep :5432
   
   # 修改 docker-compose.test.yml 中的端口映射
   ports:
     - "5433:5432"  # 使用不同端口
   ```

4. **Go 模块问题**
   ```bash
   # 清理并重新下载依赖
   go clean -modcache
   go mod download
   ```

### 日志分析

测试日志保存在 `test_logs/` 目录：

```bash
# 查看最新测试日志
ls -la test_logs/

# 查看详细错误信息
tail -f test_logs/test_run_*.log

# 查看测试摘要
cat test_logs/test_summary_*.json | jq .
```

## 自动化集成 / Automation Integration

### CI/CD 集成

```yaml
# GitHub Actions 示例
- name: Run Database Migration Tests
  run: |
    cd tests
    make ci-test
```

### 定期测试

```bash
# 添加到 crontab
0 2 * * * cd /path/to/project/tests && make test-quick >> /var/log/db-migration-test.log 2>&1
```

## 扩展和自定义 / Extensions and Customization

### 添加新的测试用例

1. 在相应的测试文件中添加测试函数
2. 在 `run_tests.sh` 中添加新的测试选项
3. 更新 Makefile 添加相应的目标

### 自定义测试数据

修改测试文件中的数据生成函数：

```go
// 在 db_migration_test.go 中
func (dmt *DatabaseMigrationTester) insertTestData() error {
    // 添加你的测试数据
    walletAddresses := []model.WalletAddress{
        // 自定义数据...
    }
    // ...
}
```

### 性能基准调整

在 `test_config.env` 中调整性能测试参数：

```bash
PERF_TEST_BATCH_SIZE=5000      # 批量操作大小
PERF_TEST_CONCURRENT_WORKERS=20 # 并发工作者数量
PERF_TEST_OPS_PER_WORKER=100   # 每个工作者的操作数
```

## 最佳实践 / Best Practices

1. **测试前备份**: 在生产环境迁移前，始终先在测试环境运行完整测试套件
2. **性能基线**: 建立性能基线，定期比较确保没有性能退化
3. **数据验证**: 重点关注数据一致性测试结果，确保迁移后数据完整
4. **逐步迁移**: 对于大型数据库，考虑分批迁移并在每批后运行验证测试
5. **监控日志**: 保留详细的测试日志，便于问题追踪和性能分析

## 支持和反馈 / Support and Feedback

如果遇到问题或有改进建议：

1. 查看 `test_logs/` 中的详细日志
2. 检查配置文件设置
3. 确认数据库服务状态
4. 参考故障排除部分

测试工具会生成详细的 JSON 格式报告，包含所有测试结果和性能指标，便于分析和监控。