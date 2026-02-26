package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
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
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/internal/service"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

type Server struct {
	store     repository.FeedRepository
	client    *http.Client
	clientMu  sync.RWMutex
	iconDir   string
	articleUC *service.ArticleService
	logger    *logger.ModuleLogger
}

const (
	settingKeyNetworkProxy  = "network_proxy_url"
	settingKeyAIApiKey      = "ai_api_key"
	settingKeyAIBaseURL     = "ai_base_url"
	settingKeyAIModel       = "ai_model"
	settingKeyAITargetLang  = "ai_target_lang"
	settingKeyRetentionDays = "article_retention_days"
	defaultAIBaseURL        = "https://api.openai.com/v1"
	defaultAIModel          = "gpt-4o-mini"
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

func NewServer(feedStore repository.FeedRepository, dataDir string) *Server {
	iconDir := filepath.Join(dataDir, "feed-icons")
	_ = os.MkdirAll(iconDir, 0o755)
	server := &Server{
		store:   feedStore,
		iconDir: iconDir,
		logger:  logger.NewModuleFromEnv("handler"),
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
	server.articleUC = service.NewArticleService(feedStore, server.httpClient)
	return server
}

func (s *Server) RegisterRoutes(mux *http.ServeMux) {
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
	mux.HandleFunc("/api/v1/settings/data", s.handleDataSettings)
	mux.HandleFunc("/healthz", s.handleHealth)
}

func (s *Server) WrapHTTPHandler(next http.Handler) http.Handler {
	return corsMiddleware(s.requestLogMiddleware(next))
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.RegisterRoutes(mux)
	return s.WrapHTTPHandler(mux)
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
	return fileExists(current)
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
	retentionDays, err := s.loadRetentionDays()
	if err != nil {
		s.logger.Warn("cleanup", "article", "failed", "load retention days failed", "error", err.Error())
		return nil
	}
	deleted, err := s.store.PurgeExpiredArticles(retentionDays)
	if err != nil {
		s.logger.Warn("cleanup", "article", "failed", "purge expired articles failed", "retention_days", retentionDays, "error", err.Error())
		return nil
	}
	if deleted > 0 {
		s.logger.Info("cleanup", "article", "ok", "expired articles purged", "deleted_count", deleted, "retention_days", retentionDays)
	}
	return nil
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
