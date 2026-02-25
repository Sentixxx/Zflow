package api

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/feedparser"
	"github.com/Sentixxx/Zflow/backend/internal/store"
)

type Server struct {
	store   *store.FeedStore
	client  *http.Client
	iconDir string
}

type createFeedRequest struct {
	URL        string `json:"url"`
	FolderID   *int64 `json:"folder_id"`
	Script     string `json:"script"`
	ScriptLang string `json:"script_lang"`
}

type createFolderRequest struct {
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id"`
}

type updateFolderRequest struct {
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id"`
}

type updateFeedRequest struct {
	FolderID *int64 `json:"folder_id"`
}

type updateFeedScriptRequest struct {
	Script     string `json:"script"`
	ScriptLang string `json:"script_lang"`
}

type updateFeedTitleRequest struct {
	Title string `json:"title"`
}

func NewServer(feedStore *store.FeedStore, dataDir string) *Server {
	iconDir := filepath.Join(dataDir, "icons")
	_ = os.MkdirAll(iconDir, 0o755)
	return &Server{
		store: feedStore,
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
		iconDir: iconDir,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/feeds", s.handleFeeds)
	mux.HandleFunc("/api/v1/feeds/", s.handleFeedByID)
	mux.HandleFunc("/api/v1/folders", s.handleFolders)
	mux.HandleFunc("/api/v1/folders/", s.handleFolderByID)
	mux.HandleFunc("/api/v1/articles", s.handleArticles)
	mux.HandleFunc("/api/v1/articles/", s.handleArticleByID)
	mux.HandleFunc("/api/v1/icons/", s.handleFeedIcon)
	mux.HandleFunc("/api/v1/data/export/profile", s.handleExportProfile)
	mux.HandleFunc("/api/v1/data/import/profile", s.handleImportProfile)
	mux.HandleFunc("/api/v1/data/export/opml", s.handleExportOPML)
	mux.HandleFunc("/api/v1/data/import/opml", s.handleImportOPML)
	mux.HandleFunc("/healthz", s.handleHealth)
	return corsMiddleware(mux)
}

type profileFolderRecord struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

type profileFeedRecord struct {
	URL              string `json:"url"`
	Title            string `json:"title"`
	FolderID         *int64 `json:"folder_id,omitempty"`
	CustomScript     string `json:"custom_script,omitempty"`
	CustomScriptLang string `json:"custom_script_lang,omitempty"`
}

type profileExportPayload struct {
	Version    string                `json:"version"`
	ExportedAt string                `json:"exported_at"`
	Folders    []profileFolderRecord `json:"folders"`
	Feeds      []profileFeedRecord   `json:"feeds"`
}

type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    opmlBody `xml:"body"`
}

type opmlHead struct {
	Title string `xml:"title"`
}

type opmlBody struct {
	Outlines []opmlOutline `xml:"outline"`
}

