-- PostgreSQL 字段类型优化迁移脚本
-- 优化 USDTMore 项目的数据模型以提供更好的精度和性能

-- ========================================
-- 阶段 1: 备份当前数据表结构
-- ========================================

-- 创建备份表（如果不存在）
CREATE TABLE IF NOT EXISTS trade_orders_backup AS 
SELECT * FROM trade_orders WHERE 1=0;

CREATE TABLE IF NOT EXISTS wallet_address_backup AS 
SELECT * FROM wallet_address WHERE 1=0;

-- ========================================
-- 阶段 2: TradeOrders 表字段类型优化
-- ========================================

BEGIN;

-- 1. 金额精度优化 (高优先级)
-- UsdtRate: varchar -> numeric(18,8) 
ALTER TABLE trade_orders 
ADD COLUMN usdt_rate_new NUMERIC(18,8);

-- 转换现有数据，处理可能的数据格式问题
UPDATE trade_orders 
SET usdt_rate_new = CASE 
    WHEN usdt_rate ~ '^[0-9]+\.?[0-9]*$' THEN usdt_rate::NUMERIC(18,8)
    ELSE NULL
END
WHERE usdt_rate IS NOT NULL AND usdt_rate != '';

-- Amount: decimal(10,2) -> numeric(18,8)
ALTER TABLE trade_orders 
ADD COLUMN amount_new NUMERIC(18,8);

UPDATE trade_orders 
SET amount_new = amount::NUMERIC(18,8);

-- Money: float64 -> numeric(18,2)
ALTER TABLE trade_orders 
ADD COLUMN money_new NUMERIC(18,2);

UPDATE trade_orders 
SET money_new = money::NUMERIC(18,2);

-- 2. 时间戳优化
-- timestamp -> timestamptz
ALTER TABLE trade_orders 
ADD COLUMN expired_at_new TIMESTAMPTZ,
ADD COLUMN created_at_new TIMESTAMPTZ,
ADD COLUMN updated_at_new TIMESTAMPTZ,
ADD COLUMN confirmed_at_new TIMESTAMPTZ;

UPDATE trade_orders 
SET 
    expired_at_new = expired_at AT TIME ZONE 'UTC',
    created_at_new = created_at AT TIME ZONE 'UTC', 
    updated_at_new = updated_at AT TIME ZONE 'UTC',
    confirmed_at_new = CASE WHEN confirmed_at IS NOT NULL THEN confirmed_at AT TIME ZONE 'UTC' ELSE NULL END;

-- 3. 整型优化
-- status, notify_num, notify_state: int -> smallint
ALTER TABLE trade_orders 
ADD COLUMN status_new SMALLINT,
ADD COLUMN notify_num_new SMALLINT,
ADD COLUMN notify_state_new SMALLINT;

UPDATE trade_orders 
SET 
    status_new = status::SMALLINT,
    notify_num_new = notify_num::SMALLINT,
    notify_state_new = notify_state::SMALLINT;

-- 4. 字符串字段优化
-- 交易哈希: varchar(64) -> char(66) (以太坊哈希格式0x+64字符)
ALTER TABLE trade_orders 
ADD COLUMN trade_hash_new CHAR(66);

UPDATE trade_orders 
SET trade_hash_new = CASE 
    WHEN LENGTH(trade_hash) <= 66 THEN trade_hash
    ELSE LEFT(trade_hash, 66)
END;

-- 链名称: varchar(255) -> varchar(20)
ALTER TABLE trade_orders 
ADD COLUMN chain_new VARCHAR(20);

UPDATE trade_orders 
SET chain_new = LEFT(chain, 20);

-- 地址字段: varchar(34) -> varchar(50)
ALTER TABLE trade_orders 
ADD COLUMN address_new VARCHAR(50),
ADD COLUMN from_address_new VARCHAR(50);

UPDATE trade_orders 
SET 
    address_new = LEFT(address, 50),
    from_address_new = LEFT(from_address, 50);

-- URL字段: varchar(255) -> text
ALTER TABLE trade_orders 
ADD COLUMN return_url_new TEXT,
ADD COLUMN notify_url_new TEXT;

UPDATE trade_orders 
SET 
    return_url_new = return_url,
    notify_url_new = notify_url;

COMMIT;

-- ========================================
-- 阶段 3: WalletAddress 表字段类型优化  
-- ========================================

BEGIN;

-- 1. 金额字段: float64/REAL -> numeric(18,8)
ALTER TABLE wallet_address 
ADD COLUMN in_amount_new NUMERIC(18,8),
ADD COLUMN out_amount_new NUMERIC(18,8);

UPDATE wallet_address 
SET 
    in_amount_new = in_amount::NUMERIC(18,8),
    out_amount_new = out_amount::NUMERIC(18,8);

-- 2. 整型优化: integer -> bigint (适合大数值)
ALTER TABLE wallet_address 
ADD COLUMN start_block_new BIGINT,
ADD COLUMN count_new BIGINT;

