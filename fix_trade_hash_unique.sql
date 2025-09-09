-- Fix trade_hash unique constraint issue
-- Drop the unique constraint on trade_hash field

ALTER TABLE trade_orders DROP CONSTRAINT IF EXISTS trade_orders_trade_hash_key;

-- Update empty strings to NULL
UPDATE trade_orders SET trade_hash = NULL WHERE trade_hash = '';

-- Optionally, add a unique constraint that excludes NULL values (only if needed)
-- CREATE UNIQUE INDEX trade_orders_trade_hash_unique ON trade_orders (trade_hash) WHERE trade_hash IS NOT NULL;
