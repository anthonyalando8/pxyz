\c pxyz_fx_p2p;

BEGIN;

-- ============================================================================
-- ENUMS (Better than CHECK constraints for PostgreSQL)
-- ============================================================================

CREATE TYPE ad_type AS ENUM ('buy', 'sell');
CREATE TYPE ad_status AS ENUM ('active', 'inactive', 'suspended', 'paused', 'deleted');
CREATE TYPE order_status AS ENUM ('pending', 'payment_pending', 'payment_submitted', 'paid', 'completed', 'cancelled', 'disputed', 'expired');
CREATE TYPE transaction_status AS ENUM ('initiated', 'in_progress', 'completed', 'failed', 'refunded');
CREATE TYPE dispute_status AS ENUM ('open', 'under_review', 'resolved', 'rejected', 'escalated');
CREATE TYPE report_status AS ENUM ('open', 'under_review', 'resolved', 'rejected', 'dismissed');
CREATE TYPE message_type AS ENUM ('text', 'image', 'file', 'system');

-- ============================================================================
-- PAYMENT METHODS
-- ============================================================================

CREATE TABLE IF NOT EXISTS payment_methods (
    id SERIAL PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    required_fields JSONB, -- e.g., {"account_number": "string", "account_name": "string"}
    is_active BOOLEAN DEFAULT TRUE,
    icon_url VARCHAR(500),
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payment_methods_code ON payment_methods(code);
CREATE INDEX idx_payment_methods_active ON payment_methods(is_active) WHERE is_active = TRUE;

COMMENT ON TABLE payment_methods IS 'Available payment methods for P2P trades';
COMMENT ON COLUMN payment_methods.required_fields IS 'JSON schema defining required fields for this payment method';

-- ============================================================================
-- P2P PROFILES
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_profiles (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL UNIQUE, -- From auth module
    username VARCHAR(100) UNIQUE, -- Optional, can be null
    phone_number VARCHAR(20), -- Optional
    email VARCHAR(255), -- Optional
    profile_picture_url VARCHAR(500),
    
    -- Trading stats
    total_trades INT NOT NULL DEFAULT 0,
    completed_trades INT NOT NULL DEFAULT 0,
    cancelled_trades INT NOT NULL DEFAULT 0,
    avg_rating NUMERIC(3, 2) DEFAULT 0.00 CHECK (avg_rating >= 0 AND avg_rating <= 5),
    total_reviews INT NOT NULL DEFAULT 0,
    
    -- Status
    is_verified BOOLEAN DEFAULT FALSE,
    is_merchant BOOLEAN DEFAULT FALSE,
    is_suspended BOOLEAN DEFAULT FALSE,
    suspension_reason TEXT,
    suspended_until TIMESTAMPTZ,
    
    -- Preferences
    preferred_currency VARCHAR(10),
    preferred_payment_methods JSONB, -- Array of payment method IDs
    auto_reply_message TEXT,
    
    -- Metadata
    metadata JSONB,
    last_active_at TIMESTAMPTZ,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_avg_rating_range CHECK (avg_rating >= 0 AND avg_rating <= 5)
);

CREATE INDEX idx_p2p_profiles_user_id ON p2p_profiles(user_id);
CREATE INDEX idx_p2p_profiles_username ON p2p_profiles(username) WHERE username IS NOT NULL;
CREATE INDEX idx_p2p_profiles_verified ON p2p_profiles(is_verified) WHERE is_verified = TRUE;
CREATE INDEX idx_p2p_profiles_merchant ON p2p_profiles(is_merchant) WHERE is_merchant = TRUE;
CREATE INDEX idx_p2p_profiles_suspended ON p2p_profiles(is_suspended) WHERE is_suspended = TRUE;

COMMENT ON TABLE p2p_profiles IS 'P2P user profiles with trading statistics';
-- new
ALTER TABLE p2p_profiles 
ADD COLUMN has_consent BOOLEAN DEFAULT FALSE,
ADD COLUMN consented_at TIMESTAMPTZ;

-- Add index for consent
CREATE INDEX idx_p2p_profiles_consent ON p2p_profiles(has_consent) WHERE has_consent = TRUE;

-- ============================================================================
-- P2P ADS
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_ads (
    id BIGSERIAL PRIMARY KEY,
    ad_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id) ON DELETE CASCADE,
    
    -- Ad details
    ad_type ad_type NOT NULL,
    asset_code VARCHAR(10) NOT NULL, -- BTC, USDT, etc.
    fiat_currency VARCHAR(10) NOT NULL, -- USD, KES, NGN, etc.
    
    -- Pricing
    price_type VARCHAR(20) NOT NULL CHECK (price_type IN ('fixed', 'floating')), -- fixed price or market-based
    fixed_price NUMERIC(20, 8), -- For fixed price ads
    margin_percentage NUMERIC(5, 2), -- For floating price ads (e.g., +2.5% or -1.5%)
    
    -- Limits
    min_order_limit NUMERIC(20, 8) NOT NULL,
    max_order_limit NUMERIC(20, 8) NOT NULL,
    available_amount NUMERIC(20, 8) NOT NULL, -- Remaining amount available
    total_amount NUMERIC(20, 8) NOT NULL, -- Original total amount
    
    -- Time limits
    payment_time_limit INT NOT NULL DEFAULT 15, -- Minutes to complete payment
    
    -- Status
    status ad_status NOT NULL DEFAULT 'active',
    is_active BOOLEAN GENERATED ALWAYS AS (status = 'active') STORED,
    
    -- Requirements
    min_completion_rate INT CHECK (min_completion_rate >= 0 AND min_completion_rate <= 100),
    requires_verification BOOLEAN DEFAULT FALSE,
    
    -- Terms
    terms_and_conditions TEXT,
    auto_reply_message TEXT,
    
    -- Metadata
    metadata JSONB,
    view_count INT NOT NULL DEFAULT 0,
    order_count INT NOT NULL DEFAULT 0,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_limits CHECK (min_order_limit <= max_order_limit),
    CONSTRAINT chk_available_amount CHECK (available_amount >= 0 AND available_amount <= total_amount),
    CONSTRAINT chk_price CHECK (
        (price_type = 'fixed' AND fixed_price > 0) OR 
        (price_type = 'floating' AND margin_percentage IS NOT NULL)
    )
);

