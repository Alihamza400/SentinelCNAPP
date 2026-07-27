# SentinelCNAPP — Technology Strategy & Implementation Roadmap

## Problem → Technology → Step Mapping

This document maps every problem we identified directly to the **technology that solves it** and the **concrete implementation step** that delivers it.

---

## P1: Data Fragmentation — Findings Live in Silos

**Problem:** Trivy, Checkov, Gitleaks, Falco each produce isolated findings with no shared context. Attack chains are invisible.

### Solution Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                    SOLUTION: Normalize + Graph                    │
│                                                                  │
│  Step 1: Wrap each engine behind a common Finding protobuf       │
│  Step 2: Emit normalized findings to NATS event bus              │
│  Step 3: Correlation service consumes & writes to Neo4j          │
│  Step 4: Dashboard queries the graph (not the raw tools)         │
└──────────────────────────────────────────────────────────────────┘
```

### Technology Choices & Rationale

| Technology | Role | Why This? | Alternatives Considered |
|---|---|---|---|
| **Protobuf (buf)** | Schema definition | Language-agnostic, backwards-compatible, codegen for Go/TS | JSON Schema (no codegen), OpenAPI (REST-only) |
| **NATS JetStream** | Event bus | Go-native, persistent, 10µs latency, simpler than Kafka | Kafka (heavy), RabbitMQ (no persistence), Redis Streams (no exactly-once) |
| **Neo4j 5.x** | Correlation graph | Native graph DB, Cypher query language, property graph model | Apache Age (PostgreSQL extension, less mature), Dgraph (GraphQL-only, less tooling) |
| **Go** | All microservices | Same language as Trivy/Falco/K8s ecosystem; single binary; excellent concurrency | Python (GIL limits concurrent scanning), Rust (slower development velocity) |
| **Connect-Go** | gRPC framework | Native Go, no external runtime, supports gRPC + gRPC-Web | Standard gRPC-Go (heavier), Twirp (less adoption) |

### Implementation Steps

| Step | What | How | Output |
|---|---|---|---|
| **1.1** | Define `Finding` protobuf | `api/proto/finding/v1/finding.proto` | `Finding`, `Asset`, `ScanResult` messages |
| **1.2** | Build `pkg/finding` | Go package with validation, normalization helpers | Reusable finding library |
| **1.3** | Build `pkg/queue` | NATS abstraction: `Publish()`, `Subscribe()` | All services use one queue interface |
| **1.4** | Build `pkg/graph` | Neo4j driver with batch writer, retry, health check | Reusable graph client |
| **1.5** | Build `scanner-iac` | Go service → runs Checkov → parses JSON → emits normalised Finding to NATS | Container image: `sentinel/scanner-iac` |
| **1.6** | Build `scanner-container` | Go service → runs Trivy → parses JSON → emits Finding to NATS | Container image: `sentinel/scanner-container` |
| **1.7** | Build `scanner-k8s` | Go service → runs Trivy K8s + custom K8s API checks → emits Findings | Container image: `sentinel/scanner-k8s` |
| **1.8** | Build `scanner-secrets` | Go service → runs Gitleaks → parses JSON → emits Findings | Container image: `sentinel/scanner-secrets` |
| **1.9** | Build `correlation-service` | Consumes NATS → enriches with asset context → batch-writes to Neo4j | Container image: `sentinel/correlation` |

### Data Flow (Concrete)

```
Trivy scan of org/checkout:v2.3 emits:
  NATS topic: sentinel.scan.finding
  Payload: {
    source: "trivy",
    type: "vulnerability",
    severity: "high",
    asset_id: "ecr:org/checkout:v2.3",
    title: "CVE-2026-1234 in libssl 1.1.1",
    ...
  }

correlation-service receives → enriches → writes to Neo4j:
  MERGE (f:Finding {id: "trivy-cve-2026-1234"})
  SET f = {severity: "high", title: "CVE-2026-1234", ...}
  WITH f
  MATCH (a:Asset {id: "ecr:org/checkout:v2.3"})
  MERGE (f)-[:FOUND_IN]->(a)

