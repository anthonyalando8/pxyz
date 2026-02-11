\c pxyz_fx_p2p;

BEGIN;

CREATE TABLE IF NOT EXISTS payment_methods(
    id SERIAL PRIMARY KEY,
    code varchar(255) NOT NULL,
    display_name varchar(255) NOT NULL,
    required_fields jsonb,
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE IF NOT EXISTS p2p_profiles(
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    username varchar(255) NOT NULL,
    phone_number varchar(20) ,
    email varchar(255),
    profile_picture_url varchar(255),
    payment_method_id INT REFERENCES payment_methods(id),
    metadata jsonb,
    joined_at TIMESTAMPTZ DEFAULT NOW(),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE IF NOT EXISTS p2p_ads(
    id SERIAL PRIMARY KEY,
    profile_id INT REFERENCES p2p_profiles(id),
    ad_type VARCHAR(10) NOT NULL CHECK (ad_type IN ('buy', 'sell')),
    payment_method_id INT REFERENCES payment_methods(id),
    currency_code VARCHAR(10) NOT NULL,
    exchange_rate NUMERIC(20, 8) NOT NULL,
    min_limit NUMERIC(20, 8) NOT NULL,
    max_limit NUMERIC(20, 8) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    status VARCHAR(20) NOT NULL CHECK (status IN ('active', 'inactive', 'suspended', 'paused')),
    metadata jsonb,
    terms_and_conditions TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE p2p_orders(
    id SERIAL PRIMARY KEY,
    order_id UUID NOT NULL,
    ad_id INT REFERENCES p2p_ads(id),
    cust_profile_id INT REFERENCES p2p_profiles(id),
    amount NUMERIC(20, 8) NOT NULL,
    currency_code VARCHAR(10) NOT NULL,
    exchange_rate NUMERIC(20, 8) NOT NULL,
    total_price NUMERIC(20, 8) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'completed', 'cancelled', 'disputed')),
    valid_until TIMESTAMPTZ NOT NULL,
    settled_at TIMESTAMPTZ,
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE p2p_transactions(
    id SERIAL PRIMARY KEY,
    transaction_id UUID NOT NULL,
    order_id INT REFERENCES p2p_orders(id),
    from_profile_id INT REFERENCES p2p_profiles(id),
    to_profile_id INT REFERENCES p2p_profiles(id),
    amount NUMERIC(20, 8) NOT NULL,
    currency_code VARCHAR(10) NOT NULL,
    exchange_rate NUMERIC(20, 8) NOT NULL,
    total_price NUMERIC(20, 8) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('initiated', 'in_progress', 'completed', 'failed')),
    proof_of_payment_files jsonb,
    external_transaction_reference varchar(255),
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE p2p_disputes(
    id SERIAL PRIMARY KEY,
    dispute_id UUID NOT NULL,
    order_id INT REFERENCES p2p_orders(id),
    raised_by_profile_id INT REFERENCES p2p_profiles(id),
    reason TEXT NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('open', 'resolved', 'rejected')),
    resolution_details TEXT,
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE p2p_reviews(
    id SERIAL PRIMARY KEY,
    review_id UUID NOT NULL,
    profile_id INT REFERENCES p2p_profiles(id),
    reviewer_profile_id INT REFERENCES p2p_profiles(id),
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE p2p_notifications(
    id SERIAL PRIMARY KEY,
    notification_id UUID NOT NULL,
    profile_id INT REFERENCES p2p_profiles(id),
    type VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE p2p_reported_profiles(
    id SERIAL PRIMARY KEY,
    report_id UUID NOT NULL,
    reported_profile_id INT REFERENCES p2p_profiles(id),
    reporter_profile_id INT REFERENCES p2p_profiles(id),
    reason TEXT NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('open', 'under_review', 'resolved', 'rejected')),
    resolution_details TEXT,
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE p2p_reported_ads(
    id SERIAL PRIMARY KEY,
    report_id UUID NOT NULL,
    reported_ad_id INT REFERENCES p2p_ads(id),
    reporter_profile_id INT REFERENCES p2p_profiles(id),
    reason TEXT NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('open', 'under_review', 'resolved', 'rejected')),
    resolution_details TEXT,
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE p2p_reported_orders(
    id SERIAL PRIMARY KEY,
    report_id UUID NOT NULL,
    reported_order_id INT REFERENCES p2p_orders(id),
    reporter_profile_id INT REFERENCES p2p_profiles(id),
    reason TEXT NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('open', 'under_review', 'resolved', 'rejected')),
    resolution_details TEXT,
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

CREATE TABLE p2p_chat_messages(
    id SERIAL PRIMARY KEY,
    message_id UUID NOT NULL,
    order_id INT REFERENCES p2p_orders(id),
    sender_profile_id INT REFERENCES p2p_profiles(id),
    receiver_profile_id INT REFERENCES p2p_profiles(id),
    message TEXT NOT NULL,
    file_attachments jsonb,
    is_read BOOLEAN DEFAULT FALSE,
    metadata jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
)

COMMIT;