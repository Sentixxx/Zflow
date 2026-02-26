package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/repository"
)

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

	result := s.fetchAndParse(req.URL, "", "")
	feed, err := s.store.AddInFolder(req.URL, result.Title, result.Items, result.Error, req.FolderID, result.ETag, result.LastModified)
	if err == repository.ErrFeedExists {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "feed already exists"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save feed"})
		return
	}

	if script := strings.TrimSpace(req.Script); script != "" {
		lang := normalizeScriptLang(req.ScriptLang)
		updated, ok, err := s.store.UpdateFeedScript(feed.ID, script, lang)
		if err == nil && ok {
			feed = updated
		}
	}
	s.tryRefreshFeedIcon(feed.ID, feed.URL, feed.IconPath, feed.IconFetchedAt, result.IconHints)

	writeJSON(w, http.StatusCreated, feed)
}

func (s *Server) updateFeed(w http.ResponseWriter, r *http.Request, id int64) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var req updateFeedRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	feed, ok, err := s.store.UpdateFeedFolder(id, req.FolderID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update feed"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "feed not found"})
		return
	}
	writeJSON(w, http.StatusOK, feed)
}

func (s *Server) updateFeedScript(w http.ResponseWriter, r *http.Request, id int64) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var req updateFeedScriptRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	lang := normalizeScriptLang(req.ScriptLang)
	if !isSupportedScriptLang(lang) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported script language"})
		return
	}
	feed, ok, err := s.store.UpdateFeedScript(id, req.Script, lang)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update feed script"})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "feed not found"})
		return
	}
	writeJSON(w, http.StatusOK, feed)
}

func (s *Server) updateFeedTitle(w http.ResponseWriter, r *http.Request, id int64) {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var req updateFeedTitleRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	feed, ok, err := s.store.UpdateFeedTitle(id, req.Title)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "feed not found"})
		return
	}
	writeJSON(w, http.StatusOK, feed)
}
