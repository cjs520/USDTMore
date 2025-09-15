-- USDTMore 数据库完整初始化脚本
-- 版本: v2.0.0
-- 日期: 2025-01-16
-- 说明: 此脚本会创建所有必需的表结构，然后执行修复操作

-- 1. 开始执行提示
SELECT 'USDTMore数据库完整初始化脚本开始执行...' as status;

-- 2. 创建钱包地址表 (wallet_address)
CREATE TABLE IF NOT EXISTS wallet_address (
    id BIGSERIAL PRIMARY KEY,
    chain VARCHAR(255) NOT NULL COMMENT '链路名称 TRON POLY OP BSC',
    start_block BIGINT NOT NULL DEFAULT 0 COMMENT '初始化块，每次查询记录一天之前的blocknum',
    in_amount DECIMAL(20,8) NOT NULL DEFAULT 0 COMMENT '累计转入',
    out_amount DECIMAL(20,8) NOT NULL DEFAULT 0 COMMENT '累计转出',
    count BIGINT NOT NULL DEFAULT 0 COMMENT '历史订单数量',
    address VARCHAR(64) NOT NULL COMMENT '钱包地址',
    status SMALLINT NOT NULL DEFAULT 1 COMMENT '地址状态 1启动 0禁止',
    other_notify SMALLINT NOT NULL DEFAULT 1 COMMENT '其它转账通知 1启动 0禁止',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间'
);

-- 3. 创建交易订单表 (trade_orders)
CREATE TABLE IF NOT EXISTS trade_orders (
    id BIGSERIAL PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL COMMENT '客户订单ID',
    trade_id VARCHAR(255) NOT NULL COMMENT '本地订单ID',
    trade_hash VARCHAR(128) DEFAULT '' COMMENT '交易哈希',
    usdt_rate VARCHAR(10) NOT NULL COMMENT 'USDT汇率',
    amount DECIMAL(10,2) NOT NULL DEFAULT 0 COMMENT 'USDT交易数额',
    money DECIMAL(10,2) NOT NULL DEFAULT 0 COMMENT '订单交易金额',
    chain VARCHAR(255) NOT NULL COMMENT '链路名称 TRON POLY OP BSC',
    address VARCHAR(64) NOT NULL COMMENT '收款地址',
    from_address VARCHAR(64) NOT NULL DEFAULT '' COMMENT '支付地址',
    status SMALLINT NOT NULL DEFAULT 0 COMMENT '交易状态 1：等待支付 2：支付成功 3：订单过期',
    return_url VARCHAR(255) NOT NULL DEFAULT '' COMMENT '同步地址',
    notify_url VARCHAR(255) NOT NULL DEFAULT '' COMMENT '异步地址',
    notify_num INTEGER NOT NULL DEFAULT 0 COMMENT '回调次数',
    notify_state SMALLINT NOT NULL DEFAULT 0 COMMENT '回调状态 1：成功 0：失败',
    expired_at TIMESTAMP NOT NULL COMMENT '订单失效时间',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间',
    confirmed_at TIMESTAMP NULL COMMENT '交易确认时间'
);

-- 4. 创建通知记录表 (notify_record)
CREATE TABLE IF NOT EXISTS notify_record (
    txid VARCHAR(66) PRIMARY KEY COMMENT '交易哈希',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '更新时间'
);

-- 5. 创建基础索引
-- wallet_address表索引
CREATE INDEX IF NOT EXISTS idx_wallet_address_chain ON wallet_address(chain);
CREATE INDEX IF NOT EXISTS idx_wallet_address_address ON wallet_address(address);
CREATE INDEX IF NOT EXISTS idx_wallet_address_status ON wallet_address(status);
CREATE INDEX IF NOT EXISTS idx_wallet_address_chain_address ON wallet_address(chain, address);

-- trade_orders表索引
CREATE INDEX IF NOT EXISTS idx_trade_orders_status ON trade_orders(status);
CREATE INDEX IF NOT EXISTS idx_trade_orders_created_at ON trade_orders(created_at);
CREATE INDEX IF NOT EXISTS idx_trade_orders_expired_at ON trade_orders(expired_at);
CREATE INDEX IF NOT EXISTS idx_trade_orders_address ON trade_orders(address);
CREATE INDEX IF NOT EXISTS idx_trade_orders_chain ON trade_orders(chain);
CREATE INDEX IF NOT EXISTS idx_trade_orders_trade_hash ON trade_orders(trade_hash);
CREATE INDEX IF NOT EXISTS idx_trade_orders_trade_id ON trade_orders(trade_id);
CREATE INDEX IF NOT EXISTS idx_trade_orders_order_id ON trade_orders(order_id);

-- notify_record表索引
CREATE INDEX IF NOT EXISTS idx_notify_record_created_at ON notify_record(created_at);

-- 6. 创建唯一约束
-- trade_orders表唯一约束（排除空值）
CREATE UNIQUE INDEX IF NOT EXISTS uni_trade_orders_trade_hash 
ON trade_orders (trade_hash) 
WHERE trade_hash IS NOT NULL AND trade_hash != '';

-- 7. 数据库修复和优化操作
-- 检查当前数据库状态
SELECT 'USDTMore数据库表结构创建完成，开始执行修复操作...' as status;

-- 检查当前字段长度
SELECT 
    table_name,
    column_name, 
    data_type, 
    character_maximum_length 
FROM information_schema.columns 
WHERE table_name IN ('wallet_address', 'trade_orders') 
    AND column_name LIKE '%address%'
ORDER BY table_name, column_name;

