\c pxyz_fx_crypto;
BEGIN;
-- ============================================================================
-- 1. CRYPTO WALLETS - User wallet addresses per blockchain
-- ============================================================================

CREATE TABLE crypto_wallets (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             VARCHAR(255) NOT NULL,
    chain               VARCHAR(50) NOT NULL,           -- TRON, BITCOIN, ETHEREUM
    asset               VARCHAR(50) NOT NULL,           -- TRX, USDT, BTC
    
    -- Wallet credentials
    address             VARCHAR(255) NOT NULL UNIQUE,
    public_key          TEXT,
    encrypted_private_key TEXT NOT NULL,                -- Encrypted with master key
    encryption_version  VARCHAR(20) NOT NULL DEFAULT 'v1',
    
    -- Wallet metadata
    label               VARCHAR(255),                   -- User-friendly name
    is_primary          BOOLEAN NOT NULL DEFAULT true,
    is_active           BOOLEAN NOT NULL DEFAULT true,
    
    -- Balance tracking (cached from blockchain)
    balance             NUMERIC(30, 18) NOT NULL DEFAULT 0,
    last_balance_update TIMESTAMPTZ,
    
    -- Monitoring
    last_deposit_check  TIMESTAMPTZ,
    last_transaction_block BIGINT,                      -- Last processed block number
    
    -- Timestamps
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT chk_address_format CHECK (LENGTH(address) > 10)
);

-- Indexes
CREATE INDEX idx_crypto_wallets_user_id ON crypto_wallets(user_id);
CREATE INDEX idx_crypto_wallets_chain_asset ON crypto_wallets(chain, asset);
CREATE INDEX idx_crypto_wallets_address ON crypto_wallets(address);
CREATE INDEX idx_crypto_wallets_active ON crypto_wallets(is_active) WHERE is_active = true;
CREATE INDEX idx_crypto_wallets_deposit_check ON crypto_wallets(last_deposit_check) WHERE is_active = true;
CREATE UNIQUE INDEX uq_crypto_wallets_primary_per_asset
ON crypto_wallets(user_id, chain, asset)
WHERE is_primary = true;

COMMENT ON TABLE crypto_wallets IS 'User cryptocurrency wallet addresses and credentials';
COMMENT ON COLUMN crypto_wallets.encrypted_private_key IS 'Private key encrypted with AES-256';
COMMENT ON COLUMN crypto_wallets.balance IS 'Cached balance in smallest unit (SUN, Satoshi, Wei)';

-- ============================================================================
-- 2. CRYPTO TRANSACTIONS - All blockchain and internal transactions
-- ============================================================================

CREATE TYPE crypto_transaction_type AS ENUM (
    'deposit',              -- External → User wallet
    'withdrawal',           -- User wallet → External
    'internal_transfer',    -- User → User (ledger only, no blockchain)
    'conversion',           -- Crypto → USD or USD → Crypto (internal)
    'fee_payment'           -- Network fee transaction
);

CREATE TYPE crypto_transaction_status AS ENUM (
    'pending',              -- Initiated, not broadcast
    'broadcasting',         -- Being broadcast to blockchain
    'broadcasted',          -- Sent to blockchain, waiting confirmation
    'confirming',           -- On blockchain, waiting confirmations
    'confirmed',            -- Fully confirmed
    'completed',            -- Internal transfer completed
    'failed',               -- Transaction failed
    'cancelled'             -- User cancelled before broadcast
);

