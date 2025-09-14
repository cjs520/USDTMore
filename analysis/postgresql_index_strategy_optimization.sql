-- PostgreSQL 索引策略深度优化分析
-- 基于 USDTMore 项目的查询模式和业务特点

-- ========================================
-- 1. 当前索引使用情况分析
-- ========================================

-- 查看当前所有索引
SELECT 
    schemaname,
    tablename,
    indexname,
    indexdef,
    pg_size_pretty(pg_relation_size(indexname::regclass)) as index_size,
    pg_stat_get_numscans(indexrelid) as scans
FROM pg_indexes 
LEFT JOIN pg_stat_user_indexes USING (schemaname, tablename, indexname)
WHERE tablename IN ('trade_orders', 'wallet_address', 'notify_record')
ORDER BY tablename, indexname;

-- 识别未使用的索引
SELECT 
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexname::regclass)) as index_size,
    idx_scan as times_used
FROM pg_stat_user_indexes
WHERE tablename IN ('trade_orders', 'wallet_address', 'notify_record')
  AND idx_scan < 100  -- 使用次数少于100次的索引需要评估
ORDER BY idx_scan;

-- ========================================
-- 2. 复合索引优化分析
-- ========================================

-- 问题：当前某些复合索引的列顺序可能不是最优的
-- 分析基础：高选择性字段应放在前面，常用查询条件优先

-- 优化建议 1: trade_orders 表复合索引重新设计

-- 当前: idx_trade_orders_chain_address ON (chain, address)
-- 分析选择性：
SELECT 
    'chain' as column_name,
    COUNT(DISTINCT chain) as distinct_values,
    COUNT(*) as total_rows,
    (COUNT(DISTINCT chain)::float / COUNT(*))::numeric(5,4) as selectivity
FROM trade_orders
UNION ALL
SELECT 
    'address' as column_name,
    COUNT(DISTINCT address) as distinct_values,
    COUNT(*) as total_rows,
    (COUNT(DISTINCT address)::float / COUNT(*))::numeric(5,4) as selectivity
FROM trade_orders
UNION ALL
SELECT 
    'status' as column_name,
    COUNT(DISTINCT status) as distinct_values,
    COUNT(*) as total_rows,
    (COUNT(DISTINCT status)::float / COUNT(*))::numeric(5,4) as selectivity
FROM trade_orders;

-- 基于业务查询模式的优化索引：
-- 1. 订单状态查询（最常用）
DROP INDEX IF EXISTS idx_trade_orders_status_created;
CREATE INDEX CONCURRENTLY idx_trade_orders_status_created_optimized 
ON trade_orders(status, created_at DESC) 
WHERE status IN (1, 2);  -- 只为活跃状态创建索引

-- 2. 地址金额状态组合查询（用于金额预留检查）
DROP INDEX IF EXISTS idx_trade_orders_address_amount_status;
CREATE INDEX CONCURRENTLY idx_trade_orders_address_amount_status_optimized
ON trade_orders(address, status, amount, created_at) 
WHERE status = 1;  -- 只为等待支付状态创建

-- 3. 链路+地址+状态查询（监控使用）
CREATE INDEX CONCURRENTLY idx_trade_orders_chain_address_status
ON trade_orders(chain, address, status, created_at DESC)
INCLUDE (amount, trade_hash);  -- 包含常用的查询列

-- ========================================
-- 3. 部分索引 (Partial Indexes) 优化
-- ========================================

-- 部分索引可以显著减少索引大小和维护成本
-- 分析各状态的数据分布：
SELECT 
    status,
    COUNT(*) as count,
    COUNT(*) * 100.0 / SUM(COUNT(*)) OVER() as percentage
FROM trade_orders
GROUP BY status
ORDER BY status;

-- 优化建议：为不同状态创建专门的部分索引

-- 1. 等待支付订单索引（查询最频繁）
CREATE INDEX CONCURRENTLY idx_trade_orders_waiting_payments
ON trade_orders(chain, address, amount, created_at DESC)
WHERE status = 1;

-- 2. 成功支付但未回调索引
CREATE INDEX CONCURRENTLY idx_trade_orders_success_no_callback
ON trade_orders(notify_num, updated_at, status)
WHERE status = 2 AND notify_state = 0;

-- 3. 过期订单清理索引
CREATE INDEX CONCURRENTLY idx_trade_orders_expired_cleanup
ON trade_orders(created_at)
WHERE status = 3 AND created_at < NOW() - INTERVAL '30 days';

-- 4. 交易哈希查询索引（排除空值）
DROP INDEX IF EXISTS idx_trade_orders_trade_hash;
CREATE INDEX CONCURRENTLY idx_trade_orders_trade_hash_nonempty
ON trade_orders(trade_hash)
WHERE trade_hash IS NOT NULL AND trade_hash != '';

-- ========================================
-- 4. 索引类型选择优化 (B-tree vs Hash vs GIN)
-- ========================================

-- B-tree 索引（默认，适用于范围查询和排序）
-- 推荐用于：created_at, expired_at, amount, version

-- Hash 索引（适用于等值查询，PostgreSQL 10+支持WAL日志）
-- 推荐用于：order_id, trade_id (唯一值等值查询)
DROP INDEX IF EXISTS idx_trade_orders_order_id;
DROP INDEX IF EXISTS idx_trade_orders_trade_id;
CREATE INDEX CONCURRENTLY idx_trade_orders_order_id_hash 
ON trade_orders USING HASH(order_id);
CREATE INDEX CONCURRENTLY idx_trade_orders_trade_id_hash 
ON trade_orders USING HASH(trade_id);

