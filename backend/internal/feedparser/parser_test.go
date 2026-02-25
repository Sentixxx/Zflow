package feedparser

import "testing"

func TestParseFeedRSS(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Example RSS</title>
    <item><title>A</title><link>https://example.com/a</link><description>DA</description></item>
    <item><title>B</title><link>https://example.com/b</link><description>DB</description></item>
  </channel>
</rss>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if feed.Title != "Example RSS" {
		t.Fatalf("title = %q, want %q", feed.Title, "Example RSS")
	}
	if len(feed.Items) != 2 {
		t.Fatalf("count = %d, want 2", len(feed.Items))
	}
	if feed.Items[0].Link != "https://example.com/a" {
		t.Fatalf("item[0].link = %q", feed.Items[0].Link)
	}
}

func TestParseFeedAtom(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example Atom</title>
  <entry><title>E1</title></entry>
  <entry><title>E2</title></entry>
  <entry><title>E3</title></entry>
</feed>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if feed.Title != "Example Atom" {
		t.Fatalf("title = %q, want %q", feed.Title, "Example Atom")
	}
	if len(feed.Items) != 3 {
		t.Fatalf("count = %d, want 3", len(feed.Items))
	}
}

func TestParseFeedExtractMediaCoverURL(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
  <channel>
    <title>Example RSS</title>
    <item>
      <title>A</title>
      <link>https://example.com/a</link>
      <description>DA</description>
      <media:thumbnail url="https://cdn.example.com/thumb-a.jpg" />
    </item>
    <item>
      <title>B</title>
      <link>https://example.com/b</link>
      <description>DB</description>
      <enclosure type="image/jpeg" url="https://cdn.example.com/cover-b.jpg" />
    </item>
  </channel>
</rss>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if len(feed.Items) != 2 {
		t.Fatalf("count = %d, want 2", len(feed.Items))
	}
	if feed.Items[0].CoverURL != "https://cdn.example.com/thumb-a.jpg" {
		t.Fatalf("item[0].cover = %q", feed.Items[0].CoverURL)
	}
	if feed.Items[1].CoverURL != "https://cdn.example.com/cover-b.jpg" {
		t.Fatalf("item[1].cover = %q", feed.Items[1].CoverURL)
	}
}

func TestParseFeedExtractFeedIconHints(t *testing.T) {
	raw := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Icon Feed</title>
    <image>
      <url>https://example.com/rss-image.png</url>
    </image>
    <itunes:image href="https://example.com/podcast-cover.jpg" />
    <item><title>A</title></item>
  </channel>
</rss>`)

	feed, err := ParseFeed(raw)
	if err != nil {
		t.Fatalf("ParseFeed() error = %v", err)
	}
	if len(feed.IconHints) == 0 {
		t.Fatalf("icon hints should not be empty")
	}
	if feed.IconHints[0] != "https://example.com/rss-image.png" {
		t.Fatalf("icon hint[0] = %q", feed.IconHints[0])
	}
}
