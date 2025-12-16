-- migrations/xxxx_create_transaction_approvals. sql
\c pxyz_fx;

BEGIN;

CREATE TYPE approval_status_enum AS ENUM (
    'pending',
    'approved',
    'rejected',
    'executed',
    'failed'
);

CREATE TABLE transaction_approvals (
    id BIGSERIAL PRIMARY KEY,
    requested_by BIGINT NOT NULL,  -- Admin user ID who requested
    transaction_type transaction_type_enum NOT NULL,  -- credit, debit, transfer, conversion
    account_number TEXT NOT NULL,
    amount NUMERIC(20,2) NOT NULL,
    currency TEXT NOT NULL,
    description TEXT,
    to_account_number TEXT,  -- For transfers/conversions
    
    -- Approval tracking
    status approval_status_enum NOT NULL DEFAULT 'pending',
    approved_by BIGINT,  -- Super admin user ID who approved/rejected
    rejection_reason TEXT,
    
    -- Execution tracking
    receipt_code TEXT,  -- Set after successful execution
    error_message TEXT,  -- Set if execution failed
    
    -- Metadata
    request_metadata JSONB,  -- Store original request details
    
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at TIMESTAMPTZ,
    executed_at TIMESTAMPTZ,
    
    -- Constraints
    CONSTRAINT valid_amount CHECK (amount > 0),
    CONSTRAINT valid_status_transition CHECK (
        (status = 'pending') OR
        (status = 'approved' AND approved_by IS NOT NULL) OR
        (status = 'rejected' AND approved_by IS NOT NULL AND rejection_reason IS NOT NULL) OR
        (status = 'executed' AND receipt_code IS NOT NULL) OR
        (status = 'failed' AND error_message IS NOT NULL)
    )
);

-- Indexes
CREATE INDEX idx_transaction_approvals_status ON transaction_approvals(status) WHERE status = 'pending';
CREATE INDEX idx_transaction_approvals_requested_by ON transaction_approvals(requested_by);
CREATE INDEX idx_transaction_approvals_approved_by ON transaction_approvals(approved_by);
CREATE INDEX idx_transaction_approvals_created_at ON transaction_approvals(created_at DESC);
CREATE INDEX idx_transaction_approvals_receipt_code ON transaction_approvals(receipt_code) WHERE receipt_code IS NOT NULL;

-- Trigger for updated_at
CREATE TRIGGER trg_transaction_approvals_set_updated_at
    BEFORE UPDATE ON transaction_approvals
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

COMMENT ON TABLE transaction_approvals IS 'Stores transaction approval requests for admin oversight';

COMMIT;