-- GIN 索引（适用于全文搜索和数组/JSON查询）
-- 当前项目暂无JSON字段，但可为未来扩展预留

-- BRIN 索引（适用于大表的时间序列数据）
-- 为时间字段创建BRIN索引（空间效率高）
CREATE INDEX CONCURRENTLY idx_trade_orders_created_at_brin 
ON trade_orders USING BRIN(created_at);

-- ========================================
-- 5. 索引维护和监控策略
-- ========================================

-- 1. 索引膨胀监控
CREATE OR REPLACE VIEW v_index_bloat AS
SELECT 
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexname::regclass)) as index_size,
    pg_stat_get_numscans(indexrelid) as scans,
    CASE 
        WHEN pg_stat_get_numscans(indexrelid) = 0 THEN 'UNUSED'
        WHEN pg_stat_get_numscans(indexrelid) < 100 THEN 'LOW_USAGE'
        ELSE 'ACTIVE'
    END as usage_status
FROM pg_indexes 
LEFT JOIN pg_stat_user_indexes USING (schemaname, tablename, indexname)
WHERE tablename IN ('trade_orders', 'wallet_address', 'notify_record');

-- 2. 自动索引重建计划
-- 每月重建高使用率的索引以减少膨胀
-- REINDEX INDEX CONCURRENTLY idx_trade_orders_status_created_optimized;

-- 3. 索引使用统计分析
CREATE OR REPLACE VIEW v_index_performance AS
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch,
    CASE 
        WHEN idx_scan > 0 THEN (idx_tup_read::float / idx_scan)
        ELSE 0 
    END as avg_tuples_per_scan,
    pg_size_pretty(pg_relation_size(indexname::regclass)) as size
FROM pg_stat_user_indexes
WHERE tablename IN ('trade_orders', 'wallet_address', 'notify_record')
ORDER BY idx_scan DESC;

-- ========================================
-- 6. 钱包地址表索引优化
-- ========================================

-- 当前索引分析和优化
-- 钱包地址查询模式：主要按chain+status查询可用地址

-- 优化后的钱包地址索引：
DROP INDEX IF EXISTS idx_wallet_address_chain_address;
DROP INDEX IF EXISTS idx_wallet_address_status;
DROP INDEX IF EXISTS idx_wallet_address_chain_status;

-- 1. 主要查询索引（chain+status+address唯一组合）
CREATE UNIQUE INDEX CONCURRENTLY idx_wallet_address_chain_address_unique
ON wallet_address(chain, address);

-- 2. 状态查询索引（包含所需字段）
CREATE INDEX CONCURRENTLY idx_wallet_address_active_status
ON wallet_address(chain, status)
INCLUDE (address, in_amount, out_amount, count)
WHERE status = 1;

-- 3. 统计查询索引
CREATE INDEX CONCURRENTLY idx_wallet_address_amounts
ON wallet_address(chain, in_amount DESC, out_amount DESC)
WHERE status = 1;

-- ========================================
-- 7. 通知记录表索引优化
-- ========================================

-- 通知记录主要用于防重复和清理
-- txid已经是主键，添加时间索引用于数据清理

DROP INDEX IF EXISTS idx_notify_record_created_at;
CREATE INDEX CONCURRENTLY idx_notify_record_created_at_cleanup
ON notify_record(created_at)
WHERE created_at < NOW() - INTERVAL '7 days';  -- 只对旧记录建索引

-- ========================================
-- 8. 索引创建执行计划
-- ========================================

-- 优先级1: 关键业务查询索引（立即执行）
/*
CREATE INDEX CONCURRENTLY idx_trade_orders_status_created_optimized 
ON trade_orders(status, created_at DESC) WHERE status IN (1, 2);

CREATE INDEX CONCURRENTLY idx_trade_orders_waiting_payments
ON trade_orders(chain, address, amount, created_at DESC) WHERE status = 1;

CREATE INDEX CONCURRENTLY idx_trade_orders_order_id_hash 
ON trade_orders USING HASH(order_id);
*/

-- 优先级2: 性能优化索引（非高峰期执行）
/*
CREATE INDEX CONCURRENTLY idx_trade_orders_success_no_callback
ON trade_orders(notify_num, updated_at, status)
WHERE status = 2 AND notify_state = 0;

CREATE INDEX CONCURRENTLY idx_trade_orders_created_at_brin 
ON trade_orders USING BRIN(created_at);
*/

-- 优先级3: 清理和维护索引（维护窗口期执行）
/*
CREATE INDEX CONCURRENTLY idx_trade_orders_expired_cleanup
ON trade_orders(created_at) WHERE status = 3 AND created_at < NOW() - INTERVAL '30 days';

CREATE INDEX CONCURRENTLY idx_notify_record_created_at_cleanup
ON notify_record(created_at) WHERE created_at < NOW() - INTERVAL '7 days';
*/

-- ========================================
-- 9. 索引效果验证查询
-- ========================================

-- 验证索引是否被正确使用
EXPLAIN (ANALYZE, BUFFERS) 
SELECT * FROM trade_orders 
WHERE status = 1 
  AND chain = 'TRON' 
  AND address = 'TExample123'
  AND amount = '100.50'
ORDER BY created_at DESC 
LIMIT 10;

-- 验证复合查询性能
EXPLAIN (ANALYZE, BUFFERS)
SELECT COUNT(*) FROM trade_orders 
WHERE status = 2 
  AND notify_state = 0 
  AND updated_at < NOW() - INTERVAL '1 hour';

-- 验证哈希索引效果
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM trade_orders WHERE order_id = 'TEST_ORDER_123';