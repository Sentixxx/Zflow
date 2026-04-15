package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/repository"
)

func TestFeedRefreshServiceFetchAndParseHonorsCancelledContext(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	svc := NewFeedRefreshService(repo, t.TempDir(), func() *http.Client { return http.DefaultClient }, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := svc.fetchAndParse(ctx, "https://example.com/feed.xml", "", "")
	if !strings.Contains(result.Error, "context canceled") {
		t.Fatalf("fetchAndParse error = %q, want context canceled", result.Error)
	}
}

func TestRunFeedScriptRejectsOversizedStdout(t *testing.T) {
	_, err := RunFeedScript(context.Background(), "python", "import sys; sys.stdout.write('a' * (1024 * 1024 + 1))", nil)
	if err == nil {
		t.Fatal("RunFeedScript() error = nil, want output too large")
	}
	if !strings.Contains(err.Error(), "output too large") && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("RunFeedScript() error = %v, want output too large", err)
	}
}

func TestFeedRefreshServiceRefreshFeedFallsBackToRawItemsWhenScriptFails(t *testing.T) {
	feedXML := newFeedXMLServer(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Fresh Feed</title>
    <item>
      <title>First Item</title>
      <link>https://example.com/items/1</link>
      <description><![CDATA[<p>raw summary</p>]]></description>
      <pubDate>Wed, 25 Feb 2001 10:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`)

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	feed, err := repo.AddInFolder(feedXML.URL, "Stale Feed", nil, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	if _, _, err := repo.UpdateFeedScript(feed.ID, "exit 1", "shell"); err != nil {
		t.Fatalf("UpdateFeedScript() error = %v", err)
	}

	backfillCalls := 0
	svc := NewFeedRefreshService(repo, t.TempDir(), func() *http.Client { return feedXML.Client() }, func(feedID int64, limit int) error {
		backfillCalls++
		if feedID != feed.ID {
			t.Fatalf("summary backfill feedID = %d, want %d", feedID, feed.ID)
		}
		if limit != 11 {
			t.Fatalf("summary backfill limit = %d, want 11", limit)
		}
		return nil
	})

	if err := svc.RefreshFeed(context.Background(), feed.ID); err != nil {
		t.Fatalf("RefreshFeed() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles() len = %d, want 1", len(articles))
	}
	if got := strings.TrimSpace(articles[0].Summary); got != "<p>raw summary</p>" {
		t.Fatalf("article summary = %q, want raw fallback summary", got)
	}
	if backfillCalls != 1 {
		t.Fatalf("summary backfill calls = %d, want 1", backfillCalls)
	}
}

func TestFeedRefreshServiceRefreshAllFeedsPurgesExpiredArticles(t *testing.T) {
	feedXML := newFeedXMLServer(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Fresh Feed</title>
    <item>
      <title>First Item</title>
      <link>https://example.com/items/1</link>
      <description><![CDATA[<p>raw summary</p>]]></description>
      <pubDate>Wed, 25 Feb 2001 10:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`)

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	feed, err := repo.AddInFolder(feedXML.URL, "Stale Feed", []repository.ArticleSeed{{
		Title:       "Old Item",
		Link:        "https://example.com/old",
		Summary:     "<p>old summary</p>",
		PublishedAt: "Wed, 25 Feb 2001 10:00:00 GMT",
	}}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	if err := repo.SetSetting("article_retention_days", "1"); err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles() len before favorite = %d, want 1", len(articles))
	}
	if _, _, err := repo.MarkArticleFavorite(articles[0].ID, true); err != nil {
		t.Fatalf("MarkArticleFavorite() error = %v", err)
	}

	svc := NewFeedRefreshService(repo, t.TempDir(), func() *http.Client { return feedXML.Client() }, func(feedID int64, limit int) error { return nil })
	if err := svc.RefreshAllFeeds(context.Background()); err != nil {
		t.Fatalf("RefreshAllFeeds() error = %v", err)
	}

	gotFeed, ok, err := repo.GetFeed(feed.ID)
	if err != nil || !ok {
		t.Fatalf("GetFeed() ok=%v err=%v, want ok", ok, err)
	}
	if got := strings.TrimSpace(gotFeed.Title); got != "Fresh Feed" {
		t.Fatalf("feed title = %q, want %q", got, "Fresh Feed")
	}

	after := repo.ListArticles()
	if len(after) != 1 {
		t.Fatalf("ListArticles() len after refresh = %d, want 1", len(after))
	}
	if !after[0].IsFavorite {
		t.Fatalf("remaining article IsFavorite = false, want true")
	}
}

