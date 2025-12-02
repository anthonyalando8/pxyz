-- ===============================================================================================
-- MIGRATION: BIGINT (cents) → NUMERIC (decimal) with Currency-Specific Precision
-- ===============================================================================================
-- Purpose: Convert all amount fields from integer cents to decimal values
-- Currency Precision: USD=2, USDT=6, BTC=8
-- ===============================================================================================

\c pxyz_fx;

BEGIN;

-- ===============================
-- STEP 0: DROP DEPENDENT VIEWS AND AGGREGATES FIRST
-- ===============================

-- Drop continuous aggregates (will be recreated later)
DROP MATERIALIZED VIEW IF EXISTS receipt_stats_hourly CASCADE;
DROP MATERIALIZED VIEW IF EXISTS receipt_stats_daily CASCADE;

-- Drop dependent views (will be recreated later)
DROP VIEW IF EXISTS v_active_receipts CASCADE;
DROP VIEW IF EXISTS v_recent_receipts CASCADE;
DROP VIEW IF EXISTS real_account_receipts CASCADE;
DROP VIEW IF EXISTS demo_account_receipts CASCADE;

-- Drop materialized views that depend on fx_receipts. amount
DROP MATERIALIZED VIEW IF EXISTS daily_transaction_volume_real CASCADE;
DROP MATERIALIZED VIEW IF EXISTS demo_activity_summary CASCADE;

-- ===============================
-- STEP 1: ADD HELPER FUNCTION FOR CONVERSION
-- ===============================

CREATE OR REPLACE FUNCTION convert_cents_to_decimal(
    amount_cents BIGINT,
    currency_code VARCHAR(8)
) RETURNS NUMERIC AS $$
DECLARE
    decimals INT;
BEGIN
    -- Get decimal places for currency
    SELECT c. decimals INTO decimals 
    FROM currencies c 
    WHERE c.code = currency_code;
    
    -- If currency not found, default to 2 decimals
    IF decimals IS NULL THEN
        decimals := 2;
    END IF;
    
    -- Convert: divide by 10^decimals
    RETURN amount_cents / (10 ^ decimals)::NUMERIC;
END;
$$ LANGUAGE plpgsql;

-- ===============================
-- STEP 2: UPDATE CURRENCIES TABLE
-- ===============================

-- Update existing currencies with correct decimal places
UPDATE currencies SET decimals = 2 WHERE code = 'USD';
UPDATE currencies SET decimals = 6 WHERE code = 'USDT';
UPDATE currencies SET decimals = 8 WHERE code = 'BTC';

-- Update demo_initial_balance (convert from cents to decimal)
UPDATE currencies 
SET demo_initial_balance = convert_cents_to_decimal(demo_initial_balance, code);

-- ===============================
-- STEP 3: MIGRATE FX_RECEIPTS TABLE
-- ===============================

-- Add new NUMERIC columns (temporary)
ALTER TABLE fx_receipts 
  ADD COLUMN amount_new NUMERIC(30, 18),
  ADD COLUMN original_amount_new NUMERIC(30, 18),
  ADD COLUMN transaction_cost_new NUMERIC(30, 18);

-- Convert data from BIGINT to NUMERIC
UPDATE fx_receipts 
SET 
  amount_new = convert_cents_to_decimal(amount, currency),
  original_amount_new = CASE 
    WHEN original_amount IS NOT NULL AND original_currency IS NOT NULL 
    THEN convert_cents_to_decimal(original_amount, original_currency)
    ELSE NULL
  END,
  transaction_cost_new = convert_cents_to_decimal(transaction_cost, currency);

-- Drop old columns and rename new ones
ALTER TABLE fx_receipts 
  DROP COLUMN amount CASCADE,
  DROP COLUMN original_amount CASCADE,
  DROP COLUMN transaction_cost CASCADE;

ALTER TABLE fx_receipts 
  RENAME COLUMN amount_new TO amount;
ALTER TABLE fx_receipts 
  RENAME COLUMN original_amount_new TO original_amount;
ALTER TABLE fx_receipts 
  RENAME COLUMN transaction_cost_new TO transaction_cost;

