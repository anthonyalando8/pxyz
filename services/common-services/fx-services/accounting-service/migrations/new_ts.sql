-- ===============================================================================================
-- PRODUCTION-READY TIMESCALEDB SCHEMA - NUMERIC AMOUNTS
-- ===============================================================================================
-- Changed: All BIGINT amounts → NUMERIC(30, 18) for decimal precision
-- Currency decimals: USD=2, USDT=6, BTC=8
-- ===============================================================================================

\c pxyz_fx;

BEGIN;

-- ===============================
-- EXTENSIONS
-- ===============================
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- ===============================
-- ENUMS (unchanged)
-- ===============================
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'owner_type_enum') THEN
    CREATE TYPE owner_type_enum AS ENUM ('system','user','agent', 'partner', 'admin');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'account_purpose_enum') THEN
    CREATE TYPE account_purpose_enum AS ENUM (
      'liquidity', 'clearing', 'fees', 'wallet', 'escrow',
      'settlement', 'revenue', 'contra', 'commission'
    );
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'account_type_enum') THEN
    CREATE TYPE account_type_enum AS ENUM ('real','demo');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'dr_cr_enum') THEN
    CREATE TYPE dr_cr_enum AS ENUM ('DR','CR');
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transaction_status_enum') THEN
    CREATE TYPE transaction_status_enum AS ENUM (
      'pending', 'processing', 'completed', 'failed',
      'reversed', 'suspended', 'expired'
    );
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transaction_type_enum') THEN
    CREATE TYPE transaction_type_enum AS ENUM (
      'deposit', 'withdrawal', 'conversion', 'trade',
      'transfer', 'fee', 'commission', 'reversal',
      'adjustment', 'demo_funding'
    );
  END IF;
END $$;

ALTER TYPE transaction_type_enum ADD VALUE IF NOT EXISTS 'crypto_deposit';
ALTER TYPE transaction_type_enum ADD VALUE IF NOT EXISTS 'crypto_withdrawal';
ALTER TYPE transaction_type_enum ADD VALUE IF NOT EXISTS 'crypto_conversion';
ALTER TYPE transaction_type_enum ADD VALUE IF NOT EXISTS 'crypto_transfer';

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'fee_type_enum') THEN
    CREATE TYPE fee_type_enum AS ENUM (
      'platform', 'network', 'conversion',
      'withdrawal', 'agent_commission'
    );
  END IF;
END $$;

ALTER TYPE fee_type_enum ADD VALUE IF NOT EXISTS 'network_fee';      -- Blockchain fee (pass-through)
ALTER TYPE fee_type_enum ADD VALUE IF NOT EXISTS 'platform_fee';     -- Platform revenue
ALTER TYPE fee_type_enum ADD VALUE IF NOT EXISTS 'combined_fee';    -- Combined network + platform fee

