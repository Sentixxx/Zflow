package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/db"
	"github.com/Sentixxx/Zflow/backend/internal/model"
)

var ErrFeedExists = errors.New("feed already exists")
var ErrFolderNameEmpty = errors.New("folder name is required")

type ArticleSeed struct {
	Title       string
	Link        string
	Summary     string
	FullContent string
	CoverURL    string
	PublishedAt string
}

type SQLiteFeedRepository struct {
	db *sql.DB
}

func NewSQLiteFeedRepository(path string) (*SQLiteFeedRepository, error) {
	dbConn, err := db.OpenSQLite(path)
	if err != nil {
		return nil, err
	}

	store := &SQLiteFeedRepository{db: dbConn}
	if err := store.migrate(context.Background()); err != nil {
		_ = dbConn.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteFeedRepository) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteFeedRepository) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS folders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			parent_id INTEGER,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(parent_id) REFERENCES folders(id) ON DELETE SET NULL
		);`,
		`CREATE TABLE IF NOT EXISTS feeds (
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
		);`,
		`CREATE TABLE IF NOT EXISTS entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			feed_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			link TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL DEFAULT '',
			full_content TEXT NOT NULL DEFAULT '',
			cover_url TEXT NOT NULL DEFAULT '',
			published_at TEXT NOT NULL DEFAULT '',
			is_read INTEGER NOT NULL DEFAULT 0,
			is_favorite INTEGER NOT NULL DEFAULT 0,
			favorited_at TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(feed_id) REFERENCES feeds(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_feeds_folder_id ON feeds(folder_id);`,
		`CREATE INDEX IF NOT EXISTS idx_entries_feed_id ON entries(feed_id);`,
		`CREATE INDEX IF NOT EXISTS idx_entries_created_at ON entries(created_at);`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL
		);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "entries", "full_content", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "entries", "cover_url", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "entries", "is_favorite", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "entries", "favorited_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "feeds", "custom_script", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "feeds", "custom_script_lang", "TEXT NOT NULL DEFAULT 'shell'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "feeds", "icon_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "feeds", "icon_fetched_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}

	return nil
}

func (s *SQLiteFeedRepository) ensureColumn(ctx context.Context, table, column, ddl string) error {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if strings.EqualFold(name, column) {
			return nil
		}
	}
	_, err = s.db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, ddl))
	return err
}

func (s *SQLiteFeedRepository) List() []model.Feed {
	rows, err := s.db.Query(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds ORDER BY id DESC`)
	if err != nil {
		return []model.Feed{}
	}
	defer rows.Close()

	feeds := make([]model.Feed, 0)
	for rows.Next() {
		var feed model.Feed
		var folderID sql.NullInt64
		if err := rows.Scan(
			&feed.ID,
			&feed.URL,
			&feed.Title,
			&folderID,
			&feed.CustomScript,
			&feed.CustomScriptLang,
			&feed.IconPath,
			&feed.IconFetchedAt,
			&feed.ItemCount,
			&feed.LastFetchedAt,
			&feed.LastFetchStatus,
			&feed.LastFetchError,
			&feed.ETag,
			&feed.LastModified,
			&feed.CreatedAt,
		); err != nil {
			continue
		}
		if folderID.Valid {
			id := folderID.Int64
			feed.FolderID = &id
		}
		if feed.IconPath != "" {
			feed.IconURL = fmt.Sprintf("/api/v1/icons/%d", feed.ID)
		}
		feeds = append(feeds, feed)
	}
	return feeds
}

func (s *SQLiteFeedRepository) ListFolders() []model.Folder {
	rows, err := s.db.Query(`SELECT id, name, parent_id, created_at, updated_at FROM folders ORDER BY id ASC`)
	if err != nil {
		return []model.Folder{}
	}
	defer rows.Close()

	folders := make([]model.Folder, 0)
	for rows.Next() {
		var folder model.Folder
		var parentID sql.NullInt64
		if err := rows.Scan(&folder.ID, &folder.Name, &parentID, &folder.CreatedAt, &folder.UpdatedAt); err != nil {
			continue
		}
		if parentID.Valid {
			id := parentID.Int64
			folder.ParentID = &id
		}
		folders = append(folders, folder)
	}
	return folders
}

