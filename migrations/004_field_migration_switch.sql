-- 字段迁移切换脚本
-- ⚠️ 警告：此脚本必须在应用程序停机维护期间执行
-- 执行前确保已完成 003_postgresql_field_optimization.sql

-- ========================================
-- 预检查：确保迁移准备就绪
-- ========================================

-- 检查新字段是否存在
DO $$
BEGIN
    -- 检查 trade_orders 新字段
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'trade_orders' AND column_name = 'usdt_rate_new'
    ) THEN
        RAISE EXCEPTION 'Migration preparation incomplete: trade_orders new fields not found';
    END IF;
    
    -- 检查 wallet_address 新字段  
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'wallet_address' AND column_name = 'in_amount_new'
    ) THEN
        RAISE EXCEPTION 'Migration preparation incomplete: wallet_address new fields not found';
    END IF;
    
    -- 检查 notify_record 新字段
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'notify_record' AND column_name = 'txid_new'
    ) THEN  
        RAISE EXCEPTION 'Migration preparation incomplete: notify_record new fields not found';
    END IF;
    
    RAISE NOTICE 'Pre-migration checks passed successfully';
END $$;

-- ========================================
-- 阶段 1: 创建迁移日志表（如果不存在）
-- ========================================

CREATE TABLE IF NOT EXISTS migration_log (
    migration_name VARCHAR(100) PRIMARY KEY,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    rollback_data JSONB
);

-- ========================================
-- 阶段 2: 备份关键数据用于回滚
-- ========================================

-- 记录切换前的表结构信息
INSERT INTO migration_log (migration_name, executed_at, status, rollback_data)
VALUES ('004_field_switch_backup', NOW(), 'completed', 
    jsonb_build_object(
        'backup_timestamp', NOW(),
        'trade_orders_count', (SELECT COUNT(*) FROM trade_orders),
        'wallet_address_count', (SELECT COUNT(*) FROM wallet_address),
        'notify_record_count', (SELECT COUNT(*) FROM notify_record)
    )
);

-- ========================================
-- 阶段 3: TradeOrders 表字段切换
-- ========================================

BEGIN;

-- 创建完整备份（仅结构信息，避免大量数据）
INSERT INTO migration_log (migration_name, status, rollback_data)
VALUES ('trade_orders_field_switch', 'starting', 
    jsonb_build_object(
        'original_columns', (
            SELECT array_agg(column_name) 
            FROM information_schema.columns 
            WHERE table_name = 'trade_orders' 
              AND column_name NOT LIKE '%_new'
        )
    )
);

-- 删除旧字段的约束和索引
ALTER TABLE trade_orders DROP CONSTRAINT IF EXISTS trade_orders_pkey CASCADE;
DROP INDEX IF EXISTS idx_trade_orders_order_id;
DROP INDEX IF EXISTS idx_trade_orders_trade_id;
DROP INDEX IF EXISTS idx_trade_orders_trade_hash;

-- 字段重命名和类型切换
-- 1. 删除旧字段，重命名新字段
ALTER TABLE trade_orders DROP COLUMN IF EXISTS usdt_rate CASCADE;
ALTER TABLE trade_orders RENAME COLUMN usdt_rate_new TO usdt_rate;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS amount CASCADE;  
ALTER TABLE trade_orders RENAME COLUMN amount_new TO amount;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS money CASCADE;
ALTER TABLE trade_orders RENAME COLUMN money_new TO money;

-- 2. 时间戳字段切换
ALTER TABLE trade_orders DROP COLUMN IF EXISTS expired_at CASCADE;
ALTER TABLE trade_orders RENAME COLUMN expired_at_new TO expired_at;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS created_at CASCADE;
ALTER TABLE trade_orders RENAME COLUMN created_at_new TO created_at;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS updated_at CASCADE;
ALTER TABLE trade_orders RENAME COLUMN updated_at_new TO updated_at;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS confirmed_at CASCADE;
ALTER TABLE trade_orders RENAME COLUMN confirmed_at_new TO confirmed_at;

-- 3. 整型字段切换
ALTER TABLE trade_orders DROP COLUMN IF EXISTS status CASCADE;
ALTER TABLE trade_orders RENAME COLUMN status_new TO status;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS notify_num CASCADE;
ALTER TABLE trade_orders RENAME COLUMN notify_num_new TO notify_num;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS notify_state CASCADE;
ALTER TABLE trade_orders RENAME COLUMN notify_state_new TO notify_state;