type opmlOutline struct {
	Text     string        `xml:"text,attr,omitempty"`
	Title    string        `xml:"title,attr,omitempty"`
	Type     string        `xml:"type,attr,omitempty"`
	XMLURL   string        `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Outlines []opmlOutline `xml:"outline,omitempty"`
}

func (s *Server) handleExportProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	folders := s.store.ListFolders()
	feeds := s.store.List()

	outFolders := make([]profileFolderRecord, 0, len(folders))
	for _, f := range folders {
		outFolders = append(outFolders, profileFolderRecord{
			ID:       f.ID,
			Name:     f.Name,
			ParentID: f.ParentID,
		})
	}
	outFeeds := make([]profileFeedRecord, 0, len(feeds))
	for _, f := range feeds {
		outFeeds = append(outFeeds, profileFeedRecord{
			URL:              f.URL,
			Title:            f.Title,
			FolderID:         f.FolderID,
			CustomScript:     f.CustomScript,
			CustomScriptLang: normalizeScriptLang(f.CustomScriptLang),
		})
	}
	payload := profileExportPayload{
		Version:    "zflow-profile-v1",
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Folders:    outFolders,
		Feeds:      outFeeds,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="zflow-profile.json"`)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *Server) handleImportProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var payload profileExportPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	folderMap := make(map[int64]int64)
	pending := append([]profileFolderRecord(nil), payload.Folders...)
	for len(pending) > 0 {
		progress := false
		rest := make([]profileFolderRecord, 0, len(pending))
		for _, f := range pending {
			var parentID *int64
			if f.ParentID != nil {
				mapped, ok := folderMap[*f.ParentID]
				if !ok {
					rest = append(rest, f)
					continue
				}
				parentID = &mapped
			}
			created, err := s.store.CreateFolder(f.Name, parentID)
			if err != nil {
				rest = append(rest, f)
				continue
			}
			folderMap[f.ID] = created.ID
			progress = true
		}
		if !progress {
			break
		}
		pending = rest
	}

	importedFeeds := 0
	updatedFeeds := 0
	for _, fr := range payload.Feeds {
		if strings.TrimSpace(fr.URL) == "" {
			continue
		}
		var newFolderID *int64
		if fr.FolderID != nil {
			if mapped, ok := folderMap[*fr.FolderID]; ok {
				newFolderID = &mapped
			}
		}

		existing, ok, err := s.store.GetFeedByURL(fr.URL)
		if err != nil {
			continue
		}
		if !ok {
			created, err := s.store.CreateFeedPlaceholder(fr.URL, fr.Title, newFolderID)
			if err != nil {
				continue
			}
			if strings.TrimSpace(fr.CustomScript) != "" {
				_, _, _ = s.store.UpdateFeedScript(created.ID, fr.CustomScript, normalizeScriptLang(fr.CustomScriptLang))
			}
			importedFeeds++
			continue
		}

		if strings.TrimSpace(fr.Title) != "" && fr.Title != existing.Title {
			_, _, _ = s.store.UpdateFeedTitle(existing.ID, fr.Title)
		}
		_, _, _ = s.store.UpdateFeedFolder(existing.ID, newFolderID)
		_, _, _ = s.store.UpdateFeedScript(existing.ID, fr.CustomScript, normalizeScriptLang(fr.CustomScriptLang))
		updatedFeeds++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"imported_feeds":   importedFeeds,
		"updated_feeds":    updatedFeeds,
		"imported_folders": len(folderMap),
	})
}