-- Add constraints back
ALTER TABLE fx_receipts 
  ALTER COLUMN amount SET NOT NULL,
  ADD CONSTRAINT chk_amount_positive CHECK (amount > 0),
  ALTER COLUMN transaction_cost SET NOT NULL,
  ALTER COLUMN transaction_cost SET DEFAULT 0;

-- ===============================
-- STEP 4: MIGRATE LEDGERS TABLE
-- ===============================

-- Add new NUMERIC columns
ALTER TABLE ledgers 
  ADD COLUMN amount_new NUMERIC(30, 18),
  ADD COLUMN balance_after_new NUMERIC(30, 18);

-- Convert data
UPDATE ledgers 
SET 
  amount_new = convert_cents_to_decimal(amount, currency),
  balance_after_new = CASE 
    WHEN balance_after IS NOT NULL 
    THEN convert_cents_to_decimal(balance_after, currency)
    ELSE NULL
  END;

-- Drop old columns and rename
ALTER TABLE ledgers 
  DROP COLUMN amount CASCADE,
  DROP COLUMN balance_after CASCADE;

ALTER TABLE ledgers 
  RENAME COLUMN amount_new TO amount;
ALTER TABLE ledgers 
  RENAME COLUMN balance_after_new TO balance_after;

-- Add constraints back
ALTER TABLE ledgers 
  ALTER COLUMN amount SET NOT NULL,
  ADD CONSTRAINT chk_ledger_amount_positive CHECK (amount > 0);

-- ===============================
-- STEP 5: MIGRATE BALANCES TABLE
-- ===============================

-- Add new NUMERIC columns
ALTER TABLE balances 
  ADD COLUMN balance_new NUMERIC(30, 18),
  ADD COLUMN available_balance_new NUMERIC(30, 18),
  ADD COLUMN pending_debit_new NUMERIC(30, 18),
  ADD COLUMN pending_credit_new NUMERIC(30, 18);

-- Convert data (need to get currency from accounts table)
UPDATE balances b
SET 
  balance_new = convert_cents_to_decimal(b.balance, a.currency),
  available_balance_new = convert_cents_to_decimal(b.available_balance, a.currency),
  pending_debit_new = convert_cents_to_decimal(b.pending_debit, a.currency),
  pending_credit_new = convert_cents_to_decimal(b.pending_credit, a.currency)
FROM accounts a
WHERE b.account_id = a.id;

-- Drop old columns and rename
ALTER TABLE balances 
  DROP COLUMN balance CASCADE,
  DROP COLUMN available_balance CASCADE,
  DROP COLUMN pending_debit CASCADE,
  DROP COLUMN pending_credit CASCADE;

ALTER TABLE balances 
  RENAME COLUMN balance_new TO balance;
ALTER TABLE balances 
  RENAME COLUMN available_balance_new TO available_balance;
ALTER TABLE balances 
  RENAME COLUMN pending_debit_new TO pending_debit;
ALTER TABLE balances 
  RENAME COLUMN pending_credit_new TO pending_credit;

-- Add constraints back
ALTER TABLE balances 
  ALTER COLUMN balance SET NOT NULL,
  ALTER COLUMN balance SET DEFAULT 0,
  ALTER COLUMN available_balance SET NOT NULL,
  ALTER COLUMN available_balance SET DEFAULT 0,
  ALTER COLUMN pending_debit SET NOT NULL,
  ALTER COLUMN pending_debit SET DEFAULT 0,
  ALTER COLUMN pending_credit SET NOT NULL,
  ALTER COLUMN pending_credit SET DEFAULT 0,
  ADD CONSTRAINT chk_balance_non_negative CHECK (balance >= 0),
  ADD CONSTRAINT chk_available_non_negative CHECK (available_balance >= 0);

-- ===============================
-- STEP 6: MIGRATE ACCOUNTS TABLE (overdraft_limit)
-- ===============================

ALTER TABLE accounts 
  ADD COLUMN overdraft_limit_new NUMERIC(30, 18);