CREATE TABLE crypto_transactions (
    id                      BIGSERIAL PRIMARY KEY,
    transaction_id          UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    user_id                 VARCHAR(255) NOT NULL,
    
    -- Transaction classification
    type                    crypto_transaction_type NOT NULL,
    chain                   VARCHAR(50) NOT NULL,
    asset                   VARCHAR(50) NOT NULL,
    
    -- Addresses
    from_wallet_id          BIGINT REFERENCES crypto_wallets(id),
    from_address            VARCHAR(255) NOT NULL,
    to_wallet_id            BIGINT REFERENCES crypto_wallets(id),
    to_address              VARCHAR(255) NOT NULL,
    is_internal             BOOLEAN NOT NULL DEFAULT false,  -- Internal vs external
    
    -- Amounts (in smallest unit:  SUN, Satoshi, Wei)
    amount                  NUMERIC(30, 18) NOT NULL,
    
    -- Fees
    network_fee             NUMERIC(30, 18) DEFAULT 0,
    network_fee_currency    VARCHAR(50),                    -- TRX, BTC, ETH
    platform_fee            NUMERIC(30, 18) DEFAULT 0,
    platform_fee_currency   VARCHAR(50),
    total_fee               NUMERIC(30, 18) DEFAULT 0,
    
    -- Blockchain details (NULL for internal transfers)
    tx_hash                 VARCHAR(255) UNIQUE,
    block_number            BIGINT,
    block_timestamp         TIMESTAMPTZ,
    confirmations           INT DEFAULT 0,
    required_confirmations  INT DEFAULT 1,
    
    -- Gas/Energy details (chain-specific)
    gas_used                BIGINT,
    gas_price               NUMERIC(30, 18),
    energy_used             BIGINT,                         -- TRON
    bandwidth_used          BIGINT,                         -- TRON
    
    -- Status tracking
    status                  crypto_transaction_status NOT NULL DEFAULT 'pending',
    status_message          TEXT,
    
    -- Internal ledger reference (for internal transfers)
    accounting_tx_id        VARCHAR(255),                   -- Link to accounting ledger
    
    -- Metadata
    memo                    TEXT,
    metadata                JSONB,                          -- Chain-specific data
    
    -- Timestamps
    initiated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    broadcasted_at          TIMESTAMPTZ,
    confirmed_at            TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ,
    failed_at               TIMESTAMPTZ,
    
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT chk_amount_positive CHECK (amount > 0),
    CONSTRAINT chk_fees_non_negative CHECK (
        network_fee >= 0 AND 
        platform_fee >= 0 AND 
        total_fee >= 0
    ),
    CONSTRAINT chk_internal_no_txhash CHECK (
        (is_internal = false AND tx_hash IS NOT NULL) OR
        (is_internal = true AND tx_hash IS NULL)
    ),
    CONSTRAINT chk_confirmations CHECK (confirmations >= 0)
);

ALTER TABLE crypto_transactions
DROP CONSTRAINT chk_internal_no_txhash;

-- Indexes
CREATE INDEX idx_crypto_tx_user_id ON crypto_transactions(user_id);
CREATE INDEX idx_crypto_tx_type ON crypto_transactions(type);
CREATE INDEX idx_crypto_tx_status ON crypto_transactions(status);
CREATE INDEX idx_crypto_tx_chain_asset ON crypto_transactions(chain, asset);
CREATE INDEX idx_crypto_tx_hash ON crypto_transactions(tx_hash) WHERE tx_hash IS NOT NULL;
CREATE INDEX idx_crypto_tx_from_wallet ON crypto_transactions(from_wallet_id);
CREATE INDEX idx_crypto_tx_to_wallet ON crypto_transactions(to_wallet_id);
CREATE INDEX idx_crypto_tx_pending ON crypto_transactions(status) 
    WHERE status IN ('pending', 'broadcasting', 'broadcasted', 'confirming');
CREATE INDEX idx_crypto_tx_created_at ON crypto_transactions(created_at DESC);
CREATE INDEX idx_crypto_tx_accounting ON crypto_transactions(accounting_tx_id) 
    WHERE accounting_tx_id IS NOT NULL;

COMMENT ON TABLE crypto_transactions IS 'All cryptocurrency transactions (blockchain and internal)';
COMMENT ON COLUMN crypto_transactions.is_internal IS 'True for ledger-only transfers, false for blockchain';
COMMENT ON COLUMN crypto_transactions.tx_hash IS 'Blockchain transaction hash (NULL for internal)';

-- ============================================================================
-- 3. DEPOSIT MONITORING - Track incoming deposits
-- ============================================================================

CREATE TYPE deposit_status AS ENUM (
    'detected',             -- Seen on blockchain
    'pending',              -- Waiting for confirmations
    'confirmed',            -- Enough confirmations
    'credited',             -- Added to user balance
    'failed'                -- Detection/crediting failed
);

