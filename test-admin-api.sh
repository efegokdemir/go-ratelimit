#!/bin/bash

# Test script for Rate Limiting Admin API
# Usage: ./test-admin-api.sh

API_BASE="http://localhost:8080"
ADMIN_KEY="admin-secret-key"
CLIENT_ID="test-client"

echo "====================================="
echo "Rate Limiting Admin API Test Suite"
echo "====================================="
echo ""

# Function to print colored output
print_test() {
    echo -e "\n\033[1;34m📝 $1\033[0m"
}

print_success() {
    echo -e "\033[1;32m✅ $1\033[0m"
}

print_error() {
    echo -e "\033[1;31m❌ $1\033[0m"
}

# Test 1: Health Check
print_test "Test 1: Health Check"
if curl -s -f "${API_BASE}/health" > /dev/null 2>&1; then
    print_success "Health check passed"
else
    print_error "Health check failed"
    exit 1
fi

# Test 2: Get default configuration
print_test "Test 2: Get default client configuration"
echo "Client ID: ${CLIENT_ID}"
CONFIG=$(curl -s -H "X-Admin-Key: ${ADMIN_KEY}" "${API_BASE}/admin/config/${CLIENT_ID}")
echo "Response: ${CONFIG}"

# Test 3: Update configuration
print_test "Test 3: Update client configuration"
echo "Setting: 20 requests, 2MB, per 60 seconds"
UPDATE_RESPONSE=$(curl -s -X PUT \
    -H "X-Admin-Key: ${ADMIN_KEY}" \
    -H "Content-Type: application/json" \
    -d '{"request_limit": 20, "byte_limit": 2097152, "window_secs": 60}' \
    "${API_BASE}/admin/config/${CLIENT_ID}")
echo "Response: ${UPDATE_RESPONSE}"

# Test 4: Verify update
print_test "Test 4: Verify configuration update"
NEW_CONFIG=$(curl -s -H "X-Admin-Key: ${ADMIN_KEY}" "${API_BASE}/admin/config/${CLIENT_ID}")
echo "New Config: ${NEW_CONFIG}"
if [[ "$NEW_CONFIG" == *"\"request_limit\":20"* ]]; then
    print_success "Configuration updated successfully"
else
    print_error "Configuration update failed"
fi

# Test 5: Rate limiting test
print_test "Test 5: Rate limiting enforcement (sending 25 requests)"
TENANT_ID="test-tenant"
SUCCESS_COUNT=0
RATE_LIMITED_COUNT=0

echo "Testing with tenant ID: ${TENANT_ID}"
for i in {1..25}; do
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Tenant-ID: ${TENANT_ID}" "${API_BASE}/api/test")
    if [ "$STATUS" = "200" ]; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
    elif [ "$STATUS" = "429" ]; then
        RATE_LIMITED_COUNT=$((RATE_LIMITED_COUNT + 1))
    fi
    printf "."
done
echo ""
echo "Results: ${SUCCESS_COUNT} successful, ${RATE_LIMITED_COUNT} rate limited"

# Test 6: Check rate limit headers
print_test "Test 6: Verify rate limit headers"
HEADERS=$(curl -s -I -H "X-Tenant-ID: ${TENANT_ID}" "${API_BASE}/api/test" 2>&1)
if [[ "$HEADERS" == *"Ratelimit-Request-Limit"* ]]; then
    print_success "Rate limit headers present"
    echo "$HEADERS" | grep -i "ratelimit"
else
    print_error "Rate limit headers missing"
fi

# Test 7: Reset configuration
print_test "Test 7: Reset client configuration to defaults"
RESET_RESPONSE=$(curl -s -X DELETE -H "X-Admin-Key: ${ADMIN_KEY}" "${API_BASE}/admin/config/${CLIENT_ID}")
echo "Response: ${RESET_RESPONSE}"
RESET_CONFIG=$(curl -s -H "X-Admin-Key: ${ADMIN_KEY}" "${API_BASE}/admin/config/${CLIENT_ID}")
echo "Config after reset: ${RESET_CONFIG}"
if [[ "$RESET_CONFIG" == *"\"request_limit\":10"* ]]; then
    print_success "Configuration reset successfully"
else
    print_error "Configuration reset failed"
fi

# Test 8: Test with missing admin key (should fail)
print_test "Test 8: Security test - Request without admin key"
UNAUTH_RESPONSE=$(curl -s -w "\n%{http_code}" "${API_BASE}/admin/config/${CLIENT_ID}" | tail -n 1)
if [ "$UNAUTH_RESPONSE" = "401" ]; then
    print_success "Unauthorized request properly rejected (401)"
else
    print_error "Security issue: Request should have been rejected"
fi

echo ""
echo "====================================="
echo "Test Suite Complete"
echo "====================================="
