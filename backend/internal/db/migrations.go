package db

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

type migration struct {
	Version     int
	Description string
	SQL         string
}

var migrations = []migration{
	{
		Version:     1,
		Description: "initial schema",
		SQL: `
CREATE TABLE IF NOT EXISTS folders (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	parent_id BIGINT,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(parent_id) REFERENCES folders(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS feeds (
	id BIGSERIAL PRIMARY KEY,
	url TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	folder_id BIGINT,
	custom_script TEXT NOT NULL DEFAULT '',
	custom_script_lang TEXT NOT NULL DEFAULT 'shell',
	icon_path TEXT NOT NULL DEFAULT '',
	icon_fetched_at TEXT NOT NULL DEFAULT '',
	item_count INTEGER NOT NULL DEFAULT 0,
	last_fetched_at TEXT NOT NULL,
	last_fetch_status TEXT NOT NULL,
	last_fetch_error TEXT NOT NULL DEFAULT '',
	etag TEXT NOT NULL DEFAULT '',
	last_modified TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(folder_id) REFERENCES folders(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS entries (
	id BIGSERIAL PRIMARY KEY,
	feed_id BIGINT NOT NULL,
	title TEXT NOT NULL,
	link TEXT NOT NULL DEFAULT '',
	summary TEXT NOT NULL DEFAULT '',
	ai_summary TEXT NOT NULL DEFAULT '',
	ai_summary_status TEXT NOT NULL DEFAULT '',
	ai_summary_updated_at TEXT NOT NULL DEFAULT '',
	display_summary TEXT NOT NULL DEFAULT '',
	display_summary_status TEXT NOT NULL DEFAULT '',
	display_summary_updated_at TEXT NOT NULL DEFAULT '',
	full_content TEXT NOT NULL DEFAULT '',
	cover_url TEXT NOT NULL DEFAULT '',
	published_at TEXT NOT NULL DEFAULT '',
	is_read INTEGER NOT NULL DEFAULT 0,
	is_favorite INTEGER NOT NULL DEFAULT 0,
	favorited_at TEXT NOT NULL DEFAULT '',
	quality_score INTEGER NOT NULL DEFAULT 0,
	relevance_score INTEGER NOT NULL DEFAULT 0,
	novelty_score INTEGER NOT NULL DEFAULT 0,
	composite_score INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(feed_id) REFERENCES feeds(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS article_features (
	article_id BIGINT PRIMARY KEY,
	gate_status TEXT NOT NULL DEFAULT '',
	quality_score INTEGER NOT NULL DEFAULT 0,
	relevance_score INTEGER NOT NULL DEFAULT 0,
	depth_score INTEGER NOT NULL DEFAULT 0,
	freshness_score INTEGER NOT NULL DEFAULT 0,
	novelty_score INTEGER NOT NULL DEFAULT 0,
	composite_score INTEGER NOT NULL DEFAULT 0,
	content_fingerprint TEXT NOT NULL DEFAULT '',
	feature_version INTEGER NOT NULL DEFAULT 0,
	scored_at TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL DEFAULT '',
	FOREIGN KEY(article_id) REFERENCES entries(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS app_settings (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_feeds_folder_id ON feeds(folder_id);
CREATE INDEX IF NOT EXISTS idx_entries_feed_id ON entries(feed_id);
CREATE INDEX IF NOT EXISTS idx_entries_created_at ON entries(created_at);
CREATE INDEX IF NOT EXISTS idx_article_features_scored_at ON article_features(scored_at);
`,
	},
	{
		Version:     2,
		Description: "embeddings table for vector search",
		SQL: `
CREATE TABLE IF NOT EXISTS embeddings (
	id BIGSERIAL PRIMARY KEY,
	source_type TEXT NOT NULL,
	source_id BIGINT NOT NULL,
	model TEXT NOT NULL DEFAULT '',
	embedding vector(1536),
	dimensions INTEGER NOT NULL DEFAULT 1536,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE(source_type, source_id, model)
);

CREATE INDEX IF NOT EXISTS idx_embeddings_source ON embeddings(source_type, source_id);
`,
	},
	{
		Version:     3,
		Description: "agent infrastructure tables",
		SQL: `
CREATE TABLE IF NOT EXISTS agent_runs (
	id BIGSERIAL PRIMARY KEY,
	agent_type TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'running',
	input_summary TEXT NOT NULL DEFAULT '',
	output_summary TEXT NOT NULL DEFAULT '',
	items_processed INTEGER NOT NULL DEFAULT 0,
	items_created INTEGER NOT NULL DEFAULT 0,
	error TEXT NOT NULL DEFAULT '',
	started_at TIMESTAMPTZ NOT NULL,
	completed_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_agent_runs_type ON agent_runs(agent_type);

CREATE TABLE IF NOT EXISTS user_interest_profiles (
	id BIGSERIAL PRIMARY KEY,
	label TEXT NOT NULL DEFAULT '',
	interest_embedding vector(1536),
	weight REAL NOT NULL DEFAULT 1.0,
	source_article_ids BIGINT[] DEFAULT '{}',
	last_reinforced_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS topic_clusters (
	id BIGSERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	summary TEXT NOT NULL DEFAULT '',
	centroid_embedding vector(1536),
	article_count INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS topic_cluster_members (
	cluster_id BIGINT NOT NULL REFERENCES topic_clusters(id) ON DELETE CASCADE,
	article_id BIGINT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
	similarity REAL NOT NULL DEFAULT 0,
	is_representative BOOLEAN NOT NULL DEFAULT FALSE,
	added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY(cluster_id, article_id)
);
CREATE INDEX IF NOT EXISTS idx_cluster_members_article ON topic_cluster_members(article_id);

CREATE TABLE IF NOT EXISTS topic_briefs (
	id BIGSERIAL PRIMARY KEY,
	title TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	content TEXT NOT NULL DEFAULT '',
	level TEXT NOT NULL DEFAULT 'daily',
	period_start DATE NOT NULL,
	period_end DATE NOT NULL,
	source_cluster_ids BIGINT[] DEFAULT '{}',
	source_article_ids BIGINT[] DEFAULT '{}',
	parent_brief_id BIGINT REFERENCES topic_briefs(id),
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_topic_briefs_level ON topic_briefs(level, period_start);
`,
	},
}

func RunMigrations(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL DEFAULT ''
		)
	`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := loadAppliedVersions(ctx, db)
	if err != nil {
		return err
	}

	sorted := make([]migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version < sorted[j].Version })

	// Try to create pgvector extension; ignore errors if not superuser
	// (extension should be pre-created by DBA in production).
	_, _ = db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`)

	for _, m := range sorted {
		if applied[m.Version] {
			continue
		}
		if _, err := db.ExecContext(ctx, m.SQL); err != nil {
			return fmt.Errorf("migration v%d (%s): %w", m.Version, m.Description, err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES($1, NOW()::TEXT)`,
			m.Version,
		); err != nil {
			return fmt.Errorf("record migration v%d: %w", m.Version, err)
		}
	}
	return nil
}

func loadAppliedVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, nil
}
