package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Client wraps a Neo4j driver connection with convenience methods.
type Client struct {
	driver neo4j.DriverWithContext
}

// New creates a new Neo4j graph client.
func New(ctx context.Context, uri, username, password string) (*Client, error) {
	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("creating neo4j driver: %w", err)
	}

	if err := driver.VerifyConnectivity(ctx); err != nil {
		driver.Close(ctx)
		return nil, fmt.Errorf("verifying neo4j connectivity: %w", err)
	}

	return &Client{driver: driver}, nil
}

// NewWithDriver creates a client from an existing driver.
func NewWithDriver(driver neo4j.DriverWithContext) *Client {
	return &Client{driver: driver}
}

// Write executes a write transaction.
//
// The transaction is committed before returning; any result cursor produced by
// the query is closed with it, so only the error is surfaced to callers.
func (c *Client) Write(ctx context.Context, query string, params map[string]any) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return tx.Run(ctx, query, params)
	})
	return err
}

// Read executes a read transaction.
func (c *Client) Read(ctx context.Context, query string, params map[string]any) ([]*neo4j.Record, error) {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cursor, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var records []*neo4j.Record
		for cursor.Next(ctx) {
			records = append(records, cursor.Record())
		}
		return records, cursor.Err()
	})
	if err != nil {
		return nil, err
	}

	return result.([]*neo4j.Record), nil
}

// ReadSingle reads a single record.
func (c *Client) ReadSingle(ctx context.Context, query string, params map[string]any) (*neo4j.Record, error) {
	records, err := c.Read(ctx, query, params)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

// BatchWrite executes multiple write queries in a single transaction.
func (c *Client) BatchWrite(ctx context.Context, queries []struct {
	Query  string
	Params map[string]any
}) error {
	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		for _, q := range queries {
			if _, err := tx.Run(ctx, q.Query, q.Params); err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

// Health checks connectivity to the graph database.
func (c *Client) Health(ctx context.Context) error {
	return c.driver.VerifyConnectivity(ctx)
}

// Close closes the driver connection.
func (c *Client) Close(ctx context.Context) error {
	return c.driver.Close(ctx)
}

// Common Cypher queries used across services.
const (
	// MergeAsset merges an asset node.
	MergeAsset = `
		MERGE (a:Asset {id: $id})
		ON CREATE SET a.type = $type, a.provider = $provider, a.name = $name,
			a.region = $region, a.created_at = $created_at, a.active = true
		ON MATCH SET a.last_seen_at = $last_seen_at, a.active = true
		RETURN a
	`

	// MergeFinding merges a finding node linked to an asset.
	MergeFinding = `
		MERGE (f:Finding {id: $id})
		ON CREATE SET f.source = $source, f.type = $type, f.severity = $severity,
			f.title = $title, f.status = $status, f.detected_at = $detected_at
		ON MATCH SET f.status = $status, f.last_updated = $last_updated
		WITH f
		MATCH (a:Asset {id: $asset_id})
		MERGE (f)-[:FOUND_IN]->(a)
		RETURN f, a
	`

	// LinkAssetToIdentity links an asset to an IAM identity.
	LinkAssetToIdentity = `
		MATCH (a:Asset {id: $asset_id})
		MERGE (i:Identity {arn: $identity_arn})
		ON CREATE SET i.type = $identity_type, i.name = $identity_name
		MERGE (a)-[:HAS_IDENTITY]->(i)
		RETURN a, i
	`

	// LinkIdentityToResource links an identity to a resource it can access.
	LinkIdentityToResource = `
		MATCH (i:Identity {arn: $identity_arn})
		MATCH (r:Asset {id: $resource_id})
		MERGE (i)-[:CAN_ACCESS {permission: $permission}]->(r)
		RETURN i, r
	`

	// GetAssetWithFindings returns an asset and all its linked findings.
	GetAssetWithFindings = `
		MATCH (a:Asset {id: $id})
		OPTIONAL MATCH (a)<-[:FOUND_IN]-(f:Finding)
		RETURN a, collect(f) AS findings
	`

	// GetAttackPaths finds all paths from a finding to exposed resources.
	GetAttackPaths = `
		MATCH path = (f:Finding {severity: "critical"})-[:FOUND_IN]->(a:Asset)
			-[:HAS_IDENTITY]->(i:Identity)-[:CAN_ACCESS]->(r:Asset {public: true})
		RETURN path LIMIT $limit
	`
)

// DefaultTimeouts
const (
	DefaultQueryTimeout = 30 * time.Second
	DefaultBatchSize    = 100
)
