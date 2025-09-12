-- connect to your db
\c pxyz;

-- Case‑insensitive text for emails
CREATE EXTENSION IF NOT EXISTS citext;

-- Auto-update updated_at on UPDATE
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
    id           TEXT PRIMARY KEY,                 
    email        CITEXT UNIQUE,                    -- nullable, unique when present
    phone        VARCHAR(20) UNIQUE,               -- nullable, unique when present
    password     TEXT NOT NULL,
    first_name   VARCHAR(100),                      -- optional
    last_name    VARCHAR(100),                      -- optional
    is_banned    BOOLEAN NOT NULL DEFAULT FALSE,
    is_verified  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_email_or_phone_chk
      CHECK (email IS NOT NULL OR phone IS NOT NULL)
);

CREATE TRIGGER users_set_updated_at_trg
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE sessions (
    id           TEXT PRIMARY KEY,                 -- e.g. "ses_01J3..."
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device       TEXT,
    ip           INET,
    token        TEXT,                             
    last_active  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);

CREATE INDEX sessions_token_idx ON sessions(token);
