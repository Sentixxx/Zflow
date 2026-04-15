package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/model"
)

type SQLiteFeedRepository struct {
	db *sql.DB
}

func NewSQLiteFeedRepository(db *sql.DB) *SQLiteFeedRepository {
	return &SQLiteFeedRepository{db: db}
}

func (s *SQLiteFeedRepository) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func scanArticleListRow(scanner interface{ Scan(dest ...any) error }) (model.Article, error) {
	var article model.Article
	var readFlag int
	var favoriteFlag int
	var scores model.RecommendationScores
	if err := scanner.Scan(
		&article.ID,
		&article.FeedID,
		&article.Title,
		&article.Link,
		&article.CoverURL,
		&article.PublishedAt,
		&readFlag,
		&favoriteFlag,
		&article.FavoritedAt,
		&scores.Quality,
		&scores.Relevance,
		&scores.Novelty,
		&scores.Composite,
		&article.CreatedAt,
	); err != nil {
		return model.Article{}, err
	}
	article.IsRead = readFlag == 1
	article.IsFavorite = favoriteFlag == 1
	article.RecommendationScores = &scores
	return article, nil
}

func buildArticleListQuerySQL(query ArticleListQuery) (string, []any) {
	whereParts := make([]string, 0, 1)
	args := make([]any, 0, len(query.FeedIDs)+2)

	if query.Scoped && len(query.FeedIDs) == 0 {
		whereParts = append(whereParts, "1 = 0")
	}
	if len(query.FeedIDs) > 0 {
		placeholders := make([]string, 0, len(query.FeedIDs))
		for _, id := range query.FeedIDs {
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		whereParts = append(whereParts, fmt.Sprintf("e.feed_id IN (%s)", strings.Join(placeholders, ",")))
	}

	queryText := `
		SELECT
			e.id, e.feed_id, e.title, e.link, e.cover_url, e.published_at,
			e.is_read, e.is_favorite, e.favorited_at,
			e.quality_score, e.relevance_score, e.novelty_score, e.composite_score,
			e.created_at
		FROM entries e`
	if len(whereParts) > 0 {
		queryText += "\nWHERE " + strings.Join(whereParts, " AND ")
	}

	orderExpr := "COALESCE(NULLIF(e.published_at, ''), e.created_at)"
	switch query.Sort {
	case "oldest":
		queryText += "\nORDER BY " + orderExpr + " ASC, e.id ASC"
	case "recommend":
		queryText += "\nORDER BY e.composite_score DESC, " + orderExpr + " DESC, e.id DESC"
	case "quality":
		queryText += "\nORDER BY e.quality_score DESC, " + orderExpr + " DESC, e.id DESC"
	case "relevance":
		queryText += "\nORDER BY e.relevance_score DESC, " + orderExpr + " DESC, e.id DESC"
	case "novelty":
		queryText += "\nORDER BY e.novelty_score DESC, " + orderExpr + " DESC, e.id DESC"
	default:
		queryText += "\nORDER BY " + orderExpr + " DESC, e.id DESC"
	}

	if query.Limit > 0 {
		queryText += " LIMIT ?"
		args = append(args, query.Limit+1)
		if query.Page > 1 {
			queryText += " OFFSET ?"
			args = append(args, (query.Page-1)*query.Limit)
		}
	}

	return queryText, args
}

func countFeedEntriesTx(tx *sql.Tx, feedID int64) (int, error) {
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM entries WHERE feed_id = ?`, feedID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *SQLiteFeedRepository) List() []model.Feed {
	rows, err := s.db.Query(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count, retention_days, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds ORDER BY id DESC`)
	if err != nil {
		return []model.Feed{}
	}
	defer rows.Close()

	feeds := make([]model.Feed, 0)
	for rows.Next() {
		var feed model.Feed
		var folderID sql.NullInt64
		if err := rows.Scan(
			&feed.ID, &feed.URL, &feed.Title, &folderID,
			&feed.CustomScript, &feed.CustomScriptLang, &feed.IconPath, &feed.IconFetchedAt,
			&feed.ItemCount, &feed.RetentionDays, &feed.LastFetchedAt, &feed.LastFetchStatus, &feed.LastFetchError,
			&feed.ETag, &feed.LastModified, &feed.CreatedAt,
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
	res, err := s.db.Exec(
		`INSERT INTO folders(name, parent_id, created_at, updated_at) VALUES(?, ?, ?, ?)`,
		name, nullableInt(parentID), now, now,
	)
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
		`INSERT INTO feeds(url, title, folder_id, item_count, retention_days, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at, updated_at)
		 VALUES(?, ?, ?, 0, 0, ?, ?, ?, ?, ?, ?, ?)`,
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
	currentCount, err := countFeedEntriesTx(tx, feedID)
	if err != nil {
		return model.Feed{}, err
	}
	if _, err := tx.Exec(`UPDATE feeds SET item_count = ? WHERE id = ?`, currentCount, feedID); err != nil {
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
		RetentionDays:    0,
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

func (s *SQLiteFeedRepository) UpdateFeedRetentionDays(id int64, retentionDays int) (model.Feed, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET retention_days = ?, updated_at = ? WHERE id = ?`, retentionDays, now, id)
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
	row := s.db.QueryRow(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count, retention_days, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds WHERE id = ?`, id)
	var feed model.Feed
	var folderID sql.NullInt64
	if err := row.Scan(
		&feed.ID, &feed.URL, &feed.Title, &folderID,
		&feed.CustomScript, &feed.CustomScriptLang, &feed.IconPath, &feed.IconFetchedAt,
		&feed.ItemCount, &feed.RetentionDays, &feed.LastFetchedAt, &feed.LastFetchStatus, &feed.LastFetchError,
		&feed.ETag, &feed.LastModified, &feed.CreatedAt,
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
	row := s.db.QueryRow(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count, retention_days, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds WHERE url = ?`, strings.TrimSpace(rawURL))
	var feed model.Feed
	var folderID sql.NullInt64
	if err := row.Scan(
		&feed.ID, &feed.URL, &feed.Title, &folderID,
		&feed.CustomScript, &feed.CustomScriptLang, &feed.IconPath, &feed.IconFetchedAt,
		&feed.ItemCount, &feed.RetentionDays, &feed.LastFetchedAt, &feed.LastFetchStatus, &feed.LastFetchError,
		&feed.ETag, &feed.LastModified, &feed.CreatedAt,
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
		`INSERT INTO feeds(url, title, folder_id, item_count, retention_days, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at, updated_at)
		 VALUES(?, ?, ?, 0, 0, ?, 'idle', '', '', '', ?, ?)`,
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
		RetentionDays:    0,
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

	if fetchErr == "" {
		_, err = s.insertEntriesTx(tx, feedID, items, now)
		if err != nil {
			return err
		}
	}
	currentCount, err := countFeedEntriesTx(tx, feedID)
	if err != nil {
		return err
	}

	if title == "" {
		if err := tx.QueryRow(`SELECT title FROM feeds WHERE id = ?`, feedID).Scan(&title); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(
		`UPDATE feeds SET title = ?, item_count = ?, last_fetched_at = ?, last_fetch_status = ?, last_fetch_error = ?, etag = ?, last_modified = ?, updated_at = ? WHERE id = ?`,
		title, currentCount, now, status, fetchErr, etag, lastModified, now, feedID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func scanFullArticle(scanner interface{ Scan(dest ...any) error }) (model.Article, error) {
	var article model.Article
	var sourcePayload string
	var readFlag int
	var favoriteFlag int
	var scores model.RecommendationScores
	var gateStatus sql.NullString
	var features model.ArticleFeatures
	var featureVersion sql.NullInt64
	var scoredAt sql.NullString
	var reasoning sql.NullString
	if err := scanner.Scan(
		&article.ID, &article.FeedID, &article.Title, &article.Link, &article.Summary, &sourcePayload, &article.AISummary, &article.AISummaryStatus, &article.AISummaryAt,
		&article.DisplaySummary, &article.DisplaySummaryStatus, &article.DisplaySummaryAt, &article.FullContent, &article.CoverURL, &article.PublishedAt,
		&readFlag, &favoriteFlag, &article.FavoritedAt, &scores.Quality, &scores.Relevance, &scores.Novelty, &scores.Composite, &article.CreatedAt,
		&gateStatus, &features.Quality, &features.Relevance, &features.Depth, &features.Freshness, &features.Novelty, &features.Composite, &features.ContentFingerprint, &featureVersion, &scoredAt, &reasoning,
	); err != nil {
		return model.Article{}, err
	}
	article.IsRead = readFlag == 1
	article.IsFavorite = favoriteFlag == 1
	article.SourcePayload = decodeSourcePayload(sourcePayload)
	article.RecommendationScores = &scores
	if gateStatus.Valid {
		features.GateStatus = model.ArticleGateStatus(gateStatus.String)
		features.FeatureVersion = int(featureVersion.Int64)
		if scoredAt.Valid {
			features.ScoredAt = scoredAt.String
		}
		if reasoning.Valid {
			features.Reasoning = reasoning.String
		}
		article.ArticleFeatures = &features
	}
	return article, nil
}

const fullArticleSelectSQL = `
	SELECT
		e.id, e.feed_id, e.title, e.link, e.summary, e.source_payload, e.ai_summary, e.ai_summary_status, e.ai_summary_updated_at,
		e.display_summary, e.display_summary_status, e.display_summary_updated_at, e.full_content, e.cover_url, e.published_at,
		e.is_read, e.is_favorite, e.favorited_at, e.quality_score, e.relevance_score, e.novelty_score, e.composite_score, e.created_at,
		af.gate_status, COALESCE(af.quality_score, 0), COALESCE(af.relevance_score, 0), COALESCE(af.depth_score, 0), COALESCE(af.freshness_score, 0), COALESCE(af.novelty_score, 0), COALESCE(af.composite_score, 0), COALESCE(af.content_fingerprint, ''), COALESCE(af.feature_version, 0), COALESCE(af.scored_at, ''), COALESCE(af.score_reasoning, '')
	FROM entries e
	LEFT JOIN article_features af ON af.article_id = e.id`

func (s *SQLiteFeedRepository) ListArticles() []model.Article {
	rows, err := s.db.Query(fullArticleSelectSQL + ` ORDER BY e.id DESC`)
	if err != nil {
		return []model.Article{}
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	for rows.Next() {
		article, err := scanFullArticle(rows)
		if err != nil {
			continue
		}
		articles = append(articles, article)
	}
	return articles
}

func (s *SQLiteFeedRepository) ListArticleListItems(query ArticleListQuery) ([]model.Article, bool) {
	if query.Page < 1 {
		query.Page = 1
	}
	sqlText, args := buildArticleListQuerySQL(query)
	rows, err := s.db.Query(sqlText, args...)
	if err != nil {
		return []model.Article{}, false
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	for rows.Next() {
		article, err := scanArticleListRow(rows)
		if err != nil {
			continue
		}
		articles = append(articles, article)
	}
	if query.Limit <= 0 {
		return articles, false
	}
	hasMore := len(articles) > query.Limit
	if hasMore {
		articles = articles[:query.Limit]
	}
	return articles, hasMore
}

func (s *SQLiteFeedRepository) ListArticlesNeedingScoreRefresh(featureVersion int, limit int) []model.Article {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		fullArticleSelectSQL+`
		WHERE af.article_id IS NULL OR COALESCE(af.feature_version, 0) < ?
		ORDER BY e.id ASC
		LIMIT ?`,
		featureVersion, limit,
	)
	if err != nil {
		return []model.Article{}
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	for rows.Next() {
		article, err := scanFullArticle(rows)
		if err != nil {
			continue
		}
		articles = append(articles, article)
	}
	return articles
}

func (s *SQLiteFeedRepository) ListArticlesMissingDisplaySummary(feedID int64, limit int) []model.Article {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, feed_id, title, link, summary, source_payload, ai_summary, ai_summary_status, ai_summary_updated_at, display_summary, display_summary_status, display_summary_updated_at, full_content, cover_url, published_at, is_read, is_favorite, favorited_at, created_at
		 FROM entries
		 WHERE feed_id = ? AND display_summary = ''
		 ORDER BY id DESC
		 LIMIT ?`,
		feedID, limit,
	)
	if err != nil {
		return []model.Article{}
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	for rows.Next() {
		var article model.Article
		var sourcePayload string
		var readFlag int
		var favoriteFlag int
		if err := rows.Scan(
			&article.ID, &article.FeedID, &article.Title, &article.Link,
			&article.Summary, &sourcePayload, &article.AISummary, &article.AISummaryStatus, &article.AISummaryAt,
			&article.DisplaySummary, &article.DisplaySummaryStatus, &article.DisplaySummaryAt,
			&article.FullContent, &article.CoverURL, &article.PublishedAt,
			&readFlag, &favoriteFlag, &article.FavoritedAt, &article.CreatedAt,
		); err != nil {
			continue
		}
		article.IsRead = readFlag == 1
		article.IsFavorite = favoriteFlag == 1
		article.SourcePayload = decodeSourcePayload(sourcePayload)
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
	row := s.db.QueryRow(fullArticleSelectSQL+` WHERE e.id = ?`, id)
	article, err := scanFullArticle(row)
	if err != nil {
		return model.Article{}, false
	}
	return article, true
}

func (s *SQLiteFeedRepository) UpdateArticleFullContent(id int64, content string) error {
	_, err := s.db.Exec(`UPDATE entries SET full_content = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(content), time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *SQLiteFeedRepository) UpdateArticleSummaryState(id int64, aiSummary string, aiStatus string, displaySummary string, displayStatus string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE entries
		 SET ai_summary = ?, ai_summary_status = ?, ai_summary_updated_at = ?,
		     display_summary = ?, display_summary_status = ?, display_summary_updated_at = ?, updated_at = ?
		 WHERE id = ?`,
		strings.TrimSpace(aiSummary),
		strings.TrimSpace(aiStatus),
		now,
		strings.TrimSpace(displaySummary),
		strings.TrimSpace(displayStatus),
		now,
		now,
		id,
	)
	return err
}

func (s *SQLiteFeedRepository) UpdateArticleScores(id int64, scores model.RecommendationScores) error {
	_, err := s.db.Exec(
		`UPDATE entries SET quality_score = ?, relevance_score = ?, novelty_score = ?, composite_score = ?, updated_at = ? WHERE id = ?`,
		scores.Quality, scores.Relevance, scores.Novelty, scores.Composite,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (s *SQLiteFeedRepository) UpdateArticleDisplaySummary(id int64, summary string, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE entries SET display_summary = ?, display_summary_status = ?, display_summary_updated_at = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(summary), strings.TrimSpace(status), now, now, id,
	)
	return err
}

func (s *SQLiteFeedRepository) UpdateArticleFeatures(id int64, features model.ArticleFeatures) error {
	now := time.Now().UTC().Format(time.RFC3339)
	scoredAt := strings.TrimSpace(features.ScoredAt)
	if scoredAt == "" {
		scoredAt = now
	}
	_, err := s.db.Exec(
		`INSERT INTO article_features(article_id, gate_status, quality_score, relevance_score, depth_score, freshness_score, novelty_score, composite_score, content_fingerprint, feature_version, scored_at, score_reasoning, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(article_id) DO UPDATE SET
		 gate_status = excluded.gate_status,
		 quality_score = excluded.quality_score,
		 relevance_score = excluded.relevance_score,
		 depth_score = excluded.depth_score,
		 freshness_score = excluded.freshness_score,
		 novelty_score = excluded.novelty_score,
		 composite_score = excluded.composite_score,
		 content_fingerprint = excluded.content_fingerprint,
		 feature_version = excluded.feature_version,
		 scored_at = excluded.scored_at,
		 score_reasoning = excluded.score_reasoning,
		 updated_at = excluded.updated_at`,
		id,
		string(features.GateStatus),
		features.Quality,
		features.Relevance,
		features.Depth,
		features.Freshness,
		features.Novelty,
		features.Composite,
		features.ContentFingerprint,
		features.FeatureVersion,
		scoredAt,
		features.Reasoning,
		now,
	)
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

func (s *SQLiteFeedRepository) PurgeExpiredArticles(retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	rows, err := s.db.Query(`SELECT e.id, e.feed_id, f.retention_days, e.published_at, e.created_at FROM entries e JOIN feeds f ON f.id = e.feed_id WHERE e.is_favorite = 0`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var expiredIDs []int64
	affectedFeedIDs := make(map[int64]struct{})
	for rows.Next() {
		var id int64
		var feedID int64
		var feedRetentionDays int
		var publishedAt string
		var createdAt string
		if err := rows.Scan(&id, &feedID, &feedRetentionDays, &publishedAt, &createdAt); err != nil {
			continue
		}
		ts, ok := parseArticleTimestamp(publishedAt, createdAt)
		if !ok {
			continue
		}
		effectiveRetentionDays := retentionDays
		if feedRetentionDays > 0 {
			effectiveRetentionDays = feedRetentionDays
		}
		effectiveCutoff := time.Now().UTC().Add(-time.Duration(effectiveRetentionDays) * 24 * time.Hour)
		if ts.Before(effectiveCutoff) {
			expiredIDs = append(expiredIDs, id)
			affectedFeedIDs[feedID] = struct{}{}
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

	deleted := 0
	for _, id := range expiredIDs {
		res, err := tx.Exec(`DELETE FROM entries WHERE id = ?`, id)
		if err != nil {
			return deleted, err
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			deleted += int(affected)
		}
	}
	for feedID := range affectedFeedIDs {
		currentCount, err := countFeedEntriesTx(tx, feedID)
		if err != nil {
			return deleted, err
		}
		if _, err := tx.Exec(`UPDATE feeds SET item_count = ?, updated_at = ? WHERE id = ?`, currentCount, time.Now().UTC().Format(time.RFC3339), feedID); err != nil {
			return deleted, err
		}
	}
	if err := tx.Commit(); err != nil {
		return deleted, err
	}
	return deleted, nil
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

// --- internal helpers ---

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
	existingKeys, err := loadDedupKeysTx(tx, feedID)
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

		scores := cleaned.RecommendationScores
		if scores == nil {
			scores = &model.RecommendationScores{}
		}

		res, err := tx.Exec(
			`INSERT INTO entries(feed_id, title, link, summary, source_payload, ai_summary, ai_summary_status, ai_summary_updated_at, display_summary, display_summary_status, display_summary_updated_at, full_content, cover_url, published_at, is_read, is_favorite, favorited_at, quality_score, relevance_score, novelty_score, composite_score, created_at, updated_at)
			 VALUES(?, ?, ?, ?, ?, '', '', '', '', '', '', ?, ?, ?, 0, 0, '', ?, ?, ?, ?, ?, ?)`,
			feedID, cleaned.Title, cleaned.Link, cleaned.Summary, encodeSourcePayload(cleaned.SourcePayload), cleaned.FullContent, cleaned.CoverURL, cleaned.PublishedAt, scores.Quality, scores.Relevance, scores.Novelty, scores.Composite, now, now,
		)
		if err != nil {
			return insertedCount, err
		}
		if cleaned.ArticleFeatures != nil {
			articleID, err := res.LastInsertId()
			if err != nil {
				return insertedCount, err
			}
			if err := upsertArticleFeaturesTx(tx, articleID, *cleaned.ArticleFeatures, now); err != nil {
				return insertedCount, err
			}
		}
		insertedCount++
	}

	return insertedCount, nil
}

func upsertArticleFeaturesTx(tx *sql.Tx, articleID int64, features model.ArticleFeatures, now string) error {
	scoredAt := strings.TrimSpace(features.ScoredAt)
	if scoredAt == "" {
		scoredAt = now
	}
	_, err := tx.Exec(
		`INSERT INTO article_features(article_id, gate_status, quality_score, relevance_score, depth_score, freshness_score, novelty_score, composite_score, content_fingerprint, feature_version, scored_at, score_reasoning, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(article_id) DO UPDATE SET
		 gate_status = excluded.gate_status,
		 quality_score = excluded.quality_score,
		 relevance_score = excluded.relevance_score,
		 depth_score = excluded.depth_score,
		 freshness_score = excluded.freshness_score,
		 novelty_score = excluded.novelty_score,
		 composite_score = excluded.composite_score,
		 content_fingerprint = excluded.content_fingerprint,
		 feature_version = excluded.feature_version,
		 scored_at = excluded.scored_at,
		 score_reasoning = excluded.score_reasoning,
		 updated_at = excluded.updated_at`,
		articleID,
		string(features.GateStatus),
		features.Quality,
		features.Relevance,
		features.Depth,
		features.Freshness,
		features.Novelty,
		features.Composite,
		features.ContentFingerprint,
		features.FeatureVersion,
		scoredAt,
		features.Reasoning,
		now,
	)
	return err
}

func loadDedupKeysTx(tx *sql.Tx, feedID int64) (map[string]struct{}, error) {
	rows, err := tx.Query(`SELECT title, link, summary FROM entries WHERE feed_id = ?`, feedID)
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
	seed.SourcePayload = cleanSourcePayload(seed.SourcePayload)
	return seed
}

func cleanSourcePayload(payload *model.ArticleSourcePayload) *model.ArticleSourcePayload {
	if payload == nil {
		return nil
	}
	cleaned := &model.ArticleSourcePayload{
		FeedType:    strings.TrimSpace(payload.FeedType),
		Title:       strings.TrimSpace(payload.Title),
		Link:        strings.TrimSpace(payload.Link),
		Summary:     strings.TrimSpace(payload.Summary),
		PublishedAt: strings.TrimSpace(payload.PublishedAt),
	}
	for _, field := range payload.Fields {
		key := strings.TrimSpace(field.Key)
		value := strings.TrimSpace(field.Value)
		valueHTML := unwrapSourceCDATA(strings.TrimSpace(field.ValueHTML))
		if key == "" || (value == "" && valueHTML == "") {
			continue
		}
		cleaned.Fields = append(cleaned.Fields, model.ArticleSourceField{
			Key:       key,
			Value:     value,
			ValueHTML: valueHTML,
		})
	}
	if cleaned.FeedType == "" && cleaned.Title == "" && cleaned.Link == "" && cleaned.Summary == "" && cleaned.PublishedAt == "" && len(cleaned.Fields) == 0 {
		return nil
	}
	return cleaned
}

func encodeSourcePayload(payload *model.ArticleSourcePayload) string {
	cleaned := cleanSourcePayload(payload)
	if cleaned == nil {
		return ""
	}
	raw, err := json.Marshal(cleaned)
	if err != nil {
		return ""
	}
	return string(raw)
}

func decodeSourcePayload(raw string) *model.ArticleSourcePayload {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var payload model.ArticleSourcePayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return cleanSourcePayload(&payload)
}

func unwrapSourceCDATA(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "<![CDATA[") && strings.HasSuffix(trimmed, "]]>") {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "<![CDATA["), "]]>"))
	}
	return trimmed
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

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
