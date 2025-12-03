-- Reset DB to seeded state (NUMERIC schema) - DESTROYS existing transactional data
-- WARNING: BACKUP your DB first. This truncates data and restarts sequences.
-- Usage (example): psql -U postgres -d pxyz_fx -f reset_to_seed.sql

\c pxyz_fx;

BEGIN;

-- Safety check: prevent accidental run outside the intended DB by checking a known table
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = 'accounts') THEN
    RAISE EXCEPTION 'accounts table not found in current DB; aborting reset';
  END IF;
END$$;

-- Drop materialized views that depend on data so we can truncate and re-create/refresh later
DROP MATERIALIZED VIEW IF EXISTS receipt_stats_hourly CASCADE;
DROP MATERIALIZED VIEW IF EXISTS receipt_stats_daily CASCADE;
DROP MATERIALIZED VIEW IF EXISTS system_holdings_real CASCADE;
DROP MATERIALIZED VIEW IF EXISTS daily_transaction_volume_real CASCADE;
DROP MATERIALIZED VIEW IF EXISTS demo_activity_summary CASCADE;

-- Truncate transactional and dependent tables and restart identities
TRUNCATE
  fx_receipts,
  ledgers,
  transaction_fees,
  agent_commissions,
  transaction_holds,
  daily_settlements,
  balances,
  journals,
  receipt_lookup,
  transaction_fee_rules,
  demo_account_resets,
  demo_account_metadata,
  agent_relationships,
  audit_log,
  accounts
RESTART IDENTITY CASCADE;

-- --------------------------
-- Re-seed master/static data
-- --------------------------

-- 1) Currencies (NUMERIC demo_initial_balance and min_amount)
INSERT INTO currencies (code, name, symbol, decimals, is_fiat, demo_enabled, demo_initial_balance, min_amount, max_amount, created_at, updated_at)
VALUES
  ('USD',  'United States Dollar', '$', 2, TRUE,  TRUE,  10000.00,      0.01,    NULL, now(), now()),
  ('USDT', 'Tether USD',          '₮', 6, FALSE, TRUE,  10000.000000, 0.000001, NULL, now(), now()),
  ('BTC',  'Bitcoin',              '₿', 8, FALSE, TRUE,  0.10000000,   0.00000001, NULL, now(), now())
ON CONFLICT (code) DO UPDATE
  SET name = EXCLUDED.name,
      symbol = EXCLUDED.symbol,
      decimals = EXCLUDED.decimals,
      is_fiat = EXCLUDED.is_fiat,
      demo_enabled = EXCLUDED.demo_enabled,
      demo_initial_balance = EXCLUDED.demo_initial_balance,
      min_amount = EXCLUDED.min_amount,
      max_amount = EXCLUDED.max_amount,
      updated_at = now();

-- 2) Create system accounts — only these system accounts will be funded (liquidity + fees)
INSERT INTO accounts (owner_type, owner_id, currency, purpose, account_type, account_number, created_at, updated_at)
VALUES
  ('system', 'system', 'USD',  'liquidity', 'real', 'SYS-LIQ-USD', now(), now()),
  ('system', 'system', 'USDT', 'liquidity', 'real', 'SYS-LIQ-USDT', now(), now()),
  ('system', 'system', 'BTC',  'liquidity', 'real', 'SYS-LIQ-BTC', now(), now()),
  ('system', 'system', 'USD',  'fees',      'real', 'SYS-FEE-USD', now(), now()),
  ('system', 'system', 'USDT', 'fees',      'real', 'SYS-FEE-USDT', now(), now()),
  ('system', 'system', 'BTC',  'fees',      'real', 'SYS-FEE-BTC', now(), now())
ON CONFLICT DO NOTHING;

