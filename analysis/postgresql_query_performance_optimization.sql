-- PostgreSQL 查询性能优化分析
-- 基于 USDTMore 项目的实际查询模式和 PostgreSQL 特有功能

-- ========================================
-- 1. 执行计划分析 - 关键业务查询
-- ========================================

-- 1.1 订单金额预留检查查询（高频使用）
-- 当前实现分析：tryReserveAmountWithTransaction
EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)
SELECT id FROM trade_orders 
WHERE status = 1 
  AND chain = 'TRON' 
  AND address = 'TExample123'
  AND amount = '100.50'
LIMIT 1;

-- 优化建议：使用 EXISTS 替代 COUNT 查询
-- 原始查询模式优化：
CREATE OR REPLACE FUNCTION check_amount_available(
    p_chain TEXT, 
    p_address TEXT, 
    p_amount NUMERIC(20,6)
) RETURNS BOOLEAN AS $$
BEGIN
    RETURN NOT EXISTS (
        SELECT 1 FROM trade_orders 
        WHERE status = 1 
          AND chain = p_chain 
          AND address = p_address 
          AND amount = p_amount
    );
END;
$$ LANGUAGE plpgsql STABLE;

-- 1.2 订单状态批量查询优化
-- 当前：GetTradeOrderByStatus
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM trade_orders 
WHERE status = 1
ORDER BY created_at DESC;

-- 优化：使用窗口函数进行分页
SELECT 
    *,
    ROW_NUMBER() OVER (ORDER BY created_at DESC) as row_num
FROM trade_orders 
WHERE status = 1
ORDER BY created_at DESC
LIMIT 100 OFFSET 0;

-- 1.3 地址可用性查询优化
-- 当前：GetAvailableAddress
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM wallet_address 
WHERE chain = 'TRON' AND status = 1;

-- 优化：使用物化视图缓存活跃地址
CREATE MATERIALIZED VIEW mv_active_wallet_addresses AS
SELECT 
    chain,
    address,
    in_amount,
    out_amount,
    count,
    updated_at
FROM wallet_address 
WHERE status = 1;

-- 创建刷新索引
CREATE INDEX ON mv_active_wallet_addresses(chain, address);

-- 定期刷新策略（每分钟刷新一次）
-- SELECT cron.schedule('refresh_active_addresses', '* * * * *', 'REFRESH MATERIALIZED VIEW CONCURRENTLY mv_active_wallet_addresses;');

-- ========================================
-- 2. PostgreSQL 特有功能应用
-- ========================================

-- 2.1 使用 UPSERT (ON CONFLICT) 优化订单创建
-- 替代当前的 INSERT + 冲突检测
CREATE OR REPLACE FUNCTION create_or_update_order(
    p_order_id TEXT,
    p_trade_id TEXT,
    p_chain TEXT,
    p_address TEXT,
    p_amount NUMERIC(20,6),
    p_money NUMERIC(20,2),
    p_usdt_rate TEXT,
    p_expired_at TIMESTAMPTZ
) RETURNS BIGINT AS $$
DECLARE
    v_id BIGINT;
BEGIN
    INSERT INTO trade_orders (
        order_id, trade_id, chain, address, amount, money, 
        usdt_rate, status, expired_at, version
    ) VALUES (
        p_order_id, p_trade_id, p_chain, p_address, p_amount, 
        p_money, p_usdt_rate, 1, p_expired_at, 0
    )
    ON CONFLICT (order_id) DO UPDATE SET
        updated_at = NOW(),
        version = trade_orders.version + 1
    RETURNING id INTO v_id;
    
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;

-- 2.2 使用 LATERAL JOIN 优化相关数据查询
-- 获取每个链的最新交易记录
SELECT 
    wa.chain,
    wa.address,
    recent_orders.order_count,
    recent_orders.total_amount
FROM wallet_address wa
CROSS JOIN LATERAL (
    SELECT 
        COUNT(*) as order_count,
        SUM(amount::numeric) as total_amount
    FROM trade_orders t
    WHERE t.chain = wa.chain 
      AND t.address = wa.address
      AND t.created_at > NOW() - INTERVAL '1 day'
) recent_orders
WHERE wa.status = 1;

-- 2.3 使用 JSONB 存储灵活配置（为未来扩展）
-- 为 trade_orders 添加 metadata 字段存储扩展信息
ALTER TABLE trade_orders ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';

-- JSONB 索引优化
CREATE INDEX CONCURRENTLY idx_trade_orders_metadata_gin 
ON trade_orders USING GIN(metadata);

