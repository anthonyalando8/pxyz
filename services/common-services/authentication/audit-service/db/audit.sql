-- Connect to your database
\c pxyz_user;

BEGIN;

CREATE TABLE IF NOT EXISTS security_audit_log (
    id BIGSERIAL PRIMARY KEY,
    
    -- Event Information
    event_type VARCHAR(100) NOT NULL, -- 'login', 'logout', 'password_change', 'failed_login', etc.
    event_category VARCHAR(50) NOT NULL, -- 'authentication', 'authorization', 'account', 'security'
    severity VARCHAR(20) NOT NULL DEFAULT 'info', -- 'info', 'warning', 'error', 'critical'
    status VARCHAR(20) NOT NULL DEFAULT 'success', -- 'success', 'failure', 'pending'
    
    -- User Information
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    target_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL, -- For admin actions on other users
    
    -- Session & Request Information
    session_id VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    request_id VARCHAR(100), -- Trace ID for correlation
    
    -- Resource Information
    resource_type VARCHAR(50), -- 'user', 'credential', 'token', 'client', etc.
    resource_id VARCHAR(255),
    
    -- Action Details
    action VARCHAR(100), -- Specific action performed
    description TEXT, -- Human-readable description
    
    -- Additional Context
    metadata JSONB, -- Flexible field for extra data
    previous_value JSONB, -- For tracking changes (before)
    new_value JSONB, -- For tracking changes (after)
    
    -- Error Information
    error_code VARCHAR(50),
    error_message TEXT,
    
    -- Geolocation (optional)
    country VARCHAR(2), -- ISO country code
    city VARCHAR(100),
    
    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT valid_event_category CHECK (
        event_category IN ('authentication', 'authorization', 'account', 'security', 'oauth2', 'admin')
    ),
    CONSTRAINT valid_severity CHECK (
        severity IN ('info', 'warning', 'error', 'critical')
    ),
    CONSTRAINT valid_status CHECK (
        status IN ('success', 'failure', 'pending')
    )
);

-- Indexes for common queries
CREATE INDEX idx_security_audit_user ON security_audit_log(user_id, created_at DESC);
CREATE INDEX idx_security_audit_target_user ON security_audit_log(target_user_id, created_at DESC);
CREATE INDEX idx_security_audit_event_type ON security_audit_log(event_type, created_at DESC);
CREATE INDEX idx_security_audit_event_category ON security_audit_log(event_category, created_at DESC);
CREATE INDEX idx_security_audit_severity ON security_audit_log(severity, created_at DESC) 
    WHERE severity IN ('error', 'critical');
CREATE INDEX idx_security_audit_status ON security_audit_log(status, created_at DESC) 
    WHERE status = 'failure';
