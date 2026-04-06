package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/db"
	_ "modernc.org/sqlite"
)

func main() {
	sqlitePath := os.Getenv("SQLITE_PATH")
	if sqlitePath == "" {
		sqlitePath = "./data/zflow.db"
	}
	pgDSN := os.Getenv("ZFLOW_POSTGRES_DSN")
	if pgDSN == "" {
		pgDSN = "postgres://localhost:5432/zflow?sslmode=disable"
	}

	fmt.Printf("SQLite path: %s\n", sqlitePath)
	fmt.Printf("PostgreSQL DSN: %s\n", pgDSN)

	src, err := sql.Open("sqlite", fmt.Sprintf("file:%s?_pragma=foreign_keys(OFF)", sqlitePath))
	if err != nil {
		fatal("open sqlite: %v", err)
	}
	defer src.Close()

	dst, err := db.OpenPostgres(pgDSN)
	if err != nil {
		fatal("open postgres: %v", err)
	}
	defer dst.Close()

	ctx := context.Background()
	if err := db.RunMigrations(ctx, dst); err != nil {
		fatal("run migrations: %v", err)
	}

	migrateFolders(ctx, src, dst)
	migrateFeeds(ctx, src, dst)
	migrateEntries(ctx, src, dst)
	migrateArticleFeatures(ctx, src, dst)
	migrateAppSettings(ctx, src, dst)
	syncSequences(ctx, dst)

	fmt.Println("Migration complete.")
}

func migrateFolders(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `SELECT id, name, parent_id, created_at, updated_at FROM folders ORDER BY id ASC`)
	if err != nil {
		fatal("query folders: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var name, createdAt, updatedAt string
		var parentID sql.NullInt64
		if err := rows.Scan(&id, &name, &parentID, &createdAt, &updatedAt); err != nil {
			fatal("scan folder: %v", err)
		}
		_, err := dst.ExecContext(ctx,
			`INSERT INTO folders(id, name, parent_id, created_at, updated_at) VALUES($1, $2, $3, $4, $5)
			 ON CONFLICT(id) DO NOTHING`,
			id, name, nullVal(parentID), createdAt, updatedAt,
		)
		if err != nil {
			fatal("insert folder %d: %v", id, err)
		}
		count++
	}
	fmt.Printf("Migrated %d folders\n", count)
}

func migrateFeeds(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx,
		`SELECT id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count,
		        last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at, updated_at
		 FROM feeds ORDER BY id ASC`)
	if err != nil {
		fatal("query feeds: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id int64
		var url, title, customScript, customScriptLang, iconPath, iconFetchedAt string
		var itemCount int
		var lastFetchedAt, lastFetchStatus, lastFetchError, etag, lastModified, createdAt, updatedAt string
		var folderID sql.NullInt64
		if err := rows.Scan(&id, &url, &title, &folderID, &customScript, &customScriptLang, &iconPath, &iconFetchedAt,
			&itemCount, &lastFetchedAt, &lastFetchStatus, &lastFetchError, &etag, &lastModified, &createdAt, &updatedAt); err != nil {
			fatal("scan feed: %v", err)
		}
		_, err := dst.ExecContext(ctx,
			`INSERT INTO feeds(id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count,
			        last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at, updated_at)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
			 ON CONFLICT(id) DO NOTHING`,
			id, url, title, nullVal(folderID), customScript, customScriptLang, iconPath, iconFetchedAt,
			itemCount, lastFetchedAt, lastFetchStatus, lastFetchError, etag, lastModified, createdAt, updatedAt,
		)
		if err != nil {
			fatal("insert feed %d: %v", id, err)
		}
		count++
	}
	fmt.Printf("Migrated %d feeds\n", count)
}