Dashboard queries:
  MATCH (f:Finding)-[:FOUND_IN]->(a:Asset)
  WHERE a.id = "ecr:org/checkout:v2.3"
  RETURN f, a
```

---

## P2: Context Fragmentation — Severity Is Blind

**Problem:** CVSS scores ignore topology, data sensitivity, and reachability. Teams chase loud findings while real risks age.

### Solution Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│              SOLUTION: Graph-Based Risk Computation               │
│                                                                  │
│  Phase 4: Build the Risk Engine that walks the Neo4j graph       │
│  to compute context-aware severity from:                         │
│    • Asset internet-facing? → +weight                            │
│    • Asset handles PII? → +weight                                │
│    • Finding has known exploit? → +weight                        │
│    • Compensating control exists? → -weight                      │
│    • Path to critical asset exists? → +weight                    │
└──────────────────────────────────────────────────────────────────┘
```

### Technology Choices & Rationale

| Technology | Role | Why This? | Alternatives |
|---|---|---|---|
| **Neo4j Graph Data Science (GDS)** | Graph algorithms | Built-in centrality, pathfinding, PageRank | Custom BFS (reinventing wheel) |
| **Go risk-engine service** | Computation engine | Stateless, horizontally scalable, cacheable | Python (slower graph traversal loops) |
| **Redis** | Risk score cache | Sub-millisecond reads for dashboard | OpenSearch (slower, meant for search) |

### Implementation Steps (Phase 4 — Post-MVP)

| Step | What | How |
|---|---|---|
| **2.1** | Define risk factors | Reachability, data classification, exploit maturity, asset criticality, compensating controls |
| **2.2** | Annotate graph nodes | Add `internet_facing`, `data_classification`, `has_exploit` properties to Asset nodes |
| **2.3** | Build risk engine | Go service: walk Neo4j graph from a Finding → compute weighted risk score |
| **2.4** | Write risk to graph | `(Finding)-[:HAS_RISK {score: 8.5}]->(RiskEvaluation)` |
| **2.5** | Return risk in dashboard | All finding queries include `risk_score`; sortable, filterable |
| **2.6** | Cache results | Redis: TTL 5min for dashboard queries |

### Concrete Example

```
Query to compute risk for CVE-2026-1234:

MATCH (f:Finding {id: "trivy-cve-2026-1234"})-[:FOUND_IN]->(a:Asset)
OPTIONAL MATCH (a)-[:CONNECTS_TO]->(e:NetworkEndpoint)
OPTIONAL MATCH (a)-[:HAS_POLICY]->(p:Policy)-[:CAN_ACCESS]->(s3:S3Bucket {public: true})
OPTIONAL MATCH (f) WHERE f.cvss_score > 9.0
OPTIONAL MATCH (a) WHERE a.datatype = "pii"

RETURN f.id,
  CASE WHEN e.internet_facing THEN 3 ELSE 0 END AS reachability,
  CASE WHEN s3.public THEN 4 ELSE 0 END AS data_risk,
  CASE WHEN f.cvss_score > 9.0 THEN 2 ELSE 0 END AS severity_weight,
  CASE WHEN a.datatype = "pii" THEN 3 ELSE 0 END AS sensitivity

Total = sum(factors) → normalized to 0-10
```

---

## P3: Workflow Fragmentation — Detection Without Action

**Problem:** Ownership is in PagerDuty, deploy process in ArgoCD, tracking in Jira. No connection between them.

