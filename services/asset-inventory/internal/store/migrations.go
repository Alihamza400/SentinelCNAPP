package store

import (
	"context"
	"fmt"
)

// Migration represents a database schema migration.
type Migration struct {
	Version int
	Name    string
	SQL     string
}

// Migrations returns all schema migrations in order.
func Migrations() []Migration {
	return []Migration{
		{
			Version: 1,
			Name:    "create_assets_table",
			SQL: `
				CREATE EXTENSION IF NOT EXISTS pgcrypto;

				CREATE TABLE IF NOT EXISTS assets (
					id              TEXT PRIMARY KEY,
					provider        TEXT NOT NULL DEFAULT 'aws',
					asset_type      TEXT NOT NULL,
					name            TEXT NOT NULL DEFAULT '',
					region          TEXT NOT NULL DEFAULT '',
					tags            JSONB NOT NULL DEFAULT '{}',
					metadata_json   TEXT NOT NULL DEFAULT '{}',
					internet_facing BOOLEAN NOT NULL DEFAULT false,
					data_classification TEXT NOT NULL DEFAULT 'public',
					environment     TEXT NOT NULL DEFAULT 'production',
					account_id      TEXT NOT NULL DEFAULT '',
					active          BOOLEAN NOT NULL DEFAULT true,
					discovered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					last_synced_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS idx_assets_provider ON assets(provider);
				CREATE INDEX IF NOT EXISTS idx_assets_type ON assets(asset_type);
				CREATE INDEX IF NOT EXISTS idx_assets_region ON assets(region);
				CREATE INDEX IF NOT EXISTS idx_assets_active ON assets(active);
				CREATE INDEX IF NOT EXISTS idx_assets_tags ON assets USING GIN(tags);
				CREATE INDEX IF NOT EXISTS idx_assets_name_trgm ON assets USING GIN(name gin_trgm_ops);
				CREATE INDEX IF NOT EXISTS idx_assets_updated ON assets(updated_at);
			`,
		},
		{
			Version: 2,
			Name:    "create_asset_history_table",
			SQL: `
				CREATE TABLE IF NOT EXISTS asset_history (
					id              SERIAL PRIMARY KEY,
					asset_id        TEXT NOT NULL,
					provider        TEXT NOT NULL DEFAULT 'aws',
					asset_type      TEXT NOT NULL,
					name            TEXT NOT NULL DEFAULT '',
					region          TEXT NOT NULL DEFAULT '',
					tags            JSONB NOT NULL DEFAULT '{}',
					metadata_json   TEXT NOT NULL DEFAULT '{}',
					internet_facing BOOLEAN NOT NULL DEFAULT false,
					environment     TEXT NOT NULL DEFAULT 'production',
					active          BOOLEAN NOT NULL DEFAULT true,
					snapshot_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					change_type     TEXT NOT NULL DEFAULT 'created'  -- created, updated, deleted
				);

				CREATE INDEX IF NOT EXISTS idx_asset_history_asset_id ON asset_history(asset_id);
				CREATE INDEX IF NOT EXISTS idx_asset_history_snapshot ON asset_history(snapshot_at);
			`,
		},
		{
			Version: 3,
			Name:    "create_users_teams_tables",
			SQL: `
				CREATE TABLE IF NOT EXISTS teams (
					id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
					name        TEXT NOT NULL UNIQUE,
					slug        TEXT NOT NULL UNIQUE,
					description TEXT NOT NULL DEFAULT '',
					created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE TABLE IF NOT EXISTS users (
					id          TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
					email       TEXT NOT NULL UNIQUE,
					name        TEXT NOT NULL DEFAULT '',
					oidc_sub    TEXT UNIQUE,
					team_id     TEXT REFERENCES teams(id),
					roles       TEXT[] NOT NULL DEFAULT '{viewer}',
					created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
				CREATE INDEX IF NOT EXISTS idx_users_team ON users(team_id);
			`,
		},
	}
}

// RunMigrations executes all pending migrations.
func RunMigrations(ctx context.Context, db DB) error {
	for _, m := range Migrations() {
		if err := runMigration(ctx, db, m); err != nil {
			return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
		}
	}
	return nil
}

func runMigration(ctx context.Context, db DB, m Migration) error {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, m.Version,
	).Scan(&exists)
	if err != nil {
		// Table might not exist yet
		_, err := db.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				name TEXT NOT NULL,
				applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			)
		`)
		if err != nil {
			return fmt.Errorf("creating schema_migrations table: %w", err)
		}
	}

	if exists {
		return nil
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, m.SQL); err != nil {
		return fmt.Errorf("exec sql: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		m.Version, m.Name,
	); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit(ctx)
}
