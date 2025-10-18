-- Connect to your database
\c pxyz_user;

BEGIN;

-- EXTENSIONS
CREATE EXTENSION IF NOT EXISTS citext;

-- CORE USERS TABLE
CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,
    account_status text DEFAULT 'active',
    account_type text DEFAULT 'password',
    account_restored boolean DEFAULT false,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now(),
    consent boolean NOT NULL DEFAULT true,
    CONSTRAINT account_type_check CHECK (account_type = ANY (ARRAY['password','social','hybrid']))
);

-- Index for querying non-active accounts
CREATE INDEX IF NOT EXISTS idx_users_account_status 
ON users(account_status) 
WHERE account_status != 'active';

-- UTILITY FUNCTIONS

-- Auto-update timestamps
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
   NEW.updated_at = now();
   RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- USER CREDENTIALS TABLE
CREATE TABLE IF NOT EXISTS user_credentials (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email citext,
    phone varchar(20),
    password_hash text,
    is_email_verified boolean DEFAULT false,
    is_phone_verified boolean DEFAULT false,
    valid boolean DEFAULT true,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now(),
    CONSTRAINT user_credentials_contact_check CHECK (email IS NOT NULL OR phone IS NOT NULL),
    CONSTRAINT phone_format_check CHECK (phone IS NULL OR phone ~ '^\+?[1-9]\d{1,14}$')
);

-- Ensures only one valid credential per user
CREATE UNIQUE INDEX idx_user_credentials_one_valid 
ON user_credentials (user_id) 
WHERE valid = true;

-- Ensures email is unique across all valid credentials
CREATE UNIQUE INDEX idx_user_credentials_email_unique
ON user_credentials (email)
WHERE valid = true AND email IS NOT NULL;

-- Ensures phone is unique across all valid credentials
CREATE UNIQUE INDEX idx_user_credentials_phone_unique
ON user_credentials (phone)
WHERE valid = true AND phone IS NOT NULL;

-- Index for looking up all credentials for a user
CREATE INDEX IF NOT EXISTS idx_credentials_user_id 
ON user_credentials(user_id);

-- CREDENTIAL TRIGGERS

-- Auto-update timestamps for credentials
CREATE TRIGGER trg_user_credentials_set_updated_at
    BEFORE UPDATE ON user_credentials
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- Automatically invalidate old credentials when inserting a new valid one
CREATE OR REPLACE FUNCTION invalidate_old_credentials()
RETURNS trigger AS $$
BEGIN
    IF NEW.valid = true THEN
        UPDATE user_credentials
        SET valid = false
        WHERE user_id = NEW.user_id 
          AND id != NEW.id 
          AND valid = true;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_invalidate_old_credentials
    BEFORE INSERT ON user_credentials
    FOR EACH ROW
    WHEN (NEW.valid = true)
    EXECUTE FUNCTION invalidate_old_credentials();

-- CREDENTIAL HISTORY (AUDIT TRAIL)
CREATE TABLE IF NOT EXISTS credential_history (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    old_email citext,
    old_phone varchar(20),
    old_password_hash text,
    changed_at timestamptz DEFAULT now()
);

-- Indexes for audit queries
CREATE INDEX IF NOT EXISTS idx_credential_history_user_id 
ON credential_history(user_id);

CREATE INDEX IF NOT EXISTS idx_credential_history_changed_at 
ON credential_history(changed_at);

-- Log credential changes to history
CREATE OR REPLACE FUNCTION log_credential_change()
RETURNS trigger AS $$
BEGIN
    -- Only log if email, phone, or password changed
    IF (OLD.email IS DISTINCT FROM NEW.email) OR 
       (OLD.phone IS DISTINCT FROM NEW.phone) OR 
       (OLD.password_hash IS DISTINCT FROM NEW.password_hash) THEN
        
        INSERT INTO credential_history (user_id, old_email, old_phone, old_password_hash)
        VALUES (OLD.user_id, OLD.email, OLD.phone, OLD.password_hash);
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_log_credential_change
    BEFORE UPDATE ON user_credentials
    FOR EACH ROW
    EXECUTE FUNCTION log_credential_change();

