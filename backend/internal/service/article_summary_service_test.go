package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
)

func TestBuildDisplaySummaryPrefersFullContent(t *testing.T) {
	summary, status := BuildDisplaySummary(model.Article{
		Title:       "Title",
		Summary:     "<p>RSS summary</p>",
		FullContent: "<article><p>Full content paragraph one.</p><p>Full content paragraph two.</p></article>",
	})
	if status != DisplaySummaryFallback {
		t.Fatalf("status = %q, want %q", status, DisplaySummaryFallback)
	}
	if !strings.Contains(summary, "Full content paragraph one.") {
		t.Fatalf("summary = %q, want full content", summary)
	}
	if strings.Contains(summary, "RSS summary") {
		t.Fatalf("summary = %q, want full content preferred over rss summary", summary)
	}
}

func TestBuildDisplaySummaryFallsBackToRawSummary(t *testing.T) {
	summary, status := BuildDisplaySummary(model.Article{
		Title:   "Title",
		Summary: "<p>RSS summary only</p>",
	})
	if status != DisplaySummaryFallback {
		t.Fatalf("status = %q, want %q", status, DisplaySummaryFallback)
	}
	if !strings.Contains(summary, "RSS summary only") {
		t.Fatalf("summary = %q, want rss summary", summary)
	}
}

func TestArticleSummaryServiceBackfillFeed(t *testing.T) {
	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feed, err := repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>Hello world</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		nil,
		nil,
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.BackfillFeed(feed.ID, 20); err != nil {
		t.Fatalf("BackfillFeed() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}
	if articles[0].DisplaySummary == "" {
		t.Fatalf("display_summary is empty, want generated summary")
	}
	if articles[0].DisplaySummaryStatus == "" {
		t.Fatalf("display_summary_status is empty, want generated status")
	}
}

func TestArticleSummaryServiceUsesAIWhenConfigured(t *testing.T) {
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"这篇文章总结了关键变化，并点出了对读者最重要的影响。"}}]}`))
	}))
	defer aiMock.Close()

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feed, err := repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>Hello world</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		func() *http.Client { return aiMock.Client() },
		func() (SummaryAIConfig, error) {
			return SummaryAIConfig{
				APIKey:  "test-key",
				BaseURL: aiMock.URL,
				Model:   "MiniMax-M2.7",
			}, nil
		},
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.BackfillFeed(feed.ID, 20); err != nil {
		t.Fatalf("BackfillFeed() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}
	if !strings.Contains(articles[0].DisplaySummary, "关键变化") {
		t.Fatalf("display_summary = %q, want ai generated summary", articles[0].DisplaySummary)
	}
	if articles[0].DisplaySummaryStatus != DisplaySummaryReady {
		t.Fatalf("display_summary_status = %q, want %q", articles[0].DisplaySummaryStatus, DisplaySummaryReady)
	}
}

func TestArticleSummaryServiceFallsBackWhenAIUnavailable(t *testing.T) {
	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		func() *http.Client { return http.DefaultClient },
		func() (SummaryAIConfig, error) {
			return SummaryAIConfig{}, context.Canceled
		},
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	summary, status := svc.buildDisplaySummary(context.Background(), model.Article{
		Title:   "Title",
		Summary: "<p>RSS summary only</p>",
	})
	if status != DisplaySummaryFallback {
		t.Fatalf("status = %q, want %q", status, DisplaySummaryFallback)
	}
	if !strings.Contains(summary, "RSS summary only") {
		t.Fatalf("summary = %q, want fallback local summary", summary)
	}
}
