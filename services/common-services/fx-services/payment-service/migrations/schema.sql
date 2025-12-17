-- migrations/001_initial_schema.sql

\c pxyz_fx;

BEGIN;

-- ===============================
-- ENUMS
-- ===============================

CREATE TYPE payment_provider_enum AS ENUM ('mpesa', 'bank', 'card', 'paypal');
CREATE TYPE payment_type_enum AS ENUM ('deposit', 'withdrawal');
CREATE TYPE payment_status_enum AS ENUM ('pending', 'processing', 'completed', 'failed', 'cancelled');
CREATE TYPE transaction_status_enum AS ENUM ('initiated', 'sent', 'callback_received', 'verified', 'completed', 'failed');

-- ===============================
-- PAYMENTS TABLE
-- ===============================

CREATE TABLE payments (
    id BIGSERIAL PRIMARY KEY,
    payment_ref TEXT UNIQUE NOT NULL,           -- Internal reference (e.g., PAY-20250101-ABC123)
    partner_id TEXT NOT NULL,                    -- Partner external ID
    partner_tx_ref TEXT NOT NULL,                -- Partner's transaction reference
    
    -- Payment details
    provider payment_provider_enum NOT NULL,
    payment_type payment_type_enum NOT NULL,
    amount NUMERIC(20,2) NOT NULL,
    currency TEXT NOT NULL DEFAULT 'KES',
    
    -- User/Account info
    user_id TEXT NOT NULL,                       -- User external ID
    account_number TEXT,                         -- Account to credit/debit
    phone_number TEXT,                           -- For M-Pesa
    bank_account TEXT,                           -- For bank transfers
    
    -- Status tracking
    status payment_status_enum NOT NULL DEFAULT 'pending',
    provider_reference TEXT,                     -- Provider's reference (e.g., M-Pesa transaction ID)
    
    -- Metadata
    description TEXT,
    metadata JSONB,
    
    -- Callback tracking
    callback_received BOOLEAN DEFAULT FALSE,
    callback_data JSONB,
    callback_at TIMESTAMPTZ,
    
    -- Partner notification
    partner_notified BOOLEAN DEFAULT FALSE,
    partner_notification_attempts INT DEFAULT 0,
    partner_notified_at TIMESTAMPTZ,
    
    -- Error handling
    error_message TEXT,
    retry_count INT DEFAULT 0,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    
    -- Constraints
    CONSTRAINT valid_amount CHECK (amount > 0),
    CONSTRAINT unique_partner_tx UNIQUE (partner_id, partner_tx_ref)
);

-- Indexes
CREATE INDEX idx_payments_payment_ref ON payments(payment_ref);
CREATE INDEX idx_payments_partner_id ON payments(partner_id);
CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_status ON payments(status) WHERE status IN ('pending', 'processing');
CREATE INDEX idx_payments_provider_reference ON payments(provider_reference) WHERE provider_reference IS NOT NULL;
CREATE INDEX idx_payments_created_at ON payments(created_at DESC);

-- ===============================
-- PROVIDER TRANSACTIONS TABLE
-- ===============================

CREATE TABLE provider_transactions (
    id BIGSERIAL PRIMARY KEY,
    payment_id BIGINT NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    
    -- Provider details
    provider payment_provider_enum NOT NULL,
    transaction_type TEXT NOT NULL,              -- stk_push, b2c, bank_transfer, etc.
    
    -- Request/Response tracking
    request_payload JSONB NOT NULL,
    response_payload JSONB,
    provider_tx_id TEXT,                         -- Provider's transaction ID
    checkout_request_id TEXT,                    -- For M-Pesa STK
    
    -- Status
    status transaction_status_enum NOT NULL DEFAULT 'initiated',
    result_code TEXT,
    result_description TEXT,
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- Indexes
CREATE INDEX idx_provider_tx_payment_id ON provider_transactions(payment_id);
CREATE INDEX idx_provider_tx_provider_tx_id ON provider_transactions(provider_tx_id) WHERE provider_tx_id IS NOT NULL;
CREATE INDEX idx_provider_tx_checkout_request_id ON provider_transactions(checkout_request_id) WHERE checkout_request_id IS NOT NULL;

-- ===============================
-- TRIGGERS
-- ===============================

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_payments_set_updated_at
    BEFORE UPDATE ON payments
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_provider_transactions_set_updated_at
    BEFORE UPDATE ON provider_transactions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

COMMIT;