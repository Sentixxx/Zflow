package store

import (
	"path/filepath"
	"testing"
)

func TestAddCleansAndDeduplicates(t *testing.T) {
	s, err := NewFeedStore(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewFeedStore() error = %v", err)
	}

	feed1, err := s.Add("https://example.com/feed1.xml", "Feed 1", []ArticleSeed{
		{Title: "  A  ", Link: "https://example.com/a", Summary: " first "},
		{Title: "A duplicate", Link: "https://example.com/a", Summary: "duplicate by link"},
		{Title: "   ", Link: "   ", Summary: "invalid"},
	}, "")
	if err != nil {
		t.Fatalf("Add(feed1) error = %v", err)
	}
	if feed1.ItemCount != 1 {
		t.Fatalf("feed1.ItemCount = %d, want 1", feed1.ItemCount)
	}

	feed2, err := s.Add("https://example.com/feed2.xml", "Feed 2", []ArticleSeed{
		{Title: "A from other feed", Link: "https://example.com/a", Summary: "duplicate across feeds"},
		{Title: "  Same title  ", Summary: "same summary"},
		{Title: "same   title", Summary: " same   summary "},
		{Title: "Unique", Summary: "U"},
	}, "")
	if err != nil {
		t.Fatalf("Add(feed2) error = %v", err)
	}
	if feed2.ItemCount != 2 {
		t.Fatalf("feed2.ItemCount = %d, want 2", feed2.ItemCount)
	}

	articles := s.ListArticles()
	if len(articles) != 3 {
		t.Fatalf("len(articles) = %d, want 3", len(articles))
	}

	if articles[0].Title != "A" || articles[0].Summary != "first" {
		t.Fatalf("first article not cleaned: %+v", articles[0])
	}
}
