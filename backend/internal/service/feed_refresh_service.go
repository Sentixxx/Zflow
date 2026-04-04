package service

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
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/feedparser"
	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

const (
	feedRefreshTimeout         = 20 * time.Second
	feedScriptOutputLimitBytes = 1 << 20
	defaultRetentionDays       = 90
	retentionDaysSettingKey    = "article_retention_days"
)

var ErrFeedNotFound = errors.New("feed not found")

type FeedCreateInput struct {
	URL        string
	FolderID   *int64
	Script     string
	ScriptLang string
}

type FeedRefreshService struct {
	store           repository.FeedRepository
	iconDir         string
	httpClient      func() *http.Client
	summaryBackfill func(feedID int64, limit int) error
	logger          *logger.ModuleLogger
}

type feedFetchResult struct {
	Title        string
	Items        []repository.ArticleSeed
	IconHints    []string
	ETag         string
	LastModified string
	NotModified  bool
	Error        string
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

type cappedBuffer struct {
	limit int
	buf   bytes.Buffer
}

func NewFeedRefreshService(
	store repository.FeedRepository,
	iconDir string,
	httpClient func() *http.Client,
	summaryBackfill func(feedID int64, limit int) error,
) *FeedRefreshService {
	_ = os.MkdirAll(iconDir, 0o755)
	return &FeedRefreshService{
		store:           store,
		iconDir:         iconDir,
		httpClient:      httpClient,
		summaryBackfill: summaryBackfill,
		logger:          logger.NewModuleFromEnv("service"),
	}
}

func NormalizeFeedScriptLang(raw string) string {
	lang := strings.ToLower(strings.TrimSpace(raw))
	if lang == "" {
		return "shell"
	}
	return lang
}

func IsSupportedFeedScriptLang(lang string) bool {
	switch NormalizeFeedScriptLang(lang) {
	case "shell", "python", "javascript":
		return true
	default:
		return false
	}
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.limit > 0 && b.buf.Len()+len(p) > b.limit {
		remaining := b.limit - b.buf.Len()
		if remaining > 0 {
			_, _ = b.buf.Write(p[:remaining])
		}
		return len(p), errors.New("output too large")
	}
	return b.buf.Write(p)
}

func (b *cappedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

func (s *FeedRefreshService) CreateFeed(parent context.Context, input FeedCreateInput) (model.Feed, error) {
	ctx, cancel := context.WithTimeout(parent, feedRefreshTimeout)
	defer cancel()

	result := s.fetchAndParse(ctx, input.URL, "", "")
	result.Items = AttachRecommendationScoresToSeeds(result.Items)
	feed, err := s.store.AddInFolder(input.URL, result.Title, result.Items, result.Error, input.FolderID, result.ETag, result.LastModified)
	if err != nil {
		return model.Feed{}, err
	}
	if script := strings.TrimSpace(input.Script); script != "" {
		lang := NormalizeFeedScriptLang(input.ScriptLang)
		updated, ok, err := s.store.UpdateFeedScript(feed.ID, script, lang)
		if err == nil && ok {
			feed = updated
		}
	}
	if s.summaryBackfill != nil {
		if err := s.summaryBackfill(feed.ID, 50); err != nil {
			s.logger.Warn("summary", "backfill", "failed", "display summary backfill failed after create feed", "feed_id", feed.ID, "error", err.Error())
		}
	}
	s.tryRefreshFeedIcon(ctx, feed.ID, feed.URL, feed.IconPath, feed.IconFetchedAt, result.IconHints)
	return feed, nil
}

func (s *FeedRefreshService) RefreshFeed(parent context.Context, feedID int64) error {
	ctx, cancel := context.WithTimeout(parent, feedRefreshTimeout)
	defer cancel()

	feed, ok, err := s.store.GetFeed(feedID)
	if err != nil {
		return fmt.Errorf("failed to load feed: %w", err)
	}
	if !ok {
		return ErrFeedNotFound
	}

	result := s.fetchAndParse(ctx, feed.URL, feed.ETag, feed.LastModified)
	if result.NotModified {
		return s.store.UpdateFeedAfterRefresh(feedID, feed.Title, nil, "", result.ETag, result.LastModified)
	}
	if result.Error != "" {
		return s.store.UpdateFeedAfterRefresh(feedID, feed.Title, nil, result.Error, feed.ETag, feed.LastModified)
	}
	if script := strings.TrimSpace(feed.CustomScript); script != "" {
		items, err := s.applyScriptToItems(ctx, feed.ID, feed.URL, script, NormalizeFeedScriptLang(feed.CustomScriptLang), result.Items)
		if err != nil {
			s.logger.Warn("refresh", "feed", "failed", "custom script failed, fallback to raw summary", "feed_id", feed.ID, "error", err.Error())
		} else {
			result.Items = items
		}
	}
	result.Items = AttachRecommendationScoresToSeeds(result.Items)
	s.tryRefreshFeedIcon(ctx, feed.ID, feed.URL, feed.IconPath, feed.IconFetchedAt, result.IconHints)
	if err := s.store.UpdateFeedAfterRefresh(feedID, result.Title, result.Items, "", result.ETag, result.LastModified); err != nil {
		return err
	}
	if s.summaryBackfill != nil {
		if err := s.summaryBackfill(feedID, len(result.Items)+10); err != nil {
			s.logger.Warn("summary", "backfill", "failed", "display summary backfill failed after refresh feed", "feed_id", feedID, "error", err.Error())
		}
	}
	return nil
}

func (s *FeedRefreshService) RefreshAllFeeds(ctx context.Context) error {
	feeds := s.store.List()
	for _, feed := range feeds {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := s.RefreshFeed(ctx, feed.ID); err != nil {
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

func RunFeedScript(parent context.Context, lang, script string, stdin []byte) ([]byte, error) {
	return runFeedScript(parent, lang, script, stdin)
}

func (s *FeedRefreshService) fetchAndParse(ctx context.Context, feedURL, etag, lastModified string) feedFetchResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return feedFetchResult{Error: err.Error()}
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
		return feedFetchResult{Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return feedFetchResult{
			NotModified:  true,
			ETag:         pickHeaderOrDefault(resp.Header.Get("ETag"), etag),
			LastModified: pickHeaderOrDefault(resp.Header.Get("Last-Modified"), lastModified),
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return feedFetchResult{Error: fmt.Sprintf("fetch failed: http %d", resp.StatusCode)}
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return feedFetchResult{Error: err.Error()}
	}
	parsed, err := feedparser.ParseFeed(raw)
	if err != nil {
		return feedFetchResult{Error: err.Error()}
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
	return feedFetchResult{
		Title:        parsed.Title,
		Items:        items,
		IconHints:    parsed.IconHints,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}
}

func (s *FeedRefreshService) applyScriptToItems(ctx context.Context, feedID int64, feedURL, script, lang string, items []repository.ArticleSeed) ([]repository.ArticleSeed, error) {
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
		stdout, err := runFeedScript(ctx, lang, script, raw)
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

func runFeedScript(parent context.Context, lang, script string, stdin []byte) ([]byte, error) {
	// NOTE: User scripts are intentionally powerful and currently run without OS-level sandboxing.
	// The only enforced guards here are timeout and bounded stdout capture; treat this as trusted-local automation, not an isolation boundary.
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch NormalizeFeedScriptLang(lang) {
	case "python":
		cmd = exec.CommandContext(ctx, "python3", "-c", script)
	case "javascript":
		cmd = exec.CommandContext(ctx, "node", "-e", script)
	default:
		cmd = exec.CommandContext(ctx, "/bin/sh", "-lc", script)
	}
	cmd.Stdin = bytes.NewReader(stdin)
	stdout := &cappedBuffer{limit: feedScriptOutputLimitBytes}
	var stderr bytes.Buffer
	cmd.Stdout = stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText == "" {
			errText = err.Error()
		}
		return nil, fmt.Errorf("script run failed: %s", errText)
	}
	return stdout.Bytes(), nil
}

func (s *FeedRefreshService) tryRefreshFeedIcon(ctx context.Context, feedID int64, feedURL, existingIconPath, iconFetchedAt string, feedIconHints []string) {
	if s.hasFreshIconAsset(existingIconPath, iconFetchedAt) {
		s.logger.Info("icon", "refresh", "skipped", "icon refresh skipped because local asset is fresh", "feed_id", feedID, "feed_url", feedURL)
		return
	}
	if s.tryReuseIconFromSameHost(feedID, feedURL) {
		return
	}
	iconCandidates := s.discoverIconURLs(ctx, feedURL, feedIconHints)
	s.logger.Info("icon", "refresh", "started", "icon refresh started", "feed_id", feedID, "feed_url", feedURL, "candidate_count", len(iconCandidates))
	for idx, iconURL := range iconCandidates {
		iconBytes, ext, err := s.fetchIcon(ctx, iconURL)
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

func (s *FeedRefreshService) tryReuseIconFromSameHost(feedID int64, feedURL string) bool {
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

func (s *FeedRefreshService) hasFreshIconAsset(existingIconPath, iconFetchedAt string) bool {
	iconPath := strings.TrimSpace(existingIconPath)
	if iconPath == "" {
		return false
	}
	if needsIconRefresh(iconFetchedAt) {
		return false
	}
	return s.iconAssetExists(iconPath)
}

func (s *FeedRefreshService) iconAssetExists(iconPath string) bool {
	iconName := filepath.Base(strings.TrimSpace(iconPath))
	if iconName == "" {
		return false
	}
	current := filepath.Join(s.iconDir, iconName)
	return fileExists(current)
}

func (s *FeedRefreshService) discoverIconURLs(ctx context.Context, feedURL string, feedIconHints []string) []string {
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
	candidates = append(candidates, s.discoverIconURLsFromHTML(ctx, origin, origin)...)
	if feedURL != origin {
		candidates = append(candidates, s.discoverIconURLsFromHTML(ctx, feedURL, origin)...)
	}
	if googleFallback := googleFaviconURL(origin); googleFallback != "" {
		candidates = append(candidates, googleFallback)
	}
	unique := uniqueURLs(candidates)
	s.logger.Info("icon", "discover", "ok", "icon candidates prepared", "feed_url", feedURL, "candidate_count", len(unique), "google_fallback", googleFaviconURL(origin))
	return unique
}

var (
	reHTMLLinkTag = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	reHrefAttr    = regexp.MustCompile(`(?is)\bhref\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+))`)
	reRelAttr     = regexp.MustCompile(`(?is)\brel\s*=\s*("([^"]*)"|'([^']*)'|([^\s"'=<>` + "`" + `]+))`)
)

func (s *FeedRefreshService) discoverIconURLsFromHTML(ctx context.Context, pageURL, origin string) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
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

func (s *FeedRefreshService) fetchIcon(ctx context.Context, iconURL string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, iconURL, nil)
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
	return raw, iconExt(iconURL, headerType, detectedType, raw), nil
}

func (s *FeedRefreshService) persistIcon(feedURL string, raw []byte, ext string) (string, error) {
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

func (s *FeedRefreshService) loadRetentionDays() (int, error) {
	value, ok, err := s.store.GetSetting(retentionDaysSettingKey)
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

func pickHeaderOrDefault(current, fallback string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	return fallback
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
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
	for _, candidate := range urls {
		trimmed := strings.TrimSpace(candidate)
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
