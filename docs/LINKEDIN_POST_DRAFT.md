# SentinelCNAPP — LinkedIn Post Draft

---

## Post 1: The Big Reveal (Product Launch)

**Headline:** We built an open-source CNAPP. Here's why.

**Body:**

After months of engineering, I'm excited to open-source SentinelCNAPP — a unified Cloud-Native Application Protection Platform that correlates Trivy, Checkov, Gitleaks, and Falco under one Neo4j graph, one risk score, and one dashboard.

**The problem we solved:**

69% of security professionals cite tool fragmentation as their #1 obstacle. 74% of organizations lack qualified security staff. Enterprise CNAPPs (Wiz, Orca, Prisma Cloud) solve this but cost $50k–$1M+/year — out of reach for most teams.

Open-source scanners are free but fragmented. A typical attack chain spans:
- A misconfiguration (Checkov)
- An over-privileged identity (AWS IAM)
- A vulnerable container (Trivy)
- A leaked secret (Gitleaks)

No single tool connects these dots. Until now.

**What we built:**

10 Go microservices, 135 files, all under Apache 2.0:

• Asset Inventory — AWS discovery across 7 services (EC2, EKS, S3, IAM, Lambda, ECR, RDS)
• 4 Scanner Integrations — Checkov, Trivy, Gitleaks, Falco with normalized findings
• Neo4j Correlation Graph — Finding → Asset → Identity → Resource with 12+ Cypher queries
• Context-Aware Risk Engine — 6 weighted factors (internet exposure, data sensitivity, CVSS, etc.)
• Attack Path Analysis — Automated chain discovery from critical findings to exposed resources
• Auto Remediation — Human-in-the-loop approval with 5 AWS actions + K8s actions
• AI Security Assistant — Natural language queries over the graph (template-based + optional LLM)

**Architecture:** Go backend → NATS event bus → Neo4j correlation graph → Next.js dashboard → Kubernetes-native deployment.

**Why this matters for the industry:**
- Open-source alternative to $1M enterprise CNAPPs
- No per-asset, per-user, or per-scan pricing
- Self-hosted on your K8s — data never leaves your network
- Full control over your integrations and roadmap

Stack: Go 1.22, Next.js 14, Neo4j 5.x, PostgreSQL 16, NATS JetStream, Redis, Envoy, Helm, distroless containers.

**GitHub:** https://github.com/Alihamza400/SentinelCNAPP

Would love feedback, contributions, and ideas from the cloud security community. What integrations would you want next? Azure? GCP? Compliance frameworks?

#CloudSecurity #CNAPP #DevSecOps #OpenSource #Kubernetes #Neo4j #GoLang #Cybersecurity #CloudNative

---

## Post 2: Technical Deep Dive (For Engineers)

**Headline:** How we built a correlation graph that connects the dots between 4 security scanners.

**Body:**

Every cloud security tool produces findings. None of them talk to each other.

At SentinelCNAPP, we faced this problem head-on. Our approach:

**1. Normalize everything.**
Every scanner (Trivy, Checkov, Gitleaks, Falco) emits findings in different formats. We wrap each behind a common protobuf schema and publish to NATS JetStream.

**2. Graph the connections.**
Our correlation service consumes findings and writes them to Neo4j:

(:Finding {severity:"critical"})-[:FOUND_IN]->(:Asset)
  -[:HAS_IDENTITY]->(:Identity)-[:CAN_ACCESS]->(:Resource {public:true})

**3. Compute context-aware risk.**
We don't trust CVSS alone. Our risk engine walks the graph weighting 6 factors:
• Internet-facing asset → +25%
• Sensitive data classification → +20%
• CVSS score → +20%
• Finding severity → +15%
• Production environment → +10%
• Connected exposed resources → +10%

**4. Find attack paths automatically.**
MATCH path = (f:Finding)-[:FOUND_IN]->(a:Asset)-[:HAS_IDENTITY]->(i:Identity)-[:CAN_ACCESS]->(r:Asset {internet_facing:true})
RETURN path

One query. The entire attack chain. From any scanner, across any tool boundary.

**5. Remediate with human approval.**
Low-severity findings auto-remediate. Critical findings wait for human approval. Remediation actions execute via AWS SDK (block public S3, revoke ingress, restrict IAM, enable scanning).

**6. Ask questions in plain English.**
10 pre-built query templates + optional LLM integration. "Show me all critical findings on internet-facing assets handling PII" → Cypher → results.

The entire platform is open-source, self-hostable, and runs on any K8s cluster.

**Stack decisions:**
• Go — because the entire cloud-native ecosystem (Docker, K8s, Trivy, Prometheus) is Go
• Neo4j — because risk is a graph property, not a finding property
• NATS — because it's 10x simpler than Kafka and written in Go
• Distroless containers — because security tools should be secure themselves

135 files, 10 microservices, 0 vendor lock-in.

**GitHub:** https://github.com/Alihamza400/SentinelCNAPP

PRs welcome. What would you build next?

