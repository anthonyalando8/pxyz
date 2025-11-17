#!/bin/bash
#services/common-services/fx-services/receipt-service/load_test.sh

# ===============================
# Receipt Service Load Test
# Target: 4000+ receipts per second
# ===============================

set -e

# Configuration
GRPC_HOST="localhost:8026"
DURATION="60s"
CONNECTIONS=100
TARGET_RPS=4000

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}Receipt Service Load Test${NC}"
echo -e "${GREEN}Target: ${TARGET_RPS} receipts/second${NC}"
echo -e "${GREEN}========================================${NC}"

# Check if ghz is installed
if ! command -v ghz &> /dev/null; then
    echo -e "${RED}Error: ghz is not installed${NC}"
    echo "Install with: go install github.com/bojand/ghz/cmd/ghz@latest"
    exit 1
fi

# Test 1: Single Receipt Creation (Baseline)
echo -e "\n${YELLOW}Test 1: Single Receipt Creation${NC}"
ghz --insecure \
    --proto=shared/proto/shared/accounting/receipt_v3.proto \
    --call=receipt.ReceiptService.CreateReceipt \
    --total=1000 \
    --concurrency=10 \
    --data='{"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":10000,"currency":"USD","creditor":{"account_id":1,"owner_type":"OWNER_TYPE_USER"},"debitor":{"account_id":2,"owner_type":"OWNER_TYPE_SYSTEM"}}' \
    $GRPC_HOST

# Test 2: Batch Creation (10 receipts per batch)
echo -e "\n${YELLOW}Test 2: Batch Creation (10 receipts/batch)${NC}"
ghz --insecure \
    --proto=shared/proto/shared/accounting/receipt_v3.proto \
    --call=receipt.ReceiptService.CreateReceiptsBatch \
    --total=1000 \
    --concurrency=50 \
    --data-file=batch_10.json \
    $GRPC_HOST

# Test 3: Batch Creation (100 receipts per batch)
echo -e "\n${YELLOW}Test 3: Batch Creation (100 receipts/batch)${NC}"
ghz --insecure \
    --proto=shared/proto/shared/accounting/receipt_v3.proto \
    --call=receipt.ReceiptService.CreateReceiptsBatch \
    --total=100 \
    --concurrency=50 \
    --data-file=batch_100.json \
    $GRPC_HOST

# Test 4: Sustained Load (4000 RPS for 60 seconds)
echo -e "\n${YELLOW}Test 4: Sustained Load - ${TARGET_RPS} RPS${NC}"
ghz --insecure \
    --proto=shared/proto/shared/accounting/receipt_v3.proto \
    --call=receipt.ReceiptService.CreateReceiptsBatch \
    --rps=${TARGET_RPS} \
    --duration=${DURATION} \
    --concurrency=${CONNECTIONS} \
    --data-file=batch_100.json \
    $GRPC_HOST

# Test 5: Read Performance (Get by Code)
echo -e "\n${YELLOW}Test 5: Read Performance (Cache Test)${NC}"
ghz --insecure \
    --proto=shared/proto/shared/accounting/receipt_v3.proto \
    --call=receipt.ReceiptService.GetReceipt \
    --total=10000 \
    --concurrency=100 \
    --data='{"code":"RCP-2025-000000012345"}' \
    $GRPC_HOST

# Test 6: Batch Read Performance
echo -e "\n${YELLOW}Test 6: Batch Read Performance${NC}"
ghz --insecure \
    --proto=shared/proto/shared/accounting/receipt_v3.proto \
    --call=receipt.ReceiptService.GetReceiptsBatch \
    --total=1000 \
    --concurrency=50 \
    --data-file=batch_read_100.json \
    $GRPC_HOST