UPDATE accounts 
SET overdraft_limit_new = convert_cents_to_decimal(overdraft_limit, currency);

ALTER TABLE accounts 
  DROP COLUMN overdraft_limit CASCADE;

ALTER TABLE accounts 
  RENAME COLUMN overdraft_limit_new TO overdraft_limit;

ALTER TABLE accounts 
  ALTER COLUMN overdraft_limit SET NOT NULL,
  ALTER COLUMN overdraft_limit SET DEFAULT 0;

-- ===============================
-- STEP 7: MIGRATE TRANSACTION_FEES TABLE
-- ===============================

ALTER TABLE transaction_fees 
  ADD COLUMN amount_new NUMERIC(30, 18);

UPDATE transaction_fees 
SET amount_new = convert_cents_to_decimal(amount, currency);

ALTER TABLE transaction_fees 
  DROP COLUMN amount CASCADE;

ALTER TABLE transaction_fees 
  RENAME COLUMN amount_new TO amount;

ALTER TABLE transaction_fees 
  ALTER COLUMN amount SET NOT NULL,
  ADD CONSTRAINT chk_fee_amount_non_negative CHECK (amount >= 0);

-- ===============================
-- STEP 8: MIGRATE TRANSACTION_FEE_RULES TABLE
-- ===============================

ALTER TABLE transaction_fee_rules 
  ADD COLUMN min_fee_new NUMERIC(30, 18),
  ADD COLUMN max_fee_new NUMERIC(30, 18);

-- Convert min/max fees (use source_currency if available, otherwise assume USD)
UPDATE transaction_fee_rules 
SET 
  min_fee_new = CASE 
    WHEN min_fee IS NOT NULL 
    THEN convert_cents_to_decimal(min_fee, COALESCE(source_currency, 'USD'))
    ELSE NULL
  END,
  max_fee_new = CASE 
    WHEN max_fee IS NOT NULL 
    THEN convert_cents_to_decimal(max_fee, COALESCE(source_currency, 'USD'))
    ELSE NULL
  END;

ALTER TABLE transaction_fee_rules 
  DROP COLUMN min_fee CASCADE,
  DROP COLUMN max_fee CASCADE;

ALTER TABLE transaction_fee_rules 
  RENAME COLUMN min_fee_new TO min_fee;
ALTER TABLE transaction_fee_rules 
  RENAME COLUMN max_fee_new TO max_fee;

-- Update tiers JSONB
CREATE OR REPLACE FUNCTION convert_tiers_to_decimal(
    tiers_json JSONB,
    currency_code VARCHAR(8)
) RETURNS JSONB AS $$
DECLARE
    tier JSONB;
    result JSONB := '[]'::JSONB;
    decimals INT;
BEGIN
    IF tiers_json IS NULL THEN
        RETURN NULL;
    END IF;
    
    SELECT c.decimals INTO decimals 
    FROM currencies c 
    WHERE c.code = currency_code;
    
    IF decimals IS NULL THEN
        decimals := 2;
    END IF;
    
    FOR tier IN SELECT * FROM jsonb_array_elements(tiers_json)
    LOOP
        result := result || jsonb_build_object(
            'min_amount', CASE 
                WHEN tier->>'min_amount' IS NOT NULL 
                THEN (tier->>'min_amount')::BIGINT / (10 ^ decimals)::NUMERIC
                ELSE NULL
            END,
            'max_amount', CASE 
                WHEN tier->>'max_amount' IS NOT NULL 
                THEN (tier->>'max_amount')::BIGINT / (10 ^ decimals)::NUMERIC
                ELSE NULL
            END,
            'rate', tier->>'rate',
            'fixed_fee', CASE 
                WHEN tier->>'fixed_fee' IS NOT NULL 
                THEN (tier->>'fixed_fee')::BIGINT / (10 ^ decimals)::NUMERIC
                ELSE NULL
            END
        );
    END LOOP;
    
    RETURN result;
END;
$$ LANGUAGE plpgsql;

UPDATE transaction_fee_rules 
SET tiers = convert_tiers_to_decimal(tiers, COALESCE(source_currency, 'USD'))
WHERE tiers IS NOT NULL;

