#!/bin/bash
# Heavy Load Test - Target: 10,000+ receipts/second
# Tests: Sustained load, spike testing, stress testing, soak testing

set -e

GRPC_HOST="localhost:8026"
PROTO_PATH="shared/proto/shared/accounting/receipt_v3.proto"
SERVICE="accounting.receipt.v3.ReceiptService"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test Configuration
WARM_UP_DURATION="30s"
SUSTAINED_DURATION="300s"  # 5 minutes
SPIKE_DURATION="60s"
SOAK_DURATION="600s"       # 10 minutes

TARGET_RPS_LOW=1000
TARGET_RPS_MEDIUM=5000
TARGET_RPS_HIGH=10000
TARGET_RPS_SPIKE=20000

echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║          Receipt Service Heavy Load Test                  ║${NC}"
echo -e "${GREEN}║          Target: 10,000+ receipts/second                  ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"

# Check prerequisites
if ! command -v ghz &> /dev/null; then
    echo -e "${RED}✗ ghz not installed${NC}"
    echo "Install: go install github.com/bojand/ghz/cmd/ghz@latest"
    exit 1
fi

if ! nc -z localhost 8026 2>/dev/null; then
    echo -e "${RED}✗ Service not running on localhost:8026${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Prerequisites OK${NC}\n"

# ===============================
# Generate Test Data
# ===============================
echo -e "${BLUE}Generating test data...${NC}"

python3 << 'PYTHON'
import json
import random

currencies = ["USD", "USDT", "BTC"]
transaction_types = [
    "TRANSACTION_TYPE_DEPOSIT",
    "TRANSACTION_TYPE_WITHDRAWAL",
    "TRANSACTION_TYPE_CONVERSION",
    "TRANSACTION_TYPE_TRADE",
    "TRANSACTION_TYPE_TRANSFER"
]

def generate_receipts(count, base_account_id=1):
    receipts = []
    for i in range(count):
        amount = random.randint(1000, 1000000)
        currency = random.choice(currencies)
        tx_type = random.choice(transaction_types)
        
        receipts.append({
            "transaction_type": tx_type,
            "account_type": "ACCOUNT_TYPE_REAL",
            "amount": amount,
            "currency": currency,
            "creditor": {
                "account_id": base_account_id + (i * 2),
                "owner_type": "OWNER_TYPE_USER"
            },
            "debitor": {
                "account_id": base_account_id + (i * 2) + 1,
                "owner_type": "OWNER_TYPE_SYSTEM"
            }
        })
    return receipts

# Small batches for high concurrency
with open('batch_10.json', 'w') as f:
    json.dump({"receipts": generate_receipts(10)}, f, indent=2)

with open('batch_50.json', 'w') as f:
    json.dump({"receipts": generate_receipts(50)}, f, indent=2)

with open('batch_100.json', 'w') as f:
    json.dump({"receipts": generate_receipts(100)}, f, indent=2)

with open('batch_500.json', 'w') as f:
    json.dump({"receipts": generate_receipts(500)}, f, indent=2)

# Generate read test data (non-existent codes for cache miss testing)
read_codes = [f"RCP-2025-{str(i).zfill(12)}" for i in range(1, 101)]
with open('batch_read_100.json', 'w') as f:
    json.dump({"codes": read_codes}, f, indent=2)

print("✅ Test data generated: batch_10, batch_50, batch_100, batch_500, batch_read_100")
PYTHON

echo -e "${GREEN}✓ Test data ready${NC}\n"

# ===============================
# Test Suite
# ===============================

run_test() {
    local test_name=$1
    local rps=$2
    local duration=$3
    local concurrency=$4
    local data_file=$5
    local call_type=$6
    
    echo -e "\n${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${YELLOW}${test_name}${NC}"
    echo -e "${YELLOW}RPS: ${rps} | Duration: ${duration} | Concurrency: ${concurrency}${NC}"
    echo -e "${YELLOW}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    ghz --insecure \
        --proto="$PROTO_PATH" \
        --call="${SERVICE}.${call_type}" \
        --rps=${rps} \
        --duration=${duration} \
        --concurrency=${concurrency} \
        --data-file=${data_file} \
        --format=pretty \
        --connections=50 \
        $GRPC_HOST \
        2>&1 | tee "results_${test_name// /_}.txt"
    
    echo -e "${GREEN}✓ ${test_name} completed${NC}"
}

# ===============================
# Phase 1: Warm-up (Ramp-up)
# ===============================
echo -e "\n${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Phase 1: Warm-up (1000 RPS for 30s)         ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"

run_test "Warm-up" $TARGET_RPS_LOW $WARM_UP_DURATION 50 "batch_10.json" "CreateReceiptsBatch"

sleep 5

# ===============================
# Phase 2: Baseline Load (Single Receipt)
# ===============================
echo -e "\n${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Phase 2: Baseline Single Receipt            ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"

ghz --insecure \
    --proto="$PROTO_PATH" \
    --call="${SERVICE}.CreateReceipt" \
    --total=10000 \
    --concurrency=100 \
    --data='{
      "transaction_type":"TRANSACTION_TYPE_DEPOSIT",
      "account_type":"ACCOUNT_TYPE_REAL",
      "amount":10000,
      "currency":"USD",
      "creditor":{"account_id":1,"owner_type":"OWNER_TYPE_USER"},
      "debitor":{"account_id":2,"owner_type":"OWNER_TYPE_SYSTEM"}
    }' \
    --format=pretty \
    $GRPC_HOST | tee "results_baseline_single.txt"

