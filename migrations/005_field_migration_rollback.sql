-- 字段迁移回滚脚本
-- ⚠️ 紧急情况下使用：用于回滚字段类型迁移
-- 使用条件：003和004迁移脚本已执行但应用无法正常启动

-- ========================================
-- 预检查：确保回滚条件满足
-- ========================================

DO $$
BEGIN
    -- 检查迁移日志表是否存在
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'migration_log') THEN
        RAISE EXCEPTION 'Migration log table not found - cannot determine rollback state';
    END IF;
    
    -- 检查是否已执行字段切换
    IF NOT EXISTS (
        SELECT 1 FROM migration_log 
        WHERE migration_name = '004_field_migration_switch' 
        AND status = 'completed'
    ) THEN
        RAISE EXCEPTION 'Field migration switch not completed - rollback not applicable';
    END IF;
    
    RAISE NOTICE 'Rollback preconditions met';
END $$;

-- ========================================
-- 回滚策略说明
-- ========================================

/*
回滚策略：
1. 创建临时备份表保存新格式数据
2. 重新创建原始表结构
3. 转换数据回原始类型（可能有精度损失）
4. 恢复原始约束和索引

⚠️ 警告：
- decimal -> string/float 转换可能有精度损失
- timestamptz -> timestamp 可能有时区信息丢失
- 建议仅在紧急情况下使用，优先考虑修复应用代码适配新结构
*/

-- ========================================
-- 阶段 1: 创建临时备份表
-- ========================================

BEGIN;

-- 备份当前优化后的数据
CREATE TABLE trade_orders_optimized_backup AS SELECT * FROM trade_orders;
CREATE TABLE wallet_address_optimized_backup AS SELECT * FROM wallet_address;  
CREATE TABLE notify_record_optimized_backup AS SELECT * FROM notify_record;

-- 记录回滚开始
INSERT INTO migration_log (migration_name, executed_at, status, rollback_data)
VALUES ('rollback_field_optimization', NOW(), 'started',
    jsonb_build_object(
        'rollback_reason', 'Emergency rollback requested',
        'backup_tables_created', array['trade_orders_optimized_backup', 'wallet_address_optimized_backup', 'notify_record_optimized_backup']
    )
);

COMMIT;

-- ========================================
-- 阶段 2: TradeOrders 表回滚
-- ========================================

BEGIN;

-- 删除当前优化版本的约束
ALTER TABLE trade_orders DROP CONSTRAINT IF EXISTS trade_orders_pkey CASCADE;

-- 重新创建原始格式的字段
ALTER TABLE trade_orders 
    ADD COLUMN usdt_rate_old VARCHAR(10),
    ADD COLUMN amount_old DECIMAL(10,2),
    ADD COLUMN money_old FLOAT8,
    ADD COLUMN status_old INTEGER,
    ADD COLUMN notify_num_old INTEGER,
    ADD COLUMN notify_state_old INTEGER,
    ADD COLUMN trade_hash_old VARCHAR(64),
    ADD COLUMN chain_old VARCHAR(255),
    ADD COLUMN address_old VARCHAR(34),
    ADD COLUMN from_address_old VARCHAR(34),
    ADD COLUMN return_url_old VARCHAR(255),
    ADD COLUMN notify_url_old VARCHAR(255),
    ADD COLUMN expired_at_old TIMESTAMP,
    ADD COLUMN created_at_old TIMESTAMP,
    ADD COLUMN updated_at_old TIMESTAMP,
    ADD COLUMN confirmed_at_old TIMESTAMP;

-- 数据转换（注意精度可能丢失）
UPDATE trade_orders SET
    usdt_rate_old = usdt_rate::TEXT,
    amount_old = CASE 
        WHEN amount > 99999999.99 THEN 99999999.99  -- 防止溢出
        ELSE amount::DECIMAL(10,2) 
    END,
    money_old = money::FLOAT8,
    status_old = status::INTEGER,
    notify_num_old = notify_num::INTEGER,
    notify_state_old = notify_state::INTEGER,
    trade_hash_old = LEFT(trade_hash, 64),
    chain_old = chain,
    address_old = LEFT(address, 34),
    from_address_old = LEFT(from_address, 34),
    return_url_old = LEFT(return_url, 255),
    notify_url_old = LEFT(notify_url, 255),
    expired_at_old = expired_at::TIMESTAMP,
    created_at_old = created_at::TIMESTAMP, 
    updated_at_old = updated_at::TIMESTAMP,
    confirmed_at_old = confirmed_at::TIMESTAMP;

