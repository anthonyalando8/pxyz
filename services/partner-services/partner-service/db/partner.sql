\c pxyz;

BEGIN;
-- Enums
CREATE TYPE partner_status_enum AS ENUM ('active','suspended');
CREATE TYPE partner_actor_type_enum AS ENUM ('system','partner_user','partner');

-- Partners table
CREATE TABLE partners (
  id            TEXT PRIMARY KEY,  -- string ID like PTxxxxxx
  name          TEXT NOT NULL,
  country       TEXT,
  contact_email TEXT,
  contact_phone TEXT,
  status        partner_status_enum NOT NULL DEFAULT 'active',
  service       TEXT,               -- new field: type of service the partner offers
  currency      TEXT,               -- new field: default currency for the partner
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_partners_name ON partners (name);


-- Partner users
CREATE TABLE partner_users (
  id         TEXT PRIMARY KEY,
  partner_id TEXT NOT NULL REFERENCES partners(id) ON DELETE CASCADE,
  role       VARCHAR(16) NOT NULL CHECK (role IN ('partner_admin','partner_user')),
  user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (partner_id, user_id)
);
CREATE INDEX idx_partner_users_partner_id ON partner_users (partner_id);

-- Partner KYC
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




-- CREATE VIEW partner_ledger AS
-- SELECT 
--     p.id              AS partner_id,
--     a.id              AS account_id,
--     ps.id             AS posting_id,
--     ps.amount,
--     ps.dr_cr,
--     ps.currency       AS local_currency,
--     fr.quote_currency AS denominator_currency,
--     fr.rate           AS fx_rate,
--     ps.created_at,
--     j.description,
--     j.external_ref
-- FROM postings ps
-- JOIN accounts a 
--     ON ps.account_id = a.id 
--     AND a.owner_type = 'partner'
-- JOIN partners p 
--     ON a.owner_id = p.id
-- JOIN journals j 
--     ON ps.journal_id = j.id
-- LEFT JOIN fx_rates fr 
--     ON ps.currency = fr.base_currency
--     -- optionally: AND fr.as_of = (SELECT MAX(as_of) FROM fx_rates WHERE base_currency = ps.currency)
-- ;



COMMIT;
