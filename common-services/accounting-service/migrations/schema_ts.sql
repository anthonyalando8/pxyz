-- timescale_schema.sql
\c pxyz_fx;

BEGIN;

-- ===============================
-- Enable TimescaleDB extension
-- ===============================
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- ===============================
-- Prereqs (enums kept as-is)
-- ===============================
CREATE TYPE IF NOT EXISTS owner_type_enum AS ENUM ('system','partner','user');
CREATE TYPE IF NOT EXISTS account_purpose_enum AS ENUM (
  'liquidity','clearing','fees','wallet','escrow','settlement','revenue','contra'
);
CREATE TYPE IF NOT EXISTS account_type_enum AS ENUM ('real','demo');
CREATE TYPE IF NOT EXISTS dr_cr_enum AS ENUM ('DR','CR');

-- ===============================
-- Currencies
-- ===============================
CREATE TABLE IF NOT EXISTS currencies (
  code        VARCHAR(8) PRIMARY KEY,
  name        TEXT NOT NULL,
  decimals    SMALLINT NOT NULL DEFAULT 2 CHECK (decimals >= 0 AND decimals <= 18),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_currencies_name ON currencies (name);

-- ===============================
-- FX Rates
-- ===============================
CREATE TABLE IF NOT EXISTS fx_rates (
  id             BIGSERIAL PRIMARY KEY,
  base_currency  VARCHAR(8) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE,
  quote_currency VARCHAR(8) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE,
  rate           NUMERIC(32,18) NOT NULL CHECK (rate > 0),
  as_of          TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (base_currency, quote_currency, as_of)
);
CREATE INDEX IF NOT EXISTS idx_fx_rates_pair ON fx_rates (base_currency, quote_currency, as_of DESC);

-- ===============================
-- Accounts
-- ===============================
CREATE TABLE IF NOT EXISTS accounts (
  id             BIGSERIAL PRIMARY KEY,
  owner_type     owner_type_enum NOT NULL,
  owner_id       TEXT,
  currency       VARCHAR(8) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE,
  purpose        account_purpose_enum NOT NULL,
  account_type   account_type_enum NOT NULL DEFAULT 'real',
  is_active      BOOLEAN NOT NULL DEFAULT true,
  account_number TEXT NOT NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_account_owner_currency_purpose UNIQUE (owner_type, owner_id, currency, purpose, account_type),
  CONSTRAINT uq_accounts_account_number UNIQUE (account_number)
);
CREATE INDEX IF NOT EXISTS idx_accounts_owner ON accounts (owner_type, owner_id);

-- ===============================
-- Journals
-- ===============================
CREATE TABLE IF NOT EXISTS journals (
  id               BIGSERIAL PRIMARY KEY,
  external_ref     TEXT,
  idempotency_key  TEXT UNIQUE,
  description      TEXT,
  created_by_user  BIGINT,
  created_by_type  owner_type_enum,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_journals_created_at ON journals (created_at DESC);

-- ===============================
-- FX Receipts (hypertable)
-- ===============================
CREATE TABLE IF NOT EXISTS fx_receipts (
  id BIGSERIAL NOT NULL,
  code VARCHAR(64) NOT NULL UNIQUE,  -- enforce global uniqueness
  creditor_account_id BIGINT NOT NULL REFERENCES accounts(id),
  creditor_ledger_id BIGINT,
  creditor_account_type owner_type_enum NOT NULL,
  creditor_status TEXT NOT NULL DEFAULT 'pending',
  debitor_account_id BIGINT NOT NULL REFERENCES accounts(id),
  debitor_ledger_id BIGINT,
  debitor_account_type owner_type_enum NOT NULL,
  debitor_status TEXT NOT NULL DEFAULT 'pending',
  type TEXT NOT NULL,
  coded_type TEXT,
  amount NUMERIC(24,8) NOT NULL CHECK (amount > 0),
  transaction_cost NUMERIC(24,8) NOT NULL DEFAULT 0,
  currency VARCHAR(8) NOT NULL REFERENCES currencies(code),
  external_ref TEXT,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ,
  created_by TEXT DEFAULT 'system',
  reversed_at TIMESTAMPTZ,
  reversed_by TEXT,
  metadata JSONB,
  account_partition_key BIGINT GENERATED ALWAYS AS (LEAST(creditor_account_id, debitor_account_id)) STORED,
  PRIMARY KEY (id, created_at)
);

-- Timescale hypertable
SELECT
  create_hypertable(
    'fx_receipts',
    'created_at',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 month',
    number_partitions => 64,
    partitioning_column => 'account_partition_key'
  );

-- Indexes
CREATE INDEX IF NOT EXISTS idx_fx_receipts_account_created_at ON fx_receipts (account_partition_key, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_fx_receipts_creditor_account ON fx_receipts (creditor_account_id);
CREATE INDEX IF NOT EXISTS idx_fx_receipts_debitor_account ON fx_receipts (debitor_account_id);
CREATE INDEX IF NOT EXISTS idx_fx_receipts_code ON fx_receipts (code);

-- Compression policy
ALTER TABLE fx_receipts SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'account_partition_key',
  timescaledb.compress_orderby = 'created_at DESC'
);
SELECT add_compression_policy('fx_receipts', INTERVAL '30 days');

-- ===============================
-- Ledgers (hypertable)
-- ===============================
CREATE TABLE IF NOT EXISTS ledgers (
  id          BIGSERIAL NOT NULL,
  journal_id  BIGINT NOT NULL REFERENCES journals(id) ON DELETE CASCADE,
  account_id  BIGINT NOT NULL REFERENCES accounts(id),
  amount      NUMERIC(24,8) NOT NULL CHECK (amount > 0),
  dr_cr       dr_cr_enum NOT NULL,
  currency    VARCHAR(8) NOT NULL REFERENCES currencies(code),
  receipt_id  BIGINT REFERENCES fx_receipts(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id, created_at)
);

SELECT
  create_hypertable(
    'ledgers',
    'created_at',
    if_not_exists => TRUE,
    chunk_time_interval => INTERVAL '1 month',
    number_partitions => 64,
    partitioning_column => 'account_id'
  );

CREATE INDEX IF NOT EXISTS idx_ledgers_account_created_at ON ledgers (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ledgers_journal_id ON ledgers (journal_id);
CREATE INDEX IF NOT EXISTS idx_ledgers_receipt_id ON ledgers (receipt_id);

ALTER TABLE ledgers SET (
  timescaledb.compress,
  timescaledb.compress_segmentby = 'account_id',
  timescaledb.compress_orderby = 'created_at DESC'
);
SELECT add_compression_policy('ledgers', INTERVAL '30 days');


CREATE TABLE balances (
  account_id  BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  balance     NUMERIC(24,8) NOT NULL DEFAULT 0,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_balances_balance ON balances (balance);

CREATE TABLE transaction_fee_rules (
  id               BIGSERIAL PRIMARY KEY,
  transaction_type VARCHAR(64) NOT NULL,  -- 'transfer','conversion','withdrawal'
  source_currency  VARCHAR(8),
  target_currency  VARCHAR(8),
  fee_type         TEXT NOT NULL,         -- 'percentage' or 'fixed'
  fee_value        NUMERIC(24,8) NOT NULL,
  min_fee          NUMERIC(24,8),
  max_fee          NUMERIC(24,8),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Transaction Fee Rules
CREATE UNIQUE INDEX idx_fee_rules_lookup
  ON transaction_fee_rules (transaction_type, source_currency, target_currency);

-- ===============================
-- Applied Transaction Fees
-- ===============================
CREATE TABLE transaction_fees (
  id            BIGSERIAL PRIMARY KEY,
  receipt_id    BIGINT NOT NULL REFERENCES fx_receipts(id),
  fee_rule_id   BIGINT REFERENCES transaction_fee_rules(id),
  fee_type      TEXT NOT NULL,            -- 'platform','network','partner'
  amount        NUMERIC(24,8) NOT NULL,
  currency      VARCHAR(8) NOT NULL,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transaction_fees_receipt_id
  ON transaction_fees (receipt_id);

ALTER TABLE transaction_fees
  ADD CONSTRAINT fk_transaction_fees_currency FOREIGN KEY (currency) REFERENCES currencies(code);

-- Prevent duplicate fee types per receipt
ALTER TABLE transaction_fees
  ADD CONSTRAINT uniq_fee_per_receipt UNIQUE (receipt_id, fee_type);
-- ===============================
-- Views
-- ===============================
CREATE OR REPLACE VIEW account_ledgers AS
SELECT id, journal_id, account_id, amount, dr_cr, currency, receipt_id, created_at
FROM ledgers;

CREATE OR REPLACE VIEW account_receipts AS
SELECT id, code, creditor_account_id, debitor_account_id, type, amount, currency, status, created_at
FROM fx_receipts;

CREATE MATERIALIZED VIEW system_holdings AS
SELECT 
    a.owner_type,
    a.currency,
    SUM(b.balance) AS total_balance
FROM balances b
JOIN accounts a ON b.account_id = a.id
GROUP BY a.owner_type, a.currency;

CREATE INDEX idx_system_holdings_currency 
    ON system_holdings (currency, owner_type);


COMMIT;

-- ===============================
-- Housekeeping / Maintenance Notes
-- ===============================
-- 1) Timescale hypertables:
--    - fx_receipts and ledgers are hypertables, partitioned by time + account key.
--    - Timescale automatically manages chunk creation/deletion.
--    - Monitor chunk size regularly: aim for 100MB–1GB per chunk for optimal performance.
--
-- 2) Compression:
--    - Currently set to compress data older than 30 days.
--    - Compressed chunks are read-optimized but slower to update.
--    - If you need faster lookups on old data, consider adjusting compress_orderby/segmentby.
--    - If you need regulatory retention, adjust or add retention policies (see #3).
--
-- 3) Retention:
--    - Example: Drop data older than 7 years (common financial compliance window).
--      SELECT add_retention_policy('fx_receipts', INTERVAL '7 years');
--      SELECT add_retention_policy('ledgers', INTERVAL '7 years');
--    - Adjust as per your regulatory and business requirements.
--    - If indefinite history is required, omit retention policies but monitor storage.
--
-- 4) NUMERIC vs BIGINT:
--    - NUMERIC(24,8) is safe but slower than BIGINT.
--    - For higher throughput, consider storing balances and amounts in atomic units (e.g., cents) as BIGINT.
--    - Migration path: keep NUMERIC now, switch later if performance is a bottleneck.
--
-- 5) Balances:
--    - Balance table is designed as "cached materialized state".
--    - It must be updated transactionally in sync with ledgers, ideally via application logic.
--    - Consider adding a trigger to auto-update balances from ledgers if you want DB-level consistency.
--
-- 6) Indexing strategy:
--    - Indexes on account_id + created_at support transaction history lookups.
--    - Indexes on receipt_id ensure fast joins between receipts, ledgers, and fees.
--    - Avoid over-indexing; each extra index adds write overhead.
--
-- 7) Uniqueness constraints:
--    - fx_receipts.code is globally unique, enforced at DB level.
--    - transaction_fee_rules has a unique composite index to prevent duplicate fee configs.
--    - transaction_fees enforces one fee_type per receipt.
--
-- 8) Journals:
--    - idempotency_key is UNIQUE: protects against double-posting transactions.
--    - Application must enforce idempotent writes when processing external requests.
--
-- 9) Growth considerations:
--    - Expect fx_receipts and ledgers to grow into billions of rows.
--    - Timescale chunking will keep queries scoped and efficient.
--    - If throughput becomes extreme (e.g., >100k writes/sec), consider:
--        * Sharding at application layer (by account_id).
--        * Using Timescale Multi-node (clustered).
--        * Moving to Citus or another distributed Postgres.
--
-- 10) Views:
--    - account_ledgers and account_receipts are convenience views for API-level reads.
--    - Extend with joins or materialized views if you need richer history queries.
--
-- 11) Monitoring:
--    - Enable TimescaleDB telemetry for chunk health and compression stats.
--    - Track slow queries with pg_stat_statements.
--    - Consider periodic VACUUM / ANALYZE or autovacuum tuning for high-ingest workloads.
