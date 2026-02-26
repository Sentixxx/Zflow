package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/fnv"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/feedparser"
	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/internal/service"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
	readability "github.com/go-shiori/go-readability"
)

type Server struct {
	store         service.FeedService
	client        *http.Client
	clientMu      sync.RWMutex
	iconDir       string
	legacyIconDir string
	logger        *logger.ModuleLogger
}

const (
	settingKeyNetworkProxy = "network_proxy_url"
	settingKeyAIApiKey     = "ai_api_key"
	settingKeyAIBaseURL    = "ai_base_url"
	settingKeyAIModel      = "ai_model"
	settingKeyAITargetLang = "ai_target_lang"
	defaultAIBaseURL       = "https://api.openai.com/v1"
	defaultAIModel         = "gpt-4o-mini"
	defaultAITargetLang    = "zh-CN"
)

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

type updateNetworkSettingsRequest struct {
	ProxyURL string `json:"proxy_url"`
}

type updateAISettingsRequest struct {
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	TargetLang string `json:"target_lang"`
}

type translateArticleRequest struct {
	TargetLang string `json:"target_lang"`
}

type aiSettings struct {
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	TargetLang string `json:"target_lang"`
}

type translationPair struct {
	Source     string
	Translated string
}

type translateStreamEvent struct {
	Type       string   `json:"type"`
	ArticleID  int64    `json:"article_id,omitempty"`
	TargetLang string   `json:"target_lang,omitempty"`
	Total      int      `json:"total,omitempty"`
	Sources    []string `json:"sources,omitempty"`
	Index      int      `json:"index,omitempty"`
	Source     string   `json:"source,omitempty"`
	Translated string   `json:"translated,omitempty"`
	Error      string   `json:"error,omitempty"`
}

