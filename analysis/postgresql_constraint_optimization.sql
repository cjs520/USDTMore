-- PostgreSQL 数据完整性和约束优化分析
-- 基于 USDTMore 项目的业务规则和数据一致性需求

-- ========================================
-- 1. 当前约束状况分析
-- ========================================

-- 查看现有约束
SELECT 
    tc.table_name,
    tc.constraint_name,
    tc.constraint_type,
    cc.column_name,
    cc.column_default,
    cc.is_nullable,
    cc.data_type
FROM information_schema.table_constraints tc
LEFT JOIN information_schema.constraint_column_usage ccu USING (constraint_name)
LEFT JOIN information_schema.columns cc ON cc.table_name = tc.table_name AND cc.column_name = ccu.column_name
WHERE tc.table_schema = 'public'
  AND tc.table_name IN ('trade_orders', 'wallet_address', 'notify_record')
ORDER BY tc.table_name, tc.constraint_type, tc.constraint_name;

-- 检查约束违反情况
SELECT 
    conname as constraint_name,
    contype as constraint_type,
    conrelid::regclass as table_name,
    pg_get_constraintdef(oid) as constraint_definition,
    convalidated as is_validated
FROM pg_constraint
WHERE conrelid IN ('trade_orders'::regclass, 'wallet_address'::regclass, 'notify_record'::regclass)
ORDER BY conrelid, contype;

-- ========================================
-- 2. CHECK 约束优化
-- ========================================

-- 2.1 订单表业务规则约束
-- 当前已有的约束需要进一步优化和完善

-- 金额有效性约束（增强版）
ALTER TABLE trade_orders DROP CONSTRAINT IF EXISTS trade_orders_amount_check;
ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_amount_check 
    CHECK (amount::numeric > 0 AND amount::numeric <= 1000000);

ALTER TABLE trade_orders DROP CONSTRAINT IF EXISTS trade_orders_money_check;
ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_money_check 
    CHECK (money > 0 AND money <= 10000000);

-- USDT汇率约束（合理范围）
ALTER TABLE trade_orders ADD CONSTRAINT IF NOT EXISTS trade_orders_usdt_rate_check 
    CHECK (usdt_rate::numeric > 0 AND usdt_rate::numeric <= 20);

-- 订单ID格式约束
ALTER TABLE trade_orders ADD CONSTRAINT IF NOT EXISTS trade_orders_order_id_format_check
    CHECK (length(order_id) >= 10 AND length(order_id) <= 100 AND order_id ~ '^[a-zA-Z0-9_-]+$');

-- 交易ID格式约束
ALTER TABLE trade_orders ADD CONSTRAINT IF NOT EXISTS trade_orders_trade_id_format_check
    CHECK (length(trade_id) >= 10 AND length(trade_id) <= 100 AND trade_id ~ '^[a-zA-Z0-9_-]+$');

-- 交易哈希格式约束（64位十六进制，可为空）
ALTER TABLE trade_orders ADD CONSTRAINT IF NOT EXISTS trade_orders_trade_hash_format_check
    CHECK (trade_hash = '' OR (length(trade_hash) = 64 AND trade_hash ~ '^[a-fA-F0-9]{64}$'));

-- 链名称约束
ALTER TABLE trade_orders ADD CONSTRAINT IF NOT EXISTS trade_orders_chain_check
    CHECK (chain IN ('TRON', 'POLY', 'OP', 'BSC', 'ARB', 'XLAYER', 'SOL', 'APT'));

-- 地址格式约束（基于链类型）
CREATE OR REPLACE FUNCTION validate_address_format(chain_name TEXT, address_value TEXT) 
RETURNS BOOLEAN AS $$
BEGIN
    CASE chain_name
        WHEN 'TRON' THEN
            RETURN address_value ~ '^T[a-zA-Z0-9]{33}$';
        WHEN 'BSC', 'OP', 'ARB', 'XLAYER' THEN
            RETURN address_value ~ '^0x[a-fA-F0-9]{40}$';
        WHEN 'SOL' THEN
            RETURN length(address_value) BETWEEN 32 AND 44;
        WHEN 'APT' THEN
            RETURN address_value ~ '^0x[a-fA-F0-9]{64}$';
        ELSE
            RETURN TRUE; -- 对未知链类型暂时通过
    END CASE;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

ALTER TABLE trade_orders ADD CONSTRAINT IF NOT EXISTS trade_orders_address_format_check
    CHECK (validate_address_format(chain, address));

-- 通知次数约束
ALTER TABLE trade_orders DROP CONSTRAINT IF EXISTS trade_orders_notify_num_check;
ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_notify_num_check
    CHECK (notify_num >= 0 AND notify_num <= 10);

-- 版本号约束（乐观锁）
ALTER TABLE trade_orders DROP CONSTRAINT IF EXISTS trade_orders_version_check;
ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_version_check
    CHECK (version >= 0);

