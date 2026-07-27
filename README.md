<div align="center">
  <img src="https://img.shields.io/badge/Status-Active-success" alt="Status">
  <img src="https://img.shields.io/badge/License-Apache%202.0-blue" alt="License">
  <img src="https://img.shields.io/badge/Go-1.22+-00ADD8" alt="Go">
  <img src="https://img.shields.io/badge/Next.js-14-000000" alt="Next.js">
  <img src="https://img.shields.io/badge/Neo4j-5.x-008CC1" alt="Neo4j">
  <img src="https://img.shields.io/badge/Kubernetes-326CE5" alt="K8s">
  <img src="https://img.shields.io/badge/AWS-FF9900" alt="AWS">
</div>

# SentinelCNAPP

**An open-source, unified Cloud-Native Application Protection Platform (CNAPP)** that correlates best-in-class open-source security tools (Trivy, Checkov, Gitleaks, Falco) under **one Neo4j correlation graph, one context-aware risk score, and one unified dashboard**.

> *Stop hunting across five tools. See the attack chain, not just the symptoms.*

---

## The Problem

Cloud security in 2026 has three interconnected gaps:

| Gap | Impact |
|---|---|
| **Tool Fragmentation** — 69% of security pros cite fragmented defenses as the #1 obstacle | Trivy finds CVEs, Checkov finds misconfigurations, Gitleaks finds secrets. Each produces isolated findings with **no shared context**. Correlation is manual — after an incident. |
| **Skills & Staffing Shortage** — 74% of organizations lack qualified security professionals | Teams cannot hire specialists for each security domain. They need one system that connects the dots. |
| **Enterprise CNAPP Cost Barrier** — Wiz, Orca, Prisma Cloud cost $50k–$1M+/year | Small-to-midsize teams are priced out despite having the same cloud exposure profile as enterprises. |

### The Attack Chain Reality

A typical breach spans multiple tools but no single tool sees the full chain:

```
Attacker finds Git repo → discovers AWS keys (Gitleaks)
  → uses keys to assume over-privileged IAM role (AWS)
    → exploits container vulnerability for RCE (Trivy)
      → exfiltrates from public S3 bucket (Checkov)
```

**SentinelCNAPP connects these four dots in one queryable graph.**

---

## What SentinelCNAPP Does

### Core Innovation

Rather than building new scanners, SentinelCNAPP **integrates mature open-source engines** and adds the missing layer:

```
┌─────────────────────────────────────────────────────────┐
│                   Unified Dashboard                      │
│           One view · One query · One risk score          │
├─────────────────────────────────────────────────────────┤
│                     Correlation Graph                    │
│    Links findings → assets → identities → infrastructure │
├────────┬────────┬────────┬────────┬────────┬────────────┤
│ Trivy  │ Checkov│Gitleaks│ Falco  │  AWS   │   Custom   │
│(Cont.) │ (IaC)  │(Secret)│Runtime │ Asset  │ Extensions │
├────────┴────────┴────────┴────────┴────────┴────────────┤
│     AWS · Azure · GCP · Kubernetes · GitHub/GitLab       │
└──────────────────────────────────────────────────────────┘
```

### Key Capabilities

| Capability | Description |
|---|---|
| **Cloud Asset Inventory** | Automatically discovers all resources across AWS accounts (EC2, EKS, S3, RDS, IAM, Lambda, ECR) via cloud APIs. Tracks asset history and change detection. |
| **Unified Security Scanning** | Wraps Checkov (IaC), Trivy (containers/K8s), and Gitleaks (secrets) behind a normalized finding schema. Single feed for all security issues. |
| **Correlation Graph** | Every finding is linked to its asset, identity, and infrastructure context in Neo4j. Query attack paths across tool boundaries. |
| **Context-Aware Risk Scoring** | Computes risk from 6 weighted factors: internet exposure, data sensitivity, CVSS, severity, environment, and resource connectivity. |
| **Attack Path Analysis** | Automatically discovers exploitable chains from critical findings through identities to exposed resources. |
| **Runtime Protection** | Ingests Falco/eBPF events in real-time, normalizes to findings, and correlates with the existing graph. |
| **Auto Remediation** | Human-in-the-loop approval workflow. Executes cloud remediation actions (block public S3, revoke ingress, restrict IAM, enable ECR scanning, enforce K8s pod security). |
| **AI Security Assistant** | Natural language interface for security queries. 10 pre-built templates (critical findings, attack paths, risk distribution, etc.) with optional LLM integration for dynamic Cypher generation. |
| **Unified Dashboard** | Next.js 14 dashboard with interactive graph visualization, severity distributions, scanner status, and remediation management. |

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Frontend (Next.js 14)                         │
│    Dashboard · Assets · Findings · Graph · Attack Paths · AI Chat   │
└───────────────────────────┬─────────────────────────────────────────┘
                            │ REST API (Envoy Gateway + OIDC Auth)
