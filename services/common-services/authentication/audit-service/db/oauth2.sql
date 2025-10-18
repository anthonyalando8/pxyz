-- Connect to your database
\c pxyz_user;

BEGIN;

-- EXTENSIONS
CREATE EXTENSION IF NOT EXISTS citext;

-- OAUTH2 PROVIDER TABLES
-- (Allow external apps to connect to YOUR platform)

-- OAuth2 Clients (Third-party applications)
CREATE TABLE IF NOT EXISTS oauth2_clients (
    id BIGSERIAL PRIMARY KEY,
    client_id VARCHAR(255) NOT NULL UNIQUE,
    client_secret_hash TEXT NOT NULL,
    client_name VARCHAR(255) NOT NULL,
    client_uri TEXT,
    logo_uri TEXT,
    owner_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    redirect_uris TEXT[] NOT NULL, -- Array of allowed redirect URIs
    grant_types TEXT[] NOT NULL DEFAULT ARRAY['authorization_code', 'refresh_token'],
    response_types TEXT[] NOT NULL DEFAULT ARRAY['code'],
    scope TEXT NOT NULL DEFAULT 'read', -- Space-separated scopes
    is_confidential BOOLEAN DEFAULT true,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT valid_grant_types CHECK (
        grant_types <@ ARRAY['authorization_code', 'client_credentials', 'refresh_token', 'implicit']::TEXT[]
    )
);

CREATE INDEX idx_oauth2_clients_client_id ON oauth2_clients(client_id);
CREATE INDEX idx_oauth2_clients_owner ON oauth2_clients(owner_user_id);
CREATE INDEX idx_oauth2_clients_active ON oauth2_clients(is_active) WHERE is_active = true;

CREATE TRIGGER trg_oauth2_clients_set_updated_at
    BEFORE UPDATE ON oauth2_clients
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

-- OAuth2 Authorization Codes
CREATE TABLE IF NOT EXISTS oauth2_authorization_codes (
    id BIGSERIAL PRIMARY KEY,
    code VARCHAR(255) NOT NULL UNIQUE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth2_clients(client_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    scope TEXT NOT NULL,
    code_challenge VARCHAR(255), -- PKCE support
    code_challenge_method VARCHAR(10), -- S256 or plain
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT valid_code_challenge_method CHECK (
        code_challenge_method IS NULL OR code_challenge_method IN ('S256', 'plain')
    )
);

CREATE INDEX idx_oauth2_codes_code ON oauth2_authorization_codes(code) WHERE used = false;
CREATE INDEX idx_oauth2_codes_user ON oauth2_authorization_codes(user_id);
CREATE INDEX idx_oauth2_codes_client ON oauth2_authorization_codes(client_id);

-- OAuth2 Access Tokens
CREATE TABLE IF NOT EXISTS oauth2_access_tokens (
    id BIGSERIAL PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth2_clients(client_id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE, -- NULL for client_credentials
    scope TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_oauth2_access_tokens_hash ON oauth2_access_tokens(token_hash) 
    WHERE revoked = false;
CREATE INDEX idx_oauth2_access_tokens_user ON oauth2_access_tokens(user_id);
CREATE INDEX idx_oauth2_access_tokens_client ON oauth2_access_tokens(client_id);
CREATE INDEX idx_oauth2_access_tokens_expires ON oauth2_access_tokens(expires_at);

-- OAuth2 Refresh Tokens
CREATE TABLE IF NOT EXISTS oauth2_refresh_tokens (
    id BIGSERIAL PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    access_token_id BIGINT REFERENCES oauth2_access_tokens(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth2_clients(client_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_oauth2_refresh_tokens_hash ON oauth2_refresh_tokens(token_hash) 
    WHERE revoked = false;
CREATE INDEX idx_oauth2_refresh_tokens_user ON oauth2_refresh_tokens(user_id);
CREATE INDEX idx_oauth2_refresh_tokens_client ON oauth2_refresh_tokens(client_id);

-- OAuth2 User Consents (What apps have access to what)
CREATE TABLE IF NOT EXISTS oauth2_user_consents (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id VARCHAR(255) NOT NULL REFERENCES oauth2_clients(client_id) ON DELETE CASCADE,
    scope TEXT NOT NULL,
    granted_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ, -- NULL = doesn't expire
    revoked BOOLEAN DEFAULT false,
    UNIQUE(user_id, client_id)
);

CREATE INDEX idx_oauth2_consents_user ON oauth2_user_consents(user_id);
CREATE INDEX idx_oauth2_consents_client ON oauth2_user_consents(client_id);
CREATE INDEX idx_oauth2_consents_active ON oauth2_user_consents(user_id, client_id) 
    WHERE revoked = false;

-- OAuth2 Scopes Definition
CREATE TABLE IF NOT EXISTS oauth2_scopes (
    id BIGSERIAL PRIMARY KEY,
    scope VARCHAR(100) NOT NULL UNIQUE,
    description TEXT NOT NULL,
    is_default BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Insert default scopes
INSERT INTO oauth2_scopes (scope, description, is_default) VALUES
    ('read', 'Read access to basic profile information', true),
    ('write', 'Write access to update profile information', false),
    ('email', 'Access to user email address', false),
    ('phone', 'Access to user phone number', false),
    ('profile', 'Access to full user profile', false),
    ('openid', 'OpenID Connect authentication', true)
ON CONFLICT (scope) DO NOTHING;

-- OAuth2 Audit Log
CREATE TABLE IF NOT EXISTS oauth2_audit_log (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(50) NOT NULL, -- 'token_issued', 'token_revoked', 'consent_granted', etc.
    client_id VARCHAR(255) REFERENCES oauth2_clients(client_id) ON DELETE SET NULL,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ip_address INET,
    user_agent TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_oauth2_audit_event_type ON oauth2_audit_log(event_type);
CREATE INDEX idx_oauth2_audit_client ON oauth2_audit_log(client_id);
CREATE INDEX idx_oauth2_audit_user ON oauth2_audit_log(user_id);
CREATE INDEX idx_oauth2_audit_created ON oauth2_audit_log(created_at);

-- OAUTH2 COMMENTS
COMMENT ON TABLE oauth2_clients IS 'Third-party applications that can access user data via OAuth2';
COMMENT ON TABLE oauth2_authorization_codes IS 'Short-lived codes exchanged for access tokens';
COMMENT ON TABLE oauth2_access_tokens IS 'Access tokens for API authentication';
COMMENT ON TABLE oauth2_refresh_tokens IS 'Long-lived tokens to get new access tokens';
COMMENT ON TABLE oauth2_user_consents IS 'User permissions granted to applications';
COMMENT ON TABLE oauth2_scopes IS 'Available OAuth2 scopes/permissions';
COMMENT ON TABLE oauth2_audit_log IS 'Audit trail for OAuth2 operations';

COMMIT;