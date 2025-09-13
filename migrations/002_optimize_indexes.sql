-- 数据库索引优化脚本
-- 提升查询性能并防止重复订单

-- ========================================
-- TradeOrders 表优化
-- ========================================

-- 1. 创建复合索引优化常用查询
-- 订单状态查询（状态+创建时间）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_status_created 
ON trade_orders(status, created_at);

-- 地址+金额+状态查询（用于金额预留检查）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_address_amount_status 
ON trade_orders(chain, address, amount, status);

-- 订单过期时间查询
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_expired_at 
ON trade_orders(expired_at) 
WHERE status = 1; -- 只对等待支付的订单建索引

-- 交易哈希查询（用于交易确认）
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_trade_hash 
ON trade_orders(trade_hash) 
WHERE trade_hash != '';

-- 回调状态查询
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_notify 
ON trade_orders(status, notify_num, notify_state)
WHERE status = 2 AND notify_state = 0; -- 成功但回调失败的订单

-- 2. 创建唯一性约束防止重复订单
-- 确保订单ID唯一性（如果不存在）
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'trade_orders_order_id_unique'
    ) THEN
        ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_order_id_unique 
        UNIQUE (order_id);
    END IF;
END $$;

-- 确保交易ID唯一性（如果不存在）
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'trade_orders_trade_id_unique'
    ) THEN
        ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_trade_id_unique 
        UNIQUE (trade_id);
    END IF;
END $$;

-- 确保非空交易哈希唯一性（防止重复处理同一笔交易）
DO $$ 
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'trade_orders_trade_hash_unique'
    ) THEN
        ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_trade_hash_unique 
        UNIQUE (trade_hash);
    END IF;
END $$;

-- ========================================
-- WalletAddress 表优化
-- ========================================

-- 创建链+地址的复合唯一索引
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_wallet_address_chain_address 
ON wallet_address(chain, address);

-- 状态查询索引
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_wallet_address_status 
ON wallet_address(status);

-- 链路查询索引
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_wallet_address_chain_status 
ON wallet_address(chain, status);

-- ========================================
-- NotifyRecord 表优化
-- ========================================

-- Txid 作为主键已经有索引，创建创建时间索引用于清理
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_notify_record_created_at 
ON notify_record(created_at);

-- ========================================
-- 数据完整性检查约束
-- ========================================

-- TradeOrders 表约束
DO $$ 
BEGIN
    -- 订单状态约束
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'trade_orders_status_check'
    ) THEN
        ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_status_check 
        CHECK (status IN (1, 2, 3));
    END IF;
    
    -- 金额必须大于0
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'trade_orders_money_check'
    ) THEN
        ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_money_check 
        CHECK (money > 0);
    END IF;
    
    -- 版本号必须大于等于0
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'trade_orders_version_check'
    ) THEN
        ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_version_check 
        CHECK (version >= 0);
    END IF;
    
    -- 通知次数必须大于等于0
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'trade_orders_notify_num_check'
    ) THEN
        ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_notify_num_check 
        CHECK (notify_num >= 0);
    END IF;
    
    -- 通知状态约束
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'trade_orders_notify_state_check'
    ) THEN
        ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_notify_state_check 
        CHECK (notify_state IN (0, 1));
    END IF;
    
    -- 过期时间必须在创建时间之后
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'trade_orders_expired_check'
    ) THEN
        ALTER TABLE trade_orders ADD CONSTRAINT trade_orders_expired_check 
        CHECK (expired_at > created_at);
    END IF;
END $$;

-- WalletAddress 表约束
DO $$ 
BEGIN
    -- 状态约束
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'wallet_address_status_check'
    ) THEN
        ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_status_check 
        CHECK (status IN (0, 1));
    END IF;
    
    -- 其他通知约束
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'wallet_address_other_notify_check'
    ) THEN
        ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_other_notify_check 
        CHECK (other_notify IN (0, 1));
    END IF;
    
    -- 起始块号必须大于等于0
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'wallet_address_start_block_check'
    ) THEN
        ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_start_block_check 
        CHECK (start_block >= 0);
    END IF;
    
    -- 累计金额必须大于等于0
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'wallet_address_amounts_check'
    ) THEN
        ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_amounts_check 
        CHECK (in_amount >= 0 AND out_amount >= 0);
    END IF;
    
    -- 历史订单数量必须大于等于0
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'wallet_address_count_check'
    ) THEN
        ALTER TABLE wallet_address ADD CONSTRAINT wallet_address_count_check 
        CHECK (count >= 0);
    END IF;
END $$;

-- ========================================
-- 性能优化设置
-- ========================================

-- 设置表的统计信息更新频率
ALTER TABLE trade_orders SET (autovacuum_analyze_scale_factor = 0.02);
ALTER TABLE wallet_address SET (autovacuum_analyze_scale_factor = 0.02);
ALTER TABLE notify_record SET (autovacuum_analyze_scale_factor = 0.02);

-- 设置表的自动清理频率
ALTER TABLE trade_orders SET (autovacuum_vacuum_scale_factor = 0.1);
ALTER TABLE wallet_address SET (autovacuum_vacuum_scale_factor = 0.1);
ALTER TABLE notify_record SET (autovacuum_vacuum_scale_factor = 0.1);

COMMIT;