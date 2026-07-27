# SentinelCNAPP — Functional Outline

## What Problem Are We Solving?

### The Core Problem

Cloud security today has three interconnected gaps:

**1. Tool Fragmentation**
Security teams use 5–15 separate tools for cloud security (Trivy for containers, Checkov for IaC, Gitleaks for secrets, Falco for runtime, cloud provider consoles for posture). Each tool produces isolated findings with **no shared context**. Correlation is done manually — after an incident.

**2. Skills Gap**
74% of organizations lack qualified security professionals. Small-to-midsize teams cannot hire specialists for each domain (container security, IaC, identity, network, compliance). They need one system that connects the dots for them.

**3. Cost-of-Entry Gap**
Enterprise CNAPPs (Wiz, Orca, Prisma Cloud) solve correlation but cost $50k–$500k+/year. They are priced for enterprises. Open-source tools are free but fragmented. There is no open-source option that **unifies** them.

### The Attack Chain Reality

Most cloud breaches follow this pattern:
1. A **misconfiguration** (e.g., public S3 bucket) — detected by Checkov
2. An **over-privileged identity** that can access it — not detected by Checkov
3. A **vulnerable container** running with that identity — not linked to the above

Each finding exists in a separate tool. The **attack path** is invisible.

---

## What Are We Building?

**SentinelCNAPP** — an open-source, self-hostable Cloud-Native Application Protection Platform that:

### 1. Integrates Best-in-Class Open-Source Engines

We do **not** build new scanners. We wrap proven ones:

| Engine | Purpose | Our Integration |
|---|---|---|
| **Trivy** | Container vulnerability scanning | Scan images in registries, running pods |
| **Checkov** | Infrastructure-as-Code scanning | Scan Terraform, CloudFormation, K8s manifests |
| **Gitleaks** | Secrets detection | Scan Git repos for leaked credentials |
| **Falco** (Phase 4) | Runtime security | Monitor runtime behavior with eBPF |

### 2. Adds the Missing Layer: Correlation

Every finding from every engine is **normalized** into a common schema and written into a **Neo4j graph**:

```
(Finding {type:"vulnerability"})-[:FOUND_IN]->(Container)
      <-[:DEPLOYS]-(Deployment)
          <-[:MANAGES]-(K8sCluster)
              -[:HAS_POLICY]->(IAMRole)
                  -[:CAN_ACCESS]->(S3Bucket {public: true})
```

Now the attack path from a container vuln → privileged IAM role → exposed S3 bucket is **queryable in one query**.

### 3. Provides a Unified Dashboard

One interface instead of five. Users see:

- **Asset Inventory** — all cloud resources in one place
- **Unified Findings** — vulnerabilities, misconfigurations, secrets, runtime alerts in a single feed
- **Risk Score** — computed from the graph (Phase 4)
- **Attack Paths** — visual graph traversal showing exposure chains (Phase 4)
- **Compliance** — posture against CIS, NIST, SOC2 (Phase 3)

### 4. Delivers Enterprise-Grade Security

- Multi-tenant RBAC via OIDC
- Audit logging on every action
- Immutable asset history
- API-driven automation
- GitOps deployment on Kubernetes

---

## Core Functionality — MVP (Phase 1-3)

| # | Function | What It Does | User Benefit |
|---|---|---|---|
| F1 | **Cloud Asset Discovery** | Discovers AWS resources (EC2, EKS, S3, RDS, IAM, Lambda, ECR) via cloud APIs | See everything you own in one place |
| F2 | **IaC Scanning** | Runs Checkov on Terraform/CloudFormation repos; returns normalized misconfigurations | Catch infra mistakes before deploy |
| F3 | **Container Scanning** | Runs Trivy on container images; returns normalized vulnerabilities | Know what's vulnerable in your images |
| F4 | **K8s Scanning** | Runs Trivy + custom checks on Kubernetes clusters | Harden your cluster configuration |
| F5 | **Secrets Scanning** | Runs Gitleaks on Git repos; detects leaked credentials | Find secrets before attackers do |
| F6 | **Finding Normalization** | Converts every engine's output to one schema | Stop context-switching between tool UIs |
| F7 | **Correlation Graph** | Links findings to assets and to each other in Neo4j | See the attack chain, not just the symptoms |
| F8 | **Unified Dashboard** | Next.js dashboard: assets, findings, filters, search, visual graph | One pane of glass |
| F9 | **User & Team Management** | OIDC login, RBAC roles (admin/viewer/scanner), team scoping | Secure multi-tenant access |

## Future Functionality (Phase 4-6)

| # | Function | Phase | Description |
|---|---|---|---|
| F10 | **Runtime Protection** | 4 | Falco/eBPF integration for real-time threat detection |
| F11 | **Attack Path Engine** | 4 | Automated Neo4j traversal finding exposure chains |
| F12 | **Risk Scoring** | 4 | Single risk score per asset/service/env from graph analysis |
| F13 | **CSPM** | 2 | Cloud Security Posture Management (CIS benchmarks) |
| F14 | **CIEM** | 2 | Cloud Infrastructure Entitlement Management (identity security) |
| F15 | **Compliance Engine** | 3 | Compliance reporting against SOC2, PCI, HIPAA, CIS |
| F16 | **Vulnerability Management** | 3 | Aggregated vuln tracking, lifecycle management |
| F17 | **Auto Remediation** | 5 | Gated automated fixes (e.g., "auto-close public S3 bucket") |
| F18 | **AI Security Assistant** | 6 | LLM-powered natural language query over the graph |

---

## Architecture Principle

```
┌─────────────────────────────────────────────────────────┐
│                   Unified Dashboard                      │
│           One view. One query. One risk score.           │
├─────────────────────────────────────────────────────────┤
│                     Correlation Graph                    │
│        Links findings → assets → identities → code      │
├────────┬────────┬────────┬────────┬────────┬────────────┤
│ Trivy  │ Checkov│Gitleaks│ Falco  │Asset   │ Custom     │
│(Cont.) │ (IaC)  │(Secret)│Runtime │Discov. │ (CSPM/CIEM)│
├────────┴────────┴────────┴────────┴────────┴────────────┤
│              AWS · Azure · GCP · K8s · Git               │
└──────────────────────────────────────────────────────────┘
```

We integrate existing scanners. We own the graph and the dashboard. That is the entire product.
