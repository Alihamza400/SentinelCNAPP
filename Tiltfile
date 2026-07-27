# -*- mode: Python -*-
# SentinelCNAPP Development Environment

# ── Kubernetes Cluster ──────────────────────────
load('ext://kind', 'kind_create_cluster', 'kind_load_docker_image')
kind_create_cluster('sentinel-dev', kubeconfig='sentinel-dev')

# ── Dependencies (infrastructure) ──────────────
k8s_yaml('deploy/helm/sentinel-cnapp/templates/')

# PostgreSQL
postgres = docker_compose('docker-compose.yml', resource_name='postgres')

# Neo4j (for Phase 2-3 development)
neo4j = docker_compose('docker-compose.yml', resource_name='neo4j')

# NATS
nats = docker_compose('docker-compose.yml', resource_name='nats')

# Redis
redis = docker_compose('docker-compose.yml', resource_name='redis')

# OpenSearch
opensearch = docker_compose('docker-compose.yml', resource_name='opensearch')

# ── Asset Inventory Service ─────────────────────
docker_build('sentinel-cnapp/asset-inventory',
    '.',
    dockerfile='services/asset-inventory/Dockerfile',
    live_update=[
        sync('./services/asset-inventory/', '/src/services/asset-inventory/'),
        sync('./pkg/', '/src/pkg/'),
        run('go build -o /bin/asset-inventory /src/services/asset-inventory/'),
    ],
    target='builder',
)

k8s_yaml('deploy/helm/sentinel-cnapp/templates/asset-inventory.yaml')
k8s_resource('asset-inventory', port_forwards=['8080:8080'])

# ── Dex ─────────────────────────────────────────
k8s_yaml('deploy/helm/sentinel-cnapp/templates/dex.yaml')
k8s_resource('dex', port_forwards=['5556:5556'])

# ── Envoy ───────────────────────────────────────
k8s_yaml('deploy/helm/sentinel-cnapp/templates/envoy.yaml')
k8s_resource('envoy', port_forwards=['8081:8080'])

# ── Frontend ────────────────────────────────────
local_resource(
    'frontend',
    'npm install && npm run dev',
    workdir='./frontend',
    trigger_mode=TRIGGER_MODE_AUTO,
    serve_cmd='npm run dev',
    deps=['./frontend/'],
)

# ── Scanner: IaC (Checkov) ──────────────────────
docker_build('sentinel-cnapp/scanner-iac',
    '.',
    dockerfile='services/scanner-iac/Dockerfile',
    live_update=[
        sync('./services/scanner-iac/', '/src/services/scanner-iac/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/scanner-iac.yaml')
k8s_resource('scanner-iac', port_forwards=['8083:8080'])

# ── Scanner: Container (Trivy) ──────────────────
docker_build('sentinel-cnapp/scanner-container',
    '.',
    dockerfile='services/scanner-container/Dockerfile',
    live_update=[
        sync('./services/scanner-container/', '/src/services/scanner-container/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/scanner-container.yaml')
k8s_resource('scanner-container', port_forwards=['8084:8080'])

# ── Scanner: K8s ────────────────────────────────
docker_build('sentinel-cnapp/scanner-k8s',
    '.',
    dockerfile='services/scanner-k8s/Dockerfile',
    live_update=[
        sync('./services/scanner-k8s/', '/src/services/scanner-k8s/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/scanner-k8s.yaml')
k8s_resource('scanner-k8s', port_forwards=['8085:8080'])

# ── Correlation Service ─────────────────────────
docker_build('sentinel-cnapp/correlation',
    '.',
    dockerfile='services/correlation/Dockerfile',
    live_update=[
        sync('./services/correlation/', '/src/services/correlation/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/correlation.yaml')
k8s_resource('correlation', port_forwards=['8082:8080'])

# ── Risk Engine ─────────────────────────────────
docker_build('sentinel-cnapp/risk-engine',
    '.',
    dockerfile='services/risk-engine/Dockerfile',
    live_update=[
        sync('./services/risk-engine/', '/src/services/risk-engine/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/risk-engine.yaml')
k8s_resource('risk-engine', port_forwards=['8087:8080'])

# ── Attack Path Engine ──────────────────────────
docker_build('sentinel-cnapp/attack-path',
    '.',
    dockerfile='services/attack-path/Dockerfile',
    live_update=[
        sync('./services/attack-path/', '/src/services/attack-path/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/attack-path.yaml')
k8s_resource('attack-path', port_forwards=['8088:8080'])

# ── AI Assistant ────────────────────────────────
docker_build('sentinel-cnapp/ai-assistant',
    '.',
    dockerfile='services/ai-assistant/Dockerfile',
    live_update=[
        sync('./services/ai-assistant/', '/src/services/ai-assistant/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/ai-assistant.yaml')
k8s_resource('ai-assistant', port_forwards=['8091:8080'])

# ── Remediation Engine ──────────────────────────
docker_build('sentinel-cnapp/remediation',
    '.',
    dockerfile='services/remediation/Dockerfile',
    live_update=[
        sync('./services/remediation/', '/src/services/remediation/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/remediation.yaml')
k8s_resource('remediation', port_forwards=['8090:8080'])

# ── Runtime Protection ──────────────────────────
docker_build('sentinel-cnapp/runtime',
    '.',
    dockerfile='services/runtime/Dockerfile',
    live_update=[
        sync('./services/runtime/', '/src/services/runtime/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/runtime.yaml')
k8s_resource('runtime', port_forwards=['8089:8080'])

# ── Scanner: Secrets (Gitleaks) ─────────────────
docker_build('sentinel-cnapp/scanner-secrets',
    '.',
    dockerfile='services/scanner-secrets/Dockerfile',
    live_update=[
        sync('./services/scanner-secrets/', '/src/services/scanner-secrets/'),
        sync('./pkg/', '/src/pkg/'),
    ],
    target='builder',
)
k8s_yaml('deploy/helm/sentinel-cnapp/templates/scanner-secrets.yaml')
k8s_resource('scanner-secrets', port_forwards=['8086:8080'])

# ── Status ──────────────────────────────────────
print('═══════════════════════════════════════════════════════')
print(' SentinelCNAPP Development Environment')
print('═══════════════════════════════════════════════════════')
print(' Asset Inventory  : http://localhost:8080   (assets)')
print(' Correlation      : http://localhost:8082   (graph + findings API)')
print(' Envoy Gateway    : http://localhost:8081   (api gateway)')
print(' Dex (OIDC)       : http://localhost:5556   (auth)')
print(' Scanner IaC      : http://localhost:8083   (Checkov)')
print(' Scanner Container: http://localhost:8084   (Trivy)')
print(' Scanner K8s      : http://localhost:8085   (K8s checks)')
print(' Scanner Secrets  : http://localhost:8086   (Gitleaks)')
print(' Risk Engine      : http://localhost:8087   (risk scoring)')
print(' Attack Path      : http://localhost:8088   (path analysis)')
print(' Runtime          : http://localhost:8089   (Falco events)')
print(' Remediation      : http://localhost:8090   (auto-fix)')
print(' AI Assistant     : http://localhost:8091   (natural language queries)')
print(' Frontend         : http://localhost:3000   (dashboard)')
print(' PostgreSQL       : localhost:5432')
print(' Neo4j            : localhost:7687')
print(' NATS             : localhost:4222')
print(' Redis            : localhost:6379')
print('═══════════════════════════════════════════════════════')