-- 删除优化字段，重命名回原始字段名
ALTER TABLE trade_orders 
    DROP COLUMN usdt_rate,
    DROP COLUMN amount,
    DROP COLUMN money,
    DROP COLUMN status,
    DROP COLUMN notify_num,
    DROP COLUMN notify_state,
    DROP COLUMN trade_hash,
    DROP COLUMN chain,
    DROP COLUMN address,
    DROP COLUMN from_address,
    DROP COLUMN return_url,
    DROP COLUMN notify_url,
    DROP COLUMN expired_at,
    DROP COLUMN created_at,
    DROP COLUMN updated_at,
    DROP COLUMN confirmed_at;

ALTER TABLE trade_orders 
    RENAME COLUMN usdt_rate_old TO usdt_rate,
    RENAME COLUMN amount_old TO amount,
    RENAME COLUMN money_old TO money,
    RENAME COLUMN status_old TO status,
    RENAME COLUMN notify_num_old TO notify_num,
    RENAME COLUMN notify_state_old TO notify_state,
    RENAME COLUMN trade_hash_old TO trade_hash,
    RENAME COLUMN chain_old TO chain,
    RENAME COLUMN address_old TO address,
    RENAME COLUMN from_address_old TO from_address,
    RENAME COLUMN return_url_old TO return_url,
    RENAME COLUMN notify_url_old TO notify_url,
    RENAME COLUMN expired_at_old TO expired_at,
    RENAME COLUMN created_at_old TO created_at,
    RENAME COLUMN updated_at_old TO updated_at,
    RENAME COLUMN confirmed_at_old TO confirmed_at;

-- 恢复原始约束
ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_pkey PRIMARY KEY (id);
ALTER TABLE trade_orders ALTER COLUMN usdt_rate SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN amount SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN money SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN chain SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN address SET NOT NULL;
ALTER TABLE trade_orders ALTER COLUMN status SET NOT NULL;

COMMIT;

-- ========================================
-- 阶段 3: WalletAddress 表回滚
-- ========================================

BEGIN;

-- 删除约束
ALTER TABLE wallet_address DROP CONSTRAINT IF EXISTS wallet_address_pkey CASCADE;

-- 重新创建原始字段
ALTER TABLE wallet_address
    ADD COLUMN in_amount_old FLOAT8,
    ADD COLUMN out_amount_old FLOAT8,
    ADD COLUMN start_block_old INTEGER,
    ADD COLUMN count_old INTEGER,
    ADD COLUMN status_old INTEGER,
    ADD COLUMN other_notify_old INTEGER,
    ADD COLUMN chain_old VARCHAR(255),
    ADD COLUMN address_old VARCHAR(255),
    ADD COLUMN created_at_old TIMESTAMP,
    ADD COLUMN updated_at_old TIMESTAMP;

-- 数据转换
UPDATE wallet_address SET
    in_amount_old = in_amount::FLOAT8,
    out_amount_old = out_amount::FLOAT8,
    start_block_old = CASE 
        WHEN start_block > 2147483647 THEN 2147483647  -- INTEGER 最大值
        ELSE start_block::INTEGER
    END,
    count_old = CASE
        WHEN count > 2147483647 THEN 2147483647
        ELSE count::INTEGER  
    END,
    status_old = status::INTEGER,
    other_notify_old = other_notify::INTEGER,
    chain_old = chain,
    address_old = address,
    created_at_old = created_at::TIMESTAMP,
    updated_at_old = updated_at::TIMESTAMP;

-- 替换字段
ALTER TABLE wallet_address
    DROP COLUMN in_amount,
    DROP COLUMN out_amount,
    DROP COLUMN start_block,
    DROP COLUMN count,
    DROP COLUMN status,
    DROP COLUMN other_notify,
    DROP COLUMN chain,
    DROP COLUMN address,
    DROP COLUMN created_at,
    DROP COLUMN updated_at;

