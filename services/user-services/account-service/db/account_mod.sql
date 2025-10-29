\c pxyz_user;
-- ============================
-- USER PROFILES
-- ============================
BEGIN;
CREATE TABLE IF NOT EXISTS user_profiles (
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE PRIMARY KEY, -- references auth.users(id) logically
    date_of_birth DATE,
    profile_image_url TEXT,
    first_name   VARCHAR(100),
    last_name    VARCHAR(100),
    surname VARCHAR(100),
    sys_username VARCHAR(100),
    address JSONB, -- { "line1": "...", "city": "...", "country": "..." }
    gender TEXT,
    bio TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

ALTER TABLE user_profiles
ADD COLUMN nationality CHAR(2) NULL,
ADD CONSTRAINT fk_user_profiles_country
    FOREIGN KEY (nationality)
    REFERENCES countries(iso2)
    ON DELETE SET NULL;

-- Indexes for user_profiles
-- primary key already covers lookups by user_id
CREATE INDEX IF NOT EXISTS idx_user_profiles_dob ON user_profiles(date_of_birth);
CREATE INDEX IF NOT EXISTS idx_user_profiles_gender ON user_profiles(gender);
CREATE INDEX IF NOT EXISTS idx_user_profiles_address_gin ON user_profiles USING GIN (address jsonb_path_ops);


-- ============================
-- USER PREFERENCES
-- ============================
CREATE TABLE IF NOT EXISTS user_preferences (
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE PRIMARY KEY,
    preferences JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for user_preferences
-- Allows fast lookups of preference keys/values
CREATE INDEX IF NOT EXISTS idx_user_preferences_gin ON user_preferences USING GIN (preferences jsonb_path_ops);


-- ============================
-- USER 2FA
-- ============================
CREATE TABLE IF NOT EXISTS user_twofa (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    method TEXT NOT NULL,    -- 'totp', 'sms', 'email'
    secret TEXT,             -- base32 secret for TOTP
    is_enabled BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT unique_user_method UNIQUE (user_id, method)
);

-- BACKUP CODES (one row per code)
CREATE TABLE IF NOT EXISTS user_twofa_backup_codes (
    id BIGSERIAL PRIMARY KEY,
    twofa_id BIGINT NOT NULL REFERENCES user_twofa(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,         -- store hashed backup code
    is_used BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    used_at TIMESTAMPTZ
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_user_twofa_user_id ON user_twofa(user_id);
CREATE INDEX IF NOT EXISTS idx_user_twofa_method ON user_twofa(method);

CREATE INDEX IF NOT EXISTS idx_backup_codes_twofa_id ON user_twofa_backup_codes(twofa_id);
CREATE INDEX IF NOT EXISTS idx_backup_codes_is_used ON user_twofa_backup_codes(is_used);

COMMIT;