-- 3) Fund only liquidity and fees accounts with the requested default balance
--    Default funded balance set to 1,000,000,000.00 (decimal) for funded accounts; others remain 0.
--    Adjust numeric literal if you intended a different unit (e.g. cents vs decimals).
WITH sys_acc AS (
  SELECT id, currency, purpose
  FROM accounts
  WHERE owner_type = 'system'
)
INSERT INTO balances (account_id, balance, available_balance, pending_debit, pending_credit, version, updated_at)
SELECT
  sa.id,
  CASE WHEN sa.purpose IN ('liquidity','fees') THEN 1000000000.00 ELSE 0 END AS balance_value,
  CASE WHEN sa.purpose IN ('liquidity','fees') THEN 1000000000.00 ELSE 0 END AS avail_value,
  0, 0, 0, now()
FROM sys_acc sa
ON CONFLICT (account_id) DO UPDATE
  SET balance = EXCLUDED.balance,
      available_balance = EXCLUDED.available_balance,
      pending_debit = EXCLUDED.pending_debit,
      pending_credit = EXCLUDED.pending_credit,
      version = EXCLUDED.version,
      updated_at = now();

-- Note: Any other system accounts seeded elsewhere will have 0 balance by default.

-- --------------------------
-- Transaction fee rules (simulated / near-realistic)
-- --------------------------
-- The values below use decimal NUMERIC amounts (not atomic cents). They cover deposit, withdrawal,
-- transfer (account-to-account), and conversion fees across USD/USDT/BTC examples. Tiers use decimal amounts.

INSERT INTO transaction_fee_rules (
    rule_name, transaction_type, source_currency, target_currency, account_type, owner_type,
    fee_type, calculation_method, fee_value, min_fee, max_fee, tiers, is_active, priority, created_at, updated_at
) VALUES
  -- Deposit fees
  ('Deposit: USD - platform pct', 'deposit', 'USD', NULL, 'real', NULL, 'platform', 'percentage', 0.001, 1.00, 500.00, NULL, TRUE, 10, now(), now()),
  ('Deposit: USDT - platform pct', 'deposit', 'USDT', NULL, 'real', NULL, 'platform', 'percentage', 0.0005, 0.50, 200.00, NULL, TRUE, 10, now(), now()),
  ('Deposit: BTC - network/fixed', 'deposit', 'BTC', NULL, 'real', NULL, 'network', 'fixed', 0.0, 0.0005, 0.005, NULL, TRUE, 10, now(), now()),

  -- Withdrawal fees
  ('Withdrawal: USD - fixed', 'withdrawal', 'USD', NULL, 'real', NULL, 'platform', 'fixed', 0.0, 2.00, NULL, NULL, TRUE, 10, now(), now()),
  ('Withdrawal: USDT - fixed', 'withdrawal', 'USDT', NULL, 'real', NULL, 'network', 'fixed', 0.0, 1.00, NULL, NULL, TRUE, 10, now(), now()),
  ('Withdrawal: BTC - percentage+min', 'withdrawal', 'BTC', NULL, 'real', NULL, 'network', 'percentage', 0.001, 0.0005, 0.01, NULL, TRUE, 10, now(), now()),

  -- Transfer (P2P) fees
  ('P2P Transfer: same-currency pct', 'transfer', NULL, NULL, 'real', NULL, 'platform', 'percentage', 0.001, 0.10, 100.00, NULL, TRUE, 20, now(), now()),
  ('P2P Transfer: USD fixed', 'transfer', 'USD', NULL, 'real', NULL, 'platform', 'fixed', 0.0, 0.50, NULL, NULL, TRUE, 20, now(), now()),

  -- Conversion fees (example cross-currency)
  ('Conversion: USD->USDT pct', 'conversion', 'USD', 'USDT', 'real', NULL, 'conversion', 'percentage', 0.003, 0.50, 500.00, NULL, TRUE, 30, now(), now()),
  ('Conversion: USDT->BTC pct', 'conversion', 'USDT', 'BTC', 'real', NULL, 'conversion', 'percentage', 0.004, 0.50, 1000.00, NULL, TRUE, 30, now(), now()),
  ('Conversion: BTC->USD pct', 'conversion', 'BTC', 'USD', 'real', NULL, 'conversion', 'percentage', 0.005, 10.00, 10000.00, NULL, TRUE, 30, now(), now()),

  -- Tiered withdrawal example (user-level VIP tiers)
  ('VIP Withdrawal - tiered', 'withdrawal', NULL, NULL, 'real', 'user', 'platform', 'tiered', 0.0, NULL, NULL,
    '[
      {"min_amount": 0.00, "max_amount": 1000.00, "rate": 0.002, "fixed_fee": 1.00},
      {"min_amount": 1000.00, "max_amount": 5000.00, "rate": 0.0015, "fixed_fee": 1.50},
      {"min_amount": 5000.00, "max_amount": null, "rate": 0.001, "fixed_fee": 2.00}
    ]'::JSONB,
    TRUE, 5, now(), now()
  )

 ON CONFLICT DO NOTHING;

