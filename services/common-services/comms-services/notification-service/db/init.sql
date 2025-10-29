\c pxyz_user;
BEGIN;
-- Main notifications table
CREATE TABLE notifications (
  id BIGSERIAL PRIMARY KEY,
  request_id UUID UNIQUE,                -- idempotency for producer
  owner_type VARCHAR(20) NOT NULL,       -- 'user', 'partner', 'admin'
  owner_id BIGINT REFERENCES users(id) ON DELETE CASCADE, 
  event_type VARCHAR(100) NOT NULL,      -- e.g. 'transaction.completed'
  channel_hint VARCHAR(20)[] NOT NULL,   -- ['sms','email','inapp']
  title TEXT,
  body TEXT,
  payload JSONB,                         -- full payload (safely redacted)
  priority VARCHAR(10) DEFAULT 'normal' 
      CHECK (priority IN ('low','normal','high','critical')),
  status VARCHAR(20) DEFAULT 'pending' 
      CHECK (status IN ('pending','delivered','failed','cancelled')),

  -- NEW: visibility flag
  visible_in_app BOOLEAN DEFAULT true,   -- whether it shows in the bell/notifications list

  -- NEW: read/unread tracking
  read_at TIMESTAMPTZ,                   -- NULL = unread, timestamp = when marked read

  created_at TIMESTAMPTZ DEFAULT now(),
  delivered_at TIMESTAMPTZ,
  metadata JSONB
);

-- Per-channel delivery audit/log
CREATE TABLE notification_deliveries (
  id BIGSERIAL PRIMARY KEY,
  notification_id BIGINT NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
  channel VARCHAR(20) NOT NULL,          -- 'sms','email','inapp'
  recipient TEXT NOT NULL,               -- email, phone, device id
  template_name TEXT,
  status VARCHAR(20) DEFAULT 'pending' 
      CHECK (status IN ('pending','sent','failed','retrying')),
  attempt_count INT DEFAULT 0,
  last_attempt_at TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);

-- User preferences (opt-in/out, quiet hours, channel preferences)
CREATE TABLE notification_preferences (
  id BIGSERIAL PRIMARY KEY,
  owner_type VARCHAR(20) NOT NULL,
  owner_id TEXT NOT NULL,
  channel_preferences JSONB DEFAULT '{}'::jsonb, 
      -- e.g. {"email":"enabled","sms":"disabled","inapp":"enabled"}
  quiet_hours JSONB DEFAULT '[]'::jsonb, 
      -- e.g. [{"start":"22:00","end":"07:00"}]
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(owner_type, owner_id)
);

-- Indexes for scale
CREATE INDEX idx_notifications_owner 
    ON notifications (owner_type, owner_id);

CREATE INDEX idx_notifications_status 
    ON notifications (status);

CREATE INDEX idx_notifications_visible 
    ON notifications (owner_type, owner_id, visible_in_app, read_at);

CREATE INDEX idx_deliveries_status 
    ON notification_deliveries (status);

CREATE INDEX idx_preferences_owner 
    ON notification_preferences (owner_type, owner_id);
COMMIT;