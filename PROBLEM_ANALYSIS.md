# SentinelCNAPP — Deep Problem Analysis

## The Crux: Three Layers of Fragmentation

The cloud security market has solved **individual detection problems** well. Trivy finds CVEs. Checkov finds misconfigurations. Gitleaks finds secrets. Falco detects runtime anomalies. Each is mature, free, and effective at its **single task**.

The problem is that **security incidents don't respect tool boundaries**. A real attack uses a chain of weaknesses across multiple domains. No single tool sees the full chain.

This document dissects the three layers of fragmentation that SentinelCNAPP exists to solve.

---

## Layer 1: Data Fragmentation — Findings Live in Silos

### The Problem

When you run five security tools, you get **five sets of findings** stored in **five different formats** in **five different places**. No tool knows what the others found.

### Concrete Example

Consider a typical microservice deployment:

```
┌─────────────────────────────────────────────┐
│                  Checkout API                │
│          (Running on EKS, Node.js)           │
├─────────────────────────────────────────────┤
│  Docker image: org/checkout:v2.3            │
│  IAM role: checkout-api-prod-role           │
│  IaC: terraform/environments/prod/          │
│  Git repo: github.com/org/checkout-api      │
└─────────────────────────────────────────────┘
```

Here's what each scanner sees **in isolation**:

| Tool | Scan Target | Finding | Severity |
|---|---|---|---|
| **Trivy** | `org/checkout:v2.3` | CVE-2026-1234 in `libssl 1.1.1` | High |
| **Checkov** | `terraform/env/prod/` | S3 bucket `checkout-logs` is public | Critical |
| **Gitleaks** | `github.com/org/checkout-api` | AWS secret key in `config.js` (committed 3 months ago) | Critical |
| **AWS Console** | checkout-api-prod-role | Role has `s3:*` on all buckets | Warning |

### Why This Is Dangerous

Each finding alone is concerning. But **together** they describe a complete attack path that no single tool detects:

1. The **public S3 bucket** (Checkov) contains customer transaction logs
2. The **over-privileged IAM role** (AWS Console) can write to that bucket
3. The **vulnerable libssl** (Trivy) can be exploited for RCE
4. The **leaked secret** (Gitleaks) gives an attacker credentials to assume that role

**Step-by-step exploitation of the blind spot:**
```
Attacker finds Git repo → finds AWS keys (Gitleaks would flag)
  → uses keys to assume checkout-api-role (AWS would log)
    → exploits libssl vulnerability for RCE on pod (Trivy found CVE)
      → uses pod's IAM role to exfiltrate from public S3 bucket (Checkov flagged)
```

**No single tool connects these four dots.** A security engineer would need to manually correlate across four tool UIs — if they even know to look.

### The Root Cause

Each tool emits findings with **no shared identifier scheme**. Trivy knows the image digest. Checkov knows the Terraform resource address. Gitleaks knows the Git commit SHA. AWS knows the IAM ARN. There is no common key to join them.

**The fix is not a better scanner. The fix is a correlation layer.**

---

## Layer 2: Context Fragmentation — Findings Without "So What?"

### The Problem

Security tools report **what** is wrong but not **why it matters** in your specific environment. A CVE in a library used by `checkout-api` that is internet-facing is treated the same as a CVE in a library used by an internal batch job that runs once a month. Severity is computed from CVSS (a generic formula), not from your topology.

### Concrete Example

```
Finding A: CVE-2026-5678 in log4j 2.14.1  (CVSS: 10.0, Critical)
Finding B: S3 bucket "backups" is public (Checkov, Medium)
```

**Tool-based severity:**
- Finding A is Critical → gets immediate attention
- Finding B is Medium → gets deferred

**Actual risk in your environment:**
- Finding A: `log4j` is used by `internal-reporting-job` (runs once a quarter, not network-accessible, no customer data) → **Actual risk: Low**
- Finding B: S3 bucket `backups` contains unencrypted PII, is internet-facing, and is linked to `checkout-api` via IAM → **Actual risk: Critical**

### Why This Happens

Scoring engines (CVSS, vendor severity) are **context-blind**. They don't know:
- Is this asset internet-facing or internal?
- Does the asset handle PII/PCI/PHI?
- What other assets can this one reach?
- Is there a compensating control (WAF, network policy, encryption)?
- Who owns this and are they on-call?
- Is this asset in production or staging?

### The Consequence

Teams spend time on **loud but low-impact findings** while **quiet but critical attack paths** go unaddressed. Alert fatigue sets in. Real risks are missed.

### The Root Cause

Risk is a **graph property**, not a finding property. The risk of a finding depends on:
- **Reachability**: can an attacker reach this asset?
- **Privilege**: what can this asset access?
- **Data sensitivity**: what data is adjacent?
- **Criticality**: is this asset in the request path for a business function?

These are properties of the **graph connecting findings to assets to topology** — not of the finding itself.