### Solution Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│           SOLUTION: Unified Dashboard + Status Lifecycle          │
│                                                                  │
│  Step 1: Asset ownership mapping (team, Slack channel, on-call)  │
│  Step 2: Finding status lifecycle (open → triaged → fixing →     │
│           resolved → auto-closed on re-scan)                     │
│  Step 3: Dashboard shows owner + deploy context + Jira link       │
│  Step 4: Webhook notifications to Slack/PagerDuty on new findings │
└──────────────────────────────────────────────────────────────────┘
```

### Technology Choices & Rationale

| Technology | Role | Why This? |
|---|---|---|
| **Next.js 14** | Dashboard frontend | SSR for SEO, RSC for performance, React ecosystem |
| **shadcn/ui + Tailwind** | UI components | Accessible, dark-mode first, composable, no design debt |
| **OpenSearch** | Full-text search over findings | Dashboards, saved searches, alerts |
| **PostgreSQL** | Ownership, status, lifecycle | Relational data (users, teams, settings) doesn't need a graph |
| **Casbin** | RBAC | Resource-level permissions per team per environment |

### Implementation Steps (MVP + Phase 4)

| Step | What | How | Phase |
|---|---|---|---|
| **3.1** | Asset-owner mapping | PostgreSQL: `asset_owners(asset_id, team_id, slack_channel, pagerduty_escalation)` | MVP |
| **3.2** | Finding status model | `status` enum: open, triaged, in_progress, resolved, false_positive, suppressed | MVP |
| **3.3** | Dashboard: asset detail view | Show asset info, linked findings, owner, tags | MVP |
| **3.4** | Dashboard: findings table | Unified view with status, owner, severity, age, source | MVP |
| **3.5** | Auto-resolve on re-scan | If scanner re-scans and finding is gone → auto-transition to `resolved` | Phase 3 |
| **3.6** | Slack notifications | Webhook on `critical` findings → notify asset owner's Slack channel | Phase 3 |
| **3.7** | Jira integration | "Create Jira" button on finding → pre-fills description, severity, asset context | Phase 4 |

---

## P4: Enterprise CNAPP Access Gap

**Problem:** Wiz/Orca/Prisma Cloud solve P1-P3 but cost $50k-$1M+/year. SMBs excluded.

### Solution Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│       SOLUTION: Open-Source, Self-Hostable, Kubernetes-Native     │
│                                                                  │
│  Step 1: Apache 2.0 license → free forever                      │
│  Step 2: Helm chart → deploy on any K8s in 5 minutes            │
│  Step 3: BYO infra (Postgres, Neo4j, Redis, NATS → all open-source) │
│  Step 4: No per-asset, per-user, per-scan pricing               │
│  Step 5: Community → open-core model (enterprise features as paid add-ons) │
└──────────────────────────────────────────────────────────────────┘
```

### Technology Choices & Rationale

| Technology | Role | Why This? |
|---|---|---|
| **Helm 3** | K8s packaging | Standard; one `helm install` deploys everything |
| **kind** | Dev K8s | Fast local cluster for development |
| **Tilt** | Dev live-reload | Code change → auto-build → auto-deploy in 2 seconds |
| **ArgoCD** | GitOps deployment | Declarative, self-healing, audit trail |
| **Distroless images** | Container security | No shell, no package manager, minimal attack surface |

### Implementation Steps

| Step | What | How |
|---|---|---|
| **4.1** | Apache 2.0 license | Add `LICENSE` file to repo root |
| **4.2** | Main Helm chart | `deploy/helm/sentinel-cnapp/` — umbrella chart with sub-charts |
| **4.3** | Dependency charts | Bitnami Postgres, Neo4j Helm, Redis, NATS |
| **4.4** | Dev environment | Tiltfile + kind config → `make dev` boots everything |
| **4.5** | CI container build | Multi-stage Dockerfiles with distroless targets |
| **4.6** | Documentation | `deploy/` README with quickstart, production tuning |

---

## P5: No Unified Data Model

**Problem:** No open-source platform connects cloud assets, identities, code, and runtime findings into one queryable graph.