┌───────────────────────────┼─────────────────────────────────────────┐
│                           ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    Correlation Graph API                     │   │
│  │              /api/v1/findings · /api/v1/dashboard            │   │
│  │              /api/v1/graph · /api/v1/attack-paths            │   │
│  └───────────────────────────┬─────────────────────────────────┘   │
│                              │                                     │
│  ┌──────────┐ ┌──────────┐ ┌▼──────────┐ ┌──────────┐ ┌────────┐ │
│  │  Asset   │ │   IaC    │ │ Container │ │ Secrets  │ │Runtime │ │
│  │Inventory │ │ Scanner  │ │  Scanner  │ │ Scanner  │ │(Falco) │ │
│  │ (Phase1) │ │(Phase 2) │ │ (Phase 2) │ │ (Phase2) │ │(Ph4)   │ │
│  └────┬─────┘ └────┬─────┘ └─────┬─────┘ └────┬─────┘ └───┬────┘ │
│       │            │             │            │           │       │
│       └────────────┴─────────────┼────────────┴───────────┘       │
│                                  │ NATS JetStream                  │
│                          ┌───────▼────────┐                        │
│                          │  Correlation   │──→ Neo4j Graph DB      │
│                          │  Service       │──→ Redis Cache         │
│                          │  (Phase 3)     │──→ OpenSearch          │
│                          └───────┬────────┘                        │
│                                  │                                 │
│              ┌───────────────────┼─────────────────────┐           │
│              │                   │                     │           │
│     ┌────────▼───────┐  ┌───────▼───────┐  ┌──────────▼──────┐   │
│     │ Risk Engine    │  │ Attack Path   │  │  Remediation    │   │
│     │ (Phase 4)      │  │ (Phase 4)     │  │  (Phase 5)      │   │
│     └────────────────┘  └───────────────┘  └─────────────────┘   │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │               AI Security Assistant (Phase 6)                │ │
│  │     Natural Language → Cypher → Graph Results → Chat UI     │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐ │
│  │  PostgreSQL  │  Neo4j 5.x  │  Redis 7  │  NATS JetStream    │ │
│  │  Assets,     │  Graph      │  Cache    │  Event Bus         │ │
│  │  Findings,   │  Correlation│           │  Scanner → Graph   │ │
│  │  Users       │             │           │                    │ │
│  └──────────────┴─────────────┴───────────┴────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

---

## Technology Stack

| Layer | Technology | Purpose |
|---|---|---|
| **Backend** | **Go 1.22+** | 10 microservices — single binary, excellent concurrency, cloud-native ecosystem |
| **Frontend** | **Next.js 14 + TypeScript** | Server-side rendering, React 18, App Router |
| **UI** | **TailwindCSS + shadcn/ui** | Dark-mode first, composable, accessible components |
| **Graph DB** | **Neo4j 5.x** | Correlation graph linking findings → assets → identities |
| **Relational** | **PostgreSQL 16** | Assets, raw findings, users, teams |
| **Cache** | **Redis 7** | Session state, risk score cache, graph query cache |
| **Search** | **OpenSearch 2.x** | Full-text search over findings and assets |
| **Event Bus** | **NATS JetStream** | Service-to-service async communication |
| **API Gateway** | **Envoy** | L7 routing, OIDC auth, rate limiting |
| **Auth** | **Dex (OIDC) + Casbin RBAC** | Authentication + fine-grained authorization |
| **Monitoring** | **Prometheus + Grafana + Loki** | Metrics, dashboards, structured logging |
| **CI/CD** | **GitHub Actions + ArgoCD** | Lint → test → build → security scan → GitOps deploy |
| **Container** | **Distroless images** | Minimal attack surface (no shell, no package manager) |
| **Orchestration** | **Kubernetes (kind/EKS)** | Helm charts + Tilt dev environment |

---

## Project Structure

