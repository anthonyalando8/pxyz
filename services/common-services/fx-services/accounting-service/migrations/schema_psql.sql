\c pxyz_fx;

BEGIN;

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
CREATE TABLE currencies (
  code        VARCHAR(8) PRIMARY KEY,
  name        TEXT NOT NULL,
  decimals    SMALLINT NOT NULL DEFAULT 2 CHECK (decimals >= 0 AND decimals <= 18),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_currencies_name ON currencies (name);

-- ===============================
-- FX Rates
-- ===============================
CREATE TABLE fx_rates (
  id             BIGSERIAL PRIMARY KEY,
  base_currency  VARCHAR(8) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE,
  quote_currency VARCHAR(8) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE,
  rate           NUMERIC(32,18) NOT NULL CHECK (rate > 0),
  as_of          TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (base_currency, quote_currency, as_of)
);
CREATE INDEX idx_fx_rates_pair ON fx_rates (base_currency, quote_currency, as_of DESC);

-- ===============================
-- Accounts
-- ===============================
CREATE TABLE accounts (
  id             BIGSERIAL PRIMARY KEY,
  owner_type     owner_type_enum NOT NULL,
  owner_id       TEXT, -- external reference (user ID, partner ID, system name)
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

CREATE INDEX idx_accounts_owner ON accounts (owner_type, owner_id);

-- ===============================
-- Journals (group of ledger lines)
-- ===============================
CREATE TABLE journals (
  id               BIGSERIAL PRIMARY KEY,
  external_ref     TEXT,
  idempotency_key  TEXT UNIQUE,
  description      TEXT,
  created_by_user  BIGINT,
  created_by_type  owner_type_enum,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_journals_created_at ON journals (created_at DESC);

-- ===============================
-- LEDGERS (hybrid partitioning: RANGE by month -> subpartition HASH by account_id)
-- ===============================
-- Parent table partitioned by range (created_at)
CREATE TABLE IF NOT EXISTS ledgers (
  id          BIGSERIAL NOT NULL,
  journal_id  BIGINT NOT NULL REFERENCES journals(id) ON DELETE CASCADE,
  account_id  BIGINT NOT NULL REFERENCES accounts(id),
  amount      NUMERIC(24,8) NOT NULL CHECK (amount > 0),
  dr_cr       dr_cr_enum NOT NULL,
  currency    VARCHAR(8) NOT NULL REFERENCES currencies(code),
  receipt_id  BIGINT REFERENCES fx_receipts(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- Primary key must include partition key (created_at) for range-partitioned tables
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Helper to create one monthly partition with N hash subpartitions
CREATE OR REPLACE FUNCTION create_month_partitions_for_ledgers(year INT, month INT, num_buckets INT DEFAULT 64)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  start_date DATE := make_date(year, month, 1);
  end_date   DATE := (start_date + INTERVAL '1 month')::date;
  i INT;
  parent_name TEXT := format('ledgers_y%04d_m%02d', year, month);
BEGIN
  -- Create monthly parent partition (which itself will be partitioned by hash)
  EXECUTE format(
    'CREATE TABLE IF NOT EXISTS %I PARTITION OF ledgers
     FOR VALUES FROM ( %L ) TO ( %L ) PARTITION BY HASH (account_id);',
     parent_name, start_date::text, end_date::text
  );

  -- Create hash subpartitions under the monthly parent
  FOR i IN 0..(num_buckets-1) LOOP
    EXECUTE format(
      'CREATE TABLE IF NOT EXISTS %I_p%s PARTITION OF %I
       FOR VALUES WITH (MODULUS %s, REMAINDER %s);',
      parent_name, i, parent_name, num_buckets::text, i
    );
    -- Create indexes on each subpartition
    EXECUTE format(
      'CREATE INDEX IF NOT EXISTS idx_%I_p%s_account_created_at ON %I_p%s (account_id, created_at DESC);',
      parent_name, i, parent_name, i
    );
    EXECUTE format(
      'CREATE INDEX IF NOT EXISTS idx_%I_p%s_journal_id ON %I_p%s (journal_id);',
      parent_name, i, parent_name, i
    );
  END LOOP;
END;
$$;

-- Example: create partitions for current month (adjust year/month)
-- SELECT create_month_partitions_for_ledgers(2025, 09, 64);
-- (call this once per month in a job)

-- ===============================
-- FX_RECEIPTS (hybrid partitioning: RANGE by month -> subpartition HASH by account_partition_key)
-- ===============================
-- We'll add a small helper table for global uniqueness of codes (recommended).
CREATE TABLE IF NOT EXISTS receipt_codes (
  code VARCHAR(64) PRIMARY KEY, -- enforces global uniqueness
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Parent receipts table
CREATE TABLE IF NOT EXISTS fx_receipts (
  id BIGSERIAL NOT NULL,
  code VARCHAR(64) NOT NULL, -- uniqueness enforced via receipt_codes table
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
  -- generated partition key so receipts touching an account are in the same hash bucket
  account_partition_key BIGINT GENERATED ALWAYS AS (LEAST(creditor_account_id, debitor_account_id)) STORED,
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Helper to create monthly partitions for receipts (with num_buckets hash subpartitions)
CREATE OR REPLACE FUNCTION create_month_partitions_for_receipts(year INT, month INT, num_buckets INT DEFAULT 64)
RETURNS void LANGUAGE plpgsql AS $$
DECLARE
  start_date DATE := make_date(year, month, 1);
  end_date   DATE := (start_date + INTERVAL '1 month')::date;
  i INT;
  parent_name TEXT := format('fx_receipts_y%04d_m%02d', year, month);
BEGIN
  EXECUTE format(
    'CREATE TABLE IF NOT EXISTS %I PARTITION OF fx_receipts
     FOR VALUES FROM ( %L ) TO ( %L ) PARTITION BY HASH (account_partition_key);',
     parent_name, start_date::text, end_date::text
  );

  FOR i IN 0..(num_buckets-1) LOOP
    EXECUTE format(
      'CREATE TABLE IF NOT EXISTS %I_p%s PARTITION OF %I
       FOR VALUES WITH (MODULUS %s, REMAINDER %s);',
      parent_name, i, parent_name, num_buckets::text, i
    );

    -- Indexes on each subpartition
    EXECUTE format(
      'CREATE INDEX IF NOT EXISTS idx_%I_p%s_account_created_at ON %I_p%s (account_partition_key, created_at DESC);',
      parent_name, i, parent_name, i
    );
    EXECUTE format(
      'CREATE INDEX IF NOT EXISTS idx_%I_p%s_creditor_account ON %I_p%s (creditor_account_id);',
      parent_name, i, parent_name, i
    );
    EXECUTE format(
      'CREATE INDEX IF NOT EXISTS idx_%I_p%s_debitor_account ON %I_p%s (debitor_account_id);',
      parent_name, i, parent_name, i
    );
  END LOOP;
END;
$$;

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
  
-- Example: create partitions for current month (adjust year/month)
-- SELECT create_month_partitions_for_receipts(2025, 09, 64);
-- (schedule to run monthly ahead of time)

-- ===============================
-- Indexes / supporting objects (global)
-- ===============================
-- For ledgers and receipts, most indexes are per-subpartition. We can create some light global indexes or views if needed.

-- Create a view to simplify account-scoped history queries (reads from partitioned table)
CREATE OR REPLACE VIEW account_ledgers AS
SELECT id, journal_id, account_id, amount, dr_cr, currency, receipt_id, created_at
FROM ledgers;

CREATE OR REPLACE VIEW account_receipts AS
SELECT id, code, creditor_account_id, debitor_account_id, type, amount, currency, status, created_at
FROM fx_receipts;

-- ===============================
-- Lightweight integrity trigger for ledgers (currency check)
-- ===============================
CREATE OR REPLACE FUNCTION trg_ledgers_check_currency() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
  acct_currency VARCHAR(8);
BEGIN
  SELECT currency INTO acct_currency FROM accounts WHERE id = NEW.account_id;
  IF acct_currency IS DISTINCT FROM NEW.currency THEN
    RAISE EXCEPTION 'Ledger currency (%) does not match account currency (%)', NEW.currency, acct_currency;
  END IF;
  RETURN NEW;
END;
$$;

-- Attach trigger to parent (it will fire for inserts routed to subpartitions)
DROP TRIGGER IF EXISTS ledgers_currency_check ON ledgers;
CREATE TRIGGER ledgers_currency_check
  BEFORE INSERT ON ledgers
  FOR EACH ROW EXECUTE FUNCTION trg_ledgers_check_currency();

-- ===============================
-- Helper: enforce global uniqueness of receipt.code
-- ===============================
-- App should insert into receipt_codes(code) before inserting fx_receipts row to guarantee global uniqueness.
-- Optionally add a trigger on fx_receipts to insert into receipt_codes automatically (with proper error handling).
CREATE OR REPLACE FUNCTION trg_fx_receipts_ensure_code() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  -- try to insert into receipt_codes; if exists, raise unique violation
  INSERT INTO receipt_codes(code) VALUES (NEW.code);
  RETURN NEW;
EXCEPTION WHEN unique_violation THEN
  RAISE EXCEPTION 'receipt code % already exists', NEW.code;
END;
$$;

-- Attach to parent so it runs on insert
DROP TRIGGER IF EXISTS fx_receipts_code_check ON fx_receipts;
CREATE TRIGGER fx_receipts_code_check
  BEFORE INSERT ON fx_receipts
  FOR EACH ROW EXECUTE FUNCTION trg_fx_receipts_ensure_code();

COMMIT;