func (s *SQLiteFeedRepository) CreateFolder(name string, parentID *int64) (model.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Folder{}, ErrFolderNameEmpty
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`INSERT INTO folders(name, parent_id, created_at, updated_at) VALUES(?, ?, ?, ?)`, name, nullableInt(parentID), now, now)
	if err != nil {
		return model.Folder{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Folder{}, err
	}
	return model.Folder{ID: id, Name: name, ParentID: parentID, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *SQLiteFeedRepository) UpdateFolder(id int64, name string, parentID *int64) (model.Folder, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Folder{}, false, ErrFolderNameEmpty
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE folders SET name = ?, parent_id = ?, updated_at = ? WHERE id = ?`, name, nullableInt(parentID), now, id)
	if err != nil {
		return model.Folder{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return model.Folder{}, false, nil
	}
	return model.Folder{ID: id, Name: name, ParentID: parentID, UpdatedAt: now}, true, nil
}

func (s *SQLiteFeedRepository) DeleteFolder(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM folders WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *SQLiteFeedRepository) Add(url, title string, items []ArticleSeed, fetchErr string) (model.Feed, error) {
	return s.AddInFolder(url, title, items, fetchErr, nil, "", "")
}

func (s *SQLiteFeedRepository) AddInFolder(url, title string, items []ArticleSeed, fetchErr string, folderID *int64, etag string, lastModified string) (model.Feed, error) {
	url = strings.TrimSpace(url)
	title = strings.TrimSpace(title)
	if title == "" {
		title = url
	}

	exists, err := s.feedExists(url)
	if err != nil {
		return model.Feed{}, err
	}
	if exists {
		return model.Feed{}, ErrFeedExists
	}

	now := time.Now().UTC().Format(time.RFC3339)
	status := "ok"
	if fetchErr != "" {
		status = "error"
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return model.Feed{}, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO feeds(url, title, folder_id, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at, updated_at)
		 VALUES(?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?)`,
		url, title, nullableInt(folderID), now, status, fetchErr, etag, lastModified, now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.Feed{}, ErrFeedExists
		}
		return model.Feed{}, err
	}
	feedID, err := res.LastInsertId()
	if err != nil {
		return model.Feed{}, err
	}

	insertedCount, err := s.insertEntriesTx(tx, feedID, items, now)
	if err != nil {
		return model.Feed{}, err
	}
	if _, err := tx.Exec(`UPDATE feeds SET item_count = ? WHERE id = ?`, insertedCount, feedID); err != nil {
		return model.Feed{}, err
	}

	if err := tx.Commit(); err != nil {
		return model.Feed{}, err
	}

	return model.Feed{
		ID:               feedID,
		URL:              url,
		Title:            title,
		FolderID:         folderID,
		CustomScript:     "",
		CustomScriptLang: "shell",
		IconPath:         "",
		IconFetchedAt:    "",
		ItemCount:        insertedCount,
		LastFetchedAt:    now,
		LastFetchStatus:  status,
		LastFetchError:   fetchErr,
		ETag:             etag,
		LastModified:     lastModified,
		CreatedAt:        now,
	}, nil
}

func (s *SQLiteFeedRepository) UpdateFeedFolder(id int64, folderID *int64) (model.Feed, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET folder_id = ?, updated_at = ? WHERE id = ?`, nullableInt(folderID), now, id)
	if err != nil {
		return model.Feed{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return model.Feed{}, false, nil
	}
	feed, ok, err := s.GetFeed(id)
	if err != nil {
		return model.Feed{}, false, err
	}
	return feed, ok, nil
}

func (s *SQLiteFeedRepository) DeleteFeed(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM feeds WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *SQLiteFeedRepository) GetFeed(id int64) (model.Feed, bool, error) {
	row := s.db.QueryRow(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds WHERE id = ?`, id)
	var feed model.Feed
	var folderID sql.NullInt64
	if err := row.Scan(
		&feed.ID,
		&feed.URL,
		&feed.Title,
		&folderID,
		&feed.CustomScript,
		&feed.CustomScriptLang,
		&feed.IconPath,
		&feed.IconFetchedAt,
		&feed.ItemCount,
		&feed.LastFetchedAt,
		&feed.LastFetchStatus,
		&feed.LastFetchError,
		&feed.ETag,
		&feed.LastModified,
		&feed.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Feed{}, false, nil
		}
		return model.Feed{}, false, err
	}
	if folderID.Valid {
		id := folderID.Int64
		feed.FolderID = &id
	}
	if feed.IconPath != "" {
		feed.IconURL = fmt.Sprintf("/api/v1/icons/%d", feed.ID)
	}
	return feed, true, nil
}

