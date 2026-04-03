package service

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	_ "modernc.org/sqlite"
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

func createArticleServiceFixtureWithPath(t *testing.T) (*ArticleService, repository.FeedRepository, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "feeds.db")
	repo, err := repository.NewSQLiteFeedRepository(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	svc := repo
	articleService := NewArticleService(svc, func() *http.Client { return http.DefaultClient })
	return articleService, svc, dbPath
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

	page1, hasMore1 := uc.List(1, 2, ArticleSortLatest)
	if len(page1) != 2 || !hasMore1 {
		t.Fatalf("page1 len/hasMore = %d/%v, want 2/true", len(page1), hasMore1)
	}
	if page1[0].RecommendationScores == nil {
		t.Fatalf("page1[0].RecommendationScores = nil, want non-nil")
	}

	page2, hasMore2 := uc.List(2, 2, ArticleSortLatest)
	if len(page2) != 1 || hasMore2 {
		t.Fatalf("page2 len/hasMore = %d/%v, want 1/false", len(page2), hasMore2)
	}
}

func TestArticleServiceListSortByRecommend(t *testing.T) {
	uc, svc := createArticleServiceFixture(t)

	_, err := svc.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{
			Title:       "Tiny note",
			Link:        "https://example.com/1",
			Summary:     "short",
			PublishedAt: "2026-02-20T00:00:00Z",
		},
		{
			Title:       "Detailed engineering update on distributed systems rollout",
			Link:        "https://example.com/2",
			Summary:     "Detailed engineering update on distributed systems rollout with context and metrics.",
			FullContent: strings.Repeat("Detailed engineering update on distributed systems rollout with metrics and context. ", 40),
			CoverURL:    "https://example.com/cover.jpg",
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	articles, hasMore := uc.List(1, 10, ArticleSortRecommend)
	if hasMore {
		t.Fatalf("hasMore = true, want false")
	}
	if len(articles) != 2 {
		t.Fatalf("len(articles) = %d, want 2", len(articles))
	}
	if articles[0].Title != "Detailed engineering update on distributed systems rollout" {
		t.Fatalf("top article = %q, want detailed article first", articles[0].Title)
	}
	if articles[0].RecommendationScores == nil || articles[0].RecommendationScores.Composite <= articles[1].RecommendationScores.Composite {
		t.Fatalf("recommendation scores not sorted descending: %+v vs %+v", articles[0].RecommendationScores, articles[1].RecommendationScores)
	}
}

func TestArticleServiceExtractReadablePersistsScores(t *testing.T) {
	articleHTML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body><article><p>Detailed engineering update with architecture, rollout metrics, migration notes, and lessons learned.</p><p>More context to make the document materially richer than the original summary.</p></article></body></html>`))
	}))
	defer articleHTML.Close()

	uc, svc := createArticleServiceFixture(t)
	_, err := svc.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: articleHTML.URL, Summary: "brief summary"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	articles := svc.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}

	before, ok := uc.Get(articles[0].ID)
	if !ok {
		t.Fatalf("Get() ok = false, want true")
	}

	updated, err := uc.ExtractReadable(context.Background(), articles[0].ID)
	if err != nil {
		t.Fatalf("ExtractReadable() error = %v", err)
	}
	if updated.RecommendationScores == nil {
		t.Fatalf("updated.RecommendationScores = nil, want non-nil")
	}

	stored, ok := svc.GetArticle(articles[0].ID)
	if !ok {
		t.Fatalf("GetArticle() ok = false, want true")
	}
	if stored.RecommendationScores == nil {
		t.Fatalf("stored.RecommendationScores = nil, want non-nil")
	}
	if stored.ArticleFeatures == nil {
		t.Fatalf("stored.ArticleFeatures = nil, want non-nil")
	}
	if stored.ArticleFeatures.GateStatus != model.ArticleGateValid {
		t.Fatalf("stored gate_status = %q, want %q", stored.ArticleFeatures.GateStatus, model.ArticleGateValid)
	}
	if stored.RecommendationScores.Composite != updated.RecommendationScores.Composite {
		t.Fatalf("stored composite = %d, want %d", stored.RecommendationScores.Composite, updated.RecommendationScores.Composite)
	}
	if before.RecommendationScores == nil || updated.RecommendationScores.Composite <= before.RecommendationScores.Composite {
		t.Fatalf("updated composite = %v, want greater than before %v", updated.RecommendationScores, before.RecommendationScores)
	}
}

func TestAttachRecommendationScoresToSeedsMarksInvalidFragments(t *testing.T) {
	items := AttachRecommendationScoresToSeeds([]repository.ArticleSeed{
		{
			Title:       "Bare fragment",
			Link:        "https://example.com/fragment",
			Summary:     "tiny",
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
	})
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].ArticleFeatures == nil {
		t.Fatalf("ArticleFeatures = nil, want non-nil")
	}
	if items[0].ArticleFeatures.GateStatus != model.ArticleGateInvalid {
		t.Fatalf("gate_status = %q, want %q", items[0].ArticleFeatures.GateStatus, model.ArticleGateInvalid)
	}
	if items[0].RecommendationScores == nil {
		t.Fatalf("RecommendationScores = nil, want non-nil")
	}
	if items[0].RecommendationScores.Composite > 30 {
		t.Fatalf("invalid fragment composite = %d, want <= 30", items[0].RecommendationScores.Composite)
	}
}

func TestAttachRecommendationScoresToSeedsPenalizesNearDuplicateNovelty(t *testing.T) {
	items := AttachRecommendationScoresToSeeds([]repository.ArticleSeed{
		{
			Title:       "Postmortem: edge cache migration outage",
			Link:        "https://example.com/a",
			Summary:     "Detailed outage review with contributing factors and mitigation timeline.",
			FullContent: strings.Repeat("Postmortem edge cache migration outage contributing factors mitigation timeline root cause fix. ", 30),
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
		{
			Title:       "Postmortem: edge cache migration outage",
			Link:        "https://example.com/b",
			Summary:     "Detailed outage review with contributing factors and mitigation timeline.",
			FullContent: strings.Repeat("Postmortem edge cache migration outage contributing factors mitigation timeline root cause fix. ", 28),
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
		{
			Title:       "How we rebuilt feed ranking for multilingual research",
			Link:        "https://example.com/c",
			Summary:     "Architecture notes on ranking, evaluation metrics, and rollout safeguards.",
			FullContent: strings.Repeat("Feed ranking multilingual research architecture metrics rollout safeguards evaluation novelty scoring. ", 30),
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
	})
	if len(items) != 3 {
		t.Fatalf("len(items) = %d, want 3", len(items))
	}

	novelties := []int{
		items[0].RecommendationScores.Novelty,
		items[1].RecommendationScores.Novelty,
		items[2].RecommendationScores.Novelty,
	}
	slices.Sort(novelties)
	if novelties[0] >= novelties[2] {
		t.Fatalf("novelty scores not differentiated: %+v", novelties)
	}
	if items[2].RecommendationScores.Novelty <= items[0].RecommendationScores.Novelty ||
		items[2].RecommendationScores.Novelty <= items[1].RecommendationScores.Novelty {
		t.Fatalf("unique article novelty = %d, want greater than duplicates %d/%d", items[2].RecommendationScores.Novelty, items[0].RecommendationScores.Novelty, items[1].RecommendationScores.Novelty)
	}
}

func TestArticleServiceGetBackfillsLegacyScoresAndFeatures(t *testing.T) {
	uc, svc, dbPath := createArticleServiceFixtureWithPath(t)

	_, err := svc.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{
			Title:       "Distributed systems migration notes",
			Link:        "https://example.com/1",
			Summary:     "Migration notes with rollout stages, impact scope, and fallback strategy.",
			FullContent: strings.Repeat("Distributed systems migration notes rollout stages impact scope fallback strategy. ", 18),
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	articles := svc.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}

	legacyDB, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer legacyDB.Close()

	if _, err := legacyDB.Exec(`DELETE FROM article_features WHERE article_id = ?`, articles[0].ID); err != nil {
		t.Fatalf("DELETE article_features error = %v", err)
	}
	if _, err := legacyDB.Exec(`UPDATE entries SET quality_score = 0, relevance_score = 0, novelty_score = 0, composite_score = 0 WHERE id = ?`, articles[0].ID); err != nil {
		t.Fatalf("UPDATE entries reset scores error = %v", err)
	}

	loaded, ok := uc.Get(articles[0].ID)
	if !ok {
		t.Fatalf("Get() ok = false, want true")
	}
	if loaded.RecommendationScores == nil || loaded.RecommendationScores.Composite <= 0 {
		t.Fatalf("loaded recommendation_scores = %+v, want persisted fallback scores", loaded.RecommendationScores)
	}
	if loaded.ArticleFeatures == nil || loaded.ArticleFeatures.GateStatus == "" {
		t.Fatalf("loaded article_features = %+v, want backfilled features", loaded.ArticleFeatures)
	}

	stored, ok := svc.GetArticle(articles[0].ID)
	if !ok {
		t.Fatalf("GetArticle() ok = false, want true")
	}
	if stored.RecommendationScores == nil || stored.RecommendationScores.Composite <= 0 {
		t.Fatalf("stored recommendation_scores = %+v, want backfilled scores", stored.RecommendationScores)
	}
	if stored.ArticleFeatures == nil || stored.ArticleFeatures.FeatureVersion <= 0 {
		t.Fatalf("stored article_features = %+v, want persisted feature row", stored.ArticleFeatures)
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

func TestArticleServiceExtractReadableSlowBody(t *testing.T) {
	articleHTML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body><article><p>Slow`))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		time.Sleep(1500 * time.Millisecond)
		_, _ = w.Write([]byte(` body content for readability test.</p></article></body></html>`))
	}))
	defer articleHTML.Close()

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	articleService := NewArticleService(repo, func() *http.Client {
		return &http.Client{Timeout: 5 * time.Second}
	})
	_, err = repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: articleHTML.URL, Summary: "S1"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}

	updated, err := articleService.ExtractReadable(context.Background(), articles[0].ID)
	if err != nil {
		t.Fatalf("ExtractReadable() error = %v", err)
	}
	if !strings.Contains(updated.FullContent, "Slow body content for readability test") {
		t.Fatalf("full_content = %q, want slow body content", updated.FullContent)
	}
}
