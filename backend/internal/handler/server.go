package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	vectorStore    repository.VectorRepository
	agentStore     repository.AgentRepository
	client         *http.Client
	clientMu       sync.RWMutex
	iconDir        string
	allowedOrigins []string
	articleUC      *service.ArticleService
	feedRefreshUC  *service.FeedRefreshService
	summaryUC      *service.ArticleSummaryService
	embeddingUC    *service.EmbeddingService
	logger         *logger.ModuleLogger
}

const (
	settingKeyNetworkProxy  = "network_proxy_url"
	settingKeyAIApiKey      = "ai_api_key"
	settingKeyAIProtocol    = "ai_protocol"
	settingKeyAIBaseURL     = "ai_base_url"
	settingKeyAIModel       = "ai_model"
	settingKeyAITargetLang  = "ai_target_lang"
	settingKeyEmbeddingAPIKey  = "embedding_api_key"
	settingKeyEmbeddingBaseURL = "embedding_base_url"
	settingKeyEmbeddingModel   = "embedding_model"
	settingKeyRetentionDays = "article_retention_days"
	defaultAIBaseURL        = "https://api.openai.com/v1"
	defaultAIModel          = "gpt-4o-mini"
	defaultAIProtocol       = "openai"
	defaultAITargetLang     = "zh-CN"
	defaultRetentionDays    = 7
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
	Protocol         string `json:"protocol"`
	APIKey           string `json:"api_key"`
	BaseURL          string `json:"base_url"`
	Model            string `json:"model"`
	TargetLang       string `json:"target_lang"`
	EmbeddingAPIKey  string `json:"embedding_api_key"`
	EmbeddingBaseURL string `json:"embedding_base_url"`
	EmbeddingModel   string `json:"embedding_model"`
}

type translateArticleRequest struct {
	TargetLang string   `json:"target_lang"`
	Sources    []string `json:"sources"`
}

type aiSettings struct {
	Protocol   string `json:"protocol"`
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	TargetLang string `json:"target_lang"`
	Embedding  embeddingSettings `json:"embedding"`
}

type embeddingSettings struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

type translationPair struct {
	Source     string
	Translated string
}

// articleContext carries article-level metadata derived from the summary
// pipeline to give the translator global awareness of the article's topic.
type articleContext struct {
	Title     string
	AISummary string
	FeedTitle string
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

func NewServer(feedStore repository.FeedRepository, dataDir string, opts ...ServerOption) *Server {
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
	server.articleUC = service.NewArticleService(feedStore, server.httpClientForReadability,
		service.WithScoringAI(server.httpClientForAI, func() (service.ScoringAIConfig, error) {
			cfg, err := server.loadAISettings()
			if err != nil {
				return service.ScoringAIConfig{}, err
			}
			return service.ScoringAIConfig{
				Protocol: cfg.Protocol,
				APIKey:   cfg.APIKey,
				BaseURL:  cfg.BaseURL,
				Model:    cfg.Model,
			}, nil
		}),
	)
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
	for _, opt := range opts {
		opt(server)
	}
	return server
}

type ServerOption func(*Server)

func WithVectorRepository(vr repository.VectorRepository) ServerOption {
	return func(s *Server) {
		s.vectorStore = vr
		s.embeddingUC = service.NewEmbeddingService(
			vr,
			s.store,
			s.httpClientForAI,
			func() (service.EmbeddingConfig, error) {
				cfg, err := s.loadEmbeddingSettings()
				if err != nil {
					return service.EmbeddingConfig{}, err
				}
				return service.EmbeddingConfig{
					APIKey:  cfg.APIKey,
					BaseURL: cfg.BaseURL,
					Model:   cfg.Model,
				}, nil
			},
		)
	}
}

func (s *Server) EmbeddingService() *service.EmbeddingService {
	return s.embeddingUC
}

func WithAgentRepository(ar repository.AgentRepository) ServerOption {
	return func(s *Server) {
		s.agentStore = ar
	}
}

func (s *Server) AgentRepository() repository.AgentRepository {
	return s.agentStore
}

func (s *Server) VectorRepository() repository.VectorRepository {
	return s.vectorStore
}

func (s *Server) FeedRepository() repository.FeedRepository {
	return s.store
}

// LLMCallFunc returns a function that calls the LLM chat completions API.
func (s *Server) LLMCallFunc() func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
		cfg, err := s.loadAISettings()
		if err != nil {
			return "", err
		}
		if cfg.APIKey == "" {
			return "", errors.New("AI API key not configured")
		}
		return s.callChatCompletion(ctx, cfg, systemPrompt, userPrompt)
	}
}

func (s *Server) callChatCompletion(ctx context.Context, cfg aiSettings, systemPrompt, userPrompt string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultAIBaseURL
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultAIModel
	}

	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := struct {
		Model    string        `json:"model"`
		Messages []chatMessage `json:"messages"`
	}{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := s.httpClientForAI().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("chat API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", errors.New("chat API returned no choices")
	}
	return result.Choices[0].Message.Content, nil
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
	mux.HandleFunc("/api/v1/topics", s.handleTopics)
	mux.HandleFunc("/api/v1/topics/", s.handleTopicByID)
	mux.HandleFunc("/api/v1/briefs", s.handleBriefs)
	mux.HandleFunc("/api/v1/briefs/", s.handleBriefByID)
	mux.HandleFunc("/api/v1/interests", s.handleInterests)
	mux.HandleFunc("/api/v1/interests/", s.handleInterestByID)
	mux.HandleFunc("/api/v1/agents/runs", s.handleAgentRuns)
	mux.HandleFunc("/api/v1/agents/trigger/", s.handleAgentTrigger)
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

func (s *Server) allowOrigin(origin string, r *http.Request) string {
	trimmed := strings.TrimSpace(origin)
	if trimmed == "" {
		return ""
	}
	for _, allowed := range s.allowedOrigins {
		if allowed == "*" || strings.EqualFold(strings.TrimSpace(allowed), trimmed) {
			return trimmed
		}
	}
	if sameOriginHost(trimmed, requestHost(r)) {
		return trimmed
	}
	return ""
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if allowed := s.allowOrigin(r.Header.Get("Origin"), r); allowed != "" {
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

func requestHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if host == "" && r.URL != nil {
		host = strings.TrimSpace(r.URL.Host)
	}
	if host == "" {
		return ""
	}
	parsedHost, _, err := net.SplitHostPort(host)
	if err == nil {
		return strings.ToLower(strings.TrimSpace(parsedHost))
	}
	return strings.ToLower(strings.TrimSpace(host))
}

func sameOriginHost(origin string, host string) bool {
	if strings.TrimSpace(origin) == "" || strings.TrimSpace(host) == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || strings.TrimSpace(u.Hostname()) == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(u.Hostname()), strings.TrimSpace(host))
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
