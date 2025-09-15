-- USDTMore 数据库初始化和修复脚本
-- 版本: v1.2.0
-- 日期: 2025-01-16
-- 说明: 此脚本设计为安全执行，会检查表和字段是否存在再进行操作

-- 1. 检查当前数据库状态
SELECT 'USDTMore数据库修复脚本开始执行...' as status;

-- 检查当前字段长度
SELECT 
    table_name,
    column_name, 
    data_type, 
    character_maximum_length 
FROM information_schema.columns 
WHERE table_name IN ('wallet_addresses', 'trade_orders') 
    AND column_name LIKE '%address%'
ORDER BY table_name, column_name;

-- 2. 修复数据库字段长度限制问题
-- 扩展钱包地址字段从varchar(34)到varchar(64)

-- 扩展钱包地址表的地址字段（仅在表存在时执行）
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'wallet_addresses') THEN
        IF EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'wallet_addresses' AND column_name = 'address') THEN
            ALTER TABLE wallet_addresses ALTER COLUMN address TYPE VARCHAR(64);
            RAISE NOTICE 'wallet_addresses.address字段已扩展到VARCHAR(64)';
        ELSE
            RAISE NOTICE 'wallet_addresses.address字段不存在';
        END IF;
    ELSE
        RAISE NOTICE 'wallet_addresses表不存在，跳过字段扩展';
    END IF;
END $$;

-- 扩展交易订单表的地址字段（仅在表存在时执行）
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'trade_orders') THEN
        -- 检查address字段是否存在
        IF EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'trade_orders' AND column_name = 'address') THEN
            ALTER TABLE trade_orders ALTER COLUMN address TYPE VARCHAR(64);
            RAISE NOTICE 'trade_orders.address字段已扩展到VARCHAR(64)';
        END IF;
        
        -- 检查from_address字段是否存在
        IF EXISTS (SELECT FROM information_schema.columns WHERE table_name = 'trade_orders' AND column_name = 'from_address') THEN
            ALTER TABLE trade_orders ALTER COLUMN from_address TYPE VARCHAR(64);
            RAISE NOTICE 'trade_orders.from_address字段已扩展到VARCHAR(64)';
        END IF;
    ELSE
        RAISE NOTICE 'trade_orders表不存在，跳过字段扩展';
    END IF;
END $$;

-- 3. 修复数据库唯一约束冲突问题
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'trade_orders') THEN
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
        
        -- 删除现有的唯一约束（如果存在）
        DROP INDEX IF EXISTS uni_trade_orders_trade_hash;
        
        -- 重新创建唯一约束，排除空值
        CREATE UNIQUE INDEX uni_trade_orders_trade_hash 
        ON trade_orders (trade_hash) 
        WHERE trade_hash IS NOT NULL AND trade_hash != '';
        
        RAISE NOTICE '已修复trade_orders表的唯一约束冲突问题';
    ELSE
        RAISE NOTICE 'trade_orders表不存在，跳过唯一约束修复';
    END IF;
END $$;

-- 4. 清理过期订单数据
DO $$
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'trade_orders') THEN
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
    ELSE
        RAISE NOTICE 'trade_orders表不存在，跳过过期订单清理';
    END IF;
END $$;

-- 5. 优化数据库性能
DO $$
BEGIN
    -- 为trade_orders表添加索引
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'trade_orders') THEN
        CREATE INDEX IF NOT EXISTS idx_trade_orders_status ON trade_orders(status);
        CREATE INDEX IF NOT EXISTS idx_trade_orders_created_at ON trade_orders(created_at);
        CREATE INDEX IF NOT EXISTS idx_trade_orders_expired_at ON trade_orders(expired_at);
        CREATE INDEX IF NOT EXISTS idx_trade_orders_address ON trade_orders(address);
        CREATE INDEX IF NOT EXISTS idx_trade_orders_chain ON trade_orders(chain);
        CREATE INDEX IF NOT EXISTS idx_trade_orders_trade_hash ON trade_orders(trade_hash);
        RAISE NOTICE '已为trade_orders表创建索引';
    END IF;
    
    -- 为wallet_addresses表添加索引
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'wallet_addresses') THEN
        CREATE INDEX IF NOT EXISTS idx_wallet_addresses_address ON wallet_addresses(address);
        CREATE INDEX IF NOT EXISTS idx_wallet_addresses_chain ON wallet_addresses(chain);
        CREATE INDEX IF NOT EXISTS idx_wallet_addresses_status ON wallet_addresses(status);
        RAISE NOTICE '已为wallet_addresses表创建索引';
    END IF;
    
    -- 为notify_records表添加索引（如果存在）
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'notify_records') THEN
        CREATE INDEX IF NOT EXISTS idx_notify_records_created_at ON notify_records(created_at);
        CREATE INDEX IF NOT EXISTS idx_notify_records_status ON notify_records(status);
        RAISE NOTICE '已为notify_records表创建索引';
    END IF;
END $$;

-- 6. 数据库维护
DO $$
BEGIN
    -- 更新表统计信息
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'trade_orders') THEN
        ANALYZE trade_orders;
        RAISE NOTICE '已更新trade_orders表统计信息';
    END IF;
    
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'wallet_addresses') THEN
        ANALYZE wallet_addresses;
        RAISE NOTICE '已更新wallet_addresses表统计信息';
    END IF;
    
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'notify_records') THEN
        ANALYZE notify_records;
        RAISE NOTICE '已更新notify_records表统计信息';
    END IF;
END $$;

-- 7. 验证修复结果
-- 检查字段长度是否已正确扩展
SELECT 
    'address_fields_check' as check_type,
    table_name,
    column_name, 
    data_type, 
    character_maximum_length 
FROM information_schema.columns 
WHERE table_name IN ('wallet_addresses', 'trade_orders') 
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
WHERE tablename IN ('trade_orders', 'wallet_addresses', 'notify_records')
ORDER BY tablename, indexname;

-- 检查数据完整性
DO $$
DECLARE
    rec RECORD;
BEGIN
    IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'trade_orders') THEN
        RAISE NOTICE '=== 订单状态统计 ===';
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
    ELSE
        RAISE NOTICE 'trade_orders表不存在，跳过数据完整性检查';
    END IF;
END $$;

-- 显示修复完成信息
SELECT 'USDTMore数据库修复脚本执行完成！' as status, NOW() as completed_at;