-- 查询示例
SELECT * FROM trade_orders 
WHERE metadata @> '{"priority": "high"}';

-- ========================================
-- 3. 窗口函数应用优化
-- ========================================

-- 3.1 订单统计分析优化
-- 替代传统的 GROUP BY 聚合查询
WITH order_analytics AS (
    SELECT 
        chain,
        address,
        status,
        amount::numeric as amount_num,
        created_at,
        -- 累计统计
        SUM(amount::numeric) OVER (
            PARTITION BY chain, address 
            ORDER BY created_at 
            ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
        ) as running_total,
        -- 排名统计
        RANK() OVER (
            PARTITION BY chain 
            ORDER BY amount::numeric DESC
        ) as amount_rank,
        -- 移动平均
        AVG(amount::numeric) OVER (
            PARTITION BY chain, address 
            ORDER BY created_at 
            ROWS BETWEEN 6 PRECEDING AND CURRENT ROW
        ) as moving_avg_7days
    FROM trade_orders
    WHERE created_at > NOW() - INTERVAL '30 days'
)
SELECT * FROM order_analytics
WHERE amount_rank <= 10;

-- 3.2 地址活跃度分析
SELECT 
    chain,
    address,
    COUNT(*) as total_orders,
    COUNT(*) FILTER (WHERE status = 2) as successful_orders,
    COUNT(*) FILTER (WHERE status = 2)::float / COUNT(*) as success_rate,
    PERCENT_RANK() OVER (
        PARTITION BY chain 
        ORDER BY COUNT(*)
    ) as activity_percentile
FROM trade_orders
WHERE created_at > NOW() - INTERVAL '30 days'
GROUP BY chain, address
HAVING COUNT(*) > 5;

-- ========================================
-- 4. CTE (Common Table Expressions) 优化
-- ========================================

-- 4.1 递归 CTE - 订单依赖关系分析（如果有父子订单关系）
-- WITH RECURSIVE order_hierarchy AS (
--     -- 基础情况：顶级订单
--     SELECT id, order_id, parent_order_id, 0 as level
--     FROM trade_orders 
--     WHERE parent_order_id IS NULL
    
--     UNION ALL
    
--     -- 递归情况：子订单
--     SELECT t.id, t.order_id, t.parent_order_id, oh.level + 1
--     FROM trade_orders t
--     INNER JOIN order_hierarchy oh ON t.parent_order_id = oh.order_id
-- )
-- SELECT * FROM order_hierarchy WHERE level <= 3;

-- 4.2 数据清理和维护 CTE
WITH expired_orders AS (
    SELECT id, order_id, created_at
    FROM trade_orders
    WHERE status = 1 
      AND expired_at < NOW()
      AND updated_at < NOW() - INTERVAL '5 minutes'  -- 防止正在处理的订单
),
update_expired AS (
    UPDATE trade_orders 
    SET status = 3, updated_at = NOW()
    WHERE id IN (SELECT id FROM expired_orders)
    RETURNING id, order_id
)
SELECT 
    COUNT(*) as expired_count,
    array_agg(order_id) as expired_order_ids
FROM update_expired;

-- ========================================
-- 5. 分区表优化（为大数据量准备）
-- ========================================

-- 5.1 基于时间的分区策略
-- 为 trade_orders 创建分区表（适用于大数据量场景）
/*
-- 创建分区主表
CREATE TABLE trade_orders_partitioned (
    LIKE trade_orders INCLUDING ALL
) PARTITION BY RANGE (created_at);

-- 创建月度分区
CREATE TABLE trade_orders_y2024m01 PARTITION OF trade_orders_partitioned
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

CREATE TABLE trade_orders_y2024m02 PARTITION OF trade_orders_partitioned
    FOR VALUES FROM ('2024-02-01') TO ('2024-03-01');

-- 自动分区管理
SELECT partman.create_parent(
    'public.trade_orders_partitioned',
    'created_at',
    'native',
    'monthly',
    p_premake => 2
);
*/

-- 5.2 基于状态的分区策略（热冷数据分离）
/*
CREATE TABLE trade_orders_active PARTITION OF trade_orders_partitioned
    FOR VALUES IN (1, 2);  -- 活跃订单

CREATE TABLE trade_orders_inactive PARTITION OF trade_orders_partitioned
    FOR VALUES IN (0, 3);  -- 非活跃订单
*/

-- ========================================
-- 6. 查询性能监控和调优
-- ========================================

