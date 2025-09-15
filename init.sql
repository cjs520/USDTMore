-- PostgreSQL initialization script for USDTMore
-- This script will be executed when the PostgreSQL container starts for the first time
-- 整合了所有数据库修复和优化

-- Set timezone
SET timezone TO 'Asia/Shanghai';

-- =============================================================================
-- 1. 基础表结构创建
-- =============================================================================

-- Create wallet_address table with optimized field lengths
CREATE TABLE IF NOT EXISTS wallet_address (
    id BIGSERIAL PRIMARY KEY,
    chain VARCHAR(255) NOT NULL,
    start_block BIGINT NOT NULL DEFAULT 0,
    in_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    out_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    count BIGINT NOT NULL DEFAULT 0,
    address VARCHAR(64) NOT NULL,  -- 扩展到64字符支持长地址
    status SMALLINT NOT NULL DEFAULT 1,
    other_notify SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create trade_orders table with optimized constraints
CREATE TABLE IF NOT EXISTS trade_orders (
    id BIGSERIAL PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    trade_id VARCHAR(255) NOT NULL,
    trade_hash VARCHAR(128) DEFAULT NULL,  -- 扩展到128字符支持长哈希
    usdt_rate VARCHAR(10) NOT NULL,
    amount DECIMAL(10,2) NOT NULL DEFAULT 0,
    money DECIMAL(10,2) NOT NULL DEFAULT 0,
    chain VARCHAR(255) NOT NULL,
    address VARCHAR(64) NOT NULL,  -- 扩展到64字符支持长地址
    from_address VARCHAR(64) NOT NULL DEFAULT '',  -- 扩展到64字符
    status SMALLINT NOT NULL DEFAULT 0,
    return_url VARCHAR(255) NOT NULL DEFAULT '',
    notify_url VARCHAR(255) NOT NULL DEFAULT '',
    notify_num INTEGER NOT NULL DEFAULT 0,
    notify_state SMALLINT NOT NULL DEFAULT 0,
    expired_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    confirmed_at TIMESTAMP WITH TIME ZONE NULL
);

-- Create notify_record table
CREATE TABLE IF NOT EXISTS notify_record (
    id BIGSERIAL PRIMARY KEY,
    txid VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- =============================================================================
-- 2. 数据清理和修复（如果表已存在）
-- =============================================================================

-- 清理可能存在的重复数据和约束冲突
DO $$ 
BEGIN
    -- 删除可能存在的旧约束和索引
    DROP INDEX IF EXISTS uni_trade_orders_trade_hash;
    DROP INDEX IF EXISTS uni_trade_orders_trade_hash_non_empty;
    DROP INDEX IF EXISTS uni_trade_orders_concurrent;
    
    -- 删除GORM可能创建的unique约束
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name LIKE '%trade_hash%' 
        AND table_name = 'trade_orders'
        AND constraint_type = 'UNIQUE'
    ) THEN
        EXECUTE 'ALTER TABLE trade_orders DROP CONSTRAINT ' || (
            SELECT constraint_name 
            FROM information_schema.table_constraints 
            WHERE constraint_name LIKE '%trade_hash%' 
            AND table_name = 'trade_orders'
            AND constraint_type = 'UNIQUE'
            LIMIT 1
        );
    END IF;
    
    -- 删除可能存在的order_id约束（避免重复创建）
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints 
        WHERE constraint_name = 'uni_trade_orders_order_id' 
        AND table_name = 'trade_orders'
        AND constraint_type = 'UNIQUE'
    ) THEN
        ALTER TABLE trade_orders DROP CONSTRAINT uni_trade_orders_order_id;
    END IF;
    
    -- 扩展字段长度（如果表已存在）
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'trade_orders') THEN
        -- 扩展地址字段长度
        ALTER TABLE trade_orders ALTER COLUMN address TYPE VARCHAR(64);
        ALTER TABLE trade_orders ALTER COLUMN from_address TYPE VARCHAR(64);
        ALTER TABLE trade_orders ALTER COLUMN trade_hash TYPE VARCHAR(128);
    END IF;
    
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'wallet_address') THEN
        -- 扩展钱包地址字段长度
        ALTER TABLE wallet_address ALTER COLUMN address TYPE VARCHAR(64);
    END IF;
    
    -- 修复notify_record表结构
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'notify_record') THEN
        -- 添加updated_at字段（如果不存在）
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns 
            WHERE table_name = 'notify_record' AND column_name = 'updated_at'
        ) THEN
            ALTER TABLE notify_record ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
        END IF;
    END IF;
    
    -- 清理空的trade_hash值
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'trade_orders') THEN
        UPDATE trade_orders 
        SET trade_hash = NULL 
        WHERE trade_hash = '';
        
        -- 删除重复的订单记录（保留最新的）
        WITH duplicate_orders AS (
            SELECT id, 
                   ROW_NUMBER() OVER (
                       PARTITION BY order_id 
                       ORDER BY created_at DESC
                   ) as rn
            FROM trade_orders
        )
        DELETE FROM trade_orders 
        WHERE id IN (
            SELECT id FROM duplicate_orders WHERE rn > 1
        );
        
        -- 清理过期订单
        UPDATE trade_orders 
        SET status = 3, -- OrderStatusExpired
            updated_at = NOW()
        WHERE status = 1 -- OrderStatusWaiting
          AND expired_at < NOW();
          
        -- 删除超过24小时的过期订单
        DELETE FROM trade_orders 
        WHERE status = 3 
          AND created_at < NOW() - INTERVAL '24 hours';
    END IF;