UPDATE wallet_address 
SET 
    start_block_new = start_block::BIGINT,
    count_new = count::BIGINT;

-- 3. 状态字段: tinyint -> smallint
ALTER TABLE wallet_address 
ADD COLUMN status_new SMALLINT,
ADD COLUMN other_notify_new SMALLINT;

UPDATE wallet_address 
SET 
    status_new = status::SMALLINT,
    other_notify_new = other_notify::SMALLINT;

-- 4. 字符串字段优化
-- 链名称: varchar(255) -> varchar(20)
ALTER TABLE wallet_address 
ADD COLUMN chain_new VARCHAR(20);

UPDATE wallet_address 
SET chain_new = LEFT(chain, 20);

-- 地址: varchar(255) -> varchar(50)
ALTER TABLE wallet_address 
ADD COLUMN address_new VARCHAR(50);

UPDATE wallet_address 
SET address_new = LEFT(address, 50);

-- 5. 时间戳优化: timestamp -> timestamptz
ALTER TABLE wallet_address 
ADD COLUMN created_at_new TIMESTAMPTZ,
ADD COLUMN updated_at_new TIMESTAMPTZ;

UPDATE wallet_address 
SET 
    created_at_new = created_at AT TIME ZONE 'UTC',
    updated_at_new = updated_at AT TIME ZONE 'UTC';

COMMIT;

-- ========================================
-- 阶段 4: NotifyRecord 表字段类型优化
-- ========================================

BEGIN;

-- 1. 主键优化: varchar(64) -> char(66)
ALTER TABLE notify_record 
ADD COLUMN txid_new CHAR(66);

UPDATE notify_record 
SET txid_new = CASE 
    WHEN LENGTH(txid) <= 66 THEN txid
    ELSE LEFT(txid, 66)
END;

-- 2. 时间戳优化: timestamp -> timestamptz
ALTER TABLE notify_record 
ADD COLUMN created_at_new TIMESTAMPTZ,
ADD COLUMN updated_at_new TIMESTAMPTZ;

UPDATE notify_record 
SET 
    created_at_new = created_at AT TIME ZONE 'UTC',
    updated_at_new = updated_at AT TIME ZONE 'UTC';

COMMIT;

-- ========================================
-- 阶段 5: 数据验证检查
-- ========================================

-- 验证 TradeOrders 数据转换
DO $$
DECLARE
    original_count INTEGER;
    converted_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO original_count FROM trade_orders;
    SELECT COUNT(*) INTO converted_count FROM trade_orders WHERE 
        usdt_rate_new IS NOT NULL OR usdt_rate IS NULL OR usdt_rate = '';
    
    IF original_count != converted_count THEN
        RAISE EXCEPTION 'TradeOrders data conversion validation failed: % original, % converted', 
            original_count, converted_count;
    END IF;
    
    RAISE NOTICE 'TradeOrders data validation passed: % records', original_count;
END $$;

-- 验证 WalletAddress 数据转换  
DO $$
DECLARE
    original_count INTEGER;
    converted_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO original_count FROM wallet_address;
    SELECT COUNT(*) INTO converted_count FROM wallet_address WHERE 
        in_amount_new IS NOT NULL AND out_amount_new IS NOT NULL;
    
    IF original_count != converted_count THEN
        RAISE EXCEPTION 'WalletAddress data conversion validation failed: % original, % converted', 
            original_count, converted_count;
    END IF;
    
    RAISE NOTICE 'WalletAddress data validation passed: % records', original_count;
END $$;

-- ========================================
-- 说明：字段切换步骤
-- ========================================

/*
重要提示：以下步骤需要在应用程序停机维护窗口期间执行

1. 停止应用程序服务
2. 执行字段切换操作（参见 004_field_migration_switch.sql）
3. 更新应用程序代码以使用新的字段类型
4. 重启应用程序服务
5. 验证系统功能正常
6. 清理旧字段（在确认稳定后执行）

字段对应关系：
TradeOrders:
- usdt_rate -> usdt_rate_new (NUMERIC(18,8))
- amount -> amount_new (NUMERIC(18,8)) 
- money -> money_new (NUMERIC(18,2))
- status -> status_new (SMALLINT)
- timestamp字段 -> timestamptz字段

WalletAddress:
- in_amount -> in_amount_new (NUMERIC(18,8))
- out_amount -> out_amount_new (NUMERIC(18,8))
- integer字段 -> bigint字段
- status -> status_new (SMALLINT)

NotifyRecord:
- txid -> txid_new (CHAR(66))
- timestamp字段 -> timestamptz字段
*/

-- 记录迁移完成时间
INSERT INTO migration_log (migration_name, executed_at, status) 
VALUES ('003_postgresql_field_optimization', NOW(), 'completed')
ON CONFLICT (migration_name) DO UPDATE SET 
    executed_at = NOW(), 
    status = 'completed';

-- 分析表以更新统计信息，提高查询性能
ANALYZE trade_orders;
ANALYZE wallet_address; 
ANALYZE notify_record;