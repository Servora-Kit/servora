#!/usr/bin/env bash
# E2E test: IAM M2M Client Credentials Grant
#
# Tests the complete M2M flow:
#   1. Admin login  →  get access token
#   2. Create M2M application via IAM REST API
#   3. Exchange client_credentials for an OIDC token
#   4. Verify token response fields
#   5. Use M2M token to call a protected /v1 endpoint through Traefik
#   6. Cleanup — delete the test M2M application
#
# Environment variables:
#   IAM_API_URL    — REST API base URL (via Traefik, default: http://localhost:8080)
#                    Used for admin login and all /v1 routes; ForwardAuth injects X-User-ID.
#   IAM_OIDC_URL   — OIDC base URL (direct IAM service, default: http://localhost:8000)
#                    Used for /oauth/token; these routes are NOT registered in Traefik.
#   ADMIN_EMAIL    — Admin account email   (default: admin@servora.dev)
#   ADMIN_PASSWORD — Admin account password (default: changeme)
#
# Usage:
#   ./iam_m2m_e2e.sh
#   IAM_API_URL=http://localhost:8080 IAM_OIDC_URL=http://localhost:8000 ./iam_m2m_e2e.sh

set -euo pipefail

IAM_API_URL="${IAM_API_URL:-http://localhost:8080}"
IAM_OIDC_URL="${IAM_OIDC_URL:-http://localhost:8000}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@servora.dev}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme}"
APP_NAME="m2m-e2e-test-$(date +%s)"
APP_ID=""
ACCESS_TOKEN=""

PASS=0
FAIL=0

pass() { echo "[PASS] $1"; ((PASS++)) || true; }
fail() { echo "[FAIL] $1"; ((FAIL++)) || true; }

require_cmd() {
  if ! command -v "$1" &>/dev/null; then
    echo "ERROR: '$1' is required but not installed."
    exit 1
  fi
}

# Cleanup on unexpected exit — delete test app if it was created
cleanup() {
  if [ -n "$APP_ID" ] && [ -n "$ACCESS_TOKEN" ]; then
    curl -s --max-time 5 -X DELETE "$IAM_API_URL/v1/applications/$APP_ID" \
      -H "Authorization: Bearer $ACCESS_TOKEN" &>/dev/null || true
  fi
}
trap cleanup EXIT

require_cmd curl
require_cmd jq

echo "=== IAM M2M Client Credentials E2E Test ==="
echo "API:  $IAM_API_URL  (via Traefik)"
echo "OIDC: $IAM_OIDC_URL (direct IAM, /oauth not Traefik-routed)"
echo ""

# ------------------------------------------------------------------
# Step 1: Admin login
# ------------------------------------------------------------------
echo "--- Step 1: Admin login ---"
LOGIN_RESP=$(curl -s --max-time 10 -w "\n%{http_code}" -X POST "$IAM_API_URL/v1/auth/login/email-password" \
  -H "Content-Type: application/json" \
  --data-binary "$(jq -n --arg e "$ADMIN_EMAIL" --arg p "$ADMIN_PASSWORD" '{email:$e,password:$p}')" 2>&1)

LOGIN_HTTP=$(echo "$LOGIN_RESP" | tail -1)
LOGIN_BODY=$(echo "$LOGIN_RESP" | head -n -1)

if [ "$LOGIN_HTTP" != "200" ]; then
  echo "Response: $LOGIN_BODY"
  fail "Admin login HTTP $LOGIN_HTTP (is the IAM service running at $IAM_API_URL?)"
  exit 1
fi

ACCESS_TOKEN=$(echo "$LOGIN_BODY" | jq -r '.accessToken // empty')
if [ -z "$ACCESS_TOKEN" ]; then
  fail "Admin login: no accessToken in response"
  exit 1
fi
pass "Admin login succeeded (HTTP 200)"

# ------------------------------------------------------------------
# Step 2: Create M2M application
# ------------------------------------------------------------------
echo ""
echo "--- Step 2: Create M2M application ---"
APP_RESP=$(curl -s --max-time 10 -w "\n%{http_code}" -X POST "$IAM_API_URL/v1/applications" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary "$(jq -n --arg n "$APP_NAME" '{name:$n,type:"m2m",grant_types:["client_credentials"],scopes:["openid"]}')")

APP_HTTP=$(echo "$APP_RESP" | tail -1)
APP_BODY=$(echo "$APP_RESP" | head -n -1)

if [ "$APP_HTTP" != "200" ] && [ "$APP_HTTP" != "201" ]; then
  echo "Response: $APP_BODY"
  fail "Create M2M application HTTP $APP_HTTP"
  exit 1
fi

APP_ID=$(echo "$APP_BODY" | jq -r '.application.id // empty')
CLIENT_ID=$(echo "$APP_BODY" | jq -r '.application.clientId // empty')
CLIENT_SECRET=$(echo "$APP_BODY" | jq -r '.clientSecret // empty')

if [ -z "$APP_ID" ] || [ -z "$CLIENT_ID" ] || [ -z "$CLIENT_SECRET" ]; then
  fail "Create M2M application: missing id/clientId/clientSecret in response"
  exit 1