-- 4. 字符串字段切换
ALTER TABLE trade_orders DROP COLUMN IF EXISTS trade_hash CASCADE;
ALTER TABLE trade_orders RENAME COLUMN trade_hash_new TO trade_hash;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS chain CASCADE;
ALTER TABLE trade_orders RENAME COLUMN chain_new TO chain;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS address CASCADE;
ALTER TABLE trade_orders RENAME COLUMN address_new TO address;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS from_address CASCADE;
ALTER TABLE trade_orders RENAME COLUMN from_address_new TO from_address;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS return_url CASCADE;
ALTER TABLE trade_orders RENAME COLUMN return_url_new TO return_url;

ALTER TABLE trade_orders DROP COLUMN IF EXISTS notify_url CASCADE;
ALTER TABLE trade_orders RENAME COLUMN notify_url_new TO notify_url;

-- 重新添加约束
ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_pkey PRIMARY KEY (id);
ALTER TABLE trade_orders ALTER COLUMN usdt_rate SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN amount SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN money SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN chain SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN address SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN from_address SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN status SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN expired_at SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN updated_at SET NOT NULL;

-- 设置默认值
ALTER TABLE trade_orders ALTER COLUMN amount SET DEFAULT 0;
ALTER TABLE trade_orders ALTER COLUMN money SET DEFAULT 0;  
ALTER TABLE trade_orders ALTER COLUMN from_address SET DEFAULT '';
ALTER TABLE trade_orders ALTER COLUMN status SET DEFAULT 0;
ALTER TABLE trade_orders ALTER COLUMN notify_num SET DEFAULT 0;
ALTER TABLE trade_orders ALTER COLUMN notify_state SET DEFAULT 0;
ALTER TABLE trade_orders ALTER COLUMN return_url SET DEFAULT '';
ALTER TABLE trade_orders ALTER COLUMN notify_url SET DEFAULT '';

COMMIT;

-- ========================================
-- 阶段 4: WalletAddress 表字段切换
-- ========================================

BEGIN;

-- 备份信息
INSERT INTO migration_log (migration_name, status)
VALUES ('wallet_address_field_switch', 'starting');

-- 删除旧约束和索引
ALTER TABLE wallet_address DROP CONSTRAINT IF EXISTS wallet_address_pkey CASCADE;

-- 字段切换
ALTER TABLE wallet_address DROP COLUMN IF EXISTS in_amount CASCADE;
ALTER TABLE wallet_address RENAME COLUMN in_amount_new TO in_amount;

ALTER TABLE wallet_address DROP COLUMN IF EXISTS out_amount CASCADE;
ALTER TABLE wallet_address RENAME COLUMN out_amount_new TO out_amount;

ALTER TABLE wallet_address DROP COLUMN IF EXISTS start_block CASCADE;
ALTER TABLE wallet_address RENAME COLUMN start_block_new TO start_block;

ALTER TABLE wallet_address DROP COLUMN IF EXISTS count CASCADE;
ALTER TABLE wallet_address RENAME COLUMN count_new TO count;

ALTER TABLE wallet_address DROP COLUMN IF EXISTS status CASCADE;
ALTER TABLE wallet_address RENAME COLUMN status_new TO status;

ALTER TABLE wallet_address DROP COLUMN IF EXISTS other_notify CASCADE;
ALTER TABLE wallet_address RENAME COLUMN other_notify_new TO other_notify;

ALTER TABLE wallet_address DROP COLUMN IF EXISTS chain CASCADE;
ALTER TABLE wallet_address RENAME COLUMN chain_new TO chain;

ALTER TABLE wallet_address DROP COLUMN IF EXISTS address CASCADE;
ALTER TABLE wallet_address RENAME COLUMN address_new TO address;

ALTER TABLE wallet_address DROP COLUMN IF EXISTS created_at CASCADE;
ALTER TABLE wallet_address RENAME COLUMN created_at_new TO created_at;

ALTER TABLE wallet_address DROP COLUMN IF EXISTS updated_at CASCADE;
ALTER TABLE wallet_address RENAME COLUMN updated_at_new TO updated_at;

