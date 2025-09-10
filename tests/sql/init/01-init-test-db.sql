-- 初始化测试数据库脚本
-- Test database initialization script

-- 创建测试数据库（如果不存在）
-- Create test database if not exists
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'usdtmore_test') THEN
        CREATE DATABASE usdtmore_test
            WITH OWNER = test_user
            ENCODING = 'UTF8'
            LC_COLLATE = 'en_US.UTF-8'
            LC_CTYPE = 'en_US.UTF-8'
            TEMPLATE = template0;
    END IF;
END
$$;

-- 连接到测试数据库
\c usdtmore_test;

-- 创建扩展（如果需要）
-- Create extensions if needed
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";

-- 设置时区
-- Set timezone
SET timezone = 'Asia/Shanghai';

-- 创建测试用户的schema（如果需要）
-- Create schema for test user if needed
CREATE SCHEMA IF NOT EXISTS test_user AUTHORIZATION test_user;

-- 授予权限
-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE usdtmore_test TO test_user;
GRANT ALL PRIVILEGES ON SCHEMA public TO test_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO test_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO test_user;

-- 设置默认权限
-- Set default privileges
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO test_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO test_user;

-- 创建测试数据表（预创建，供参考）
-- Create test tables (pre-creation for reference)

-- 钱包地址表
CREATE TABLE IF NOT EXISTS wallet_address (
    id SERIAL PRIMARY KEY,
    chain VARCHAR(255) NOT NULL,
    start_block BIGINT NOT NULL DEFAULT 0,
    in_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    out_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    count BIGINT NOT NULL DEFAULT 0,
    address VARCHAR(255) NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    other_notify INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_wallet_address_chain ON wallet_address(chain);
CREATE INDEX IF NOT EXISTS idx_wallet_address_address ON wallet_address(address);
CREATE INDEX IF NOT EXISTS idx_wallet_address_status ON wallet_address(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_address_chain_address ON wallet_address(chain, address);

-- 交易订单表
CREATE TABLE IF NOT EXISTS trade_orders (
    id SERIAL PRIMARY KEY,
    chain VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    real_amount DECIMAL(20,8) NOT NULL,
    token VARCHAR(50) NOT NULL,
    status INTEGER NOT NULL DEFAULT 0,
    block_id VARCHAR(255),
    callback_status INTEGER NOT NULL DEFAULT 0,
    hash VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_trade_orders_chain ON trade_orders(chain);
CREATE INDEX IF NOT EXISTS idx_trade_orders_address ON trade_orders(address);
CREATE INDEX IF NOT EXISTS idx_trade_orders_status ON trade_orders(status);
CREATE INDEX IF NOT EXISTS idx_trade_orders_hash ON trade_orders(hash);
CREATE INDEX IF NOT EXISTS idx_trade_orders_created_at ON trade_orders(created_at);

-- 通知记录表
CREATE TABLE IF NOT EXISTS notify_record (
    id SERIAL PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    chain VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL,
    hash VARCHAR(255) NOT NULL,
    amount DECIMAL(20,8) NOT NULL,
    status INTEGER NOT NULL DEFAULT 0,
    try_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 创建索引
CREATE INDEX IF NOT EXISTS idx_notify_record_order_id ON notify_record(order_id);
CREATE INDEX IF NOT EXISTS idx_notify_record_status ON notify_record(status);
CREATE INDEX IF NOT EXISTS idx_notify_record_try_count ON notify_record(try_count);
CREATE INDEX IF NOT EXISTS idx_notify_record_created_at ON notify_record(created_at);

-- 创建更新时间触发器函数
-- Create updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 为各表创建更新时间触发器
-- Create updated_at triggers for each table
DROP TRIGGER IF EXISTS update_wallet_address_updated_at ON wallet_address;
CREATE TRIGGER update_wallet_address_updated_at
    BEFORE UPDATE ON wallet_address
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_trade_orders_updated_at ON trade_orders;
CREATE TRIGGER update_trade_orders_updated_at
    BEFORE UPDATE ON trade_orders
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_notify_record_updated_at ON notify_record;
CREATE TRIGGER update_notify_record_updated_at
    BEFORE UPDATE ON notify_record
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 插入一些初始测试数据（可选）
-- Insert some initial test data (optional)
INSERT INTO wallet_address (chain, address, start_block, in_amount, out_amount, count, status, other_notify) 
VALUES 
    ('TRON', 'TRXTestAddress1', 1000, 500.00, 100.00, 5, 1, 1),
    ('POLY', '0xTestAddress1', 2000, 1000.00, 200.00, 10, 1, 1),
    ('BSC', '0xTestAddress2', 3000, 750.00, 150.00, 7, 1, 0)
ON CONFLICT (chain, address) DO NOTHING;

-- 创建测试用的存储过程（可选）
-- Create test stored procedures (optional)
CREATE OR REPLACE FUNCTION get_wallet_stats(p_chain VARCHAR DEFAULT NULL)
RETURNS TABLE(
    total_wallets BIGINT,
    active_wallets BIGINT,
    total_in_amount DECIMAL,
    total_out_amount DECIMAL
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        COUNT(*) as total_wallets,
        COUNT(*) FILTER (WHERE status = 1) as active_wallets,
        COALESCE(SUM(in_amount), 0) as total_in_amount,
        COALESCE(SUM(out_amount), 0) as total_out_amount
    FROM wallet_address 
    WHERE (p_chain IS NULL OR chain = p_chain);
END;
$$ LANGUAGE plpgsql;

-- 输出初始化完成消息
-- Output initialization completion message
DO $$
BEGIN
    RAISE NOTICE 'Test database initialization completed successfully!';
    RAISE NOTICE 'Database: usdtmore_test';
    RAISE NOTICE 'User: test_user';
    RAISE NOTICE 'Tables created: wallet_address, trade_orders, notify_record';
END
$$;