func NewServer(feedStore service.FeedService, dataDir string) *Server {
	iconDir := filepath.Join(dataDir, "feed-icons")
	legacyIconDir := filepath.Join(dataDir, "icons")
	_ = os.MkdirAll(iconDir, 0o755)
	_ = os.MkdirAll(legacyIconDir, 0o755)
	server := &Server{
		store:         feedStore,
		iconDir:       iconDir,
		legacyIconDir: legacyIconDir,
		logger:        logger.NewModuleFromEnv("handler"),
	}
	proxyURL, ok, err := feedStore.GetSetting(settingKeyNetworkProxy)
	if err != nil {
		server.logger.Warn("settings", "network", "failed", "load network proxy setting failed", "error", err.Error())
	}
	if !ok {
		proxyURL = firstNonEmpty(os.Getenv("ZFLOW_HTTP_PROXY"), os.Getenv("HTTPS_PROXY"), os.Getenv("HTTP_PROXY"))
	}
	if err := server.applyNetworkProxy(proxyURL); err != nil {
		server.logger.Warn("settings", "network", "failed", "apply initial network proxy failed", "proxy_url", proxyURL, "error", err.Error())
		_ = server.applyNetworkProxy("")
	}
	return server
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
	mux.HandleFunc("/api/v1/settings/network", s.handleNetworkSettings)
	mux.HandleFunc("/api/v1/settings/ai", s.handleAISettings)
	mux.HandleFunc("/healthz", s.handleHealth)
	return corsMiddleware(s.requestLogMiddleware(mux))
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

func (s *Server) handleNetworkSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		value, ok, err := s.store.GetSetting(settingKeyNetworkProxy)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load network settings"})
			return
		}
		if !ok {
			value = ""
		}
		writeJSON(w, http.StatusOK, map[string]string{"proxy_url": strings.TrimSpace(value)})
	case http.MethodPatch:
		defer r.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		var req updateNetworkSettingsRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		proxyURL := strings.TrimSpace(req.ProxyURL)
		if err := validateProxyURL(proxyURL); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := s.store.SetSetting(settingKeyNetworkProxy, proxyURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save network settings"})
			return
		}
		if err := s.applyNetworkProxy(proxyURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to apply proxy setting"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"proxy_url": proxyURL})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) handleAISettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg, err := s.loadAISettings()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load ai settings"})
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	case http.MethodPatch:
		defer r.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		var req updateAISettingsRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}

		cfg := aiSettings{
			APIKey:     strings.TrimSpace(req.APIKey),
			BaseURL:    strings.TrimSpace(req.BaseURL),
			Model:      strings.TrimSpace(req.Model),
			TargetLang: strings.TrimSpace(req.TargetLang),
		}
		if cfg.BaseURL != "" {
			parsed, err := url.Parse(cfg.BaseURL)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "base_url is invalid"})
				return
			}
		}
		if cfg.TargetLang == "" {
			cfg.TargetLang = defaultAITargetLang
		}

		if err := s.store.SetSetting(settingKeyAIApiKey, cfg.APIKey); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save ai settings"})
			return
		}
		if err := s.store.SetSetting(settingKeyAIBaseURL, cfg.BaseURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save ai settings"})
			return
		}
		if err := s.store.SetSetting(settingKeyAIModel, cfg.Model); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save ai settings"})
			return
		}
		if err := s.store.SetSetting(settingKeyAITargetLang, cfg.TargetLang); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save ai settings"})
			return
		}

		writeJSON(w, http.StatusOK, cfg)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
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
	if fileExists(iconFile) {
		http.ServeFile(w, r, iconFile)
		return
	}
	legacyIconFile := filepath.Join(s.legacyIconDir, filepath.Base(feed.IconPath))
	if fileExists(legacyIconFile) {
		http.ServeFile(w, r, legacyIconFile)
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
	all := s.store.ListArticles()

	limit := 0
	page := 1
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

	if limit <= 0 {
		writeJSON(w, http.StatusOK, map[string]any{"articles": all, "has_more": false})
		return
	}

	start := (page - 1) * limit
	if start >= len(all) {
		writeJSON(w, http.StatusOK, map[string]any{"articles": []any{}, "has_more": false})
		return
	}
	endExclusive := start + limit + 1
	if endExclusive > len(all) {
		endExclusive = len(all)
	}
	window := all[start:endExclusive]
	hasMore := len(window) > limit
	if hasMore {
		window = window[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"articles": window,
		"has_more": hasMore,
	})
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

	if len(parts) == 2 && parts[1] == "readability" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		article, ok := s.store.GetArticle(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			return
		}
		if strings.TrimSpace(article.Link) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "article link is empty"})
			return
		}
		content, err := s.fetchReadableContent(r.Context(), article.Link)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "readability fetch failed: " + err.Error()})
			return
		}
		if err := s.store.UpdateArticleFullContent(id, content); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save extracted content"})
			return
		}
		updated, ok := s.store.GetArticle(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}

	if len(parts) == 2 && parts[1] == "refresh-cache" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		article, ok := s.store.GetArticle(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			return
		}
		// Current cache refresh scope:
		// 1) Readability full_content
		// Future scope can include summary/rating/etc.
		if strings.TrimSpace(article.Link) != "" {
			content, err := s.fetchReadableContent(r.Context(), article.Link)
			if err != nil {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "cache refresh failed: readability " + err.Error()})
				return
			}
			if err := s.store.UpdateArticleFullContent(id, content); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save refreshed readability content"})
				return
			}
		}
		updated, ok := s.store.GetArticle(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}

	if len(parts) == 2 && parts[1] == "translate" {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}

		article, ok := s.store.GetArticle(id)
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

		translatedParts := make([]string, 0, len(paragraphs))
		err = s.translateParagraphs(r.Context(), paragraphs, targetLang, settings, func(_ int, _ int, _ string, translated string) error {
			translatedParts = append(translatedParts, translated)
			return nil
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "translation failed: " + err.Error()})
			return
		}
		translated := strings.Join(translatedParts, "\n\n")
		writeJSON(w, http.StatusOK, map[string]any{
			"article_id":        article.ID,
			"target_lang":       targetLang,
			"translated_text":   translated,
			"source_char_count": len(strings.Join(paragraphs, "\n\n")),
		})
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

