package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Server) handleFeedIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/icons/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid feed id"})
		return
	}
	feed, ok, err := s.store.GetFeed(id)
	if err != nil || !ok || strings.TrimSpace(feed.IconPath) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "icon not found"})
		return
	}
	iconFile := filepath.Join(s.iconDir, filepath.Base(feed.IconPath))
	if fileExists(iconFile) {
		http.ServeFile(w, r, iconFile)
		return
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "icon file not found"})
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

func (s *Server) handleFeedByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/feeds/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 || strings.TrimSpace(parts[0]) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid feed id"})
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			feed, ok, err := s.store.GetFeed(id)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load feed"})
				return
			}
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "feed not found"})
				return
			}
			writeJSON(w, http.StatusOK, feed)
		case http.MethodPatch:
			s.updateFeed(w, r, id)
		case http.MethodDelete:
			ok, err := s.store.DeleteFeed(id)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete feed"})
				return
			}
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "feed not found"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
		return
	}

	if len(parts) == 2 && parts[1] == "refresh" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		if err := s.refreshFeedByID(id); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"refreshed": true})
		return
	}
	if len(parts) == 2 && parts[1] == "script" {
		if r.Method != http.MethodPatch {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		s.updateFeedScript(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "title" {
		if r.Method != http.MethodPatch {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		s.updateFeedTitle(w, r, id)
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (s *Server) handleFolders(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"folders": s.store.ListFolders()})
	case http.MethodPost:
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		var req createFolderRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		folder, err := s.store.CreateFolder(req.Name, req.ParentID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, folder)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleFolderByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/api/v1/folders/")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid folder id"})
		return
	}

	switch r.Method {
	case http.MethodPatch:
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		var req updateFolderRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		folder, ok, err := s.store.UpdateFolder(id, req.Name, req.ParentID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "folder not found"})
			return
		}
		writeJSON(w, http.StatusOK, folder)
	case http.MethodDelete:
		deleted, err := s.store.DeleteFolder(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete folder"})
			return
		}
		if !deleted {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "folder not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}
