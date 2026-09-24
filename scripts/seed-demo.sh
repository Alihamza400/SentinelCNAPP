#!/usr/bin/env bash
# seed-demo.sh — Populate Neo4j + PostgreSQL with a realistic demo attack chain.
#
# Story: A public S3 bucket (checkout-logs) is reachable by an over-privileged
# IAM role that is attached to an internet-facing EC2 instance which has a
# critical container vulnerability. Secrets and misconfig findings round it out
# so every dashboard page has data.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NEO4J_CONTAINER="${NEO4J_CONTAINER:-sentinel-neo4j-1}"
PG_CONTAINER="${PG_CONTAINER:-sentinel-postgres-1}"
NEO4J_USER="${NEO4J_USER:-neo4j}"
NEO4J_PASS="${NEO4J_PASS:-changeme}"
PG_USER="${PG_USER:-sentinel}"
PG_DB="${PG_DB:-sentinel}"

log() { printf '\033[36m[seed]\033[0m %s\n' "$*"; }

cypher() {
  # cypher-shell exits non-zero on error; keep pipefail happy with || true on empty
  docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain "$1"
}

psql_exec() {
  docker exec -i "$PG_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 -c "$1"
}

log "waiting for Neo4j..."
for i in $(seq 1 60); do
  if docker exec "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" "RETURN 1;" >/dev/null 2>&1; then
    break
  fi
  [ "$i" = 60 ] && { echo "Neo4j not ready"; exit 1; }
  sleep 2
done

log "waiting for PostgreSQL..."
for i in $(seq 1 60); do
  if docker exec "$PG_CONTAINER" pg_isready -U "$PG_USER" >/dev/null 2>&1; then
    break
  fi
  [ "$i" = 60 ] && { echo "Postgres not ready"; exit 1; }
  sleep 2
done

# ── Ensure pg_trgm (needed by asset-inventory migration) ────────────────────
log "ensuring pg_trgm extension..."
psql_exec "CREATE EXTENSION IF NOT EXISTS pgcrypto; CREATE EXTENSION IF NOT EXISTS pg_trgm;" || true

# ── Neo4j: Assets ───────────────────────────────────────────────────────────
log "seeding Neo4j assets..."
cypher "
MERGE (s3:Asset {id: 'arn:aws:s3:::checkout-logs'})
SET s3.provider='aws', s3.type='s3_bucket', s3.name='checkout-logs',
    s3.region='us-east-1', s3.environment='production', s3.internet_facing=true,
    s3.data_classification='restricted', s3.tags='{\"env\":\"prod\",\"team\":\"checkout\"}',
    s3.active=true, s3.first_seen=timestamp(), s3.last_seen=timestamp();

MERGE (ec2:Asset {id: 'arn:aws:ec2:us-east-1:i-0abc123def456'})
SET ec2.provider='aws', ec2.type='ec2_instance', ec2.name='checkout-api-server',
    ec2.region='us-east-1', ec2.environment='production', ec2.internet_facing=true,
    ec2.data_classification='confidential', ec2.tags='{\"env\":\"prod\",\"role\":\"api\"}',
    ec2.active=true, ec2.first_seen=timestamp(), ec2.last_seen=timestamp();

MERGE (eks:Asset {id: 'arn:aws:eks:us-east-1:123456789012:cluster/prod-eks'})
SET eks.provider='aws', eks.type='eks_cluster', eks.name='prod-eks',
    eks.region='us-east-1', eks.environment='production', eks.internet_facing=false,
    eks.data_classification='confidential', eks.tags='{\"env\":\"prod\"}',
    eks.active=true, eks.first_seen=timestamp(), eks.last_seen=timestamp();

MERGE (img:Asset {id: 'image:org/checkout:v2.3'})
SET img.provider='docker', img.type='container_image', img.name='org/checkout:v2.3',
    img.region='global', img.environment='production', img.internet_facing=false,
    img.data_classification='confidential', img.tags='{\"registry\":\"ecr\"}',
    img.active=true, img.first_seen=timestamp(), img.last_seen=timestamp();

MERGE (repo:Asset {id: 'git:github.com/acme/checkout-api'})
SET repo.provider='github', repo.type='git_repository', repo.name='acme/checkout-api',
    repo.region='global', repo.environment='production', repo.internet_facing=false,
    repo.data_classification='confidential', repo.tags='{\"visibility\":\"private\"}',
    repo.active=true, repo.first_seen=timestamp(), repo.last_seen=timestamp();

