-- ============================================
-- COMPLETE FX RATES SETUP (CORRECTED)
-- Realistic rates for simulation
-- ============================================

BEGIN;

INSERT INTO fx_rates (base_currency, quote_currency, rate, bid_rate, ask_rate, spread, source, valid_from, valid_to) VALUES
-- USD ↔ USDT (1:1 parity with tiny spread)
('USD', 'USDT', 1.000000000000000000, 0.999500000000000000, 1.000500000000000000, 0.001000, 'market_avg', now(), null),
('USDT', 'USD', 1.000000000000000000, 0.999500000000000000, 1.000500000000000000, 0.001000, 'market_avg', now(), null),

-- BTC/USD
('BTC', 'USD', 
 42000.000000000000000000,
 41916.000000000000000000,
 42084.000000000000000000,
 0.200000,
 'coinbase_pro',
 now(),
 null
),

-- USD/BTC (inverse with same spread)
('USD', 'BTC',
 0.000023809523809524,
 0.000023755656108597,
 0.000023863636363636,
 0.200000,
 'coinbase_pro',
 now(),
 null
),

-- BTC/USDT (slightly tighter spread than BTC/USD)
('BTC', 'USDT',
 42000.000000000000000000,
 41937.000000000000000000,
 42063.000000000000000000,
 0.150000,
 'binance',
 now(),
 null
),

-- USDT/BTC (inverse)
('USDT', 'BTC',
 0.000023809523809524,
 0.000023773584905660,
 0.000023845034168566,
 0.150000,
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