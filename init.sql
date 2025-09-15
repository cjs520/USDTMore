-- USDTMore 数据库初始化和修复脚本
-- 版本: v1.1.0
-- 日期: 2024-01-15

-- 1. 修复数据库字段长度限制问题
-- 扩展钱包地址字段从varchar(34)到varchar(64)

-- 检查当前字段长度
SELECT 
    column_name, 
    data_type, 
    character_maximum_length 
FROM information_schema.columns 
WHERE table_name IN ('wallet_addresses', 'trade_orders') 
    AND column_name LIKE '%address%';

-- 扩展钱包地址表的地址字段
ALTER TABLE wallet_addresses 
ALTER COLUMN address TYPE VARCHAR(64);

-- 扩展交易订单表的地址字段（如果存在）
ALTER TABLE trade_orders 
ALTER COLUMN address TYPE VARCHAR(64);

-- 扩展发送方地址字段（如果存在）
ALTER TABLE trade_orders 
ALTER COLUMN from_address TYPE VARCHAR(64);

-- 2. 修复数据库唯一约束冲突问题
-- 删除重复的空trade_hash值

-- 查看重复的trade_hash记录
SELECT trade_hash, COUNT(*) as count 
FROM trade_orders 
WHERE trade_hash IS NULL OR trade_hash = '' 
GROUP BY trade_hash 
HAVING COUNT(*) > 1;

-- 删除重复的空trade_hash记录，保留最新的一条
WITH duplicate_records AS (
    SELECT id, 
           ROW_NUMBER() OVER (
               PARTITION BY COALESCE(trade_hash, '') 
               ORDER BY created_at DESC
           ) as rn
    FROM trade_orders 
    WHERE trade_hash IS NULL OR trade_hash = ''
)
DELETE FROM trade_orders 
WHERE id IN (
    SELECT id FROM duplicate_records WHERE rn > 1
);

-- 3. 清理过期订单（超过24小时的待支付订单）
UPDATE trade_orders 
SET status = 3, -- 设置为过期状态
    updated_at = NOW()
WHERE status = 1 -- 待支付状态
    AND created_at < NOW() - INTERVAL '24 hours';

-- 4. 重建唯一约束（如果之前删除了）
-- 注意：只有在trade_hash字段不为空时才创建唯一约束
CREATE UNIQUE INDEX CONCURRENTLY uni_trade_orders_trade_hash 
ON trade_orders (trade_hash) 
WHERE trade_hash IS NOT NULL AND trade_hash != '';

-- 5. 优化数据库性能
-- 重建统计信息
ANALYZE wallet_addresses;
ANALYZE trade_orders;
ANALYZE notify_records;

-- 6. 检查数据完整性
-- 验证地址字段长度是否足够
SELECT 
    MAX(LENGTH(address)) as max_address_length,
    MIN(LENGTH(address)) as min_address_length,
    AVG(LENGTH(address)) as avg_address_length
FROM wallet_addresses 
WHERE address IS NOT NULL;

-- 验证订单数据完整性
SELECT 
    status,
    COUNT(*) as count,
    MIN(created_at) as oldest,
    MAX(created_at) as newest
FROM trade_orders 
GROUP BY status 
ORDER BY status;

-- 7. 创建必要的索引以提高性能
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_status_created 
ON trade_orders (status, created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_trade_orders_address_amount 
ON trade_orders (address, amount) 
WHERE status = 1;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_wallet_addresses_chain_address 
ON wallet_addresses (chain, address);

-- 8. 清理通知记录表（删除30天前的记录）
DELETE FROM notify_records 
WHERE created_at < NOW() - INTERVAL '30 days';

-- 完成提示
SELECT 'Database initialization and fixes completed successfully!' as message;