func (s *SQLiteFeedRepository) GetFeedByURL(rawURL string) (model.Feed, bool, error) {
	row := s.db.QueryRow(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds WHERE url = ?`, strings.TrimSpace(rawURL))
	var feed model.Feed
	var folderID sql.NullInt64
	if err := row.Scan(
		&feed.ID,
		&feed.URL,
		&feed.Title,
		&folderID,
		&feed.CustomScript,
		&feed.CustomScriptLang,
		&feed.IconPath,
		&feed.IconFetchedAt,
		&feed.ItemCount,
		&feed.LastFetchedAt,
		&feed.LastFetchStatus,
		&feed.LastFetchError,
		&feed.ETag,
		&feed.LastModified,
		&feed.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Feed{}, false, nil
		}
		return model.Feed{}, false, err
	}
	if folderID.Valid {
		id := folderID.Int64
		feed.FolderID = &id
	}
	if feed.IconPath != "" {
		feed.IconURL = fmt.Sprintf("/api/v1/icons/%d", feed.ID)
	}
	return feed, true, nil
}

func (s *SQLiteFeedRepository) CreateFeedPlaceholder(url string, title string, folderID *int64) (model.Feed, error) {
	url = strings.TrimSpace(url)
	title = strings.TrimSpace(title)
	if url == "" {
		return model.Feed{}, errors.New("url is required")
	}
	if title == "" {
		title = url
	}
	exists, err := s.feedExists(url)
	if err != nil {
		return model.Feed{}, err
	}
	if exists {
		return model.Feed{}, ErrFeedExists
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO feeds(url, title, folder_id, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at, updated_at)
		 VALUES(?, ?, ?, 0, ?, 'idle', '', '', '', ?, ?)`,
		url, title, nullableInt(folderID), now, now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return model.Feed{}, ErrFeedExists
		}
		return model.Feed{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return model.Feed{}, err
	}
	return model.Feed{
		ID:               id,
		URL:              url,
		Title:            title,
		FolderID:         folderID,
		CustomScript:     "",
		CustomScriptLang: "shell",
		IconPath:         "",
		IconFetchedAt:    "",
		ItemCount:        0,
		LastFetchedAt:    now,
		LastFetchStatus:  "idle",
		LastFetchError:   "",
		ETag:             "",
		LastModified:     "",
		CreatedAt:        now,
	}, nil
}

