#!/bin/bash
# Load test for Rate Limiter
# Usage: ./load_test.sh [base_url]
# Requires: curl, optionally hey (https://github.com/rakyll/hey)

BASE_URL="${1:-http://localhost:8080}"

echo "=== Rate Limiter Load Test ==="
echo "Target: $BASE_URL"
echo ""

# Quick connectivity check
echo "1. Readiness check..."
curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/ready"
echo " (expected: 200)"
echo ""

# Burst test: send 150 requests rapidly from same IP (limit 100/min)
echo "2. Burst test (150 requests, limit 100/min)..."
allowed=0
rejected=0
for i in $(seq 1 150); do
  code=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/")
  if [ "$code" = "200" ]; then
    ((allowed++))
  elif [ "$code" = "429" ]; then
    ((rejected++))
  fi
done
echo "   Allowed: $allowed, Rejected: $rejected (expected: ~100 allowed, ~50 rejected)"
echo ""

# Using hey if available for more detailed load test
if command -v hey &> /dev/null; then
  echo "3. Load test with hey (1000 req, 50 concurrent, 10 unique IPs)..."
  hey -n 1000 -c 50 -H "X-Forwarded-For: 10.0.0.1" "$BASE_URL/" 2>/dev/null | tail -15
else
  echo "3. Install 'hey' for detailed load stats: go install github.com/rakyll/hey@latest"
fi

echo ""
echo "=== Metrics available at http://localhost:9090/metrics ==="
