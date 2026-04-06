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

	"github.com/Sentixxx/Zflow/backend/internal/model"
)

type PostgresFeedRepository struct {
	db *sql.DB
}

func NewPostgresFeedRepository(db *sql.DB) *PostgresFeedRepository {
	return &PostgresFeedRepository{db: db}
}

func (s *PostgresFeedRepository) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func pgScanArticleListRow(scanner interface{ Scan(dest ...any) error }) (model.Article, error) {
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

func pgBuildArticleListQuerySQL(query ArticleListQuery) (string, []any) {
	whereParts := make([]string, 0, 1)
	args := make([]any, 0, len(query.FeedIDs)+2)
	argIdx := 1

	if query.Scoped && len(query.FeedIDs) == 0 {
		whereParts = append(whereParts, "1 = 0")
	}
	if len(query.FeedIDs) > 0 {
		placeholders := make([]string, 0, len(query.FeedIDs))
		for _, id := range query.FeedIDs {
			placeholders = append(placeholders, fmt.Sprintf("$%d", argIdx))
			args = append(args, id)
			argIdx++
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
		queryText += fmt.Sprintf("\nLIMIT $%d", argIdx)
		args = append(args, query.Limit+1)
		argIdx++
		if query.Page > 1 {
			queryText += fmt.Sprintf(" OFFSET $%d", argIdx)
			args = append(args, (query.Page-1)*query.Limit)
			argIdx++
		}
	}

	return queryText, args
}

func (s *PostgresFeedRepository) List() []model.Feed {
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

func (s *PostgresFeedRepository) ListFolders() []model.Folder {
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

func (s *PostgresFeedRepository) CreateFolder(name string, parentID *int64) (model.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Folder{}, ErrFolderNameEmpty
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var id int64
	err := s.db.QueryRow(
		`INSERT INTO folders(name, parent_id, created_at, updated_at) VALUES($1, $2, $3, $4) RETURNING id`,
		name, pgNullableInt(parentID), now, now,
	).Scan(&id)
	if err != nil {
		return model.Folder{}, err
	}
	return model.Folder{ID: id, Name: name, ParentID: parentID, CreatedAt: now, UpdatedAt: now}, nil
}

func (s *PostgresFeedRepository) UpdateFolder(id int64, name string, parentID *int64) (model.Folder, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Folder{}, false, ErrFolderNameEmpty
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE folders SET name = $1, parent_id = $2, updated_at = $3 WHERE id = $4`, name, pgNullableInt(parentID), now, id)
	if err != nil {
		return model.Folder{}, false, err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return model.Folder{}, false, nil
	}
	return model.Folder{ID: id, Name: name, ParentID: parentID, UpdatedAt: now}, true, nil
}

func (s *PostgresFeedRepository) DeleteFolder(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM folders WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *PostgresFeedRepository) AddInFolder(url, title string, items []ArticleSeed, fetchErr string, folderID *int64, etag string, lastModified string) (model.Feed, error) {
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

	var feedID int64
	err = tx.QueryRow(
		`INSERT INTO feeds(url, title, folder_id, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at, updated_at)
		 VALUES($1, $2, $3, 0, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
		url, title, pgNullableInt(folderID), now, status, fetchErr, etag, lastModified, now, now,
	).Scan(&feedID)
	if err != nil {
		if pgIsUniqueViolation(err) {
			return model.Feed{}, ErrFeedExists
		}
		return model.Feed{}, err
	}

	insertedCount, err := s.insertEntriesTx(tx, feedID, items, now)
	if err != nil {
		return model.Feed{}, err
	}
	if _, err := tx.Exec(`UPDATE feeds SET item_count = $1 WHERE id = $2`, insertedCount, feedID); err != nil {
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

func (s *PostgresFeedRepository) UpdateFeedFolder(id int64, folderID *int64) (model.Feed, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET folder_id = $1, updated_at = $2 WHERE id = $3`, pgNullableInt(folderID), now, id)
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

func (s *PostgresFeedRepository) DeleteFeed(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM feeds WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *PostgresFeedRepository) GetFeed(id int64) (model.Feed, bool, error) {
	row := s.db.QueryRow(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds WHERE id = $1`, id)
	var feed model.Feed
	var folderID sql.NullInt64
	if err := row.Scan(
		&feed.ID, &feed.URL, &feed.Title, &folderID,
		&feed.CustomScript, &feed.CustomScriptLang, &feed.IconPath, &feed.IconFetchedAt,
		&feed.ItemCount, &feed.LastFetchedAt, &feed.LastFetchStatus, &feed.LastFetchError,
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

func (s *PostgresFeedRepository) GetFeedByURL(rawURL string) (model.Feed, bool, error) {
	row := s.db.QueryRow(`SELECT id, url, title, folder_id, custom_script, custom_script_lang, icon_path, icon_fetched_at, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at FROM feeds WHERE url = $1`, strings.TrimSpace(rawURL))
	var feed model.Feed
	var folderID sql.NullInt64
	if err := row.Scan(
		&feed.ID, &feed.URL, &feed.Title, &folderID,
		&feed.CustomScript, &feed.CustomScriptLang, &feed.IconPath, &feed.IconFetchedAt,
		&feed.ItemCount, &feed.LastFetchedAt, &feed.LastFetchStatus, &feed.LastFetchError,
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

func (s *PostgresFeedRepository) CreateFeedPlaceholder(url string, title string, folderID *int64) (model.Feed, error) {
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
	var id int64
	err = s.db.QueryRow(
		`INSERT INTO feeds(url, title, folder_id, item_count, last_fetched_at, last_fetch_status, last_fetch_error, etag, last_modified, created_at, updated_at)
		 VALUES($1, $2, $3, 0, $4, 'idle', '', '', '', $5, $6) RETURNING id`,
		url, title, pgNullableInt(folderID), now, now, now,
	).Scan(&id)
	if err != nil {
		if pgIsUniqueViolation(err) {
			return model.Feed{}, ErrFeedExists
		}
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

func (s *PostgresFeedRepository) UpdateFeedAfterRefresh(feedID int64, title string, items []ArticleSeed, fetchErr, etag, lastModified string) error {
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
		if err := tx.QueryRow(`SELECT title FROM feeds WHERE id = $1`, feedID).Scan(&title); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(
		`UPDATE feeds SET title = $1, item_count = item_count + $2, last_fetched_at = $3, last_fetch_status = $4, last_fetch_error = $5, etag = $6, last_modified = $7, updated_at = $8 WHERE id = $9`,
		title, inserted, now, status, fetchErr, etag, lastModified, now, feedID,
	); err != nil {
		return err
	}

	return tx.Commit()
}

func pgScanFullArticle(scanner interface{ Scan(dest ...any) error }) (model.Article, error) {
	var article model.Article
	var readFlag int
	var favoriteFlag int
	var scores model.RecommendationScores
	var gateStatus sql.NullString
	var features model.ArticleFeatures
	var featureVersion sql.NullInt64
	var scoredAt sql.NullString
	if err := scanner.Scan(
		&article.ID, &article.FeedID, &article.Title, &article.Link, &article.Summary, &article.AISummary, &article.AISummaryStatus, &article.AISummaryAt,
		&article.DisplaySummary, &article.DisplaySummaryStatus, &article.DisplaySummaryAt, &article.FullContent, &article.CoverURL, &article.PublishedAt,
		&readFlag, &favoriteFlag, &article.FavoritedAt, &scores.Quality, &scores.Relevance, &scores.Novelty, &scores.Composite, &article.CreatedAt,
		&gateStatus, &features.Quality, &features.Relevance, &features.Depth, &features.Freshness, &features.Novelty, &features.Composite, &features.ContentFingerprint, &featureVersion, &scoredAt,
	); err != nil {
		return model.Article{}, err
	}
	article.IsRead = readFlag == 1
	article.IsFavorite = favoriteFlag == 1
	article.RecommendationScores = &scores
	if gateStatus.Valid {
		features.GateStatus = model.ArticleGateStatus(gateStatus.String)
		features.FeatureVersion = int(featureVersion.Int64)
		if scoredAt.Valid {
			features.ScoredAt = scoredAt.String
		}
		article.ArticleFeatures = &features
	}
	return article, nil
}

const fullArticleSelectSQL = `
	SELECT
		e.id, e.feed_id, e.title, e.link, e.summary, e.ai_summary, e.ai_summary_status, e.ai_summary_updated_at,
		e.display_summary, e.display_summary_status, e.display_summary_updated_at, e.full_content, e.cover_url, e.published_at,
		e.is_read, e.is_favorite, e.favorited_at, e.quality_score, e.relevance_score, e.novelty_score, e.composite_score, e.created_at,
		af.gate_status, COALESCE(af.quality_score, 0), COALESCE(af.relevance_score, 0), COALESCE(af.depth_score, 0), COALESCE(af.freshness_score, 0), COALESCE(af.novelty_score, 0), COALESCE(af.composite_score, 0), COALESCE(af.content_fingerprint, ''), COALESCE(af.feature_version, 0), COALESCE(af.scored_at, '')
	FROM entries e
	LEFT JOIN article_features af ON af.article_id = e.id`

func (s *PostgresFeedRepository) ListArticles() []model.Article {
	rows, err := s.db.Query(fullArticleSelectSQL + ` ORDER BY e.id DESC`)
	if err != nil {
		return []model.Article{}
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	for rows.Next() {
		article, err := pgScanFullArticle(rows)
		if err != nil {
			continue
		}
		articles = append(articles, article)
	}
	return articles
}

func (s *PostgresFeedRepository) ListArticleListItems(query ArticleListQuery) ([]model.Article, bool) {
	if query.Page < 1 {
		query.Page = 1
	}
	sqlText, args := pgBuildArticleListQuerySQL(query)
	rows, err := s.db.Query(sqlText, args...)
	if err != nil {
		return []model.Article{}, false
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	for rows.Next() {
		article, err := pgScanArticleListRow(rows)
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

func (s *PostgresFeedRepository) ListArticlesNeedingScoreRefresh(featureVersion int, limit int) []model.Article {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		fullArticleSelectSQL+`
		WHERE af.article_id IS NULL OR COALESCE(af.feature_version, 0) < $1
		ORDER BY e.id ASC
		LIMIT $2`,
		featureVersion, limit,
	)
	if err != nil {
		return []model.Article{}
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	for rows.Next() {
		article, err := pgScanFullArticle(rows)
		if err != nil {
			continue
		}
		articles = append(articles, article)
	}
	return articles
}

func (s *PostgresFeedRepository) ListArticlesMissingDisplaySummary(feedID int64, limit int) []model.Article {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id, feed_id, title, link, summary, ai_summary, ai_summary_status, ai_summary_updated_at, display_summary, display_summary_status, display_summary_updated_at, full_content, cover_url, published_at, is_read, is_favorite, favorited_at, created_at
		 FROM entries
		 WHERE feed_id = $1 AND display_summary = ''
		 ORDER BY id DESC
		 LIMIT $2`,
		feedID, limit,
	)
	if err != nil {
		return []model.Article{}
	}
	defer rows.Close()

	articles := make([]model.Article, 0)
	for rows.Next() {
		var article model.Article
		var readFlag int
		var favoriteFlag int
		if err := rows.Scan(
			&article.ID, &article.FeedID, &article.Title, &article.Link,
			&article.Summary, &article.AISummary, &article.AISummaryStatus, &article.AISummaryAt,
			&article.DisplaySummary, &article.DisplaySummaryStatus, &article.DisplaySummaryAt,
			&article.FullContent, &article.CoverURL, &article.PublishedAt,
			&readFlag, &favoriteFlag, &article.FavoritedAt, &article.CreatedAt,
		); err != nil {
			continue
		}
		article.IsRead = readFlag == 1
		article.IsFavorite = favoriteFlag == 1
		articles = append(articles, article)
	}
	return articles
}

func (s *PostgresFeedRepository) DeleteArticle(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM entries WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s *PostgresFeedRepository) GetArticle(id int64) (model.Article, bool) {
	row := s.db.QueryRow(fullArticleSelectSQL+` WHERE e.id = $1`, id)
	article, err := pgScanFullArticle(row)
	if err != nil {
		return model.Article{}, false
	}
	return article, true
}

func (s *PostgresFeedRepository) UpdateArticleFullContent(id int64, content string) error {
	_, err := s.db.Exec(`UPDATE entries SET full_content = $1, updated_at = $2 WHERE id = $3`, strings.TrimSpace(content), time.Now().UTC().Format(time.RFC3339), id)
	return err
}

func (s *PostgresFeedRepository) UpdateArticleSummaryState(id int64, aiSummary string, aiStatus string, displaySummary string, displayStatus string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE entries
		 SET ai_summary = $1, ai_summary_status = $2, ai_summary_updated_at = $3,
		     display_summary = $4, display_summary_status = $5, display_summary_updated_at = $6, updated_at = $7
		 WHERE id = $8`,
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

func (s *PostgresFeedRepository) UpdateArticleScores(id int64, scores model.RecommendationScores) error {
	_, err := s.db.Exec(
		`UPDATE entries SET quality_score = $1, relevance_score = $2, novelty_score = $3, composite_score = $4, updated_at = $5 WHERE id = $6`,
		scores.Quality, scores.Relevance, scores.Novelty, scores.Composite,
		time.Now().UTC().Format(time.RFC3339), id,
	)
	return err
}

func (s *PostgresFeedRepository) UpdateArticleDisplaySummary(id int64, summary string, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`UPDATE entries SET display_summary = $1, display_summary_status = $2, display_summary_updated_at = $3, updated_at = $4 WHERE id = $5`,
		strings.TrimSpace(summary), strings.TrimSpace(status), now, now, id,
	)
	return err
}

func (s *PostgresFeedRepository) UpdateArticleFeatures(id int64, features model.ArticleFeatures) error {
	now := time.Now().UTC().Format(time.RFC3339)
	scoredAt := strings.TrimSpace(features.ScoredAt)
	if scoredAt == "" {
		scoredAt = now
	}
	_, err := s.db.Exec(
		`INSERT INTO article_features(article_id, gate_status, quality_score, relevance_score, depth_score, freshness_score, novelty_score, composite_score, content_fingerprint, feature_version, scored_at, updated_at)
		 VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT(article_id) DO UPDATE SET
		 gate_status = EXCLUDED.gate_status,
		 quality_score = EXCLUDED.quality_score,
		 relevance_score = EXCLUDED.relevance_score,
		 depth_score = EXCLUDED.depth_score,
		 freshness_score = EXCLUDED.freshness_score,
		 novelty_score = EXCLUDED.novelty_score,
		 composite_score = EXCLUDED.composite_score,
		 content_fingerprint = EXCLUDED.content_fingerprint,
		 feature_version = EXCLUDED.feature_version,
		 scored_at = EXCLUDED.scored_at,
		 updated_at = EXCLUDED.updated_at`,
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
		now,
	)
	return err
}

func (s *PostgresFeedRepository) MarkArticleRead(id int64, read bool) (model.Article, bool, error) {
	flag := 0
	if read {
		flag = 1
	}
	res, err := s.db.Exec(`UPDATE entries SET is_read = $1, updated_at = $2 WHERE id = $3`, flag, time.Now().UTC().Format(time.RFC3339), id)
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

func (s *PostgresFeedRepository) MarkArticleFavorite(id int64, favorite bool) (model.Article, bool, error) {
	flag := 0
	favoritedAt := ""
	if favorite {
		flag = 1
		favoritedAt = time.Now().UTC().Format(time.RFC3339)
	}
	res, err := s.db.Exec(`UPDATE entries SET is_favorite = $1, favorited_at = $2, updated_at = $3 WHERE id = $4`, flag, favoritedAt, time.Now().UTC().Format(time.RFC3339), id)
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

func (s *PostgresFeedRepository) PurgeExpiredArticles(retentionDays int) (int, error) {
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
		ts, ok := pgParseArticleTimestamp(publishedAt, createdAt)
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

	deleted := 0
	for _, id := range expiredIDs {
		res, err := tx.Exec(`DELETE FROM entries WHERE id = $1`, id)
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

func (s *PostgresFeedRepository) GetSetting(key string) (string, bool, error) {
	row := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = $1`, strings.TrimSpace(key))
	var value string
	if err := row.Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}

func (s *PostgresFeedRepository) SetSetting(key, value string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(
		`INSERT INTO app_settings(key, value, updated_at) VALUES($1, $2, $3)
		 ON CONFLICT(key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`,
		strings.TrimSpace(key), strings.TrimSpace(value), now,
	)
	return err
}

func (s *PostgresFeedRepository) feedExists(url string) (bool, error) {
	row := s.db.QueryRow(`SELECT 1 FROM feeds WHERE url = $1 LIMIT 1`, url)
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

func (s *PostgresFeedRepository) insertEntriesTx(tx *sql.Tx, feedID int64, items []ArticleSeed, now string) (int, error) {
	existingKeys, err := pgLoadDedupKeysTx(tx)
	if err != nil {
		return 0, err
	}
	insertedCount := 0

	for _, item := range items {
		cleaned := pgCleanSeed(item)
		if cleaned.Title == "" && cleaned.Link == "" {
			continue
		}

		key := pgDedupKey(cleaned)
		if _, exists := existingKeys[key]; exists {
			continue
		}
		existingKeys[key] = struct{}{}

		scores := cleaned.RecommendationScores
		if scores == nil {
			scores = &model.RecommendationScores{}
		}

		var articleID int64
		err := tx.QueryRow(
			`INSERT INTO entries(feed_id, title, link, summary, ai_summary, ai_summary_status, ai_summary_updated_at, display_summary, display_summary_status, display_summary_updated_at, full_content, cover_url, published_at, is_read, is_favorite, favorited_at, quality_score, relevance_score, novelty_score, composite_score, created_at, updated_at)
			 VALUES($1, $2, $3, $4, '', '', '', '', '', '', $5, $6, $7, 0, 0, '', $8, $9, $10, $11, $12, $13) RETURNING id`,
			feedID, cleaned.Title, cleaned.Link, cleaned.Summary, cleaned.FullContent, cleaned.CoverURL, cleaned.PublishedAt, scores.Quality, scores.Relevance, scores.Novelty, scores.Composite, now, now,
		).Scan(&articleID)
		if err != nil {
			return insertedCount, err
		}
		if cleaned.ArticleFeatures != nil {
			if err := pgUpsertArticleFeaturesTx(tx, articleID, *cleaned.ArticleFeatures, now); err != nil {
				return insertedCount, err
			}
		}
		insertedCount++
	}

	return insertedCount, nil
}

func pgUpsertArticleFeaturesTx(tx *sql.Tx, articleID int64, features model.ArticleFeatures, now string) error {
	scoredAt := strings.TrimSpace(features.ScoredAt)
	if scoredAt == "" {
		scoredAt = now
	}
	_, err := tx.Exec(
		`INSERT INTO article_features(article_id, gate_status, quality_score, relevance_score, depth_score, freshness_score, novelty_score, composite_score, content_fingerprint, feature_version, scored_at, updated_at)
		 VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT(article_id) DO UPDATE SET
		 gate_status = EXCLUDED.gate_status,
		 quality_score = EXCLUDED.quality_score,
		 relevance_score = EXCLUDED.relevance_score,
		 depth_score = EXCLUDED.depth_score,
		 freshness_score = EXCLUDED.freshness_score,
		 novelty_score = EXCLUDED.novelty_score,
		 composite_score = EXCLUDED.composite_score,
		 content_fingerprint = EXCLUDED.content_fingerprint,
		 feature_version = EXCLUDED.feature_version,
		 scored_at = EXCLUDED.scored_at,
		 updated_at = EXCLUDED.updated_at`,
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
		now,
	)
	return err
}

func pgLoadDedupKeysTx(tx *sql.Tx) (map[string]struct{}, error) {
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
		keys[pgDedupKey(ArticleSeed{Title: title, Link: link, Summary: summary})] = struct{}{}
	}
	return keys, nil
}

func pgCleanSeed(seed ArticleSeed) ArticleSeed {
	seed.Title = strings.TrimSpace(seed.Title)
	seed.Link = strings.TrimSpace(seed.Link)
	seed.Summary = strings.TrimSpace(seed.Summary)
	seed.FullContent = strings.TrimSpace(seed.FullContent)
	seed.CoverURL = strings.TrimSpace(seed.CoverURL)
	seed.PublishedAt = strings.TrimSpace(seed.PublishedAt)
	return seed
}

func (s *PostgresFeedRepository) UpdateFeedScript(id int64, script string, lang string) (model.Feed, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET custom_script = $1, custom_script_lang = $2, updated_at = $3 WHERE id = $4`, strings.TrimSpace(script), strings.TrimSpace(lang), now, id)
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

func (s *PostgresFeedRepository) UpdateFeedTitle(id int64, title string) (model.Feed, bool, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return model.Feed{}, false, errors.New("title is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET title = $1, updated_at = $2 WHERE id = $3`, title, now, id)
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

func (s *PostgresFeedRepository) UpdateFeedIcon(id int64, iconPath string) (model.Feed, bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE feeds SET icon_path = $1, icon_fetched_at = $2, updated_at = $3 WHERE id = $4`, strings.TrimSpace(iconPath), now, now, id)
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

func pgDedupKey(seed ArticleSeed) string {
	normalizedLink := pgNormalizeForKey(seed.Link)
	if normalizedLink != "" {
		return pgHashText("link:" + normalizedLink)
	}
	normalizedTitle := pgNormalizeForKey(seed.Title)
	normalizedSummary := pgNormalizeForKey(seed.Summary)
	return pgHashText("text:" + normalizedTitle + "|" + normalizedSummary)
}

func pgNormalizeForKey(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	return strings.Join(strings.Fields(v), " ")
}

func pgHashText(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func pgNullableInt(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

func pgParseArticleTimestamp(publishedAt string, createdAt string) (time.Time, bool) {
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

func pgIsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "unique_violation") ||
		strings.Contains(strings.ToLower(msg), "unique") || strings.Contains(strings.ToLower(msg), "duplicate key")
}
