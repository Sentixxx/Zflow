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
