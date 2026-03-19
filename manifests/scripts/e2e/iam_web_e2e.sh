#!/usr/bin/env bash
# E2E test: IAM Web Frontend (_auth/ routes)
#
# Tests the complete web authentication flow:
#   T01  /  → redirect to /login
#   T02  /login renders correctly
#   T03  Login with wrong credentials shows error
#   T04  Admin login succeeds and redirects to /dashboard
#   T05  Logout redirects back to /login
#   T06  /register renders correctly (incl. Cap widget)
#   T07  /reset-password renders correctly (request-reset view)
#   T08  Unknown route shows custom 404 component
#   T09  /login?authRequestID=... shows OIDC subtitle
#
# Dependencies:
#   - Node.js + npx (for Playwright)
#   - IAM web frontend running at WEB_URL (default: http://localhost:3000)
#   - IAM backend running at IAM_URL  (default: http://localhost:8080)
#
# Environment variables:
#   WEB_URL        — Frontend base URL (default: http://localhost:3000)
#   IAM_URL        — IAM backend base URL (default: http://localhost:8080)
#   ADMIN_EMAIL    — Admin email  (default: admin@servora.dev)
#   ADMIN_PASSWORD — Admin password (default: changeme)
#   HEADED         — Set to "1" to run browser in headed mode (default: headless)
#
# Usage:
#   ./iam_web_e2e.sh
#   HEADED=1 ./iam_web_e2e.sh

set -euo pipefail

WEB_URL="${WEB_URL:-http://localhost:3000}"
IAM_URL="${IAM_URL:-http://localhost:8080}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@servora.dev}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme}"
HEADED="${HEADED:-0}"

PASS=0
FAIL=0

pass() { echo "[PASS] $1"; ((PASS++)) || true; }
fail() { echo "[FAIL] $1"; ((FAIL++)) || true; }
section() { echo ""; echo "=== $1 ==="; }

require_cmd() {
  if ! command -v "$1" &>/dev/null; then
    echo "ERROR: '$1' is required but not installed."
    exit 1
  fi
}

require_cmd node
require_cmd npx
require_cmd curl

# ── Preflight ────────────────────────────────────────────────────────────────

section "Preflight"

# Check frontend is reachable
if curl -sf --max-time 5 "$WEB_URL/" -o /dev/null; then
  pass "Frontend reachable at $WEB_URL"
else
  fail "Frontend not reachable at $WEB_URL — is 'make web.dev' running?"
  exit 1
fi

# Check backend is reachable
if curl -sf --max-time 5 "$IAM_URL/healthz" -o /dev/null; then
  pass "Backend reachable at $IAM_URL"
else
  fail "Backend not reachable at $IAM_URL — is 'make compose.dev' running?"
  exit 1
fi

# ── Playwright runner ────────────────────────────────────────────────────────

HEADED_FLAG=""
if [ "$HEADED" = "1" ]; then
  HEADED_FLAG="--headed"
fi

# Write inline Playwright test script
PLAYWRIGHT_SCRIPT=$(mktemp /tmp/iam_web_e2e_XXXXXX.mjs)
trap 'rm -f "$PLAYWRIGHT_SCRIPT"' EXIT

cat > "$PLAYWRIGHT_SCRIPT" << 'PLAYWRIGHT_EOF'
import { chromium } from 'playwright';

const WEB_URL         = process.env.WEB_URL         ?? 'http://localhost:3000';
const ADMIN_EMAIL     = process.env.ADMIN_EMAIL     ?? 'admin@servora.dev';
const ADMIN_PASSWORD  = process.env.ADMIN_PASSWORD  ?? 'changeme';
const HEADED          = process.env.HEADED === '1';

let pass = 0;
let fail = 0;

function ok(name)   { console.log(`[PASS] ${name}`); pass++; }
function err(name, msg) { console.log(`[FAIL] ${name}: ${msg}`); fail++; }