-- ===============================
-- STEP 9: MIGRATE AGENT_COMMISSIONS TABLE
-- ===============================

ALTER TABLE agent_commissions 
  ADD COLUMN transaction_amount_new NUMERIC(30, 18),
  ADD COLUMN commission_amount_new NUMERIC(30, 18);

UPDATE agent_commissions 
SET 
  transaction_amount_new = convert_cents_to_decimal(transaction_amount, currency),
  commission_amount_new = convert_cents_to_decimal(commission_amount, currency);

ALTER TABLE agent_commissions 
  DROP COLUMN transaction_amount CASCADE,
  DROP COLUMN commission_amount CASCADE;

ALTER TABLE agent_commissions 
  RENAME COLUMN transaction_amount_new TO transaction_amount;
ALTER TABLE agent_commissions 
  RENAME COLUMN commission_amount_new TO commission_amount;

ALTER TABLE agent_commissions 
  ALTER COLUMN transaction_amount SET NOT NULL,
  ALTER COLUMN commission_amount SET NOT NULL;

-- ===============================
-- STEP 10: MIGRATE TRANSACTION_HOLDS TABLE
-- ===============================

ALTER TABLE transaction_holds 
  ADD COLUMN hold_amount_new NUMERIC(30, 18);

UPDATE transaction_holds 
SET hold_amount_new = convert_cents_to_decimal(hold_amount, currency);

ALTER TABLE transaction_holds 
  DROP COLUMN hold_amount CASCADE;

ALTER TABLE transaction_holds 
  RENAME COLUMN hold_amount_new TO hold_amount;

ALTER TABLE transaction_holds 
  ALTER COLUMN hold_amount SET NOT NULL,
  ADD CONSTRAINT chk_hold_amount_positive CHECK (hold_amount > 0);

-- ===============================
-- STEP 11: MIGRATE DAILY_SETTLEMENTS TABLE
-- ===============================

ALTER TABLE daily_settlements 
  ADD COLUMN total_volume_new NUMERIC(30, 18),
  ADD COLUMN total_fees_new NUMERIC(30, 18),
  ADD COLUMN deposit_volume_new NUMERIC(30, 18),
  ADD COLUMN withdrawal_volume_new NUMERIC(30, 18),
  ADD COLUMN conversion_volume_new NUMERIC(30, 18),
  ADD COLUMN trade_volume_new NUMERIC(30, 18);

UPDATE daily_settlements 
SET 
  total_volume_new = convert_cents_to_decimal(total_volume, currency),
  total_fees_new = convert_cents_to_decimal(total_fees, currency),
  deposit_volume_new = convert_cents_to_decimal(deposit_volume, currency),
  withdrawal_volume_new = convert_cents_to_decimal(withdrawal_volume, currency),
  conversion_volume_new = convert_cents_to_decimal(conversion_volume, currency),
  trade_volume_new = convert_cents_to_decimal(trade_volume, currency);

ALTER TABLE daily_settlements 
  DROP COLUMN total_volume CASCADE,
  DROP COLUMN total_fees CASCADE,
  DROP COLUMN deposit_volume CASCADE,
  DROP COLUMN withdrawal_volume CASCADE,
  DROP COLUMN conversion_volume CASCADE,
  DROP COLUMN trade_volume CASCADE;

ALTER TABLE daily_settlements 
  RENAME COLUMN total_volume_new TO total_volume;
ALTER TABLE daily_settlements 
  RENAME COLUMN total_fees_new TO total_fees;
ALTER TABLE daily_settlements 
  RENAME COLUMN deposit_volume_new TO deposit_volume;
ALTER TABLE daily_settlements 
  RENAME COLUMN withdrawal_volume_new TO withdrawal_volume;
ALTER TABLE daily_settlements 
  RENAME COLUMN conversion_volume_new TO conversion_volume;
ALTER TABLE daily_settlements 
  RENAME COLUMN trade_volume_new TO trade_volume;