**The fix is not better severity scoring. The fix is topology-aware risk computation.**

---

## Layer 3: Workflow Fragmentation — Remediation Without Ownership

### The Problem

Even when a finding is identified and understood, the workflow to fix it is fragmented across tools and teams.

### The Concrete Friction

```
1. Trivy alerts: CVE in checkout-api image
2. Engineer logs into Trivy UI → sees CVE
3. But: who owns checkout-api? Tracks are in PagerDuty, not Trivy
4. Engineer checks PagerDuty → finds team "Checkout" owns it
5. But: what's the deploy process? Not in PagerDuty or Trivy
6. Engineer checks GitHub → finds deploy is via ArgoCD
7. But: PR to update base image? That's a different team (Platform)
8. Engineer files a Jira ticket → assigned to Platform team
9. Platform team doesn't see the risk context → deprioritizes it
10. Finding stays open for 6 months
```

**Each step is a context switch. Each switch is a delay. Each delay is an exposure window.**

### The Systemic Issues

| Issue | Description |
|---|---|
| **No unified ownership** | Finding says what, but not who owns it. Owner is inferred from asset, but the asset-to-team mapping lives in a different system |
| **No unified action** | Remediation requires knowledge of deploy process, CI pipeline, code ownership — none of which the security tool has |
| **No unified prioritization** | The CVE in checkout-api competes for attention with every other finding, with no shared priority signal |
| **No unified tracking** | Once the finding moves to Jira, the security tool loses visibility. Status drift is inevitable |
| **No feedback loop** | When the image is rebuilt, does the old finding auto-resolve? Only if someone remembers to re-scan and check |

### The Root Cause

Security tools are designed for **detection**, not for **remediation workflows**. They lack:
- Integration with ownership systems (PagerDuty, Slack, GitHub teams)
- Integration with deploy systems (ArgoCD, CI pipelines)
- Integration with tracking systems (Jira, Linear)
- Status lifecycle management (auto-resolve on re-scan)

**The fix is not a better detection engine. The fix is a workflow layer natively connected to the detection graph.**

---

## The Meta-Problem: The Enterprise CNAPP Gap

### Why Not Just Buy Wiz?

Enterprise CNAPPs (Wiz, Orca, Prisma Cloud) **do** solve these three layers. They correlate findings, compute context-aware risk, and provide workflow integration.

**The problem is access:**

| Factor | Enterprise CNAPP | SentinelCNAPP |
|---|---|---|
| **Pricing** | $50k–$1M+/year | Free (open-source) |
| **Deployment** | SaaS-only, data leaves your network | Self-hosted on your K8s |
| **Customization** | Vendor-controlled roadmap | You own the code |
| **Integration** | Vendor-controlled integrations | Add any engine via adapter |
| **Data residency** | Vendor's cloud | Your infrastructure |
| **Vendor lock-in** | Migration cost is massive | Zero lock-in |

### The Numbers That Matter

- **69%** of security pros cite **tool fragmentation** as #1 obstacle
- **74%** of orgs have **security staffing shortage**
- **~1 in 3** SMBs had a breach **despite owning security tools**
- **77%** cite **identity and access** as top concern
- **70%** cite **misconfigured cloud services**
- **66%** cite **data exposure**

These numbers describe a market that:
1. Has detection tools (fragmented)
2. Lacks staff to manually correlate them
3. Is experiencing breaches anyway
4. Cannot afford enterprise CNAPPs

**That is the gap SentinelCNAPP fills.**

---

## Summary: The Problems We Solve

| # | Problem | What It Costs | Our Solution |
|---|---|---|---|
| P1 | **Findings live in silos** across 5+ tools with no shared context | Attack paths invisible; manual correlation takes hours; incidents missed | **Correlation graph (Neo4j)** linking every finding to every connected asset |
| P2 | **Severity is context-blind** — CVSS ignores topology, data sensitivity, reachability | Teams chase loud false positives while real risks age; alert fatigue | **Graph-based risk scoring** that accounts for asset context, reachability, data classification |
| P3 | **Remediation workflow is fragmented** — ownership, deploy process, tracking are in separate systems | Mean-time-to-remediate measured in months; findings age out without action | **Unified dashboard** showing ownership, deploy context, and lifecycle status alongside findings |
| P4 | **Enterprise CNAPPs are priced out of reach** for small-to-midsize teams | Entire market segment is under-protected despite having the same exposure profile | **Open-source, self-hostable** platform with no per-asset or per-user licensing |
| P5 | **No single graph model** connects cloud assets, identities, code, and runtime | Correlation is ad-hoc or non-existent; incident response is reactive | **Unified data model** (Asset → Identity → Code → Runtime → Finding) |

---

## The Core Insight

> Every finding is a node in a graph. The attack is an edge traversal. The defense is graph comprehension.

SentinelCNAPP does not build better scanners. It builds the **graph** and the **dashboard** that scanners cannot provide on their own.