CREATE INDEX idx_p2p_ads_profile_id ON p2p_ads(profile_id);
CREATE INDEX idx_p2p_ads_status ON p2p_ads(status);
CREATE INDEX idx_p2p_ads_active ON p2p_ads(is_active) WHERE is_active = TRUE;
CREATE INDEX idx_p2p_ads_type_currency ON p2p_ads(ad_type, asset_code, fiat_currency) WHERE is_active = TRUE;
CREATE INDEX idx_p2p_ads_created_at ON p2p_ads(created_at DESC);

COMMENT ON TABLE p2p_ads IS 'P2P buy/sell advertisements';

-- ============================================================================
-- AD PAYMENT METHODS (Junction Table)
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_ad_payment_methods (
    id BIGSERIAL PRIMARY KEY,
    ad_id BIGINT NOT NULL REFERENCES p2p_ads(id) ON DELETE CASCADE,
    payment_method_id INT NOT NULL REFERENCES payment_methods(id) ON DELETE CASCADE,
    payment_details JSONB, -- Account details for this specific payment method
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(ad_id, payment_method_id)
);

CREATE INDEX idx_ad_payment_methods_ad ON p2p_ad_payment_methods(ad_id);
CREATE INDEX idx_ad_payment_methods_method ON p2p_ad_payment_methods(payment_method_id);

COMMENT ON TABLE p2p_ad_payment_methods IS 'Payment methods accepted by each ad';

-- ============================================================================
-- P2P ORDERS
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_orders (
    id BIGSERIAL PRIMARY KEY,
    order_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    ad_id BIGINT NOT NULL REFERENCES p2p_ads(id),
    
    -- Parties
    maker_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id), -- Ad creator
    taker_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id), -- Order placer
    
    -- Order details
    asset_code VARCHAR(10) NOT NULL,
    fiat_currency VARCHAR(10) NOT NULL,
    crypto_amount NUMERIC(20, 8) NOT NULL,
    fiat_amount NUMERIC(20, 8) NOT NULL,
    price_per_unit NUMERIC(20, 8) NOT NULL,
    
    -- Payment
    payment_method_id INT NOT NULL REFERENCES payment_methods(id),
    payment_details JSONB, -- Payment account details used
    
    -- Status
    status order_status NOT NULL DEFAULT 'pending',
    
    -- Timing
    expires_at TIMESTAMPTZ NOT NULL,
    payment_deadline TIMESTAMPTZ,
    paid_at TIMESTAMPTZ,
    released_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ,
    cancelled_by_profile_id BIGINT REFERENCES p2p_profiles(id),
    cancellation_reason TEXT,
    
    -- Chat room
    chat_room_id BIGINT, -- Reference to chat room (added later)
    
    -- Metadata
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_different_profiles CHECK (maker_profile_id != taker_profile_id)
);

