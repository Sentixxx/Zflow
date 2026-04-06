package feedparser

import (
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