MERGE (fn:Asset {id: 'arn:aws:lambda:us-east-1:123456789012:function:payments'})
SET fn.provider='aws', fn.type='lambda_function', fn.name='payments',
    fn.region='us-east-1', fn.environment='production', fn.internet_facing=false,
    fn.data_classification='restricted', fn.tags='{\"env\":\"prod\"}',
    fn.active=true, fn.first_seen=timestamp(), fn.last_seen=timestamp();

MERGE (sg:Asset {id: 'arn:aws:ec2:us-east-1:123456789012:security-group/sg-public'})
SET sg.provider='aws', sg.type='security_group', sg.name='sg-public',
    sg.region='us-east-1', sg.environment='production', sg.internet_facing=true,
    sg.data_classification='public', sg.tags='{\"env\":\"prod\"}',
    sg.active=true, sg.first_seen=timestamp(), sg.last_seen=timestamp();
" 2>&1 | grep -v "^$" || true

# ── Neo4j: Identity ─────────────────────────────────────────────────────────
log "seeding Neo4j identity..."
cypher "
MERGE (role:Identity {arn: 'arn:aws:iam::123456789012:role/checkout-role'})
SET role.name='checkout-role', role.type='iam_role', role.provider='aws',
    role.permissions='s3:*,ec2:*,lambda:*', role.active=true,
    role.first_seen=timestamp(), role.last_seen=timestamp();

MATCH (ec2:Asset {id: 'arn:aws:ec2:us-east-1:i-0abc123def456'})
MATCH (role:Identity {arn: 'arn:aws:iam::123456789012:role/checkout-role'})
MERGE (ec2)-[:HAS_IDENTITY]->(role);

MATCH (eks:Asset {id: 'arn:aws:eks:us-east-1:123456789012:cluster/prod-eks'})
MATCH (role:Identity {arn: 'arn:aws:iam::123456789012:role/checkout-role'})
MERGE (eks)-[:HAS_IDENTITY]->(role);

MATCH (s3:Asset {id: 'arn:aws:s3:::checkout-logs'})
MATCH (role:Identity {arn: 'arn:aws:iam::123456789012:role/checkout-role'})
MERGE (s3)-[:HAS_IDENTITY]->(role);

MATCH (role:Identity {arn: 'arn:aws:iam::123456789012:role/checkout-role'})
MATCH (s3:Asset {id: 'arn:aws:s3:::checkout-logs'})
MERGE (role)-[:CAN_ACCESS {permission: 's3:GetObject'}]->(s3);

MATCH (role:Identity {arn: 'arn:aws:iam::123456789012:role/checkout-role'})
MATCH (ec2:Asset {id: 'arn:aws:ec2:us-east-1:i-0abc123def456'})
MERGE (role)-[:CAN_ACCESS {permission: 'ec2:DescribeInstances'}]->(ec2);
" 2>&1 | grep -v "^$" || true

# ── Neo4j: Findings ─────────────────────────────────────────────────────────
log "seeding Neo4j findings..."
NOW_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
WEEK_AGO_TS="$(date -u -d '7 days ago' +%Y-%m-%dT%H:%M:%SZ)"

seed_finding() {
  # $1 = finding id, $2 = asset id, rest unused — body built inline by caller
  local fid="$1" aid="$2" body="$3"
  docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain \
"MERGE (f:Finding {id: '$fid'})
$f body
WITH f
MATCH (a:Asset {id: '$aid'})
MERGE (f)-[:FOUND_IN]->(a)
RETURN f.id AS id;" 2>&1 | grep -v "^$" || true
}

# cypher-shell treats ';' as statement terminator, so each finding must be
# ONE statement (MERGE ... SET ... WITH ... MATCH ... MERGE ...).

log "  finding 1/8: public S3 (checkov, critical)"
docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain "
MERGE (f:Finding {id: 'checkov:arn:aws:s3:::checkout-logs:CKV_AWS_20_S3_public_access'})
SET f.source='checkov', f.type='misconfiguration', f.severity='critical',
    f.cvss_score=0.0, f.title='CKV_AWS_20: S3 bucket is publicly accessible',
    f.description='Bucket checkout-logs allows public read via bucket policy',
    f.remediation='Enable Block Public Access and remove public bucket ACL/policy',
    f.status='open', f.detected_at='${NOW_TS}', f.metadata='{}',
    f.tags='[\"iac\",\"s3\",\"public\"]', f.first_seen=timestamp(), f.risk_score=9.2
WITH f MATCH (a:Asset {id: 'arn:aws:s3:::checkout-logs'})
MERGE (f)-[:FOUND_IN]->(a)
RETURN f.id;" 2>&1 | grep -v "^$" || true

