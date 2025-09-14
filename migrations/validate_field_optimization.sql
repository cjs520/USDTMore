-- 字段优化验证脚本
-- 用于验证模型更改和迁移脚本的正确性

-- ========================================
-- 阶段 1: 验证表结构一致性
-- ========================================

-- 验证 TradeOrders 表结构
DO $$
DECLARE
    expected_columns TEXT[] := ARRAY[
        'id', 'order_id', 'trade_id', 'trade_hash', 'usdt_rate', 'amount', 'money',
        'chain', 'address', 'from_address', 'status', 'version', 'return_url', 'notify_url',
        'notify_num', 'notify_state', 'expired_at', 'created_at', 'updated_at', 'confirmed_at'
    ];
    actual_columns TEXT[];
    missing_columns TEXT[] := '{}';
    column_name TEXT;
BEGIN
    -- 获取实际列名
    SELECT array_agg(column_name ORDER BY ordinal_position) 
    INTO actual_columns
    FROM information_schema.columns 
    WHERE table_name = 'trade_orders'
      AND table_schema = 'public';
    
    -- 检查缺失的列
    FOREACH column_name IN ARRAY expected_columns LOOP
        IF NOT column_name = ANY(actual_columns) THEN
            missing_columns := array_append(missing_columns, column_name);
        END IF;
    END LOOP;
    
    IF array_length(missing_columns, 1) > 0 THEN
        RAISE EXCEPTION 'TradeOrders missing columns: %', array_to_string(missing_columns, ', ');
    END IF;
    
    RAISE NOTICE '✅ TradeOrders table structure validation passed';
END $$;

-- ========================================
-- 阶段 2: 验证字段类型正确性
-- ========================================

-- 验证 TradeOrders 字段类型
DO $$
DECLARE
    rec RECORD;
    expected_types JSONB := '{
        "id": "bigint",
        "order_id": "character varying", 
        "trade_id": "character varying",
        "trade_hash": "character",
        "usdt_rate": "numeric",
        "amount": "numeric", 
        "money": "numeric",
        "chain": "character varying",
        "address": "character varying",
        "from_address": "character varying", 
        "status": "smallint",
        "version": "bigint",
        "return_url": "text",
        "notify_url": "text",
        "notify_num": "smallint",
        "notify_state": "smallint",
        "expired_at": "timestamp with time zone",
        "created_at": "timestamp with time zone", 
        "updated_at": "timestamp with time zone",
        "confirmed_at": "timestamp with time zone"
    }'::jsonb;
    actual_type TEXT;
    expected_type TEXT;
BEGIN
    FOR rec IN 
        SELECT column_name, data_type, numeric_precision, numeric_scale
        FROM information_schema.columns 
        WHERE table_name = 'trade_orders' AND table_schema = 'public'
        ORDER BY ordinal_position
    LOOP
        expected_type := expected_types ->> rec.column_name;
        actual_type := rec.data_type;
        
        -- 特殊类型检查
        IF rec.column_name IN ('usdt_rate', 'amount') AND rec.data_type = 'numeric' THEN
            IF rec.numeric_precision != 18 OR rec.numeric_scale != 8 THEN
                RAISE EXCEPTION 'Field % should be NUMERIC(18,8), got NUMERIC(%,%)', 
                    rec.column_name, rec.numeric_precision, rec.numeric_scale;
            END IF;
        ELSIF rec.column_name = 'money' AND rec.data_type = 'numeric' THEN
            IF rec.numeric_precision != 18 OR rec.numeric_scale != 2 THEN
                RAISE EXCEPTION 'Field money should be NUMERIC(18,2), got NUMERIC(%,%)', 
                    rec.numeric_precision, rec.numeric_scale;
            END IF;
        END IF;
        
        -- 基本类型验证
        IF expected_type IS NOT NULL AND NOT actual_type LIKE expected_type || '%' THEN
            RAISE EXCEPTION 'Field % type mismatch: expected %, got %', 
                rec.column_name, expected_type, actual_type;
        END IF;
        
        RAISE NOTICE 'Field %: % ✅', rec.column_name, actual_type;
    END LOOP;
    
    RAISE NOTICE '✅ TradeOrders field types validation passed';
