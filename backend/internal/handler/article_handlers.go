package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/service"
)

type articleListItem struct {
	ID                   int64                         `json:"id"`
	FeedID               int64                         `json:"feed_id"`
	Title                string                        `json:"title"`
	Link                 string                        `json:"link"`
	CoverURL             string                        `json:"cover_url,omitempty"`
	PublishedAt          string                        `json:"published_at,omitempty"`
	IsRead               bool                          `json:"is_read"`
	IsFavorite           bool                          `json:"is_favorite"`
	FavoritedAt          string                        `json:"favorited_at,omitempty"`
	CreatedAt            string                        `json:"created_at"`
	RecommendationScores map[string]int                `json:"recommendation_scores,omitempty"`
}

type markReadRequest struct {
	Read bool `json:"read"`
}

type markFavoriteRequest struct {
	Favorite bool `json:"favorite"`
}

func (s *Server) handleArticles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	limit := 0
	page := 1
	var feedID *int64
	var folderID *int64
	sortMode, err := service.NormalizeArticleSortMode(r.URL.Query().Get("sort"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sort"})
		return
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("feed_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid feed_id"})
			return
		}
		feedID = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("folder_id")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid folder_id"})
			return
		}
		folderID = &parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}
		if parsed > 200 {
			parsed = 200
		}
		limit = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid page"})
			return
		}
		page = parsed
	}

	articles, hasMore := s.articleUC.List(page, limit, sortMode, feedID, folderID)
	items := make([]articleListItem, 0, len(articles))
	for _, article := range articles {
		item := articleListItem{
			ID:          article.ID,
			FeedID:      article.FeedID,
			Title:       article.Title,
			Link:        article.Link,
			CoverURL:    article.CoverURL,
			PublishedAt: article.PublishedAt,
			IsRead:      article.IsRead,
			IsFavorite:  article.IsFavorite,
			FavoritedAt: article.FavoritedAt,
			CreatedAt:   article.CreatedAt,
		}
		if article.RecommendationScores != nil {
			item.RecommendationScores = map[string]int{
				"quality":   article.RecommendationScores.Quality,
				"relevance": article.RecommendationScores.Relevance,
				"novelty":   article.RecommendationScores.Novelty,
				"composite": article.RecommendationScores.Composite,
			}
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"articles": items, "has_more": hasMore})
}

func (s *Server) handleArticleByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/articles/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	if len(parts) == 1 {
		if r.Method == http.MethodGet {
			article, ok := s.articleUC.Get(id)
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
				return
			}
			writeJSON(w, http.StatusOK, article)
			return
		}
		if r.Method == http.MethodDelete {
			deleted, err := s.articleUC.Delete(id)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete article"})
				return
			}
			if !deleted {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
			return
		}
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if len(parts) == 2 && parts[1] == "read" {
		if r.Method != http.MethodPatch {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		var req markReadRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		article, ok, err := s.articleUC.MarkRead(id, req.Read)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update article"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			return
		}
		writeJSON(w, http.StatusOK, article)
		return
	}

	if len(parts) == 2 && parts[1] == "favorite" {
		if r.Method != http.MethodPatch {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		var req markFavoriteRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		article, ok, err := s.articleUC.MarkFavorite(id, req.Favorite)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update favorite"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			return
		}
		writeJSON(w, http.StatusOK, article)
		return
	}

	if len(parts) == 2 && parts[1] == "readability" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		updated, err := s.articleUC.ExtractReadable(r.Context(), id)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrArticleNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			case errors.Is(err, service.ErrArticleLinkEmpty):
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "article link is empty"})
			case errors.Is(err, service.ErrReadabilityFetchFail):
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			case errors.Is(err, service.ErrSaveArticleContent):
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save extracted content"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to extract readable content"})
			}
			return
		}
		if err := s.summaryUC.RefreshArticle(id); err != nil && !errors.Is(err, service.ErrArticleNotFound) {
			s.logger.Warn("summary", "refresh", "failed", "display summary refresh failed after readability extraction", "article_id", id, "error", err.Error())
		}
		if article, ok := s.articleUC.Get(id); ok {
			updated = article
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}

	if len(parts) == 2 && parts[1] == "refresh-cache" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		updated, err := s.articleUC.RefreshCache(r.Context(), id)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrArticleNotFound):
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			case errors.Is(err, service.ErrReadabilityFetchFail):
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "cache refresh failed: " + err.Error()})
			case errors.Is(err, service.ErrSaveArticleContent):
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save refreshed readability content"})
			default:
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to refresh cache"})
			}
			return
		}
		if err := s.summaryUC.RefreshArticle(id); err != nil && !errors.Is(err, service.ErrArticleNotFound) {
			s.logger.Warn("summary", "refresh", "failed", "display summary refresh failed after cache refresh", "article_id", id, "error", err.Error())
		}
		if article, ok := s.articleUC.Get(id); ok {
			updated = article
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}

	if len(parts) == 2 && parts[1] == "translate" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		article, ok := s.articleUC.Get(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			return
		}

		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		settings, err := s.loadAISettings()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load ai settings"})
			return
		}
		req := translateArticleRequest{TargetLang: settings.TargetLang}
		if len(strings.TrimSpace(string(body))) > 0 {
			if err := json.Unmarshal(body, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
				return
			}
		}
		targetLang := strings.TrimSpace(req.TargetLang)
		if targetLang == "" {
			targetLang = settings.TargetLang
		}
		if targetLang == "" {
			targetLang = defaultAITargetLang
		}

		paragraphs := extractArticleTranslationParagraphs(article)
		if len(paragraphs) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "article content is empty"})
			return
		}

		artCtx := articleContext{
			Title:     article.Title,
			AISummary: article.AISummary,
		}
		if feed, ok, _ := s.store.GetFeed(article.FeedID); ok {
			artCtx.FeedTitle = feed.Title
		}

		translatedParts := make([]string, 0, len(paragraphs))
		err = s.translateParagraphs(r.Context(), paragraphs, targetLang, settings, artCtx, func(_ int, _ int, _ string, translated string) error {
			translatedParts = append(translatedParts, translated)
			return nil
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "translation failed: " + err.Error()})
			return
		}
		translated := strings.Join(translatedParts, "\n\n")
		writeJSON(w, http.StatusOK, map[string]any{"article_id": article.ID, "target_lang": targetLang, "translated_text": translated, "source_char_count": len(strings.Join(paragraphs, "\n\n"))})
		return
	}

	if len(parts) == 3 && parts[1] == "translate" && parts[2] == "stream" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		s.streamArticleTranslation(w, r, id)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}
