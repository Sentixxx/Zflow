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
