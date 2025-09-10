-- SQLite到PostgreSQL数据迁移脚本
-- 使用方法:
-- 1. 先导出SQLite数据为CSV
-- 2. 然后使用此脚本导入PostgreSQL

-- 禁用自动提交以提高导入性能
BEGIN;

-- 临时禁用触发器以提高导入性能
SET session_replication_role = replica;

-- 导入钱包地址数据
-- 假设已将SQLite数据导出为CSV文件
COPY wallet_address (id, chain, start_block, in_amount, out_amount, count, address, status, other_notify, created_at, updated_at)
FROM '/path/to/wallet_address.csv'
WITH (FORMAT csv, HEADER true, DELIMITER ',');

-- 导入交易订单数据
COPY trade_orders (id, order_id, trade_id, trade_hash, usdt_rate, amount, money, chain, address, from_address, status, return_url, notify_url, notify_num, notify_state, expired_at, created_at, updated_at, confirmed_at)
FROM '/path/to/trade_orders.csv'
WITH (FORMAT csv, HEADER true, DELIMITER ',');

-- 导入通知记录数据
COPY notify_record (txid, created_at, updated_at)
FROM '/path/to/notify_record.csv'
WITH (FORMAT csv, HEADER true, DELIMITER ',');

-- 重新启用触发器
SET session_replication_role = DEFAULT;

-- 重置序列值
SELECT setval('wallet_address_id_seq', (SELECT MAX(id) FROM wallet_address));
SELECT setval('trade_orders_id_seq', (SELECT MAX(id) FROM trade_orders));

-- 提交事务
COMMIT;

-- 验证数据导入
SELECT 'wallet_address' as table_name, COUNT(*) as record_count FROM wallet_address
UNION ALL
SELECT 'trade_orders' as table_name, COUNT(*) as record_count FROM trade_orders
UNION ALL
SELECT 'notify_record' as table_name, COUNT(*) as record_count FROM notify_record;