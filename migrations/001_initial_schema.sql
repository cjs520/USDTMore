-- PostgreSQL 数据库初始化脚本
-- 作者: Database Administrator
-- 创建时间: 2025-09-10

-- 创建数据库扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 创建钱包地址表
CREATE TABLE wallet_address (
    id BIGSERIAL PRIMARY KEY,
    chain VARCHAR(255) NOT NULL,
    start_block BIGINT NOT NULL DEFAULT 0,
    in_amount DECIMAL(18,6) NOT NULL DEFAULT 0,
    out_amount DECIMAL(18,6) NOT NULL DEFAULT 0,
    count BIGINT NOT NULL DEFAULT 0,
    address VARCHAR(255) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1 CHECK (status IN (0, 1)),
    other_notify SMALLINT NOT NULL DEFAULT 1 CHECK (other_notify IN (0, 1)),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 创建钱包地址表注释
COMMENT ON TABLE wallet_address IS '钱包地址管理表';
COMMENT ON COLUMN wallet_address.id IS '主键ID';
COMMENT ON COLUMN wallet_address.chain IS '链路名称 TRON POLY OP BSC';
COMMENT ON COLUMN wallet_address.start_block IS '初始化块，每次查询记录一天之前的blocknum';
COMMENT ON COLUMN wallet_address.in_amount IS '累计转入';
COMMENT ON COLUMN wallet_address.out_amount IS '累计转出';
COMMENT ON COLUMN wallet_address.count IS '历史订单数量';
COMMENT ON COLUMN wallet_address.address IS '钱包地址';
COMMENT ON COLUMN wallet_address.status IS '地址状态 1启动 0禁止';
COMMENT ON COLUMN wallet_address.other_notify IS '其它转账通知 1启动 0禁止';

-- 创建交易订单表
CREATE TABLE trade_orders (
    id BIGSERIAL PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL,
    trade_id VARCHAR(255) NOT NULL,
    trade_hash VARCHAR(64) DEFAULT '',
    usdt_rate VARCHAR(10) NOT NULL,
    amount DECIMAL(12,2) NOT NULL DEFAULT 0,
    money DECIMAL(12,2) NOT NULL DEFAULT 0,
    chain VARCHAR(255) NOT NULL,
    address VARCHAR(34) NOT NULL,
    from_address VARCHAR(34) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 0 CHECK (status IN (0, 1, 2, 3)),
    return_url VARCHAR(500) NOT NULL DEFAULT '',
    notify_url VARCHAR(500) NOT NULL DEFAULT '',
    notify_num INTEGER NOT NULL DEFAULT 0,
    notify_state SMALLINT NOT NULL DEFAULT 0 CHECK (notify_state IN (0, 1)),
    expired_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    confirmed_at TIMESTAMP WITH TIME ZONE NULL
);

-- 创建交易订单表注释
COMMENT ON TABLE trade_orders IS '交易订单表';
COMMENT ON COLUMN trade_orders.id IS '主键ID';
COMMENT ON COLUMN trade_orders.order_id IS '客户订单ID';
COMMENT ON COLUMN trade_orders.trade_id IS '本地订单ID';
COMMENT ON COLUMN trade_orders.trade_hash IS '交易哈希';
COMMENT ON COLUMN trade_orders.usdt_rate IS 'USDT汇率';
COMMENT ON COLUMN trade_orders.amount IS 'USDT交易数额';
COMMENT ON COLUMN trade_orders.money IS '订单交易金额';
COMMENT ON COLUMN trade_orders.chain IS '链路名称 TRON POLY OP BSC';
COMMENT ON COLUMN trade_orders.address IS '收款地址';
COMMENT ON COLUMN trade_orders.from_address IS '支付地址';
COMMENT ON COLUMN trade_orders.status IS '交易状态 0：初始 1：等待支付 2：支付成功 3：订单过期';
COMMENT ON COLUMN trade_orders.return_url IS '同步地址';
COMMENT ON COLUMN trade_orders.notify_url IS '异步地址';
COMMENT ON COLUMN trade_orders.notify_num IS '回调次数';
COMMENT ON COLUMN trade_orders.notify_state IS '回调状态 1：成功 0：失败';
COMMENT ON COLUMN trade_orders.expired_at IS '订单失效时间';
COMMENT ON COLUMN trade_orders.confirmed_at IS '交易确认时间';

-- 创建通知记录表
CREATE TABLE notify_record (
    txid VARCHAR(64) PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 创建通知记录表注释
COMMENT ON TABLE notify_record IS '通知记录表';
COMMENT ON COLUMN notify_record.txid IS '交易哈希';

-- 创建索引
-- 钱包地址表索引
CREATE INDEX idx_wallet_address_chain_address ON wallet_address(chain, address);
CREATE INDEX idx_wallet_address_status ON wallet_address(status);
CREATE INDEX idx_wallet_address_created_at ON wallet_address(created_at);

-- 交易订单表索引
CREATE UNIQUE INDEX idx_trade_orders_order_id ON trade_orders(order_id);
CREATE UNIQUE INDEX idx_trade_orders_trade_id ON trade_orders(trade_id);
CREATE UNIQUE INDEX idx_trade_orders_trade_hash ON trade_orders(trade_hash) WHERE trade_hash != '';
CREATE INDEX idx_trade_orders_status ON trade_orders(status);
CREATE INDEX idx_trade_orders_chain_address ON trade_orders(chain, address);
CREATE INDEX idx_trade_orders_expired_at ON trade_orders(expired_at);
CREATE INDEX idx_trade_orders_created_at ON trade_orders(created_at);
CREATE INDEX idx_trade_orders_notify_state ON trade_orders(notify_state);

-- 通知记录表索引
CREATE INDEX idx_notify_record_created_at ON notify_record(created_at);

-- 创建自动更新时间戳的函数
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 创建触发器
CREATE TRIGGER update_wallet_address_updated_at 
    BEFORE UPDATE ON wallet_address 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_trade_orders_updated_at 
    BEFORE UPDATE ON trade_orders 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_notify_record_updated_at 
    BEFORE UPDATE ON notify_record 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 创建性能监控视图
CREATE VIEW v_trade_order_stats AS
SELECT 
    chain,
    status,
    COUNT(*) as order_count,
    SUM(amount::DECIMAL) as total_amount,
    AVG(amount::DECIMAL) as avg_amount,
    MIN(created_at) as first_order,
    MAX(created_at) as last_order
FROM trade_orders 
GROUP BY chain, status;

COMMENT ON VIEW v_trade_order_stats IS '交易订单统计视图';