```
sentinel-cnapp/
├── api/proto/                  # Protobuf schemas (Finding, Asset, Common)
├── pkg/                        # 8 shared Go libraries
│   ├── finding/                # Finding model, validation, normalization
│   ├── config/                 # Type-safe environment configuration
│   ├── logging/                # Structured JSON logger
│   ├── metrics/                # Prometheus metric registry
│   ├── queue/                  # NATS JetStream abstraction
│   ├── graph/                  # Neo4j client with Cypher helpers
│   ├── auth/                   # Casbin RBAC enforcer
│   └── scanner/                # Shared scanner toolkit
├── services/                   # 10 Go microservices
│   ├── asset-inventory/        # Phase 1 — Cloud asset discovery (AWS)
│   ├── scanner-iac/            # Phase 2 — Checkov IaC scanning
│   ├── scanner-container/      # Phase 2 — Trivy container scanning
│   ├── scanner-k8s/            # Phase 2 — K8s security scanning
│   ├── scanner-secrets/        # Phase 2 — Gitleaks secrets scanning
│   ├── correlation/            # Phase 3 — Neo4j correlation graph
│   ├── risk-engine/            # Phase 4 — Context-aware risk scoring
│   ├── attack-path/            # Phase 4 — Attack chain analysis
│   ├── runtime/                # Phase 4 — Falco runtime protection
│   ├── remediation/            # Phase 5 — Automated remediation
│   └── ai-assistant/           # Phase 6 — AI security assistant
├── frontend/                   # Next.js 14 dashboard
├── deploy/                     # Helm charts, Kustomize, Terraform
├── test/                       # E2E and integration tests
├── .github/workflows/          # CI/CD pipelines
├── docker-compose.yml          # Local development infrastructure
├── Tiltfile                    # Live-reload development environment
└── Makefile                    # Build automation
```

---

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 20+
- Docker
- kind (Kubernetes in Docker) + Tilt

### Development Environment

```bash
# Start the full development environment
make dev
```

This boots:
| Service | URL | Description |
|---|---|---|
| Frontend | http://localhost:3000 | Next.js dashboard |
| Asset Inventory | http://localhost:8080 | Asset discovery API |
| Correlation | http://localhost:8082 | Graph query API |
| Envoy Gateway | http://localhost:8081 | API gateway |
| Scanner IaC | http://localhost:8083 | Checkov scanner |
| Scanner Container | http://localhost:8084 | Trivy scanner |
| Scanner K8s | http://localhost:8085 | K8s security scanner |
| Scanner Secrets | http://localhost:8086 | Gitleaks scanner |
| Risk Engine | http://localhost:8087 | Risk scoring |
| Attack Path | http://localhost:8088 | Path analysis |
| Runtime | http://localhost:8089 | Falco events |
| Remediation | http://localhost:8090 | Auto-fix actions |
| AI Assistant | http://localhost:8091 | NL query interface |
| Dex (OIDC) | http://localhost:5556 | Identity provider |

### Makefile Commands

```bash
make lint            # Run all linters (Go, TypeScript, Protobuf)
make test            # Run all tests
make build           # Build all Go services
make proto-gen       # Generate protobuf code
make docker-build    # Build all Docker images
make clean           # Clean build artifacts
```

---

## Services API

### Asset Inventory
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/assets` | List assets (filtered + paginated) |
| `GET` | `/api/v1/assets/{id}` | Get single asset detail |

### Correlation Graph
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/findings` | List findings across all scanners |
| `GET` | `/api/v1/dashboard/stats` | Aggregate dashboard statistics |
| `GET` | `/api/v1/dashboard/severity-distribution` | Finding counts by severity |
| `GET` | `/api/v1/graph` | Graph data for visualization |
| `GET` | `/api/v1/attack-paths` | Attack path chains |

### Risk Engine
| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/risk/evaluate` | Evaluate single finding |
| `POST` | `/api/v1/risk/evaluate-all` | Evaluate all open findings |
| `GET` | `/api/v1/risk/{finding_id}` | Get risk for specific finding |

### Attack Path
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/attack-paths` | List all attack paths |
| `GET` | `/api/v1/attack-paths/summary` | Aggregate path statistics |
| `GET` | `/api/v1/attack-paths/{finding_id}` | Paths from a specific finding |

### Remediation
| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/remediation/suggest/{finding_id}` | Suggest remediation actions |
| `POST` | `/api/v1/remediation/approve/{id}` | Approve and execute |
| `GET` | `/api/v1/remediation/pending` | List pending approvals |
| `POST` | `/api/v1/remediation/auto-execute` | Execute auto-remediations |

### AI Assistant
| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/ai/query` | Natural language security query |
| `GET` | `/api/v1/ai/templates` | List available query templates |
| `GET` | `/api/v1/ai/history` | Query history |

