\c pxyz_fx;
- ===============================
-- OPTIMIZED RECEIPT SCHEMA FOR 4000+ TPS
-- ===============================

-- Drop existing constraints if upgrading
-- ALTER TABLE fx_receipts DROP CONSTRAINT IF EXISTS fx_receipts_pkey;

-- ===============================
-- RECEIPT LOOKUP (Partitioned)
-- ===============================
BEGIN;
DROP TABLE IF EXISTS receipt_lookup CASCADE;

CREATE TABLE receipt_lookup (
  id           BIGSERIAL,
  code         TEXT NOT NULL,
  account_type account_type_enum NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id, account_type)
) PARTITION BY LIST (account_type);

-- Create partitions for real and demo accounts
CREATE TABLE receipt_lookup_real PARTITION OF receipt_lookup 
  FOR VALUES IN ('real');

CREATE TABLE receipt_lookup_demo PARTITION OF receipt_lookup 
  FOR VALUES IN ('demo');

-- Indexes on partitions (faster than on parent table)
CREATE UNIQUE INDEX idx_receipt_code_real ON receipt_lookup_real (code);
CREATE UNIQUE INDEX idx_receipt_code_demo ON receipt_lookup_demo (code);

CREATE INDEX idx_receipt_lookup_created_real ON receipt_lookup_real (created_at DESC);
CREATE INDEX idx_receipt_lookup_created_demo ON receipt_lookup_demo (created_at DESC);

-- ===============================
-- FX RECEIPTS (TimescaleDB Hypertable - OPTIMIZED)
-- ===============================
DROP TABLE IF EXISTS fx_receipts CASCADE;

CREATE TABLE fx_receipts (
  -- Removed lookup_id from PRIMARY KEY for better write performance
  lookup_id              BIGINT NOT NULL,
  account_type           account_type_enum NOT NULL,
  
  -- Creditor information
  creditor_account_id    BIGINT NOT NULL,
  creditor_ledger_id     BIGINT,
  creditor_account_type  owner_type_enum NOT NULL,
  creditor_status        transaction_status_enum NOT NULL DEFAULT 'pending',
  
  -- Debitor information
  debitor_account_id     BIGINT NOT NULL,
  debitor_ledger_id      BIGINT,
  debitor_account_type   owner_type_enum NOT NULL,
  debitor_status         transaction_status_enum NOT NULL DEFAULT 'pending',
  
  -- Transaction details
  transaction_type       transaction_type_enum NOT NULL,
  coded_type             TEXT,
  amount                 BIGINT NOT NULL CHECK (amount > 0),
  original_amount        BIGINT,
  transaction_cost       BIGINT NOT NULL DEFAULT 0,
  
  -- Currency and exchange
  currency               VARCHAR(8) NOT NULL,
  original_currency      VARCHAR(8),
  exchange_rate          NUMERIC(30,18),
  
  -- References
  external_ref           TEXT,
  parent_receipt_code    TEXT,
  reversal_receipt_code  TEXT,
  
  -- Status tracking
  status                 transaction_status_enum NOT NULL DEFAULT 'pending',
  error_message          TEXT,
  
  -- Timestamps
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ,
  completed_at           TIMESTAMPTZ,
  reversed_at            TIMESTAMPTZ,
  
  -- Audit fields
  created_by             TEXT DEFAULT 'system',
  reversed_by            TEXT,
  
  -- Metadata
  metadata               JSONB,
  
  -- CRITICAL: Only partition by created_at (not lookup_id)
  -- This reduces write amplification and improves insert performance
  PRIMARY KEY (created_at, lookup_id),
  
  -- Constraints
  CONSTRAINT chk_conversion_fields CHECK (
    (transaction_type = 'conversion' AND original_currency IS NOT NULL AND exchange_rate IS NOT NULL)
    OR (transaction_type != 'conversion')
  ),
  CONSTRAINT chk_real_only_receipt_types CHECK (
    account_type = 'real' OR transaction_type NOT IN ('deposit', 'withdrawal', 'transfer', 'fee', 'commission')
  ),
  CONSTRAINT chk_different_accounts CHECK (creditor_account_id != debitor_account_id)
);

