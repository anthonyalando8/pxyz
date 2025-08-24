-- Switch DB (only works in psql shell, not migrations)
\c pxyz;

CREATE TABLE kyc_submissions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    id_number VARCHAR(100) NOT NULL,
    nationality VARCHAR(100),
    document_type VARCHAR(50) NOT NULL DEFAULT 'national_id',
    document_front_url TEXT NOT NULL,
    document_back_url TEXT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending', 
        -- pending, under_review, approved, rejected
    rejection_reason TEXT,
    submitted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE kyc_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    kyc_id BIGINT NOT NULL,
    action VARCHAR(50) NOT NULL, -- submitted, reviewed, approved, rejected
    actor VARCHAR(100),          -- system/admin identifier
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_kyc FOREIGN KEY (kyc_id) 
        REFERENCES kyc_submissions(id) ON DELETE CASCADE
);

-- Indexes
CREATE INDEX idx_kyc_user ON kyc_submissions(user_id);
CREATE INDEX idx_kyc_status ON kyc_submissions(status);
CREATE INDEX idx_kyc_logs_kyc_id ON kyc_audit_logs(kyc_id);


CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_set_updated_at
BEFORE UPDATE ON kyc_submissions
FOR EACH ROW EXECUTE FUNCTION set_updated_at();