-- Add additional simulated/realistic rules as required for your flows:
--  - transfer fees by currency (USD/USDT/BTC)
--  - conversion rules for each currency pair you support
--  - withdrawal rules with network vs platform splits
-- You can add them in the same INSERT block above following the patterns.

-- --------------------------
-- Reset sequences (safe default)
-- --------------------------
DO $$
DECLARE
  t record;
BEGIN
  FOR t IN
    SELECT sequence_schema || '.' || sequence_name AS seqname
    FROM information_schema.sequences
    WHERE sequence_schema NOT IN ('pg_catalog','information_schema')
  LOOP
    BEGIN
      EXECUTE format('SELECT setval(%L, (SELECT COALESCE(MAX(id),0) FROM %I) + 1)', t.seqname, split_part(t.seqname, '.', 2));
    EXCEPTION WHEN OTHERS THEN
      -- ignore sequences we can't set here
      PERFORM 1;
    END;
  END LOOP;
END$$;

-- --------------------------
-- Recreate / refresh materialized views
-- --------------------------

-- Recreate system_holdings_real
DROP MATERIALIZED VIEW IF EXISTS system_holdings_real CASCADE;
CREATE MATERIALIZED VIEW system_holdings_real AS
SELECT a.owner_type, a.currency, COUNT(DISTINCT a.id) AS account_count,
       SUM(COALESCE(b.balance, 0)) AS total_balance
FROM accounts a
LEFT JOIN balances b ON a.id = b.account_id
WHERE a.is_active = true AND a.account_type = 'real'
GROUP BY a.owner_type, a.currency;

-- Recreate daily_transaction_volume_real (empty until fx_receipts present)
DROP MATERIALIZED VIEW IF EXISTS daily_transaction_volume_real CASCADE;
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

-- Recreate demo_activity_summary
DROP MATERIALIZED VIEW IF EXISTS demo_activity_summary CASCADE;
CREATE MATERIALIZED VIEW demo_activity_summary AS
SELECT 
    DATE(r.created_at) AS activity_date,
    r.currency,
    r.transaction_type,
    COUNT(*) AS transaction_count,
    COUNT(DISTINCT r.debitor_account_id) AS active_users,
    SUM(r.amount) AS total_volume
FROM fx_receipts r
WHERE r.status = 'completed' AND r.account_type = 'demo'
GROUP BY DATE(r.created_at), r.currency, r.transaction_type;

-- Update planner stats
ANALYZE;

COMMIT;

-- Optional maintenance: VACUUM FULL (run during maintenance window if desired)
-- VACUUM FULL;

-- Final notice:
-- - This script truncates transactional data and resets sequences.
-- - Only system accounts with purpose 'liquidity' and 'fees' are funded with 1,000,000,000.00.
-- - All other accounts (if any) will have 0 balances after this reset.
-- - The transaction_fee_rules block seeds realistic-looking fee rules for deposit, withdrawal, transfer and conversions.
-- - Add or tune additional fee rules as needed to cover every currency pair and scenario in your environment.