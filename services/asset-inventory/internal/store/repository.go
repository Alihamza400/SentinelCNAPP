package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the interface for database operations.
type DB interface {
	Exec(ctx context.Context, sql string, args ...any) (pgx.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Repository handles asset CRUD operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new asset repository.
func NewRepository(ctx context.Context, databaseURL string) (*Repository, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}

	config.MaxConns = 20
	config.MinConns = 2

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return &Repository{pool: pool}, nil
}

// Close closes the connection pool.
func (r *Repository) Close() {
	r.pool.Close()
}

// Pool returns the underlying connection pool.
func (r *Repository) Pool() *pgxpool.Pool {
	return r.pool
}

// UpsertAsset inserts or updates an asset.
func (r *Repository) UpsertAsset(ctx context.Context, asset *Asset) error {
	tagsJSON, err := json.Marshal(asset.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	now := time.Now().UTC()
	_, err = r.pool.Exec(ctx, `
		INSERT INTO assets (id, provider, asset_type, name, region, tags, metadata_json,
		                    internet_facing, data_classification, environment, account_id,
		                    active, discovered_at, last_synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $15)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			tags = EXCLUDED.tags,
			metadata_json = EXCLUDED.metadata_json,
			internet_facing = EXCLUDED.internet_facing,
			environment = EXCLUDED.environment,
			active = EXCLUDED.active,
			last_synced_at = EXCLUDED.last_synced_at,
			updated_at = EXCLUDED.updated_at
	`,
		asset.ID, asset.Provider, asset.AssetType, asset.Name, asset.Region,
		string(tagsJSON), asset.MetadataJSON, asset.InternetFacing,
		asset.DataClassification, asset.Environment, asset.AccountID,
		asset.Active, asset.DiscoveredAt, asset.LastSyncedAt, now,
	)
	return err
}

// BatchUpsertAssets performs a batch upsert of assets in a transaction.
func (r *Repository) BatchUpsertAssets(ctx context.Context, assets []Asset) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	for i := range assets {
		if err := r.upsertAssetTx(ctx, tx, &assets[i]); err != nil {
			return fmt.Errorf("batch upsert at index %d: %w", i, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) upsertAssetTx(ctx context.Context, tx pgx.Tx, asset *Asset) error {
	tagsJSON, err := json.Marshal(asset.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		INSERT INTO assets (id, provider, asset_type, name, region, tags, metadata_json,
		                    internet_facing, data_classification, environment, account_id,
		                    active, discovered_at, last_synced_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $15)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			tags = EXCLUDED.tags,
			metadata_json = EXCLUDED.metadata_json,
			internet_facing = EXCLUDED.internet_facing,
			environment = EXCLUDED.environment,
			active = EXCLUDED.active,
			last_synced_at = EXCLUDED.last_synced_at,
			updated_at = EXCLUDED.updated_at
	`,
		asset.ID, asset.Provider, asset.AssetType, asset.Name, asset.Region,
		string(tagsJSON), asset.MetadataJSON, asset.InternetFacing,
		asset.DataClassification, asset.Environment, asset.AccountID,
		asset.Active, asset.DiscoveredAt, asset.LastSyncedAt, now,
	)
	return err
}

// GetAsset retrieves a single asset by ID.
func (r *Repository) GetAsset(ctx context.Context, id string) (*Asset, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, provider, asset_type, name, region, tags, metadata_json,
		       internet_facing, data_classification, environment, account_id,
		       active, discovered_at, last_synced_at, created_at, updated_at
		FROM assets WHERE id = $1
	`, id)
	return scanAsset(row)
}

// ListAssets retrieves assets with filtering and pagination.
func (r *Repository) ListAssets(ctx context.Context, filter AssetFilter) ([]Asset, int, error) {
	where := []string{"1=1"}
	args := []any{}
	argIdx := 1

	if filter.Provider != "" {
		where = append(where, fmt.Sprintf("provider = $%d", argIdx))
		args = append(args, filter.Provider)
		argIdx++
	}
	if filter.AssetType != "" {
		where = append(where, fmt.Sprintf("asset_type = $%d", argIdx))
		args = append(args, filter.AssetType)
		argIdx++
	}
	if filter.Region != "" {
		where = append(where, fmt.Sprintf("region = $%d", argIdx))
		args = append(args, filter.Region)
		argIdx++
	}
	if filter.Environment != "" {
		where = append(where, fmt.Sprintf("environment = $%d", argIdx))
		args = append(args, filter.Environment)
		argIdx++
	}
	if filter.Active != nil {
		where = append(where, fmt.Sprintf("active = $%d", argIdx))
		args = append(args, *filter.Active)
		argIdx++
	}
	if filter.Search != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR id ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}

	// Count total
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM assets WHERE %s", strings.Join(where, " AND "))
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting assets: %w", err)
	}

	// Pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > MaxPageSize {
		pageSize = DefaultPageSize
	}
	offset := (page - 1) * pageSize

	whereClause := strings.Join(where, " AND ")
	sql := fmt.Sprintf(`
		SELECT id, provider, asset_type, name, region, tags, metadata_json,
		       internet_facing, data_classification, environment, account_id,
		       active, discovered_at, last_synced_at, created_at, updated_at
		FROM assets WHERE %s
		ORDER BY updated_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, pageSize, offset)
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("querying assets: %w", err)
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scanning asset row: %w", err)
		}
		assets = append(assets, *a)
	}

	return assets, total, nil
}

// GetAllActiveAssets retrieves all active assets (for sync comparison).
func (r *Repository) GetAllActiveAssets(ctx context.Context) ([]Asset, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, provider, asset_type, name, region, tags, metadata_json,
		       internet_facing, data_classification, environment, account_id,
		       active, discovered_at, last_synced_at, created_at, updated_at
		FROM assets WHERE active = true
	`)
	if err != nil {
		return nil, fmt.Errorf("querying active assets: %w", err)
	}
	defer rows.Close()

	var assets []Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning asset row: %w", err)
		}
		assets = append(assets, *a)
	}
	return assets, nil
}

// DeactivateAsset marks an asset as inactive.
func (r *Repository) DeactivateAsset(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE assets SET active = false, updated_at = NOW() WHERE id = $1`, id,
	)
	return err
}

// RecordHistoryRecords snapshots the current state into asset_history.
func (r *Repository) RecordHistory(ctx context.Context, asset *Asset, changeType string) error {
	tagsJSON, err := json.Marshal(asset.Tags)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO asset_history (asset_id, provider, asset_type, name, region, tags,
		                           metadata_json, internet_facing, environment, active, change_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, asset.ID, asset.Provider, asset.AssetType, asset.Name, asset.Region,
		string(tagsJSON), asset.MetadataJSON, asset.InternetFacing,
		asset.Environment, asset.Active, changeType)
	return err
}

// scanAsset scans a single asset row from a row-like interface.
func scanAsset(row interface {
	Scan(dest ...any) error
}) (*Asset, error) {
	var a Asset
	var tagsJSON []byte

	err := row.Scan(
		&a.ID, &a.Provider, &a.AssetType, &a.Name, &a.Region, &tagsJSON,
		&a.MetadataJSON, &a.InternetFacing, &a.DataClassification,
		&a.Environment, &a.AccountID, &a.Active,
		&a.DiscoveredAt, &a.LastSyncedAt, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &a.Tags); err != nil {
			a.Tags = make(map[string]string)
		}
	} else {
		a.Tags = make(map[string]string)
	}

	return &a, nil
}