### Solution Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│    SOLUTION: Unified Graph Schema (Asset → Identity → Code →      │
│               Runtime → Finding)                                 │
│                                                                  │
│  All data normalized to one graph model:                         │
│    (Asset {id, type, provider, region, tags})                    │
│    (Identity {id, type, arn, permissions, trust_policy})         │
│    (Code{id, repo, commit, path})                                │
│    (Finding {id, source, type, severity, status})                │
│                                                                  │
│  Edges connect them all. Query any path.                         │
└──────────────────────────────────────────────────────────────────┘
```

### Technology Choices & Rationale

| Technology | Role | Why This? |
|---|---|---|
| **Neo4j** | Graph storage | Native property graph, ACID, Cypher query language |
| **Cypher** | Query language | Declarative, pattern-matching, built for these traversals |
| **Go graph client** | Service-to-graph | Batch writes, connection pooling, query builder |

### Implementation Steps

| Step | What | How |
|---|---|---|
| **5.1** | Define full graph schema | `docs/graph-schema.md` — all node types, edge types, properties |
| **5.2** | Asset node | `MERGE (a:Asset {id: $id}) ON CREATE SET a.type, a.provider, ...` |
| **5.3** | Identity node | `MERGE (i:Identity {arn: $arn})` + `(a)-[:HAS_IDENTITY]->(i)` |
| **5.4** | Code node | `MERGE (c:Code {repo: $repo, commit: $commit})` + `(a)-[:DEFINED_BY]->(c)` |
| **5.5** | Finding node | `MERGE (f:Finding {id: $id})` + `(f)-[:FOUND_IN]->(a)` |
| **5.6** | Graph queries | Pre-built Cypher templates for dashboard: asset tree, attack path, risk score |

### Concrete Graph Traversal

```
"Show me all critical findings on internet-facing assets that handle PII,
 reachable from a public S3 bucket":

MATCH (f:Finding {severity: "critical"})-[:FOUND_IN]->(a:Asset {internet_facing: true, datatype: "pii"})
MATCH (s3:S3Bucket {public: true})-[:CAN_ACCESS]->(i:Identity)<-[:HAS_IDENTITY]-(a)
RETURN f.title, a.id, s3.bucket_name, i.arn
```

---

## Full Tech Stack Summary

| Layer | Technology | Version | Purpose |
|---|---|---|---|
| **Backend language** | Go | 1.22+ | All microservices |
| **Frontend** | Next.js + React + TypeScript | 14 / 18 / 5 | Dashboard |
| **UI framework** | shadcn/ui + TailwindCSS | Latest | Component library |
| **Graph database** | Neo4j | 5.x | Correlation graph |
| **Relational database** | PostgreSQL | 16 | Assets, users, settings, raw findings |
| **Cache** | Redis | 7.x | Session, rate limit, risk score cache |
| **Search** | OpenSearch | 2.x | Full-text search over findings |
| **Event bus** | NATS JetStream | 2.x | Service-to-service events |
| **API protocol** | Connect-Go (gRPC) | Latest | Service communication |
| **API gateway** | Envoy | 1.28+ | L7 routing, auth, rate limiting |
| **Auth** | OIDC + Casbin RBAC | — | Authentication + authorization |
| **Package management** | Go workspaces + npm | — | Monorepo dependency management |
| **Protobuf** | buf CLI | Latest | Schema management |
| **CI/CD** | GitHub Actions | — | Lint, test, build, scan |
| **GitOps** | ArgoCD | 2.x | Deployment |
| **Dev environment** | Tilt + kind | — | Live-reload development |
| **Container registry** | GitHub Container Registry | — | Image storage |
| **Observability** | OpenTelemetry + Prometheus + Grafana + Loki | Latest | Traces, metrics, logs |
| **Secrets** | HashiCorp Vault | 1.x | Secret management |
| **Testing** | testcontainers-go + Playwright + k6 | — | Integration, E2E, performance |
| **License** | Apache 2.0 | — | Open-source license |

---

## Build Sequence — Step by Step

### Phase 0: Foundation (Week 1)

```
Day 1-2: Monorepo scaffold
  ├── go.work, go.mod for each service
  ├── Next.js app skeleton
  ├── buf.yaml, proto directory
  └── Makefile (lint, test, build, dev)

Day 3-4: Shared libraries
  ├── pkg/finding — Finding protobuf + validation
  ├── pkg/config — Env/config file loader
  ├── pkg/logging — Structured JSON logger (slog)
  └── pkg/metrics — Prometheus counter/histogram helpers

