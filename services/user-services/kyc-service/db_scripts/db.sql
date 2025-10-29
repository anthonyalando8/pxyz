-- Switch DB (only works in psql shell, not migrations)
\c pxyz_user;

BEGIN;

-- Table: kyc_submissions

DROP TABLE IF EXISTS kyc_submissions;

CREATE TABLE IF NOT EXISTS kyc_submissions
(
    id BIGSERIAL NOT NULL,
    user_id bigint NOT NULL,
    id_number character varying(100) COLLATE pg_catalog."default" NOT NULL,
    nationality character varying(100) COLLATE pg_catalog."default",
    document_type character varying(50) COLLATE pg_catalog."default" NOT NULL DEFAULT 'national_id'::character varying,
    document_front_url text COLLATE pg_catalog."default" NOT NULL,
    document_back_url text COLLATE pg_catalog."default" NOT NULL,
    status character varying(30) COLLATE pg_catalog."default" NOT NULL DEFAULT 'pending'::character varying,
    rejection_reason text COLLATE pg_catalog."default",
    submitted_at timestamp without time zone NOT NULL DEFAULT now(),
    reviewed_at timestamp without time zone,
    updated_at timestamp without time zone NOT NULL DEFAULT now(),
    selfie_image_url text COLLATE pg_catalog."default",
    date_of_birth date,
    consent boolean NOT NULL DEFAULT true,
    CONSTRAINT kyc_submissions_pkey PRIMARY KEY (id),
    CONSTRAINT fk_kyc_user FOREIGN KEY (user_id)
        REFERENCES users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE CASCADE
);

DROP INDEX IF EXISTS idx_kyc_status;


DROP INDEX IF EXISTS idx_kyc_user;


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

-- DROP TRIGGER IF EXISTS trg_set_updated_at ON kyc_submissions;

CREATE OR REPLACE TRIGGER trg_set_updated_at
    BEFORE UPDATE 
    ON kyc_submissions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

COMMIT;