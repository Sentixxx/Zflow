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
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	parent_id INTEGER,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY(parent_id) REFERENCES folders(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS feeds (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	url TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	folder_id INTEGER,
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
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	feed_id INTEGER NOT NULL,
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
	article_id INTEGER PRIMARY KEY,
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
		Description: "embeddings table for vector search metadata and blobs",
		SQL: `
CREATE TABLE IF NOT EXISTS embeddings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_type TEXT NOT NULL,
	source_id INTEGER NOT NULL,
	model TEXT NOT NULL DEFAULT '',
	dimensions INTEGER NOT NULL DEFAULT 1536,
	embedding_blob BLOB NOT NULL DEFAULT X'',
	created_at TEXT NOT NULL DEFAULT '',
	UNIQUE(source_type, source_id, model)
);

CREATE INDEX IF NOT EXISTS idx_embeddings_source ON embeddings(source_type, source_id);
`,
	},
	{
		Version:     3,
		Description: "article source payload",
		SQL: `
ALTER TABLE entries ADD COLUMN source_payload TEXT NOT NULL DEFAULT '';
`,
	},
	{
		Version:     4,
		Description: "feed retention days override",
		SQL: `
ALTER TABLE feeds ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 0;
`,
	},
	{
		Version:     5,
		Description: "add score_reasoning to article_features",
		SQL: `
ALTER TABLE article_features ADD COLUMN score_reasoning TEXT NOT NULL DEFAULT '';
`,
	},
	{
		Version:     6,
		Description: "store embeddings in plain blob column for non-CGO sqlite",
		SQL: `
ALTER TABLE embeddings ADD COLUMN embedding_blob BLOB NOT NULL DEFAULT X'';
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

	for _, m := range sorted {
		if applied[m.Version] {
			continue
		}
		if m.Version == 6 {
			hasColumn, err := tableHasColumn(ctx, db, "embeddings", "embedding_blob")
			if err != nil {
				return fmt.Errorf("inspect migration v%d (%s): %w", m.Version, m.Description, err)
			}
			if hasColumn {
				now := "datetime('now')"
				if _, err := db.ExecContext(ctx,
					`INSERT INTO schema_migrations(version, applied_at) VALUES(?, `+now+`)`,
					m.Version,
				); err != nil {
					return fmt.Errorf("record migration v%d: %w", m.Version, err)
				}
				continue
			}
		}
		if _, err := db.ExecContext(ctx, m.SQL); err != nil {
			return fmt.Errorf("migration v%d (%s): %w", m.Version, m.Description, err)
		}
		now := "datetime('now')"
		if _, err := db.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES(?, `+now+`)`,
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

func tableHasColumn(ctx context.Context, db *sql.DB, table string, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