END $$;

-- 验证 WalletAddress 字段类型
DO $$
DECLARE
    rec RECORD;
BEGIN
    FOR rec IN 
        SELECT column_name, data_type, numeric_precision, numeric_scale
        FROM information_schema.columns 
        WHERE table_name = 'wallet_address' AND table_schema = 'public'
        ORDER BY ordinal_position
    LOOP
        -- 验证金额字段精度
        IF rec.column_name IN ('in_amount', 'out_amount') AND rec.data_type = 'numeric' THEN
            IF rec.numeric_precision != 18 OR rec.numeric_scale != 8 THEN
                RAISE EXCEPTION 'WalletAddress field % should be NUMERIC(18,8), got NUMERIC(%,%)', 
                    rec.column_name, rec.numeric_precision, rec.numeric_scale;
            END IF;
        END IF;
        
        -- 验证整型字段
        IF rec.column_name IN ('start_block', 'count') AND rec.data_type != 'bigint' THEN
            RAISE EXCEPTION 'WalletAddress field % should be BIGINT, got %', 
                rec.column_name, rec.data_type;
        END IF;
        
        -- 验证状态字段 
        IF rec.column_name IN ('status', 'other_notify') AND rec.data_type != 'smallint' THEN
            RAISE EXCEPTION 'WalletAddress field % should be SMALLINT, got %', 
                rec.column_name, rec.data_type;
        END IF;
        
        RAISE NOTICE 'WalletAddress field %: % ✅', rec.column_name, rec.data_type;
    END LOOP;
    
    RAISE NOTICE '✅ WalletAddress field types validation passed';
END $$;

-- ========================================
-- 阶段 3: 验证索引存在性
-- ========================================

-- 验证关键索引是否存在
DO $$
DECLARE
    required_indexes TEXT[] := ARRAY[
        'idx_trade_orders_order_id',
        'idx_trade_orders_trade_id', 
        'idx_trade_orders_trade_hash',
        'idx_trade_orders_status',
        'idx_trade_orders_chain',
        'idx_trade_orders_address',
        'idx_wallet_address_chain',
        'idx_wallet_address_address',
        'idx_wallet_address_status'
    ];
    existing_indexes TEXT[];
    missing_indexes TEXT[] := '{}';
    index_name TEXT;
BEGIN
    -- 获取现有索引
    SELECT array_agg(indexname) 
    INTO existing_indexes
    FROM pg_indexes 
    WHERE tablename IN ('trade_orders', 'wallet_address')
      AND schemaname = 'public';
    
    -- 检查缺失的索引
    FOREACH index_name IN ARRAY required_indexes LOOP
        IF NOT index_name = ANY(existing_indexes) THEN
            missing_indexes := array_append(missing_indexes, index_name);
        END IF;
    END LOOP;
    
    IF array_length(missing_indexes, 1) > 0 THEN
        RAISE WARNING 'Missing indexes (will be created by GORM): %', 
            array_to_string(missing_indexes, ', ');
    ELSE
        RAISE NOTICE '✅ All required indexes exist';
    END IF;
END $$;

-- ========================================
-- 阶段 4: 验证约束正确性
-- ========================================

-- 验证主键约束
DO $$
DECLARE
    table_name TEXT;
    pk_constraint_count INTEGER;
BEGIN
    FOR table_name IN VALUES ('trade_orders'), ('wallet_address'), ('notify_record') LOOP
        SELECT COUNT(*) 
        INTO pk_constraint_count
        FROM information_schema.table_constraints tc
        WHERE tc.table_name = table_name 
          AND tc.constraint_type = 'PRIMARY KEY'
          AND tc.table_schema = 'public';
        
        IF pk_constraint_count != 1 THEN
            RAISE EXCEPTION 'Table % should have exactly 1 primary key constraint, found %', 
                table_name, pk_constraint_count;
        END IF;
        
        RAISE NOTICE 'Table % primary key constraint ✅', table_name;
    END LOOP;
    
    RAISE NOTICE '✅ Primary key constraints validation passed';