async function run() {
  const browser = await chromium.launch({ headless: !HEADED });
  const ctx     = await browser.newContext();
  const page    = await ctx.newPage();

  // ── T01: / → /login redirect ─────────────────────────────────────────────
  try {
    await page.goto(`${WEB_URL}/`);
    await page.waitForURL(`${WEB_URL}/login**`, { timeout: 5000 });
    if (page.url().startsWith(`${WEB_URL}/login`)) {
      ok('T01: / redirects to /login');
    } else {
      err('T01: / redirects to /login', `landed on ${page.url()}`);
    }
  } catch (e) { err('T01: / redirects to /login', e.message); }

  // ── T02: Login page renders ───────────────────────────────────────────────
  try {
    await page.goto(`${WEB_URL}/login`);
    await page.waitForSelector('input[type="email"]', { timeout: 5000 });
    const hasEmail    = await page.locator('input[type="email"]').count() > 0;
    const hasPassword = await page.locator('input[type="password"]').count() > 0;
    const hasSubmit   = await page.locator('button[type="submit"]').count() > 0;
    const hasRegLink  = await page.getByRole('link', { name: '立即注册' }).count() > 0;
    const hasForgot   = await page.getByRole('link', { name: '忘记密码？' }).count() > 0;
    if (hasEmail && hasPassword && hasSubmit && hasRegLink && hasForgot) {
      ok('T02: Login page renders all elements');
    } else {
      err('T02: Login page renders all elements', 'one or more elements missing');
    }
  } catch (e) { err('T02: Login page renders all elements', e.message); }

  // ── T03: Wrong credentials shows error ───────────────────────────────────
  try {
    await page.goto(`${WEB_URL}/login`);
    await page.locator('input[type="email"]').fill('wrong@test.com');
    await page.locator('input[type="password"]').fill('wrongpassword');
    await page.locator('button[type="submit"]').click();
    await page.waitForSelector('text=invalid email or password', { timeout: 6000 });
    ok('T03: Wrong credentials shows error message');
  } catch (e) { err('T03: Wrong credentials shows error message', e.message); }

  // ── T04: Admin login succeeds → /dashboard ───────────────────────────────
  try {
    await page.goto(`${WEB_URL}/login`);
    await page.locator('input[type="email"]').fill(ADMIN_EMAIL);
    await page.locator('input[type="password"]').fill(ADMIN_PASSWORD);
    await page.locator('button[type="submit"]').click();
    await page.waitForURL(`${WEB_URL}/dashboard`, { timeout: 8000 });
    const onDashboard = page.url().startsWith(`${WEB_URL}/dashboard`);
    if (onDashboard) {
      ok('T04: Admin login succeeds → redirects to /dashboard');
    } else {
      err('T04: Admin login succeeds', `landed on ${page.url()}`);
    }
  } catch (e) { err('T04: Admin login succeeds → /dashboard', e.message); }

  // ── T05: Logout → /login ─────────────────────────────────────────────────
  try {
    // Assume we're on /dashboard after T04
    await page.getByRole('button', { name: /^A$/ }).click();
    await page.getByRole('menuitem', { name: '退出登录' }).click();
    await page.waitForURL(`${WEB_URL}/login**`, { timeout: 5000 });
    if (page.url().startsWith(`${WEB_URL}/login`)) {
      ok('T05: Logout redirects to /login');
    } else {
      err('T05: Logout redirects to /login', `landed on ${page.url()}`);
    }
  } catch (e) { err('T05: Logout redirects to /login', e.message); }

  // ── T06: Register page renders ───────────────────────────────────────────
  try {
    await page.goto(`${WEB_URL}/register`);
    await page.waitForSelector('input[type="email"]', { timeout: 5000 });
    const hasName     = await page.locator('input[placeholder="至少 5 个字符"]').count() > 0;
    const hasEmail    = await page.locator('input[type="email"]').count() > 0;
    const hasPassword = (await page.locator('input[type="password"]').count()) >= 2;
    const hasCap      = await page.locator('cap-widget').count() > 0;
    if (hasName && hasEmail && hasPassword && hasCap) {
      ok('T06: Register page renders all elements (incl. Cap widget)');
    } else {
      err('T06: Register page renders all elements', `name=${hasName} email=${hasEmail} pw=${hasPassword} cap=${hasCap}`);
    }
  } catch (e) { err('T06: Register page renders all elements', e.message); }

  // ── T07: Reset password page renders (request-reset view) ────────────────
  try {
    await page.goto(`${WEB_URL}/reset-password`);
    await page.waitForSelector('text=忘记密码', { timeout: 5000 });
    const hasHeading = await page.locator('h1:has-text("忘记密码")').count() > 0;
    const hasEmail   = await page.locator('input[type="email"]').count() > 0;
    const hasSubmit  = await page.locator('button:has-text("发送重置邮件")').count() > 0;
    const hasBack    = await page.getByRole('link', { name: '返回登录' }).count() > 0;
    if (hasHeading && hasEmail && hasSubmit && hasBack) {
      ok('T07: Reset password page renders (request-reset view)');
    } else {
      err('T07: Reset password page renders', `h=${hasHeading} email=${hasEmail} btn=${hasSubmit} back=${hasBack}`);
    }
  } catch (e) { err('T07: Reset password page renders', e.message); }

  // ── T08: 404 shows custom NotFound component ──────────────────────────────
  try {
    await page.goto(`${WEB_URL}/this-route-definitely-does-not-exist-xyz`);
    await page.waitForSelector('text=页面不存在', { timeout: 5000 });
    const has404Text = await page.locator('text=页面不存在').count() > 0;
    const hasLink    = await page.getByRole('link', { name: '返回登录' }).count() > 0;
    if (has404Text && hasLink) {
      ok('T08: Unknown route shows custom 404 component');
    } else {
      err('T08: Unknown route shows custom 404', `text=${has404Text} link=${hasLink}`);
    }
  } catch (e) { err('T08: Unknown route shows custom 404 component', e.message); }

  // ── T09: OIDC mode — authRequestID changes subtitle ──────────────────────
  try {
    await page.goto(`${WEB_URL}/login?authRequestID=test-oidc-request&redirect=`);
    await page.waitForSelector('text=请登录以继续授权', { timeout: 5000 });
    const hasOidcSubtitle = await page.locator('text=请登录以继续授权').count() > 0;
    if (hasOidcSubtitle) {
      ok('T09: OIDC mode login shows "请登录以继续授权" subtitle');
    } else {
      err('T09: OIDC mode login subtitle', 'subtitle not found');
    }
  } catch (e) { err('T09: OIDC mode login subtitle', e.message); }

  await browser.close();

  console.log('');
  console.log(`Results: ${pass} passed, ${fail} failed`);
  if (fail > 0) process.exit(1);
}

run().catch(err => {
  console.error('Fatal error:', err);
  process.exit(2);
});
PLAYWRIGHT_EOF

section "Running Playwright Tests"

# Install playwright if not present
if ! npx --yes playwright --version &>/dev/null 2>&1; then
  echo "Installing Playwright..."
  npx --yes playwright install chromium --with-deps
fi

WEB_URL="$WEB_URL" \
  ADMIN_EMAIL="$ADMIN_EMAIL" \
  ADMIN_PASSWORD="$ADMIN_PASSWORD" \
  HEADED="$HEADED" \
  node "$PLAYWRIGHT_SCRIPT"

EXIT_CODE=$?

section "Summary"
if [ $EXIT_CODE -eq 0 ]; then
  echo "All tests PASSED"
else
  echo "Some tests FAILED (exit code $EXIT_CODE)"
fi
exit $EXIT_CODE