func TestPurgeExpiredArticlesUsesFeedRetentionOverrideAndZeroFallback(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	overrideFeed, err := repo.AddInFolder("https://example.com/override.xml", "Override Feed", []repository.ArticleSeed{{
		Title:       "Override Old",
		Link:        "https://example.com/override-old",
		Summary:     "<p>old summary</p>",
		PublishedAt: time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339),
	}}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder(override) error = %v", err)
	}
	globalFeed, err := repo.AddInFolder("https://example.com/global.xml", "Global Feed", []repository.ArticleSeed{{
		Title:       "Global Old",
		Link:        "https://example.com/global-old",
		Summary:     "<p>old summary</p>",
		PublishedAt: time.Now().UTC().Add(-10 * 24 * time.Hour).Format(time.RFC3339),
	}}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder(global) error = %v", err)
	}
	if err := repo.SetSetting("article_retention_days", "1"); err != nil {
		t.Fatalf("SetSetting() error = %v", err)
	}
	if _, ok, err := repo.UpdateFeedRetentionDays(overrideFeed.ID, 30); err != nil || !ok {
		t.Fatalf("UpdateFeedRetentionDays(override) ok=%v err=%v, want ok", ok, err)
	}
	if _, ok, err := repo.UpdateFeedRetentionDays(globalFeed.ID, 0); err != nil || !ok {
		t.Fatalf("UpdateFeedRetentionDays(global) ok=%v err=%v, want ok", ok, err)
	}

	deleted, err := repo.PurgeExpiredArticles(1)
	if err != nil {
		t.Fatalf("PurgeExpiredArticles() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles() len = %d, want 1", len(articles))
	}
	if articles[0].FeedID != overrideFeed.ID {
		t.Fatalf("remaining article feed_id = %d, want %d", articles[0].FeedID, overrideFeed.ID)
	}

	gotOverride, ok, err := repo.GetFeed(overrideFeed.ID)
	if err != nil || !ok {
		t.Fatalf("GetFeed(override) ok=%v err=%v, want ok", ok, err)
	}
	if gotOverride.ItemCount != 1 {
		t.Fatalf("override item_count = %d, want 1", gotOverride.ItemCount)
	}

	gotGlobal, ok, err := repo.GetFeed(globalFeed.ID)
	if err != nil || !ok {
		t.Fatalf("GetFeed(global) ok=%v err=%v, want ok", ok, err)
	}
	if gotGlobal.ItemCount != 0 {
		t.Fatalf("global item_count = %d, want 0", gotGlobal.ItemCount)
	}
}

func newFeedXMLServer(t *testing.T, payload string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(payload))
	}))
}

// --- Conditional GET (ETag / 304) ---