END $$;

-- =============================================================================
-- 3. 创建优化的索引和约束
-- =============================================================================

-- 基础性能索引
CREATE INDEX IF NOT EXISTS idx_wallet_address_chain_address ON wallet_address(chain, address);
CREATE INDEX IF NOT EXISTS idx_wallet_address_status ON wallet_address(status);

-- 订单表的核心约束（支持并发）
-- 删除可能存在的旧约束，然后重新创建
DROP INDEX IF EXISTS uni_trade_orders_order_id;
CREATE UNIQUE INDEX uni_trade_orders_order_id ON trade_orders(order_id);

DROP INDEX IF EXISTS uni_trade_orders_trade_id;
CREATE UNIQUE INDEX uni_trade_orders_trade_id ON trade_orders(trade_id);

-- 只对非空trade_hash创建唯一约束，允许多个NULL值
CREATE UNIQUE INDEX IF NOT EXISTS uni_trade_orders_trade_hash_when_not_null 
ON trade_orders (trade_hash) 
WHERE trade_hash IS NOT NULL;

-- 性能优化索引
CREATE INDEX IF NOT EXISTS idx_trade_orders_status ON trade_orders(status);
CREATE INDEX IF NOT EXISTS idx_trade_orders_chain_address ON trade_orders(chain, address);
CREATE INDEX IF NOT EXISTS idx_trade_orders_notify_state ON trade_orders(notify_state);
CREATE INDEX IF NOT EXISTS idx_trade_orders_expired_at ON trade_orders(expired_at);
CREATE INDEX IF NOT EXISTS idx_trade_orders_status_created ON trade_orders(status, created_at);
CREATE INDEX IF NOT EXISTS idx_trade_orders_address_amount ON trade_orders(address, amount);
CREATE INDEX IF NOT EXISTS idx_trade_orders_chain_status ON trade_orders(chain, status);

-- 通知记录索引
CREATE UNIQUE INDEX IF NOT EXISTS uni_notify_record_txid ON notify_record(txid);
CREATE INDEX IF NOT EXISTS idx_notify_record_created_at ON notify_record(created_at);

-- =============================================================================
-- 4. 创建触发器和函数
-- =============================================================================

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers to automatically update updated_at
DROP TRIGGER IF EXISTS update_wallet_address_updated_at ON wallet_address;
CREATE TRIGGER update_wallet_address_updated_at BEFORE UPDATE ON wallet_address
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_trade_orders_updated_at ON trade_orders;
CREATE TRIGGER update_trade_orders_updated_at BEFORE UPDATE ON trade_orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- 5. 权限设置
-- =============================================================================

-- Grant permissions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO usdtmore;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO usdtmore;
GRANT USAGE ON SCHEMA public TO usdtmore;

-- =============================================================================
-- 6. 验证和报告
-- =============================================================================

-- 显示初始化完成信息
SELECT 'USDTMore database initialization completed successfully' as result;

-- 显示表结构信息
SELECT 
    table_name,
    column_name,
    data_type,
    character_maximum_length,
    is_nullable
FROM information_schema.columns 
WHERE table_name IN ('trade_orders', 'wallet_address', 'notify_record')
ORDER BY table_name, ordinal_position;

-- 显示索引信息
SELECT 
    tablename,
    indexname,
    indexdef
FROM pg_indexes 
WHERE tablename IN ('trade_orders', 'wallet_address', 'notify_record')
ORDER BY tablename, indexname;