sleep 5

# ===============================
# Phase 3: Medium Load (5000 RPS)
# ===============================
echo -e "\n${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Phase 3: Medium Load (5000 RPS for 5min)    ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"

run_test "Medium Load - Batch 50" $TARGET_RPS_MEDIUM $SUSTAINED_DURATION 200 "batch_50.json" "CreateReceiptsBatch"

sleep 10

# ===============================
# Phase 4: High Load (10000 RPS)
# ===============================
echo -e "\n${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Phase 4: High Load (10000 RPS for 5min)     ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"

run_test "High Load - Batch 100" $TARGET_RPS_HIGH $SUSTAINED_DURATION 400 "batch_100.json" "CreateReceiptsBatch"

sleep 10

# ===============================
# Phase 5: Spike Test (20000 RPS)
# ===============================
echo -e "\n${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Phase 5: Spike Test (20000 RPS for 1min)    ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"

run_test "Spike Test - Batch 500" $TARGET_RPS_SPIKE $SPIKE_DURATION 800 "batch_500.json" "CreateReceiptsBatch"

sleep 10

# ===============================
# Phase 6: Read Performance (Cache Test)
# ===============================
echo -e "\n${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Phase 6: Read Performance (GetReceipt)      ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"

ghz --insecure \
    --proto="$PROTO_PATH" \
    --call="${SERVICE}.GetReceipt" \
    --total=100000 \
    --concurrency=500 \
    --data='{"code":"RCP-2025-000000000001","include_metadata":false}' \
    --format=pretty \
    $GRPC_HOST | tee "results_read_performance.txt"

sleep 5

# ===============================
# Phase 7: Batch Read Performance
# ===============================
echo -e "\n${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Phase 7: Batch Read (GetReceiptsBatch)      ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"

ghz --insecure \
    --proto="$PROTO_PATH" \
    --call="${SERVICE}.GetReceiptsBatch" \
    --total=10000 \
    --concurrency=200 \
    --data-file="batch_read_100.json" \
    --format=pretty \
    $GRPC_HOST | tee "results_batch_read.txt"

sleep 5

# ===============================
# Phase 8: Mixed Workload (70% Read, 30% Write)
# ===============================
echo -e "\n${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Phase 8: Mixed Workload (70R/30W for 5min)  ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"

(
    # 70% Reads
    ghz --insecure \
        --proto="$PROTO_PATH" \
        --call="${SERVICE}.GetReceipt" \
        --rps=$((TARGET_RPS_HIGH * 70 / 100)) \
        --duration=$SUSTAINED_DURATION \
        --concurrency=300 \
        --data='{"code":"RCP-2025-000000000001","include_metadata":false}' \
        --format=pretty \
        $GRPC_HOST > results_mixed_read.txt 2>&1 &
    
    # 30% Writes
    ghz --insecure \
        --proto="$PROTO_PATH" \
        --call="${SERVICE}.CreateReceiptsBatch" \
        --rps=$((TARGET_RPS_HIGH * 30 / 100)) \
        --duration=$SUSTAINED_DURATION \
        --concurrency=100 \
        --data-file="batch_100.json" \
        --format=pretty \
        $GRPC_HOST > results_mixed_write.txt 2>&1 &
    
    wait
)

echo -e "${GREEN}✓ Mixed workload completed${NC}"

sleep 10

# ===============================
# Phase 9: Soak Test (Long Duration)
# ===============================
echo -e "\n${BLUE}╔═══════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║  Phase 9: Soak Test (5000 RPS for 10min)     ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════╝${NC}"

run_test "Soak Test" $TARGET_RPS_MEDIUM $SOAK_DURATION 200 "batch_100.json" "CreateReceiptsBatch"

# ===============================
# Results Summary
# ===============================
echo -e "\n${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                  Load Test Complete!                      ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"

echo -e "\n${YELLOW}Results saved to:${NC}"
ls -lh results_*.txt

echo -e "\n${YELLOW}Quick Stats:${NC}"
for file in results_*.txt; do
    echo -e "\n${BLUE}$file:${NC}"
    grep -E "Requests/sec|Average|Slowest|Fastest|Success|Error" "$file" || true
done

echo -e "\n${YELLOW}Monitoring:${NC}"
echo -e "  Prometheus: http://localhost:9091"
echo -e "  Grafana:    http://localhost:3000"

echo -e "\n${YELLOW}Database Check:${NC}"
echo "SELECT COUNT(*) as total_receipts FROM fx_receipts;" | psql -U postgres -d pxyz_fx

echo -e "\n${GREEN}✓ All tests completed successfully!${NC}"