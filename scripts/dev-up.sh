#!/usr/bin/env bash
# dev-up.sh — Bring up the full SentinelCNAPP stack for local development/demo.
#
# Starts (in order):
#   1. Infrastructure  (postgres/redis/neo4j/nats via docker compose + local override)
#   2. All 11 Go microservices with README ports
#   3. Dev gateway on :8081 (stands in for Envoy)
#   4. Frontend on :3400 (3000/3100/3200 often taken by grafana/loki/tempo)
#   5. Demo seed data
#
# Usage:
#   ./scripts/dev-up.sh            # full stack
#   ./scripts/dev-up.sh --no-seed  # skip demo data
#   ./scripts/dev-up.sh --build    # force rebuild binaries
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN="$ROOT/bin"
LOGS="$ROOT/logs"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-sentinel}"
export PATH="$HOME/.local/bin:$PATH"

NO_SEED=0
FORCE_BUILD=0
for arg in "$@"; do
  case "$arg" in
    --no-seed) NO_SEED=1 ;;
    --build) FORCE_BUILD=1 ;;
    *) echo "unknown flag: $arg" >&2; exit 1 ;;
  esac
done

mkdir -p "$BIN" "$LOGS"
: > "$LOGS/pids"

log()  { printf '\033[36m[dev-up]\033[0m %s\n' "$*"; }
err()  { printf '\033[31m[dev-up] ERROR:\033[0m %s\n' "$*" >&2; }

# ── 1. Infrastructure ───────────────────────────────────────────────────────
log "starting infrastructure (postgres:5434 redis:6381 neo4j:7687 nats:4222)..."
docker compose \
  -f "$ROOT/docker-compose.yml" \
  -f "$ROOT/docker-compose.local.yml" \
  -p "$COMPOSE_PROJECT" \
  up -d 2>&1 | grep -v "obsolete" || true

wait_for() {
  local name="$1" check_cmd="$2" max="${3:-60}"
  for i in $(seq 1 "$max"); do
    if bash -c "$check_cmd" >/dev/null 2>&1; then
      log "$name ready"
      return 0
    fi
    sleep 1
  done
  err "$name not ready after ${max}s"
  return 1
}

wait_for "postgres"  "docker exec ${COMPOSE_PROJECT}-postgres-1 pg_isready -U sentinel" 60
wait_for "redis"     "docker exec ${COMPOSE_PROJECT}-redis-1 redis-cli ping | grep -q PONG" 30
wait_for "neo4j"     "docker exec ${COMPOSE_PROJECT}-neo4j-1 cypher-shell -u neo4j -p changeme 'RETURN 1;'" 90
wait_for "nats"      "curl -sf http://localhost:8222/varz" 30

# ── 2. Build ────────────────────────────────────────────────────────────────
need_build=$FORCE_BUILD
if [ "$need_build" -eq 0 ]; then
  for d in "$ROOT"/services/*/; do
    name="$(basename "$d")"
    if [ ! -x "$BIN/$name" ] || [ -n "$(find "$d" -name '*.go' -newer "$BIN/$name" 2>/dev/null)" ]; then
      need_build=1
      break
    fi
  done
  [ -x "$BIN/dev-gateway" ] || need_build=1
fi

if [ "$need_build" -eq 1 ]; then
  log "building Go binaries → bin/ ..."
  for d in "$ROOT"/services/*/; do
    [ -f "$d/go.mod" ] || continue
    name="$(basename "$d")"
    (cd "$d" && go build -o "$BIN/$name" .)
    log "  built $name"
  done
  (cd "$ROOT" && go build -o "$BIN/dev-gateway" ./scripts/dev-gateway)
  log "  built dev-gateway"
else
  log "binaries up to date (use --build to force)"
fi

# ── 3. Common env ───────────────────────────────────────────────────────────
export SENTINEL_POSTGRES_URL="postgres://sentinel:changeme@localhost:5434/sentinel"
export SENTINEL_REDIS_URL="localhost:6381"
export SENTINEL_NEO4J_URI="bolt://localhost:7687"
export SENTINEL_NEO4J_USER="neo4j"
export SENTINEL_NEO4J_PASSWORD="changeme"
export SENTINEL_NATS_URL="nats://localhost:4222"
export SENTINEL_AWS_ACCOUNT_ID="123456789012"   # skip STS GetCallerIdentity
export SENTINEL_AWS_REGION="us-east-1"
export SENTINEL_AWS_REGIONS="us-east-1"
export SENTINEL_SYNC_INTERVAL="6h"
export SENTINEL_WEBHOOK_SECRET="dev-webhook-secret"

start() {
  local name="$1" port="$2" bin="$3"
  if curl -sf "http://localhost:${port}/health" >/dev/null 2>&1; then
    log "$name already listening on :$port (skip)"
    return 0
  fi
  # setsid+nohup: detach from this shell so services survive the launcher exiting.
  setsid nohup env SENTINEL_SERVICE_NAME="sentinel-$name" SENTINEL_SERVICE_PORT="$port" \
    "$bin" >>"$LOGS/$name.log" 2>&1 < /dev/null &
  echo "$!" >> "$LOGS/pids"
  log "started $name → :$port (log: logs/$name.log)"
}

wait_http() {
  local url="$1" name="$2" max="${3:-45}"
  for i in $(seq 1 "$max"); do
    if curl -sf "$url" >/dev/null 2>&1; then
      log "$name healthy"
      return 0
    fi
    sleep 1
  done
  err "$name unhealthy after ${max}s — last 30 log lines:"
  tail -30 "$LOGS/$name.log" 2>/dev/null || true
  return 1
}

