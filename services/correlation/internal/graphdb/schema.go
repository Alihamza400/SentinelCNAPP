package graphdb

// Schema defines all node labels and relationship types used in the Neo4j graph.
const (
	// Node labels
	NodeAsset      = "Asset"
	NodeFinding    = "Finding"
	NodeIdentity   = "Identity"
	NodeService    = "Service"
	NodeDependency = "Dependency"
	NodeCode       = "Code"
	NodeEndpoint   = "NetworkEndpoint"
	NodePolicy     = "Policy"
	NodeRisk       = "RiskEvaluation"

	// Relationship types
	RelFoundIn     = "FOUND_IN"
	RelHasIdentity = "HAS_IDENTITY"
	RelCanAccess   = "CAN_ACCESS"
	RelDependsOn   = "DEPENDS_ON"
	RelConnectsTo  = "CONNECTS_TO"
	RelHasPolicy   = "HAS_POLICY"
	RelDefinedBy   = "DEFINED_BY"
	RelHasRisk     = "HAS_RISK"
	RelHosts       = "HOSTS"
	RelExecutedBy  = "EXECUTED_BY"
)

// ── Merge Queries ─────────────────────────────────────────────

// MergeAsset creates or updates an Asset node.
const MergeAsset = `
	MERGE (a:Asset {id: $id})
	ON CREATE SET
		a.provider = $provider,
		a.type = $type,
		a.name = $name,
		a.region = $region,
		a.environment = $environment,
		a.internet_facing = $internet_facing,
		a.tags = $tags,
		a.active = true,
		a.first_seen = $now
	ON MATCH SET
		a.name = $name,
		a.region = $region,
		a.environment = $environment,
		a.internet_facing = $internet_facing,
		a.tags = $tags,
		a.active = true,
		a.last_seen = $now
	RETURN a
`

// MergeFinding creates or updates a Finding node linked to an Asset.
const MergeFinding = `
	MERGE (f:Finding {id: $id})
	ON CREATE SET
		f.source = $source,
		f.type = $type,
		f.severity = $severity,
		f.cvss_score = $cvss_score,
		f.title = $title,
		f.description = $description,
		f.remediation = $remediation,
		f.status = $status,
		f.detected_at = $detected_at,
		f.metadata = $metadata,
		f.tags = $tags,
		f.first_seen = $now,
		f.risk_score = 0.0
	ON MATCH SET
		f.status = $status,
		f.last_updated = $now,
		f.risk_score = CASE WHEN $risk_score IS NOT NULL THEN $risk_score ELSE f.risk_score END
	WITH f
	MATCH (a:Asset {id: $asset_id})
	MERGE (f)-[:FOUND_IN]->(a)
	RETURN f, a
`

// MergeIdentity creates or updates an Identity node and links it to an Asset.
const MergeIdentity = `
	MERGE (i:Identity {arn: $arn})
	ON CREATE SET
		i.name = $name,
		i.type = $type,
		i.provider = $provider,
		i.permissions = $permissions,
		i.active = true,
		i.first_seen = $now
	ON MATCH SET
		i.name = $name,
		i.permissions = $permissions,
		i.last_seen = $now
	WITH i
	MATCH (a:Asset {id: $asset_id})
	MERGE (a)-[:HAS_IDENTITY]->(i)
	RETURN a, i
`

// LinkIdentityToResource creates a CAN_ACCESS edge from an Identity to a resource.
const LinkIdentityToResource = `
	MATCH (i:Identity {arn: $identity_arn})
	MATCH (r:Asset {id: $resource_id})
	MERGE (i)-[:CAN_ACCESS {permission: $permission}]->(r)
	RETURN i, r
`

// ── Query Queries ─────────────────────────────────────────────

// GetAssetWithFindings returns an asset with all linked findings.
const GetAssetWithFindings = `
	MATCH (a:Asset {id: $id})
	OPTIONAL MATCH (f:Finding)-[:FOUND_IN]->(a)
	OPTIONAL MATCH (a)-[:HAS_IDENTITY]->(i:Identity)
	RETURN a,
		collect(DISTINCT {id: f.id, source: f.source, type: f.type,
			severity: f.severity, title: f.title, status: f.status,
			risk_score: f.risk_score, detected_at: f.detected_at}) AS findings,
		collect(DISTINCT {arn: i.arn, name: i.name, type: i.type}) AS identities
`