log "  finding 2/8: open security group (checkov, critical)"
docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain "
MERGE (f:Finding {id: 'checkov:arn:aws:ec2:us-east-1:123456789012:security-group/sg-public:CKV_AWS_2_SG_open'})
SET f.source='checkov', f.type='misconfiguration', f.severity='critical',
    f.cvss_score=0.0, f.title='CKV_AWS_2: Security group allows 0.0.0.0/0 ingress',
    f.description='sg-public allows unrestricted inbound access on port 22/3389',
    f.remediation='Restrict ingress rules to known CIDR ranges',
    f.status='open', f.detected_at='${NOW_TS}', f.metadata='{}',
    f.tags='[\"iac\",\"network\",\"exposure\"]', f.first_seen=timestamp(), f.risk_score=8.7
WITH f MATCH (a:Asset {id: 'arn:aws:ec2:us-east-1:123456789012:security-group/sg-public'})
MERGE (f)-[:FOUND_IN]->(a)
RETURN f.id;" 2>&1 | grep -v "^$" || true

log "  finding 3/8: leaked AWS key (gitleaks, critical)"
docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain "
MERGE (f:Finding {id: 'gitleaks:git:github.com/acme/checkout-api:AKIAIOSFODNN7EXAMPLE'})
SET f.source='gitleaks', f.type='secret', f.severity='critical',
    f.cvss_score=0.0, f.title='Secret found: AWS Access Key ID',
    f.description='AWS Access Key ID detected in config/settings.js',
    f.remediation='Rotate the exposed credential and remove from codebase',
    f.status='open', f.detected_at='${NOW_TS}', f.metadata='{}',
    f.tags='[\"secret\",\"aws\",\"credential\"]', f.first_seen=timestamp(), f.risk_score=9.5
WITH f MATCH (a:Asset {id: 'git:github.com/acme/checkout-api'})
MERGE (f)-[:FOUND_IN]->(a)
RETURN f.id;" 2>&1 | grep -v "^$" || true

log "  finding 4/8: image CVE (trivy, high)"
docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain "
MERGE (f:Finding {id: 'trivy:image:org/checkout:v2.3:CVE-2026-1234'})
SET f.source='trivy', f.type='vulnerability', f.severity='high',
    f.cvss_score=8.1, f.title='CVE-2026-1234: libssl buffer overflow',
    f.description='Buffer overflow in libssl allows remote code execution',
    f.remediation='Upgrade base image to org/checkout:v2.4 (libssl 3.0.15)',
    f.status='open', f.detected_at='${NOW_TS}', f.metadata='{}',
    f.tags='[\"container\",\"vulnerability\",\"cve\"]', f.first_seen=timestamp(), f.risk_score=7.8
WITH f MATCH (a:Asset {id: 'image:org/checkout:v2.3'})
MERGE (f)-[:FOUND_IN]->(a)
RETURN f.id;" 2>&1 | grep -v "^$" || true

log "  finding 5/8: host CVE (trivy, high)"
docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain "
MERGE (f:Finding {id: 'trivy:arn:aws:ec2:us-east-1:i-0abc123def456:CVE-2026-1234'})
SET f.source='trivy', f.type='vulnerability', f.severity='high',
    f.cvss_score=8.1, f.title='CVE-2026-1234 on checkout-api-server',
    f.description='Host is running an image with a critical libssl vulnerability',
    f.remediation='Patch the instance and redeploy with checkout:v2.4',
    f.status='open', f.detected_at='${NOW_TS}', f.metadata='{}',
    f.tags='[\"container\",\"vulnerability\"]', f.first_seen=timestamp(), f.risk_score=8.4
WITH f MATCH (a:Asset {id: 'arn:aws:ec2:us-east-1:i-0abc123def456'})
MERGE (f)-[:FOUND_IN]->(a)
RETURN f.id;" 2>&1 | grep -v "^$" || true

log "  finding 6/8: over-privileged IAM (custom, medium)"
docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain "
MERGE (f:Finding {id: 'custom:arn:aws:iam::123456789012:role/checkout-role:over_privileged'})
SET f.source='custom', f.type='identity_risk', f.severity='medium',
    f.cvss_score=0.0, f.title='IAM role has wildcard s3:* permissions',
    f.description='checkout-role grants s3:* which exceeds least privilege',
    f.remediation='Scope IAM policy down to specific buckets/actions',
    f.status='open', f.detected_at='${NOW_TS}', f.metadata='{}',
    f.tags='[\"iam\",\"least-privilege\"]', f.first_seen=timestamp(), f.risk_score=5.1
WITH f MATCH (a:Asset {id: 'arn:aws:ec2:us-east-1:i-0abc123def456'})
MERGE (f)-[:FOUND_IN]->(a)
RETURN f.id;" 2>&1 | grep -v "^$" || true

