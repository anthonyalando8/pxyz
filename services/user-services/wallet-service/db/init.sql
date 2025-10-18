\c pxyz;

CREATE TABLE IF NOT EXISTS wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id TEXT NOT NULL,
    currency VARCHAR(10) NOT NULL, -- e.g., BTC, ETH, BTCT
    balance NUMERIC(20, 8) DEFAULT 0,
    available NUMERIC(20, 8) DEFAULT 0, -- for withdrawals
    locked NUMERIC(20, 8) DEFAULT 0,    -- in orders, pending, etc.
    type VARCHAR(20) NOT NULL, -- e.g., 'crypto', 'fiat'
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_user_currency ON wallets(user_id, currency);

CREATE TABLE wallet_transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID NOT NULL,
    user_id TEXT NOT NULL,
    currency VARCHAR(10) NOT NULL,
    tx_status VARCHAR(20) NOT NULL, -- pending, completed, failed
    amount NUMERIC(20, 8) NOT NULL,
    tx_type VARCHAR(20) NOT NULL, -- deposit, withdrawal, trade, transfer
    description TEXT,
    ref_id UUID, -- optional link to external transaction/order
    created_at TIMESTAMP DEFAULT NOW()
);
