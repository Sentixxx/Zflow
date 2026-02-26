package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sentixxx/Zflow/backend/internal/repository"
)

func createArticleServiceFixture(t *testing.T) (*ArticleService, repository.FeedRepository) {
	t.Helper()
	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	svc := repo
	articleService := NewArticleService(svc, func() *http.Client { return http.DefaultClient })
	return articleService, svc
}

func TestArticleServiceListPagination(t *testing.T) {
	uc, svc := createArticleServiceFixture(t)

	_, err := svc.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "S1"},
		{Title: "A2", Link: "https://example.com/2", Summary: "S2"},
		{Title: "A3", Link: "https://example.com/3", Summary: "S3"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	page1, hasMore1 := uc.List(1, 2)
	if len(page1) != 2 || !hasMore1 {
		t.Fatalf("page1 len/hasMore = %d/%v, want 2/true", len(page1), hasMore1)
	}

	page2, hasMore2 := uc.List(2, 2)
	if len(page2) != 1 || hasMore2 {
		t.Fatalf("page2 len/hasMore = %d/%v, want 1/false", len(page2), hasMore2)
	}
}

func TestArticleServiceExtractReadable(t *testing.T) {
	articleHTML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body><article><p>Readable service content.</p></article></body></html>`))
	}))
	defer articleHTML.Close()

	uc, svc := createArticleServiceFixture(t)
	_, err := svc.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: articleHTML.URL, Summary: "S1"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	articles := svc.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}

	updated, err := uc.ExtractReadable(context.Background(), articles[0].ID)
	if err != nil {
		t.Fatalf("ExtractReadable() error = %v", err)
	}
	if !strings.Contains(updated.FullContent, "Readable service content") {
		t.Fatalf("full_content = %q, want contains readability text", updated.FullContent)
	}
}

func TestArticleServiceExtractReadableRejectPDF(t *testing.T) {
	pdfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	}))
	defer pdfServer.Close()

	uc, svc := createArticleServiceFixture(t)
	_, err := svc.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: pdfServer.URL, Summary: "S1"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	articles := svc.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}

	_, err = uc.ExtractReadable(context.Background(), articles[0].ID)
	if err == nil {
		t.Fatalf("ExtractReadable() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "unsupported readability content type: pdf") {
		t.Fatalf("error = %q, want pdf unsupported", err.Error())
	}
}
