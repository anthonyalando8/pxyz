-- SQL script to create authentication tables for DB: pxyz
-- Last Updated: 2025-08-13

-- Connect to DB
\c pxyz;

-- Enable UUID if needed (currently using BIGINT/Snowflake)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

--------------------------
-- USERS TABLE
--------------------------
CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,
    
    -- Contact info (either can be NULL, but at least one must be set)
    email        CITEXT UNIQUE,
    phone        VARCHAR(20) UNIQUE,
    
    password_hash TEXT, -- NULL until password is set

    first_name   VARCHAR(100),
    last_name    VARCHAR(100),

    -- Independent verification flags
    is_email_verified BOOLEAN DEFAULT FALSE,
    is_phone_verified BOOLEAN DEFAULT FALSE,

    -- Tracks overall progress in account creation
    signup_stage TEXT DEFAULT 'email_or_phone_submitted', 
    -- stages: 'email_or_phone_submitted', 'otp_verified', 'password_set', 'complete'

    -- Account lifecycle
    account_status TEXT DEFAULT 'active', -- 'active', 'deleted', 'suspended'
    account_type TEXT DEFAULT 'password', -- 'password', 'social', 'hybrid'
    account_restored BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Restrict signup_stage to valid values
ALTER TABLE users
    ADD CONSTRAINT signup_stage_check 
    CHECK (signup_stage IN (
        'email_or_phone_submitted',
        'otp_verified',
        'password_set',
        'complete'
    ));

-- Restrict account_type to valid values
ALTER TABLE users
    ADD CONSTRAINT account_type_check
    CHECK (account_type IN ('password', 'social', 'hybrid'));

-- Ensure at least one contact field (email or phone) is provided
ALTER TABLE users
    ADD CONSTRAINT users_contact_check 
    CHECK (email IS NOT NULL OR phone IS NOT NULL);

--------------------------
-- USER OTPS TABLE
--------------------------
CREATE TABLE IF NOT EXISTS user_otps (
    id BIGINT PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    code TEXT NOT NULL,
    channel TEXT NOT NULL, -- 'email', 'sms'
    purpose TEXT NOT NULL, -- 'login', 'reset', etc.
    issued_at TIMESTAMPTZ DEFAULT NOW(),
    valid_until TIMESTAMPTZ NOT NULL,
    is_verified BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

--------------------------
-- OAUTH ACCOUNTS TABLE
--------------------------
CREATE TABLE IF NOT EXISTS oauth_accounts (
    id BIGINT PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL, -- 'google', 'facebook', 'telegram'
    provider_uid TEXT NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    linked_at TIMESTAMPTZ DEFAULT NOW()
);

--------------------------
-- SESSIONS TABLE
CREATE TABLE IF NOT EXISTS sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    auth_token TEXT NOT NULL,
    device_id TEXT,
    ip_address TEXT,
    user_agent TEXT,
    geo_location TEXT,
    device_metadata JSONB,
    is_active BOOLEAN DEFAULT TRUE,
    is_temp BOOLEAN DEFAULT FALSE,
    is_single_use BOOLEAN DEFAULT FALSE,
    is_used BOOLEAN DEFAULT FALSE,
    purpose TEXT,
    last_seen_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT unique_user_device UNIQUE (user_id, device_id)
);

-- sessions
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_is_active ON sessions(is_active);


--------------------------
-- EMAIL LOGS TABLE
--------------------------
CREATE TABLE IF NOT EXISTS email_logs (
    id BIGINT PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    subject TEXT,
    recipient_email TEXT,
    type TEXT, -- 'otp', 'password-reset', etc.
    status TEXT DEFAULT 'sent', -- 'sent', 'failed'
    sent_at TIMESTAMPTZ DEFAULT NOW()
);

-- 1. Drop the foreign key constraint first
ALTER TABLE email_logs
DROP CONSTRAINT IF EXISTS email_logs_user_id_fkey;

-- 2. Alter the column type to TEXT
ALTER TABLE email_logs
ALTER COLUMN user_id TYPE TEXT;

--------------------------
-- ACCOUNT DELETION REQUESTS
--------------------------
CREATE TABLE IF NOT EXISTS account_deletion_requests (
    id BIGINT PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    reason TEXT,
    requested_at TIMESTAMPTZ DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    processed_by BIGINT -- Optional admin ID
);

--------------------------
-- INDEXES
--------------------------

-- users
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_phone ON users(phone);
CREATE INDEX idx_users_account_status ON users(account_status);
CREATE INDEX idx_users_account_type ON users(account_type);
CREATE INDEX idx_users_created_at ON users(created_at);

-- user_otps
CREATE INDEX idx_user_otps_user_id ON user_otps(user_id);
CREATE INDEX idx_user_otps_code ON user_otps(code);
CREATE INDEX idx_user_otps_channel ON user_otps(channel);
CREATE INDEX idx_user_otps_purpose ON user_otps(purpose);
CREATE INDEX idx_user_otps_issued_at ON user_otps(issued_at);

-- oauth_accounts
CREATE INDEX idx_oauth_accounts_user_id ON oauth_accounts(user_id);
CREATE INDEX idx_oauth_accounts_provider_uid ON oauth_accounts(provider_uid);



-- email_logs
CREATE INDEX idx_email_logs_user_id ON email_logs(user_id);
CREATE INDEX idx_email_logs_type ON email_logs(type);
CREATE INDEX idx_email_logs_status ON email_logs(status);

-- account_deletion_requests
CREATE INDEX idx_account_deletion_user_id ON account_deletion_requests(user_id);
CREATE INDEX idx_account_deletion_requested_at ON account_deletion_requests(requested_at);

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

-- users
CREATE TRIGGER trg_users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- sessions
CREATE TRIGGER trg_sessions_set_updated_at
BEFORE UPDATE ON sessions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- user_otps
CREATE TRIGGER trg_user_otps_set_updated_at
BEFORE UPDATE ON user_otps
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();


-- Existing tables remain the same
CREATE TYPE role_enum AS ENUM (
    'system_admin',
    'partner_admin',
    'partner_user',
    'trader'
);

-- Update roles table
CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    name role_enum UNIQUE NOT NULL,
    description TEXT
);

CREATE TABLE permissions (
    id SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,          -- 'login', 'manage_users', 'view_partner_wallets', 'approve_withdrawals'
    description TEXT
);

CREATE TABLE role_permissions (
    role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    permission_id INT REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE user_roles (
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    role_id INT REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE user_permissions (
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    permission_id INT REFERENCES permissions(id) ON DELETE CASCADE,
    is_allowed BOOLEAN NOT NULL,       -- true = grant, false = deny
    assigned_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (user_id, permission_id)
);