func (s *SQLiteFeedRepository) UpdateFeedAfterRefresh(feedID int64, title string, items []ArticleSeed, fetchErr, etag, lastModified string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	status := "ok"
	if fetchErr != "" {
		status = "error"
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	inserted := 0
	if fetchErr == "" {
		inserted, err = s.insertEntriesTx(tx, feedID, items, now)
		if err != nil {
			return err
		}
	}

	if title == "" {
		if err := tx.QueryRow(`SELECT title FROM feeds WHERE id = ?`, feedID).Scan(&title); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(
		`UPDATE feeds SET title = ?, item_count = item_count + ?, last_fetched_at = ?, last_fetch_status = ?, last_fetch_error = ?, etag = ?, last_modified = ?, updated_at = ? WHERE id = ?`,
		title, inserted, now, status, fetchErr, etag, lastModified, now, feedID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *SQLiteFeedRepository) ListArticles() []model.Article {
	rows, err := s.db.Query(`SELECT id, feed_id, title, link, summary, full_content, cover_url, published_at, is_read, is_favorite, favorited_at, created_at FROM entries ORDER BY id DESC`)
	if err != nil {
		return []model.Article{}
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	for rows.Next() {
		var article model.Article
		var readFlag int
		var favoriteFlag int
		if err := rows.Scan(&article.ID, &article.FeedID, &article.Title, &article.Link, &article.Summary, &article.FullContent, &article.CoverURL, &article.PublishedAt, &readFlag, &favoriteFlag, &article.FavoritedAt, &article.CreatedAt); err != nil {
			continue
		}
		article.IsRead = readFlag == 1
		article.IsFavorite = favoriteFlag == 1
		articles = append(articles, article)
	}
	return articles
}

func (s *SQLiteFeedRepository) DeleteArticle(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM entries WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *SQLiteFeedRepository) GetArticle(id int64) (model.Article, bool) {
	row := s.db.QueryRow(`SELECT id, feed_id, title, link, summary, full_content, cover_url, published_at, is_read, is_favorite, favorited_at, created_at FROM entries WHERE id = ?`, id)
	var article model.Article
	var readFlag int
	var favoriteFlag int
	if err := row.Scan(&article.ID, &article.FeedID, &article.Title, &article.Link, &article.Summary, &article.FullContent, &article.CoverURL, &article.PublishedAt, &readFlag, &favoriteFlag, &article.FavoritedAt, &article.CreatedAt); err != nil {
		return model.Article{}, false
	}
	article.IsRead = readFlag == 1
	article.IsFavorite = favoriteFlag == 1
	return article, true
}

func (s *SQLiteFeedRepository) UpdateArticleFullContent(id int64, content string) error {
	_, err := s.db.Exec(`UPDATE entries SET full_content = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(content), time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *SQLiteFeedRepository) MarkArticleRead(id int64, read bool) (model.Article, bool, error) {
	flag := 0
	if read {
		flag = 1
	}

	res, err := s.db.Exec(`UPDATE entries SET is_read = ?, updated_at = ? WHERE id = ?`, flag, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return model.Article{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return model.Article{}, false, nil
	}

	article, ok := s.GetArticle(id)
	return article, ok, nil
}

func (s *SQLiteFeedRepository) MarkArticleFavorite(id int64, favorite bool) (model.Article, bool, error) {
	flag := 0
	favoritedAt := ""
	if favorite {
		flag = 1
		favoritedAt = time.Now().UTC().Format(time.RFC3339)
	}

	res, err := s.db.Exec(`UPDATE entries SET is_favorite = ?, favorited_at = ?, updated_at = ? WHERE id = ?`, flag, favoritedAt, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return model.Article{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return model.Article{}, false, nil
	}

	article, ok := s.GetArticle(id)
	return article, ok, nil
}

func parseArticleTimestamp(publishedAt string, createdAt string) (time.Time, bool) {
	publishedAt = strings.TrimSpace(publishedAt)
	if publishedAt != "" {
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			time.RFC1123Z,
			time.RFC1123,
			time.RFC822Z,
			time.RFC822,
			time.RFC850,
		}
		for _, layout := range formats {
			if ts, err := time.Parse(layout, publishedAt); err == nil {
				return ts.UTC(), true
			}
		}
	}
	createdAt = strings.TrimSpace(createdAt)
	if createdAt != "" {
		if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
			return ts.UTC(), true
		}
	}
	return time.Time{}, false
}

func (s *SQLiteFeedRepository) PurgeExpiredArticles(retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	rows, err := s.db.Query(`SELECT id, published_at, created_at FROM entries WHERE is_favorite = 0`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var expiredIDs []int64
	for rows.Next() {
		var id int64
		var publishedAt string
		var createdAt string
		if err := rows.Scan(&id, &publishedAt, &createdAt); err != nil {
			continue
		}
		ts, ok := parseArticleTimestamp(publishedAt, createdAt)
		if !ok {
			continue
		}
		if ts.Before(cutoff) {
			expiredIDs = append(expiredIDs, id)
		}
	}
	if len(expiredIDs) == 0 {
		return 0, nil
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`DELETE FROM entries WHERE id = ?`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	deleted := 0
	for _, id := range expiredIDs {
		res, err := stmt.Exec(id)
		if err != nil {
			return deleted, err
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			deleted += int(affected)
		}
	}
	if err := tx.Commit(); err != nil {
		return deleted, err
	}
	return deleted, nil
}

func (s *SQLiteFeedRepository) feedExists(url string) (bool, error) {
	row := s.db.QueryRow(`SELECT 1 FROM feeds WHERE url = ? LIMIT 1`, url)
	var one int
	err := row.Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *SQLiteFeedRepository) insertEntriesTx(tx *sql.Tx, feedID int64, items []ArticleSeed, now string) (int, error) {
	existingKeys, err := s.loadDedupKeysTx(tx)
	if err != nil {
		return 0, err
	}
	insertedCount := 0

	for _, item := range items {
		cleaned := cleanSeed(item)
		if cleaned.Title == "" && cleaned.Link == "" {
			continue
		}

		key := dedupKey(cleaned)
		if _, exists := existingKeys[key]; exists {
			continue
		}
		existingKeys[key] = struct{}{}

		if _, err := tx.Exec(
			`INSERT INTO entries(feed_id, title, link, summary, full_content, cover_url, published_at, is_read, is_favorite, favorited_at, created_at, updated_at)
			 VALUES(?, ?, ?, ?, ?, ?, ?, 0, 0, '', ?, ?)`,
			feedID, cleaned.Title, cleaned.Link, cleaned.Summary, cleaned.FullContent, cleaned.CoverURL, cleaned.PublishedAt, now, now,
		); err != nil {
			return insertedCount, err
		}
		insertedCount++
	}

	return insertedCount, nil
}

func (s *SQLiteFeedRepository) loadDedupKeysTx(tx *sql.Tx) (map[string]struct{}, error) {
	rows, err := tx.Query(`SELECT title, link, summary FROM entries`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make(map[string]struct{})
	for rows.Next() {
		var title, link, summary string
		if err := rows.Scan(&title, &link, &summary); err != nil {
			continue
		}
		keys[dedupKey(ArticleSeed{Title: title, Link: link, Summary: summary})] = struct{}{}
	}
	return keys, nil
}

func cleanSeed(seed ArticleSeed) ArticleSeed {
	seed.Title = strings.TrimSpace(seed.Title)
	seed.Link = strings.TrimSpace(seed.Link)
	seed.Summary = strings.TrimSpace(seed.Summary)
	seed.FullContent = strings.TrimSpace(seed.FullContent)
	seed.CoverURL = strings.TrimSpace(seed.CoverURL)
	seed.PublishedAt = strings.TrimSpace(seed.PublishedAt)
	return seed
}

func (s *SQLiteFeedRepository) UpdateFeedScript(id int64, script string, lang string) (model.Feed, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET custom_script = ?, custom_script_lang = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(script), strings.TrimSpace(lang), now, id)
	if err != nil {
		return model.Feed{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return model.Feed{}, false, nil
	}
	feed, ok, err := s.GetFeed(id)
	if err != nil {
		return model.Feed{}, false, err
	}
	return feed, ok, nil
}

func (s *SQLiteFeedRepository) UpdateFeedTitle(id int64, title string) (model.Feed, bool, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return model.Feed{}, false, errors.New("title is required")
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET title = ?, updated_at = ? WHERE id = ?`, title, now, id)
	if err != nil {
		return model.Feed{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return model.Feed{}, false, nil
	}
	feed, ok, err := s.GetFeed(id)
	if err != nil {
		return model.Feed{}, false, err
	}
	return feed, ok, nil
}

func (s *SQLiteFeedRepository) UpdateFeedIcon(id int64, iconPath string) (model.Feed, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET icon_path = ?, icon_fetched_at = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(iconPath), now, now, id)
	if err != nil {
		return model.Feed{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return model.Feed{}, false, nil
	}
	feed, ok, err := s.GetFeed(id)
	if err != nil {
		return model.Feed{}, false, err
	}
	return feed, ok, nil
}

func dedupKey(seed ArticleSeed) string {
	normalizedLink := normalizeForKey(seed.Link)
	if normalizedLink != "" {
		return hashText("link:" + normalizedLink)
	}

	normalizedTitle := normalizeForKey(seed.Title)
	normalizedSummary := normalizeForKey(seed.Summary)
	return hashText("text:" + normalizedTitle + "|" + normalizedSummary)
}

func normalizeForKey(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	return strings.Join(strings.Fields(v), " ")
}

func hashText(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func nullableInt(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func (s *SQLiteFeedRepository) GetSetting(key string) (string, bool, error) {
	row := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, strings.TrimSpace(key))
	var value string
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}

func (s *SQLiteFeedRepository) SetSetting(key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO app_settings(key, value, updated_at) VALUES(?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		strings.TrimSpace(key), strings.TrimSpace(value), now,
	)
	return err
}

func isUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint")
}