-- 时间逻辑约束
ALTER TABLE trade_orders ADD CONSTRAINT IF NOT EXISTS trade_orders_time_logic_check
    CHECK (
        expired_at > created_at 
        AND (confirmed_at IS NULL OR confirmed_at >= created_at)
        AND expired_at <= created_at + INTERVAL '24 hours'  -- 最长24小时有效期
    );

-- 2.2 钱包地址表约束
-- 地址格式约束（同订单表）
ALTER TABLE wallet_address ADD CONSTRAINT IF NOT EXISTS wallet_address_format_check
    CHECK (validate_address_format(chain, address));

-- 链名称约束
ALTER TABLE wallet_address ADD CONSTRAINT IF NOT EXISTS wallet_address_chain_check
    CHECK (chain IN ('TRON', 'POLY', 'OP', 'BSC', 'ARB', 'XLAYER', 'SOL', 'APT'));

-- 金额约束
ALTER TABLE wallet_address DROP CONSTRAINT IF EXISTS wallet_address_amounts_check;
ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_amounts_check
    CHECK (in_amount >= 0 AND out_amount >= 0 AND in_amount >= out_amount);

-- 起始块号约束
ALTER TABLE wallet_address DROP CONSTRAINT IF EXISTS wallet_address_start_block_check;
ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_start_block_check
    CHECK (start_block >= 0);

-- 订单计数约束
ALTER TABLE wallet_address DROP CONSTRAINT IF EXISTS wallet_address_count_check;
ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_count_check
    CHECK (count >= 0);

-- 2.3 通知记录表约束
-- 交易哈希格式约束
ALTER TABLE notify_record ADD CONSTRAINT IF NOT EXISTS notify_record_txid_format_check
    CHECK (length(txid) = 64 AND txid ~ '^[a-fA-F0-9]{64}$');

-- ========================================
-- 3. EXCLUDE 约束优化
-- ========================================

-- 3.1 防止同一地址同时有相同金额的等待订单
-- 这是业务层面的重要约束，防止金额冲突
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE trade_orders ADD CONSTRAINT IF NOT EXISTS trade_orders_amount_exclusion
    EXCLUDE USING gist (
        chain WITH =,
        address WITH =,
        amount WITH =,
        tsrange(created_at, expired_at, '[)') WITH &&
    ) WHERE (status = 1);

-- 3.2 防止重复的交易哈希（在同一时间窗口内）
-- 对于非空交易哈希，防止重复
ALTER TABLE trade_orders ADD CONSTRAINT IF NOT EXISTS trade_orders_hash_exclusion
    EXCLUDE (trade_hash WITH =) 
    WHERE (trade_hash IS NOT NULL AND trade_hash != '');

-- 3.3 钱包地址唯一性约束
ALTER TABLE wallet_address ADD CONSTRAINT IF NOT EXISTS wallet_address_unique_exclusion
    EXCLUDE (chain WITH =, address WITH =);

-- ========================================
-- 4. 触发器 vs 应用层验证对比分析
-- ========================================

-- 4.1 数据一致性触发器（推荐使用触发器的场景）

-- 自动更新钱包地址统计信息
CREATE OR REPLACE FUNCTION update_wallet_statistics()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        -- 新订单创建时更新计数
        UPDATE wallet_address 
        SET count = count + 1,
            updated_at = NOW()
        WHERE chain = NEW.chain AND address = NEW.address;
        RETURN NEW;
        
    ELSIF TG_OP = 'UPDATE' THEN
        -- 订单状态变更时更新金额统计
        IF OLD.status != NEW.status AND NEW.status = 2 THEN
            -- 订单成功时更新转入金额
            UPDATE wallet_address 
            SET in_amount = in_amount + NEW.amount::numeric,
                updated_at = NOW()
            WHERE chain = NEW.chain AND address = NEW.address;
        END IF;
        RETURN NEW;
        
    ELSIF TG_OP = 'DELETE' THEN
        -- 订单删除时减少计数
        UPDATE wallet_address 
        SET count = GREATEST(0, count - 1),
            updated_at = NOW()
        WHERE chain = OLD.chain AND address = OLD.address;
        RETURN OLD;
    END IF;
    
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- 创建触发器
DROP TRIGGER IF EXISTS tr_update_wallet_statistics ON trade_orders;
CREATE TRIGGER tr_update_wallet_statistics
    AFTER INSERT OR UPDATE OR DELETE ON trade_orders
    FOR EACH ROW EXECUTE FUNCTION update_wallet_statistics();

-- 4.2 数据验证触发器

-- 订单状态变更验证
CREATE OR REPLACE FUNCTION validate_order_status_change()
RETURNS TRIGGER AS $$
BEGIN
    -- 防止已成功的订单被修改为其他状态
    IF OLD.status = 2 AND NEW.status != 2 THEN
        RAISE EXCEPTION 'Cannot change status of successful order from % to %', OLD.status, NEW.status;
    END IF;
    
    -- 防止过期订单被重新激活
    IF OLD.status = 3 AND NEW.status = 1 THEN
        RAISE EXCEPTION 'Cannot reactivate expired order';
    END IF;
    
    -- 验证状态变更的时间逻辑
    IF NEW.status = 2 AND NEW.confirmed_at IS NULL THEN
        NEW.confirmed_at = NOW();
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS tr_validate_order_status_change ON trade_orders;
CREATE TRIGGER tr_validate_order_status_change
    BEFORE UPDATE ON trade_orders
    FOR EACH ROW EXECUTE FUNCTION validate_order_status_change();