func extractArticleTranslationParagraphs(article model.Article) []string {
	candidates := []string{article.FullContent, article.Summary, article.Title}
	for _, raw := range candidates {
		text := strings.TrimSpace(raw)
		if text == "" {
			continue
		}
		if looksLikePDFPayload(text) {
			continue
		}
		paragraphs := splitTranslationParagraphs(text)
		if len(paragraphs) > 0 {
			return paragraphs
		}
	}
	return nil
}

func looksLikePDFPayload(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(strings.ToUpper(trimmed), "%PDF-") {
		return true
	}
	lower := strings.ToLower(trimmed)
	return strings.Contains(lower, "xref") && strings.Contains(lower, "endobj")
}

func stripHTMLTags(raw string) string {
	re := regexp.MustCompile(`(?s)<[^>]*>`)
	stripped := re.ReplaceAllString(raw, " ")
	normalized := html.UnescapeString(stripped)
	return strings.Join(strings.Fields(normalized), " ")
}

func splitTranslationParagraphs(raw string) []string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}

	if strings.Contains(text, "<") && strings.Contains(text, ">") {
		blockBreaks := regexp.MustCompile(`(?is)</(p|div|li|h[1-6]|blockquote|section|article|pre|tr|ul|ol)>|<br\s*/?>`)
		text = blockBreaks.ReplaceAllString(text, "\n\n")
		tags := regexp.MustCompile(`(?s)<[^>]*>`)
		text = tags.ReplaceAllString(text, " ")
		text = html.UnescapeString(text)
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	parts := regexp.MustCompile(`\n\s*\n+`).Split(text, -1)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		normalized := strings.TrimSpace(p)
		if normalized == "" {
			continue
		}
		normalized = strings.Join(strings.Fields(normalized), " ")
		if normalized != "" {
			result = append(result, normalized)
		}
	}

	if len(result) == 0 {
		fallback := strings.TrimSpace(strings.Join(strings.Fields(text), " "))
		if fallback != "" {
			return []string{fallback}
		}
	}
	return result
}