log "  finding 7/8: privileged pod (k8s-custom, critical)"
docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain "
MERGE (f:Finding {id: 'k8s-custom:k8s:production/pod/checkout-api-7f8b9:privileged-container'})
SET f.source='k8s-custom', f.type='misconfiguration', f.severity='critical',
    f.cvss_score=0.0, f.title='Privileged container running',
    f.description='Pod checkout-api-7f8b9 runs with securityContext.privileged=true',
    f.remediation='Remove securityContext.privileged and enforce restricted PSS',
    f.status='open', f.detected_at='${NOW_TS}', f.metadata='{}',
    f.tags='[\"kubernetes\",\"pod-security\"]', f.first_seen=timestamp(), f.risk_score=8.9
WITH f MATCH (a:Asset {id: 'arn:aws:eks:us-east-1:123456789012:cluster/prod-eks'})
MERGE (f)-[:FOUND_IN]->(a)
RETURN f.id;" 2>&1 | grep -v "^$" || true

log "  finding 8/8: Lambda KMS (checkov, low, resolved)"
docker exec -i "$NEO4J_CONTAINER" cypher-shell -u "$NEO4J_USER" -p "$NEO4J_PASS" --format plain "
MERGE (f:Finding {id: 'checkov:arn:aws:lambda:us-east-1:123456789012:function:payments:CKV_AWS_173'})
SET f.source='checkov', f.type='misconfiguration', f.severity='low',
    f.cvss_score=0.0, f.title='Lambda environment variables not encrypted with CMK',
    f.description='payments function uses default KMS key for env encryption',
    f.remediation='Configure a customer-managed KMS key for environment encryption',
    f.status='resolved', f.detected_at='${WEEK_AGO_TS}', f.metadata='{}',
    f.tags='[\"iac\",\"lambda\"]', f.first_seen=timestamp(), f.risk_score=2.4
WITH f MATCH (a:Asset {id: 'arn:aws:lambda:us-east-1:123456789012:function:payments'})
MERGE (f)-[:FOUND_IN]->(a)
RETURN f.id;" 2>&1 | grep -v "^$" || true

# ── PostgreSQL: assets table (for /api/v1/assets) ───────────────────────────
log "seeding PostgreSQL assets..."
psql_exec "
INSERT INTO assets (id, provider, asset_type, name, region, tags, metadata_json,
                    internet_facing, data_classification, environment, account_id, active)
VALUES
  ('arn:aws:s3:::checkout-logs', 'aws', 's3_bucket', 'checkout-logs', 'us-east-1',
   '{\"env\":\"prod\",\"team\":\"checkout\"}', '{}', true, 'restricted', 'production', '123456789012', true),
  ('arn:aws:ec2:us-east-1:i-0abc123def456', 'aws', 'ec2_instance', 'checkout-api-server', 'us-east-1',
   '{\"env\":\"prod\",\"role\":\"api\"}', '{}', true, 'confidential', 'production', '123456789012', true),
  ('arn:aws:eks:us-east-1:123456789012:cluster/prod-eks', 'aws', 'eks_cluster', 'prod-eks', 'us-east-1',
   '{\"env\":\"prod\"}', '{}', false, 'confidential', 'production', '123456789012', true),
  ('image:org/checkout:v2.3', 'docker', 'container_image', 'org/checkout:v2.3', 'global',
   '{\"registry\":\"ecr\"}', '{}', false, 'confidential', 'production', '123456789012', true),
  ('git:github.com/acme/checkout-api', 'github', 'git_repository', 'acme/checkout-api', 'global',
   '{\"visibility\":\"private\"}', '{}', false, 'confidential', 'production', '123456789012', true),
  ('arn:aws:lambda:us-east-1:123456789012:function:payments', 'aws', 'lambda_function', 'payments', 'us-east-1',
   '{\"env\":\"prod\"}', '{}', false, 'restricted', 'production', '123456789012', true),
  ('arn:aws:ec2:us-east-1:123456789012:security-group/sg-public', 'aws', 'security_group', 'sg-public', 'us-east-1',
   '{\"env\":\"prod\"}', '{}', true, 'public', 'production', '123456789012', true)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name,
  internet_facing = EXCLUDED.internet_facing,
  data_classification = EXCLUDED.data_classification,
  environment = EXCLUDED.environment,
  active = true,
  last_synced_at = NOW(),
  updated_at = NOW();
" 2>&1 | grep -v "^$" || true

log "seed complete."
log "Neo4j nodes:"
cypher "MATCH (n) RETURN labels(n)[0] AS label, count(*) AS cnt;" 2>/dev/null || true