# Test 7: Mixed Workload (70% Read, 30% Write)
echo -e "\n${YELLOW}Test 7: Mixed Workload (70% Read, 30% Write)${NC}"
(
    # 70% Reads
    ghz --insecure \
        --proto=shared/proto/shared/accounting/receipt_v3.proto \
        --call=receipt.ReceiptService.GetReceipt \
        --rps=$((TARGET_RPS * 70 / 100)) \
        --duration=30s \
        --concurrency=70 \
        --data='{"code":"RCP-2025-000000012345"}' \
        $GRPC_HOST &
    
    # 30% Writes
    ghz --insecure \
        --proto=shared/proto/shared/accounting/receipt_v3.proto \
        --call=receipt.ReceiptService.CreateReceiptsBatch \
        --rps=$((TARGET_RPS * 30 / 100)) \
        --duration=30s \
        --concurrency=30 \
        --data-file=batch_100.json \
        $GRPC_HOST &
    
    wait
)

# Summary
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Load Test Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo -e "\nCheck Prometheus metrics at: http://localhost:9091"
echo -e "Check Grafana dashboards at: http://localhost:3000"
echo -e "\nKey Metrics to Monitor:"
echo -e "  - receipt_create_duration_seconds (p95, p99)"
echo -e "  - receipt_cache_hit_total"
echo -e "  - grpc_requests_total"
echo -e "  - Database connection pool stats"

# Generate test data files
generate_test_data() {
    # batch_10.json
    cat > batch_10.json << 'EOF'
{
  "receipts": [
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":10000,"currency":"USD","creditor":{"account_id":1},"debitor":{"account_id":2}},
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":20000,"currency":"USD","creditor":{"account_id":3},"debitor":{"account_id":4}},
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":30000,"currency":"USD","creditor":{"account_id":5},"debitor":{"account_id":6}},
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":40000,"currency":"USD","creditor":{"account_id":7},"debitor":{"account_id":8}},
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":50000,"currency":"USD","creditor":{"account_id":9},"debitor":{"account_id":10}},
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":60000,"currency":"USD","creditor":{"account_id":11},"debitor":{"account_id":12}},
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":70000,"currency":"USD","creditor":{"account_id":13},"debitor":{"account_id":14}},
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":80000,"currency":"USD","creditor":{"account_id":15},"debitor":{"account_id":16}},
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":90000,"currency":"USD","creditor":{"account_id":17},"debitor":{"account_id":18}},
    {"transaction_type":"TRANSACTION_TYPE_DEPOSIT","amount":100000,"currency":"USD","creditor":{"account_id":19},"debitor":{"account_id":20}}
  ]
}
EOF

    # Generate batch_100.json (100 receipts)
    python3 << 'PYTHON'
import json

receipts = []
for i in range(100):
    receipts.append({
        "transaction_type": "TRANSACTION_TYPE_DEPOSIT",
        "amount": (i + 1) * 1000,
        "currency": "USD",
        "creditor": {"account_id": i * 2 + 1},
        "debitor": {"account_id": i * 2 + 2}
    })

with open('batch_100.json', 'w') as f:
    json.dump({"receipts": receipts}, f, indent=2)
PYTHON

    # Generate batch_read_100.json
    python3 << 'PYTHON'
import json

codes = [f"RCP-2025-{i:012d}" for i in range(100)]

with open('batch_read_100.json', 'w') as f:
    json.dump({"codes": codes}, f, indent=2)
PYTHON

    echo -e "${GREEN}Test data files generated${NC}"
}

# Generate test data before running tests
generate_test_data

# Benchmark Results Analyzer
analyze_results() {
    echo -e "\n${YELLOW}Analyzing Results...${NC}"
    
    # Query Prometheus for metrics
    curl -s "http://localhost:9091/api/v1/query?query=rate(receipt_create_total[1m])" | jq .
    curl -s "http://localhost:9091/api/v1/query?query=histogram_quantile(0.95,rate(receipt_create_duration_seconds_bucket[5m]))" | jq .
    curl -s "http://localhost:9091/api/v1/query?query=rate(receipt_cache_hit_total[1m])" | jq .
}

# Run analysis if Prometheus is available
if curl -s -f "http://localhost:9091/-/healthy" > /dev/null 2>&1; then
    analyze_results
fi