-- ===============================
-- CURRENCIES
-- ===============================
CREATE TABLE IF NOT EXISTS currencies (
  code        VARCHAR(8) PRIMARY KEY,
  name        TEXT NOT NULL,
  symbol      VARCHAR(10),
  decimals    SMALLINT NOT NULL DEFAULT 2 CHECK (decimals >= 0 AND decimals <= 18),
  is_fiat     BOOLEAN NOT NULL DEFAULT true,
  is_active   BOOLEAN NOT NULL DEFAULT true,
  
  demo_enabled BOOLEAN NOT NULL DEFAULT true,
  demo_initial_balance NUMERIC(30, 18) DEFAULT 0,  -- ✅ Changed from BIGINT
  
  min_amount  NUMERIC(30, 18) NOT NULL DEFAULT 0. 01,  -- ✅ Changed from BIGINT
  max_amount  NUMERIC(30, 18),  -- ✅ Changed from BIGINT
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_currencies_active ON currencies (is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_currencies_demo ON currencies (demo_enabled) WHERE demo_enabled = true;

COMMENT ON TABLE currencies IS 'Master list of supported currencies (fiat and crypto)';
COMMENT ON COLUMN currencies.demo_enabled IS 'Whether this currency is available for demo accounts';
COMMENT ON COLUMN currencies.demo_initial_balance IS 'Starting balance for new demo accounts in this currency (in decimal, e.g., 10000. 00 for USD)';

-- New
UPDATE currencies 
SET 
    is_fiat = true,
    is_crypto = false,
    updated_at = NOW()
WHERE code = 'USD' AND is_fiat = true;

-- migrations/001_add_crypto_to_currencies.sql

-- ============================================================================
-- Step 1: Add crypto-specific columns
-- ============================================================================

ALTER TABLE currencies 
ADD COLUMN IF NOT EXISTS is_crypto BOOLEAN NOT NULL DEFAULT false,
ADD COLUMN IF NOT EXISTS blockchain VARCHAR(50),
ADD COLUMN IF NOT EXISTS contract_address TEXT,
ADD COLUMN IF NOT EXISTS contract_type VARCHAR(20),  -- TRC20, ERC20, BEP20, native
ADD COLUMN IF NOT EXISTS network VARCHAR(50),        -- mainnet, testnet, shasta, etc.
ADD COLUMN IF NOT EXISTS confirmation_blocks SMALLINT DEFAULT 1,
ADD COLUMN IF NOT EXISTS withdrawal_enabled BOOLEAN NOT NULL DEFAULT true,
ADD COLUMN IF NOT EXISTS deposit_enabled BOOLEAN NOT NULL DEFAULT true,
ADD COLUMN IF NOT EXISTS min_withdrawal NUMERIC(30, 18),
ADD COLUMN IF NOT EXISTS max_withdrawal NUMERIC(30, 18),
ADD COLUMN IF NOT EXISTS withdrawal_fee_reserve NUMERIC(30, 18) DEFAULT 0;

-- Add constraints
ALTER TABLE currencies
ADD CONSTRAINT chk_crypto_or_fiat CHECK (
    (is_fiat = true AND is_crypto = false) OR 
    (is_fiat = false AND is_crypto = true) OR
    (is_fiat = false AND is_crypto = false)
);

ALTER TABLE currencies
ADD CONSTRAINT chk_crypto_has_blockchain CHECK (
    (is_crypto = false) OR 
    (is_crypto = true AND blockchain IS NOT NULL)
);

ALTER TABLE currencies
ADD CONSTRAINT chk_native_no_contract CHECK (
    (contract_type != 'native') OR 
    (contract_type = 'native' AND contract_address IS NULL)
);

-- Add indexes
CREATE INDEX IF NOT EXISTS idx_currencies_crypto ON currencies(is_crypto) WHERE is_crypto = true;
CREATE INDEX IF NOT EXISTS idx_currencies_blockchain ON currencies(blockchain) WHERE blockchain IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_currencies_active_crypto ON currencies(is_active, is_crypto) WHERE is_crypto = true;

-- ============================================================================
-- Step 2: Update existing USDT and BTC (assuming they exist)
-- ============================================================================

-- Update USDT to USDT TRC20 Mainnet
UPDATE currencies 
SET 
    is_crypto = true,
    blockchain = 'TRON',
    contract_address = 'TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t',
    contract_type = 'TRC20',
    network = 'mainnet',
    confirmation_blocks = 1,
    withdrawal_enabled = true,
    deposit_enabled = true,
    min_withdrawal = 10.000000,
    max_withdrawal = 100000.000000,
    withdrawal_fee_reserve = 10.000000,  -- Need 10 TRX for fees
    decimals = 6,
    demo_enabled = true,
    demo_initial_balance = 1000.000000,
    min_amount = 1.000000,
    updated_at = NOW()
WHERE code = 'USDT';

-- Update BTC if it exists
UPDATE currencies 
SET 
    is_crypto = true,
    blockchain = 'BITCOIN',
    contract_type = 'native',
    network = 'mainnet',
    confirmation_blocks = 3,
    withdrawal_enabled = true,
    deposit_enabled = true,
    min_withdrawal = 0.001,
    max_withdrawal = 10.0,
    decimals = 8,
    demo_enabled = true,
    demo_initial_balance = 0.1,
    min_amount = 0.00000001,  -- 1 satoshi
    updated_at = NOW()
WHERE code = 'BTC';

-- ============================================================================
-- Step 3: Insert new currencies (if they don't exist)
-- ============================================================================

-- TRX (TRON native token)
INSERT INTO currencies (
    code, 
    name, 
    symbol, 
    decimals, 
    is_fiat, 
    is_crypto, 
    blockchain, 
    contract_type, 
    network,
    confirmation_blocks,
    is_active,
    demo_enabled,
    demo_initial_balance,
    min_amount,
    max_amount,
    min_withdrawal,
    max_withdrawal,
    withdrawal_enabled,
    deposit_enabled
) VALUES (
    'TRX',
    'TRON',
    'TRX',
    6,
    false,
    true,
    'TRON',
    'native',
    'mainnet',
    1,
    true,
    true,
    10000.000000,      -- 10,000 TRX for demo
    0.000001,          -- 1 SUN minimum
    NULL,
    1.000000,          -- Minimum 1 TRX withdrawal
    NULL,
    true,
    true
) ON CONFLICT (code) DO UPDATE SET
    is_crypto = true,
    blockchain = 'TRON',
    contract_type = 'native',
    network = 'mainnet',
    updated_at = NOW();

-- USDT Testnet (Shasta)
INSERT INTO currencies (
    code, 
    name, 
    symbol, 
    decimals, 
    is_fiat, 
    is_crypto, 
    blockchain, 
    contract_address,
    contract_type, 
    network,
    confirmation_blocks,
    is_active,
    demo_enabled,
    demo_initial_balance,
    min_amount,
    max_amount,
    min_withdrawal,
    max_withdrawal,
    withdrawal_enabled,
    deposit_enabled,
    withdrawal_fee_reserve
) VALUES (
    'USDT_TST',
    'Tether USD (Testnet)',
    'USDT',
    6,
    false,
    true,
    'TRON',
    'TG3XXyExBkPp9nzdajDZsozEu4BkaSJozs',  -- Shasta testnet contract
    'TRC20',
    'shasta',
    1,
    true,
    true,
    1000.000000,
    1.000000,
    NULL,
    10.000000,
    100000.000000,
    true,
    true,
    10.000000
) ON CONFLICT (code) DO NOTHING;

-- TRX Testnet
INSERT INTO currencies (
    code, 
    name, 
    symbol, 
    decimals, 
    is_fiat, 
    is_crypto, 
    blockchain, 
    contract_type, 
    network,
    confirmation_blocks,
    is_active,
    demo_enabled,
    demo_initial_balance,
    min_amount,
    min_withdrawal,
    withdrawal_enabled,
    deposit_enabled
) VALUES (
    'TRX_TST',
    'TRON (Testnet)',
    'TRX',
    6,
    false,
    true,
    'TRON',
    'native',
    'shasta',
    1,
    true,
    true,
    10000.000000,
    0.000001,
    1.000000,
    true,
    true
) ON CONFLICT (code) DO NOTHING;

-- BTC (if doesn't exist)
INSERT INTO currencies (
    code, 
    name, 
    symbol, 
    decimals, 
    is_fiat, 
    is_crypto, 
    blockchain, 
    contract_type, 
    network,
    confirmation_blocks,
    is_active,
    demo_enabled,
    demo_initial_balance,
    min_amount,
    max_amount,
    min_withdrawal,
    max_withdrawal,
    withdrawal_enabled,
    deposit_enabled
) VALUES (
    'BTC',
    'Bitcoin',
    '₿',
    8,
    false,
    true,
    'BITCOIN',
    'native',
    'mainnet',
    3,
    true,
    true,
    0.1,
    0.00000001,     -- 1 satoshi
    NULL,
    0.001,
    10.0,
    true,
    true
) ON CONFLICT (code) DO NOTHING;


-- End New

-- ===============================
-- FX RATES (Hypertable)
-- ===============================
CREATE TABLE IF NOT EXISTS fx_rates (
  id             BIGSERIAL NOT NULL,
  base_currency  TEXT NOT NULL REFERENCES currencies(code),
  quote_currency TEXT NOT NULL REFERENCES currencies(code),
  rate           NUMERIC(30,18) NOT NULL CHECK (rate > 0),
  bid_rate       NUMERIC(30,18),
  ask_rate       NUMERIC(30,18),
  spread         NUMERIC(10,6),
  source         TEXT,
  valid_from     TIMESTAMPTZ NOT NULL DEFAULT now(),
  valid_to       TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id, valid_from),
  CONSTRAINT chk_currency_length CHECK (
    length(base_currency) <= 8 AND length(quote_currency) <= 8
  )
);

SELECT create_hypertable('fx_rates', 'valid_from', if_not_exists => TRUE, chunk_time_interval => INTERVAL '7 days');

CREATE INDEX IF NOT EXISTS idx_fx_rates_current 
  ON fx_rates (base_currency, quote_currency, valid_from DESC) WHERE valid_to IS NULL;

ALTER TABLE fx_rates SET (timescaledb.compress, timescaledb.compress_orderby = 'valid_from DESC');
SELECT add_compression_policy('fx_rates', INTERVAL '90 days');

-- ===============================
-- AGENT RELATIONSHIPS CACHE
-- ===============================
CREATE TABLE IF NOT EXISTS agent_relationships (
  user_external_id     TEXT NOT NULL,
  agent_external_id    TEXT NOT NULL,
  service TEXT NOT NULL, -- mpesa, bank, etc
  commission_rate      NUMERIC(5,4) NOT NULL,
  relationship_type    TEXT NOT NULL DEFAULT 'direct',
  is_active            BOOLEAN NOT NULL DEFAULT true,
  synced_from_auth_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  PRIMARY KEY (user_external_id, agent_external_id),
  CONSTRAINT chk_different_ids CHECK (user_external_id != agent_external_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_rel_user ON agent_relationships (user_external_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_agent_rel_agent ON agent_relationships (agent_external_id) WHERE is_active = true;

COMMENT ON TABLE agent_relationships IS 
  'Cached from auth service.  Sync every 5 minutes.  Prevents cross-service calls during transactions.';

-- ===============================
-- DEMO ACCOUNT METADATA
-- ===============================
CREATE TABLE IF NOT EXISTS demo_account_metadata (
  user_external_id   TEXT PRIMARY KEY,
  demo_enabled       BOOLEAN NOT NULL DEFAULT true,
  demo_reset_count   INT NOT NULL DEFAULT 0,
  last_demo_reset    TIMESTAMPTZ,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE demo_account_metadata IS 'Demo-specific data.  User master data in auth service. ';

-- ===============================
-- ACCOUNTS
-- ===============================
CREATE TABLE IF NOT EXISTS accounts (
  id             BIGSERIAL PRIMARY KEY,
  account_number TEXT NOT NULL UNIQUE,
  owner_type     owner_type_enum NOT NULL,
  owner_id       TEXT NOT NULL,
  currency       VARCHAR(8) NOT NULL REFERENCES currencies(code),
  purpose        account_purpose_enum NOT NULL,
  account_type   account_type_enum NOT NULL DEFAULT 'real',
  is_active      BOOLEAN NOT NULL DEFAULT true,
  is_locked      BOOLEAN NOT NULL DEFAULT false,
  overdraft_limit NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  
  parent_agent_external_id TEXT,
  commission_rate          NUMERIC(5,4),
  
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT uq_account UNIQUE (owner_type, owner_id, currency, purpose, account_type),
  CONSTRAINT chk_system_real CHECK (owner_type != 'system' OR account_type = 'real'),
  CONSTRAINT chk_demo_no_overdraft CHECK (account_type = 'real' OR overdraft_limit = 0),
  CONSTRAINT chk_agent_commission 
    CHECK (
      (owner_type != 'agent' OR commission_rate IS NOT NULL) AND
      (account_type = 'real' OR commission_rate IS NULL)
    ),
  CONSTRAINT chk_demo_purpose CHECK (account_type = 'real' OR purpose = 'wallet')
);

CREATE INDEX IF NOT EXISTS idx_accounts_real ON accounts (owner_type, owner_id) WHERE account_type = 'real';
CREATE INDEX IF NOT EXISTS idx_accounts_demo ON accounts (owner_type, owner_id) WHERE account_type = 'demo';
CREATE INDEX IF NOT EXISTS idx_accounts_agent_parent ON accounts (parent_agent_external_id) WHERE parent_agent_external_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_accounts_real_currency 
  ON accounts (currency) 
  WHERE is_active = true AND account_type = 'real';
CREATE INDEX IF NOT EXISTS idx_accounts_demo_currency 
  ON accounts (currency) 
  WHERE is_active = true AND account_type = 'demo';

COMMENT ON COLUMN accounts.owner_id IS 'External ID from auth service (UUID/string)';
COMMENT ON COLUMN accounts.parent_agent_external_id IS 'Agent external ID from auth service';

-- ===============================
-- BALANCES
-- ===============================
CREATE TABLE IF NOT EXISTS balances (
  account_id        BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  balance           NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  available_balance NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  pending_debit     NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  pending_credit    NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  last_ledger_id    BIGINT,
  version           BIGINT NOT NULL DEFAULT 0,
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT chk_balance_non_negative CHECK (balance >= 0),
  CONSTRAINT chk_available_non_negative CHECK (available_balance >= 0)
);

CREATE INDEX IF NOT EXISTS idx_balances_updated ON balances (updated_at DESC);

-- ===============================
-- JOURNALS
-- ===============================
CREATE TABLE IF NOT EXISTS journals (
  id                      BIGSERIAL PRIMARY KEY,
  idempotency_key         TEXT UNIQUE,
  transaction_type        transaction_type_enum NOT NULL,
  account_type            account_type_enum NOT NULL,
  external_ref            TEXT,
  description             TEXT,
  created_by_external_id  TEXT,
  created_by_type         owner_type_enum,
  ip_address              INET,
  user_agent              TEXT,
  created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT chk_real_only CHECK (
    account_type = 'real' OR transaction_type NOT IN ('deposit', 'withdrawal', 'transfer', 'fee', 'commission')
  )
);

CREATE INDEX IF NOT EXISTS idx_journals_real ON journals (created_at DESC) WHERE account_type = 'real';
CREATE INDEX IF NOT EXISTS idx_journals_demo ON journals (created_at DESC) WHERE account_type = 'demo';
CREATE INDEX IF NOT EXISTS idx_journals_type_real 
  ON journals (transaction_type, created_at DESC) 
  WHERE account_type = 'real';
CREATE INDEX IF NOT EXISTS idx_journals_type_demo 
  ON journals (transaction_type, created_at DESC) 
  WHERE account_type = 'demo';

COMMENT ON COLUMN journals.created_by_external_id IS 'External ID from auth service';

-- ===============================
-- RECEIPT LOOKUP
-- ===============================
DROP TABLE IF EXISTS receipt_lookup CASCADE;

CREATE TABLE receipt_lookup (
  id           BIGSERIAL PRIMARY KEY,
  code         TEXT NOT NULL UNIQUE,
  account_type account_type_enum NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_receipt_lookup_type ON receipt_lookup(account_type);
CREATE INDEX idx_receipt_lookup_created ON receipt_lookup(created_at DESC);

-- ===============================
-- FX RECEIPTS (TimescaleDB Hypertable - NUMERIC AMOUNTS)
-- ===============================
DROP TABLE IF EXISTS fx_receipts CASCADE;

CREATE TABLE fx_receipts (
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
  amount                 NUMERIC(30, 18) NOT NULL CHECK (amount > 0),  -- ✅ Changed from BIGINT
  original_amount        NUMERIC(30, 18),  -- ✅ Changed from BIGINT
  transaction_cost       NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  
  -- Currency and exchange
  currency               TEXT NOT NULL,
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
  
  PRIMARY KEY (created_at, lookup_id),
  
  -- Constraints
  CONSTRAINT chk_currency_codes CHECK (
    length(currency) <= 8 AND 
    (original_currency IS NULL OR length(original_currency) <= 8)
  ),
  CONSTRAINT chk_conversion_fields CHECK (
    (transaction_type = 'conversion' AND original_currency IS NOT NULL AND exchange_rate IS NOT NULL)
    OR (transaction_type != 'conversion')
  ),
  CONSTRAINT chk_real_only_receipt_types CHECK (
    account_type = 'real' OR transaction_type NOT IN ('deposit', 'withdrawal', 'transfer', 'fee', 'commission')
  ),
  CONSTRAINT chk_different_accounts CHECK (creditor_account_id != debitor_account_id)
);

SELECT create_hypertable('fx_receipts', 'created_at', 
  if_not_exists => TRUE, 
  chunk_time_interval => INTERVAL '1 week'
);

-- Indexes (unchanged structure)
CREATE INDEX idx_fx_receipts_real_creditor 
  ON fx_receipts (creditor_account_id, created_at DESC) 
  WHERE account_type = 'real';
CREATE INDEX idx_fx_receipts_demo_creditor 
  ON fx_receipts (creditor_account_id, created_at DESC) 
  WHERE account_type = 'demo';
CREATE INDEX idx_fx_receipts_real_debitor 
  ON fx_receipts (debitor_account_id, created_at DESC) 
  WHERE account_type = 'real';
CREATE INDEX idx_fx_receipts_demo_debitor 
  ON fx_receipts (debitor_account_id, created_at DESC) 
  WHERE account_type = 'demo';
CREATE INDEX idx_fx_receipts_lookup_id 
  ON fx_receipts (lookup_id, created_at DESC);
CREATE INDEX idx_fx_receipts_status 
  ON fx_receipts (status, created_at DESC) 
  WHERE account_type = 'real';
CREATE INDEX idx_fx_receipts_transaction_type 
  ON fx_receipts (transaction_type, created_at DESC);
CREATE INDEX idx_fx_receipts_currency 
  ON fx_receipts (currency, created_at DESC) 
  WHERE account_type = 'real';
CREATE INDEX idx_fx_receipts_external_ref 
  ON fx_receipts (external_ref) 
  WHERE external_ref IS NOT NULL;
CREATE INDEX idx_fx_receipts_parent_code 
  ON fx_receipts (parent_receipt_code) 
  WHERE parent_receipt_code IS NOT NULL;
CREATE INDEX idx_fx_receipts_status_type 
  ON fx_receipts (status, transaction_type, created_at DESC);
CREATE INDEX idx_fx_receipts_metadata 
  ON fx_receipts USING GIN (metadata jsonb_path_ops);

ALTER TABLE fx_receipts SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'lookup_id, account_type, transaction_type, currency',
  timescaledb.compress_orderby = 'created_at DESC'
);

SELECT add_compression_policy('fx_receipts', INTERVAL '90 days');

-- ===============================
-- CONTINUOUS AGGREGATES
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

SELECT add_retention_policy(
  'fx_receipts', 
  INTERVAL '2 years',
  if_not_exists => TRUE
);

-- ===============================
-- VIEWS
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
JOIN receipt_lookup rl ON rl. id = fr.lookup_id
WHERE fr.created_at > NOW() - INTERVAL '24 hours'
ORDER BY fr.created_at DESC;

-- Delete triggers (unchanged)
CREATE OR REPLACE FUNCTION delete_receipts_on_lookup_delete()
RETURNS TRIGGER AS $$
BEGIN
  PERFORM set_config('receipt_service.deleting_lookup', 'true', true);
  DELETE FROM fx_receipts WHERE lookup_id = OLD.id;
  RAISE NOTICE 'Deleted receipts for lookup_id: %', OLD.id;
  PERFORM set_config('receipt_service.deleting_lookup', '', true);
  RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_delete_receipts_on_lookup_delete ON receipt_lookup;
CREATE TRIGGER trigger_delete_receipts_on_lookup_delete
  BEFORE DELETE ON receipt_lookup
  FOR EACH ROW
  EXECUTE FUNCTION delete_receipts_on_lookup_delete();

CREATE OR REPLACE FUNCTION delete_lookup_if_no_receipts()
RETURNS TRIGGER AS $$
DECLARE
  receipt_count INT;
  is_deleting_lookup TEXT;
BEGIN
  is_deleting_lookup := current_setting('receipt_service.deleting_lookup', true);
  IF is_deleting_lookup = 'true' THEN
    RETURN OLD;
  END IF;
  
  SELECT COUNT(*) INTO receipt_count FROM fx_receipts WHERE lookup_id = OLD.lookup_id;
  
  IF receipt_count = 0 THEN
    DELETE FROM receipt_lookup WHERE id = OLD.lookup_id;
    RAISE NOTICE 'Deleted orphaned lookup_id: %', OLD.lookup_id;
  END IF;
  
  RETURN OLD;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_delete_lookup_if_no_receipts ON fx_receipts;
CREATE TRIGGER trigger_delete_lookup_if_no_receipts
  AFTER DELETE ON fx_receipts
  FOR EACH ROW
  EXECUTE FUNCTION delete_lookup_if_no_receipts();

CREATE OR REPLACE FUNCTION prevent_orphaned_receipts()
RETURNS TRIGGER AS $$
DECLARE
  lookup_exists BOOLEAN;
BEGIN
  SELECT EXISTS(SELECT 1 FROM receipt_lookup WHERE id = NEW.lookup_id) INTO lookup_exists;
  IF NOT lookup_exists THEN
    RAISE EXCEPTION 'Cannot insert receipt: lookup_id % does not exist', NEW.lookup_id;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_prevent_orphaned_receipts ON fx_receipts;
CREATE TRIGGER trigger_prevent_orphaned_receipts
  BEFORE INSERT ON fx_receipts
  FOR EACH ROW
  EXECUTE FUNCTION prevent_orphaned_receipts();

-- ===============================
-- LEDGERS (NUMERIC AMOUNTS)
-- ===============================
CREATE TABLE IF NOT EXISTS ledgers (
  id            BIGSERIAL NOT NULL,
  journal_id    BIGINT NOT NULL REFERENCES journals(id),
  account_id    BIGINT NOT NULL REFERENCES accounts(id),
  account_type  account_type_enum NOT NULL,
  amount        NUMERIC(30, 18) NOT NULL CHECK (amount > 0),  -- ✅ Changed from BIGINT
  dr_cr         dr_cr_enum NOT NULL,
  currency      VARCHAR(8) NOT NULL REFERENCES currencies(code),
  receipt_code  TEXT REFERENCES receipt_lookup(code),
  balance_after NUMERIC(30, 18),  -- ✅ Changed from BIGINT
  description   TEXT,
  metadata      JSONB,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  PRIMARY KEY (id, account_id, created_at)
);

SELECT create_hypertable('ledgers', 'created_at', if_not_exists => TRUE, chunk_time_interval => INTERVAL '1 week', number_partitions => 128, partitioning_column => 'account_id');

CREATE INDEX IF NOT EXISTS idx_ledgers_real ON ledgers (account_id, created_at DESC) WHERE account_type = 'real';
CREATE INDEX IF NOT EXISTS idx_ledgers_demo ON ledgers (account_id, created_at DESC) WHERE account_type = 'demo';
CREATE INDEX IF NOT EXISTS idx_ledgers_journal ON ledgers (journal_id);
CREATE INDEX IF NOT EXISTS idx_ledgers_receipt ON ledgers (receipt_code) WHERE receipt_code IS NOT NULL;

ALTER TABLE ledgers SET (timescaledb.compress, timescaledb.compress_segmentby = 'account_type, account_id, currency', timescaledb.compress_orderby = 'created_at DESC');
SELECT add_compression_policy('ledgers', INTERVAL '90 days');

-- ===============================
-- TRANSACTION FEE RULES (NUMERIC AMOUNTS)
-- ===============================
CREATE TABLE IF NOT EXISTS transaction_fee_rules (
  id                  BIGSERIAL PRIMARY KEY,
  rule_name           TEXT NOT NULL,
  transaction_type    transaction_type_enum NOT NULL,
  source_currency     VARCHAR(8) REFERENCES currencies(code),
  target_currency     VARCHAR(8) REFERENCES currencies(code),
  account_type        account_type_enum,
  owner_type          owner_type_enum,
  fee_type            fee_type_enum NOT NULL,
  calculation_method  TEXT NOT NULL CHECK (calculation_method IN ('percentage', 'fixed', 'tiered')),
  fee_value           NUMERIC(10,6) NOT NULL,
  min_fee             NUMERIC(30, 18),  -- ✅ Changed from BIGINT
  max_fee             NUMERIC(30, 18),  -- ✅ Changed from BIGINT
  tiers               JSONB,
  valid_from          TIMESTAMPTZ NOT NULL DEFAULT now(),
  valid_to            TIMESTAMPTZ,
  is_active           BOOLEAN NOT NULL DEFAULT true,
  priority            INT NOT NULL DEFAULT 0,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT chk_real_fees CHECK (account_type IS NULL OR account_type = 'real'),
  CONSTRAINT chk_min_max_fee CHECK (min_fee IS NULL OR max_fee IS NULL OR min_fee <= max_fee),
  CONSTRAINT chk_fee_value_positive CHECK (fee_value >= 0),
  CONSTRAINT chk_percentage_range CHECK (
    calculation_method != 'percentage' OR 
    (fee_value >= 0 AND fee_value <= 1)
  ),
  CONSTRAINT chk_tiered_has_tiers CHECK (
    (calculation_method = 'tiered' AND tiers IS NOT NULL) OR
    (calculation_method != 'tiered')
  ),
  CONSTRAINT chk_valid_date_range CHECK (valid_to IS NULL OR valid_from < valid_to)
);

-- Add tariffs column (similar to tiers but for amount-based pricing)
ALTER TABLE transaction_fee_rules 
ADD COLUMN tariffs JSONB;

-- Add index for tariffs queries
CREATE INDEX idx_transaction_fee_rules_tariffs ON transaction_fee_rules USING GIN (tariffs) 
WHERE tariffs IS NOT NULL;

-- Add constraint to ensure tariffs is valid JSON when present
ALTER TABLE transaction_fee_rules
ADD CONSTRAINT chk_tariffs_valid CHECK (
    tariffs IS NULL OR jsonb_typeof(tariffs) = 'array'
);

COMMENT ON COLUMN transaction_fee_rules.tariffs IS 
'Amount-based tariff structure:  [{"min_amount": 0, "max_amount": 100, "fee_bps": 50, "fixed_fee": 0.50}, ...]';

ALTER TABLE transaction_fee_rules
DROP CONSTRAINT IF EXISTS chk_percentage_range;

ALTER TABLE transaction_fee_rules
ADD CONSTRAINT chk_percentage_range CHECK (
  calculation_method != 'percentage'
  OR (fee_value >= 0 AND fee_value <= 10000)
);

-- New
-- Add network fee tracking table
CREATE TABLE IF NOT EXISTS network_fees (
    id BIGSERIAL PRIMARY KEY,
    chain VARCHAR(50) NOT NULL,
    asset VARCHAR(50) NOT NULL,
    operation VARCHAR(50) NOT NULL,
    
    -- Estimated fees (from blockchain)
    base_fee NUMERIC(30, 18) NOT NULL,
    priority_fee NUMERIC(30, 18),
    
    -- TRON-specific
    energy_used BIGINT,
    bandwidth_used BIGINT,
    
    -- BTC-specific
    satoshis_per_byte BIGINT,
    estimated_bytes INT,
    
    -- Fee markup configuration
    markup_percentage NUMERIC(5, 2) DEFAULT 0,  -- e.g., 10% = 10. 00
    
    estimated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ NOT NULL,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_network_fees_chain_asset ON network_fees(chain, asset, operation);
-- End New

CREATE UNIQUE INDEX IF NOT EXISTS uq_fee_rule_active 
  ON transaction_fee_rules (
    transaction_type, 
    fee_type,
    priority,
    (source_currency IS NOT DISTINCT FROM NULL),
    (target_currency IS NOT DISTINCT FROM NULL),
    (account_type IS NOT DISTINCT FROM NULL),
    (owner_type IS NOT DISTINCT FROM NULL)
  ) 
  WHERE is_active = true AND valid_to IS NULL;
-- Indexes for fee rules (unchanged)

DROP INDEX IF EXISTS uq_fee_rule_active;

CREATE UNIQUE INDEX IF NOT EXISTS uq_fee_rule_active_simple
  ON transaction_fee_rules (
    transaction_type,
    fee_type,
    priority,
    source_currency,
    target_currency,
    account_type,
    owner_type
  )
  WHERE is_active = true AND valid_to IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_rule_name_active 
  ON transaction_fee_rules (rule_name) 
  WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_fee_rules_lookup 
  ON transaction_fee_rules (transaction_type, priority DESC, valid_from, valid_to) 
  WHERE is_active = true;

COMMENT ON TABLE transaction_fee_rules IS 
  'Fee rules for transactions. Uses priority-based matching with unique indexes to prevent conflicts.';
COMMENT ON COLUMN transaction_fee_rules.priority IS 
  'Higher priority rules are selected first.  Use: 1=default, 2-9=specific, 10+=special cases, 100+=promotions. ';
COMMENT ON COLUMN transaction_fee_rules.tiers IS 
  'JSON array for tiered fees. Format: [{"min_amount": 100. 00, "max_amount": 1000.00, "rate": 0.002, "fixed_fee": 0.50}]';

-- ===============================
-- APPLIED FEES (NUMERIC AMOUNTS)
-- ===============================
CREATE TABLE IF NOT EXISTS transaction_fees (
  id                      BIGSERIAL PRIMARY KEY,
  receipt_code            TEXT NOT NULL REFERENCES receipt_lookup(code),
  fee_rule_id             BIGINT REFERENCES transaction_fee_rules(id),
  fee_type                fee_type_enum NOT NULL,
  amount                  NUMERIC(30, 18) NOT NULL CHECK (amount >= 0),  -- ✅ Changed from BIGINT
  currency                VARCHAR(8) NOT NULL REFERENCES currencies(code),
  collected_by_account_id BIGINT REFERENCES accounts(id),
  ledger_id               BIGINT,
  agent_external_id       TEXT,
  commission_rate         NUMERIC(5,4),
  created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT uniq_fee_per_receipt UNIQUE (receipt_code, fee_type),
  CONSTRAINT chk_agent_commission CHECK (
    (fee_type = 'agent_commission' AND agent_external_id IS NOT NULL) OR
    (fee_type != 'agent_commission')
  )
);

CREATE INDEX IF NOT EXISTS idx_fees_receipt ON transaction_fees (receipt_code);
CREATE INDEX IF NOT EXISTS idx_fees_agent ON transaction_fees (agent_external_id) WHERE agent_external_id IS NOT NULL;

-- ===============================
-- AGENT COMMISSIONS (NUMERIC AMOUNTS)
-- ===============================
CREATE TABLE IF NOT EXISTS agent_commissions (
  id                    BIGSERIAL PRIMARY KEY,
  agent_external_id     TEXT NOT NULL,
  user_external_id      TEXT NOT NULL,
  agent_account_id      BIGINT NOT NULL REFERENCES accounts(id),
  user_account_id       BIGINT NOT NULL REFERENCES accounts(id),
  receipt_code          TEXT NOT NULL REFERENCES receipt_lookup(code),
  transaction_amount    NUMERIC(30, 18) NOT NULL,  -- ✅ Changed from BIGINT
  commission_rate       NUMERIC(5,4) NOT NULL,
  commission_amount     NUMERIC(30, 18) NOT NULL,  -- ✅ Changed from BIGINT
  currency              VARCHAR(8) NOT NULL REFERENCES currencies(code),
  paid_out              BOOLEAN NOT NULL DEFAULT false,
  payout_receipt_code   TEXT REFERENCES receipt_lookup(code),
  paid_out_at           TIMESTAMPTZ,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_commissions_agent ON agent_commissions (agent_external_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_commissions_unpaid ON agent_commissions (agent_external_id) WHERE paid_out = false;

-- ===============================
-- TRANSACTION HOLDS (NUMERIC AMOUNTS)
-- ===============================
CREATE TABLE IF NOT EXISTS transaction_holds (
  id           BIGSERIAL PRIMARY KEY,
  account_id   BIGINT NOT NULL REFERENCES accounts(id),
  receipt_code TEXT REFERENCES receipt_lookup(code),
  hold_amount  NUMERIC(30, 18) NOT NULL CHECK (hold_amount > 0),  -- ✅ Changed from BIGINT
  currency     VARCHAR(8) NOT NULL REFERENCES currencies(code),
  hold_type    TEXT NOT NULL,
  reason       TEXT,
  expires_at   TIMESTAMPTZ,
  released     BOOLEAN NOT NULL DEFAULT false,
  released_at  TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

  CONSTRAINT chk_hold_release CHECK (
    (released = false AND released_at IS NULL) OR
    (released = true AND released_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_holds ON transaction_holds (account_id) WHERE released = false;

-- ===============================
-- AUDIT LOG
-- ===============================
CREATE TABLE IF NOT EXISTS audit_log (
  id                 BIGSERIAL NOT NULL,
  entity_type        TEXT NOT NULL,
  entity_id          TEXT NOT NULL,
  action             TEXT NOT NULL,
  account_type       account_type_enum,
  actor_type         owner_type_enum,
  actor_external_id  TEXT,
  ip_address         INET,
  changes            JSONB,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  PRIMARY KEY (id, created_at)
);

SELECT create_hypertable('audit_log', 'created_at', if_not_exists => TRUE, chunk_time_interval => INTERVAL '1 month');
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_log (entity_type, entity_id, created_at DESC);
ALTER TABLE audit_log SET (timescaledb.compress, timescaledb.compress_segmentby = 'account_type, entity_type', timescaledb.compress_orderby = 'created_at DESC');
SELECT add_compression_policy('audit_log', INTERVAL '180 days');

-- ===============================
-- DAILY SETTLEMENTS (NUMERIC AMOUNTS)
-- ===============================
CREATE TABLE IF NOT EXISTS daily_settlements (
  id                  BIGSERIAL NOT NULL,
  settlement_date     DATE NOT NULL,
  currency            VARCHAR(8) NOT NULL REFERENCES currencies(code),
  owner_type          owner_type_enum NOT NULL,
  transaction_count   BIGINT NOT NULL DEFAULT 0,
  total_volume        NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  total_fees          NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  deposit_volume      NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  withdrawal_volume   NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  conversion_volume   NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  trade_volume        NUMERIC(30, 18) NOT NULL DEFAULT 0,  -- ✅ Changed from BIGINT
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  PRIMARY KEY (settlement_date, currency, owner_type)
);

CREATE INDEX IF NOT EXISTS idx_settlements ON daily_settlements (settlement_date DESC);

-- ===============================
-- DEMO RESETS
-- ===============================
CREATE TABLE IF NOT EXISTS demo_account_resets (
  id               BIGSERIAL PRIMARY KEY,
  user_external_id TEXT NOT NULL,
  reset_reason     TEXT,
  accounts_reset   INT NOT NULL,
  balances_reset   JSONB,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_resets ON demo_account_resets (user_external_id, created_at DESC);

-- ===============================
-- VIEWS
-- ===============================
CREATE OR REPLACE VIEW real_account_ledgers AS
SELECT l.id, l.journal_id, j.transaction_type, l.account_id, a.account_number,
       a.owner_type, a.owner_id AS owner_external_id, l.amount, l.dr_cr,
       l.currency, l.receipt_code, l.balance_after, l.created_at
FROM ledgers l
JOIN journals j ON l.journal_id = j.id
JOIN accounts a ON l.account_id = a.id
WHERE l.account_type = 'real';

CREATE OR REPLACE VIEW demo_account_ledgers AS
SELECT l.id, l.journal_id, j.transaction_type, l.account_id, a.account_number,
       a.owner_type, a.owner_id AS owner_external_id, l.amount, l.dr_cr,
       l.currency, l.receipt_code, l.balance_after, l.created_at
FROM ledgers l
JOIN journals j ON l.journal_id = j.id
JOIN accounts a ON l.account_id = a. id
WHERE l.account_type = 'demo';

CREATE OR REPLACE VIEW real_account_receipts AS
SELECT 
    rl.code AS receipt_code,
    r.transaction_type,
    ca.account_number AS creditor_account,
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
    r.transaction_type,
    ca.account_number AS creditor_account,
    ca.owner_type AS creditor_type,
    ca.owner_id AS creditor_id,
    da. account_number AS debitor_account,
    da.owner_type AS debitor_type,
    da.owner_id AS debitor_id,
    r.amount,
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
WHERE r.account_type = 'demo';

CREATE OR REPLACE VIEW real_user_accounts AS
SELECT 
    a.id AS account_id,
    a.account_number,
    a.owner_id AS user_id,
    a.currency,
    a.purpose,
    COALESCE(b.balance, 0) AS balance,
    COALESCE(b.available_balance, 0) AS available_balance,
    COALESCE(b.pending_debit, 0) AS pending_debit,
    COALESCE(b.pending_credit, 0) AS pending_credit,
    a.is_active,
    a.is_locked
FROM accounts a
LEFT JOIN balances b ON a.id = b. account_id
WHERE a.owner_type = 'user' AND a.account_type = 'real';

CREATE OR REPLACE VIEW demo_user_accounts AS
SELECT 
    a.id AS account_id,
    a.account_number,
    a.owner_id AS user_id,
    a. currency,
    a.purpose,
    COALESCE(b.balance, 0) AS balance,
    COALESCE(b.available_balance, 0) AS available_balance,
    a.is_active,
    a.is_locked
FROM accounts a
LEFT JOIN balances b ON a.id = b.account_id
WHERE a.owner_type = 'user' AND a.account_type = 'demo';

CREATE OR REPLACE VIEW agent_performance AS
SELECT agent_external_id, COUNT(*) AS total_transactions,
       SUM(transaction_amount) AS total_volume,
       SUM(commission_amount) AS total_commission_earned,
       SUM(CASE WHEN paid_out THEN commission_amount ELSE 0 END) AS commission_paid,
       SUM(CASE WHEN NOT paid_out THEN commission_amount ELSE 0 END) AS commission_pending,
       COUNT(DISTINCT user_external_id) AS unique_customers
FROM agent_commissions
GROUP BY agent_external_id;

-- ===============================
-- MATERIALIZED VIEWS
-- ===============================
CREATE MATERIALIZED VIEW IF NOT EXISTS system_holdings_real AS
SELECT a.owner_type, a.currency, COUNT(DISTINCT a.id) AS account_count,
       SUM(COALESCE(b.balance, 0)) AS total_balance
FROM accounts a
LEFT JOIN balances b ON a.id = b. account_id
WHERE a.is_active = true AND a.account_type = 'real'
GROUP BY a.owner_type, a.currency;

CREATE UNIQUE INDEX idx_holdings_pk ON system_holdings_real (owner_type, currency);

CREATE MATERIALIZED VIEW IF NOT EXISTS daily_transaction_volume_real AS
SELECT 
    DATE(r.created_at) AS transaction_date,
    r.currency,
    r.transaction_type,
    COUNT(*) AS transaction_count,
    SUM(r.amount) AS total_volume,
    AVG(r.amount) AS avg_transaction_size
FROM fx_receipts r
WHERE r. status = 'completed' AND r.account_type = 'real'
GROUP BY DATE(r.created_at), r.currency, r.transaction_type;

CREATE UNIQUE INDEX idx_daily_volume_real_pk 
    ON daily_transaction_volume_real (transaction_date, currency, transaction_type);

CREATE MATERIALIZED VIEW IF NOT EXISTS demo_activity_summary AS
SELECT 
    DATE(r.created_at) AS activity_date,
    r.currency,
    r.transaction_type,
    COUNT(*) AS transaction_count,
    COUNT(DISTINCT r.debitor_account_id) AS active_users,
    SUM(r. amount) AS total_volume
FROM fx_receipts r
WHERE r.status = 'completed' AND r.account_type = 'demo'
GROUP BY DATE(r.created_at), r. currency, r.transaction_type;

CREATE UNIQUE INDEX idx_demo_activity_pk 
    ON demo_activity_summary (activity_date, currency, transaction_type);

-- ===============================
-- FUNCTIONS
-- ===============================
CREATE OR REPLACE FUNCTION generate_receipt_code(p_account_type account_type_enum)
RETURNS TEXT AS $$
BEGIN
    RETURN (CASE WHEN p_account_type = 'real' THEN 'RCP' ELSE 'DEMO' END) || '-' || 
           TO_CHAR(NOW(), 'YYYY') || '-' || LPAD(nextval('receipt_lookup_id_seq')::TEXT, 12, '0');
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION reset_demo_account(p_user_external_id TEXT)
RETURNS JSONB AS $$
DECLARE
    v_account RECORD;
    v_balances_reset JSONB := '[]'::JSONB;
    v_reset_count INT;
    v_initial_balance NUMERIC(30, 18);  -- ✅ Changed from BIGINT
BEGIN
    FOR v_account IN 
        SELECT a.id, a.currency 
        FROM accounts a 
        WHERE a.owner_id = p_user_external_id AND a.account_type = 'demo'
    LOOP
        SELECT demo_initial_balance INTO v_initial_balance
        FROM currencies 
        WHERE code = v_account.currency;
        
        UPDATE balances 
        SET balance = v_initial_balance,
            available_balance = v_initial_balance,
            pending_debit = 0,
            pending_credit = 0,
            version = version + 1,
            updated_at = now()
        WHERE account_id = v_account.id;
        
        v_balances_reset := v_balances_reset || jsonb_build_object(
            'currency', v_account.currency,
            'new_balance', v_initial_balance
        );
    END LOOP;
    
    INSERT INTO demo_account_metadata (user_external_id, demo_reset_count, last_demo_reset)
    VALUES (p_user_external_id, 1, now())
    ON CONFLICT (user_external_id) DO UPDATE 
    SET demo_reset_count = demo_account_metadata.demo_reset_count + 1, 
        last_demo_reset = now()
    RETURNING demo_reset_count INTO v_reset_count;
    
    INSERT INTO demo_account_resets (user_external_id, reset_reason, accounts_reset, balances_reset)
    VALUES (p_user_external_id, 'User requested reset', jsonb_array_length(v_balances_reset), v_balances_reset);
    
    RETURN jsonb_build_object(
        'success', true, 
        'reset_count', v_reset_count,
        'balances', v_balances_reset
    );
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION validate_account_type_match()
RETURNS TRIGGER AS $$
DECLARE
    v_creditor_type account_type_enum;
    v_debitor_type account_type_enum;
BEGIN
    SELECT account_type INTO v_creditor_type FROM accounts WHERE id = NEW.creditor_account_id;
    SELECT account_type INTO v_debitor_type FROM accounts WHERE id = NEW.debitor_account_id;
    
    IF v_creditor_type != NEW.account_type OR v_debitor_type != NEW.account_type THEN
        RAISE EXCEPTION 'Cannot mix real and demo accounts';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_validate_receipt_types BEFORE INSERT OR UPDATE ON fx_receipts
    FOR EACH ROW EXECUTE FUNCTION validate_account_type_match();

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_currencies_updated_at BEFORE UPDATE ON currencies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_accounts_updated_at BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON agent_relationships
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ===============================
-- INITIAL DATA (NUMERIC VALUES)
-- ===============================
INSERT INTO currencies (code, name, symbol, decimals, is_fiat, demo_enabled, demo_initial_balance) VALUES
('USD', 'United States Dollar', '$', 2, true, true, 10000. 00),  -- ✅ Decimal value
('USDT', 'Tether USD', '₮', 6, false, true, 10000.000000),  -- ✅ Decimal value
('BTC', 'Bitcoin', '₿', 8, false, true, 0.10000000)  -- ✅ Decimal value
ON CONFLICT DO NOTHING;

INSERT INTO accounts (owner_type, owner_id, currency, purpose, account_type, account_number) VALUES
('system', 'system', 'USD', 'liquidity', 'real', 'SYS-LIQ-USD'),
('system', 'system', 'USDT', 'liquidity', 'real', 'SYS-LIQ-USDT'),
('system', 'system', 'BTC', 'liquidity', 'real', 'SYS-LIQ-BTC'),
('system', 'system', 'USD', 'fees', 'real', 'SYS-FEE-USD'),
('system', 'system', 'USDT', 'fees', 'real', 'SYS-FEE-USDT'),
('system', 'system', 'BTC', 'fees', 'real', 'SYS-FEE-BTC')
ON CONFLICT DO NOTHING;

INSERT INTO balances (account_id, balance, available_balance)
SELECT id, 10000000. 00, 10000000.00 FROM accounts WHERE owner_type = 'system'  -- ✅ Decimal value
ON CONFLICT DO NOTHING;

-- Fee rules with NUMERIC values in tiers
INSERT INTO transaction_fee_rules (
    rule_name, 
    transaction_type, 
    source_currency,
    target_currency,
    account_type,
    owner_type,
    fee_type, 
    calculation_method, 
    fee_value, 
    min_fee,
    max_fee,
    tiers,
    is_active, 
    priority
) VALUES
('Standard USD Deposit Fee', 'deposit', 'USD', NULL, 'real', NULL, 'platform', 'percentage', 0.001, 1. 00, 500. 00, NULL, true, 1),
('VIP User Withdrawal Fee', 'withdrawal', NULL, NULL, 'real', 'user', 'platform', 'tiered', 0, NULL, NULL, 
'[
  {"min_amount": 0, "max_amount": 1000. 00, "rate": 0. 002, "fixed_fee": 1.00},
  {"min_amount": 1000.00, "max_amount": 5000.00, "rate": 0.0015, "fixed_fee": 1.50},
  {"min_amount": 5000.00, "max_amount": null, "rate": 0.001, "fixed_fee": 2.00}
]'::JSONB, 
true, 2)
ON CONFLICT DO NOTHING;

COMMIT;

ANALYZE;