CREATE INDEX idx_p2p_orders_ad_id ON p2p_orders(ad_id);
CREATE INDEX idx_p2p_orders_maker ON p2p_orders(maker_profile_id);
CREATE INDEX idx_p2p_orders_taker ON p2p_orders(taker_profile_id);
CREATE INDEX idx_p2p_orders_status ON p2p_orders(status);
CREATE INDEX idx_p2p_orders_created_at ON p2p_orders(created_at DESC);
CREATE INDEX idx_p2p_orders_expires_at ON p2p_orders(expires_at) WHERE status IN ('pending', 'payment_pending');

COMMENT ON TABLE p2p_orders IS 'P2P trade orders';

-- ============================================================================
-- CHAT ROOMS
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_chat_rooms (
    id BIGSERIAL PRIMARY KEY,
    room_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    order_id BIGINT NOT NULL UNIQUE REFERENCES p2p_orders(id) ON DELETE CASCADE,
    
    -- Participants
    participant_1_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    participant_2_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    
    -- Status
    is_active BOOLEAN DEFAULT TRUE,
    is_locked BOOLEAN DEFAULT FALSE, -- Lock after order completion/cancellation
    
    -- Metadata
    last_message_at TIMESTAMPTZ,
    total_messages INT NOT NULL DEFAULT 0,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_different_participants CHECK (participant_1_id != participant_2_id)
);

CREATE INDEX idx_chat_rooms_order ON p2p_chat_rooms(order_id);
CREATE INDEX idx_chat_rooms_participant_1 ON p2p_chat_rooms(participant_1_id);
CREATE INDEX idx_chat_rooms_participant_2 ON p2p_chat_rooms(participant_2_id);
CREATE INDEX idx_chat_rooms_active ON p2p_chat_rooms(is_active) WHERE is_active = TRUE;

COMMENT ON TABLE p2p_chat_rooms IS 'Chat rooms for P2P orders (one per order)';

-- ============================================================================
-- CHAT MESSAGES
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_chat_messages (
    id BIGSERIAL PRIMARY KEY,
    message_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    chat_room_id BIGINT NOT NULL REFERENCES p2p_chat_rooms(id) ON DELETE CASCADE,
    sender_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    
    -- Message content
    message_type message_type NOT NULL DEFAULT 'text',
    message TEXT, -- Required for text messages
    file_attachments JSONB, -- [{url, name, size, type}]
    
    -- Status
    is_read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMPTZ,
    is_deleted BOOLEAN DEFAULT FALSE,
    deleted_at TIMESTAMPTZ,
    
    -- System messages
    is_system_message BOOLEAN DEFAULT FALSE,
    system_event VARCHAR(100), -- 'order_created', 'payment_submitted', etc.
    
    -- Metadata
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_text_message CHECK (
        message_type != 'text' OR (message IS NOT NULL AND LENGTH(TRIM(message)) > 0)
    )
);

CREATE INDEX idx_chat_messages_room ON p2p_chat_messages(chat_room_id, created_at DESC);
CREATE INDEX idx_chat_messages_sender ON p2p_chat_messages(sender_profile_id);
CREATE INDEX idx_chat_messages_unread ON p2p_chat_messages(chat_room_id, is_read) WHERE is_read = FALSE;
CREATE INDEX idx_chat_messages_created_at ON p2p_chat_messages(created_at DESC);

COMMENT ON TABLE p2p_chat_messages IS 'Messages in P2P order chat rooms';