Day 5: Dev environment
  ├── Tiltfile
  ├── kind config
  └── docker-compose for infra (Postgres, Neo4j, Redis, NATS)

Day 6-7: CI/CD + Helm
  ├── GitHub Actions: lint → test → build → scan
  ├── Helm chart skeleton
  └── Pre-commit hooks
```

### Phase 1: Asset Discovery + Auth (Weeks 2-3)

```
Week 2:
  ├── Asset protobuf model
  ├── asset-inventory service: AWS SDK, region enumeration
  ├── PostgreSQL asset store
  └── OIDC auth (Dex or Keycloak)

Week 3:
  ├── Envoy gateway config (routes, auth, rate limit)
  ├── RBAC (Casbin: resources, roles, permissions)
  ├── Dashboard shell: login, asset list
  └── API key management for service-to-service
```

### Phase 2: Scanning Integration (Weeks 4-6)

```
Week 4:
  ├── scanner-iac: Checkov wrapper, parsing, NATS emit
  ├── Finding normalization tests (Checkov JSON → Finding proto)
  └── GitHub webhook receiver for IaC scans

Week 5:
  ├── scanner-container: Trivy wrapper, ECR auth
  ├── scanner-k8s: Trivy K8s + custom checks
  └── Container registry webhook receiver

Week 6:
  ├── scanner-secrets: Gitleaks wrapper
  ├── Git webhook receiver (push events)
  ├── Integration tests for all scanners
  └── Scanner Helm charts
```

### Phase 3: Correlation + Dashboard (Weeks 7-8)

```
Week 7:
  ├── Neo4j schema (Asset, Finding, edges)
  ├── correlation-service: NATS → Neo4j batch writer
  ├── Graph query API (gRPC)
  └── Asset-finding linking logic

Week 8:
  ├── Dashboard: findings table (unified, filterable, searchable)
  ├── Dashboard: asset detail with graph viz
  ├── Dashboard: interactive graph (react-force-graph)
  └── OpenSearch integration for full-text search
```

### Phase 4: Testing + Hardening (Week 9)

```
Week 9:
  ├── E2E tests: deploy sample app, run scans, verify graph
  ├── Performance: k6, Neo4j query profiling
  ├── Security: Trivy scan own images, SAST, DAST
  ├── Documentation: API docs, deploy guide
  └── Demo environment with seeded data
```

---

## Dependency Graph

```
                         ┌─────────────┐
                         │   Envoy GW  │
                         └──────┬──────┘
                                │
          ┌─────────────────────┼─────────────────────┐
          │                     │                     │
   ┌──────▼──────┐    ┌────────▼────────┐    ┌───────▼───────┐
   │  Asset Inv. │    │  All Scanners    │    │  Auth Service  │
   │  (Phase 1)  │    │  (Phase 2)       │    │  (Phase 1)     │
   └──────┬──────┘    └────────┬────────┘    └───────┬───────┘
          │                    │                      │
          └────────────────────┼──────────────────────┘
                               │
                        ┌──────▼──────┐
                        │    NATS     │
                        └──────┬──────┘
                               │
                        ┌──────▼──────┐
                        │ Correlation │  ← depends on Neo4j + PG
                        │  Service    │
                        └──────┬──────┘
                               │
                    ┌──────────┼──────────┐
                    │          │          │
              ┌─────▼──┐ ┌────▼───┐ ┌───▼──────┐
              │ Neo4j  │ │Postgres│ │Redis     │
              └────────┘ └────────┘ └──────────┘
                             │
                       ┌─────▼──────┐
                       │ OpenSearch  │
                       └────────────┘
```

**Build order must follow this dependency chain:**
1. Infra (Postgres, Neo4j, Redis, NATS, OpenSearch)
2. Shared libraries (pkg/*)
3. Auth service + Envoy gateway
4. Asset inventory
5. Scanners (any order — they're independent)
6. Correlation service (depends on assets + findings flowing)
7. Dashboard (depends on correlation API)
