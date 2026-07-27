# SentinelCNAPP

An open-source, unified Cloud-Native Application Protection Platform.

## Architecture

```
sentinel-cnapp/
├── api/proto/          # Protobuf definitions
├── services/           # Go microservices
├── pkg/                # Shared Go libraries
├── frontend/          # Next.js dashboard
├── deploy/            # Helm, Kustomize, Terraform
└── test/              # E2E and integration tests
```

## Quick Start

```bash
# Prerequisites: Go 1.22+, Node 20+, Docker, kind, tilt
make dev
```

## Development

```bash
make lint       # Run all linters
make test       # Run all tests
make build      # Build all services
make dev        # Start dev environment
```

## Services

| Service | Description | Status |
|---|---|---|
| asset-inventory | Cloud asset discovery (AWS) | Phase 1 |
| scanner-iac | IaC scanning (Checkov) | Phase 2 |
| scanner-container | Container scanning (Trivy) | Phase 2 |
| scanner-k8s | Kubernetes scanning (Trivy+) | Phase 2 |
| scanner-secrets | Secrets scanning (Gitleaks) | Phase 2 |
| correlation | Neo4j graph correlation | Phase 3 |
| risk-engine | Graph-based risk scoring | Phase 4 |
| attack-path | Attack path analysis | Phase 4 |

## License

Apache 2.0