-- 8. 修复数据库字段长度限制问题（如果需要）
-- 扩展钱包地址字段从varchar(34)到varchar(64)
DO $$
BEGIN
    -- 检查wallet_address表的address字段长度
    IF EXISTS (
        SELECT FROM information_schema.columns 
        WHERE table_name = 'wallet_address' 
        AND column_name = 'address' 
        AND character_maximum_length < 64
    ) THEN
        ALTER TABLE wallet_address ALTER COLUMN address TYPE VARCHAR(64);
        RAISE NOTICE 'wallet_address.address字段已扩展到VARCHAR(64)';
    END IF;
    
    -- 检查trade_orders表的address字段长度
    IF EXISTS (
        SELECT FROM information_schema.columns 
        WHERE table_name = 'trade_orders' 
        AND column_name = 'address' 
        AND character_maximum_length < 64
    ) THEN
        ALTER TABLE trade_orders ALTER COLUMN address TYPE VARCHAR(64);
        RAISE NOTICE 'trade_orders.address字段已扩展到VARCHAR(64)';
    END IF;
    
    -- 检查trade_orders表的from_address字段长度
    IF EXISTS (
        SELECT FROM information_schema.columns 
        WHERE table_name = 'trade_orders' 
        AND column_name = 'from_address' 
        AND character_maximum_length < 64
    ) THEN
        ALTER TABLE trade_orders ALTER COLUMN from_address TYPE VARCHAR(64);
        RAISE NOTICE 'trade_orders.from_address字段已扩展到VARCHAR(64)';
    END IF;
END $$;

-- 9. 修复数据库唯一约束冲突问题
DO $$
BEGIN
    -- 删除重复的空trade_hash值
    DELETE FROM trade_orders 
    WHERE id IN (
        SELECT id FROM (
            SELECT id, ROW_NUMBER() OVER (
                PARTITION BY COALESCE(trade_hash, '') 
                ORDER BY created_at DESC
            ) as rn
            FROM trade_orders 
            WHERE trade_hash IS NULL OR trade_hash = ''
        ) t 
        WHERE t.rn > 1
    );
    
    RAISE NOTICE '已清理重复的空trade_hash记录';
END $$;

-- 10. 清理过期订单数据
DO $$
BEGIN
    -- 删除超过30天的已过期订单
    DELETE FROM trade_orders 
    WHERE status IN (3, 4) -- 过期或失败状态
        AND created_at < NOW() - INTERVAL '30 days';
    
    -- 更新超过24小时的待支付订单为过期状态
    UPDATE trade_orders 
    SET status = 3, -- 设置为过期状态
        updated_at = NOW()
    WHERE status = 1 -- 待支付状态
        AND created_at < NOW() - INTERVAL '24 hours';
        
    RAISE NOTICE '已清理过期订单数据';
END $$;

-- 11. 更新表统计信息
DO $$
BEGIN
    ANALYZE wallet_address;
    ANALYZE trade_orders;
    ANALYZE notify_record;
    RAISE NOTICE '已更新所有表的统计信息';
END $$;

-- 12. 验证修复结果
-- 检查表是否正确创建
SELECT 
    'tables_check' as check_type,
    table_name,
    table_type
FROM information_schema.tables 
WHERE table_name IN ('wallet_address', 'trade_orders', 'notify_record')
ORDER BY table_name;

-- 检查字段长度是否正确
SELECT 
    'address_fields_check' as check_type,
    table_name,
    column_name, 
    data_type, 
    character_maximum_length 
FROM information_schema.columns 
WHERE table_name IN ('wallet_address', 'trade_orders') 
    AND column_name LIKE '%address%'
ORDER BY table_name, column_name;

-- 检查索引是否正确创建
SELECT 
    'indexes_check' as check_type,
    schemaname,
    tablename,
    indexname,
    indexdef
FROM pg_indexes 
WHERE tablename IN ('trade_orders', 'wallet_address', 'notify_record')
ORDER BY tablename, indexname;

-- 13. 数据完整性检查
DO $$
DECLARE
    rec RECORD;
    table_count INTEGER;
BEGIN
    -- 检查trade_orders表
    SELECT COUNT(*) INTO table_count FROM trade_orders;
    RAISE NOTICE '=== trade_orders表统计 ===';
    RAISE NOTICE '总记录数: %', table_count;
    
    IF table_count > 0 THEN
        FOR rec IN 
            SELECT 
                status,
                COUNT(*) as count,
                MIN(created_at) as oldest,
                MAX(created_at) as newest
            FROM trade_orders 
            GROUP BY status 
            ORDER BY status
        LOOP
            RAISE NOTICE '状态: %, 数量: %, 最早: %, 最新: %', rec.status, rec.count, rec.oldest, rec.newest;
        END LOOP;
    END IF;
    
    -- 检查wallet_address表
    SELECT COUNT(*) INTO table_count FROM wallet_address;
    RAISE NOTICE '=== wallet_address表统计 ===';
    RAISE NOTICE '总记录数: %', table_count;
    
    IF table_count > 0 THEN
        FOR rec IN 
            SELECT 
                chain,
                status,
                COUNT(*) as count
            FROM wallet_address 
            GROUP BY chain, status 
            ORDER BY chain, status
        LOOP
            RAISE NOTICE '链: %, 状态: %, 数量: %', rec.chain, rec.status, rec.count;
        END LOOP;
    END IF;
    
    -- 检查notify_record表
    SELECT COUNT(*) INTO table_count FROM notify_record;
    RAISE NOTICE '=== notify_record表统计 ===';
    RAISE NOTICE '总记录数: %', table_count;
END $$;

-- 14. 显示完成信息
SELECT 'USDTMore数据库完整初始化脚本执行完成！' as status, NOW() as completed_at;
SELECT '所有表结构已创建，索引已建立，数据已修复！' as message;