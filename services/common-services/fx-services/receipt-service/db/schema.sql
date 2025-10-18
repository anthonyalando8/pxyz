CREATE TABLE accounts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NULL,          -- optional, FK to users table
    type TEXT NOT NULL,           -- user, agent, partner, system
    label TEXT,                   -- finance label / department grouping
    currency VARCHAR(8) NOT NULL, -- default currency for this account
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

-- Indexes
CREATE UNIQUE INDEX idx_accounts_user_type_currency ON accounts(user_id, type, currency);


CREATE TABLE ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    related_account_id BIGINT NULL,   -- counterparty,
    type TEXT NOT NULL,               -- deposit, withdrawal, transfer, etc.
    coded_type TEXT,                  -- optional subtype (fee, cashback)
    amount NUMERIC(24,8) NOT NULL CHECK (amount != 0),
    currency VARCHAR(8) NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',  -- pending, success, failed
    running_balance NUMERIC(24,8),           -- running balance after this entry
    external_ref TEXT,                        -- bank txn id, blockchain tx, etc.
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

-- Indexes for fast queries
CREATE INDEX idx_ledger_account ON ledger_entries(account_id);
CREATE INDEX idx_ledger_created_at ON ledger_entries(created_at DESC);
CREATE INDEX idx_ledger_external_ref ON ledger_entries(external_ref);

CREATE TABLE account_balances (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id),
    currency VARCHAR(8) NOT NULL,
    balance NUMERIC(24,8) NOT NULL DEFAULT 0,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Unique constraint per account + currency
CREATE UNIQUE INDEX idx_account_balances_account_currency ON account_balances(account_id, currency);


SELECT a.id AS account_id, a.user_id, ab.balance, ab.currency
FROM accounts a
JOIN account_balances ab ON a.id = ab.account_id
WHERE a.type = 'user';


SELECT ab.currency, SUM(ab.balance) AS total_holding
FROM account_balances ab
JOIN accounts a ON a.id = ab.account_id
WHERE a.type = 'user'
GROUP BY ab.currency;

INSERT INTO account_balances(account_id, currency, balance, last_updated)
SELECT 
    account_id,
    currency,
    SUM(amount) AS balance,
    MAX(created_at) AS last_updated
FROM ledger_entries
WHERE status = 'success'
GROUP BY account_id, currency
ON CONFLICT (account_id) DO UPDATE 
    SET balance = EXCLUDED.balance,
        last_updated = EXCLUDED.last_updated;