func (s *Server) streamArticleTranslation(w http.ResponseWriter, r *http.Request, articleID int64) {
	article, ok := s.store.GetArticle(articleID)
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

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming is not supported"})
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writer := bufio.NewWriter(w)
	emit := func(event translateStreamEvent) error {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := writer.Write(payload); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	if err := emit(translateStreamEvent{
		Type:       "start",
		ArticleID:  articleID,
		TargetLang: targetLang,
		Total:      len(paragraphs),
		Sources:    paragraphs,
	}); err != nil {
		return
	}

	err = s.translateParagraphs(r.Context(), paragraphs, targetLang, settings, func(index int, total int, source string, translated string) error {
		return emit(translateStreamEvent{
			Type:       "chunk",
			ArticleID:  articleID,
			TargetLang: targetLang,
			Total:      total,
			Index:      index,
			Source:     source,
			Translated: translated,
		})
	})
	if err != nil {
		_ = emit(translateStreamEvent{
			Type:      "error",
			ArticleID: articleID,
			Error:     err.Error(),
		})
		return
	}
	_ = emit(translateStreamEvent{
		Type:      "done",
		ArticleID: articleID,
		Total:     len(paragraphs),
	})
}

func (s *Server) translateTextWithAI(ctx context.Context, text string, targetLang string, settings aiSettings, history []translationPair) (string, error) {
	apiKey := strings.TrimSpace(settings.APIKey)
	if apiKey == "" {
		return "", errors.New("missing AI API key")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(firstNonEmpty(settings.BaseURL, defaultAIBaseURL)), "/")
	model := strings.TrimSpace(firstNonEmpty(settings.Model, defaultAIModel))
	contextText := buildTranslationContext(history)

	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqPayload := map[string]any{
		"model": model,
		"messages": []chatMessage{
			{
				Role:    "system",
				Content: "You are a precise translator. Keep terminology consistent with prior translated context. Return only translated plain text for the current segment without explanations.",
			},
			{
				Role: "user",
				Content: fmt.Sprintf(
					"Translate the CURRENT segment to %s.\nRequirements:\n1) Preserve meaning accurately.\n2) Keep names/terms consistent with prior context.\n3) Output only translated text for CURRENT segment.\n\nPrior translated context (for consistency):\n%s\n\nCURRENT segment:\n%s",
					targetLang,
					contextText,
					text,
				),
			},
		},
		"temperature": 0.2,
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := s.httpClientForAI().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("empty translation response")
	}
	result := strings.TrimSpace(out.Choices[0].Message.Content)
	if result == "" {
		return "", errors.New("empty translated content")
	}
	return result, nil
}

func (s *Server) translateParagraphs(
	ctx context.Context,
	paragraphs []string,
	targetLang string,
	settings aiSettings,
	onChunk func(index int, total int, source string, translated string) error,
) error {
	total := len(paragraphs)
	history := make([]translationPair, 0, total)
	for idx, source := range paragraphs {
		translated, err := s.translateTextWithAI(ctx, source, targetLang, settings, history)
		if err != nil {
			return err
		}
		if err := onChunk(idx+1, total, source, translated); err != nil {
			return err
		}
		history = append(history, translationPair{
			Source:     source,
			Translated: translated,
		})
	}
	return nil
}

func buildTranslationContext(history []translationPair) string {
	if len(history) == 0 {
		return "(none)"
	}
	const (
		maxPairs = 4
		maxChars = 2200
	)
	start := 0
	if len(history) > maxPairs {
		start = len(history) - maxPairs
	}
	var b strings.Builder
	for i := start; i < len(history); i++ {
		pair := history[i]
		if strings.TrimSpace(pair.Source) == "" || strings.TrimSpace(pair.Translated) == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("Segment %d Source:\n%s\nSegment %d Translation:\n%s\n\n", i+1, pair.Source, i+1, pair.Translated))
		if b.Len() > maxChars {
			break
		}
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return "(none)"
	}
	if len(text) > maxChars {
		return text[len(text)-maxChars:]
	}
	return text
}

func (s *Server) loadAISettings() (aiSettings, error) {
	get := func(key string) (string, error) {
		value, ok, err := s.store.GetSetting(key)
		if err != nil {
			return "", err
		}
		if ok {
			return strings.TrimSpace(value), nil
		}
		return "", nil
	}

	apiKey, err := get(settingKeyAIApiKey)
	if err != nil {
		return aiSettings{}, err
	}
	baseURL, err := get(settingKeyAIBaseURL)
	if err != nil {
		return aiSettings{}, err
	}
	model, err := get(settingKeyAIModel)
	if err != nil {
		return aiSettings{}, err
	}
	targetLang, err := get(settingKeyAITargetLang)
	if err != nil {
		return aiSettings{}, err
	}

	if baseURL == "" {
		baseURL = defaultAIBaseURL
	}
	if model == "" {
		model = defaultAIModel
	}
	if targetLang == "" {
		targetLang = defaultAITargetLang
	}

	return aiSettings{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		Model:      model,
		TargetLang: targetLang,
	}, nil
}

func (s *Server) fetchReadableContent(ctx context.Context, rawURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Zflow/0.1 (+https://github.com/Sentixxx/Zflow)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.Contains(contentType, "application/pdf") || bytes.HasPrefix(rawBody, []byte("%PDF-")) {
		return "", errors.New("unsupported readability content type: pdf")
	}
	doc, err := readability.FromReader(bytes.NewReader(rawBody), parsedURL)
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		content = strings.TrimSpace(doc.TextContent)
	}
	if content == "" {
		return "", errors.New("empty readable content")
	}
	return content, nil
}