// TestFeedRefreshService_When_Server304_Should_NotModifyExistingArticles verifies that
// when the server returns HTTP 304, the service does not overwrite existing articles.
func TestFeedRefreshService_When_Server304_Should_NotModifyExistingArticles(t *testing.T) {
	const storedETag = `"abc123"`

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == storedETag {
			// Honour the conditional request — nothing has changed.
			w.Header().Set("ETag", storedETag)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		// First fetch: return fresh content.
		w.Header().Set("Content-Type", "application/rss+xml")
		w.Header().Set("ETag", storedETag)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Conditional Feed</title>
    <item>
      <title>Original Item</title>
      <link>https://example.com/items/cond-1</link>
      <description><![CDATA[<p>original summary</p>]]></description>
      <pubDate>Mon, 14 Apr 2026 10:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	t.Cleanup(feedXML.Close)

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "cond.db"))
	if err != nil {
		t.Fatalf("NewTestSQLiteFeedRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	svc := NewFeedRefreshService(repo, t.TempDir(), func() *http.Client { return feedXML.Client() }, nil)

	// First refresh — populates feed + article
	if err := svc.RefreshFeed(context.Background(), func() int64 {
		feed, ferr := repo.AddInFolder(feedXML.URL, "", nil, "", nil, "", "")
		if ferr != nil {
			t.Fatalf("AddInFolder() error = %v", ferr)
		}
		return feed.ID
	}()); err != nil {
		t.Fatalf("first RefreshFeed() error = %v", err)
	}

	articlesAfterFirst := repo.ListArticles()
	if len(articlesAfterFirst) != 1 {
		t.Fatalf("articles after first refresh = %d, want 1", len(articlesAfterFirst))
	}

	// Store the ETag so the next refresh sends If-None-Match
	feedList := repo.List()
	if len(feedList) == 0 {
		t.Fatal("no feeds stored after first refresh")
	}
	feedID := feedList[0].ID
	if feedList[0].ETag != storedETag {
		t.Fatalf("stored ETag = %q, want %q", feedList[0].ETag, storedETag)
	}

	// Second refresh — server responds 304, articles must remain unchanged
	if err := svc.RefreshFeed(context.Background(), feedID); err != nil {
		t.Fatalf("second RefreshFeed() error = %v", err)
	}

	articlesAfterSecond := repo.ListArticles()
	if len(articlesAfterSecond) != 1 {
		t.Fatalf("articles after 304 refresh = %d, want unchanged (1)", len(articlesAfterSecond))
	}
	if articlesAfterSecond[0].Title != articlesAfterFirst[0].Title {
		t.Fatalf("article title changed after 304: got %q, want %q",
			articlesAfterSecond[0].Title, articlesAfterFirst[0].Title)
	}
}

// TestFeedRefreshService_When_Server304WithNoNewETag_Should_PreserveOldETag verifies that
// when the 304 response omits the ETag header, the existing ETag is preserved (not cleared).
func TestFeedRefreshService_When_Server304WithNoNewETag_Should_PreserveOldETag(t *testing.T) {
	const storedETag = `"etag-preserved"`
	callCount := 0
	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: serve content with ETag
			w.Header().Set("Content-Type", "application/rss+xml")
			w.Header().Set("ETag", storedETag)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Etag Feed</title>
<item><title>Item</title><link>https://example.com/x</link><pubDate>Mon, 14 Apr 2026 10:00:00 GMT</pubDate></item>
</channel></rss>`))
			return
		}
		// Subsequent calls: 304 with no ETag header (server omits it)
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(feedXML.Close)

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "etag.db"))
	if err != nil {
		t.Fatalf("NewTestSQLiteFeedRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	feed, err := repo.AddInFolder(feedXML.URL, "", nil, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewFeedRefreshService(repo, t.TempDir(), func() *http.Client { return feedXML.Client() }, nil)

	// First refresh: store ETag
	if err := svc.RefreshFeed(context.Background(), feed.ID); err != nil {
		t.Fatalf("first RefreshFeed() error = %v", err)
	}
	storedFeed, ok, _ := repo.GetFeed(feed.ID)
	if !ok || storedFeed.ETag != storedETag {
		t.Fatalf("stored ETag after first refresh = %q, want %q", storedFeed.ETag, storedETag)
	}

	// Second refresh: 304, no ETag header — existing ETag must be preserved
	if err := svc.RefreshFeed(context.Background(), feed.ID); err != nil {
		t.Fatalf("second RefreshFeed() error = %v", err)
	}
	updatedFeed, ok, _ := repo.GetFeed(feed.ID)
	if !ok || updatedFeed.ETag != storedETag {
		t.Fatalf("ETag after 304 without header = %q, want preserved %q", updatedFeed.ETag, storedETag)
	}
}

// TestFeedRefreshService_When_ScriptReturnsOkFalse_Should_FallbackToRawItem verifies
// that when a script runs successfully but returns ok=false, the raw item is kept.
func TestFeedRefreshService_When_ScriptReturnsOkFalse_Should_FallbackToRawItem(t *testing.T) {
	feedXML := newFeedXMLServer(t, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Script Feed</title>
<item>
  <title>Original Title</title>
  <link>https://example.com/script-item</link>
  <description><![CDATA[<p>original summary</p>]]></description>
  <pubDate>Mon, 14 Apr 2026 10:00:00 GMT</pubDate>
</item>
</channel></rss>`)

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "scriptok.db"))
	if err != nil {
		t.Fatalf("NewTestSQLiteFeedRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	feed, err := repo.AddInFolder(feedXML.URL, "", nil, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	// Script outputs valid JSON but ok=false with a debug message
	script := `import sys, json; print(json.dumps({"ok": False, "debug": "intentional rejection"}))`
	if _, _, err := repo.UpdateFeedScript(feed.ID, script, "python"); err != nil {
		t.Fatalf("UpdateFeedScript() error = %v", err)
	}

	svc := NewFeedRefreshService(repo, t.TempDir(), func() *http.Client { return feedXML.Client() }, nil)
	if err := svc.RefreshFeed(context.Background(), feed.ID); err != nil {
		t.Fatalf("RefreshFeed() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("articles = %d, want 1", len(articles))
	}
	if articles[0].Title != "Original Title" {
		t.Fatalf("title = %q, want original title (script ok=false fallback)", articles[0].Title)
	}
}

// TestFeedRefreshService_When_RetentionDaysSettingMissing_Should_UseDefault7 verifies
// that the default 7-day retention is applied when the setting key is absent.
func TestFeedRefreshService_When_RetentionDaysSettingMissing_Should_UseDefault7(t *testing.T) {
	// Build a feed with an old article (9 days old) and no retention setting
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatalf("NewTestSQLiteFeedRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })

	oldDate := time.Now().UTC().Add(-9 * 24 * time.Hour).Format(time.RFC3339)
	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>T</title></channel></rss>`))
	}))
	t.Cleanup(feedXML.Close)

	_, err = repo.AddInFolder(feedXML.URL, "Retention Feed", []repository.ArticleSeed{{
		Title:       "Old",
		Link:        "https://example.com/old",
		Summary:     "<p>old</p>",
		PublishedAt: oldDate,
	}}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	// Confirm no retention setting exists
	_, ok, _ := repo.GetSetting("article_retention_days")
	if ok {
		t.Fatal("retention setting unexpectedly exists")
	}

	svc := NewFeedRefreshService(repo, t.TempDir(), func() *http.Client { return feedXML.Client() }, nil)
	if err := svc.RefreshAllFeeds(context.Background()); err != nil {
		t.Fatalf("RefreshAllFeeds() error = %v", err)
	}

	// 9 days > 7 days default → old article must be purged
	articles := repo.ListArticles()
	if len(articles) != 0 {
		t.Fatalf("articles after purge = %d, want 0 (default 7-day retention removes 9-day-old article)", len(articles))
	}
}