fi
pass "M2M application created (id=$APP_ID, client_id=$CLIENT_ID)"

# ------------------------------------------------------------------
# Step 3: Get OIDC token via client_credentials grant
# Note: /oauth/token is served by IAM's OIDC server directly;
#       it is NOT registered as a Traefik route, so we hit IAM_OIDC_URL.
# ------------------------------------------------------------------
echo ""
echo "--- Step 3: Get token via client_credentials (direct OIDC) ---"
TOKEN_RESP=$(curl -s --max-time 10 -w "\n%{http_code}" -X POST "$IAM_OIDC_URL/oauth/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  --data-urlencode "grant_type=client_credentials" \
  --data-urlencode "client_id=$CLIENT_ID" \
  --data-urlencode "client_secret=$CLIENT_SECRET" \
  --data-urlencode "scope=openid")

TOKEN_HTTP=$(echo "$TOKEN_RESP" | tail -1)
TOKEN_BODY=$(echo "$TOKEN_RESP" | head -n -1)

if [ "$TOKEN_HTTP" != "200" ]; then
  echo "Response: $TOKEN_BODY"
  fail "client_credentials token request HTTP $TOKEN_HTTP"
  exit 1
fi
pass "Token request HTTP 200"

# ------------------------------------------------------------------
# Step 4: Verify token response fields
# ------------------------------------------------------------------
echo ""
echo "--- Step 4: Verify token response ---"
M2M_ACCESS_TOKEN=$(echo "$TOKEN_BODY" | jq -r '.access_token // empty')
TOKEN_TYPE=$(echo "$TOKEN_BODY" | jq -r '.token_type // empty' | tr '[:upper:]' '[:lower:]')
EXPIRES_IN=$(echo "$TOKEN_BODY" | jq -r '.expires_in // empty')

if [ -n "$M2M_ACCESS_TOKEN" ]; then
  pass "access_token present in token response"
else
  fail "Token response missing access_token"
  echo "Response: $TOKEN_BODY"
fi

if [ "$TOKEN_TYPE" = "bearer" ]; then
  pass "token_type is bearer"
else
  fail "token_type expected 'bearer', got '$TOKEN_TYPE'"
fi

if [ -n "$EXPIRES_IN" ] && [ "$EXPIRES_IN" -gt 0 ] 2>/dev/null; then
  pass "expires_in=$EXPIRES_IN (positive)"
else
  fail "expires_in missing or invalid: '$EXPIRES_IN'"
fi

# ------------------------------------------------------------------
# Step 5: Use M2M token to call a protected /v1 endpoint via Traefik
# Traefik's ForwardAuth validates the token and injects X-User-ID.
# ------------------------------------------------------------------
echo ""
echo "--- Step 5: Verify M2M token works against protected API (via Traefik) ---"
if [ -n "$M2M_ACCESS_TOKEN" ]; then
  VERIFY_RESP=$(curl -s --max-time 10 -w "\n%{http_code}" -X GET "$IAM_API_URL/v1/applications" \
    -H "Authorization: Bearer $M2M_ACCESS_TOKEN")

  VERIFY_HTTP=$(echo "$VERIFY_RESP" | tail -1)
  VERIFY_BODY=$(echo "$VERIFY_RESP" | head -n -1)

  # M2M tokens may not have admin privileges; accept 200 (OK) or 403 (auth passed, authz denied)
  # but NOT 401 (auth failed) which means the token was rejected.
  if [ "$VERIFY_HTTP" = "401" ]; then
    fail "M2M token rejected by Traefik ForwardAuth (HTTP 401)"
    echo "Response: $VERIFY_BODY"
  elif [ "$VERIFY_HTTP" = "200" ] || [ "$VERIFY_HTTP" = "403" ]; then
    pass "M2M token accepted by Traefik ForwardAuth (HTTP $VERIFY_HTTP — not 401)"
  else
    fail "Unexpected HTTP $VERIFY_HTTP when using M2M token"
    echo "Response: $VERIFY_BODY"
  fi
else
  fail "Skipped step 5: no M2M access token available"
fi

# ------------------------------------------------------------------
# Step 6: Cleanup — delete the M2M application
# ------------------------------------------------------------------
echo ""
echo "--- Step 6: Cleanup ---"
DEL_RESP=$(curl -s --max-time 10 -w "\n%{http_code}" -X DELETE "$IAM_API_URL/v1/applications/$APP_ID" \
  -H "Authorization: Bearer $ACCESS_TOKEN")

DEL_HTTP=$(echo "$DEL_RESP" | tail -1)

if [ "$DEL_HTTP" = "200" ] || [ "$DEL_HTTP" = "204" ]; then
  pass "M2M application deleted (id=$APP_ID, HTTP $DEL_HTTP)"
  APP_ID=""  # Clear so trap doesn't retry
else
  fail "Delete returned HTTP $DEL_HTTP"
fi

# ------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------
echo ""
echo "=== Results ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"
echo ""
if [ "$FAIL" -eq 0 ]; then
  echo "✓ All tests passed"
  exit 0
else
  echo "✗ $FAIL test(s) failed"
  exit 1
fi
