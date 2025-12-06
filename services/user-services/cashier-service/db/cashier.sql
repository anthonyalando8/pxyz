\c pxyz_fx;

BEGIN;

-- Add to user database
CREATE TABLE deposit_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    partner_id TEXT NOT NULL,
    request_ref TEXT UNIQUE NOT NULL, -- Our internal reference
    amount NUMERIC(20,2) NOT NULL,
    currency TEXT NOT NULL,
    service TEXT NOT NULL, -- mpesa, paypal, etc
    payment_method TEXT,
    status TEXT NOT NULL DEFAULT 'pending', -- pending, sent_to_partner, processing, completed, failed, cancelled
    partner_transaction_ref TEXT, -- Partner's reference (once they respond)
    receipt_code TEXT, -- Accounting receipt code
    journal_id BIGINT, -- Accounting journal ID
    metadata JSONB,
    error_message TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_deposit_requests_user ON deposit_requests(user_id, created_at DESC);
CREATE INDEX idx_deposit_requests_status ON deposit_requests(status);
CREATE INDEX idx_deposit_requests_ref ON deposit_requests(request_ref);
CREATE INDEX idx_deposit_requests_partner_ref ON deposit_requests(partner_transaction_ref);

-- Trigger for updated_at
CREATE TRIGGER trg_deposit_requests_set_updated_at
    BEFORE UPDATE ON deposit_requests
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Withdrawal requests table
CREATE TABLE withdrawal_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    request_ref TEXT UNIQUE NOT NULL,
    amount NUMERIC(20,2) NOT NULL,
    currency TEXT NOT NULL,
    destination TEXT NOT NULL, -- phone number, account number, etc
    service TEXT, -- mpesa, bank, etc
    agent_external_id TEXT, -- External agent ID if applicable
    status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed, cancelled
    receipt_code TEXT,
    journal_id BIGINT,
    metadata JSONB,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_withdrawal_requests_user ON withdrawal_requests(user_id, created_at DESC);
CREATE INDEX idx_withdrawal_requests_status ON withdrawal_requests(status);
CREATE INDEX idx_withdrawal_requests_ref ON withdrawal_requests(request_ref);

CREATE TRIGGER trg_withdrawal_requests_set_updated_at
    BEFORE UPDATE ON withdrawal_requests
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

COMMIT;