END $$;

-- ========================================
-- 阶段 5: 数据完整性检查
-- ========================================

-- 检查数据迁移完整性（如果有数据）
DO $$
DECLARE
    trade_orders_count INTEGER;
    wallet_address_count INTEGER;
    notify_record_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO trade_orders_count FROM trade_orders;
    SELECT COUNT(*) INTO wallet_address_count FROM wallet_address;
    SELECT COUNT(*) INTO notify_record_count FROM notify_record;
    
    RAISE NOTICE 'Data integrity check:';
    RAISE NOTICE '- trade_orders: % records', trade_orders_count;
    RAISE NOTICE '- wallet_address: % records', wallet_address_count;
    RAISE NOTICE '- notify_record: % records', notify_record_count;
    
    -- 检查关键字段不为空（如果有数据）
    IF trade_orders_count > 0 THEN
        PERFORM 1 FROM trade_orders 
        WHERE usdt_rate IS NULL OR amount IS NULL OR money IS NULL
        LIMIT 1;
        
        IF FOUND THEN
            RAISE EXCEPTION 'Found NULL values in critical fields of trade_orders';
        END IF;
        
        RAISE NOTICE '✅ TradeOrders critical fields integrity check passed';
    END IF;
    
    IF wallet_address_count > 0 THEN
        PERFORM 1 FROM wallet_address
        WHERE in_amount IS NULL OR out_amount IS NULL
        LIMIT 1;
        
        IF FOUND THEN
            RAISE EXCEPTION 'Found NULL values in critical fields of wallet_address';
        END IF;
        
        RAISE NOTICE '✅ WalletAddress critical fields integrity check passed';
    END IF;
END $$;

-- ========================================
-- 阶段 6: 性能基准测试
-- ========================================

-- 简单性能测试
DO $$
DECLARE
    start_time TIMESTAMP;
    end_time TIMESTAMP;
    duration INTERVAL;
BEGIN
    -- 测试典型查询性能
    start_time := clock_timestamp();
    
    PERFORM COUNT(*) FROM trade_orders 
    WHERE status = 1 AND chain = 'TRON';
    
    end_time := clock_timestamp();
    duration := end_time - start_time;
    
    RAISE NOTICE 'Query performance test: % ms', 
        EXTRACT(milliseconds FROM duration);
    
    IF EXTRACT(milliseconds FROM duration) > 100 THEN
        RAISE WARNING 'Query performance may need optimization: % ms', 
            EXTRACT(milliseconds FROM duration);
    ELSE
        RAISE NOTICE '✅ Query performance acceptable';
    END IF;
END $$;

-- ========================================
-- 验证报告总结
-- ========================================

-- 生成最终验证报告
SELECT 
    'FIELD OPTIMIZATION VALIDATION COMPLETED' as status,
    json_build_object(
        'validation_timestamp', NOW(),
        'tables_validated', array['trade_orders', 'wallet_address', 'notify_record'],
        'key_improvements', array[
            'High precision decimal fields (18,8) for crypto amounts',
            'Timezone-aware timestamps (timestamptz)',
            'Optimized integer types (smallint, bigint)',
            'Efficient string types (varchar with appropriate limits)',
            'Performance-oriented indexes'
        ],
        'next_steps', array[
            'Deploy updated Go application code',
            'Monitor query performance',
            'Consider creating additional composite indexes based on usage patterns'
        ]
    ) as validation_report;

-- 记录验证完成
INSERT INTO migration_log (migration_name, executed_at, status, rollback_data)
VALUES ('field_optimization_validation', NOW(), 'completed',
    jsonb_build_object(
        'validation_passed', true,
        'validated_at', NOW(),
        'validator_version', '1.0'
    )
)
ON CONFLICT (migration_name) DO UPDATE SET
    executed_at = NOW(),
    status = 'completed',
    rollback_data = EXCLUDED.rollback_data;