# ── 4. Microservices ────────────────────────────────────────────────────────
log "starting microservices..."
start asset-inventory    8080 "$BIN/asset-inventory"
start correlation        8082 "$BIN/correlation"
start scanner-iac        8083 "$BIN/scanner-iac"
start scanner-container  8084 "$BIN/scanner-container"
start scanner-k8s        8085 "$BIN/scanner-k8s"
start scanner-secrets    8086 "$BIN/scanner-secrets"
start risk-engine        8087 "$BIN/risk-engine"
start attack-path        8088 "$BIN/attack-path"
start runtime            8089 "$BIN/runtime"
start remediation        8090 "$BIN/remediation"
start ai-assistant       8091 "$BIN/ai-assistant"

for pair in "asset-inventory:8080" "correlation:8082" "scanner-iac:8083" \
            "scanner-container:8084" "scanner-k8s:8085" "scanner-secrets:8086" \
            "risk-engine:8087" "attack-path:8088" "runtime:8089" \
            "remediation:8090" "ai-assistant:8091"; do
  name="${pair%%:*}"; port="${pair##*:}"
  wait_http "http://localhost:${port}/health" "$name" 45 || true
done

# ── 5. Gateway ──────────────────────────────────────────────────────────────
if curl -sf http://localhost:8081/health >/dev/null 2>&1; then
  log "dev-gateway already listening on :8081 (skip)"
else
  setsid nohup "$BIN/dev-gateway" >>"$LOGS/gateway.log" 2>&1 < /dev/null &
  echo "$!" >> "$LOGS/pids"
  wait_http "http://localhost:8081/health" "dev-gateway" 15 || true
fi

# ── 6. Seed demo data ───────────────────────────────────────────────────────
if [ "$NO_SEED" -eq 0 ]; then
  log "seeding demo data..."
  bash "$ROOT/scripts/seed-demo.sh" || err "seed failed (continuing)"
  # Give correlation a beat to invalidate any empty-state caches, then flush redis
  docker exec "${COMPOSE_PROJECT}-redis-1" redis-cli FLUSHDB >/dev/null 2>&1 || true
  log "redis cache flushed"
else
  log "skipping seed (--no-seed)"
fi

# Re-trigger risk evaluation now that findings exist
log "running risk evaluation pass..."
curl -sf -X POST http://localhost:8087/api/v1/risk/evaluate-all >/dev/null 2>&1 \
  && log "risk evaluation complete" \
  || log "risk evaluation skipped/failed (non-fatal)"

# ── 7. Frontend ─────────────────────────────────────────────────────────────
FRONTEND_PORT=3400
if ! ss -tln 2>/dev/null | grep -q ":3000 "; then
  FRONTEND_PORT=3000
fi

if curl -sf "http://localhost:${FRONTEND_PORT}" >/dev/null 2>&1; then
  log "frontend already listening on :$FRONTEND_PORT (skip)"
else
  log "starting frontend on :$FRONTEND_PORT..."
  if [ ! -d "$ROOT/frontend/.next" ] || [ "$FORCE_BUILD" -eq 1 ]; then
    (cd "$ROOT/frontend" && npm run build) || err "frontend build failed"
  fi
  # output:'standalone' → serve via node .next/standalone/server.js
  # (next start refuses to run with standalone output)
  if [ -d "$ROOT/frontend/.next/standalone" ]; then
    mkdir -p "$ROOT/frontend/.next/standalone/.next"
    cp -r "$ROOT/frontend/.next/static" "$ROOT/frontend/.next/standalone/.next/static" 2>/dev/null || true
    if [ -d "$ROOT/frontend/public" ]; then
      cp -r "$ROOT/frontend/public" "$ROOT/frontend/.next/standalone/public" 2>/dev/null || true
    fi
    setsid nohup env PORT="$FRONTEND_PORT" HOSTNAME="0.0.0.0" \
      NEXT_PUBLIC_API_URL="http://localhost:8081" \
      node "$ROOT/frontend/.next/standalone/server.js" >>"$LOGS/frontend.log" 2>&1 < /dev/null &
    echo "$!" >> "$LOGS/pids"
  else
    setsid nohup env NEXT_PUBLIC_API_URL="http://localhost:8081" \
      "$ROOT/frontend/node_modules/.bin/next" start -p "$FRONTEND_PORT" \
      >>"$LOGS/frontend.log" 2>&1 < /dev/null &
    echo "$!" >> "$LOGS/pids"
  fi
  wait_http "http://localhost:${FRONTEND_PORT}" "frontend" 60 || true
fi

# ── Summary ─────────────────────────────────────────────────────────────────
cat <<EOF

\033[32m✓ SentinelCNAPP stack is up\033[0m

  Frontend:      http://localhost:${FRONTEND_PORT}   (login: admin@sentinel-cnapp.io / password)
  Gateway:       http://localhost:8081
  Asset API:     http://localhost:8080/api/v1/assets
  Correlation:   http://localhost:8082/api/v1/findings
  Attack paths:  http://localhost:8088/api/v1/attack-paths/summary
  Risk engine:   http://localhost:8087/api/v1/risk/evaluate-all
  Remediation:   http://localhost:8090/api/v1/remediation/pending
  AI assistant:  http://localhost:8091/api/v1/ai/templates
  Runtime:       http://localhost:8089/api/v1/runtime/event
  Scanners:      :8083 (iac) :8084 (container) :8085 (k8s) :8086 (secrets)

  Logs:          logs/*.log
  Stop:          ./scripts/dev-down.sh
  Smoke test:    ./scripts/smoke-test.sh

EOF
