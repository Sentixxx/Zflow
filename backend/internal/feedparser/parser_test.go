package feedparser

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sentixxx/Zflow/backend/internal/model"
)

func TestParseFeedCapturesOriginalItemFields(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>HN Feed</title>
    <item>
      <title>Example story</title>
      <link>https://news.ycombinator.com/item?id=1</link>
      <description><![CDATA[<p>Original RSS summary</p>]]></description>
      <pubDate>Mon, 07 Apr 2026 00:00:00 GMT</pubDate>
      <comments>https://news.ycombinator.com/item?id=1</comments>
      <guid>https://news.ycombinator.com/item?id=1</guid>
      <dc:creator xmlns:dc="http://purl.org/dc/elements/1.1/">pg</dc:creator>
      <content:encoded xmlns:content="http://purl.org/rss/1.0/modules/content/"><![CDATA[<p>Encoded body</p>]]></content:encoded>
    </item>
  </channel>
</rss>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(feed.Items))
	}

	payload := feed.Items[0].SourcePayload
	if payload == nil {
		t.Fatalf("SourcePayload = nil, want non-nil")
	}
	if payload.Summary != "<p>Original RSS summary</p>" {
		t.Fatalf("payload.Summary = %q, want RSS summary", payload.Summary)
	}
	if len(payload.Fields) < 3 {
		t.Fatalf("payload.Fields len = %d, want >= 3", len(payload.Fields))
	}
	if !hasSourceField(payload.Fields, "comments", "https://news.ycombinator.com/item?id=1") {
		t.Fatalf("payload.Fields = %+v, want comments field", payload.Fields)
	}
	if !hasSourceField(payload.Fields, "creator", "pg") {
		t.Fatalf("payload.Fields = %+v, want creator field", payload.Fields)
	}
	if !hasSourceHTMLField(payload.Fields, "encoded", "<p>Encoded body</p>") {
		t.Fatalf("payload.Fields = %+v, want encoded html field", payload.Fields)
	}
}

func hasSourceField(fields []model.ArticleSourceField, key string, value string) bool {
	for _, field := range fields {
		if field.Key == key && field.Value == value {
			return true
		}
	}
	return false
}

func hasSourceHTMLField(fields []model.ArticleSourceField, key string, html string) bool {
	for _, field := range fields {
		if field.Key == key && field.ValueHTML == html {
			return true
		}
	}
	return false
}

// --- Atom format ---

func TestParseFeed_When_AtomFeed_Should_PickAlternateLinkFirst(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Blog</title>
  <entry>
    <title>Atom Entry</title>
    <link href="https://example.com/feed.xml" rel="self"/>
    <link href="https://example.com/articles/1" rel="alternate"/>
    <summary>short summary</summary>
    <published>2026-04-01T10:00:00Z</published>
  </entry>
</feed>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(feed.Items))
	}
	// Must pick rel=alternate, not rel=self
	if feed.Items[0].Link != "https://example.com/articles/1" {
		t.Fatalf("link = %q, want alternate link", feed.Items[0].Link)
	}
}

func TestParseFeed_When_AtomEntryMissingPublished_Should_FallbackToUpdated(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Blog</title>
  <entry>
    <title>Entry Without Published</title>
    <link href="https://example.com/articles/2" rel="alternate"/>
    <summary>no published date</summary>
    <updated>2026-04-02T12:00:00Z</updated>
  </entry>
</feed>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(feed.Items))
	}
	// published is absent, must fall back to updated
	if feed.Items[0].PublishedAt != "2026-04-02T12:00:00Z" {
		t.Fatalf("published_at = %q, want updated fallback", feed.Items[0].PublishedAt)
	}
}

func TestParseFeed_When_AtomFeedNoRelLink_Should_UseHrefFallback(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Blog</title>
  <entry>
    <title>Entry No Rel</title>
    <link href="https://example.com/articles/3"/>
    <summary>no rel attribute</summary>
    <published>2026-04-03T08:00:00Z</published>
  </entry>
</feed>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(feed.Items))
	}
	// No rel attribute → the single href must be returned as fallback
	if feed.Items[0].Link != "https://example.com/articles/3" {
		t.Fatalf("link = %q, want fallback link", feed.Items[0].Link)
	}
}

// --- RSS resilience ---

func TestParseFeed_When_RSSItemMissingTitleAndLink_Should_NotPanic(t *testing.T) {
	// spec: partial/missing field items must not crash the parser; they are included with empty fields
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Resilient Feed</title>
    <item>
      <description><![CDATA[<p>Only description, no title or link</p>]]></description>
      <pubDate>Mon, 14 Apr 2026 10:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v (should not crash)", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(feed.Items))
	}
	if feed.Items[0].Title != "" {
		t.Fatalf("title = %q, want empty", feed.Items[0].Title)
	}
	if feed.Items[0].Link != "" {
		t.Fatalf("link = %q, want empty", feed.Items[0].Link)
	}
	if !strings.Contains(feed.Items[0].Summary, "Only description") {
		t.Fatalf("summary = %q, want description content", feed.Items[0].Summary)
	}
}

func TestParseFeed_When_UnknownRoot_Should_ReturnErrUnsupportedFeed(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?><opml version="2.0"><head><title>OPML</title></head></opml>`)

	_, err := ParseFeed(raw)
	if err == nil {
		t.Fatal("ParseFeed() error = nil, want ErrUnsupportedFeed")
	}
	if !errors.Is(err, ErrUnsupportedFeed) {
		t.Fatalf("ParseFeed() error = %v, want ErrUnsupportedFeed", err)
	}
}

func TestParseFeed_When_RSSItemHasEnclosure_Should_ExtractCoverURL(t *testing.T) {
	// spec tech-refer_feed_script_contract: cover_url is extracted from media:content / enclosure
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Podcast Feed</title>
    <item>
      <title>Episode 1</title>
      <link>https://example.com/ep1</link>
      <enclosure url="https://cdn.example.com/cover.jpg" type="image/jpeg" length="12345"/>
      <pubDate>Mon, 14 Apr 2026 10:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(feed.Items))
	}
	if feed.Items[0].CoverURL != "https://cdn.example.com/cover.jpg" {
		t.Fatalf("cover_url = %q, want enclosure image url", feed.Items[0].CoverURL)
	}
}

func TestParseFeed_When_RSSItemHasMediaContent_Should_ExtractCoverURL(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
  <channel>
    <title>Media Feed</title>
    <item>
      <title>Article With Media</title>
      <link>https://example.com/article</link>
      <media:content url="https://cdn.example.com/thumb.png" medium="image"/>
      <pubDate>Mon, 14 Apr 2026 10:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(feed.Items))
	}
	if feed.Items[0].CoverURL != "https://cdn.example.com/thumb.png" {
		t.Fatalf("cover_url = %q, want media:content url", feed.Items[0].CoverURL)
	}
}
