#!/usr/bin/env bash
# smoke-test.sh — Exercise every gateway endpoint and assert sane responses.
# Intended to run after dev-up.sh. Exit 0 = all checks passed.
set -uo pipefail

GW="${GATEWAY_URL:-http://localhost:8081}"
FE="${FRONTEND_URL:-http://localhost:3400}"
TOKEN="dev-mock-token-admin"
PASS=0
FAIL=0

log()  { printf '\033[36m[smoke]\033[0m %s\n' "$*"; }
ok()   { PASS=$((PASS+1)); printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  \033[31m✗\033[0m %s\n' "$1"; }

# check METHOD URL [json_body] [expect_substring]
check() {
  local method="$1" url="$2" body="${3:-}" expect="${4:-}"
  local args=(-sS -m 15 -X "$method" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json")
  if [ -n "$body" ]; then
    args+=(-d "$body")
  fi
  local out code
  out=$(curl "${args[@]}" -w '\n%{http_code}' "$url" 2>&1) || true
  code=$(printf '%s' "$out" | tail -n1)
  local payload
  payload=$(printf '%s' "$out" | sed '$d')

  if [ "$code" != "200" ] && [ "$code" != "202" ]; then
    bad "$method $url → HTTP $code  ($(printf '%s' "$payload" | head -c 160))"
    return 1
  fi
  if [ -n "$expect" ] && ! printf '%s' "$payload" | grep -qi -- "$expect"; then
    bad "$method $url → 200 but missing expected content '$expect': $(printf '%s' "$payload" | head -c 160)"
    return 1
  fi
  ok "$method $url → $code"
  # stash payload for callers that need IDs
  SMOKE_PAYLOAD="$payload"
}

log "gateway health..."
check GET "$GW/health" "" '"status"' || true

log "asset inventory..."
check GET "$GW/api/v1/assets?page_size=5" "" '"assets"' || true
ASSETS_PAYLOAD="$SMOKE_PAYLOAD"

log "correlation: findings + dashboard..."
check GET "$GW/api/v1/findings?page_size=5" "" '"findings"' || true
FINDINGS_PAYLOAD="$SMOKE_PAYLOAD"
check GET "$GW/api/v1/dashboard/stats" "" '"total_assets"' || true
check GET "$GW/api/v1/dashboard/severity-distribution" "" "" || true
check GET "$GW/api/v1/graph?limit=50" "" '"nodes"' || true

# Extract a finding ID for downstream checks
FINDING_ID=$(printf '%s' "$FINDINGS_PAYLOAD" | grep -oP '"id":\s*"\K[^"]+' | head -1 || true)
if [ -n "$FINDING_ID" ]; then
  log "using finding id: $FINDING_ID"
else
  log "WARN: no finding id extracted (seed may have failed)"
  FINDING_ID="checkov:arn:aws:s3:::checkout-logs:CKV_AWS_20_S3_public_access"
fi

log "attack path..."
check GET "$GW/api/v1/attack-paths/summary" "" '"total_paths"' || true

log "risk engine..."
check POST "$GW/api/v1/risk/evaluate-all" '{}' '"total"' || true
check GET "$GW/api/v1/risk/$FINDING_ID" "" '"overall_score"' || true

log "remediation workflow..."
check GET "$GW/api/v1/remediation/suggest/$FINDING_ID" "" '"remediations"' || true
check GET "$GW/api/v1/remediation/pending" "" '"remediations"' || true
check POST "$GW/api/v1/remediation/auto-execute" '{}' '"executed"' || true

log "ai assistant..."
check GET "$GW/api/v1/ai/templates" "" '"templates"' || true
check POST "$GW/api/v1/ai/query" '{"question":"List critical findings"}' '"cypher"' || true

log "runtime protection..."
check POST "$GW/api/v1/runtime/event" \
  '{"rule":"Terminal shell in container","output":"Anomalous process","priority":"WARNING","severity":"high","container_id":"abc123","container_name":"checkout"}' \
  '"status"' || true

log "scanner health (direct)..."
for pair in "scanner-iac:8083" "scanner-container:8084" "scanner-k8s:8085" "scanner-secrets:8086"; do
  name="${pair%%:*}"; port="${pair##*:}"
  if curl -sf "http://localhost:${port}/health" >/dev/null 2>&1; then
    ok "$name :$port/health"
  else
    bad "$name :$port/health unreachable"
  fi
done

log "frontend..."
# Frontend may be on 3000 or 3400 — probe both.
FE_OK=0
for port in 3400 3000; do
  if curl -sf "http://localhost:${port}" -o /dev/null 2>/dev/null; then
    ok "frontend responding on :$port"
    FE_OK=1
    break
  fi
done
[ "$FE_OK" -eq 1 ] || bad "frontend not responding on :3400 or :3000"

log "CORS preflight..."
code=$(curl -s -o /dev/null -w '%{http_code}' -X OPTIONS \
  -H "Origin: http://localhost:${FRONTEND_PORT:-3400}" \
  -H "Access-Control-Request-Method: GET" \
  "$GW/api/v1/findings" || true)
if [ "$code" = "204" ] || [ "$code" = "200" ]; then
  ok "OPTIONS preflight → $code"
else
  bad "OPTIONS preflight → $code"
fi

echo
if [ "$FAIL" -eq 0 ]; then
  printf '\033[32m✓ ALL %d CHECKS PASSED\033[0m\n' "$PASS"
  exit 0
else
  printf '\033[31m✗ %d/%d CHECKS FAILED\033[0m\n' "$FAIL" "$((PASS+FAIL))"
  exit 1
fi