### Runtime Protection
| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/runtime/event` | Ingest single Falco event |
| `POST` | `/api/v1/runtime/events` | Ingest batch Falco events |
| `POST` | `/api/v1/runtime/falco-webhook` | Falco webhook endpoint |

---

## Security Architecture

### Authentication & Authorization
- **OIDC via Dex** — Supports local accounts, GitHub OAuth, and any OIDC provider
- **Casbin RBAC** — Role-based access control with 3 roles: `admin`, `viewer`, `scanner`
- **JWT Token Validation** — Envoy gateway validates tokens before routing
- **API Key Authentication** — Service-to-service communication

### Container Security
- **Distroless Base Images** — All services use `gcr.io/distroless/static-debian12:nonroot` (no shell, no package manager)
- **Non-Root Users** — Containers run without root privileges
- **Read-Only Root Filesystem** — Filesystem is immutable at runtime
- **Regular Scanning** — All images scanned with Trivy in CI/CD (blocks CRITICAL/HIGH)

### Pipeline Security
- **SAST** — golangci-lint with 70+ linters, TypeScript strict mode
- **Secrets Detection** — Gitleaks runs on every commit (pre-commit hook + CI)
- **Vulnerability Scanning** — Trivy scans all dependencies in CI
- **SBOM Generation** — syft generates SPDX SBOM for every build
- **Dependency Management** — Renovate bot automates dependency updates

### Kubernetes Security
- **Pod Security Standards** — Restricted profile enforced
- **Network Policies** — Default deny-all with per-service ingress/egress
- **RBAC** — Least-privilege ClusterRole for scanner-k8s (get/list only)
- **Secrets Management** — External Secrets Operator with Vault backend

---

## Deployment

### Kubernetes (Production)

```bash
# Install Helm chart
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add neo4j https://helm.neo4j.com/neo4j
helm repo add nats https://nats-io.github.io/k8s/helm/charts/

# Deploy SentinelCNAPP
helm install sentinel-cnapp ./deploy/helm/sentinel-cnapp \
  --namespace sentinel --create-namespace \
  --set global.environment=production
```

### Configuration

All configuration is done via environment variables with sensible defaults:

| Variable | Default | Description |
|---|---|---|
| `SENTINEL_POSTGRES_URL` | `postgres://sentinel:changeme@localhost:5432/sentinel` | PostgreSQL connection string |
| `SENTINEL_NEO4J_URI` | `bolt://localhost:7687` | Neo4j connection URI |
| `SENTINEL_NATS_URL` | `nats://localhost:4222` | NATS server URL |
| `SENTINEL_REDIS_URL` | `localhost:6379` | Redis server address |
| `SENTINEL_AWS_REGIONS` | `us-east-1,us-west-2,eu-west-1` | AWS regions to scan |
| `SENTINEL_SYNC_INTERVAL` | `15m` | Asset discovery interval |
| `SENTINEL_LLM_API_KEY` | `` | OpenAI API key (optional) |

---

## Implementation Roadmap

| Phase | What | Services | Status |
|---|---|---|---|
| **Phase 0** | Foundation — monorepo, shared libs, CI/CD, protobuf, frontend shell | 8 packages | ✅ Complete |
| **Phase 1** | Asset Inventory — AWS discovery, PostgreSQL store, REST API, OIDC auth | 1 service | ✅ Complete |
| **Phase 2** | Scanner Integration — Checkov, Trivy, Gitleaks wrappers with NATS emit | 4 services | ✅ Complete |
| **Phase 3** | Correlation Graph — Neo4j schema, batch consumer, Redis cache, graph API | 1 service | ✅ Complete |
| **Phase 4** | Risk + Attack + Runtime — Context-aware scoring, path analysis, Falco | 3 services | ✅ Complete |
| **Phase 5** | Auto Remediation — Approval workflow, AWS + K8s remediation actions | 1 service | ✅ Complete |
| **Phase 6** | AI Assistant — NL→Cypher templates, optional LLM, chat interface | 1 service | ✅ Complete |

---

## License

Apache 2.0 — See [LICENSE](LICENSE) for details.

---

<div align="center">
  <strong>SentinelCNAPP</strong> — Correlating best-in-class open-source security tools under one graph, one risk score, and one dashboard.
</div>
