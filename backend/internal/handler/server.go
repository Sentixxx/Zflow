package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/internal/service"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

type Server struct {
	store          repository.FeedRepository
	client         *http.Client
	clientMu       sync.RWMutex
	iconDir        string
	allowedOrigins []string
	articleUC      *service.ArticleService
	feedRefreshUC  *service.FeedRefreshService
	summaryUC      *service.ArticleSummaryService
	logger         *logger.ModuleLogger
}

const (
	settingKeyNetworkProxy  = "network_proxy_url"
	settingKeyAIApiKey      = "ai_api_key"
	settingKeyAIProtocol    = "ai_protocol"
	settingKeyAIBaseURL     = "ai_base_url"
	settingKeyAIModel       = "ai_model"
	settingKeyAITargetLang  = "ai_target_lang"
	settingKeyRetentionDays = "article_retention_days"
	defaultAIBaseURL        = "https://api.openai.com/v1"
	defaultAIModel          = "gpt-4o-mini"
	defaultAIProtocol       = "openai"
	defaultAITargetLang     = "zh-CN"
	defaultRetentionDays    = 90
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

type updateAISettingsRequest struct {
	Protocol   string `json:"protocol"`
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	TargetLang string `json:"target_lang"`
}

type translateArticleRequest struct {
	TargetLang string `json:"target_lang"`
}

type aiSettings struct {
	Protocol   string `json:"protocol"`
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

func NewServer(feedStore repository.FeedRepository, dataDir string) *Server {
	iconDir := filepath.Join(dataDir, "feed-icons")
	_ = os.MkdirAll(iconDir, 0o755)
	server := &Server{
		store:          feedStore,
		iconDir:        iconDir,
		allowedOrigins: loadAllowedOriginsFromEnv(),
		logger:         logger.NewModuleFromEnv("handler"),
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
	server.articleUC = service.NewArticleService(feedStore, server.httpClientForReadability)
	server.summaryUC = service.NewArticleSummaryService(
		feedStore,
		server.httpClientForAI,
		func() (service.SummaryAIConfig, error) {
			cfg, err := server.loadAISettings()
			if err != nil {
				return service.SummaryAIConfig{}, err
			}
			return service.SummaryAIConfig{
				Protocol: cfg.Protocol,
				APIKey:   cfg.APIKey,
				BaseURL:  cfg.BaseURL,
				Model:    cfg.Model,
			}, nil
		},
		defaultAIBaseURL,
		defaultAIModel,
	)
	server.feedRefreshUC = service.NewFeedRefreshService(
		feedStore,
		iconDir,
		server.httpClient,
		server.summaryUC.BackfillFeed,
	)
	return server
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/feeds", s.handleFeeds)
	mux.HandleFunc("/api/v1/feeds/", s.handleFeedByID)
	mux.HandleFunc("/api/v1/folders", s.handleFolders)
	mux.HandleFunc("/api/v1/folders/", s.handleFolderByID)
	mux.HandleFunc("/api/v1/articles", s.handleArticles)
	mux.HandleFunc("/api/v1/articles/", s.handleArticleByID)
	mux.HandleFunc("/api/v1/dev/articles/", s.handleDevArticles)
	mux.HandleFunc("/api/v1/icons/", s.handleFeedIcon)
	mux.HandleFunc("/api/v1/data/export/profile", s.handleExportProfile)
	mux.HandleFunc("/api/v1/data/import/profile", s.handleImportProfile)
	mux.HandleFunc("/api/v1/data/export/opml", s.handleExportOPML)
	mux.HandleFunc("/api/v1/data/import/opml", s.handleImportOPML)
	mux.HandleFunc("/api/v1/settings/network", s.handleNetworkSettings)
	mux.HandleFunc("/api/v1/settings/ai", s.handleAISettings)
	mux.HandleFunc("/api/v1/settings/data", s.handleDataSettings)
	mux.HandleFunc("/api/v1/settings/data/regenerate-summaries", s.handleSummaryRegeneration)
	mux.HandleFunc("/healthz", s.handleHealth)
}

func (s *Server) WrapHTTPHandler(next http.Handler) http.Handler {
	return s.corsMiddleware(s.requestLogMiddleware(next))
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	return s.WrapHTTPHandler(mux)
}

func (s *Server) ArticleService() *service.ArticleService {
	return s.articleUC
}

func (s *Server) FeedRefreshService() *service.FeedRefreshService {
	return s.feedRefreshUC
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

func (s *Server) httpClientForReadability() *http.Client {
	base := s.httpClient()
	return &http.Client{
		Timeout:   25 * time.Second,
		Transport: base.Transport,
	}
}

func (s *Server) applyNetworkProxy(rawProxyURL string) error {
	proxyURL := strings.TrimSpace(rawProxyURL)
	if err := validateProxyURL(proxyURL); err != nil {
		return err
	}

	var parsedProxyURL *url.URL
	if proxyURL != "" {
		parsedProxyURL, _ = url.Parse(proxyURL)
	}

	transport := &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			if shouldBypassProxy(req.URL) {
				return nil, nil
			}
			if parsedProxyURL != nil {
				return parsedProxyURL, nil
			}
			return http.ProxyFromEnvironment(req)
		},
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

func shouldBypassProxy(target *url.URL) bool {
	if target == nil {
		return true
	}
	host := strings.TrimSpace(target.Hostname())
	if host == "" {
		return true
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
	return s.feedRefreshUC.RefreshAllFeeds(ctx)
}

func (s *Server) loadRetentionDays() (int, error) {
	value, ok, err := s.store.GetSetting(settingKeyRetentionDays)
	if err != nil {
		return 0, err
	}
	if !ok || strings.TrimSpace(value) == "" {
		return defaultRetentionDays, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return defaultRetentionDays, nil
	}
	return parsed, nil
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

func loadAllowedOriginsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("ZFLOW_ALLOWED_ORIGINS"))
	if raw == "" {
		return []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"http://localhost:4173",
			"http://127.0.0.1:4173",
			"http://localhost:5173",
			"http://127.0.0.1:5173",
		}
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	return origins
}

func (s *Server) allowOrigin(origin string) string {
	trimmed := strings.TrimSpace(origin)
	if trimmed == "" {
		return ""
	}
	for _, allowed := range s.allowedOrigins {
		if allowed == "*" || strings.EqualFold(strings.TrimSpace(allowed), trimmed) {
			return trimmed
		}
	}
	return ""
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowed := s.allowOrigin(r.Header.Get("Origin")); allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			w.Header().Set("Vary", "Origin")
		}
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
	case strings.HasPrefix(path, "/api/v1/dev"):
		return "settings"
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