// ListFindings returns findings with optional filters and pagination.
const ListFindings = `
	MATCH (f:Finding)-[:FOUND_IN]->(a:Asset)
	WHERE ($severity IS NULL OR f.severity = $severity)
		AND ($source IS NULL OR f.source = $source)
		AND ($type IS NULL OR f.type = $type)
		AND ($status IS NULL OR f.status = $status)
		AND ($search IS NULL OR f.title CONTAINS $search OR f.description CONTAINS $search)
	OPTIONAL MATCH (f)-[:HAS_RISK]->(r:RiskEvaluation)
	RETURN f, a.id AS asset_id,
		r.score AS risk_score
	ORDER BY
		CASE f.severity
			WHEN 'critical' THEN 0
			WHEN 'high' THEN 1
			WHEN 'medium' THEN 2
			WHEN 'low' THEN 3
			ELSE 4
		END ASC,
		f.detected_at DESC
	SKIP $skip
	LIMIT $limit
`

// CountFindings returns the total count matching filters.
const CountFindings = `
	MATCH (f:Finding)-[:FOUND_IN]->(a:Asset)
	WHERE ($severity IS NULL OR f.severity = $severity)
		AND ($source IS NULL OR f.source = $source)
		AND ($type IS NULL OR f.type = $type)
		AND ($status IS NULL OR f.status = $status)
		AND ($search IS NULL OR f.title CONTAINS $search OR f.description CONTAINS $search)
	RETURN count(f) AS total
`

// GetDashboardStats returns aggregate stats for the dashboard.
const GetDashboardStats = `
	MATCH (a:Asset) WHERE a.active = true
	WITH count(a) AS total_assets
	OPTIONAL MATCH (f:Finding) WHERE f.status = 'open'
	WITH total_assets, count(f) AS open_findings
	OPTIONAL MATCH (f:Finding {severity: 'critical'}) WHERE f.status = 'open'
	WITH total_assets, open_findings, count(f) AS critical_findings
	RETURN total_assets, open_findings, critical_findings
`

// GetFindingSeverityDistribution returns finding counts by severity.
const GetFindingSeverityDistribution = `
	MATCH (f:Finding)
	WHERE f.status = 'open'
	RETURN f.severity AS severity, count(f) AS count
	ORDER BY CASE f.severity
		WHEN 'critical' THEN 0
		WHEN 'high' THEN 1
		WHEN 'medium' THEN 2
		WHEN 'low' THEN 3
		ELSE 4
	END
`

// GetAssetTypeDistribution returns asset counts by type.
const GetAssetTypeDistribution = `
	MATCH (a:Asset)
	WHERE a.active = true
	RETURN a.type AS type, count(a) AS count
	ORDER BY count DESC
	LIMIT 20
`

// GetAttackPaths finds all paths from critical findings to exposed resources.
const GetAttackPaths = `
	MATCH path = (f:Finding {severity: 'critical'})-[:FOUND_IN]->(a:Asset)
		-[:HAS_IDENTITY]->(i:Identity)-[:CAN_ACCESS]->(r:Asset)
	WHERE r.internet_facing = true OR r.name CONTAINS 'public'
	RETURN [node IN nodes(path) | {
		id: node.id,
		labels: labels(node),
		type: COALESCE(node.type, node.source, 'unknown'),
		severity: node.severity,
		title: node.title
	}] AS nodes,
	[rel IN relationships(path) | type(rel)] AS relationships
	LIMIT $limit
`

// GetGraphData returns all nodes and edges for the interactive graph visualization.
const GetGraphData = `
	MATCH (a:Asset)
	WHERE a.active = true
	OPTIONAL MATCH (a)<-[:FOUND_IN]-(f:Finding)
	OPTIONAL MATCH (a)-[:HAS_IDENTITY]->(i:Identity)
	OPTIONAL MATCH (i)-[:CAN_ACCESS]->(r:Asset)
	WITH a, collect(DISTINCT {id: f.id, labels: ['Finding'], type: f.type, severity: f.severity, title: f.title}) AS findings,
		collect(DISTINCT {id: i.arn, labels: ['Identity'], type: i.type, name: i.name}) AS identities
	RETURN {id: a.id, labels: ['Asset'], type: a.type, name: a.name, region: a.region, environment: a.environment,
		internet_facing: a.internet_facing} AS node,
		findings + identities AS connected
	LIMIT $limit
`