type fetchResult struct {
	Title        string
	Items        []repository.ArticleSeed
	IconHints    []string
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

	resp, err := s.httpClient().Do(req)
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

	items := make([]repository.ArticleSeed, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		items = append(items, repository.ArticleSeed{
			Title:       item.Title,
			Link:        item.Link,
			Summary:     item.Summary,
			CoverURL:    item.CoverURL,
			PublishedAt: item.PublishedAt,
		})
	}

	return fetchResult{
		Title:        parsed.Title,
		Items:        items,
		IconHints:    parsed.IconHints,
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
			s.logger.Warn("refresh", "feed", "failed", "custom script failed, fallback to raw summary", "feed_id", feed.ID, "error", err.Error())
		} else {
			result.Items = items
		}
	}
	s.tryRefreshFeedIcon(feed.ID, feed.URL, feed.IconPath, feed.IconFetchedAt, result.IconHints)
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

func (s *Server) applyScriptToItems(feedID int64, feedURL, script, lang string, items []repository.ArticleSeed) ([]repository.ArticleSeed, error) {
	out := make([]repository.ArticleSeed, 0, len(items))
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
			s.logger.Warn("refresh", "feed", "failed", "script execution failed", "feed_id", feedID, "item_host", logger.ExtractHost(item.Link), "error", err.Error())
			out = append(out, item)
			continue
		}

		var result scriptResultPayload
		if err := json.Unmarshal(stdout, &result); err != nil {
			s.logger.Warn("refresh", "feed", "failed", "script output is not valid json", "feed_id", feedID, "item_host", logger.ExtractHost(item.Link), "error", err.Error())
			out = append(out, item)
			continue
		}
		if !result.OK {
			if msg := strings.TrimSpace(result.Debug); msg != "" {
				s.logger.Warn("refresh", "feed", "failed", "script returned ok=false", "feed_id", feedID, "item_host", logger.ExtractHost(item.Link), "debug", msg)
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

func (s *Server) tryRefreshFeedIcon(feedID int64, feedURL, existingIconPath, iconFetchedAt string, feedIconHints []string) {
	if s.hasFreshIconAsset(existingIconPath, iconFetchedAt) {
		s.logger.Info("icon", "refresh", "skipped", "icon refresh skipped because local asset is fresh", "feed_id", feedID, "feed_url", feedURL)
		return
	}
	if s.tryReuseIconFromSameHost(feedID, feedURL) {
		return
	}
	iconCandidates := s.discoverIconURLs(feedURL, feedIconHints)
	s.logger.Info("icon", "refresh", "started", "icon refresh started", "feed_id", feedID, "feed_url", feedURL, "candidate_count", len(iconCandidates))
	for idx, iconURL := range iconCandidates {
		iconBytes, ext, err := s.fetchIcon(iconURL)
		if err != nil {
			s.logger.Info("icon", "fetch", "failed", "icon candidate failed", "feed_id", feedID, "candidate_index", idx+1, "candidate_url", iconURL, "error", err.Error())
			continue
		}
		relativePath, err := s.persistIcon(feedURL, iconBytes, ext)
		if err != nil {
			s.logger.Warn("icon", "persist", "failed", "persist icon failed", "feed_id", feedID, "icon_url", iconURL, "error", err.Error())
			return
		}
		if _, _, err := s.store.UpdateFeedIcon(feedID, relativePath); err != nil {
			s.logger.Warn("icon", "update", "failed", "update icon path failed", "feed_id", feedID, "icon_url", iconURL, "error", err.Error())
		}
		s.logger.Info("icon", "refresh", "ok", "icon refresh succeeded", "feed_id", feedID, "icon_url", iconURL, "stored_path", relativePath)
		return
	}
	s.logger.Warn("icon", "refresh", "failed", "icon refresh exhausted candidates", "feed_id", feedID, "feed_url", feedURL, "candidate_count", len(iconCandidates))
}

func (s *Server) tryReuseIconFromSameHost(feedID int64, feedURL string) bool {
	targetHost := logger.ExtractHost(feedURL)
	if strings.TrimSpace(targetHost) == "" {
		return false
	}
	feeds := s.store.List()
	for _, candidate := range feeds {
		if candidate.ID == feedID {
			continue
		}
		if !strings.EqualFold(logger.ExtractHost(candidate.URL), targetHost) {
			continue
		}
		iconPath := strings.TrimSpace(candidate.IconPath)
		if iconPath == "" {
			continue
		}
		if !s.iconAssetExists(iconPath) {
			continue
		}
		if _, _, err := s.store.UpdateFeedIcon(feedID, iconPath); err != nil {
			s.logger.Warn("icon", "reuse", "failed", "reuse icon from same host failed", "feed_id", feedID, "source_feed_id", candidate.ID, "host", targetHost, "icon_path", iconPath, "error", err.Error())
			return false
		}
		s.logger.Info("icon", "reuse", "ok", "reused icon from same host", "feed_id", feedID, "source_feed_id", candidate.ID, "host", targetHost, "icon_path", iconPath)
		return true
	}
	return false
}

func (s *Server) hasFreshIconAsset(existingIconPath, iconFetchedAt string) bool {
	iconPath := strings.TrimSpace(existingIconPath)
	if iconPath == "" {
		return false
	}
	if needsIconRefresh(iconFetchedAt) {
		return false
	}
	return s.iconAssetExists(iconPath)
}

func (s *Server) iconAssetExists(iconPath string) bool {
	iconName := filepath.Base(strings.TrimSpace(iconPath))
	if iconName == "" {
		return false
	}
	current := filepath.Join(s.iconDir, iconName)
	legacy := filepath.Join(s.legacyIconDir, iconName)
	return fileExists(current) || fileExists(legacy)
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

func normalizeOrigin(feedURL string) (string, bool) {
	u, err := url.Parse(feedURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

func (s *Server) discoverIconURLs(feedURL string, feedIconHints []string) []string {
	origin, ok := normalizeOrigin(feedURL)
	if !ok {
		return nil
	}
	candidates := make([]string, 0, 16)
	candidates = append(candidates, normalizeIconHints(feedURL, origin, feedIconHints)...)
	candidates = append(candidates, []string{
		origin + "/favicon.ico",
		origin + "/favicon.png",
		origin + "/favicon.svg",
		origin + "/favicon-32x32.png",
		origin + "/favicon-16x16.png",
		origin + "/static/favicon.ico",
		origin + "/apple-touch-icon.png",
		origin + "/apple-touch-icon-precomposed.png",
	}...)
	candidates = append(candidates, s.discoverIconURLsFromHTML(origin, origin)...)
	if feedURL != origin {
		candidates = append(candidates, s.discoverIconURLsFromHTML(feedURL, origin)...)
	}
	if googleFallback := googleFaviconURL(origin); googleFallback != "" {
		candidates = append(candidates, googleFallback)
	}
	unique := uniqueURLs(candidates)
	s.logger.Info("icon", "discover", "ok", "icon candidates prepared", "feed_url", feedURL, "candidate_count", len(unique), "google_fallback", googleFaviconURL(origin))
	return unique
}

func normalizeIconHints(feedURL, origin string, hints []string) []string {
	if len(hints) == 0 {
		return nil
	}
	base, err := url.Parse(feedURL)
	if err != nil {
		base, _ = url.Parse(origin)
	}
	out := make([]string, 0, len(hints))
	for _, hint := range hints {
		trimmed := strings.TrimSpace(hint)
		if trimmed == "" {
			continue
		}
		u, err := url.Parse(trimmed)
		if err != nil {
			continue
		}
		if base != nil {
			u = base.ResolveReference(u)
		}
		if u.Scheme == "" || u.Host == "" {
			continue
		}
		if !sameHost(u.String(), origin) {
			continue
		}
		out = append(out, u.String())
	}
	return out
}

func googleFaviconURL(origin string) string {
	u, err := url.Parse(origin)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return "https://www.google.com/s2/favicons?sz=128&domain=" + url.QueryEscape(u.Hostname())
}

var (
	reHTMLLinkTag = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	reHrefAttr    = regexp.MustCompile(`(?is)\bhref\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+))`)
	reRelAttr     = regexp.MustCompile(`(?is)\brel\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+))`)
)

func (s *Server) discoverIconURLsFromHTML(pageURL, origin string) []string {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Zflow/0.1 (+https://github.com/Sentixxx/Zflow)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml;q=0.9,*/*;q=0.1")
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil || len(raw) == 0 {
		return nil
	}
	pageBase, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}
	icons := make([]string, 0, 4)
	linkTags := reHTMLLinkTag.FindAllString(string(raw), -1)
	for _, tag := range linkTags {
		rel := firstMatchGroup(reRelAttr, tag)
		if rel == "" || !strings.Contains(strings.ToLower(rel), "icon") {
			continue
		}
		href := firstMatchGroup(reHrefAttr, tag)
		if href == "" {
			continue
		}
		parsedHref, err := url.Parse(strings.TrimSpace(href))
		if err != nil {
			continue
		}
		absURL := pageBase.ResolveReference(parsedHref).String()
		if !sameHost(absURL, origin) {
			continue
		}
		icons = append(icons, absURL)
	}
	return icons
}

func firstMatchGroup(re *regexp.Regexp, input string) string {
	match := re.FindStringSubmatch(input)
	if len(match) == 0 {
		return ""
	}
	for i := 2; i < len(match); i++ {
		if strings.TrimSpace(match[i]) != "" {
			return strings.TrimSpace(match[i])
		}
	}
	return strings.TrimSpace(match[1])
}

func sameHost(rawURL, rawOrigin string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	origin, err := url.Parse(rawOrigin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Hostname(), origin.Hostname())
}

func uniqueURLs(urls []string) []string {
	seen := make(map[string]struct{}, len(urls))
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		trimmed := strings.TrimSpace(u)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func (s *Server) fetchIcon(iconURL string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, iconURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Zflow/0.1 (+https://github.com/Sentixxx/Zflow)")
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	if u, err := url.Parse(iconURL); err == nil && u.Scheme != "" && u.Host != "" {
		req.Header.Set("Referer", u.Scheme+"://"+u.Host+"/")
	}
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", fmt.Errorf("icon fetch failed: %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return nil, "", err
	}
	if len(raw) == 0 {
		return nil, "", errors.New("icon is empty")
	}
	headerType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	detectedType := strings.ToLower(http.DetectContentType(raw))
	if !isIconLikeContent(headerType, detectedType, raw) {
		return nil, "", fmt.Errorf("icon is not image content: header=%q detected=%q", headerType, detectedType)
	}
	ext := iconExt(iconURL, headerType, detectedType, raw)
	return raw, ext, nil
}

func isIconLikeContent(headerType, detectedType string, raw []byte) bool {
	if strings.HasPrefix(strings.Split(headerType, ";")[0], "image/") {
		return true
	}
	if strings.HasPrefix(strings.Split(detectedType, ";")[0], "image/") {
		return true
	}
	return isICO(raw) || looksLikeSVG(raw)
}

func isICO(raw []byte) bool {
	return len(raw) >= 4 && raw[0] == 0x00 && raw[1] == 0x00 && raw[2] == 0x01 && raw[3] == 0x00
}

func looksLikeSVG(raw []byte) bool {
	trimmed := strings.TrimSpace(string(raw))
	return strings.HasPrefix(trimmed, "<svg") || strings.HasPrefix(trimmed, "<?xml")
}

func iconExt(iconURL, headerType, detectedType string, raw []byte) string {
	ext := strings.ToLower(filepath.Ext(iconURL))
	if ext == ".ico" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" || ext == ".svg" {
		if ext == ".jpeg" {
			return ".jpg"
		}
		return ext
	}
	if isICO(raw) {
		return ".ico"
	}
	if looksLikeSVG(raw) {
		return ".svg"
	}
	for _, contentType := range []string{strings.Split(headerType, ";")[0], strings.Split(detectedType, ";")[0]} {
		if strings.TrimSpace(contentType) == "" {
			continue
		}
		if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
			switch exts[0] {
			case ".jpeg":
				return ".jpg"
			default:
				return exts[0]
			}
		}
	}
	return ".ico"
}

func (s *Server) persistIcon(feedURL string, raw []byte, ext string) (string, error) {
	h := fnv.New64a()
	_, _ = h.Write(raw)
	hostPrefix := sanitizeHostPrefix(logger.ExtractHost(feedURL))
	if hostPrefix == "" {
		hostPrefix = "unknown-host"
	}
	fileName := fmt.Sprintf("host-%s-%x%s", hostPrefix, h.Sum64(), ext)
	fullPath := filepath.Join(s.iconDir, fileName)
	if err := os.WriteFile(fullPath, raw, 0o644); err != nil {
		return "", err
	}
	return fileName, nil
}

func sanitizeHostPrefix(host string) string {
	trimmed := strings.ToLower(strings.TrimSpace(host))
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(trimmed))
	for _, ch := range trimmed {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			b.WriteRune(ch)
			continue
		}
		if ch == '.' || ch == '-' || ch == '_' {
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return ""
	}
	if len(result) > 64 {
		return result[:64]
	}
	return result
}

func (s *Server) httpClient() *http.Client {
	s.clientMu.RLock()
	client := s.client
	s.clientMu.RUnlock()
	return client
}

func (s *Server) httpClientForAI() *http.Client {
	base := s.httpClient()
	return &http.Client{
		Timeout:   60 * time.Second,
		Transport: base.Transport,
	}
}

func (s *Server) applyNetworkProxy(rawProxyURL string) error {
	proxyURL := strings.TrimSpace(rawProxyURL)
	if err := validateProxyURL(proxyURL); err != nil {
		return err
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
	}
	if proxyURL != "" {
		parsedProxyURL, _ := url.Parse(proxyURL)
		transport.Proxy = http.ProxyURL(parsedProxyURL)
	}

	client := &http.Client{
		Timeout:   8 * time.Second,
		Transport: transport,
	}

	s.clientMu.Lock()
	s.client = client
	s.clientMu.Unlock()
	return nil
}

func validateProxyURL(raw string) error {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}
	parsed, err := url.Parse(text)
	if err != nil {
		return errors.New("proxy_url is invalid")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "socks5":
	default:
		return errors.New("proxy_url scheme must be http/https/socks5")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return errors.New("proxy_url host is required")
	}
	return nil
}

func (s *Server) RefreshAllFeeds(ctx context.Context) error {
	feeds := s.store.List()
	for _, feed := range feeds {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := s.refreshFeedByID(feed.ID); err != nil {
			s.logger.Warn("refresh", "feed", "failed", "scheduled refresh failed", "feed_id", feed.ID, "error", err.Error())
		}
	}
	return nil
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

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (s *Server) requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{
			ResponseWriter: w,
			status:         http.StatusOK,
		}
		next.ServeHTTP(recorder, r)

		status := recorder.status
		result := "ok"
		if status >= 400 && status < 500 {
			result = "failed"
		} else if status >= 500 {
			result = "failed"
		}

		action := "request"
		switch r.Method {
		case http.MethodGet:
			action = "fetch"
		case http.MethodPost:
			action = "create"
		case http.MethodPatch:
			action = "update"
		case http.MethodDelete:
			action = "delete"
		}

		resource := resourceFromPath(r.URL.Path)
		durationMS := time.Since(start).Milliseconds()
		s.logger.Info(action, resource, result, "http request", "method", r.Method, "path", r.URL.Path, "status_code", status, "duration_ms", durationMS)
	})
}

func resourceFromPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/v1/feeds"):
		return "feed"
	case strings.HasPrefix(path, "/api/v1/folders"):
		return "folder"
	case strings.HasPrefix(path, "/api/v1/articles"):
		return "entry"
	case strings.HasPrefix(path, "/api/v1/icons"):
		return "icon"
	case strings.HasPrefix(path, "/api/v1/data"):
		return "settings"
	default:
		return "http"
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
