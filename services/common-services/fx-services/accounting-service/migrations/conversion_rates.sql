-- ============================================
-- COMPLETE FX RATES SETUP (CORRECTED)
-- Realistic rates for simulation
-- ============================================

BEGIN;

INSERT INTO fx_rates (base_currency, quote_currency, rate, bid_rate, ask_rate, spread, source, valid_from, valid_to) VALUES

-- ============================================================================
-- USD ↔ USDT (Stablecoin Parity)
-- ============================================================================
('USD', 'USDT', 
 1.000000000000000000, 
 0.999500000000000000, 
 1.000500000000000000, 
 0.001000, 
 'market_avg', 
 now(), 
 null
),

('USDT', 'USD', 
 1.000000000000000000, 
 0.999500000000000000, 
 1.000500000000000000, 
 0.001000, 
 'market_avg', 
 now(), 
 null
),

-- ============================================================================
-- BTC ↔ USD (Bitcoin)
-- ============================================================================
('BTC', 'USD', 
 102500. 000000000000000000,  -- Updated:  $102,500 (current BTC price)
 102295.000000000000000000,  -- Bid: -0.2%
 102705.000000000000000000,  -- Ask: +0.2%
 0.200000, 
 'coinbase_pro', 
 now(), 
 null
),

('USD', 'BTC',
 0.000009756097560976,      -- 1 / 102500
 0.000009735599216100,      -- Inverse bid
 0.000009776536312849,      -- Inverse ask
 0.200000, 
 'coinbase_pro', 
 now(), 
 null
),

-- ============================================================================
-- BTC ↔ USDT (Bitcoin/Tether)
-- ============================================================================
('BTC', 'USDT',
 102500.000000000000000000,  -- Same as BTC/USD
 102346.250000000000000000,  -- Bid: -0.15%
 102653.750000000000000000,  -- Ask: +0.15%
 0.150000, 
 'binance', 
 now(), 
 null
),

('USDT', 'BTC',
 0.000009756097560976,
 0.000009741219963032,
 0.000009770994475138,
 0.150000, 
 'binance', 
 now(), 
 null
),

-- ============================================================================
-- TRX ↔ USD (Tron)
-- ============================================================================
('TRX', 'USD',
 0.250000000000000000,       -- $0.25 per TRX (current market rate)
 0.249375000000000000,       -- Bid: -0.25%
 0.250625000000000000,       -- Ask: +0.25%
 0.250000,
 'binance',
 now(),
 null
),

('USD', 'TRX',
 4.000000000000000000,       -- 1 USD = 4 TRX
 3.990000000000000000,       -- Bid
 4.010000000000000000,       -- Ask
 0.250000,
 'binance',
 now(),
 null
),

-- ============================================================================
-- TRX ↔ USDT (Tron/Tether) ✅ ADDED
-- ============================================================================
('TRX', 'USDT',
 0.250000000000000000,       -- $0.25 USDT per TRX
 0.249375000000000000,       -- Bid: -0.25%
 0.250625000000000000,       -- Ask: +0.25%
 0.250000,
 'binance',
 now(),
 null
),

('USDT', 'TRX',
 4.000000000000000000,       -- 1 USDT = 4 TRX
 3.990000000000000000,       -- Bid
 4.010000000000000000,       -- Ask
 0.250000,
 'binance',
 now(),
 null
),

-- ============================================================================
-- TRX ↔ BTC (Tron/Bitcoin) ✅ ADDED
-- ============================================================================
('TRX', 'BTC',
 0.000002439024390244,      -- 0.25 / 102500
 0.000002433902439024,      -- Bid
 0.000002444146341463,      -- Ask
 0.200000,
 'binance',
 now(),
 null
),

('BTC', 'TRX',
 410000.000000000000000000,  -- 102500 / 0.25 = 410,000 TRX per BTC
 409180.000000000000000000,  -- Bid
 410820.000000000000000000,  -- Ask
 0.200000,
 'binance',
 now(),
 null
);

COMMIT;
-- ============================================
-- VERIFICATION QUERIES
-- ============================================

-- Check all rates
SELECT 
    base_currency || '/' || quote_currency as pair,
    rate::numeric(12,2) as rate,
    bid_rate::numeric(12,2) as bid,
    ask_rate::numeric(12,2) as ask,
    spread,
    source
FROM fx_rates
WHERE base_currency IN ('USD', 'USDT', 'BTC')
  AND valid_to IS NULL
ORDER BY base_currency, quote_currency;

-- Verify inverse relationships
SELECT 
    r1.base_currency || '/' || r1.quote_currency as pair,
    r1.rate::numeric(12,8) as forward_rate,
    r2.rate::numeric(12,8) as inverse_rate,
    (1.0 / r1.rate)::numeric(12,8) as calculated_inverse,
    CASE 
        WHEN abs(r2.rate - (1.0/r1.rate)) < 0.00000001 
        THEN '✓ Match' 
        ELSE '✗ Mismatch' 
    END as check
FROM fx_rates r1
JOIN fx_rates r2 
    ON r1.base_currency = r2.quote_currency 
    AND r1.quote_currency = r2.base_currency
WHERE r1.base_currency IN ('USD', 'USDT', 'BTC')
  AND r1.valid_to IS NULL
  AND r2.valid_to IS NULL
ORDER BY r1.base_currency;