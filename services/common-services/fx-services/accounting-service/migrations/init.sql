\c pxyz_fx;

BEGIN;

-- ===============================
-- ENUMs
-- ===============================
CREATE TYPE owner_type_enum AS ENUM ('system','partner','user');
CREATE TYPE account_purpose_enum AS ENUM (
  'liquidity','clearing','fees','wallet','escrow','settlement','revenue','contra'
);
CREATE TYPE account_type_enum AS ENUM ('real','demo');
CREATE TYPE dr_cr_enum AS ENUM ('DR','CR');

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

-- =======================================
-- Parent Table (Partitioned)
-- =======================================
CREATE TABLE ledgers (
  id          BIGSERIAL PRIMARY KEY,
  journal_id  BIGINT NOT NULL REFERENCES journals(id) ON DELETE CASCADE,
  account_id  BIGINT NOT NULL REFERENCES accounts(id),
  amount      NUMERIC(24,8) NOT NULL CHECK (amount > 0),
  dr_cr       dr_cr_enum NOT NULL,
  currency    VARCHAR(8) NOT NULL REFERENCES currencies(code),
  receipt_id  BIGINT REFERENCES fx_receipts(id),
  balance     NUMERIC(24,8) NOT NULL,  -- running balance after this entry
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
) PARTITION BY HASH (account_id);

-- =======================================
-- Child Partitions (16 buckets)
-- =======================================
DO $$
DECLARE
    i INT;
BEGIN
    FOR i IN 0..15 LOOP
        EXECUTE format(
            'CREATE TABLE ledgers_p%s PARTITION OF ledgers
             FOR VALUES WITH (MODULUS 16, REMAINDER %s);',
            i, i
        );
    END LOOP;
END$$;


-- =======================================
-- Indexes (applied to all partitions)
-- =======================================
-- Each partition inherits the primary key, foreign keys, and constraints
-- but indexes must be defined explicitly for each partition.

-- Index for fast lookups by account_id + created_at
DO $$
DECLARE
    i INT;
BEGIN
    FOR i IN 0..15 LOOP
        EXECUTE format(
          'CREATE INDEX idx_ledgers_p%s_account_created_at
           ON ledgers_p%s (account_id, created_at DESC)',
          i, i
        );
    END LOOP;
END$$;

-- Index for journal_id
DO $$
DECLARE
    i INT;
BEGIN
    FOR i IN 0..15 LOOP
        EXECUTE format(
          'CREATE INDEX idx_ledgers_p%s_journal_id
           ON ledgers_p%s (journal_id)',
          i, i
        );
    END LOOP;
END$$;


-- ===============================
-- Receipts (external transaction references)
-- ===============================
CREATE TABLE fx_receipts (
  id BIGSERIAL PRIMARY KEY,
  code VARCHAR(64) NOT NULL UNIQUE,
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

  -- generated column for partitioning
  account_partition_key BIGINT GENERATED ALWAYS AS (
    LEAST(creditor_account_id, debitor_account_id)
  ) STORED
) PARTITION BY HASH (account_partition_key);

-- Example: 16 hash partitions
DO $$
DECLARE
  i INT;
BEGIN
  FOR i IN 0..15 LOOP
    EXECUTE format(
      'CREATE TABLE fx_receipts_p%s PARTITION OF fx_receipts
       FOR VALUES WITH (MODULUS 16, REMAINDER %s);',
      i, i
    );
  END LOOP;
END$$;

-- Indexes per partition
DO $$
DECLARE
  i INT;
BEGIN
  FOR i IN 0..15 LOOP
    EXECUTE format(
      'CREATE INDEX idx_fx_receipts_p%s_account_created_at
       ON fx_receipts_p%s (account_partition_key, created_at DESC);',
      i, i
    );
  END LOOP;
END$$;


-- ===============================
-- Balances (fast cache, app-managed)
-- ===============================
CREATE TABLE balances (
  account_id  BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  balance     NUMERIC(24,8) NOT NULL DEFAULT 0,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_balances_balance ON balances (balance);

-- ===============================
-- Transaction Fee Rules
-- ===============================
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

-- ===============================
-- Integrity Triggers (lightweight only)
-- ===============================

-- Ensure currency consistency between account and ledger
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

DROP TRIGGER IF EXISTS ledgers_currency_check ON ledgers;
CREATE TRIGGER ledgers_currency_check
  BEFORE INSERT ON ledgers
  FOR EACH ROW EXECUTE FUNCTION trg_ledgers_check_currency();

COMMIT;