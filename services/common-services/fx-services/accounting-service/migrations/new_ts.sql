-- ===============================================================================================
-- Target: 1M+ users, millions of transactions/day
-- Features: Multi-tenancy, Agent support, Currency conversion, Transaction fees, Audit trails
-- Version: 2.0
-- Last Updated: 2025-11-16
-- ===============================================================================================

\c pxyz_fx;

BEGIN;

-- ===============================
-- EXTENSIONS
-- ===============================
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";           -- For UUID generation
CREATE EXTENSION IF NOT EXISTS pgcrypto;              -- For encryption functions
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;    -- For query performance monitoring

-- ===============================
-- ENUMS (with conditional creation)
-- ===============================
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'owner_type_enum') THEN
    CREATE TYPE owner_type_enum AS ENUM ('system','partner','user','agent');  -- Added 'agent'
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'account_purpose_enum') THEN
    CREATE TYPE account_purpose_enum AS ENUM (
      'liquidity',      -- System liquidity pool
      'clearing',       -- Clearing account for settlements
      'fees',           -- Fee collection account
      'wallet',         -- User/partner trading wallet
      'escrow',         -- Escrow for pending transactions
      'settlement',     -- Settlement account
      'revenue',        -- Platform revenue
      'contra',         -- Contra/offset account
      'commission'      -- Agent commission account
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
      'pending',        -- Initial state
      'processing',     -- Being processed
      'completed',      -- Successfully completed
      'failed',         -- Failed
      'reversed',       -- Reversed/cancelled
      'suspended',      -- Temporarily suspended
      'expired'         -- Expired (for time-limited transactions)
    );
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'transaction_type_enum') THEN
    CREATE TYPE transaction_type_enum AS ENUM (
      'deposit',        -- Admin -> Partner, Partner -> User
      'withdrawal',     -- User -> Partner, Partner -> System
      'conversion',     -- Currency conversion (e.g., USD -> USDT)
      'trade',          -- Crypto purchase/sale
      'transfer',       -- P2P transfer
      'fee',            -- Fee deduction
      'commission',     -- Agent commission
      'reversal',       -- Transaction reversal
      'adjustment'      -- Manual adjustment
    );
  END IF;
END $$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'fee_type_enum') THEN
    CREATE TYPE fee_type_enum AS ENUM (
      'platform',       -- Platform fee
      'network',        -- Blockchain network fee
      'partner',        -- Partner fee
      'conversion',     -- Currency conversion fee
      'withdrawal',     -- Withdrawal fee
      'agent_commission' -- Agent commission
    );
  END IF;
END $$;

-- ===============================
-- CURRENCIES
-- ===============================
CREATE TABLE IF NOT EXISTS currencies (
  code        VARCHAR(8) PRIMARY KEY,
  name        TEXT NOT NULL,
  symbol      VARCHAR(10),                    -- Display symbol (e.g., $, ₿)
  decimals    SMALLINT NOT NULL DEFAULT 2 CHECK (decimals >= 0 AND decimals <= 18),
  is_fiat     BOOLEAN NOT NULL DEFAULT true,  -- Distinguish fiat vs crypto
  is_active   BOOLEAN NOT NULL DEFAULT true,  -- Can be disabled without deletion
  min_amount  BIGINT NOT NULL DEFAULT 1,      -- Minimum transaction amount (in atomic units)
  max_amount  BIGINT,                         -- Maximum transaction amount
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_currencies_active ON currencies (is_active, code) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_currencies_name ON currencies (name);

COMMENT ON TABLE currencies IS 'Master list of supported currencies (fiat and crypto)';
COMMENT ON COLUMN currencies.decimals IS 'Number of decimal places (e.g., 2 for USD, 8 for BTC)';

-- ===============================
-- FX RATES (Hypertable for historical rates)
-- ===============================
CREATE TABLE IF NOT EXISTS fx_rates (
  id             BIGSERIAL NOT NULL,
  base_currency  VARCHAR(8) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE,
  quote_currency VARCHAR(8) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE,
  rate           NUMERIC(30,18) NOT NULL CHECK (rate > 0),  -- High precision for crypto rates
  bid_rate       NUMERIC(30,18),                            -- Bid price (optional)
  ask_rate       NUMERIC(30,18),                            -- Ask price (optional)
  spread         NUMERIC(10,6),                             -- Spread percentage
  source         TEXT,                                       -- Rate source (e.g., 'coinbase','binance')
  valid_from     TIMESTAMPTZ NOT NULL DEFAULT now(),
  valid_to       TIMESTAMPTZ,                               -- Null = current rate
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id, valid_from)
);

-- Create hypertable for fx_rates
SELECT create_hypertable(
  'fx_rates',
  'valid_from',
  if_not_exists => TRUE,
  chunk_time_interval => INTERVAL '7 days'  -- Weekly chunks for rate history
);

CREATE INDEX IF NOT EXISTS idx_fx_rates_pair_current 
  ON fx_rates (base_currency, quote_currency, valid_from DESC) 
  WHERE valid_to IS NULL;

CREATE INDEX IF NOT EXISTS idx_fx_rates_lookup 
  ON fx_rates (base_currency, quote_currency, valid_from DESC);