ALTER TABLE daily_settlements 
  ALTER COLUMN total_volume SET NOT NULL,
  ALTER COLUMN total_volume SET DEFAULT 0,
  ALTER COLUMN total_fees SET NOT NULL,
  ALTER COLUMN total_fees SET DEFAULT 0,
  ALTER COLUMN deposit_volume SET NOT NULL,
  ALTER COLUMN deposit_volume SET DEFAULT 0,
  ALTER COLUMN withdrawal_volume SET NOT NULL,
  ALTER COLUMN withdrawal_volume SET DEFAULT 0,
  ALTER COLUMN conversion_volume SET NOT NULL,
  ALTER COLUMN conversion_volume SET DEFAULT 0,
  ALTER COLUMN trade_volume SET NOT NULL,
  ALTER COLUMN trade_volume SET DEFAULT 0;

-- ===============================
-- STEP 12: RECREATE MATERIALIZED VIEWS
-- ===============================

-- Refresh materialized views with new data types
DROP MATERIALIZED VIEW IF EXISTS system_holdings_real CASCADE;

CREATE MATERIALIZED VIEW system_holdings_real AS
SELECT a.owner_type, a.currency, COUNT(DISTINCT a.id) AS account_count,
       SUM(COALESCE(b.balance, 0)) AS total_balance
FROM accounts a
LEFT JOIN balances b ON a.id = b.account_id
WHERE a.is_active = true AND a.account_type = 'real'
GROUP BY a.owner_type, a.currency;

CREATE UNIQUE INDEX idx_holdings_pk ON system_holdings_real (owner_type, currency);

-- Daily transaction volume (REAL ONLY)
CREATE MATERIALIZED VIEW daily_transaction_volume_real AS
SELECT 
    DATE(r.created_at) AS transaction_date,
    r.currency,
    r.transaction_type,
    COUNT(*) AS transaction_count,
    SUM(r.amount) AS total_volume,
    AVG(r.amount) AS avg_transaction_size
FROM fx_receipts r
WHERE r.status = 'completed' AND r.account_type = 'real'
GROUP BY DATE(r.created_at), r.currency, r.transaction_type;

CREATE UNIQUE INDEX idx_daily_volume_real_pk 
    ON daily_transaction_volume_real (transaction_date, currency, transaction_type);

-- Demo activity summary
CREATE MATERIALIZED VIEW demo_activity_summary AS
SELECT 
    DATE(r.created_at) AS activity_date,
    r.currency,
    r.transaction_type,
    COUNT(*) AS transaction_count,
    COUNT(DISTINCT r.debitor_account_id) AS active_users,
    SUM(r. amount) AS total_volume
FROM fx_receipts r
WHERE r.status = 'completed' AND r.account_type = 'demo'
GROUP BY DATE(r. created_at), r.currency, r.transaction_type;

CREATE UNIQUE INDEX idx_demo_activity_pk 
    ON demo_activity_summary (activity_date, currency, transaction_type);

-- ===============================
-- STEP 13: RECREATE CONTINUOUS AGGREGATES
-- ===============================

CREATE MATERIALIZED VIEW receipt_stats_hourly
WITH (timescaledb.continuous) AS
SELECT 
  time_bucket('1 hour', created_at) AS hour,
  account_type,
  transaction_type,
  status,
  currency,
  COUNT(*) AS count,
  SUM(amount) AS total_amount,
  AVG(amount) AS avg_amount,
  MIN(amount) AS min_amount,
  MAX(amount) AS max_amount,
  PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY amount) AS median_amount,
  PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY amount) AS p95_amount,
  PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY amount) AS p99_amount
FROM fx_receipts
GROUP BY hour, account_type, transaction_type, status, currency
WITH NO DATA;

CREATE INDEX receipt_stats_hourly_idx 
ON receipt_stats_hourly (hour, account_type, transaction_type, status, currency);

SELECT add_continuous_aggregate_policy(
  'receipt_stats_hourly',
  start_offset => INTERVAL '3 hours',
  end_offset   => INTERVAL '30 minutes',
  schedule_interval => INTERVAL '30 minutes'
);

