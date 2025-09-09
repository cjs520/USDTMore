-- Migration script to fix address field length in existing database
-- Run this on existing PostgreSQL database

-- Update trade_orders table address fields
ALTER TABLE trade_orders ALTER COLUMN address TYPE VARCHAR(255);
ALTER TABLE trade_orders ALTER COLUMN from_address TYPE VARCHAR(255);

-- Verify the changes
\d trade_orders;