-- ============================================================================
-- TRANSACTIONS (Payment proof tracking)
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_transactions (
    id BIGSERIAL PRIMARY KEY,
    transaction_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    order_id BIGINT NOT NULL REFERENCES p2p_orders(id) ON DELETE CASCADE,
    
    -- Parties
    from_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    to_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    
    -- Transaction details
    asset_code VARCHAR(10) NOT NULL,
    fiat_currency VARCHAR(10) NOT NULL,
    crypto_amount NUMERIC(20, 8) NOT NULL,
    fiat_amount NUMERIC(20, 8) NOT NULL,
    price_per_unit NUMERIC(20, 8) NOT NULL,
    
    -- Status
    status transaction_status NOT NULL DEFAULT 'initiated',
    
    -- Payment proof
    proof_of_payment_files JSONB, -- [{url, name, upload_time}]
    external_transaction_reference VARCHAR(255), -- Bank ref, tx hash, etc.
    payment_notes TEXT,
    
    -- Accounting references
    accounting_tx_id VARCHAR(255), -- Reference to accounting module
    
    -- Metadata
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_p2p_transactions_order ON p2p_transactions(order_id);
CREATE INDEX idx_p2p_transactions_from ON p2p_transactions(from_profile_id);
CREATE INDEX idx_p2p_transactions_to ON p2p_transactions(to_profile_id);
CREATE INDEX idx_p2p_transactions_status ON p2p_transactions(status);

COMMENT ON TABLE p2p_transactions IS 'Payment transaction records for P2P orders';

-- ============================================================================
-- DISPUTES
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_disputes (
    id BIGSERIAL PRIMARY KEY,
    dispute_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    order_id BIGINT NOT NULL REFERENCES p2p_orders(id) ON DELETE CASCADE,
    raised_by_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    against_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    
    -- Dispute details
    reason TEXT NOT NULL,
    category VARCHAR(50), -- 'payment_not_received', 'payment_not_released', 'scam', etc.
    evidence_files JSONB, -- [{url, name, type}]
    
    -- Status
    status dispute_status NOT NULL DEFAULT 'open',
    
    -- Resolution
    resolved_by VARCHAR(255), -- Admin user ID
    resolution_details TEXT,
    resolved_at TIMESTAMPTZ,
    
    -- Metadata
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_different_dispute_parties CHECK (raised_by_profile_id != against_profile_id)
);

CREATE INDEX idx_p2p_disputes_order ON p2p_disputes(order_id);
CREATE INDEX idx_p2p_disputes_raised_by ON p2p_disputes(raised_by_profile_id);
CREATE INDEX idx_p2p_disputes_status ON p2p_disputes(status);
CREATE INDEX idx_p2p_disputes_created_at ON p2p_disputes(created_at DESC);

COMMENT ON TABLE p2p_disputes IS 'Trade disputes raised by users';

-- ============================================================================
-- REVIEWS
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_reviews (
    id BIGSERIAL PRIMARY KEY,
    review_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    order_id BIGINT NOT NULL REFERENCES p2p_orders(id) ON DELETE CASCADE,
    reviewed_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    reviewer_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    
    -- Review content
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    tags JSONB, -- ['fast', 'reliable', 'good_communication']
    
    -- Status
    is_visible BOOLEAN DEFAULT TRUE,
    is_edited BOOLEAN DEFAULT FALSE,
    edited_at TIMESTAMPTZ,
    
    -- Metadata
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_different_review_parties CHECK (reviewed_profile_id != reviewer_profile_id),
    UNIQUE(order_id, reviewer_profile_id) -- One review per user per order
);

CREATE INDEX idx_p2p_reviews_reviewed ON p2p_reviews(reviewed_profile_id);
CREATE INDEX idx_p2p_reviews_reviewer ON p2p_reviews(reviewer_profile_id);
CREATE INDEX idx_p2p_reviews_order ON p2p_reviews(order_id);
CREATE INDEX idx_p2p_reviews_visible ON p2p_reviews(is_visible) WHERE is_visible = TRUE;

COMMENT ON TABLE p2p_reviews IS 'User reviews after completed trades';

-- ============================================================================
-- NOTIFICATIONS
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_notifications (
    id BIGSERIAL PRIMARY KEY,
    notification_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id) ON DELETE CASCADE,
    
    -- Notification content
    type VARCHAR(50) NOT NULL, -- 'new_order', 'payment_received', 'dispute_raised', etc.
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    
    -- Related entities
    order_id BIGINT REFERENCES p2p_orders(id),
    ad_id BIGINT REFERENCES p2p_ads(id),
    dispute_id BIGINT REFERENCES p2p_disputes(id),
    
    -- Status
    is_read BOOLEAN DEFAULT FALSE,
    read_at TIMESTAMPTZ,
    
    -- Priority
    priority VARCHAR(20) DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    
    -- Metadata
    metadata JSONB,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_p2p_notifications_profile ON p2p_notifications(profile_id, created_at DESC);
CREATE INDEX idx_p2p_notifications_unread ON p2p_notifications(profile_id, is_read) WHERE is_read = FALSE;
CREATE INDEX idx_p2p_notifications_type ON p2p_notifications(type);

COMMENT ON TABLE p2p_notifications IS 'User notifications for P2P events';

-- ============================================================================
-- REPORTED PROFILES
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_reported_profiles (
    id BIGSERIAL PRIMARY KEY,
    report_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    reported_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    reporter_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    
    -- Report details
    reason TEXT NOT NULL,
    category VARCHAR(50), -- 'scam', 'harassment', 'spam', etc.
    evidence_files JSONB,
    related_order_id BIGINT REFERENCES p2p_orders(id),
    
    -- Status
    status report_status NOT NULL DEFAULT 'open',
    
    -- Resolution
    reviewed_by VARCHAR(255), -- Admin user ID
    resolution_details TEXT,
    action_taken VARCHAR(100), -- 'warning', 'suspension', 'no_action', etc.
    resolved_at TIMESTAMPTZ,
    
    -- Metadata
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_different_report_parties CHECK (reported_profile_id != reporter_profile_id)
);

CREATE INDEX idx_reported_profiles_reported ON p2p_reported_profiles(reported_profile_id);
CREATE INDEX idx_reported_profiles_reporter ON p2p_reported_profiles(reporter_profile_id);
CREATE INDEX idx_reported_profiles_status ON p2p_reported_profiles(status);

COMMENT ON TABLE p2p_reported_profiles IS 'User reports against other profiles';

-- ============================================================================
-- REPORTED ADS
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_reported_ads (
    id BIGSERIAL PRIMARY KEY,
    report_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    reported_ad_id BIGINT NOT NULL REFERENCES p2p_ads(id),
    reporter_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    
    -- Report details
    reason TEXT NOT NULL,
    category VARCHAR(50), -- 'misleading', 'scam', 'inappropriate', etc.
    evidence_files JSONB,
    
    -- Status
    status report_status NOT NULL DEFAULT 'open',
    
    -- Resolution
    reviewed_by VARCHAR(255),
    resolution_details TEXT,
    action_taken VARCHAR(100),
    resolved_at TIMESTAMPTZ,
    
    -- Metadata
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reported_ads_ad ON p2p_reported_ads(reported_ad_id);
CREATE INDEX idx_reported_ads_reporter ON p2p_reported_ads(reporter_profile_id);
CREATE INDEX idx_reported_ads_status ON p2p_reported_ads(status);

COMMENT ON TABLE p2p_reported_ads IS 'User reports against ads';

-- ============================================================================
-- REPORTED ORDERS
-- ============================================================================

CREATE TABLE IF NOT EXISTS p2p_reported_orders (
    id BIGSERIAL PRIMARY KEY,
    report_id UUID NOT NULL UNIQUE DEFAULT gen_random_uuid(),
    reported_order_id BIGINT NOT NULL REFERENCES p2p_orders(id),
    reporter_profile_id BIGINT NOT NULL REFERENCES p2p_profiles(id),
    
    -- Report details
    reason TEXT NOT NULL,
    category VARCHAR(50),
    evidence_files JSONB,
    
    -- Status
    status report_status NOT NULL DEFAULT 'open',
    
    -- Resolution
    reviewed_by VARCHAR(255),
    resolution_details TEXT,
    action_taken VARCHAR(100),
    resolved_at TIMESTAMPTZ,
    
    -- Metadata
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_reported_orders_order ON p2p_reported_orders(reported_order_id);
CREATE INDEX idx_reported_orders_reporter ON p2p_reported_orders(reporter_profile_id);
CREATE INDEX idx_reported_orders_status ON p2p_reported_orders(status);

COMMENT ON TABLE p2p_reported_orders IS 'User reports against orders';

-- ============================================================================
-- ADD FOREIGN KEY FOR CHAT ROOM IN ORDERS
-- ============================================================================

ALTER TABLE p2p_orders ADD CONSTRAINT fk_orders_chat_room 
    FOREIGN KEY (chat_room_id) REFERENCES p2p_chat_rooms(id) ON DELETE SET NULL;

CREATE INDEX idx_p2p_orders_chat_room ON p2p_orders(chat_room_id);

-- ============================================================================
-- TRIGGERS FOR UPDATED_AT
-- ============================================================================

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_payment_methods_updated_at BEFORE UPDATE ON payment_methods
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_p2p_profiles_updated_at BEFORE UPDATE ON p2p_profiles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_p2p_ads_updated_at BEFORE UPDATE ON p2p_ads
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_p2p_orders_updated_at BEFORE UPDATE ON p2p_orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_p2p_chat_rooms_updated_at BEFORE UPDATE ON p2p_chat_rooms
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_p2p_chat_messages_updated_at BEFORE UPDATE ON p2p_chat_messages
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_p2p_transactions_updated_at BEFORE UPDATE ON p2p_transactions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_p2p_disputes_updated_at BEFORE UPDATE ON p2p_disputes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_p2p_reviews_updated_at BEFORE UPDATE ON p2p_reviews
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_p2p_notifications_updated_at BEFORE UPDATE ON p2p_notifications
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;