-- PASSWORD RESET TOKENS
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    used boolean DEFAULT false,
    created_at timestamptz DEFAULT now(),
    CONSTRAINT token_not_expired CHECK (expires_at > created_at)
);

-- Index for fast token lookup (only active tokens)
CREATE INDEX IF NOT EXISTS idx_reset_tokens_hash 
ON password_reset_tokens(token_hash)
WHERE used = false;

-- Index for cleanup queries
CREATE INDEX IF NOT EXISTS idx_reset_tokens_expires 
ON password_reset_tokens(expires_at);

-- EMAIL VERIFICATION TOKENS
CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email citext NOT NULL,
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    used boolean DEFAULT false,
    created_at timestamptz DEFAULT now(),
    CONSTRAINT email_token_not_expired CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_hash 
ON email_verification_tokens(token_hash) 
WHERE used = false;

-- PHONE VERIFICATION TOKENS (OTP)
CREATE TABLE IF NOT EXISTS phone_verification_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    phone varchar(20) NOT NULL,
    token_hash text NOT NULL,  -- Hashed OTP code
    expires_at timestamptz NOT NULL,
    attempts int DEFAULT 0,
    used boolean DEFAULT false,
    created_at timestamptz DEFAULT now(),
    CONSTRAINT phone_token_not_expired CHECK (expires_at > created_at),
    CONSTRAINT max_attempts CHECK (attempts <= 5)
);

CREATE INDEX IF NOT EXISTS idx_phone_verification_tokens_hash 
ON phone_verification_tokens(token_hash) 
WHERE used = false;

-- COMMENTS FOR DOCUMENTATION
COMMENT ON TABLE users IS 'Core user accounts with immutable IDs';
COMMENT ON TABLE user_credentials IS 'User authentication credentials (email/phone + password). Only one credential can be valid per user at a time.';
COMMENT ON TABLE credential_history IS 'Audit trail of credential changes';
COMMENT ON TABLE password_reset_tokens IS 'Tokens for password reset flow';
COMMENT ON TABLE email_verification_tokens IS 'Tokens for email verification';
COMMENT ON TABLE phone_verification_tokens IS 'OTP tokens for phone verification';

COMMENT ON COLUMN users.id IS 'Snowflake ID generated by application';
COMMENT ON COLUMN user_credentials.valid IS 'Only one credential per user can be valid=true at a time';
COMMENT ON COLUMN user_credentials.email IS 'Case-insensitive email using citext';
COMMENT ON COLUMN user_credentials.phone IS 'Phone in E.164 format';

-- Create oauth_accounts table
CREATE TABLE IF NOT EXISTS oauth_accounts (
    id VARCHAR(255) PRIMARY KEY,
    user_id BIGINT NOT NULL,
    provider VARCHAR(50) NOT NULL,
    provider_uid VARCHAR(255) NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    expires_at TIMESTAMP,
    scope TEXT,
    metadata JSONB DEFAULT '{}',
    linked_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    -- Foreign key to users table
    CONSTRAINT fk_oauth_user 
        FOREIGN KEY (user_id) 
        REFERENCES users(id) 
        ON DELETE CASCADE,
    
    -- Ensure one provider per user
    CONSTRAINT unique_user_provider 
        UNIQUE (user_id, provider),
    
    -- Ensure unique provider accounts
    CONSTRAINT unique_provider_uid 
        UNIQUE (provider, provider_uid)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_oauth_accounts_user_id 
    ON oauth_accounts(user_id);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_provider 
    ON oauth_accounts(provider);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_provider_uid 
    ON oauth_accounts(provider, provider_uid);

CREATE INDEX IF NOT EXISTS idx_oauth_accounts_linked_at 
    ON oauth_accounts(linked_at DESC);


CREATE TRIGGER oauth_accounts_updated_at
    BEFORE UPDATE ON oauth_accounts
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

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
CREATE INDEX idx_sessions_token ON sessions(auth_token);
CREATE INDEX idx_sessions_is_active ON sessions(is_active);

COMMIT;