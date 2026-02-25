package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/feedparser"
	"github.com/Sentixxx/Zflow/backend/internal/store"
)

type Server struct {
	store  *store.FeedStore
	client *http.Client
}

type createFeedRequest struct {
	URL string `json:"url"`
}

func NewServer(feedStore *store.FeedStore) *Server {
	return &Server{
		store: feedStore,
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/feeds", s.handleFeeds)
	mux.HandleFunc("/api/v1/articles", s.handleArticles)
	mux.HandleFunc("/api/v1/articles/", s.handleArticleByID)
	mux.HandleFunc("/healthz", s.handleHealth)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleFeeds(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"feeds": s.store.List()})
	case http.MethodPost:
		s.createFeed(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) createFeed(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	var req createFeedRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	req.URL = strings.TrimSpace(req.URL)

	if !isValidFeedURL(req.URL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid feed url"})
		return
	}

	title, items, fetchErr := s.fetchAndParse(req.URL)
	feed, err := s.store.Add(req.URL, title, items, fetchErr)
	if err == store.ErrFeedExists {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "feed already exists"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save feed"})
		return
	}

	writeJSON(w, http.StatusCreated, feed)
}

type markReadRequest struct {
	Read bool `json:"read"`
}

func (s *Server) handleArticles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"articles": s.store.ListArticles()})
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
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		article, ok := s.store.GetArticle(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			return
		}
		writeJSON(w, http.StatusOK, article)
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

		article, ok, err := s.store.MarkArticleRead(id, req.Read)
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

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (s *Server) fetchAndParse(feedURL string) (string, []store.ArticleSeed, string) {
	resp, err := s.client.Get(feedURL)
	if err != nil {
		return "", nil, err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", nil, "fetch failed: non-2xx status"
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", nil, err.Error()
	}

	parsed, err := feedparser.ParseFeed(raw)
	if err != nil {
		return "", nil, err.Error()
	}

	items := make([]store.ArticleSeed, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		items = append(items, store.ArticleSeed{
			Title:       item.Title,
			Link:        item.Link,
			Summary:     item.Summary,
			PublishedAt: item.PublishedAt,
		})
	}

	return parsed.Title, items, ""
}

func isValidFeedURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
