package store

import (
	"time"
)

// Asset represents a cloud resource stored in PostgreSQL.
type Asset struct {
	ID               string            `db:"id" json:"id"`
	Provider         string            `db:"provider" json:"provider"`         // "aws", "azure", "gcp"
	AssetType        string            `db:"asset_type" json:"asset_type"`     // "ec2", "s3_bucket", "iam_role"
	Name             string            `db:"name" json:"name"`
	Region           string            `db:"region" json:"region"`
	Tags             map[string]string `db:"tags" json:"tags"`
	MetadataJSON     string            `db:"metadata_json" json:"metadata_json"`
	InternetFacing   bool              `db:"internet_facing" json:"internet_facing"`
	DataClassification string          `db:"data_classification" json:"data_classification"`
	Environment      string            `db:"environment" json:"environment"`
	AccountID        string            `db:"account_id" json:"account_id"`
	Active           bool              `db:"active" json:"active"`
	DiscoveredAt     time.Time         `db:"discovered_at" json:"discovered_at"`
	LastSyncedAt     time.Time         `db:"last_synced_at" json:"last_synced_at"`
	CreatedAt        time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time         `db:"updated_at" json:"updated_at"`
}

// AssetFilter represents search/filter parameters for querying assets.
type AssetFilter struct {
	Provider    string   `json:"provider,omitempty"`
	AssetType   string   `json:"asset_type,omitempty"`
	Region      string   `json:"region,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Active      *bool    `json:"active,omitempty"`
	Tags        []string `json:"tags,omitempty"`  // key:value pairs
	Search      string   `json:"search,omitempty"`
	Page        int      `json:"page,omitempty"`
	PageSize    int      `json:"page_size,omitempty"`
}

// AssetDiff represents the difference between two asset scans.
type AssetDiff struct {
	Added   []Asset `json:"added"`
	Updated []Asset `json:"updated"`
	Removed []Asset `json:"removed"`
}

// Constants
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)