-- Compression for old rates
ALTER TABLE fx_rates SET (
  timescaledb.compress,
  timescaledb.compress_orderby = 'valid_from DESC'
);
SELECT add_compression_policy('fx_rates', INTERVAL '90 days');

COMMENT ON TABLE fx_rates IS 'Historical FX rates for currency conversions';

-- ===============================
-- USERS (Optional - for reference) agents
-- ===============================
CREATE TABLE IF NOT EXISTS users (
  id              BIGSERIAL PRIMARY KEY,
  external_id     TEXT UNIQUE NOT NULL,          -- ID from your auth/user service
  email           TEXT,                           -- Encrypted or hashed
  kyc_level       SMALLINT NOT NULL DEFAULT 0,   -- 0=none, 1=basic, 2=verified, 3=premium
  is_active       BOOLEAN NOT NULL DEFAULT true,
  is_agent        BOOLEAN NOT NULL DEFAULT false, -- Flag for agent users
  agent_level     SMALLINT DEFAULT 0,             -- Agent tier (0=not agent, 1-5=tiers)
  referred_by     BIGINT REFERENCES users(id),    -- For agent referral tracking
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_users_external_id ON users (external_id);
CREATE INDEX IF NOT EXISTS idx_users_agent ON users (is_agent) WHERE is_agent = true;
CREATE INDEX IF NOT EXISTS idx_users_referrer ON users (referred_by) WHERE referred_by IS NOT NULL;

COMMENT ON TABLE users IS 'Minimal user reference table - full data in separate user service';


-- ===============================
-- ACCOUNTS (Enhanced with sharding key)
-- ===============================
CREATE TABLE IF NOT EXISTS accounts (
  id             BIGSERIAL PRIMARY KEY,
  account_number TEXT NOT NULL UNIQUE,           -- Human-readable account number
  owner_type     owner_type_enum NOT NULL,
  owner_id       TEXT NOT NULL,                  -- External ID from user/partner service
  currency       VARCHAR(8) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE,
  purpose        account_purpose_enum NOT NULL,
  account_type   account_type_enum NOT NULL DEFAULT 'real',
  is_active      BOOLEAN NOT NULL DEFAULT true,
  is_locked      BOOLEAN NOT NULL DEFAULT false, -- For security holds
  overdraft_limit BIGINT NOT NULL DEFAULT 0,     -- Overdraft allowance (usually 0)
  
  -- Agent-specific fields
  parent_agent_id BIGINT REFERENCES accounts(id), -- For agent hierarchy
  commission_rate NUMERIC(5,4),                    -- Agent-specific commission
  
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT uq_account_owner_currency_purpose 
    UNIQUE (owner_type, owner_id, currency, purpose, account_type),
  CONSTRAINT chk_agent_commission 
    CHECK (owner_type != 'agent' OR commission_rate IS NOT NULL)
);

-- Partition-friendly indexes
CREATE INDEX IF NOT EXISTS idx_accounts_owner ON accounts (owner_type, owner_id);
CREATE INDEX IF NOT EXISTS idx_accounts_currency ON accounts (currency) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_accounts_active ON accounts (is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_accounts_agent_hierarchy ON accounts (parent_agent_id) 
  WHERE parent_agent_id IS NOT NULL;

COMMENT ON TABLE accounts IS 'All accounts for users, partners, agents, and system';
COMMENT ON COLUMN accounts.owner_id IS 'External ID reference - do not use internal auto-increment IDs';

-- ===============================
-- BALANCES (Real-time balance cache)
-- ===============================
CREATE TABLE IF NOT EXISTS balances (
  account_id      BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  balance         BIGINT NOT NULL DEFAULT 0,
  available_balance BIGINT NOT NULL DEFAULT 0,  -- Balance minus holds/locks
  pending_debit   BIGINT NOT NULL DEFAULT 0,    -- Sum of pending debits
  pending_credit  BIGINT NOT NULL DEFAULT 0,    -- Sum of pending credits
  last_ledger_id  BIGINT,                       -- Last ledger entry ID processed
  version         BIGINT NOT NULL DEFAULT 0,    -- Optimistic locking version
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT chk_balance_non_negative CHECK (balance >= 0),
  CONSTRAINT chk_available_non_negative CHECK (available_balance >= 0)
);

CREATE INDEX IF NOT EXISTS idx_balances_balance ON balances (balance) WHERE balance > 0;
CREATE INDEX IF NOT EXISTS idx_balances_updated ON balances (updated_at DESC);

COMMENT ON TABLE balances IS 'Cached account balances - updated transactionally with ledger entries';
COMMENT ON COLUMN balances.version IS 'Optimistic lock version - increment on every update';

-- ===============================
-- JOURNALS (Transaction container)
-- ===============================
CREATE TABLE IF NOT EXISTS journals (
  id               BIGSERIAL PRIMARY KEY,
  idempotency_key  TEXT UNIQUE,                 -- Global unique key for deduplication
  transaction_type transaction_type_enum NOT NULL,
  external_ref     TEXT,                        -- Reference to external system
  description      TEXT,
  created_by_user  BIGINT,                      -- User ID who initiated
  created_by_type  owner_type_enum,
  ip_address       INET,                        -- For audit trail
  user_agent       TEXT,                        -- For audit trail
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT chk_journal_creator CHECK (
    created_by_user IS NULL OR created_by_type IS NOT NULL
  )
);

CREATE INDEX IF NOT EXISTS idx_journals_created_at ON journals (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_journals_type ON journals (transaction_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_journals_external_ref ON journals (external_ref) 
  WHERE external_ref IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_journals_creator ON journals (created_by_type, created_by_user) 
  WHERE created_by_user IS NOT NULL;

COMMENT ON TABLE journals IS 'Transaction header - groups related ledger entries';
COMMENT ON COLUMN journals.idempotency_key IS 'Ensures exactly-once transaction processing';

-- ===============================
-- RECEIPT LOOKUP (Stable IDs for receipts)
-- ===============================
CREATE TABLE IF NOT EXISTS receipt_lookup (
  id          BIGSERIAL PRIMARY KEY,
  code        TEXT NOT NULL UNIQUE,             -- Human-readable code (e.g., RCP-2025-001234567)
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_receipt_lookup_code ON receipt_lookup (code);

COMMENT ON TABLE receipt_lookup IS 'Stable receipt IDs and codes for hypertable references';

-- ===============================
-- FX RECEIPTS (Hypertable - Transaction records)
-- ===============================
CREATE TABLE IF NOT EXISTS fx_receipts (
  lookup_id              BIGINT NOT NULL REFERENCES receipt_lookup(id) ON DELETE CASCADE,
  
  -- Creditor (receiver) details
  creditor_account_id    BIGINT NOT NULL REFERENCES accounts(id),
  creditor_ledger_id     BIGINT,
  creditor_account_type  owner_type_enum NOT NULL,
  creditor_status        transaction_status_enum NOT NULL DEFAULT 'pending',
  
  -- Debitor (sender) details
  debitor_account_id     BIGINT NOT NULL REFERENCES accounts(id),
  debitor_ledger_id      BIGINT,
  debitor_account_type   owner_type_enum NOT NULL,
  debitor_status         transaction_status_enum NOT NULL DEFAULT 'pending',
  
  -- Transaction details
  transaction_type       transaction_type_enum NOT NULL,
  coded_type             TEXT,                    -- Additional type coding
  amount                 BIGINT NOT NULL CHECK (amount > 0),
  original_amount        BIGINT,                  -- For conversions (original currency amount)
  transaction_cost       BIGINT NOT NULL DEFAULT 0,
  currency               VARCHAR(8) NOT NULL REFERENCES currencies(code),
  original_currency      VARCHAR(8) REFERENCES currencies(code), -- For conversions
  exchange_rate          NUMERIC(30,18),          -- Rate used for conversion
  
  -- References
  external_ref           TEXT,
  parent_receipt_code    TEXT REFERENCES receipt_lookup(code), -- For linked transactions
  
  -- Status and lifecycle
  status                 transaction_status_enum NOT NULL DEFAULT 'pending',
  error_message          TEXT,                    -- Error details if failed
  completed_at           TIMESTAMPTZ,
  
  -- Audit trail
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ,
  created_by             TEXT DEFAULT 'system',
  reversed_at            TIMESTAMPTZ,
  reversed_by            TEXT,
  reversal_receipt_code  TEXT REFERENCES receipt_lookup(code), -- Link to reversal receipt
  
  -- Metadata
  metadata               JSONB,                   -- Flexible JSON for additional data
  
  PRIMARY KEY (lookup_id, created_at),
  
  CONSTRAINT chk_conversion_fields CHECK (
    (transaction_type = 'conversion' AND original_currency IS NOT NULL AND exchange_rate IS NOT NULL)
    OR (transaction_type != 'conversion')
  ),
  CONSTRAINT chk_different_accounts CHECK (creditor_account_id != debitor_account_id)
);

-- Create hypertable
SELECT create_hypertable(
  'fx_receipts',
  'created_at',
  if_not_exists => TRUE,
  chunk_time_interval => INTERVAL '1 week'  -- Weekly chunks for better query performance
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_fx_receipts_created_at 
  ON fx_receipts (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_fx_receipts_status 
  ON fx_receipts (status, created_at DESC) WHERE status != 'completed';

CREATE INDEX IF NOT EXISTS idx_fx_receipts_creditor 
  ON fx_receipts (creditor_account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_fx_receipts_debitor 
  ON fx_receipts (debitor_account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_fx_receipts_type 
  ON fx_receipts (transaction_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_fx_receipts_currency 
  ON fx_receipts (currency, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_fx_receipts_external_ref 
  ON fx_receipts (external_ref) WHERE external_ref IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_fx_receipts_parent 
  ON fx_receipts (parent_receipt_code) WHERE parent_receipt_code IS NOT NULL;

-- Compression policy
ALTER TABLE fx_receipts SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'transaction_type, currency',
  timescaledb.compress_orderby = 'created_at DESC'
);

SELECT add_compression_policy('fx_receipts', INTERVAL '90 days');

COMMENT ON TABLE fx_receipts IS 'Transaction receipts - main hypertable for all FX transactions';

-- ===============================
-- LEDGERS (Hypertable - Double-entry bookkeeping)
-- ===============================
CREATE TABLE IF NOT EXISTS ledgers (
  id           BIGSERIAL NOT NULL,
  journal_id   BIGINT NOT NULL REFERENCES journals(id) ON DELETE CASCADE,
  account_id   BIGINT NOT NULL REFERENCES accounts(id),
  amount       BIGINT NOT NULL CHECK (amount > 0),
  dr_cr        dr_cr_enum NOT NULL,
  currency     VARCHAR(8) NOT NULL REFERENCES currencies(code),
  
  -- Receipt reference
  receipt_code TEXT REFERENCES receipt_lookup(code),
  
  -- Balance snapshot (for faster reconciliation)
  balance_after BIGINT,                        -- Account balance after this entry
  
  -- Metadata
  description   TEXT,
  metadata      JSONB,
  
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  PRIMARY KEY (id, account_id, created_at)
);

-- Create hypertable with space partitioning
SELECT create_hypertable(
  'ledgers',
  'created_at',
  if_not_exists => TRUE,
  chunk_time_interval => INTERVAL '1 week',
  number_partitions => 128,                    -- Increased for 1M+ users
  partitioning_column => 'account_id'
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_ledgers_account_created 
  ON ledgers (account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ledgers_journal 
  ON ledgers (journal_id);

CREATE INDEX IF NOT EXISTS idx_ledgers_receipt 
  ON ledgers (receipt_code) WHERE receipt_code IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_ledgers_currency 
  ON ledgers (currency, created_at DESC);

-- Compression
ALTER TABLE ledgers SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'account_id, currency',
  timescaledb.compress_orderby = 'created_at DESC'
);

SELECT add_compression_policy('ledgers', INTERVAL '90 days');

COMMENT ON TABLE ledgers IS 'Double-entry ledger - all debits and credits';
COMMENT ON COLUMN ledgers.balance_after IS 'Snapshot balance for fast reconciliation';

-- ===============================
-- TRANSACTION FEE RULES
-- ===============================
CREATE TABLE IF NOT EXISTS transaction_fee_rules (
  id                  BIGSERIAL PRIMARY KEY,
  rule_name           TEXT NOT NULL,
  transaction_type    transaction_type_enum NOT NULL,
  source_currency     VARCHAR(8) REFERENCES currencies(code),
  target_currency     VARCHAR(8) REFERENCES currencies(code),
  
  -- Account type based fees
  account_type        account_type_enum,
  owner_type          owner_type_enum,
  kyc_level           SMALLINT,                -- Fee can vary by KYC level
  
  -- Fee structure
  fee_type            fee_type_enum NOT NULL,
  calculation_method  TEXT NOT NULL CHECK (calculation_method IN ('percentage', 'fixed', 'tiered')),
  fee_value           NUMERIC(10,6) NOT NULL,  -- Percentage or fixed amount
  min_fee             BIGINT,
  max_fee             BIGINT,
  
  -- Tiered pricing (JSON array for multiple tiers)
  tiers               JSONB,  -- e.g., [{"min":0,"max":1000,"rate":0.01},{"min":1000,"max":null,"rate":0.005}]
  
  -- Validity
  valid_from          TIMESTAMPTZ NOT NULL DEFAULT now(),
  valid_to            TIMESTAMPTZ,
  is_active           BOOLEAN NOT NULL DEFAULT true,
  priority            INT NOT NULL DEFAULT 0,  -- Higher priority rules override lower
  
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT chk_fee_currencies CHECK (
    (source_currency IS NULL AND target_currency IS NULL) OR
    (source_currency IS NOT NULL OR target_currency IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_fee_rules_lookup 
  ON transaction_fee_rules (transaction_type, source_currency, target_currency, is_active, priority DESC)
  WHERE is_active = true;

CREATE INDEX IF NOT EXISTS idx_fee_rules_owner_type 
  ON transaction_fee_rules (owner_type, transaction_type, is_active)
  WHERE owner_type IS NOT NULL AND is_active = true;

COMMENT ON TABLE transaction_fee_rules IS 'Configurable fee rules with tiered pricing support';
COMMENT ON COLUMN transaction_fee_rules.tiers IS 'JSON array for tiered fee structure';

-- ===============================
-- APPLIED TRANSACTION FEES
-- ===============================
CREATE TABLE IF NOT EXISTS transaction_fees (
  id               BIGSERIAL PRIMARY KEY,
  receipt_code     TEXT NOT NULL REFERENCES receipt_lookup(code),
  fee_rule_id      BIGINT REFERENCES transaction_fee_rules(id),
  fee_type         fee_type_enum NOT NULL,
  amount           BIGINT NOT NULL CHECK (amount >= 0),
  currency         VARCHAR(8) NOT NULL REFERENCES currencies(code),
  
  -- Fee collection account
  collected_by_account_id BIGINT REFERENCES accounts(id),
  ledger_id        BIGINT,                     -- Link to ledger entry
  
  -- Agent commission fields
  agent_account_id BIGINT REFERENCES accounts(id),
  commission_rate  NUMERIC(5,4),
  
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT uniq_fee_per_receipt UNIQUE (receipt_code, fee_type),
  CONSTRAINT chk_agent_commission CHECK (
    (fee_type = 'agent_commission' AND agent_account_id IS NOT NULL) OR
    (fee_type != 'agent_commission')
  )
);

CREATE INDEX IF NOT EXISTS idx_transaction_fees_receipt 
  ON transaction_fees (receipt_code);

CREATE INDEX IF NOT EXISTS idx_transaction_fees_rule 
  ON transaction_fees (fee_rule_id);

CREATE INDEX IF NOT EXISTS idx_transaction_fees_agent 
  ON transaction_fees (agent_account_id) 
  WHERE agent_account_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_transaction_fees_collector 
  ON transaction_fees (collected_by_account_id);

COMMENT ON TABLE transaction_fees IS 'Actual fees charged per transaction';

-- ===============================
-- AGENT COMMISSIONS (Tracking)
-- ===============================
CREATE TABLE IF NOT EXISTS agent_commissions (
  id                    BIGSERIAL PRIMARY KEY,
  agent_account_id      BIGINT NOT NULL REFERENCES accounts(id),
  user_account_id       BIGINT NOT NULL REFERENCES accounts(id),
  receipt_code          TEXT NOT NULL REFERENCES receipt_lookup(code),
  transaction_amount    BIGINT NOT NULL,
  commission_rate       NUMERIC(5,4) NOT NULL,
  commission_amount     BIGINT NOT NULL,
  currency              VARCHAR(8) NOT NULL REFERENCES currencies(code),
  
  -- Payout tracking
  paid_out              BOOLEAN NOT NULL DEFAULT false,
  payout_receipt_code   TEXT REFERENCES receipt_lookup(code),
  paid_out_at           TIMESTAMPTZ,
  
  created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT chk_commission_positive CHECK (commission_amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_agent_commissions_agent 
  ON agent_commissions (agent_account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_commissions_unpaid 
  ON agent_commissions (agent_account_id) 
  WHERE paid_out = false;

CREATE INDEX IF NOT EXISTS idx_agent_commissions_receipt 
  ON agent_commissions (receipt_code);

COMMENT ON TABLE agent_commissions IS 'Agent commission tracking and payout management';

-- ===============================
-- TRANSACTION HOLDS (Pending/escrow)
-- ===============================
CREATE TABLE IF NOT EXISTS transaction_holds (
  id              BIGSERIAL PRIMARY KEY,
  account_id      BIGINT NOT NULL REFERENCES accounts(id),
  receipt_code    TEXT REFERENCES receipt_lookup(code),
  hold_amount     BIGINT NOT NULL CHECK (hold_amount > 0),
  currency        VARCHAR(8) NOT NULL REFERENCES currencies(code),
  hold_type       TEXT NOT NULL,              -- 'escrow', 'pending', 'compliance'
  reason          TEXT,
  expires_at      TIMESTAMPTZ,
  released        BOOLEAN NOT NULL DEFAULT false,
  released_at     TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  CONSTRAINT chk_hold_release CHECK (
    (released = false AND released_at IS NULL) OR
    (released = true AND released_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_holds_account 
  ON transaction_holds (account_id) WHERE released = false;

CREATE INDEX IF NOT EXISTS idx_holds_expires 
  ON transaction_holds (expires_at) WHERE released = false AND expires_at IS NOT NULL;

COMMENT ON TABLE transaction_holds IS 'Temporary holds on account balances';

-- ===============================
-- AUDIT LOG (Compliance trail)
-- ===============================
CREATE TABLE IF NOT EXISTS audit_log (
  id              BIGSERIAL NOT NULL,
  entity_type     TEXT NOT NULL,              -- 'account', 'receipt', 'user', etc.
  entity_id       TEXT NOT NULL,
  action          TEXT NOT NULL,              -- 'create', 'update', 'delete', 'approve'
  actor_type      owner_type_enum,
  actor_id        TEXT,
  ip_address      INET,
  changes         JSONB,                      -- Before/after snapshot
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  PRIMARY KEY (id, created_at)
);

-- Hypertable for audit log
SELECT create_hypertable(
  'audit_log',
  'created_at',
  if_not_exists => TRUE,
  chunk_time_interval => INTERVAL '1 month'
);

CREATE INDEX IF NOT EXISTS idx_audit_entity 
  ON audit_log (entity_type, entity_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_actor 
  ON audit_log (actor_type, actor_id, created_at DESC);

-- Compression
ALTER TABLE audit_log SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'entity_type',
  timescaledb.compress_orderby = 'created_at DESC'
);

SELECT add_compression_policy('audit_log', INTERVAL '180 days');

COMMENT ON TABLE audit_log IS 'Immutable audit trail for all system changes';

-- ===============================
-- DAILY SETTLEMENT SUMMARY (Reporting)
-- ===============================
CREATE TABLE IF NOT EXISTS daily_settlements (
  id                  BIGSERIAL NOT NULL,
  settlement_date     DATE NOT NULL,
  currency            VARCHAR(8) NOT NULL REFERENCES currencies(code),
  owner_type          owner_type_enum NOT NULL,
  
  -- Volume metrics
  transaction_count   BIGINT NOT NULL DEFAULT 0,
  total_volume        BIGINT NOT NULL DEFAULT 0,
  total_fees          BIGINT NOT NULL DEFAULT 0,
  
  -- By transaction type
  deposit_volume      BIGINT NOT NULL DEFAULT 0,
  withdrawal_volume   BIGINT NOT NULL DEFAULT 0,
  conversion_volume   BIGINT NOT NULL DEFAULT 0,
  trade_volume        BIGINT NOT NULL DEFAULT 0,
  
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  
  PRIMARY KEY (settlement_date, currency, owner_type)
);

CREATE INDEX IF NOT EXISTS idx_daily_settlements_date 
  ON daily_settlements (settlement_date DESC);

COMMENT ON TABLE daily_settlements IS 'Daily aggregated settlement data for reporting';

-- ===============================
-- VIEWS (Convenience layers)
-- ===============================

-- Account ledgers with enriched data
CREATE OR REPLACE VIEW account_ledgers AS
SELECT 
    l.id,
    l.journal_id,
    j.transaction_type,
    l.account_id,
    a.account_number,
    a.owner_type,
    a.owner_id,
    l.amount,
    l.dr_cr,
    l.currency,
    l.receipt_code,
    l.balance_after,
    l.description,
    l.created_at
FROM ledgers l
JOIN journals j ON l.journal_id = j.id
JOIN accounts a ON l.account_id = a.id;

-- Account receipts with human-readable codes
CREATE OR REPLACE VIEW account_receipts AS
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
JOIN accounts da ON r.debitor_account_id = da.id;

-- User account summary
CREATE OR REPLACE VIEW user_account_summary AS
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
LEFT JOIN balances b ON a.id = b.account_id
WHERE a.owner_type = 'user' AND a.account_type = 'real';

-- Agent performance metrics
CREATE OR REPLACE VIEW agent_performance AS
SELECT 
    ac.agent_account_id,
    a.account_number,
    a.owner_id AS agent_user_id,
    COUNT(*) AS total_transactions,
    SUM(ac.transaction_amount) AS total_volume,
    SUM(ac.commission_amount) AS total_commission_earned,
    SUM(CASE WHEN ac.paid_out THEN ac.commission_amount ELSE 0 END) AS commission_paid,
    SUM(CASE WHEN NOT ac.paid_out THEN ac.commission_amount ELSE 0 END) AS commission_pending,
    COUNT(DISTINCT ac.user_account_id) AS unique_customers
FROM agent_commissions ac
JOIN accounts a ON ac.agent_account_id = a.id
GROUP BY ac.agent_account_id, a.account_number, a.owner_id;

COMMENT ON VIEW agent_performance IS 'Agent commission and performance metrics';

-- ===============================
-- MATERIALIZED VIEWS (Pre-computed aggregates)
-- ===============================

-- System holdings by currency and owner type
CREATE MATERIALIZED VIEW IF NOT EXISTS system_holdings AS
SELECT 
    a.owner_type,
    a.currency,
    COUNT(DISTINCT a.id) AS account_count,
    SUM(COALESCE(b.balance, 0)) AS total_balance,
    SUM(COALESCE(b.available_balance, 0)) AS total_available,
    SUM(COALESCE(b.pending_debit, 0)) AS total_pending_debit,
    SUM(COALESCE(b.pending_credit, 0)) AS total_pending_credit
FROM accounts a
LEFT JOIN balances b ON a.id = b.account_id
WHERE a.is_active = true
GROUP BY a.owner_type, a.currency;

CREATE UNIQUE INDEX idx_system_holdings_pk 
    ON system_holdings (owner_type, currency);

COMMENT ON MATERIALIZED VIEW system_holdings IS 'System-wide balance aggregates - refresh periodically';

-- Daily transaction volume by currency
CREATE MATERIALIZED VIEW IF NOT EXISTS daily_transaction_volume AS
SELECT 
    DATE(r.created_at) AS transaction_date,
    r.currency,
    r.transaction_type,
    COUNT(*) AS transaction_count,
    SUM(r.amount) AS total_volume,
    AVG(r.amount) AS avg_transaction_size
FROM fx_receipts r
WHERE r.status = 'completed'
GROUP BY DATE(r.created_at), r.currency, r.transaction_type;

CREATE UNIQUE INDEX idx_daily_volume_pk 
    ON daily_transaction_volume (transaction_date, currency, transaction_type);

COMMENT ON MATERIALIZED VIEW daily_transaction_volume IS 'Daily volume metrics for reporting';

-- ===============================
-- FUNCTIONS
-- ===============================

-- Function to generate unique receipt code
CREATE OR REPLACE FUNCTION generate_receipt_code()
RETURNS TEXT AS $$
DECLARE
    new_code TEXT;
BEGIN
    new_code := 'RCP-' || TO_CHAR(NOW(), 'YYYY') || '-' || LPAD(nextval('receipt_lookup_id_seq')::TEXT, 12, '0');
    RETURN new_code;
END;
$$ LANGUAGE plpgsql;

-- Function to get current exchange rate
CREATE OR REPLACE FUNCTION get_current_fx_rate(p_base VARCHAR(8), p_quote VARCHAR(8))
RETURNS NUMERIC(30,18) AS $$
DECLARE
    current_rate NUMERIC(30,18);
BEGIN
    SELECT rate INTO current_rate
    FROM fx_rates
    WHERE base_currency = p_base 
      AND quote_currency = p_quote
      AND valid_to IS NULL
    ORDER BY valid_from DESC
    LIMIT 1;
    
    IF current_rate IS NULL THEN
        RAISE EXCEPTION 'No exchange rate found for % -> %', p_base, p_quote;
    END IF;
    
    RETURN current_rate;
END;
$$ LANGUAGE plpgsql;

-- Function to calculate available balance
CREATE OR REPLACE FUNCTION calculate_available_balance(p_account_id BIGINT)
RETURNS BIGINT AS $$
DECLARE
    current_balance BIGINT;
    total_holds BIGINT;
BEGIN
    SELECT balance INTO current_balance
    FROM balances
    WHERE account_id = p_account_id;
    
    SELECT COALESCE(SUM(hold_amount), 0) INTO total_holds
    FROM transaction_holds
    WHERE account_id = p_account_id AND released = false;
    
    RETURN GREATEST(current_balance - total_holds, 0);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION calculate_available_balance IS 'Calculate available balance after holds';

-- ===============================
-- TRIGGERS
-- ===============================

-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply to relevant tables
CREATE TRIGGER trg_currencies_updated_at BEFORE UPDATE ON currencies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_accounts_updated_at BEFORE UPDATE ON accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();


-- ===============================
-- INITIAL DATA SETUP
-- ===============================

-- Insert supported currencies
INSERT INTO currencies (code, name, symbol, decimals, is_fiat, is_active) VALUES
('USD', 'United States Dollar', '$', 2, true, true),
('USDT', 'Tether USD', '₮', 2, false, true),
('BTC', 'Bitcoin', '₿', 8, false, true)
ON CONFLICT (code) DO NOTHING;

-- Insert system accounts
DO $$
DECLARE
    sys_liquidity_usd BIGINT;
    sys_liquidity_usdt BIGINT;
    sys_liquidity_btc BIGINT;
    sys_fee_usd BIGINT;
    sys_fee_usdt BIGINT;
    sys_fee_btc BIGINT;
BEGIN
    -- Liquidity accounts for each currency
    INSERT INTO accounts (owner_type, owner_id, currency, purpose, account_type, account_number)
    VALUES ('system', 'system', 'USD', 'liquidity', 'real', 'SYS-LIQ-USD')
    ON CONFLICT DO NOTHING
    RETURNING id INTO sys_liquidity_usd;
    
    INSERT INTO accounts (owner_type, owner_id, currency, purpose, account_type, account_number)
    VALUES ('system', 'system', 'USDT', 'liquidity', 'real', 'SYS-LIQ-USDT')
    ON CONFLICT DO NOTHING
    RETURNING id INTO sys_liquidity_usdt;
    
    INSERT INTO accounts (owner_type, owner_id, currency, purpose, account_type, account_number)
    VALUES ('system', 'system', 'BTC', 'liquidity', 'real', 'SYS-LIQ-BTC')
    ON CONFLICT DO NOTHING
    RETURNING id INTO sys_liquidity_btc;
    
    -- Fee collection accounts
    INSERT INTO accounts (owner_type, owner_id, currency, purpose, account_type, account_number)
    VALUES ('system', 'system', 'USD', 'fees', 'real', 'SYS-FEE-USD')
    ON CONFLICT DO NOTHING
    RETURNING id INTO sys_fee_usd;
    
    INSERT INTO accounts (owner_type, owner_id, currency, purpose, account_type, account_number)
    VALUES ('system', 'system', 'USDT', 'fees', 'real', 'SYS-FEE-USDT')
    ON CONFLICT DO NOTHING
    RETURNING id INTO sys_fee_usdt;
    
    INSERT INTO accounts (owner_type, owner_id, currency, purpose, account_type, account_number)
    VALUES ('system', 'system', 'BTC', 'fees', 'real', 'SYS-FEE-BTC')
    ON CONFLICT DO NOTHING
    RETURNING id INTO sys_fee_btc;
    
    -- Initialize balances for system accounts
    INSERT INTO balances (account_id, balance, available_balance)
    SELECT id, 0, 0 FROM accounts WHERE owner_type = 'system'
    ON CONFLICT (account_id) DO NOTHING;
END $$;

-- Insert default fee rules
INSERT INTO transaction_fee_rules (
    rule_name, transaction_type, fee_type, calculation_method, 
    fee_value, min_fee, is_active, priority
) VALUES
('Standard Deposit Fee', 'deposit', 'platform', 'percentage', 0.001, 100, true, 1),
('Standard Withdrawal Fee', 'withdrawal', 'platform', 'fixed', 500, NULL, true, 1),
('Standard Conversion Fee', 'conversion', 'conversion', 'percentage', 0.005, 100, true, 1),
('Agent Commission', 'trade', 'agent_commission', 'percentage', 0.002, NULL, true, 1)
ON CONFLICT DO NOTHING;

COMMIT;

-- ===============================
-- POST-DEPLOYMENT OPTIMIZATION
-- ===============================

-- Analyze tables for query planner
ANALYZE currencies;
ANALYZE accounts;
ANALYZE balances;
ANALYZE journals;
ANALYZE fx_receipts;
ANALYZE ledgers;
ANALYZE transaction_fee_rules;

-- Refresh materialized views (run after initial data load)
-- REFRESH MATERIALIZED VIEW CONCURRENTLY system_holdings;
-- REFRESH MATERIALIZED VIEW CONCURRENTLY daily_transaction_volume;

-- ===============================
-- HOUSEKEEPING & MAINTENANCE NOTES
-- ===============================
-- 
-- DEPLOYMENT CHECKLIST:
-- □ Configure PostgreSQL max_connections (500+ for high concurrency)
-- □ Set shared_buffers to 25% of RAM
-- □ Configure effective_cache_size to 50-75% of RAM
-- □ Enable pg_stat_statements for query monitoring
-- □ Set up connection pooling (PgBouncer/PgPool) - transaction mode recommended
-- □ Configure WAL archiving for point-in-time recovery
-- □ Set up streaming replication (1+ read replicas)
-- □ Enable SSL/TLS for all connections
-- □ Configure firewall rules (only application servers can connect)
-- □ Set up automated backups (pg_dump + continuous archiving)
-- □ Configure monitoring (Prometheus + Grafana or DataDog)
--
-- SCALING STRATEGY:
-- 1. Phase 1 (0-100k users): Single primary + 1 read replica
-- 2. Phase 2 (100k-500k users): Primary + 2-3 read replicas, connection pooling
-- 3. Phase 3 (500k-1M users): Consider TimescaleDB multi-node or Citus for sharding
-- 4. Phase 4 (1M+ users): Full sharding by user_id/account_id ranges
--
-- PARTITION STRATEGY:
-- - fx_receipts: Time-based (weekly chunks) + compression after 90 days
-- - ledgers: Time + space partitioning (by account_id) for optimal query performance
-- - audit_log: Time-based (monthly chunks) + compression after 180 days
--
-- BACKUP STRATEGY:
-- - Full backup: Daily (off-peak hours)
-- - Incremental backup: Every 6 hours
-- - WAL archiving: Continuous
-- - Retention: 30 days online, 7 years archive (compliance)
-- - Test restore procedure: Monthly
--
-- MONITORING METRICS:
-- - Query performance: p50, p95, p99 latency
-- - Database size and growth rate
-- - Active connections and connection pool saturation
-- - Cache hit ratio (should be >95%)
-- - Replication lag (should be <100ms)
-- - Chunk health (compression status, chunk size)
-- - Slow queries (>100ms should be investigated)
-- - Lock contention and wait events
--
-- SECURITY BEST PRACTICES:
-- - Use RLS (Row Level Security) for multi-tenant isolation if needed
-- - Encrypt sensitive columns (email, external IDs) at rest
-- - Use parameterized queries only (prevent SQL injection)
-- - Rotate database passwords quarterly
-- - Audit all DDL changes
-- - Restrict superuser access (use specific roles)
-- - Enable pgAudit extension for compliance logging
--
-- DATA RETENTION:
-- - Ledgers: 7 years (financial compliance - Sarbanes-Oxley, GDPR)
-- - Receipts: 7 years (same as ledgers)
-- - Audit logs: 7 years (compliance)
-- - Compressed data: Can be moved to cold storage after 2 years
-- - Add retention policy (example):
--   SELECT add_retention_policy('fx_receipts', INTERVAL '7 years');
--   SELECT add_retention_policy('ledgers', INTERVAL '7 years');
--
-- PERFORMANCE TUNING:
-- - Vacuum: Auto-vacuum should be aggressive for high-write tables
--   ALTER TABLE balances SET (autovacuum_vacuum_scale_factor = 0.01);
-- - Statistics: Increase statistics target for frequently queried columns
--   ALTER TABLE accounts ALTER COLUMN owner_id SET STATISTICS 1000;
-- - Indexes: Monitor unused indexes monthly (drop if not used)
-- - Query optimization: Use EXPLAIN ANALYZE for slow queries
-- - Connection pooling: Use transaction mode for better performance
--
-- DISASTER RECOVERY:
-- - RTO (Recovery Time Objective): 1 hour
-- - RPO (Recovery Point Objective): 5 minutes
-- - Failover: Automated with streaming replication
-- - Backup location: Multi-region cloud storage
-- - DR drill: Quarterly
--
-- COMPLIANCE NOTES:
-- - GDPR: Implement right to erasure (account deletion workflow)
-- - PCI-DSS: Not storing card data, but secure transaction records
-- - AML/KYC: Audit logs track all account activities
-- - Data residency: Ensure backups comply with regional regulations
--
-- MAINTENANCE TASKS:
-- - Daily: Monitor alerts, check replication lag
-- - Weekly: Review slow query log, check disk space
-- - Monthly: Analyze table statistics, review unused indexes
-- - Quarterly: Test backup restoration, security audit
-- - Annually: Capacity planning review, architecture review
--
-- UPGRADE PATH:
-- - Always test on staging environment first
-- - Use blue-green deployment for zero-downtime upgrades
-- - Keep TimescaleDB version current (security patches)
-- - Plan for PostgreSQL major version upgrades annually
--
-- TROUBLESHOOTING COMMON ISSUES:
-- - High CPU: Check for missing indexes or bad query plans
-- - High I/O: Consider faster storage (NVMe SSDs) or more RAM
-- - Connection exhaustion: Increase max_connections or add pooling
-- - Replication lag: Check network bandwidth and disk I/O on replica
-- - Lock contention: Review long-running transactions
-- - Slow queries: Add indexes, rewrite queries, or partition tables
--
-- COST OPTIMIZATION:
-- - Use compression aggressively (can save 90% storage on old data)
-- - Archive old data to cheaper storage (S3/Glacier)
-- - Use read replicas for reporting queries (offload from primary)
-- - Monitor and kill idle connections
-- - Right-size instance based on actual metrics
--
-- ===============================