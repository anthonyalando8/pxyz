\c pxyz_fx;

BEGIN;

-- ===============================
-- Receipts
-- ===============================
CREATE TABLE fx_receipts (
    id                     BIGSERIAL PRIMARY KEY,
    code                   VARCHAR(64) NOT NULL UNIQUE,        -- receipt code (e.g., Unix timestamp + rand suffix)

    -- Creditor info
    creditor_account_id    BIGINT NOT NULL REFERENCES accounts(id),
    creditor_ledger_id     BIGINT NOT NULL REFERENCES ledgers(id),
    creditor_account_type  owner_type_enum NOT NULL,           -- system, partner, user
    creditor_status        TEXT NOT NULL DEFAULT 'pending',    -- pending, success, failed

    -- Debitor info
    debitor_account_id     BIGINT NOT NULL REFERENCES accounts(id),
    debitor_ledger_id      BIGINT NOT NULL REFERENCES ledgers(id),
    debitor_account_type   owner_type_enum NOT NULL,           -- system, partner, user
    debitor_status         TEXT NOT NULL DEFAULT 'pending',

    -- Transaction info
    type                   TEXT NOT NULL,                     -- deposit, withdrawal, transfer, conversion, admin_credit
    coded_type             TEXT,                              -- optional: subtype (ex: fee, cashback, promo)
    amount                 NUMERIC(24,8) NOT NULL CHECK (amount > 0),
    transaction_cost       NUMERIC(24,8) NOT NULL DEFAULT 0,  -- new field: transaction fee/cost
    currency               VARCHAR(8) NOT NULL REFERENCES currencies(code),
    external_ref           TEXT,                              -- optional: external reference (bank txn id, blockchain tx hash)
    status                 TEXT NOT NULL DEFAULT 'pending',   -- overall txn status: pending, success, failed, reversed

    -- Audit info
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ,
    created_by             TEXT DEFAULT 'system',
    reversed_at            TIMESTAMPTZ,
    reversed_by            TEXT,                              -- who reversed the txn (admin, system, etc.)

    -- Metadata
    metadata               JSONB
);

CREATE INDEX idx_fx_receipts_creditor_account ON fx_receipts (creditor_account_id);
CREATE INDEX idx_fx_receipts_debitor_account ON fx_receipts (debitor_account_id);
CREATE INDEX idx_fx_receipts_created_at ON fx_receipts (created_at DESC);
CREATE INDEX idx_fx_receipts_external_ref ON fx_receipts (external_ref);

COMMIT;
