\c pxyz_partner

BEGIN;
--------------------------
-- TRIGGER: set_updated_at
--------------------------
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE partner_status_enum AS ENUM ('active', 'inactive', 'suspended');
CREATE TYPE partner_actor_type_enum AS ENUM ('system', 'user', 'admin');


CREATE TABLE partners (
  id            TEXT PRIMARY KEY,  -- string ID like PTxxxxxx
  name          TEXT NOT NULL,
  country       TEXT,
  contact_email TEXT,
  contact_phone TEXT,
  status        partner_status_enum NOT NULL DEFAULT 'active',
  service       TEXT,               -- new field: type of service the partner offers
  currency      TEXT,               -- new field: default currency for the partner
  local_currency TEXT NOT NULL,          -- e.g. KES
  rate NUMERIC(18,8) NOT NULL,           -- e.g. 120.50
  inverse_rate NUMERIC(18,8) GENERATED ALWAYS AS (1 / rate) STORED,
--   api_key_hash    TEXT,                           -- Hashed API key
--   webhook_url     TEXT,                           -- For callbacks
  commission_rate NUMERIC(5,4) DEFAULT 0,         -- Partner commission (e.g., 0.0050 = 0.5%)
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_partners_name ON partners (name);

ALTER TABLE partners 
ADD CONSTRAINT unique_partner_service UNIQUE (service);

-- Optional: Add index for better query performance (if not already covered by unique constraint)
CREATE INDEX IF NOT EXISTS idx_partners_service ON partners (service);

-- CREATE INDEX IF NOT EXISTS idx_partners_active ON partners (is_active) WHERE is_active = true;

COMMENT ON TABLE partners IS 'Partner entities that facilitate user transactions';

CREATE TABLE partner_kyc (
  partner_id TEXT PRIMARY KEY REFERENCES partners(id) ON DELETE CASCADE,
  status     TEXT NOT NULL DEFAULT 'pending',
  kyc_data   JSONB,
  limits     JSONB,
  risk_flags JSONB,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partner configs
CREATE TABLE partner_configs (
  partner_id       TEXT PRIMARY KEY REFERENCES partners(id) ON DELETE CASCADE,
  default_fx_spread NUMERIC(8,6) DEFAULT 0.005,
  webhook_secret   TEXT,
  config_data      JSONB,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Partner audit logs
CREATE TABLE partner_audit_logs (
  id          BIGSERIAL PRIMARY KEY,
  actor_type  partner_actor_type_enum NOT NULL,
  actor_id    BIGINT,
  action      TEXT NOT NULL,
  target_type TEXT,
  target_id   TEXT,  -- target partner ID can be string
  metadata    JSONB,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_logs_actor ON partner_audit_logs (actor_type, actor_id);
CREATE INDEX idx_audit_logs_action ON partner_audit_logs (action);


CREATE TABLE IF NOT EXISTS users
(
    id bigint NOT NULL,
    partner_id TEXT NOT NULL,  -- new column
    email citext COLLATE pg_catalog."default",
    phone character varying(20) COLLATE pg_catalog."default",
    password_hash text COLLATE pg_catalog."default",
    first_name character varying(100) COLLATE pg_catalog."default",
    last_name character varying(100) COLLATE pg_catalog."default",
    is_email_verified boolean DEFAULT false,
    is_phone_verified boolean DEFAULT false,
    account_status text COLLATE pg_catalog."default" DEFAULT 'active'::text,
    account_type text COLLATE pg_catalog."default" DEFAULT 'password'::text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    consent boolean NOT NULL DEFAULT true,
    is_temp_pass boolean NOT NULL DEFAULT false,
    role text COLLATE pg_catalog."default" DEFAULT 'partner_user'::text,
    changed_emails jsonb,
    changed_phones jsonb,
    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_phone_key UNIQUE (phone),
    CONSTRAINT account_type_check CHECK (
        account_type = ANY (ARRAY['password'::text, 'social'::text, 'hybrid'::text])
    ),
    CONSTRAINT role_check CHECK (
        role = ANY (ARRAY['partner_admin'::text, 'partner_user'::text])
    ),
    CONSTRAINT users_contact_check CHECK (email IS NOT NULL OR phone IS NOT NULL),
    CONSTRAINT fk_users_partner FOREIGN KEY (partner_id) REFERENCES partners(id)
        ON DELETE CASCADE
        ON UPDATE CASCADE
)
TABLESPACE pg_default;


COMMENT ON COLUMN users.consent
    IS 'User agrees to terms and conditions';

COMMENT ON COLUMN users.changed_emails
    IS 'changed user emails';
-- Index: idx_users_account_status

-- DROP INDEX IF EXISTS idx_users_account_status;

CREATE INDEX IF NOT EXISTS idx_users_account_status
    ON users USING btree
    (account_status COLLATE pg_catalog."default" ASC NULLS LAST)
    TABLESPACE pg_default;
-- Index: idx_users_account_type

-- DROP INDEX IF EXISTS idx_users_account_type;

CREATE INDEX IF NOT EXISTS idx_users_account_type
    ON users USING btree
    (account_type COLLATE pg_catalog."default" ASC NULLS LAST)
    TABLESPACE pg_default;
-- Index: idx_users_created_at

-- DROP INDEX IF EXISTS idx_users_created_at;

CREATE INDEX IF NOT EXISTS idx_users_created_at
    ON users USING btree
    (created_at ASC NULLS LAST)
    TABLESPACE pg_default;
-- Index: idx_users_email

-- DROP INDEX IF EXISTS idx_users_email;

CREATE INDEX IF NOT EXISTS idx_users_email
    ON users USING btree
    (email COLLATE pg_catalog."default" ASC NULLS LAST)
    TABLESPACE pg_default;
-- Index: idx_users_phone

-- DROP INDEX IF EXISTS idx_users_phone;

CREATE INDEX IF NOT EXISTS idx_users_phone
    ON users USING btree
    (phone COLLATE pg_catalog."default" ASC NULLS LAST)
    TABLESPACE pg_default;

-- Trigger: trg_users_set_updated_at

-- DROP TRIGGER IF EXISTS trg_users_set_updated_at ON users;

CREATE OR REPLACE TRIGGER trg_users_set_updated_at
    BEFORE UPDATE 
    ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();
-- Table: sessions

-- DROP TABLE IF EXISTS sessions;

CREATE TABLE IF NOT EXISTS sessions
(
    id bigint NOT NULL,
    user_id bigint,
    auth_token text COLLATE pg_catalog."default" NOT NULL,
    device_id text COLLATE pg_catalog."default",
    ip_address text COLLATE pg_catalog."default",
    user_agent text COLLATE pg_catalog."default",
    geo_location text COLLATE pg_catalog."default",
    device_metadata jsonb,
    is_active boolean DEFAULT true,
    last_seen_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    is_single_use boolean DEFAULT false,
    is_temp boolean DEFAULT false,
    is_used boolean DEFAULT false,
    purpose text COLLATE pg_catalog."default",
    expires_at timestamp with time zone,
    CONSTRAINT sessions_pkey PRIMARY KEY (id),
    CONSTRAINT unique_user_device_type UNIQUE (user_id, device_id, is_temp),
    CONSTRAINT sessions_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE CASCADE
)

TABLESPACE pg_default;


-- DROP INDEX IF EXISTS idx_sessions_is_active;

CREATE INDEX IF NOT EXISTS idx_sessions_is_active
    ON sessions USING btree
    (is_active ASC NULLS LAST)
    TABLESPACE pg_default;
-- Index: idx_sessions_token

-- DROP INDEX IF EXISTS idx_sessions_token;

CREATE INDEX IF NOT EXISTS idx_sessions_token
    ON sessions USING btree
    (auth_token COLLATE pg_catalog."default" ASC NULLS LAST)
    TABLESPACE pg_default;
-- Index: idx_sessions_user_id

-- DROP INDEX IF EXISTS idx_sessions_user_id;

CREATE INDEX IF NOT EXISTS idx_sessions_user_id
    ON sessions USING btree
    (user_id ASC NULLS LAST)
    TABLESPACE pg_default;

-- Trigger: trg_sessions_set_updated_at

-- DROP TRIGGER IF EXISTS trg_sessions_set_updated_at ON sessions;

CREATE OR REPLACE TRIGGER trg_sessions_set_updated_at
    BEFORE UPDATE 
    ON sessions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Table: user_twofa

-- DROP TABLE IF EXISTS user_twofa;

CREATE SEQUENCE user_twofa_id_seq;
CREATE SEQUENCE user_twofa_backup_codes_id_seq;

CREATE TABLE IF NOT EXISTS user_twofa
(
    id bigint NOT NULL DEFAULT nextval('user_twofa_id_seq'::regclass),
    user_id bigint NOT NULL,
    method text COLLATE pg_catalog."default" NOT NULL,
    secret text COLLATE pg_catalog."default",
    is_enabled boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    CONSTRAINT user_twofa_pkey PRIMARY KEY (id),
    CONSTRAINT unique_user_method UNIQUE (user_id, method),
    CONSTRAINT fk_user_twofa_user FOREIGN KEY (user_id)
        REFERENCES users (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE CASCADE
)

TABLESPACE pg_default;

-- Index: idx_user_twofa_method

-- DROP INDEX IF EXISTS idx_user_twofa_method;

CREATE INDEX IF NOT EXISTS idx_user_twofa_method
    ON user_twofa USING btree
    (method COLLATE pg_catalog."default" ASC NULLS LAST)
    TABLESPACE pg_default;
-- Index: idx_user_twofa_user_id

-- DROP INDEX IF EXISTS idx_user_twofa_user_id;

CREATE INDEX IF NOT EXISTS idx_user_twofa_user_id
    ON user_twofa USING btree
    (user_id ASC NULLS LAST)
    TABLESPACE pg_default;


-- Table: user_twofa_backup_codes

-- DROP TABLE IF EXISTS user_twofa_backup_codes;

CREATE TABLE IF NOT EXISTS user_twofa_backup_codes
(
    id bigint NOT NULL DEFAULT nextval('user_twofa_backup_codes_id_seq'::regclass),
    twofa_id bigint NOT NULL,
    code_hash text COLLATE pg_catalog."default" NOT NULL,
    is_used boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    used_at timestamp with time zone,
    CONSTRAINT user_twofa_backup_codes_pkey PRIMARY KEY (id),
    CONSTRAINT user_twofa_backup_codes_twofa_id_fkey FOREIGN KEY (twofa_id)
        REFERENCES user_twofa (id) MATCH SIMPLE
        ON UPDATE NO ACTION
        ON DELETE CASCADE
)

TABLESPACE pg_default;


-- DROP INDEX IF EXISTS idx_backup_codes_is_used;

CREATE INDEX IF NOT EXISTS idx_backup_codes_is_used
    ON user_twofa_backup_codes USING btree
    (is_used ASC NULLS LAST)
    TABLESPACE pg_default;
-- Index: idx_backup_codes_twofa_id

-- DROP INDEX IF EXISTS idx_backup_codes_twofa_id;

CREATE INDEX IF NOT EXISTS idx_backup_codes_twofa_id
    ON user_twofa_backup_codes USING btree
    (twofa_id ASC NULLS LAST)
    TABLESPACE pg_default;

-- Partner users
-- CREATE TABLE partner_users (
--   id         TEXT PRIMARY KEY,
--   partner_id TEXT NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
--   role       VARCHAR(16) NOT NULL CHECK (role IN ('partner_admin','partner_user')),
--   user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
--   is_active  BOOLEAN NOT NULL DEFAULT true,
--   created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
--   updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
--   UNIQUE (partner_id, user_id)
-- );
-- CREATE INDEX idx_partner_users_partner_id ON partner_users (partner_id);

-- Add new columns to partners table
ALTER TABLE partners 
ADD COLUMN IF NOT EXISTS api_key TEXT UNIQUE,
ADD COLUMN IF NOT EXISTS api_secret_hash TEXT,
ADD COLUMN IF NOT EXISTS webhook_url TEXT,
ADD COLUMN IF NOT EXISTS webhook_secret TEXT,
ADD COLUMN IF NOT EXISTS callback_url TEXT,
ADD COLUMN IF NOT EXISTS is_api_enabled BOOLEAN DEFAULT false,
ADD COLUMN IF NOT EXISTS api_rate_limit INTEGER DEFAULT 1000,
ADD COLUMN IF NOT EXISTS allowed_ips JSONB,
ADD COLUMN IF NOT EXISTS metadata JSONB;

-- Create index for API key lookups
CREATE INDEX IF NOT EXISTS idx_partners_api_key ON partners(api_key) WHERE api_key IS NOT NULL;

-- Partner API logs table
CREATE TABLE IF NOT EXISTS partner_api_logs (
  id BIGSERIAL PRIMARY KEY,
  partner_id TEXT NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  endpoint TEXT NOT NULL,
  method TEXT NOT NULL,
  request_body JSONB,
  response_body JSONB,
  status_code INTEGER,
  ip_address TEXT,
  user_agent TEXT,
  latency_ms INTEGER,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_api_logs_partner ON partner_api_logs(partner_id, created_at DESC);
CREATE INDEX idx_api_logs_endpoint ON partner_api_logs(endpoint);

-- Partner webhooks table (for outgoing webhooks to partner)
CREATE TABLE IF NOT EXISTS partner_webhooks (
  id BIGSERIAL PRIMARY KEY,
  partner_id TEXT NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL, -- 'deposit_received', 'wallet_funded', etc
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending', -- pending, sent, failed, retrying
  attempts INTEGER DEFAULT 0,
  max_attempts INTEGER DEFAULT 3,
  last_attempt_at TIMESTAMPTZ,
  next_retry_at TIMESTAMPTZ,
  response_status INTEGER,
  response_body TEXT,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhooks_partner_status ON partner_webhooks(partner_id, status);
CREATE INDEX idx_webhooks_retry ON partner_webhooks(next_retry_at) WHERE status = 'retrying';

-- Partner transactions table (partner-initiated deposits)
CREATE TABLE IF NOT EXISTS partner_transactions (
  id BIGSERIAL PRIMARY KEY,
  partner_id TEXT NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  transaction_ref TEXT NOT NULL, -- partner's reference
  user_id BIGINT NOT NULL,
  amount NUMERIC(20,2) NOT NULL,
  currency TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
  payment_method TEXT,
  external_ref TEXT, -- external payment reference
  metadata JSONB,
  processed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(partner_id, transaction_ref)
);

ALTER TABLE partner_transactions 
ADD COLUMN IF NOT EXISTS error_message TEXT;

ALTER TABLE partner_transactions
ADD COLUMN IF NOT EXISTS transaction_type TEXT NOT NULL DEFAULT 'deposit';

CREATE INDEX idx_partner_transactions_partner ON partner_transactions(partner_id, created_at DESC);
CREATE INDEX idx_partner_transactions_user ON partner_transactions(user_id, created_at DESC);
CREATE INDEX idx_partner_transactions_status ON partner_transactions(status);
CREATE INDEX idx_partner_transactions_ref ON partner_transactions(transaction_ref);

-- Trigger for partner_webhooks
CREATE TRIGGER trg_partner_webhooks_set_updated_at
    BEFORE UPDATE ON partner_webhooks
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Trigger for partner_transactions
CREATE TRIGGER trg_partner_transactions_set_updated_at
    BEFORE UPDATE ON partner_transactions
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

COMMIT;