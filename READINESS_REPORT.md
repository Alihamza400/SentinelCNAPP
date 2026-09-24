# SentinelCNAPP — Client Presentation Readiness Report

**Date:** 2026-09-24
**Base commit:** `e79663b` (stack verified green; this report follows)
**Status:** READY FOR DEMO

## Verification Summary

| Check | Result |
|---|---|
| Go build (11 services) | PASS (`go build` / `go vet` clean) |
| Go unit tests (root module) | PASS |
| Frontend lint + typecheck | PASS (2 pre-existing react-hooks warnings in `graph/page.tsx`) |
| Frontend production build (standalone) | PASS |
| Smoke test (21 API checks) | PASS — 21/21 |
| All frontend routes | 200 (dashboard, assets, findings, graph, attack-paths, remediation, ai-assistant, login, signup) |
| CORS preflight | 204 |
| Working tree | Clean; pushed to GitHub |

## What Is Running

- **Infra (docker compose project `sentinel`):** Postgres :5434, Neo4j 7687/7474, Redis :6381, NATS :4222/8222
- **11 Go microservices** on :8080–:8091 (asset-inventory, gateway, correlation, 4 scanners, risk-engine, attack-path, runtime, remediation, ai-assistant)
- **Frontend (Next.js standalone)** on **http://localhost:3400**

## Demo Flow

1. Open `http://localhost:3400` → redirected to **Login** (protected routes enforced).
2. Sign in: `admin@sentinel-cnapp.io` / `password` (or `viewer@sentinel-cnapp.io` / `password`; new signups work too).
3. Dashboard: 7 assets, 8 open findings, 4 critical — unified risk overview.
4. Walk **Assets → Findings → Attack Graph → Attack Paths → Remediation → AI Assistant**.
5. Seed data: 7 assets, 1 identity, 8 findings, 3 attack paths; graph renders 16 nodes / 11 edges.

## Session Todos

All 10 items completed (toolchain, tests, lint, build, infra, smoke test, frontend↔backend, audit, fixes, this report).

## Known Gaps (disclose or defer)

- **Auth is mock/local-only** (hardcoded users + localStorage signup); roles (`admin`/`viewer`/`user`) do not gate UI yet — Phase 1 plan is OIDC via Dex (`auth-provider.tsx:47`).
- **No multi-tenancy:** every account sees the same shared demo dataset.
- **Empty placeholders:** `services/ciem`, `services/compliance`, `services/cspm`, `test/*` (by design, Phase 2+).
- **README still says frontend :3000** — actual port is **3400**.
- Unit-test coverage exists for `pkg/finding` / `pkg/config`; service-level tests sparse.

## Restart (if needed)

```bash
./scripts/dev-down.sh && ./scripts/dev-up.sh   # then start services as in README/dev-up
./scripts/smoke-test.sh                        # expect 21/21
PORT=3400 HOSTNAME=0.0.0.0 node frontend/.next/standalone/server.js
```

**Bottom line:** Stack is green end-to-end, auth UI works, everything is committed and pushed — ready for client walkthrough.