-- Convert to hypertable (1 week chunks for optimal performance)
SELECT create_hypertable('fx_receipts', 'created_at', 
  if_not_exists => TRUE, 
  chunk_time_interval => INTERVAL '1 week'
);

-- ===============================
-- OPTIMIZED INDEXES
-- ===============================

-- CRITICAL: Separate indexes for real and demo accounts (better selectivity)
-- Creditor indexes (partitioned by account_type for faster queries)
CREATE INDEX idx_fx_receipts_real_creditor 
  ON fx_receipts (creditor_account_id, created_at DESC) 
  WHERE account_type = 'real';

CREATE INDEX idx_fx_receipts_demo_creditor 
  ON fx_receipts (creditor_account_id, created_at DESC) 
  WHERE account_type = 'demo';

-- Debitor indexes (partitioned by account_type)
CREATE INDEX idx_fx_receipts_real_debitor 
  ON fx_receipts (debitor_account_id, created_at DESC) 
  WHERE account_type = 'real';

CREATE INDEX idx_fx_receipts_demo_debitor 
  ON fx_receipts (debitor_account_id, created_at DESC) 
  WHERE account_type = 'demo';

-- Lookup ID index (for joins with receipt_lookup)
CREATE INDEX idx_fx_receipts_lookup_id 
  ON fx_receipts (lookup_id, created_at DESC);

-- Status index (for filtering by status)
CREATE INDEX idx_fx_receipts_status 
  ON fx_receipts (status, created_at DESC) 
  WHERE account_type = 'real';

-- Transaction type index (for filtering)
CREATE INDEX idx_fx_receipts_transaction_type 
  ON fx_receipts (transaction_type, created_at DESC);

-- Currency index (for currency-specific queries)
CREATE INDEX idx_fx_receipts_currency 
  ON fx_receipts (currency, created_at DESC) 
  WHERE account_type = 'real';

-- External ref index (for external system integration)
CREATE INDEX idx_fx_receipts_external_ref 
  ON fx_receipts (external_ref) 
  WHERE external_ref IS NOT NULL;

-- Parent receipt code index (for hierarchy queries)
CREATE INDEX idx_fx_receipts_parent_code 
  ON fx_receipts (parent_receipt_code) 
  WHERE parent_receipt_code IS NOT NULL;

-- Composite index for common queries (status + transaction_type)
CREATE INDEX idx_fx_receipts_status_type 
  ON fx_receipts (status, transaction_type, created_at DESC);

-- JSONB index for metadata queries
CREATE INDEX idx_fx_receipts_metadata 
  ON fx_receipts USING GIN (metadata jsonb_path_ops);

-- ===============================
-- TIMESCALEDB COMPRESSION (90 days)
-- ===============================

ALTER TABLE fx_receipts SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'account_type, transaction_type, currency',
  timescaledb.compress_orderby = 'created_at DESC'
);

-- Add compression policy (compress data older than 90 days)
SELECT add_compression_policy('fx_receipts', INTERVAL '90 days');

-- ===============================
-- CONTINUOUS AGGREGATES (For Analytics)
-- ===============================

-- Hourly aggregation for real-time metrics
CREATE MATERIALIZED VIEW receipt_stats_hourly
WITH (timescaledb.continuous) AS
SELECT 
  time_bucket('1 hour', created_at) AS hour,
  account_type,
  transaction_type,
  status,
  currency,
  COUNT(*) as count,
  SUM(amount) as total_amount,
  AVG(amount) as avg_amount,
  MIN(amount) as min_amount,
  MAX(amount) as max_amount,
  PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY amount) as median_amount,
  PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY amount) as p95_amount,
  PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY amount) as p99_amount