CREATE TABLE crypto_deposits (
    id                      BIGSERIAL PRIMARY KEY,
    deposit_id              UUID NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    
    -- Wallet receiving deposit
    wallet_id               BIGINT NOT NULL REFERENCES crypto_wallets(id),
    user_id                 VARCHAR(255) NOT NULL,
    
    -- Deposit details
    chain                   VARCHAR(50) NOT NULL,
    asset                   VARCHAR(50) NOT NULL,
    from_address            VARCHAR(255) NOT NULL,
    to_address              VARCHAR(255) NOT NULL,
    amount                  NUMERIC(30, 18) NOT NULL,
    
    -- Blockchain details
    tx_hash                 VARCHAR(255) NOT NULL,
    block_number            BIGINT NOT NULL,
    block_timestamp         TIMESTAMPTZ,
    confirmations           INT NOT NULL DEFAULT 0,
    required_confirmations  INT NOT NULL DEFAULT 1,
    
    -- Status
    status                  deposit_status NOT NULL DEFAULT 'detected',
    
    -- Linked transaction
    transaction_id          BIGINT REFERENCES crypto_transactions(id),
    
    -- User notification
    user_notified           BOOLEAN NOT NULL DEFAULT false,
    notified_at             TIMESTAMPTZ,
    notification_sent       BOOLEAN NOT NULL DEFAULT false,
    
    -- Timestamps
    detected_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    confirmed_at            TIMESTAMPTZ,
    credited_at             TIMESTAMPTZ,
    
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Constraints
    CONSTRAINT uq_deposit_tx_hash UNIQUE(tx_hash, to_address),
    CONSTRAINT chk_amount_positive CHECK (amount > 0)
);

-- Indexes
CREATE INDEX idx_crypto_deposits_wallet ON crypto_deposits(wallet_id);
CREATE INDEX idx_crypto_deposits_user ON crypto_deposits(user_id);
CREATE INDEX idx_crypto_deposits_status ON crypto_deposits(status);
CREATE INDEX idx_crypto_deposits_tx_hash ON crypto_deposits(tx_hash);
CREATE INDEX idx_crypto_deposits_pending ON crypto_deposits(status) 
    WHERE status IN ('detected', 'pending');
CREATE INDEX idx_crypto_deposits_notification ON crypto_deposits(user_notified, notification_sent) 
    WHERE user_notified = false OR notification_sent = false;

COMMENT ON TABLE crypto_deposits IS 'Incoming cryptocurrency deposits detected on blockchain';

-- ============================================================================
-- 4. BLOCKCHAIN SYNC STATUS - Track blockchain scanning progress
-- ============================================================================