ALTER TABLE wallet_address
    RENAME COLUMN in_amount_old TO in_amount,
    RENAME COLUMN out_amount_old TO out_amount,
    RENAME COLUMN start_block_old TO start_block,
    RENAME COLUMN count_old TO count,
    RENAME COLUMN status_old TO status,
    RENAME COLUMN other_notify_old TO other_notify,
    RENAME COLUMN chain_old TO chain,
    RENAME COLUMN address_old TO address,
    RENAME COLUMN created_at_old TO created_at,
    RENAME COLUMN updated_at_old TO updated_at;

-- 恢复约束
ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_pkey PRIMARY KEY (id);
ALTER TABLE wallet_address ALTER COLUMN chain SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN start_block SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN in_amount SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN out_amount SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN count SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN address SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN status SET NOT NULL;
ALTER TABLE wallet_address ALTER COLUMN other_notify SET NOT NULL;

COMMIT;

-- ========================================
-- 阶段 4: NotifyRecord 表回滚
-- ========================================

BEGIN;

ALTER TABLE notify_record DROP CONSTRAINT IF EXISTS notify_record_pkey CASCADE;

ALTER TABLE notify_record
    ADD COLUMN txid_old VARCHAR(64),
    ADD COLUMN created_at_old TIMESTAMP,
    ADD COLUMN updated_at_old TIMESTAMP;

UPDATE notify_record SET
    txid_old = LEFT(txid, 64),
    created_at_old = created_at::TIMESTAMP,
    updated_at_old = updated_at::TIMESTAMP;

ALTER TABLE notify_record
    DROP COLUMN txid,
    DROP COLUMN created_at,
    DROP COLUMN updated_at;

ALTER TABLE notify_record
    RENAME COLUMN txid_old TO txid,
    RENAME COLUMN created_at_old TO created_at,
    RENAME COLUMN updated_at_old TO updated_at;

ALTER TABLE notify_record ADD CONSTRAINT notify_record_pkey PRIMARY KEY (txid);

COMMIT;

-- ========================================
-- 阶段 5: 恢复基础索引
-- ========================================

-- 恢复原始索引结构
CREATE UNIQUE INDEX IF NOT EXISTS idx_trade_orders_order_id ON trade_orders(order_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_trade_orders_trade_id ON trade_orders(trade_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_trade_orders_trade_hash ON trade_orders(trade_hash) WHERE trade_hash != '';

-- ========================================
-- 阶段 6: 记录回滚完成
-- ========================================

-- 更新回滚状态
UPDATE migration_log 
SET status = 'completed', executed_at = NOW(),
    rollback_data = rollback_data || jsonb_build_object(
        'rollback_completed_at', NOW(),
        'data_conversion_warnings', array[
            'Decimal precision may have been lost in amount fields',
            'Timezone information may have been lost in timestamp fields',
            'Integer overflow protection applied for large values'
        ]
    )
WHERE migration_name = 'rollback_field_optimization';

-- 生成回滚报告
SELECT 
    'ROLLBACK COMPLETED' as status,
    'Original field types restored' as message,
    NOW() as completed_at,
    jsonb_build_object(
        'trade_orders', (SELECT COUNT(*) FROM trade_orders),
        'wallet_address', (SELECT COUNT(*) FROM wallet_address), 
        'notify_record', (SELECT COUNT(*) FROM notify_record),
        'backup_tables', array[
            'trade_orders_optimized_backup',
            'wallet_address_optimized_backup', 
            'notify_record_optimized_backup'
        ]
    ) as rollback_summary;

-- ========================================
-- 清理说明
-- ========================================

/*
回滚完成后的建议操作：

1. 验证应用程序可正常启动
2. 确认核心功能正常工作
3. 分析回滚原因，修复应用代码
4. 重新规划字段优化策略

清理备份表（确认系统稳定后）：
DROP TABLE IF EXISTS trade_orders_optimized_backup;
DROP TABLE IF EXISTS wallet_address_optimized_backup;
DROP TABLE IF EXISTS notify_record_optimized_backup;

重置迁移状态（准备重新迁移时）：
DELETE FROM migration_log WHERE migration_name LIKE '%field%';
*/