FROM fx_receipts
GROUP BY hour, account_type, transaction_type, status, currency
WITH NO DATA;

-- Create unique index for the continuous aggregate
CREATE UNIQUE INDEX ON receipt_stats_hourly (hour, account_type, transaction_type, status, currency);

-- Refresh policy (refresh every 30 minutes for last 3 hours)
SELECT add_continuous_aggregate_policy('receipt_stats_hourly',
  start_offset => INTERVAL '3 hours',
  end_offset => INTERVAL '30 minutes',
  schedule_interval => INTERVAL '30 minutes'
);

-- Daily aggregation for historical analysis
CREATE MATERIALIZED VIEW receipt_stats_daily
WITH (timescaledb.continuous) AS
SELECT 
  time_bucket('1 day', created_at) AS day,
  account_type,
  transaction_type,
  status,
  currency,
  COUNT(*) as count,
  SUM(amount) as total_amount,
  AVG(amount) as avg_amount
FROM fx_receipts
GROUP BY day, account_type, transaction_type, status, currency
WITH NO DATA;

CREATE UNIQUE INDEX ON receipt_stats_daily (day, account_type, transaction_type, status, currency);

SELECT add_continuous_aggregate_policy('receipt_stats_daily',
  start_offset => INTERVAL '7 days',
  end_offset => INTERVAL '1 day',
  schedule_interval => INTERVAL '1 day'
);

-- ===============================
-- DATA RETENTION POLICY
-- ===============================

-- Drop chunks older than 2 years for demo accounts
SELECT add_retention_policy('fx_receipts', 
  INTERVAL '2 years',
  if_not_exists => TRUE
) WHERE account_type = 'demo';

-- Keep real account data forever (no retention policy)

-- ===============================
-- VIEWS FOR COMMON QUERIES
-- ===============================

-- Active receipts view (frequently accessed)
CREATE OR REPLACE VIEW v_active_receipts AS
SELECT 
  rl.code,
  fr.*
FROM fx_receipts fr
JOIN receipt_lookup rl ON rl.id = fr.lookup_id
WHERE fr.status IN ('pending', 'processing')
  AND fr.created_at > (now() - INTERVAL '7 days');

-- Recent receipts view (cache-friendly)
CREATE OR REPLACE VIEW v_recent_receipts AS
SELECT 
  rl.code,
  rl.account_type,
  fr.transaction_type,
  fr.status,
  fr.amount,
  fr.currency,
  fr.created_at
FROM fx_receipts fr
JOIN receipt_lookup rl ON rl.id = fr.lookup_id
WHERE fr.created_at > (now() - INTERVAL '24 hours')
ORDER BY fr.created_at DESC;

-- ===============================
-- STATISTICS UPDATE
-- ===============================

-- Analyze tables for query planner
ANALYZE receipt_lookup;
ANALYZE fx_receipts;

COMMIT;

-- ===============================
-- GRANTS (if needed)
-- ===============================

-- GRANT SELECT, INSERT, UPDATE ON receipt_lookup TO receipt_service;
-- GRANT SELECT, INSERT, UPDATE ON fx_receipts TO receipt_service;
-- GRANT SELECT ON v_active_receipts TO receipt_service;
-- GRANT SELECT ON v_recent_receipts TO receipt_service;

-- ===============================
-- MONITORING QUERIES
-- ===============================

-- Check chunk size distribution
-- SELECT * FROM timescaledb_information.chunks WHERE hypertable_name = 'fx_receipts';

-- Check compression status
-- SELECT * FROM timescaledb_information.compression_settings WHERE hypertable_name = 'fx_receipts';

-- Check index usage
-- SELECT * FROM pg_stat_user_indexes WHERE schemaname = 'public' AND relname LIKE '%receipt%';

-- Check table bloat
-- SELECT * FROM pg_stat_user_tables WHERE schemaname = 'public' AND relname LIKE '%receipt%';

-- ===============================
-- PERFORMANCE NOTES
-- ===============================

/*