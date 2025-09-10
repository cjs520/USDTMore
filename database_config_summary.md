# 数据库配置扩展摘要

本次修改为 `/Users/jay/code/Usdt/app/config/config.go` 文件添加了完整的PostgreSQL数据库配置支持，同时保持了原有SQLite配置的兼容性。

## 新增的配置函数

### 数据库类型选择
- `GetDatabaseType()` - 获取数据库类型（"sqlite"或"postgresql"）
- `UsePostgreSQL()` - 检查是否使用PostgreSQL（已存在，改进了）
- `UseSQLite()` - 检查是否使用SQLite

### PostgreSQL连接配置
- `GetPostgreSQLHost()` - 获取PostgreSQL主机地址（默认：localhost）
- `GetPostgreSQLPort()` - 获取PostgreSQL端口（默认：5432）
- `GetPostgreSQLUser()` - 获取PostgreSQL用户名（默认：postgres）
- `GetPostgreSQLPassword()` - 获取PostgreSQL密码
- `GetPostgreSQLDatabase()` - 获取PostgreSQL数据库名（默认：usdtmore）
- `GetPostgreSQLSSLMode()` - 获取PostgreSQL SSL模式（默认：disable）
- `GetPostgreSQLTimeZone()` - 获取PostgreSQL时区（默认：Asia/Shanghai）
- `GetPostgreSQLConnectTimeout()` - 获取连接超时时间（默认：10秒）
- `GetPostgreSQLAppName()` - 获取应用程序名称（默认：usdtmore）

### DSN连接字符串
- `GetPostgreSQLDSN()` - 获取基本PostgreSQL连接DSN（已改进）
- `GetPostgreSQLFullDSN()` - 获取完整PostgreSQL连接DSN（包含所有可选参数）
- `GetDatabaseConnectionString()` - 根据数据库类型返回相应连接字符串

### 连接池配置（已存在，适用于两种数据库）
- `GetDbMaxIdleConns()` - 最大空闲连接数（默认：10）
- `GetDbMaxOpenConns()` - 最大开放连接数（默认：100）
- `GetDbConnMaxLifetime()` - 连接最大生存时间（默认：5分钟）
- `GetDbConnMaxIdleTime()` - 连接最大空闲时间（默认：1分钟）

### 数据库运维配置
- `GetDbBackupPath()` - 数据库备份路径（默认：./backups）
- `GetDbBackupRetentionDays()` - 备份保留天数（默认：7天）
- `IsDbMonitoringEnabled()` - 是否启用数据库监控（默认：false）
- `GetDbMonitoringInterval()` - 数据库监控间隔（默认：30秒）
- `GetDbSlowQueryThreshold()` - 慢查询阈值（默认：1秒）

### 调试配置（已存在）
- `GetDbDebug()` - 数据库调试模式

## 支持的环境变量

### 数据库类型
- `DB_TYPE` - 数据库类型（"sqlite"或"postgresql"/"postgres"）

### PostgreSQL连接参数
- `POSTGRESQL_DSN` - 完整PostgreSQL连接字符串（优先级最高）
- `DB_HOST` - 主机地址
- `DB_PORT` - 端口号
- `DB_USER` - 用户名
- `DB_PASSWORD` - 密码
- `DB_NAME` - 数据库名
- `DB_SSLMODE` - SSL模式
- `DB_TIMEZONE` - 时区
- `DB_CONNECT_TIMEOUT` - 连接超时时间
- `DB_APP_NAME` - 应用程序名称

### 连接池配置
- `DB_MAX_IDLE_CONNS` - 最大空闲连接数
- `DB_MAX_OPEN_CONNS` - 最大开放连接数
- `DB_CONN_MAX_LIFETIME` - 连接最大生存时间
- `DB_CONN_MAX_IDLE_TIME` - 连接最大空闲时间

### 运维配置
- `DB_BACKUP_DIR` - 备份目录
- `DB_BACKUP_RETENTION_DAYS` - 备份保留天数
- `DB_MONITORING_ENABLED` - 是否启用监控
- `DB_MONITORING_INTERVAL` - 监控间隔
- `DB_SLOW_QUERY_THRESHOLD` - 慢查询阈值

### SQLite配置（保持兼容）
- `DB_DIR` - SQLite数据库文件目录

### 调试配置
- `DB_DEBUG` - 数据库调试模式

## 使用示例

### 配置PostgreSQL
```bash
export DB_TYPE="postgresql"
export DB_HOST="localhost"
export DB_PORT="5432"
export DB_USER="usdtmore_user"
export DB_PASSWORD="your_password"
export DB_NAME="usdtmore"
export DB_SSLMODE="require"
export DB_MAX_OPEN_CONNS="50"
export DB_MAX_IDLE_CONNS="5"
```

### 配置SQLite（默认）
```bash
export DB_TYPE="sqlite"
export DB_DIR="/path/to/database"
```

### 或使用完整PostgreSQL DSN
```bash
export DB_TYPE="postgresql"
export POSTGRESQL_DSN="host=localhost port=5432 user=postgres password=yourpassword dbname=usdtmore sslmode=disable TimeZone=Asia/Shanghai"
```

## 兼容性说明

1. **向后兼容**: 如果不设置`DB_TYPE`环境变量，系统默认使用SQLite
2. **现有代码**: 原有的`GetDbPath()`函数继续可用于SQLite
3. **统一接口**: 新增的`GetDatabaseConnectionString()`函数可根据配置自动返回正确的连接字符串

## 连接池最佳实践

建议的连接池配置：
- **小型应用**: MaxOpenConns=10, MaxIdleConns=2
- **中型应用**: MaxOpenConns=50, MaxIdleConns=10
- **大型应用**: MaxOpenConns=100, MaxIdleConns=20
- **ConnMaxLifetime**: 5-30分钟
- **ConnMaxIdleTime**: 1-5分钟

## 备份和监控配置

建议启用以下配置以确保数据安全和性能监控：
```bash
export DB_BACKUP_RETENTION_DAYS="30"
export DB_MONITORING_ENABLED="true"
export DB_MONITORING_INTERVAL="60s"
export DB_SLOW_QUERY_THRESHOLD="2s"
```