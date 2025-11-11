\c pxyz_admin

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

CREATE SEQUENCE user_twofa_id_seq;
CREATE SEQUENCE user_twofa_backup_codes_id_seq;


CREATE TABLE IF NOT EXISTS users
(
    id bigint NOT NULL,
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
    changed_emails jsonb,
    changed_phones jsonb,
    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_phone_key UNIQUE (phone),
    CONSTRAINT account_type_check CHECK (account_type = ANY (ARRAY['password'::text, 'social'::text, 'hybrid'::text])),
    CONSTRAINT users_contact_check CHECK (email IS NOT NULL OR phone IS NOT NULL)
)

TABLESPACE pg_default;

ALTER TABLE IF EXISTS users
    OWNER to postgres;

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

ALTER TABLE IF EXISTS sessions
    OWNER to postgres;
-- Index: idx_sessions_is_active

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

ALTER TABLE IF EXISTS user_twofa
    OWNER to postgres;
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

ALTER TABLE IF EXISTS user_twofa_backup_codes
    OWNER to postgres;
-- Index: idx_backup_codes_is_used

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


COMMIT;