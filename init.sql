-- PostgreSQL initialization script for USDTMore
-- This script will be executed when the PostgreSQL container starts for the first time

-- Create database (already created by POSTGRES_DB environment variable)
-- CREATE DATABASE IF NOT EXISTS usdtmore;

-- Set timezone
SET timezone TO 'Asia/Shanghai';

-- Create wallet_address table
CREATE TABLE IF NOT EXISTS wallet_address (
    id BIGSERIAL PRIMARY KEY,
    chain VARCHAR(255) NOT NULL,
    start_block BIGINT NOT NULL DEFAULT 0,
    in_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    out_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    count BIGINT NOT NULL DEFAULT 0,
    address VARCHAR(255) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1,
    other_notify SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create trade_orders table
CREATE TABLE IF NOT EXISTS trade_orders (
    id BIGSERIAL PRIMARY KEY,
    order_id VARCHAR(255) NOT NULL UNIQUE,
    trade_id VARCHAR(255) NOT NULL UNIQUE,
    trade_hash VARCHAR(66) DEFAULT NULL,
    usdt_rate VARCHAR(10) NOT NULL,
    amount DECIMAL(10,2) NOT NULL DEFAULT 0,
    money DECIMAL(10,2) NOT NULL DEFAULT 0,
    chain VARCHAR(255) NOT NULL,
    address VARCHAR(255) NOT NULL,
    from_address VARCHAR(255) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 0,
    return_url VARCHAR(255) NOT NULL DEFAULT '',
    notify_url VARCHAR(255) NOT NULL DEFAULT '',
    notify_num INTEGER NOT NULL DEFAULT 0,
    notify_state SMALLINT NOT NULL DEFAULT 0,
    expired_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    confirmed_at TIMESTAMP WITH TIME ZONE NULL
);

-- Create notify_record table
CREATE TABLE IF NOT EXISTS notify_record (
    id BIGSERIAL PRIMARY KEY,
    txid VARCHAR(255) NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better performance
CREATE INDEX IF NOT EXISTS idx_wallet_address_chain_address ON wallet_address(chain, address);
CREATE INDEX IF NOT EXISTS idx_wallet_address_status ON wallet_address(status);

CREATE INDEX IF NOT EXISTS idx_trade_orders_order_id ON trade_orders(order_id);
CREATE INDEX IF NOT EXISTS idx_trade_orders_trade_id ON trade_orders(trade_id);
CREATE INDEX IF NOT EXISTS idx_trade_orders_status ON trade_orders(status);
CREATE INDEX IF NOT EXISTS idx_trade_orders_chain_address ON trade_orders(chain, address);
CREATE INDEX IF NOT EXISTS idx_trade_orders_notify_state ON trade_orders(notify_state);
CREATE INDEX IF NOT EXISTS idx_trade_orders_expired_at ON trade_orders(expired_at);

CREATE INDEX IF NOT EXISTS idx_notify_record_txid ON notify_record(txid);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers to automatically update updated_at
CREATE TRIGGER update_wallet_address_updated_at BEFORE UPDATE ON wallet_address
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_trade_orders_updated_at BEFORE UPDATE ON trade_orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Grant permissions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO usdtmore;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO usdtmore;