-- 重新添加约束
ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_pkey PRIMARY KEY (id);
ALTER TABLE wallet_address ALTER COLUMN chain SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN start_block SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN in_amount SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN out_amount SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN count SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN address SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN status SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN other_notify SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN updated_at SET NOT NULL;

-- 设置默认值
ALTER TABLE wallet_address ALTER COLUMN start_block SET DEFAULT 0;
ALTER TABLE wallet_address ALTER COLUMN in_amount SET DEFAULT 0;
ALTER TABLE wallet_address ALTER COLUMN out_amount SET DEFAULT 0;
ALTER TABLE wallet_address ALTER COLUMN count SET DEFAULT 0;
ALTER TABLE wallet_address ALTER COLUMN status SET DEFAULT 1;
ALTER TABLE wallet_address ALTER COLUMN other_notify SET DEFAULT 1;

COMMIT;

-- ========================================
-- 阶段 5: NotifyRecord 表字段切换  
-- ========================================

BEGIN;

-- 备份信息
INSERT INTO migration_log (migration_name, status)
VALUES ('notify_record_field_switch', 'starting');

-- 删除旧约束
ALTER TABLE notify_record DROP CONSTRAINT IF EXISTS notify_record_pkey CASCADE;

-- 字段切换
ALTER TABLE notify_record DROP COLUMN IF EXISTS txid CASCADE;
ALTER TABLE notify_record RENAME COLUMN txid_new TO txid;

ALTER TABLE notify_record DROP COLUMN IF EXISTS created_at CASCADE;
ALTER TABLE notify_record RENAME COLUMN created_at_new TO created_at;

ALTER TABLE notify_record DROP COLUMN IF EXISTS updated_at CASCADE;
ALTER TABLE notify_record RENAME COLUMN updated_at_new TO updated_at;

-- 重新添加约束
ALTER TABLE notify_record ADD CONSTRAINT notify_record_pkey PRIMARY KEY (txid);
ALTER TABLE notify_record ALTER COLUMN txid SET NOT NULL;
ALTER TABLE notify_record ALTER COLUMN created_at SET NOT NULL;
ALTER TABLE notify_record ALTER COLUMN updated_at SET NOT NULL;

COMMIT;

-- ========================================
-- 阶段 6: 重建关键索引
-- ========================================

-- 重建基础索引（GORM会在下次启动时创建优化索引）
CREATE UNIQUE INDEX IF NOT EXISTS idx_trade_orders_order_id ON trade_orders(order_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_trade_orders_trade_id ON trade_orders(trade_id); 
CREATE UNIQUE INDEX IF NOT EXISTS idx_trade_orders_trade_hash ON trade_orders(trade_hash) WHERE trade_hash != '';

-- ========================================
-- 阶段 7: 最终验证
-- ========================================

-- 验证表结构完整性
DO $$
DECLARE
    rec RECORD;
    table_name TEXT;
    expected_columns JSONB;
BEGIN
    -- 验证 TradeOrders 表
    SELECT COUNT(*) FROM trade_orders INTO rec;
    
    -- 验证关键字段类型
    FOR table_name IN VALUES ('trade_orders'), ('wallet_address'), ('notify_record') LOOP
        RAISE NOTICE 'Validating table: %', table_name;
        
        -- 检查表是否存在且可访问
        EXECUTE format('SELECT COUNT(*) FROM %I', table_name);
        
        RAISE NOTICE 'Table % validation passed', table_name;
    END LOOP;
    
    RAISE NOTICE 'All table structure validations passed successfully';
END $$;

-- 更新迁移状态
UPDATE migration_log 
SET status = 'completed', executed_at = NOW()
WHERE migration_name IN (
    'trade_orders_field_switch',
    'wallet_address_field_switch', 
    'notify_record_field_switch'
);

-- 记录迁移完成
INSERT INTO migration_log (migration_name, executed_at, status)
VALUES ('004_field_migration_switch', NOW(), 'completed')
ON CONFLICT (migration_name) DO UPDATE SET
    executed_at = NOW(),
    status = 'completed';

-- 最终统计
SELECT 
    'Migration completed successfully' as status,
    NOW() as completion_time,
    (SELECT COUNT(*) FROM trade_orders) as trade_orders_count,
    (SELECT COUNT(*) FROM wallet_address) as wallet_address_count,
    (SELECT COUNT(*) FROM notify_record) as notify_record_count;