#CloudSecurity #CNAPP #GoLang #Neo4j #Kubernetes #DevSecOps #OpenSource #SoftwareArchitecture

---

## Post 3: The Business Case (For Decision Makers)

**Headline:** Your cloud security tools don't talk to each other. Here's what that costs you.

**Body:**

You're running Trivy for containers. Checkov for IaC. Gitleaks for secrets. Maybe Falco for runtime.

Each tool finds real issues. But none of them connect the dots.

The result:
• Teams spend hours manually correlating findings across 4+ tool UIs
• Attack paths spanning multiple tools go undetected (until they're exploited)
• Critical fixes get deprioritized because context is missing
• Mean-time-to-remediate stretches to months

**The enterprise CNAPP solution exists — but at $50k–$1M+/year.**

SentinelCNAPP is an open-source alternative. Here's what it does differently:

**1. It connects everything.**
A Neo4j correlation graph links every finding to its asset, identity, and infrastructure context. One query shows the full attack chain from a container vulnerability through an over-privileged IAM role to an exposed S3 bucket.

**2. It computes real risk.**
CVSS scores are generic. Our risk engine walks the graph and factors in your specific topology: internet exposure, data sensitivity, exploit paths, environment criticality.

**3. It automates remediation.**
Human-in-the-loop approval for critical findings. Auto-remediation for low-severity. Executes via cloud provider APIs. No tickets, no context switches, no delays.

**4. It speaks your language.**
Ask questions in plain English: "Show me all high-risk findings on internet-facing assets." The AI assistant translates to graph queries and returns results instantly.

**5. It runs on your infrastructure.**
Self-hosted on Kubernetes. Data never leaves your network. No per-asset licensing. No vendor lock-in.

**The tech:**
10 Go microservices, Neo4j graph database, PostgreSQL, NATS JetStream event bus, Next.js dashboard, Helm-deployed on any K8s cluster.

Open-source. Apache 2.0. Built for teams who need enterprise-grade cloud security without enterprise pricing.

**GitHub:** https://github.com/Alihamza400/SentinelCNAPP

If you're evaluating CNAPP solutions or frustrated with tool fragmentation, I'd love to connect.

#CloudSecurity #Cybersecurity #CNAPP #DevSecOps #EnterpriseArchitecture #OpenSource #CloudComputing

---

## Post 4: The Architecture Post (For Architects)

**Headline:** Architecture of an open-source CNAPP: 10 microservices, 3 databases, 1 graph.

**Body:**

We built SentinelCNAPP to solve one specific problem: security tools produce isolated findings with no shared context.

Here's the architecture that connects them:

**Data Flow:**

Scanners (Trivy, Checkov, Gitleaks, Falco) →
  Normalized Finding (protobuf schema) →
    NATS JetStream (event bus) →
      Correlation Service (batch consumer) →
        Neo4j Graph (batch writes every 5s / 50 items)

**The Graph Schema:**
6 node types: Asset, Finding, Identity, Service, Dependency, Risk
8 relationship types: FOUND_IN, HAS_IDENTITY, CAN_ACCESS, DEPENDS_ON, CONNECTS_TO, HAS_POLICY, HAS_RISK, HOSTS

**The 10 Services:**

| Service | Role | Data Store |
|---|---|---|
| asset-inventory | AWS resource discovery | PostgreSQL |
| scanner-iac | Checkov wrapper | NATS → Neo4j |
| scanner-container | Trivy wrapper | NATS → Neo4j |
| scanner-k8s | K8s security checks | NATS → Neo4j |
| scanner-secrets | Gitleaks wrapper | NATS → Neo4j |
| correlation | Graph writer + query API | Neo4j + Redis |
| risk-engine | Graph-based risk scoring | Neo4j |
| attack-path | Attack chain discovery | Neo4j |
| runtime | Falco event ingestion | NATS → Neo4j |
| remediation | Auto-fix actions | AWS SDK + K8s API |
| ai-assistant | NL → Cypher queries | Neo4j + LLM |

**Key Architectural Decisions:**
- Go for all services (cloud-native ecosystem alignment)
- NATS over Kafka (10µs latency, 10x simpler, Go-native)
- Neo4j over relational (graph traversal for attack paths)
- PostgreSQL for structured data (assets, users, raw findings)
- Distroless containers (no shell = minimal attack surface)
- Tilt + kind for dev (live-reload, one command bootstrap)

**Security by design:**
- OIDC auth with Dex
- Casbin RBAC (admin/viewer/scanner)
- Envoy gateway with rate limiting
- mTLS between services
- All containers non-root, read-only filesystem

**Deployment:** Helm chart → any K8s cluster (kind dev → EKS prod)

The entire platform is open-source. 135 files. Apache 2.0.

**GitHub:** https://github.com/Alihamza400/SentinelCNAPP

What architecture questions do you have?

#SoftwareArchitecture #CloudSecurity #CNAPP #GoLang #Neo4j #Kubernetes #SystemDesign #DevSecOps