CREATE INDEX idx_security_audit_ip ON security_audit_log(ip_address, created_at DESC);
CREATE INDEX idx_security_audit_session ON security_audit_log(session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_security_audit_resource ON security_audit_log(resource_type, resource_id);
CREATE INDEX idx_security_audit_created ON security_audit_log(created_at DESC);

-- GIN index for JSONB metadata searches
CREATE INDEX idx_security_audit_metadata ON security_audit_log USING GIN(metadata);

CREATE INDEX idx_security_audit_recent_critical ON security_audit_log(created_at DESC)
    WHERE severity = 'critical';


-- AUDIT LOG RETENTION POLICY

-- Table to store retention settings
CREATE TABLE IF NOT EXISTS audit_retention_policy (
    id SERIAL PRIMARY KEY,
    event_category VARCHAR(50) NOT NULL UNIQUE,
    retention_days INT NOT NULL,
    archive_enabled BOOLEAN DEFAULT false,
    archive_location TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT positive_retention CHECK (retention_days > 0)
);

-- Default retention policies
INSERT INTO audit_retention_policy (event_category, retention_days, archive_enabled) VALUES
    ('authentication', 90, true),
    ('authorization', 90, true),
    ('account', 365, true),
    ('security', 730, true), -- 2 years for security events
    ('oauth2', 180, true),
    ('admin', 365, true)
ON CONFLICT (event_category) DO NOTHING;

CREATE TRIGGER trg_audit_retention_policy_updated
    BEFORE UPDATE ON audit_retention_policy
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();


-- SUSPICIOUS ACTIVITY TRACKING

CREATE TABLE IF NOT EXISTS suspicious_activity (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    activity_type VARCHAR(100) NOT NULL, -- 'multiple_failed_logins', 'unusual_location', etc.
    risk_score INT NOT NULL DEFAULT 0, -- 0-100
    ip_address INET,
    details JSONB,
    status VARCHAR(20) DEFAULT 'active', -- 'active', 'resolved', 'false_positive'
    resolved_by TEXT,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT valid_risk_score CHECK (risk_score >= 0 AND risk_score <= 100),
    CONSTRAINT valid_activity_status CHECK (status IN ('active', 'resolved', 'false_positive'))
);

CREATE INDEX idx_suspicious_activity_user ON suspicious_activity(user_id, status);
CREATE INDEX idx_suspicious_activity_status ON suspicious_activity(status, created_at DESC) 
    WHERE status = 'active';
CREATE INDEX idx_suspicious_activity_risk ON suspicious_activity(risk_score DESC) 
    WHERE status = 'active' AND risk_score >= 70;
CREATE INDEX idx_suspicious_activity_ip ON suspicious_activity(ip_address);

CREATE TRIGGER trg_suspicious_activity_updated
    BEFORE UPDATE ON suspicious_activity
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();


-- FAILED LOGIN ATTEMPTS TRACKING

CREATE TABLE IF NOT EXISTS failed_login_attempts (
    id BIGSERIAL PRIMARY KEY,
    identifier VARCHAR(255) NOT NULL, -- email, phone, or user_id
    identifier_type VARCHAR(20) NOT NULL, -- 'email', 'phone', 'user_id'
    ip_address INET NOT NULL,
    user_agent TEXT,
    failure_reason VARCHAR(100), -- 'invalid_password', 'user_not_found', 'account_locked', etc.
    attempted_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT valid_identifier_type CHECK (identifier_type IN ('email', 'phone', 'user_id'))
);

-- Indexes for rate limiting and security checks
CREATE INDEX idx_failed_login_identifier ON failed_login_attempts(identifier, attempted_at DESC);
CREATE INDEX idx_failed_login_ip ON failed_login_attempts(ip_address, attempted_at DESC);
CREATE INDEX idx_failed_login_attempted ON failed_login_attempts(attempted_at DESC);

-- Composite index for rate limiting queries
CREATE INDEX idx_failed_login_rate_limit ON failed_login_attempts(identifier, ip_address, attempted_at DESC);


-- ACCOUNT LOCKOUT TRACKING

CREATE TABLE IF NOT EXISTS account_lockouts (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason VARCHAR(100) NOT NULL, -- 'too_many_failed_attempts', 'suspicious_activity', 'admin_action'
    locked_by TEXT, -- NULL for automatic, user_id for manual
    locked_at TIMESTAMPTZ DEFAULT NOW(),
    unlock_at TIMESTAMPTZ, -- NULL for permanent lock
    unlocked_at TIMESTAMPTZ,
    unlocked_by TEXT,
    is_active BOOLEAN DEFAULT true,
    metadata JSONB
);

CREATE INDEX idx_account_lockouts_user ON account_lockouts(user_id, is_active);
CREATE INDEX idx_account_lockouts_active ON account_lockouts(user_id) WHERE is_active = true;
CREATE INDEX idx_account_lockouts_unlock ON account_lockouts(unlock_at) 
    WHERE is_active = true AND unlock_at IS NOT NULL;


-- SECURITY EVENTS SUMMARY (Materialized View)

CREATE MATERIALIZED VIEW security_events_summary AS
SELECT 
    DATE(created_at) as event_date,
    event_category,
    event_type,
    status,
    severity,
    COUNT(*) as event_count,
    COUNT(DISTINCT user_id) as unique_users,
    COUNT(DISTINCT ip_address) as unique_ips
FROM security_audit_log
WHERE created_at > NOW() - INTERVAL '90 days'
GROUP BY DATE(created_at), event_category, event_type, status, severity;

CREATE UNIQUE INDEX idx_security_summary_unique 
ON security_events_summary(event_date, event_category, event_type, status, severity);

-- Refresh materialized view daily
-- Run this as a scheduled job: REFRESH MATERIALIZED VIEW CONCURRENTLY security_events_summary;


-- AUDIT HELPER FUNCTIONS


-- Function to check if account is locked
CREATE OR REPLACE FUNCTION is_account_locked(p_user_id BIGINT)
RETURNS BOOLEAN AS $fn$
BEGIN
    RETURN EXISTS (
        SELECT 1 
        FROM account_lockouts 
        WHERE user_id = p_user_id 
          AND is_active = true
          AND (unlock_at IS NULL OR unlock_at > NOW())
    );
END;
$fn$ LANGUAGE plpgsql;

-- Function to count recent failed login attempts
CREATE OR REPLACE FUNCTION count_recent_failed_logins(p_identifier VARCHAR, p_minutes INT DEFAULT 15)
RETURNS INT AS $fn$
BEGIN
    RETURN (
        SELECT COUNT(*) 
        FROM failed_login_attempts 
        WHERE identifier = p_identifier 
          AND attempted_at > NOW() - (p_minutes || ' minutes')::INTERVAL
    );
END;
$fn$ LANGUAGE plpgsql;


-- Function to get user risk score
CREATE OR REPLACE FUNCTION get_user_risk_score(p_user_id BIGINT)
RETURNS INT AS $fn$
DECLARE
    v_risk_score INT := 0;
    v_failed_logins INT;
    v_suspicious_activities INT;
BEGIN
    -- Count recent failed logins
    SELECT COUNT(*) INTO v_failed_logins
    FROM security_audit_log
    WHERE user_id = p_user_id
      AND event_type = 'failed_login'
      AND created_at > NOW() - INTERVAL '24 hours';
    
    -- Count active suspicious activities
    SELECT COUNT(*) INTO v_suspicious_activities
    FROM suspicious_activity
    WHERE user_id = p_user_id
      AND status = 'active';
    
    -- Calculate risk score
    v_risk_score := LEAST(100, (v_failed_logins * 10) + (v_suspicious_activities * 30));
    
    RETURN v_risk_score;
END;
$fn$ LANGUAGE plpgsql;



-- COMMENTS

COMMENT ON TABLE security_audit_log IS 'Comprehensive security and authentication audit trail';
COMMENT ON TABLE audit_retention_policy IS 'Retention policies for different audit log categories';
COMMENT ON TABLE suspicious_activity IS 'Tracking of suspicious user activities';
COMMENT ON TABLE failed_login_attempts IS 'Failed login attempts for rate limiting and security';
COMMENT ON TABLE account_lockouts IS 'Account lockout history and active locks';
COMMENT ON MATERIALIZED VIEW security_events_summary IS 'Daily summary of security events for analytics';

COMMIT;