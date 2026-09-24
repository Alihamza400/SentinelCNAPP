#!/usr/bin/env bash
# dev-down.sh — Stop all SentinelCNAPP local processes + infrastructure.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
LOGS="$ROOT/logs"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-sentinel}"

log() { printf '\033[36m[dev-down]\033[0m %s\n' "$*"; }

# ── Kill tracked service PIDs ───────────────────────────────────────────────
if [ -f "$LOGS/pids" ]; then
  log "stopping local processes..."
  while read -r pid; do
    [ -n "$pid" ] || continue
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
      log "  killed pid $pid"
    fi
  done < "$LOGS/pids"
  rm -f "$LOGS/pids"
fi

# Safety net: kill any leftover listeners on our ports
for port in 8080 8081 8082 8083 8084 8085 8086 8087 8088 8089 8090 8091 3400; do
  pids=$(ss -tlnp 2>/dev/null | grep ":${port} " | grep -oP 'pid=\K[0-9]+' | sort -u || true)
  for pid in $pids; do
    # only kill if it looks like one of our processes
    if ps -p "$pid" -o args= 2>/dev/null | grep -qE 'bin/|next'; then
      kill "$pid" 2>/dev/null || true
      log "  killed listener on :$port (pid $pid)"
    fi
  done
done

# ── Stop infrastructure (keep volumes by default) ───────────────────────────
if [ "${1:-}" = "--purge" ]; then
  log "stopping infrastructure AND removing volumes..."
  docker compose \
    -f "$ROOT/docker-compose.yml" \
    -f "$ROOT/docker-compose.local.yml" \
    -p "$COMPOSE_PROJECT" down -v 2>&1 | grep -v obsolete || true
else
  log "stopping infrastructure (volumes kept; use --purge to remove)..."
  docker compose \
    -f "$ROOT/docker-compose.yml" \
    -f "$ROOT/docker-compose.local.yml" \
    -p "$COMPOSE_PROJECT" stop 2>&1 | grep -v obsolete || true
fi

log "done."
