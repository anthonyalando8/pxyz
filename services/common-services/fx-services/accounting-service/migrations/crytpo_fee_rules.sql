-- migrations/002_crypto_fee_rules.sql

-- ============================================================================
-- TRON USDT WITHDRAWAL FEES
-- ============================================================================

-- Platform fee for USDT withdrawal (minimum 100 KES)
INSERT INTO transaction_fee_rules (
    rule_name,
    transaction_type,
    source_currency,
    fee_type,
    calculation_method,
    fee_value,
    min_fee,
    max_fee,
    priority,
    is_active
) VALUES (
    'USDT TRC20 Withdrawal - Platform Fee',
    'crypto_withdrawal',
    'USDT',
    'platform_fee',
    'fixed',
    1.00,           -- Base platform fee (1 USDT)
    0.77,           -- Minimum 100 KES (≈ 0.77 USD at 130 KES/USD)
    NULL,           -- No maximum
    10,
    true
);

-- Network fee for USDT (TRX-based, dynamic)
INSERT INTO transaction_fee_rules (
    rule_name,
    transaction_type,
    source_currency,
    target_currency,
    fee_type,
    calculation_method,
    fee_value,
    min_fee,
    max_fee,
    priority,
    is_active
) VALUES (
    'USDT TRC20 Withdrawal - Network Fee',
    'crypto_withdrawal',
    'USDT',
    'TRX',          -- Fee paid in TRX
    'network_fee',
    'fixed',
    0.00,           -- Will be calculated dynamically from blockchain
    0.00,
    NULL,
    20,             -- Higher priority (applied after platform fee)
    true
);

-- ============================================================================
-- TRX WITHDRAWAL FEES
-- ============================================================================

INSERT INTO transaction_fee_rules (
    rule_name,
    transaction_type,
    source_currency,
    fee_type,
    calculation_method,
    fee_value,
    min_fee,
    max_fee,
    priority,
    is_active
) VALUES (
    'TRX Withdrawal - Platform Fee',
    'crypto_withdrawal',
    'TRX',
    'platform_fee',
    'fixed',
    0.10,           -- 0.1 TRX platform fee
    0.10,
    NULL,
    10,
    true
);

-- ============================================================================
-- CRYPTO → USD CONVERSION FEES
-- ============================================================================

INSERT INTO transaction_fee_rules (
    rule_name,
    transaction_type,
    source_currency,
    target_currency,
    fee_type,
    calculation_method,
    fee_value,
    min_fee,
    max_fee,
    priority,
    is_active
) VALUES (
    'USDT to USD Conversion Fee',
    'crypto_conversion',
    'USDT',
    'USD',
    'platform_fee',
    'percentage',
    50,          -- 0.5%
    0.77,           -- Minimum 100 KES
    NULL,
    10,
    true
);

-- ============================================================================
-- TIERED FEES (For high-value transactions)
-- ============================================================================

INSERT INTO transaction_fee_rules (
    rule_name,
    transaction_type,
    source_currency,
    fee_type,
    calculation_method,
    fee_value,
    tiers,
    priority,
    is_active
) VALUES (
    'BTC Withdrawal - Tiered Platform Fee',
    'crypto_withdrawal',
    'BTC',
    'platform_fee',
    'tiered',
    0,
    '[
        {"min_amount": 0, "max_amount": 0.01, "fee_bps": 100, "fixed_fee": 0.0001},
        {"min_amount":  0.01, "max_amount": 0.1, "fee_bps": 75, "fixed_fee": 0.0001},
        {"min_amount":  0.1, "max_amount": null, "fee_bps": 50, "fixed_fee": 0.0001}
    ]':: jsonb,
    10,
    true
);