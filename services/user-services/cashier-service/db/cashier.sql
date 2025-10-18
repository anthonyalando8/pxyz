\c pxyz;

BEGIN;

-- ===============================
-- ENUMs
-- ===============================
CREATE TYPE owner_type_enum AS ENUM ('system','partner','user');
CREATE TYPE account_purpose_enum AS ENUM ('liquidity','clearing','fees','wallet','escrow','settlement','revenue','contra');
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
  id          BIGSERIAL PRIMARY KEY,
  owner_type  owner_type_enum NOT NULL,
  owner_id    TEXT, -- support user/partner IDs
  currency    VARCHAR(8) NOT NULL REFERENCES currencies(code) ON UPDATE CASCADE,
  purpose     account_purpose_enum NOT NULL,
  is_active   BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_account_owner_currency_purpose UNIQUE (owner_type, owner_id, currency, purpose)
);
CREATE INDEX idx_accounts_owner ON accounts (owner_type, owner_id);

-- ===============================
-- Journals
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
-- Receipts
-- ===============================
CREATE TABLE receipts (
    id           BIGSERIAL PRIMARY KEY,
    code         VARCHAR(15) NOT NULL UNIQUE, -- TIJ5LW4VDT format
    journal_id   BIGINT NOT NULL REFERENCES journals(id) ON DELETE CASCADE,
    account_id   BIGINT NOT NULL REFERENCES accounts(id),
    account_type owner_type_enum NOT NULL,
    type         TEXT NOT NULL, -- deposit, withdrawal, admin_credit
    amount       NUMERIC(24,8) NOT NULL,
    currency     VARCHAR(8) NOT NULL REFERENCES currencies(code),
    status       TEXT NOT NULL DEFAULT 'pending', -- pending, success, failed
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_receipts_account ON receipts (account_id);
CREATE INDEX idx_receipts_created_at ON receipts (created_at DESC);

-- ===============================
-- Postings (Partitioned by month)
-- ===============================
CREATE TABLE postings (
  id          BIGSERIAL NOT NULL,
  journal_id  BIGINT NOT NULL REFERENCES journals(id) ON DELETE CASCADE,
  account_id  BIGINT NOT NULL REFERENCES accounts(id),
  amount      NUMERIC(24,8) NOT NULL CHECK (amount > 0),
  dr_cr       dr_cr_enum NOT NULL,
  currency    VARCHAR(8) NOT NULL REFERENCES currencies(code),
  receipt_id  BIGINT REFERENCES receipts(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id)
) PARTITION BY RANGE (created_at);

-- Example partition
CREATE TABLE postings_202509 PARTITION OF postings
  FOR VALUES FROM ('2025-09-01 00:00:00+00') TO ('2025-10-01 00:00:00+00');

CREATE INDEX idx_postings_account_created_at ON postings (account_id, created_at DESC);
CREATE INDEX idx_postings_journal_id ON postings (journal_id);

-- ===============================
-- Balances
-- ===============================
CREATE TABLE balances (
  account_id  BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
  balance     NUMERIC(24,8) NOT NULL DEFAULT 0,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_balances_balance ON balances (balance);

-- ===============================
-- Currency Consistency Trigger
-- ===============================
CREATE OR REPLACE FUNCTION trg_postings_check_currency() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
  acct_currency VARCHAR(8);
BEGIN
  SELECT currency INTO acct_currency FROM accounts WHERE id = NEW.account_id;
  IF acct_currency IS DISTINCT FROM NEW.currency THEN
    RAISE EXCEPTION 'Posting currency (%) does not match account currency (%)', NEW.currency, acct_currency;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER postings_currency_check
  BEFORE INSERT ON postings
  FOR EACH ROW EXECUTE FUNCTION trg_postings_check_currency();

-- ===============================
-- Statements View
-- ===============================
CREATE OR REPLACE VIEW account_statements AS
SELECT
  p.id AS posting_id,
  p.journal_id,
  r.code AS receipt_code,
  p.account_id,
  p.dr_cr,
  p.amount,
  p.currency,
  p.created_at,
  j.description,
  j.external_ref
FROM postings p
JOIN journals j ON j.id = p.journal_id
LEFT JOIN receipts r ON r.journal_id = j.id;

COMMIT;
