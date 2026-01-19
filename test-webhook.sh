#!/bin/bash

# Test script for FastShip webhook endpoints
# Usage: ./test-webhook.sh [base_url] [secret]

BASE_URL="${1:-http://localhost:9000}"
SECRET="${2:-test-secret-123}"

echo "Testing FastShip at: $BASE_URL"
echo "Using secret: $SECRET"
echo ""

# Test health endpoint
echo "=== Testing Health Endpoint ==="
curl -s "$BASE_URL/api/health" | jq .
echo ""

# Test deploy endpoint (simple secret)
echo "=== Testing Deploy Endpoint (Simple Secret) ==="
curl -X POST "$BASE_URL/api/deploy/myapp" \
  -H "X-FastShip-Secret: $SECRET" \
  -H "Content-Type: application/json" \
  -d '{"image": "ghcr.io/user/myapp:latest"}' \
  -s | jq .
echo ""

# Test deploy endpoint with HMAC
echo "=== Testing Deploy Endpoint (HMAC Signature) ==="
BODY='{"image": "ghcr.io/user/myapp:v1.0.0"}'
SIGNATURE=$(echo -n "$BODY" | openssl dgst -sha256 -hmac "$SECRET" | awk '{print $2}')
curl -X POST "$BASE_URL/api/deploy/myapp" \
  -H "X-FastShip-Secret: sha256=$SIGNATURE" \
  -H "Content-Type: application/json" \
  -d "$BODY" \
  -s | jq .
echo ""

# Test list deployments
echo "=== Testing List Deployments ==="
curl -s "$BASE_URL/api/deployments" | jq .
echo ""

# Test invalid service
echo "=== Testing Invalid Service ==="
curl -X POST "$BASE_URL/api/deploy/nonexistent" \
  -H "X-FastShip-Secret: $SECRET" \
  -H "Content-Type: application/json" \
  -s | jq .
echo ""

# Test invalid auth
echo "=== Testing Invalid Auth ==="
curl -X POST "$BASE_URL/api/deploy/myapp" \
  -H "X-FastShip-Secret: wrong-secret" \
  -H "Content-Type: application/json" \
  -s | jq .
echo ""

echo "All tests completed!"