func migrateEntries(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx,
		`SELECT id, feed_id, title, link, summary, ai_summary, ai_summary_status, ai_summary_updated_at,
		        display_summary, display_summary_status, display_summary_updated_at, full_content, cover_url, published_at,
		        is_read, is_favorite, favorited_at, quality_score, relevance_score, novelty_score, composite_score,
		        created_at, updated_at
		 FROM entries ORDER BY id ASC`)
	if err != nil {
		fatal("query entries: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id, feedID int64
		var title, link, summary, aiSummary, aiSummaryStatus, aiSummaryAt string
		var displaySummary, displaySummaryStatus, displaySummaryAt, fullContent, coverURL, publishedAt string
		var isRead, isFavorite, qualityScore, relevanceScore, noveltyScore, compositeScore int
		var favoritedAt, createdAt, updatedAt string
		if err := rows.Scan(&id, &feedID, &title, &link, &summary, &aiSummary, &aiSummaryStatus, &aiSummaryAt,
			&displaySummary, &displaySummaryStatus, &displaySummaryAt, &fullContent, &coverURL, &publishedAt,
			&isRead, &isFavorite, &favoritedAt, &qualityScore, &relevanceScore, &noveltyScore, &compositeScore,
			&createdAt, &updatedAt); err != nil {
			fatal("scan entry: %v", err)
		}
		_, err := dst.ExecContext(ctx,
			`INSERT INTO entries(id, feed_id, title, link, summary, ai_summary, ai_summary_status, ai_summary_updated_at,
			        display_summary, display_summary_status, display_summary_updated_at, full_content, cover_url, published_at,
			        is_read, is_favorite, favorited_at, quality_score, relevance_score, novelty_score, composite_score,
			        created_at, updated_at)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)
			 ON CONFLICT(id) DO NOTHING`,
			id, feedID, title, link, summary, aiSummary, aiSummaryStatus, aiSummaryAt,
			displaySummary, displaySummaryStatus, displaySummaryAt, fullContent, coverURL, publishedAt,
			isRead, isFavorite, favoritedAt, qualityScore, relevanceScore, noveltyScore, compositeScore,
			createdAt, updatedAt,
		)
		if err != nil {
			fatal("insert entry %d: %v", id, err)
		}
		count++
	}
	fmt.Printf("Migrated %d entries\n", count)
}

func migrateArticleFeatures(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx,
		`SELECT article_id, gate_status, quality_score, relevance_score, depth_score, freshness_score,
		        novelty_score, composite_score, content_fingerprint, feature_version, scored_at, updated_at
		 FROM article_features ORDER BY article_id ASC`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			fmt.Println("Skipped article_features (table does not exist)")
			return
		}
		fatal("query article_features: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var articleID int64
		var gateStatus, contentFingerprint, scoredAt, updatedAt string
		var qualityScore, relevanceScore, depthScore, freshnessScore, noveltyScore, compositeScore, featureVersion int
		if err := rows.Scan(&articleID, &gateStatus, &qualityScore, &relevanceScore, &depthScore, &freshnessScore,
			&noveltyScore, &compositeScore, &contentFingerprint, &featureVersion, &scoredAt, &updatedAt); err != nil {
			fatal("scan article_features: %v", err)
		}
		_, err := dst.ExecContext(ctx,
			`INSERT INTO article_features(article_id, gate_status, quality_score, relevance_score, depth_score, freshness_score,
			        novelty_score, composite_score, content_fingerprint, feature_version, scored_at, updated_at)
			 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			 ON CONFLICT(article_id) DO NOTHING`,
			articleID, gateStatus, qualityScore, relevanceScore, depthScore, freshnessScore,
			noveltyScore, compositeScore, contentFingerprint, featureVersion, scoredAt, updatedAt,
		)
		if err != nil {
			fatal("insert article_features %d: %v", articleID, err)
		}
		count++
	}
	fmt.Printf("Migrated %d article_features\n", count)
}

func migrateAppSettings(ctx context.Context, src, dst *sql.DB) {
	rows, err := src.QueryContext(ctx, `SELECT key, value, updated_at FROM app_settings`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			fmt.Println("Skipped app_settings (table does not exist)")
			return
		}
		fatal("query app_settings: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var key, value, updatedAt string
		if err := rows.Scan(&key, &value, &updatedAt); err != nil {
			fatal("scan app_settings: %v", err)
		}
		_, err := dst.ExecContext(ctx,
			`INSERT INTO app_settings(key, value, updated_at) VALUES($1, $2, $3)
			 ON CONFLICT(key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`,
			key, value, updatedAt,
		)
		if err != nil {
			fatal("insert app_settings %s: %v", key, err)
		}
		count++
	}
	fmt.Printf("Migrated %d app_settings\n", count)
}

func syncSequences(ctx context.Context, dst *sql.DB) {
	tables := []struct{ table, column string }{
		{"folders", "id"},
		{"feeds", "id"},
		{"entries", "id"},
	}
	for _, t := range tables {
		_, err := dst.ExecContext(ctx, fmt.Sprintf(
			`SELECT setval(pg_get_serial_sequence('%s', '%s'), COALESCE((SELECT MAX(%s) FROM %s), 1))`,
			t.table, t.column, t.column, t.table,
		))
		if err != nil {
			fmt.Printf("Warning: failed to sync sequence for %s.%s: %v\n", t.table, t.column, err)
		}
	}
	fmt.Println("Synced sequences")
}

func nullVal(n sql.NullInt64) any {
	if n.Valid {
		return n.Int64
	}
	return nil
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}
