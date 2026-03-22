package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/service"
)

type clearSummariesResponse struct {
	Cleared int `json:"cleared"`
}

func (s *Server) handleDevArticles(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/dev/articles/")
	if strings.TrimSpace(path) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	if path == "clear-recent-ai-summaries" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		cleared, err := s.summaryUC.ClearRecentArticleAI(100)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear ai summaries"})
			return
		}
		writeJSON(w, http.StatusOK, clearSummariesResponse{Cleared: cleared})
		return
	}

	parts := strings.Split(path, "/")
	if len(parts) != 2 || (parts[1] != "clear-ai-summary" && parts[1] != "refresh-ai-summary") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	switch parts[1] {
	case "clear-ai-summary":
		if err := s.summaryUC.ClearArticleAI(id); err != nil {
			if err == service.ErrArticleNotFound {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear ai summary"})
			return
		}
	case "refresh-ai-summary":
		article, err := s.summaryUC.RefreshArticleWithDebug(id)
		if err != nil {
			if err == service.ErrArticleNotFound {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to refresh ai summary"})
			return
		}
		writeJSON(w, http.StatusOK, article)
		return
	}
	article, ok := s.articleUC.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
		return
	}
	writeJSON(w, http.StatusOK, article)
}