func (s *Server) handleExportOPML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	folders := s.store.ListFolders()
	feeds := s.store.List()

	root := make([]opmlOutline, 0)
	foldersByParent := make(map[int64][]storeFolderView)
	rootFolders := make([]storeFolderView, 0)
	feedsByFolder := make(map[int64][]opmlOutline)
	rootFeeds := make([]opmlOutline, 0)

	for _, f := range feeds {
		node := opmlOutline{
			Text:    feedTitleFallback(f.Title, f.URL),
			Title:   feedTitleFallback(f.Title, f.URL),
			Type:    "rss",
			XMLURL:  f.URL,
			HTMLURL: f.URL,
		}
		if f.FolderID == nil {
			rootFeeds = append(rootFeeds, node)
		} else {
			feedsByFolder[*f.FolderID] = append(feedsByFolder[*f.FolderID], node)
		}
	}

	for _, f := range folders {
		v := storeFolderView{ID: f.ID, Name: f.Name, ParentID: f.ParentID}
		if f.ParentID == nil {
			rootFolders = append(rootFolders, v)
		} else {
			foldersByParent[*f.ParentID] = append(foldersByParent[*f.ParentID], v)
		}
	}

	var buildFolder func(storeFolderView) opmlOutline
	buildFolder = func(f storeFolderView) opmlOutline {
		node := opmlOutline{
			Text:  f.Name,
			Title: f.Name,
		}
		node.Outlines = append(node.Outlines, feedsByFolder[f.ID]...)
		for _, child := range foldersByParent[f.ID] {
			node.Outlines = append(node.Outlines, buildFolder(child))
		}
		return node
	}

	for _, rf := range rootFolders {
		root = append(root, buildFolder(rf))
	}
	root = append(root, rootFeeds...)

	doc := opmlDoc{
		Version: "2.0",
		Head:    opmlHead{Title: "Zflow Subscriptions"},
		Body:    opmlBody{Outlines: root},
	}
	encoded, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate opml"})
		return
	}
	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="zflow-subscriptions.opml"`)
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(encoded)
}

type storeFolderView struct {
	ID       int64
	Name     string
	ParentID *int64
}

func feedTitleFallback(title, rawURL string) string {
	if strings.TrimSpace(title) != "" {
		return strings.TrimSpace(title)
	}
	return strings.TrimSpace(rawURL)
}

func (s *Server) handleImportOPML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	var doc opmlDoc
	if err := xml.Unmarshal(body, &doc); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid opml"})
		return
	}

	createdFolders := 0
	importedFeeds := 0
	updatedFeeds := 0

	var walk func([]opmlOutline, *int64)
	walk = func(nodes []opmlOutline, parentID *int64) {
		for _, node := range nodes {
			if strings.TrimSpace(node.XMLURL) != "" || strings.EqualFold(strings.TrimSpace(node.Type), "rss") {
				url := strings.TrimSpace(node.XMLURL)
				if url == "" {
					continue
				}
				title := strings.TrimSpace(node.Title)
				if title == "" {
					title = strings.TrimSpace(node.Text)
				}
				existing, ok, err := s.store.GetFeedByURL(url)
				if err != nil {
					continue
				}
				if !ok {
					_, err := s.store.CreateFeedPlaceholder(url, title, parentID)
					if err == nil {
						importedFeeds++
					}
					continue
				}
				if title != "" && title != existing.Title {
					_, _, _ = s.store.UpdateFeedTitle(existing.ID, title)
				}
				_, _, _ = s.store.UpdateFeedFolder(existing.ID, parentID)
				updatedFeeds++
				continue
			}

			name := strings.TrimSpace(node.Title)
			if name == "" {
				name = strings.TrimSpace(node.Text)
			}
			if name == "" {
				name = "Untitled Folder"
			}
			f, err := s.store.CreateFolder(name, parentID)
			var currentParent *int64 = parentID
			if err == nil {
				createdFolders++
				currentParent = &f.ID
			}
			walk(node.Outlines, currentParent)
		}
	}
	walk(doc.Body.Outlines, nil)

	writeJSON(w, http.StatusOK, map[string]any{
		"imported_feeds":   importedFeeds,
		"updated_feeds":    updatedFeeds,
		"imported_folders": createdFolders,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

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
	http.ServeFile(w, r, iconFile)
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
		ok, err := s.store.DeleteFolder(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete folder"})
			return
		}
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "folder not found"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
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

	result := s.fetchAndParse(req.URL, "", "")
	feed, err := s.store.AddInFolder(req.URL, result.Title, result.Items, result.Error, req.FolderID, result.ETag, result.LastModified)
	if err == store.ErrFeedExists {
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
	s.tryRefreshFeedIcon(feed.ID, feed.URL, feed.IconPath, feed.IconFetchedAt)

	writeJSON(w, http.StatusCreated, feed)
}

type markReadRequest struct {
	Read bool `json:"read"`
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
		switch r.Method {
		case http.MethodGet:
			article, ok := s.store.GetArticle(id)
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
				return
			}
			writeJSON(w, http.StatusOK, article)
		case http.MethodDelete:
			ok, err := s.store.DeleteArticle(id)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete article"})
				return
			}
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
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

type fetchResult struct {
	Title        string
	Items        []store.ArticleSeed
	ETag         string
	LastModified string
	NotModified  bool
	Error        string
}

func (s *Server) fetchAndParse(feedURL, etag, lastModified string) fetchResult {
	req, err := http.NewRequest(http.MethodGet, feedURL, nil)
	if err != nil {
		return fetchResult{Error: err.Error()}
	}
	req.Header.Set("User-Agent", "Zflow/0.1 (+https://github.com/Sentixxx/Zflow)")
	req.Header.Set("Accept", "application/atom+xml, application/rss+xml, application/xml, text/xml, */*")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fetchResult{Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return fetchResult{
			NotModified:  true,
			ETag:         pickHeaderOrDefault(resp.Header.Get("ETag"), etag),
			LastModified: pickHeaderOrDefault(resp.Header.Get("Last-Modified"), lastModified),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fetchResult{Error: fmt.Sprintf("fetch failed: http %d", resp.StatusCode)}
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fetchResult{Error: err.Error()}
	}

	parsed, err := feedparser.ParseFeed(raw)
	if err != nil {
		return fetchResult{Error: err.Error()}
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

	return fetchResult{
		Title:        parsed.Title,
		Items:        items,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}
}

func (s *Server) refreshFeedByID(feedID int64) error {
	feed, ok, err := s.store.GetFeed(feedID)
	if err != nil {
		return fmt.Errorf("failed to load feed: %w", err)
	}
	if !ok {
		return errors.New("feed not found")
	}

	result := s.fetchAndParse(feed.URL, feed.ETag, feed.LastModified)
	if result.NotModified {
		return s.store.UpdateFeedAfterRefresh(feedID, feed.Title, nil, "", result.ETag, result.LastModified)
	}
	if result.Error != "" {
		return s.store.UpdateFeedAfterRefresh(feedID, feed.Title, nil, result.Error, feed.ETag, feed.LastModified)
	}
	if script := strings.TrimSpace(feed.CustomScript); script != "" {
		items, err := s.applyScriptToItems(feed.ID, feed.URL, script, normalizeScriptLang(feed.CustomScriptLang), result.Items)
		if err != nil {
			log.Printf("feed %d custom script failed, fallback to raw summary: %v", feed.ID, err)
		} else {
			result.Items = items
		}
	}
	s.tryRefreshFeedIcon(feed.ID, feed.URL, feed.IconPath, feed.IconFetchedAt)
	return s.store.UpdateFeedAfterRefresh(feedID, result.Title, result.Items, "", result.ETag, result.LastModified)
}

type scriptItemPayload struct {
	Title       string `json:"title"`
	Link        string `json:"link"`
	Summary     string `json:"summary"`
	PublishedAt string `json:"published_at"`
}

type scriptFeedPayload struct {
	ID  int64  `json:"id"`
	URL string `json:"url"`
}

type scriptRequestPayload struct {
	Version string            `json:"version"`
	Feed    scriptFeedPayload `json:"feed"`
	Item    scriptItemPayload `json:"item"`
}

type scriptResultPayload struct {
	OK          bool   `json:"ok"`
	Title       string `json:"title"`
	SummaryHTML string `json:"summary_html"`
	ContentHTML string `json:"content_html"`
	ExcerptText string `json:"excerpt_text"`
	Debug       string `json:"debug"`
}

func (s *Server) applyScriptToItems(feedID int64, feedURL, script, lang string, items []store.ArticleSeed) ([]store.ArticleSeed, error) {
	out := make([]store.ArticleSeed, 0, len(items))
	for _, item := range items {
		payload := scriptRequestPayload{
			Version: "v1",
			Feed: scriptFeedPayload{
				ID:  feedID,
				URL: feedURL,
			},
			Item: scriptItemPayload{
				Title:       item.Title,
				Link:        item.Link,
				Summary:     item.Summary,
				PublishedAt: item.PublishedAt,
			},
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		stdout, err := runScript(lang, script, raw)
		if err != nil {
			log.Printf("feed %d script failed for item %q: %v", feedID, item.Link, err)
			out = append(out, item)
			continue
		}

		var result scriptResultPayload
		if err := json.Unmarshal(stdout, &result); err != nil {
			log.Printf("feed %d script output is not valid JSON for item %q: %v", feedID, item.Link, err)
			out = append(out, item)
			continue
		}
		if !result.OK {
			if msg := strings.TrimSpace(result.Debug); msg != "" {
				log.Printf("feed %d script returned ok=false for item %q: %s", feedID, item.Link, msg)
			}
			out = append(out, item)
			continue
		}

		if v := strings.TrimSpace(result.Title); v != "" {
			item.Title = v
		}
		if v := strings.TrimSpace(result.SummaryHTML); v != "" {
			item.Summary = v
		}
		if v := strings.TrimSpace(result.ContentHTML); v != "" {
			item.FullContent = v
		}
		out = append(out, item)
	}
	return out, nil
}

func runScript(lang, script string, stdin []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch normalizeScriptLang(lang) {
	case "python":
		cmd = exec.CommandContext(ctx, "python3", "-c", script)
	case "javascript":
		cmd = exec.CommandContext(ctx, "node", "-e", script)
	default:
		cmd = exec.CommandContext(ctx, "/bin/sh", "-lc", script)
	}
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		return nil, fmt.Errorf("script run failed: %s", errText)
	}
	if stdout.Len() > 1<<20 {
		return nil, errors.New("script output too large")
	}
	return stdout.Bytes(), nil
}

func normalizeScriptLang(raw string) string {
	lang := strings.ToLower(strings.TrimSpace(raw))
	if lang == "" {
		return "shell"
	}
	return lang
}

func isSupportedScriptLang(lang string) bool {
	switch normalizeScriptLang(lang) {
	case "shell", "python", "javascript":
		return true
	default:
		return false
	}
}

func (s *Server) tryRefreshFeedIcon(feedID int64, feedURL, existingIconPath, iconFetchedAt string) {
	if strings.TrimSpace(existingIconPath) != "" && !needsIconRefresh(iconFetchedAt) {
		return
	}
	iconURL, ok := discoverIconURL(feedURL)
	if !ok {
		return
	}
	iconBytes, ext, err := s.fetchIcon(iconURL)
	if err != nil {
		return
	}
	relativePath, err := s.persistIcon(feedID, iconBytes, ext)
	if err != nil {
		return
	}
	_, _, _ = s.store.UpdateFeedIcon(feedID, relativePath)
}

func needsIconRefresh(last string) bool {
	trimmed := strings.TrimSpace(last)
	if trimmed == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return true
	}
	return time.Since(t) >= 7*24*time.Hour
}

func discoverIconURL(feedURL string) (string, bool) {
	u, err := url.Parse(feedURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host + "/favicon.ico", true
}

func (s *Server) fetchIcon(iconURL string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, iconURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Zflow/0.1 (+https://github.com/Sentixxx/Zflow)")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", fmt.Errorf("icon fetch failed: %d", resp.StatusCode)
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		return nil, "", errors.New("icon is not image content")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return nil, "", err
	}
	if len(raw) == 0 {
		return nil, "", errors.New("icon is empty")
	}
	ext := iconExt(iconURL, contentType)
	return raw, ext, nil
}

func iconExt(iconURL, contentType string) string {
	ext := strings.ToLower(filepath.Ext(iconURL))
	if ext == ".ico" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".svg" {
		if ext == ".jpeg" {
			return ".jpg"
		}
		return ext
	}
	if exts, _ := mime.ExtensionsByType(strings.Split(contentType, ";")[0]); len(exts) > 0 {
		switch exts[0] {
		case ".jpeg":
			return ".jpg"
		default:
			return exts[0]
		}
	}
	return ".ico"
}

func (s *Server) persistIcon(feedID int64, raw []byte, ext string) (string, error) {
	h := fnv.New64a()
	_, _ = h.Write(raw)
	fileName := fmt.Sprintf("feed-%d-%x%s", feedID, h.Sum64(), ext)
	fullPath := filepath.Join(s.iconDir, fileName)
	if err := os.WriteFile(fullPath, raw, 0o644); err != nil {
		return "", err
	}
	return fileName, nil
}

func (s *Server) StartRefreshLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			feeds := s.store.List()
			for _, feed := range feeds {
				if err := s.refreshFeedByID(feed.ID); err != nil {
					log.Printf("refresh feed %d failed: %v", feed.ID, err)
				}
			}
		}
	}
}

func pickHeaderOrDefault(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