CREATE TABLE blockchain_sync_status (
    id                      SERIAL PRIMARY KEY,
    chain                   VARCHAR(50) NOT NULL UNIQUE,
    
    -- Current sync status
    last_synced_block       BIGINT NOT NULL DEFAULT 0,
    current_block           BIGINT,
    blocks_behind           BIGINT,
    
    -- Sync health
    is_syncing              BOOLEAN NOT NULL DEFAULT false,
    last_sync_at            TIMESTAMPTZ,
    sync_error              TEXT,
    consecutive_errors      INT NOT NULL DEFAULT 0,
    
    -- Performance metrics
    blocks_per_minute       NUMERIC(10, 2),
    avg_scan_time_ms        INT,
    
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO blockchain_sync_status (chain, last_synced_block) VALUES
('TRON', 0),
('BITCOIN', 0)
ON CONFLICT (chain) DO NOTHING;

COMMENT ON TABLE blockchain_sync_status IS 'Blockchain synchronization status for deposit monitoring';

-- ============================================================================
-- 5. WITHDRAWAL APPROVALS - For manual review if needed
-- ============================================================================

CREATE TYPE withdrawal_approval_status AS ENUM (
    'pending_review',
    'approved',
    'rejected',
    'auto_approved'
);

CREATE TABLE withdrawal_approvals (
    id                      BIGSERIAL PRIMARY KEY,
    transaction_id          BIGINT NOT NULL REFERENCES crypto_transactions(id),
    user_id                 VARCHAR(255) NOT NULL,
    
    -- Withdrawal details
    amount                  NUMERIC(30, 18) NOT NULL,
    asset                   VARCHAR(50) NOT NULL,
    to_address              VARCHAR(255) NOT NULL,
    
    -- Risk assessment
    risk_score              INT,                            -- 0-100
    risk_factors            JSONB,
    requires_approval       BOOLEAN NOT NULL DEFAULT false,
    
    -- Approval
    status                  withdrawal_approval_status NOT NULL DEFAULT 'pending_review',
    approved_by             VARCHAR(255),
    approved_at             TIMESTAMPTZ,
    rejection_reason        TEXT,
    
    -- Auto-approval rules
    auto_approved_reason    TEXT,
    
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_withdrawal_approvals_tx ON withdrawal_approvals(transaction_id);
CREATE INDEX idx_withdrawal_approvals_status ON withdrawal_approvals(status);
CREATE INDEX idx_withdrawal_approvals_pending ON withdrawal_approvals(status) 
    WHERE status = 'pending_review';

COMMENT ON TABLE withdrawal_approvals IS 'Withdrawal approval workflow for high-risk transactions';

-- ============================================================================
-- 6. ADDRESS BOOK - User's saved external addresses
-- ============================================================================

CREATE TABLE crypto_address_book (
    id                      BIGSERIAL PRIMARY KEY,
    user_id                 VARCHAR(255) NOT NULL,
    
    -- Address details
    chain                   VARCHAR(50) NOT NULL,
    address                 VARCHAR(255) NOT NULL,
    label                   VARCHAR(255) NOT NULL,
    
    -- Verification
    is_verified             BOOLEAN NOT NULL DEFAULT false,
    verified_at             TIMESTAMPTZ,
    
    -- Usage tracking
    last_used_at            TIMESTAMPTZ,
    usage_count             INT NOT NULL DEFAULT 0,
    
    -- Whitelisting
    is_whitelisted          BOOLEAN NOT NULL DEFAULT false,
    whitelisted_at          TIMESTAMPTZ,
    
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT uq_user_chain_address UNIQUE(user_id, chain, address)
);

CREATE INDEX idx_address_book_user ON crypto_address_book(user_id);
CREATE INDEX idx_address_book_chain ON crypto_address_book(chain);

COMMENT ON TABLE crypto_address_book IS 'User-saved external cryptocurrency addresses';


-- ============================================================================
-- VIEWS - Convenient data access
-- ============================================================================

-- Active user wallets
CREATE OR REPLACE VIEW v_active_crypto_wallets AS
SELECT 
    w.id,
    w.user_id,
    w.chain,
    w.asset,
    w.address,
    w.balance,
    w.is_primary,
    w.last_balance_update,
    w.created_at
FROM crypto_wallets w
WHERE w.is_active = true;

-- Recent transactions
CREATE OR REPLACE VIEW v_recent_crypto_transactions AS
SELECT 
    t.id,
    t.transaction_id,
    t.user_id,
    t.type,
    t.chain,
    t.asset,
    t.from_address,
    t.to_address,
    t.amount,
    t.total_fee,
    t.status,
    t.tx_hash,
    t.is_internal,
    t.created_at,
    t.confirmed_at
FROM crypto_transactions t
ORDER BY t.created_at DESC;

-- Pending deposits
CREATE OR REPLACE VIEW v_pending_deposits AS
SELECT 
    d. id,
    d.user_id,
    d.chain,
    d.asset,
    d.amount,
    d.tx_hash,
    d.confirmations,
    d.required_confirmations,
    d. status,
    d.user_notified,
    d. detected_at,
    w.address as wallet_address
FROM crypto_deposits d
JOIN crypto_wallets w ON d. wallet_id = w.id
WHERE d.status IN ('detected', 'pending');

COMMENT ON VIEW v_active_crypto_wallets IS 'Active cryptocurrency wallets by user';
COMMENT ON VIEW v_recent_crypto_transactions IS 'Recent crypto transactions ordered by date';
COMMENT ON VIEW v_pending_deposits IS 'Deposits waiting for confirmation';

COMMIT;