CREATE MATERIALIZED VIEW receipt_stats_daily
WITH (timescaledb.continuous) AS
SELECT 
  time_bucket('1 day', created_at) AS day,
  account_type,
  transaction_type,
  status,
  currency,
  COUNT(*) AS count,
  SUM(amount) AS total_amount,
  AVG(amount) AS avg_amount
FROM fx_receipts
GROUP BY day, account_type, transaction_type, status, currency
WITH NO DATA;

CREATE INDEX receipt_stats_daily_idx 
ON receipt_stats_daily (day, account_type, transaction_type, status, currency);

SELECT add_continuous_aggregate_policy(
  'receipt_stats_daily',
  start_offset => INTERVAL '7 days',
  end_offset   => INTERVAL '1 day',
  schedule_interval => INTERVAL '1 day'
);

-- ===============================
-- STEP 14: RECREATE VIEWS
-- ===============================

CREATE OR REPLACE VIEW v_active_receipts AS
SELECT 
  rl.code,
  fr.*
FROM fx_receipts fr
JOIN receipt_lookup rl ON rl.id = fr.lookup_id
WHERE fr.status IN ('pending', 'processing')
  AND fr.created_at > NOW() - INTERVAL '7 days';

CREATE OR REPLACE VIEW v_recent_receipts AS
SELECT 
  rl.code,
  rl.account_type,
  fr. transaction_type,
  fr.status,
  fr.amount,
  fr.currency,
  fr.created_at
FROM fx_receipts fr
JOIN receipt_lookup rl ON rl.id = fr.lookup_id
WHERE fr.created_at > NOW() - INTERVAL '24 hours'
ORDER BY fr.created_at DESC;

CREATE OR REPLACE VIEW real_account_receipts AS
SELECT 
    rl.code AS receipt_code,
    r.transaction_type,
    ca.account_number AS creditor_account,
    ca. owner_type AS creditor_type,
    ca.owner_id AS creditor_id,
    da.account_number AS debitor_account,
    da.owner_type AS debitor_type,
    da.owner_id AS debitor_id,
    r. amount,
    r.currency,
    r.original_currency,
    r.exchange_rate,
    r.status,
    r.created_at,
    r.completed_at
FROM fx_receipts r
JOIN receipt_lookup rl ON r.lookup_id = rl.id
JOIN accounts ca ON r.creditor_account_id = ca.id
JOIN accounts da ON r.debitor_account_id = da.id
WHERE r.account_type = 'real';

CREATE OR REPLACE VIEW demo_account_receipts AS
SELECT 
    rl.code AS receipt_code,
    r. transaction_type,
    ca. account_number AS creditor_account,
    ca.owner_type AS creditor_type,
    ca.owner_id AS creditor_id,
    da.account_number AS debitor_account,
    da.owner_type AS debitor_type,
    da.owner_id AS debitor_id,
    r.amount,
    r.currency,
    r.original_currency,
    r.exchange_rate,
    r.status,
    r. created_at,
    r. completed_at
FROM fx_receipts r
JOIN receipt_lookup rl ON r.lookup_id = rl.id
JOIN accounts ca ON r.creditor_account_id = ca.id
JOIN accounts da ON r.debitor_account_id = da.id
WHERE r.account_type = 'demo';

-- ===============================
-- STEP 15: VERIFY MIGRATION
-- ===============================

DO $$
DECLARE
    v_count INT;
BEGIN
    -- Check fx_receipts
    SELECT COUNT(*) INTO v_count FROM fx_receipts WHERE amount::TEXT LIKE '%. %';
    RAISE NOTICE 'fx_receipts with decimal amounts: %', v_count;
    
    -- Check ledgers
    SELECT COUNT(*) INTO v_count FROM ledgers WHERE amount::TEXT LIKE '%.%';
    RAISE NOTICE 'ledgers with decimal amounts: %', v_count;
    
    -- Check balances
    SELECT COUNT(*) INTO v_count FROM balances WHERE balance::TEXT LIKE '%.%';
    RAISE NOTICE 'balances with decimal values: %', v_count;
    
    RAISE NOTICE 'Migration verification complete!';
END $$;

COMMIT;

ANALYZE;