-- 6.1 慢查询监控视图
CREATE OR REPLACE VIEW v_slow_queries AS
SELECT 
    query,
    calls,
    total_exec_time,
    mean_exec_time,
    max_exec_time,
    stddev_exec_time,
    rows as total_rows,
    (total_exec_time / calls) as avg_time_per_call
FROM pg_stat_statements 
WHERE mean_exec_time > 100  -- 超过100ms的查询
ORDER BY mean_exec_time DESC;

-- 6.2 索引效率监控
CREATE OR REPLACE VIEW v_index_efficiency AS
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read,
    idx_tup_fetch,
    CASE 
        WHEN idx_scan > 0 
        THEN round((idx_tup_fetch::numeric / idx_tup_read * 100), 2)
        ELSE 0 
    END as efficiency_percentage
FROM pg_stat_user_indexes
WHERE schemaname = 'public'
  AND tablename IN ('trade_orders', 'wallet_address')
ORDER BY efficiency_percentage DESC;

-- ========================================
-- 7. 应用层查询优化建议
-- ========================================

-- 7.1 批量操作优化
-- 替代逐个插入，使用批量插入
INSERT INTO trade_orders (order_id, trade_id, chain, address, amount, money, usdt_rate, status, expired_at)
SELECT * FROM unnest(
    array['order1', 'order2', 'order3'],  -- order_ids
    array['trade1', 'trade2', 'trade3'],  -- trade_ids
    array['TRON', 'TRON', 'BSC'],         -- chains
    array['addr1', 'addr2', 'addr3'],     -- addresses
    array[100.50, 200.25, 300.75],       -- amounts
    array[100.50, 200.25, 300.75],       -- money
    array['6.8', '6.8', '6.8'],          -- rates
    array[1, 1, 1],                      -- status
    array[NOW() + INTERVAL '1 hour', NOW() + INTERVAL '1 hour', NOW() + INTERVAL '1 hour']  -- expired_at
);

-- 7.2 连接池优化配置
-- 基于当前的连接池配置，添加查询超时设置
/*
-- 在连接字符串中添加：
statement_timeout=30000      -- 30秒查询超时
idle_in_transaction_timeout=60000  -- 60秒事务超时
*/

-- ========================================
-- 8. 性能测试基准查询
-- ========================================

-- 8.1 并发插入性能测试
DO $$
DECLARE
    i INTEGER;
    start_time TIMESTAMP;
    end_time TIMESTAMP;
BEGIN
    start_time := clock_timestamp();
    
    FOR i IN 1..1000 LOOP
        INSERT INTO trade_orders (
            order_id, trade_id, chain, address, amount, 
            money, usdt_rate, status, expired_at
        ) VALUES (
            'perf_test_' || i,
            'trade_' || i,
            'TRON',
            'TTestAddress' || (i % 10),
            (100 + random() * 900)::numeric(20,6),
            (100 + random() * 900)::numeric(20,2),
            '6.8',
            1,
            NOW() + INTERVAL '1 hour'
        );
    END LOOP;
    
    end_time := clock_timestamp();
    RAISE NOTICE 'Inserted 1000 orders in % seconds', 
        EXTRACT(epoch FROM (end_time - start_time));
END $$;

-- 8.2 查询性能基准测试
EXPLAIN (ANALYZE, BUFFERS, TIMING)
SELECT 
    chain,
    COUNT(*) as order_count,
    SUM(amount::numeric) as total_amount,
    AVG(amount::numeric) as avg_amount
FROM trade_orders
WHERE created_at > NOW() - INTERVAL '1 day'
  AND status IN (1, 2)
GROUP BY chain
ORDER BY total_amount DESC;

-- ========================================
-- 9. 查询重写建议
-- ========================================

-- 9.1 避免 N+1 查询问题
-- 原始代码模式（伪代码）：
/*
orders := GetTradeOrderByStatus(1)
for order := range orders {
    address := GetWalletAddress(order.Chain, order.Address)
    // 处理逻辑
}
*/

-- 优化为单次查询：
SELECT 
    t.*,
    w.in_amount,
    w.out_amount,
    w.count as wallet_count
FROM trade_orders t
LEFT JOIN wallet_address w ON t.chain = w.chain AND t.address = w.address
WHERE t.status = 1 AND w.status = 1;

-- 9.2 子查询优化为 JOIN
-- 原始查询：
-- SELECT * FROM trade_orders 
-- WHERE address IN (SELECT address FROM wallet_address WHERE status = 1);

-- 优化为：
SELECT DISTINCT t.* 
FROM trade_orders t
INNER JOIN wallet_address w ON t.address = w.address AND t.chain = w.chain
WHERE w.status = 1;