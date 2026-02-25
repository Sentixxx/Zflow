package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Sentixxx/Zflow/backend/internal/domain"
)

var ErrFeedExists = errors.New("feed already exists")
var ErrFolderNameEmpty = errors.New("folder name is required")

type ArticleSeed struct {
	Title       string
	Link        string
	Summary     string
	FullContent string
	PublishedAt string
}

type FeedStore struct {
	db *sql.DB
}

func NewFeedStore(path string) (*FeedStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(30000)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	store := &FeedStore{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *FeedStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *FeedStore) migrate(ctx context.Context) error {
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
			published_at TEXT NOT NULL DEFAULT '',
			is_read INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(feed_id) REFERENCES feeds(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_feeds_folder_id ON feeds(folder_id);`,
		`CREATE INDEX IF NOT EXISTS idx_entries_feed_id ON entries(feed_id);`,
		`CREATE INDEX IF NOT EXISTS idx_entries_created_at ON entries(created_at);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "entries", "full_content", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "feeds", "custom_script", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "feeds", "custom_script_lang", "TEXT NOT NULL DEFAULT 'shell'"); err != nil {
		return err
	}

	return nil
}

func (s *FeedStore) ensureColumn(ctx context.Context, table, column, ddl string) error {
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

func (s *FeedStore) List() []domain.Feed {
	rows, err := s.db.Query(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds ORDER BY id DESC`)
	if err != nil {
		return []domain.Feed{}
	}
	defer rows.Close()

	feeds := make([]domain.Feed, 0)
	for rows.Next() {
		var feed domain.Feed
		var folderID sql.NullInt64
		if err := rows.Scan(
			&feed.ID,
			&feed.URL,
			&feed.Title,
			&folderID,
			&feed.CustomScript,
			&feed.CustomScriptLang,
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
		feeds = append(feeds, feed)
	}
	return feeds
}

func (s *FeedStore) ListFolders() []domain.Folder {
	rows, err := s.db.Query(`SELECT id, name, parent_id, created_at, updated_at FROM folders ORDER BY id ASC`)
	if err != nil {
		return []domain.Folder{}
	}
	defer rows.Close()

	folders := make([]domain.Folder, 0)
	for rows.Next() {
		var folder domain.Folder
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

func (s *FeedStore) CreateFolder(name string, parentID *int64) (domain.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Folder{}, ErrFolderNameEmpty
	}

	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`INSERT INTO folders(name, parent_id, created_at, updated_at) VALUES(?, ?, ?, ?)`, name, nullableInt(parentID), now, now)
	if err != nil {
		return domain.Folder{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Folder{}, err
	}
	return domain.Folder{ID: id, Name: name, ParentID: parentID, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *FeedStore) UpdateFolder(id int64, name string, parentID *int64) (domain.Folder, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Folder{}, false, ErrFolderNameEmpty
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE folders SET name = ?, parent_id = ?, updated_at = ? WHERE id = ?`, name, nullableInt(parentID), now, id)
	if err != nil {
		return domain.Folder{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.Folder{}, false, nil
	}
	return domain.Folder{ID: id, Name: name, ParentID: parentID, UpdatedAt: now}, true, nil
}

func (s *FeedStore) DeleteFolder(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM folders WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *FeedStore) Add(url, title string, items []ArticleSeed, fetchErr string) (domain.Feed, error) {
	return s.AddInFolder(url, title, items, fetchErr, nil, "", "")
}

func (s *FeedStore) AddInFolder(url, title string, items []ArticleSeed, fetchErr string, folderID *int64, etag string, lastModified string) (domain.Feed, error) {
	url = strings.TrimSpace(url)
	title = strings.TrimSpace(title)
	if title == "" {
		title = url
	}

	exists, err := s.feedExists(url)
	if err != nil {
		return domain.Feed{}, err
	}
	if exists {
		return domain.Feed{}, ErrFeedExists
	}

	now := time.Now().UTC().Format(time.RFC3339)
	status := "ok"
	if fetchErr != "" {
		status = "error"
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return domain.Feed{}, err
	}
	defer tx.Rollback()

	res, err := tx.Exec(
		`INSERT INTO feeds(url, title, folder_id, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at, updated_at)
		 VALUES(?, ?, ?, 0, ?, ?, ?, ?, ?, ?, ?)`,
		url, title, nullableInt(folderID), now, status, fetchErr, etag, lastModified, now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Feed{}, ErrFeedExists
		}
		return domain.Feed{}, err
	}
	feedID, err := res.LastInsertId()
	if err != nil {
		return domain.Feed{}, err
	}

	insertedCount, err := s.insertEntriesTx(tx, feedID, items, now)
	if err != nil {
		return domain.Feed{}, err
	}
	if _, err := tx.Exec(`UPDATE feeds SET item_count = ? WHERE id = ?`, insertedCount, feedID); err != nil {
		return domain.Feed{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.Feed{}, err
	}

	return domain.Feed{
		ID:              feedID,
		URL:             url,
		Title:           title,
		FolderID:        folderID,
		CustomScript:    "",
		CustomScriptLang: "shell",
		ItemCount:       insertedCount,
		LastFetchedAt:   now,
		LastFetchStatus: status,
		LastFetchError:  fetchErr,
		ETag:            etag,
		LastModified:    lastModified,
		CreatedAt:       now,
	}, nil
}

func (s *FeedStore) UpdateFeedFolder(id int64, folderID *int64) (domain.Feed, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET folder_id = ?, updated_at = ? WHERE id = ?`, nullableInt(folderID), now, id)
	if err != nil {
		return domain.Feed{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.Feed{}, false, nil
	}
	feed, ok, err := s.GetFeed(id)
	if err != nil {
		return domain.Feed{}, false, err
	}
	return feed, ok, nil
}

func (s *FeedStore) DeleteFeed(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM feeds WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *FeedStore) GetFeed(id int64) (domain.Feed, bool, error) {
	row := s.db.QueryRow(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds WHERE id = ?`, id)
	var feed domain.Feed
	var folderID sql.NullInt64
	if err := row.Scan(
		&feed.ID,
		&feed.URL,
		&feed.Title,
		&folderID,
		&feed.CustomScript,
		&feed.CustomScriptLang,
		&feed.ItemCount,
		&feed.LastFetchedAt,
		&feed.LastFetchStatus,
		&feed.LastFetchError,
		&feed.ETag,
		&feed.LastModified,
		&feed.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Feed{}, false, nil
		}
		return domain.Feed{}, false, err
	}
	if folderID.Valid {
		id := folderID.Int64
		feed.FolderID = &id
	}
	return feed, true, nil
}

func (s *FeedStore) UpdateFeedAfterRefresh(feedID int64, title string, items []ArticleSeed, fetchErr, etag, lastModified string) error {
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

func (s *FeedStore) ListArticles() []domain.Article {
	rows, err := s.db.Query(`SELECT id, feed_id, title, link, summary, full_content, published_at, is_read, created_at FROM entries ORDER BY id DESC`)
	if err != nil {
		return []domain.Article{}
	}
	defer rows.Close()

	articles := make([]domain.Article, 0)
	for rows.Next() {
		var article domain.Article
		var readFlag int
		if err := rows.Scan(&article.ID, &article.FeedID, &article.Title, &article.Link, &article.Summary, &article.FullContent, &article.PublishedAt, &readFlag, &article.CreatedAt); err != nil {
			continue
		}
		article.IsRead = readFlag == 1
		articles = append(articles, article)
	}
	return articles
}

func (s *FeedStore) DeleteArticle(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM entries WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *FeedStore) GetArticle(id int64) (domain.Article, bool) {
	row := s.db.QueryRow(`SELECT id, feed_id, title, link, summary, full_content, published_at, is_read, created_at FROM entries WHERE id = ?`, id)
	var article domain.Article
	var readFlag int
	if err := row.Scan(&article.ID, &article.FeedID, &article.Title, &article.Link, &article.Summary, &article.FullContent, &article.PublishedAt, &readFlag, &article.CreatedAt); err != nil {
		return domain.Article{}, false
	}
	article.IsRead = readFlag == 1
	return article, true
}

func (s *FeedStore) UpdateArticleFullContent(id int64, content string) error {
	_, err := s.db.Exec(`UPDATE entries SET full_content = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(content), time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *FeedStore) MarkArticleRead(id int64, read bool) (domain.Article, bool, error) {
	flag := 0
	if read {
		flag = 1
	}

	res, err := s.db.Exec(`UPDATE entries SET is_read = ?, updated_at = ? WHERE id = ?`, flag, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return domain.Article{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.Article{}, false, nil
	}

	article, ok := s.GetArticle(id)
	return article, ok, nil
}

func (s *FeedStore) feedExists(url string) (bool, error) {
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

func (s *FeedStore) insertEntriesTx(tx *sql.Tx, feedID int64, items []ArticleSeed, now string) (int, error) {
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
			`INSERT INTO entries(feed_id, title, link, summary, full_content, published_at, is_read, created_at, updated_at)
			 VALUES(?, ?, ?, ?, ?, ?, 0, ?, ?)`,
			feedID, cleaned.Title, cleaned.Link, cleaned.Summary, cleaned.FullContent, cleaned.PublishedAt, now, now,
		); err != nil {
			return insertedCount, err
		}
		insertedCount++
	}

	return insertedCount, nil
}

func (s *FeedStore) loadDedupKeysTx(tx *sql.Tx) (map[string]struct{}, error) {
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
	seed.PublishedAt = strings.TrimSpace(seed.PublishedAt)
	return seed
}

func (s *FeedStore) UpdateFeedScript(id int64, script string, lang string) (domain.Feed, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET custom_script = ?, custom_script_lang = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(script), strings.TrimSpace(lang), now, id)
	if err != nil {
		return domain.Feed{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.Feed{}, false, nil
	}
	feed, ok, err := s.GetFeed(id)
	if err != nil {
		return domain.Feed{}, false, err
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

func isUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "constraint")
}