-- 4.3 应用层验证建议

-- 推荐在应用层进行的验证：
/*
1. 业务逻辑验证（如汇率合理性、金额计算）
2. 外部系统集成验证（如区块链地址验证）
3. 用户权限验证
4. 复杂的跨表业务规则验证
*/

-- 推荐在数据库层进行的验证：
/*
1. 数据格式和范围验证
2. 引用完整性
3. 唯一性约束
4. 自动统计更新
5. 审计日志
*/

-- ========================================
-- 5. 外键约束优化
-- ========================================

-- 虽然当前设计使用了逻辑外键（chain+address），我们可以添加一些参照完整性检查

-- 5.1 创建引用完整性验证函数
CREATE OR REPLACE FUNCTION validate_wallet_reference()
RETURNS TRIGGER AS $$
BEGIN
    -- 验证订单引用的钱包地址确实存在且可用
    IF NOT EXISTS (
        SELECT 1 FROM wallet_address 
        WHERE chain = NEW.chain 
          AND address = NEW.address 
          AND status = 1
    ) THEN
        RAISE EXCEPTION 'Referenced wallet address %:% does not exist or is inactive', 
            NEW.chain, NEW.address;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 创建验证触发器
DROP TRIGGER IF EXISTS tr_validate_wallet_reference ON trade_orders;
CREATE TRIGGER tr_validate_wallet_reference
    BEFORE INSERT OR UPDATE ON trade_orders
    FOR EACH ROW EXECUTE FUNCTION validate_wallet_reference();

-- ========================================
-- 6. 约束性能影响分析
-- ========================================

-- 6.1 约束检查性能测试
-- 测试插入性能（带约束 vs 无约束）
EXPLAIN (ANALYZE, BUFFERS) 
INSERT INTO trade_orders (
    order_id, trade_id, chain, address, amount, money, 
    usdt_rate, status, expired_at, version
) VALUES (
    'constraint_test_001',
    'trade_test_001', 
    'TRON',
    'TTestAddress123456789012345678901234',
    '100.50',
    100.50,
    '6.8',
    1,
    NOW() + INTERVAL '1 hour',
    0
);

-- 6.2 约束违反处理
-- 创建约束违反日志表
CREATE TABLE IF NOT EXISTS constraint_violations (
    id BIGSERIAL PRIMARY KEY,
    table_name TEXT NOT NULL,
    constraint_name TEXT NOT NULL,
    violation_data JSONB,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 约束违反处理函数
CREATE OR REPLACE FUNCTION log_constraint_violation()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO constraint_violations (
        table_name, 
        constraint_name, 
        violation_data, 
        error_message
    ) VALUES (
        TG_TABLE_NAME,
        TG_NAME,
        row_to_json(NEW)::jsonb,
        SQLERRM
    );
    
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- ========================================
-- 7. 约束监控和维护
-- ========================================

-- 7.1 约束状态监控视图
CREATE OR REPLACE VIEW v_constraint_status AS
SELECT 
    n.nspname as schema_name,
    c.relname as table_name,
    con.conname as constraint_name,
    CASE con.contype
        WHEN 'c' THEN 'CHECK'
        WHEN 'f' THEN 'FOREIGN KEY'
        WHEN 'p' THEN 'PRIMARY KEY'
        WHEN 'u' THEN 'UNIQUE'
        WHEN 'x' THEN 'EXCLUDE'
        ELSE con.contype::text
    END as constraint_type,
    con.convalidated as is_validated,
    con.condef as definition
FROM pg_constraint con
JOIN pg_class c ON con.conrelid = c.oid
JOIN pg_namespace n ON c.relnamespace = n.oid
WHERE n.nspname = 'public'
  AND c.relname IN ('trade_orders', 'wallet_address', 'notify_record');

-- 7.2 约束性能影响监控
SELECT 
    schemaname,
    tablename,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes,
    CASE 
        WHEN n_tup_ins > 0 THEN 
            round((n_tup_upd + n_tup_del)::numeric / n_tup_ins * 100, 2)
        ELSE 0 
    END as mutation_ratio
FROM pg_stat_user_tables
WHERE tablename IN ('trade_orders', 'wallet_address', 'notify_record');

-- ========================================
-- 8. 约束优化建议总结
-- ========================================

/*
高优先级约束（立即实施）：
1. 金额和汇率范围检查
2. 地址格式验证
3. 时间逻辑约束
4. 状态变更验证触发器

中优先级约束（计划实施）：
1. EXCLUDE约束防止金额冲突
2. 钱包统计自动更新触发器
3. 引用完整性验证

低优先级约束（可选实施）：
1. 约束违反日志记录
2. 复杂业务规则检查
3. 性能监控视图
*/