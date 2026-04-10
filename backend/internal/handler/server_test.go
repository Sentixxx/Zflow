package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/internal/service"
	_ "github.com/mattn/go-sqlite3"
)

func scoreHandlerTestSeeds(items []repository.ArticleSeed) []repository.ArticleSeed {
	return service.AttachRecommendationScoresToSeeds(items)
}

func TestCreateFeedAndList(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>A</title>
      <link>https://example.com/a</link>
      <description>DA</description>
      <pubDate>Wed, 25 Feb 2026 10:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer upstream.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := repo
	server := NewServer(feedService, t.TempDir())

	body, _ := json.Marshal(map[string]string{"url": upstream.URL})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rr.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/feeds", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)

	if rrList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/feeds status = %d, want %d", rrList.Code, http.StatusOK)
	}
}

func TestArticleListDetailAndMarkRead(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Read Feed</title>
    <item>
      <title>Article One</title>
      <link>https://example.com/1</link>
      <description>desc</description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer upstream.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := repo
	server := NewServer(feedService, t.TempDir())

	createBody, _ := json.Marshal(map[string]string{"url": upstream.URL})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rrCreate.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles status = %d, want %d", rrList.Code, http.StatusOK)
	}

	var listResp struct {
		Articles []struct {
			ID     int64 `json:"id"`
			IsRead bool  `json:"is_read"`
			Scores struct {
				Composite int `json:"composite"`
			} `json:"recommendation_scores"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}
	if listResp.Articles[0].Scores.Composite <= 0 {
		t.Fatalf("composite score = %d, want > 0", listResp.Articles[0].Scores.Composite)
	}

	articleID := listResp.Articles[0].ID
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/articles/"+strconv.FormatInt(articleID, 10), nil)
	rrDetail := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrDetail, reqDetail)
	if rrDetail.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles/:id status = %d, want %d", rrDetail.Code, http.StatusOK)
	}
	var createdDetail struct {
		DisplaySummary       string `json:"display_summary"`
		DisplaySummaryStatus string `json:"display_summary_status"`
	}
	if err := json.Unmarshal(rrDetail.Body.Bytes(), &createdDetail); err != nil {
		t.Fatalf("unmarshal created detail response error = %v", err)
	}
	if !strings.Contains(createdDetail.DisplaySummary, "desc") {
		t.Fatalf("display_summary = %q, want generated summary from raw rss summary", createdDetail.DisplaySummary)
	}
	if createdDetail.DisplaySummaryStatus == "" {
		t.Fatalf("display_summary_status is empty, want generated status")
	}

	readBody, _ := json.Marshal(map[string]bool{"read": true})
	reqRead := httptest.NewRequest(http.MethodPatch, "/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/read", bytes.NewReader(readBody))
	rrRead := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrRead, reqRead)
	if rrRead.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/articles/:id/read status = %d, want %d", rrRead.Code, http.StatusOK)
	}

	reqDetail2 := httptest.NewRequest(http.MethodGet, "/api/v1/articles/"+strconv.FormatInt(articleID, 10), nil)
	rrDetail2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrDetail2, reqDetail2)
	if rrDetail2.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles/:id status after read = %d, want %d", rrDetail2.Code, http.StatusOK)
	}
	var detailResp struct {
		IsRead bool `json:"is_read"`
	}
	if err := json.Unmarshal(rrDetail2.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("unmarshal detail response error = %v", err)
	}
	if !detailResp.IsRead {
		t.Fatalf("is_read = false, want true")
	}
}

func TestArticleListOmitsHeavyFields(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	_, err = repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{
			Title:       "Detailed article",
			Link:        "https://example.com/1",
			Summary:     "summary payload",
			FullContent: "<p>full content payload</p>",
			CoverURL:    "https://example.com/cover.jpg",
			PublishedAt: "2026-04-01T00:00:00Z",
		},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("repo articles len = %d, want 1", len(articles))
	}
	articleID := articles[0].ID

	if err := repo.UpdateArticleSummaryState(articleID, "ai summary payload", "ready", "display summary payload", "ready"); err != nil {
		t.Fatalf("UpdateArticleSummaryState() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles status = %d, want %d", rr.Code, http.StatusOK)
	}

	var listResp struct {
		Articles []map[string]any `json:"articles"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}
	listArticle := listResp.Articles[0]
	for _, field := range []string{"summary", "display_summary", "display_summary_status", "display_summary_updated_at", "full_content", "ai_summary", "ai_summary_status", "ai_summary_updated_at", "summary_debug"} {
		if _, ok := listArticle[field]; ok {
			t.Fatalf("list article unexpectedly contains heavy field %q", field)
		}
	}
	if _, ok := listArticle["recommendation_scores"]; !ok {
		t.Fatalf("list article missing recommendation_scores")
	}

	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/articles/"+strconv.FormatInt(articleID, 10), nil)
	rrDetail := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrDetail, reqDetail)
	if rrDetail.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles/:id status = %d, want %d", rrDetail.Code, http.StatusOK)
	}

	var detailResp map[string]any
	if err := json.Unmarshal(rrDetail.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("unmarshal detail response error = %v", err)
	}
	for _, field := range []string{"summary", "display_summary", "display_summary_status", "full_content", "ai_summary", "ai_summary_status"} {
		if _, ok := detailResp[field]; !ok {
			t.Fatalf("detail article missing field %q", field)
		}
	}
}

func TestArticleListSortByRecommend(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	_, err = repo.AddInFolder("https://example.com/feed", "Feed", scoreHandlerTestSeeds([]repository.ArticleSeed{
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
			PublishedAt: "2026-02-26T00:00:00Z",
		},
	}), "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles?sort=recommend", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles?sort=recommend status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		Articles []struct {
			Title  string `json:"title"`
			Scores struct {
				Composite int `json:"composite"`
			} `json:"recommendation_scores"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response error = %v", err)
	}
	if len(resp.Articles) != 2 {
		t.Fatalf("articles len = %d, want 2", len(resp.Articles))
	}
	if resp.Articles[0].Title != "Detailed engineering update on distributed systems rollout" {
		t.Fatalf("top article = %q, want detailed article first", resp.Articles[0].Title)
	}
	if resp.Articles[0].Scores.Composite <= resp.Articles[1].Scores.Composite {
		t.Fatalf("composite order = %d <= %d, want descending", resp.Articles[0].Scores.Composite, resp.Articles[1].Scores.Composite)
	}
}

func TestArticleListRejectInvalidSort(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles?sort=garbage", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/v1/articles?sort=garbage status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestArticleListSupportsFeedAndFolderScope(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	rootFolder, err := repo.CreateFolder("Root", nil)
	if err != nil {
		t.Fatalf("CreateFolder(root) error = %v", err)
	}
	childFolder, err := repo.CreateFolder("Child", &rootFolder.ID)
	if err != nil {
		t.Fatalf("CreateFolder(child) error = %v", err)
	}

	rootFeed, err := repo.AddInFolder("https://example.com/root.xml", "Root Feed", []repository.ArticleSeed{
		{Title: "Root article", Link: "https://example.com/root", Summary: "root", PublishedAt: "2026-03-01T00:00:00Z"},
	}, "", &rootFolder.ID, "", "")
	if err != nil {
		t.Fatalf("AddInFolder(root) error = %v", err)
	}
	childFeed, err := repo.AddInFolder("https://example.com/child.xml", "Child Feed", []repository.ArticleSeed{
		{Title: "Child article", Link: "https://example.com/child", Summary: "child", PublishedAt: "2026-03-02T00:00:00Z"},
	}, "", &childFolder.ID, "", "")
	if err != nil {
		t.Fatalf("AddInFolder(child) error = %v", err)
	}
	_, err = repo.AddInFolder("https://example.com/other.xml", "Other Feed", []repository.ArticleSeed{
		{Title: "Other article", Link: "https://example.com/other", Summary: "other", PublishedAt: "2026-03-03T00:00:00Z"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder(other) error = %v", err)
	}

	reqFeed := httptest.NewRequest(http.MethodGet, "/api/v1/articles?feed_id="+strconv.FormatInt(rootFeed.ID, 10), nil)
	rrFeed := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrFeed, reqFeed)
	if rrFeed.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles?feed_id status = %d, want %d", rrFeed.Code, http.StatusOK)
	}
	var feedResp struct {
		Articles []struct {
			FeedID int64  `json:"feed_id"`
			Title  string `json:"title"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrFeed.Body.Bytes(), &feedResp); err != nil {
		t.Fatalf("unmarshal feed scope response error = %v", err)
	}
	if len(feedResp.Articles) != 1 || feedResp.Articles[0].FeedID != rootFeed.ID || feedResp.Articles[0].Title != "Root article" {
		t.Fatalf("feed scope articles = %+v, want only root feed article", feedResp.Articles)
	}

	reqFolder := httptest.NewRequest(http.MethodGet, "/api/v1/articles?folder_id="+strconv.FormatInt(rootFolder.ID, 10), nil)
	rrFolder := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrFolder, reqFolder)
	if rrFolder.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles?folder_id status = %d, want %d", rrFolder.Code, http.StatusOK)
	}
	var folderResp struct {
		Articles []struct {
			FeedID int64  `json:"feed_id"`
			Title  string `json:"title"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrFolder.Body.Bytes(), &folderResp); err != nil {
		t.Fatalf("unmarshal folder scope response error = %v", err)
	}
	if len(folderResp.Articles) != 2 {
		t.Fatalf("folder scope articles len = %d, want 2", len(folderResp.Articles))
	}
	gotFeedIDs := []int64{folderResp.Articles[0].FeedID, folderResp.Articles[1].FeedID}
	sort.Slice(gotFeedIDs, func(i, j int) bool { return gotFeedIDs[i] < gotFeedIDs[j] })
	wantFeedIDs := []int64{rootFeed.ID, childFeed.ID}
	if !reflect.DeepEqual(gotFeedIDs, wantFeedIDs) {
		t.Fatalf("folder scope feed ids = %v, want %v", gotFeedIDs, wantFeedIDs)
	}

	emptyFolder, err := repo.CreateFolder("Empty", nil)
	if err != nil {
		t.Fatalf("CreateFolder(empty) error = %v", err)
	}
	reqEmptyFolder := httptest.NewRequest(http.MethodGet, "/api/v1/articles?folder_id="+strconv.FormatInt(emptyFolder.ID, 10), nil)
	rrEmptyFolder := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrEmptyFolder, reqEmptyFolder)
	if rrEmptyFolder.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles?folder_id(empty) status = %d, want %d", rrEmptyFolder.Code, http.StatusOK)
	}
	var emptyFolderResp struct {
		Articles []struct {
			ID int64 `json:"id"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrEmptyFolder.Body.Bytes(), &emptyFolderResp); err != nil {
		t.Fatalf("unmarshal empty folder scope response error = %v", err)
	}
	if len(emptyFolderResp.Articles) != 0 {
		t.Fatalf("empty folder scope articles len = %d, want 0", len(emptyFolderResp.Articles))
	}
}

func TestArticleDetailDoesNotBackfillLegacyScores(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "feeds.db")
	repo, err := repository.NewTestSQLiteFeedRepository(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	_, err = repo.AddInFolder("https://example.com/feed", "Feed", scoreHandlerTestSeeds([]repository.ArticleSeed{
		{
			Title:       "Distributed systems migration notes",
			Link:        "https://example.com/1",
			Summary:     "Migration notes with rollout stages, impact scope, and fallback strategy.",
			FullContent: strings.Repeat("Distributed systems migration notes rollout stages impact scope fallback strategy. ", 18),
			PublishedAt: "2026-02-26T00:00:00Z",
		},
	}), "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}

	legacyDB, err := sql.Open("sqlite3", "file:"+dbPath)
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles/"+strconv.FormatInt(articles[0].ID, 10), nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles/:id status = %d, want %d", rr.Code, http.StatusOK)
	}

	var detail struct {
		Scores struct {
			Composite int `json:"composite"`
		} `json:"recommendation_scores"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &detail); err != nil {
		t.Fatalf("unmarshal detail response error = %v", err)
	}
	if detail.Scores.Composite != 0 {
		t.Fatalf("detail composite = %d, want stored zero score without sync backfill", detail.Scores.Composite)
	}

	stored, ok := repo.GetArticle(articles[0].ID)
	if !ok {
		t.Fatalf("GetArticle() ok = false, want true")
	}
	if stored.RecommendationScores == nil || stored.RecommendationScores.Composite != 0 {
		t.Fatalf("stored recommendation_scores = %+v, want unchanged zero scores", stored.RecommendationScores)
	}
	if stored.ArticleFeatures != nil {
		t.Fatalf("stored article_features = %+v, want no persisted feature row after pure read", stored.ArticleFeatures)
	}
}

func TestCORSPreflightAndHeaders(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := repo
	server := NewServer(feedService, t.TempDir())

	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/articles", nil)
	preflight.Header.Set("Origin", "http://localhost:5173")
	preflight.Header.Set("Access-Control-Request-Method", "GET")
	rrPreflight := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrPreflight, preflight)

	if rrPreflight.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS /api/v1/articles status = %d, want %d", rrPreflight.Code, http.StatusNoContent)
	}
	if rrPreflight.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want localhost origin", rrPreflight.Header().Get("Access-Control-Allow-Origin"))
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want localhost origin", rr.Header().Get("Access-Control-Allow-Origin"))
	}

	reqBlocked := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	reqBlocked.Header.Set("Origin", "https://evil.example.com")
	rrBlocked := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrBlocked, reqBlocked)
	if rrBlocked.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("blocked origin unexpectedly allowed: %q", rrBlocked.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSAllowsConfiguredOrigins(t *testing.T) {
	t.Setenv("ZFLOW_ALLOWED_ORIGINS", "https://app.example.com, https://admin.example.com")

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want configured origin", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestCORSAllowsSameHostLanOrigin(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "http://192.168.1.9:8080/healthz", nil)
	req.Host = "192.168.1.9:8080"
	req.Header.Set("Origin", "http://192.168.1.9:5173")
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "http://192.168.1.9:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want same-host LAN origin", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}

func TestAISettingsGetAndPatch(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := repo
	server := NewServer(feedService, t.TempDir())

	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/settings/ai", nil)
	rrGet := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/settings/ai status = %d, want %d", rrGet.Code, http.StatusOK)
	}

	reqPatch := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/ai", bytes.NewReader([]byte(`{
		"api_key":"test-ai-key",
		"base_url":"https://example-ai.local/v1",
		"model":"test-model",
		"target_lang":"ja",
		"embedding_api_key":"embed-key",
		"embedding_base_url":"https://example-embed.local/v1",
		"embedding_model":"text-embedding-test"
	}`)))
	rrPatch := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrPatch, reqPatch)
	if rrPatch.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/settings/ai status = %d, want %d, body=%s", rrPatch.Code, http.StatusOK, rrPatch.Body.String())
	}
	if strings.Contains(rrPatch.Body.String(), "test-ai-key") {
		t.Fatalf("PATCH /api/v1/settings/ai leaked plaintext api key: %s", rrPatch.Body.String())
	}

	rrGet2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrGet2, reqGet)
	if rrGet2.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/settings/ai status(after patch) = %d, want %d", rrGet2.Code, http.StatusOK)
	}
	var resp struct {
		Protocol         string `json:"protocol"`
		APIKey           string `json:"api_key"`
		APIKeyMasked     string `json:"api_key_masked"`
		APIKeyConfigured bool   `json:"api_key_configured"`
		BaseURL          string `json:"base_url"`
		Model            string `json:"model"`
		TargetLang       string `json:"target_lang"`
		EmbeddingAPIKey  string `json:"embedding_api_key"`
		EmbeddingMasked  string `json:"embedding_api_key_masked"`
		EmbeddingHasKey  bool   `json:"embedding_api_key_configured"`
		EmbeddingBaseURL string `json:"embedding_base_url"`
		EmbeddingModel   string `json:"embedding_model"`
	}
	if err := json.Unmarshal(rrGet2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal ai settings response error = %v", err)
	}
	if resp.Protocol != "openai" || resp.APIKey != "" || resp.APIKeyMasked == "" || !resp.APIKeyConfigured || resp.BaseURL != "https://example-ai.local/v1" || resp.Model != "test-model" || resp.TargetLang != "ja" || resp.EmbeddingAPIKey != "" || resp.EmbeddingMasked == "" || !resp.EmbeddingHasKey || resp.EmbeddingBaseURL != "https://example-embed.local/v1" || resp.EmbeddingModel != "text-embedding-test" {
		t.Fatalf("ai settings response mismatch: %+v", resp)
	}
}

func TestEmbeddingSettingsFallbackToChatSettings(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	if err := repo.SetSetting(settingKeyAIApiKey, "chat-key"); err != nil {
		t.Fatalf("SetSetting(ai_api_key) error = %v", err)
	}
	if err := repo.SetSetting(settingKeyAIBaseURL, "https://example-ai.local/v1"); err != nil {
		t.Fatalf("SetSetting(ai_base_url) error = %v", err)
	}
	if err := repo.SetSetting(settingKeyAIModel, "chat-model"); err != nil {
		t.Fatalf("SetSetting(ai_model) error = %v", err)
	}

	server := NewServer(repo, t.TempDir(), WithVectorRepository(&repository.SQLiteVectorRepository{}))

	cfg, err := server.loadEmbeddingSettings()
	if err != nil {
		t.Fatalf("loadEmbeddingSettings() error = %v", err)
	}
	if cfg.APIKey != "chat-key" || cfg.BaseURL != "https://example-ai.local/v1" || cfg.Model != "chat-model" {
		t.Fatalf("embedding fallback mismatch: %+v", cfg)
	}
}

func TestAISettingsPersistsAnthropicProtocol(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	reqPatch := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/ai", bytes.NewReader([]byte(`{
		"protocol":"anthropic",
		"api_key":"test-ai-key",
		"base_url":"https://example-ai.local/anthropic",
		"model":"test-model",
		"target_lang":"zh-CN"
	}`)))
	rrPatch := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrPatch, reqPatch)
	if rrPatch.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/settings/ai status = %d, want %d, body=%s", rrPatch.Code, http.StatusOK, rrPatch.Body.String())
	}

	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/settings/ai", nil)
	rrGet := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/settings/ai status = %d, want %d", rrGet.Code, http.StatusOK)
	}

	var resp struct {
		Protocol string `json:"protocol"`
		BaseURL  string `json:"base_url"`
	}
	if err := json.Unmarshal(rrGet.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal ai settings response error = %v", err)
	}
	if resp.Protocol != "anthropic" || resp.BaseURL != "https://example-ai.local/anthropic" {
		t.Fatalf("ai settings response mismatch: %+v", resp)
	}
}

func TestAISettingsInfersAnthropicProtocolFromBaseURL(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	if err := repo.SetSetting(settingKeyAIBaseURL, "https://api.minimaxi.com/anthropic"); err != nil {
		t.Fatalf("SetSetting(ai_base_url) error = %v", err)
	}
	if err := repo.SetSetting(settingKeyAIApiKey, "test-ai-key"); err != nil {
		t.Fatalf("SetSetting(ai_api_key) error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/settings/ai", nil)
	rrGet := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrGet, reqGet)
	if rrGet.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/settings/ai status = %d, want %d", rrGet.Code, http.StatusOK)
	}

	var resp struct {
		Protocol string `json:"protocol"`
		BaseURL  string `json:"base_url"`
	}
	if err := json.Unmarshal(rrGet.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal ai settings response error = %v", err)
	}
	if resp.Protocol != "anthropic" || resp.BaseURL != "https://api.minimaxi.com/anthropic" {
		t.Fatalf("ai settings response mismatch: %+v", resp)
	}
}

func TestArticleReadabilityExtraction(t *testing.T) {
	articleHTML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html><head><title>Readable</title></head>
<body>
  <article>
    <h1>Readable Title</h1>
    <p>这是 Readability 抽取测试段落。</p>
  </article>
</body></html>`))
	}))
	defer articleHTML.Close()

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Readability Feed</title>
    <item>
      <title>Readability Item</title>
      <link>` + articleHTML.URL + `</link>
      <description>desc</description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := repo
	server := NewServer(feedService, t.TempDir())

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rrCreate.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles status = %d, want %d", rrList.Code, http.StatusOK)
	}
	var listResp struct {
		Articles []struct {
			ID int64 `json:"id"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}

	articleID := listResp.Articles[0].ID
	reqReadable := httptest.NewRequest(http.MethodPost, "/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/readability", nil)
	rrReadable := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrReadable, reqReadable)
	if rrReadable.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/articles/:id/readability status = %d, want %d, body=%s", rrReadable.Code, http.StatusOK, rrReadable.Body.String())
	}

	var detailResp struct {
		FullContent          string `json:"full_content"`
		DisplaySummary       string `json:"display_summary"`
		DisplaySummaryStatus string `json:"display_summary_status"`
	}
	if err := json.Unmarshal(rrReadable.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("unmarshal readability response error = %v", err)
	}
	if detailResp.FullContent == "" {
		t.Fatalf("full_content is empty, want non-empty")
	}
	if !strings.Contains(detailResp.FullContent, "Readability 抽取测试段落") {
		t.Fatalf("full_content = %q, want contains readability text", detailResp.FullContent)
	}
	if !strings.Contains(detailResp.DisplaySummary, "Readability 抽取测试段落") {
		t.Fatalf("display_summary = %q, want contains readability text", detailResp.DisplaySummary)
	}
	if detailResp.DisplaySummaryStatus != "fallback" {
		t.Fatalf("display_summary_status = %q, want fallback", detailResp.DisplaySummaryStatus)
	}
}

func TestArticleReadabilityRejectPDF(t *testing.T) {
	pdfServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4\n%fake pdf content"))
	}))
	defer pdfServer.Close()

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>PDF Feed</title>
    <item>
      <title>PDF Item</title>
      <link>` + pdfServer.URL + `</link>
      <description>desc</description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := repo
	server := NewServer(feedService, t.TempDir())

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rrCreate.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles status = %d, want %d", rrList.Code, http.StatusOK)
	}
	var listResp struct {
		Articles []struct {
			ID int64 `json:"id"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}

	articleID := listResp.Articles[0].ID
	reqReadable := httptest.NewRequest(http.MethodPost, "/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/readability", nil)
	rrReadable := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrReadable, reqReadable)
	if rrReadable.Code != http.StatusBadGateway {
		t.Fatalf("POST /api/v1/articles/:id/readability status = %d, want %d", rrReadable.Code, http.StatusBadGateway)
	}
	if !strings.Contains(rrReadable.Body.String(), "unsupported readability content type: pdf") {
		t.Fatalf("readability error body = %q, want pdf unsupported error", rrReadable.Body.String())
	}
}

func TestArticleReadabilityKeepsSourcePayload(t *testing.T) {
	articleHTML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body><article><p>Readable service content</p></article></body></html>`))
	}))
	defer articleHTML.Close()

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>HN-like Feed</title>
    <item>
      <title>Example story</title>
      <link>` + articleHTML.URL + `</link>
      <description><![CDATA[<p>Original RSS summary</p>]]></description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
      <comments>https://news.ycombinator.com/item?id=1</comments>
      <dc:creator>pg</dc:creator>
      <content:encoded><![CDATA[<p>Encoded body</p>]]></content:encoded>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rrCreate.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles status = %d, want %d", rrList.Code, http.StatusOK)
	}
	var listResp struct {
		Articles []struct {
			ID int64 `json:"id"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}

	articleID := listResp.Articles[0].ID
	reqReadable := httptest.NewRequest(http.MethodPost, "/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/readability", nil)
	rrReadable := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrReadable, reqReadable)
	if rrReadable.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/articles/:id/readability status = %d, want %d, body=%s", rrReadable.Code, http.StatusOK, rrReadable.Body.String())
	}

	var detailResp struct {
		FullContent   string `json:"full_content"`
		SourcePayload *struct {
			Summary string `json:"summary"`
			Fields  []struct {
				Key       string `json:"key"`
				Value     string `json:"value"`
				ValueHTML string `json:"value_html"`
			} `json:"fields"`
		} `json:"source_payload"`
	}
	if err := json.Unmarshal(rrReadable.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("unmarshal readability response error = %v", err)
	}
	if !strings.Contains(detailResp.FullContent, "Readable service content") {
		t.Fatalf("full_content = %q, want readability content", detailResp.FullContent)
	}
	if detailResp.SourcePayload == nil {
		t.Fatalf("source_payload = nil, want non-nil")
	}
	if !strings.Contains(detailResp.SourcePayload.Summary, "Original RSS summary") {
		t.Fatalf("source_payload.summary = %q, want rss summary", detailResp.SourcePayload.Summary)
	}
	if !handlerHasSourceField(detailResp.SourcePayload.Fields, "comments", "https://news.ycombinator.com/item?id=1") {
		t.Fatalf("source_payload.fields = %+v, want comments field", detailResp.SourcePayload.Fields)
	}
	if !handlerHasSourceHTMLField(detailResp.SourcePayload.Fields, "encoded", "<p>Encoded body</p>") {
		t.Fatalf("source_payload.fields = %+v, want encoded html field", detailResp.SourcePayload.Fields)
	}
}

func handlerHasSourceField(fields []struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	ValueHTML string `json:"value_html"`
}, key string, value string) bool {
	for _, field := range fields {
		if field.Key == key && field.Value == value {
			return true
		}
	}
	return false
}

func handlerHasSourceHTMLField(fields []struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	ValueHTML string `json:"value_html"`
}, key string, html string) bool {
	for _, field := range fields {
		if field.Key == key && field.ValueHTML == html {
			return true
		}
	}
	return false
}

func TestArticleRefreshCache(t *testing.T) {
	articleHTML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body><article><p>Cache refresh readability text.</p></article></body></html>`))
	}))
	defer articleHTML.Close()

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Cache Refresh Feed</title>
    <item>
      <title>Cache Refresh Item</title>
      <link>` + articleHTML.URL + `</link>
      <description>desc</description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := repo
	server := NewServer(feedService, t.TempDir())

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rrCreate.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles status = %d, want %d", rrList.Code, http.StatusOK)
	}
	var listResp struct {
		Articles []struct {
			ID int64 `json:"id"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}

	articleID := listResp.Articles[0].ID
	reqRefresh := httptest.NewRequest(http.MethodPost, "/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/refresh-cache", nil)
	rrRefresh := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrRefresh, reqRefresh)
	if rrRefresh.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/articles/:id/refresh-cache status = %d, want %d, body=%s", rrRefresh.Code, http.StatusOK, rrRefresh.Body.String())
	}
	var detailResp struct {
		FullContent    string `json:"full_content"`
		DisplaySummary string `json:"display_summary"`
	}
	if err := json.Unmarshal(rrRefresh.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("unmarshal refresh-cache response error = %v", err)
	}
	if !strings.Contains(detailResp.FullContent, "Cache refresh readability text") {
		t.Fatalf("full_content = %q, want contains refreshed readability text", detailResp.FullContent)
	}
	if !strings.Contains(detailResp.DisplaySummary, "Cache refresh readability text") {
		t.Fatalf("display_summary = %q, want refreshed summary from readable content", detailResp.DisplaySummary)
	}
}

func TestArticleTranslateByAI(t *testing.T) {
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "choices": [
    {
      "message": {
        "content": "这是翻译后的中文内容。"
      }
    }
  ]
}`))
	}))
	defer aiMock.Close()

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Translate Feed</title>
    <item>
      <title>Hello World</title>
      <link>https://example.com/translate</link>
      <description><![CDATA[<p>Hello translation test.</p>]]></description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := repo
	server := NewServer(feedService, t.TempDir())

	reqSaveAI := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/ai", bytes.NewReader([]byte(`{
		"api_key":"test-key",
		"base_url":"`+aiMock.URL+`",
		"model":"test-model",
		"target_lang":"zh-CN"
	}`)))
	rrSaveAI := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrSaveAI, reqSaveAI)
	if rrSaveAI.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/settings/ai status = %d, want %d, body=%s", rrSaveAI.Code, http.StatusOK, rrSaveAI.Body.String())
	}

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rrCreate.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles status = %d, want %d", rrList.Code, http.StatusOK)
	}
	var listResp struct {
		Articles []struct {
			ID int64 `json:"id"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}

	articleID := listResp.Articles[0].ID
	reqTranslate := httptest.NewRequest(http.MethodPost, "/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/translate", bytes.NewReader([]byte(`{"target_lang":"zh-CN"}`)))
	rrTranslate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrTranslate, reqTranslate)
	if rrTranslate.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/articles/:id/translate status = %d, want %d, body=%s", rrTranslate.Code, http.StatusOK, rrTranslate.Body.String())
	}

	var translateResp struct {
		TranslatedText string `json:"translated_text"`
	}
	if err := json.Unmarshal(rrTranslate.Body.Bytes(), &translateResp); err != nil {
		t.Fatalf("unmarshal translate response error = %v", err)
	}
	if !strings.Contains(translateResp.TranslatedText, "翻译后的中文内容") {
		t.Fatalf("translated_text = %q, want contains translated output", translateResp.TranslatedText)
	}
}

func TestArticleTranslateStreamByParagraph(t *testing.T) {
	callCount := 0
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		content := "第1段译文"
		if callCount == 2 {
			content = "第2段译文"
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + content + `"}}]}`))
	}))
	defer aiMock.Close()

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Translate Stream Feed</title>
    <item>
      <title>Hello Stream</title>
      <link>https://example.com/translate-stream</link>
      <description><![CDATA[<p>First paragraph.</p><p>Second paragraph.</p>]]></description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := repo
	server := NewServer(feedService, t.TempDir())

	reqSaveAI := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/ai", bytes.NewReader([]byte(`{
		"api_key":"test-key",
		"base_url":"`+aiMock.URL+`",
		"model":"test-model",
		"target_lang":"zh-CN"
	}`)))
	rrSaveAI := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrSaveAI, reqSaveAI)
	if rrSaveAI.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/settings/ai status = %d, want %d, body=%s", rrSaveAI.Code, http.StatusOK, rrSaveAI.Body.String())
	}

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rrCreate.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)
	if rrList.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles status = %d, want %d", rrList.Code, http.StatusOK)
	}
	var listResp struct {
		Articles []struct {
			ID int64 `json:"id"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}

	articleID := listResp.Articles[0].ID
	reqStream := httptest.NewRequest(http.MethodPost, "/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/translate/stream", bytes.NewReader([]byte(`{"target_lang":"zh-CN","sources":["First paragraph","Second paragraph"]}`)))
	rrStream := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrStream, reqStream)
	if rrStream.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/articles/:id/translate/stream status = %d, want %d, body=%s", rrStream.Code, http.StatusOK, rrStream.Body.String())
	}
	if !strings.Contains(rrStream.Header().Get("Content-Type"), "application/x-ndjson") {
		t.Fatalf("content-type = %q, want ndjson", rrStream.Header().Get("Content-Type"))
	}

	lines := strings.Split(strings.TrimSpace(rrStream.Body.String()), "\n")
	if len(lines) < 4 {
		t.Fatalf("stream lines = %d, want >= 4, body=%s", len(lines), rrStream.Body.String())
	}
	var start struct {
		Type    string   `json:"type"`
		Total   int      `json:"total"`
		Sources []string `json:"sources"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &start); err != nil {
		t.Fatalf("unmarshal start event error = %v", err)
	}
	if start.Type != "start" || start.Total != 2 || len(start.Sources) != 2 {
		t.Fatalf("start event = %+v, want type=start total=2", start)
	}

	var chunk1 struct {
		Type       string `json:"type"`
		Index      int    `json:"index"`
		Source     string `json:"source"`
		Translated string `json:"translated"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &chunk1); err != nil {
		t.Fatalf("unmarshal chunk1 error = %v", err)
	}
	if chunk1.Type != "chunk" || chunk1.Index != 1 || !strings.Contains(chunk1.Source, "First paragraph") {
		t.Fatalf("chunk1 event mismatch: %+v", chunk1)
	}

	var chunk2 struct {
		Type  string `json:"type"`
		Index int    `json:"index"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &chunk2); err != nil {
		t.Fatalf("unmarshal chunk2 error = %v", err)
	}
	if chunk2.Type != "chunk" || chunk2.Index != 2 {
		t.Fatalf("chunk2 event mismatch: %+v", chunk2)
	}

	var done struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &done); err != nil {
		t.Fatalf("unmarshal done error = %v", err)
	}
	if done.Type != "done" {
		t.Fatalf("done event mismatch: %+v", done)
	}
}

func TestArticleFavoriteToggle(t *testing.T) {
	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Favorite Feed</title>
    <item>
      <title>Favorite Item</title>
      <link>https://example.com/favorite-item</link>
      <description>desc</description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rrCreate.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)
	var listResp struct {
		Articles []struct {
			ID         int64 `json:"id"`
			IsFavorite bool  `json:"is_favorite"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}

	articleID := listResp.Articles[0].ID
	reqFav := httptest.NewRequest(http.MethodPatch, "/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/favorite", bytes.NewReader([]byte(`{"favorite":true}`)))
	rrFav := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrFav, reqFav)
	if rrFav.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/articles/:id/favorite status = %d, want %d", rrFav.Code, http.StatusOK)
	}
	var favResp struct {
		IsFavorite bool `json:"is_favorite"`
	}
	if err := json.Unmarshal(rrFav.Body.Bytes(), &favResp); err != nil {
		t.Fatalf("unmarshal favorite response error = %v", err)
	}
	if !favResp.IsFavorite {
		t.Fatalf("is_favorite = false, want true")
	}
}

func TestDataSettingsAndRetentionCleanupKeepFavorites(t *testing.T) {
	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Retention Feed</title>
    <item>
      <title>Old Item A</title>
      <link>https://example.com/old-a</link>
      <description>desc</description>
      <pubDate>Wed, 25 Feb 2001 11:00:00 GMT</pubDate>
    </item>
    <item>
      <title>Old Item B</title>
      <link>https://example.com/old-b</link>
      <description>desc</description>
      <pubDate>Wed, 25 Feb 2001 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	reqSave := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/data", bytes.NewReader([]byte(`{"retention_days":1}`)))
	rrSave := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrSave, reqSave)
	if rrSave.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/settings/data status = %d, want %d", rrSave.Code, http.StatusOK)
	}

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/feeds", bytes.NewReader(createBody))
	rrCreate := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrCreate, reqCreate)
	if rrCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/feeds status = %d, want %d", rrCreate.Code, http.StatusCreated)
	}

	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)
	rrList := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList, reqList)
	var listResp struct {
		Articles []struct {
			ID int64 `json:"id"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 2 {
		t.Fatalf("articles len = %d, want 2", len(listResp.Articles))
	}

	keepID := listResp.Articles[0].ID
	reqFav := httptest.NewRequest(http.MethodPatch, "/api/v1/articles/"+strconv.FormatInt(keepID, 10)+"/favorite", bytes.NewReader([]byte(`{"favorite":true}`)))
	rrFav := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrFav, reqFav)
	if rrFav.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/articles/:id/favorite status = %d, want %d", rrFav.Code, http.StatusOK)
	}

	if err := server.RefreshAllFeeds(context.Background()); err != nil {
		t.Fatalf("RefreshAllFeeds() error = %v", err)
	}

	rrList2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrList2, reqList)
	var listResp2 struct {
		Articles []struct {
			ID         int64 `json:"id"`
			IsFavorite bool  `json:"is_favorite"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList2.Body.Bytes(), &listResp2); err != nil {
		t.Fatalf("unmarshal list response2 error = %v", err)
	}
	if len(listResp2.Articles) != 1 {
		t.Fatalf("articles len after cleanup = %d, want 1", len(listResp2.Articles))
	}
	if !listResp2.Articles[0].IsFavorite {
		t.Fatalf("remaining article is_favorite = false, want true")
	}
}

func TestDataSettingsDefaultRetentionDays(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings/data", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/settings/data status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp struct {
		RetentionDays int `json:"retention_days"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal data settings response error = %v", err)
	}
	if resp.RetentionDays != 7 {
		t.Fatalf("retention_days = %d, want 7", resp.RetentionDays)
	}
}

func TestRegenerateSummariesEndpoint(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	_, err = repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>第一篇摘要</p>"},
		{Title: "A2", Link: "https://example.com/2", Summary: "<p>第二篇摘要</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/data/regenerate-summaries", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/settings/data/regenerate-summaries status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Refreshed int `json:"refreshed"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal regenerate summaries response error = %v", err)
	}
	if resp.Refreshed != 2 {
		t.Fatalf("refreshed = %d, want 2", resp.Refreshed)
	}

	for _, article := range repo.ListArticles() {
		if article.DisplaySummary == "" {
			t.Fatalf("article %d display_summary is empty", article.ID)
		}
	}
}

func TestClearCurrentArticleAISummaryEndpoint(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	_, err = repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{
			Title:       "A1",
			Link:        "https://example.com/1",
			Summary:     "<p>RSS 摘要</p>",
			FullContent: "<article><p>正文内容</p></article>",
		},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	article := repo.ListArticles()[0]
	if err := repo.UpdateArticleSummaryState(article.ID, "AI 摘要内容", service.DisplaySummaryReady, "<p>AI 摘要内容</p>", service.DisplaySummaryReady); err != nil {
		t.Fatalf("UpdateArticleSummaryState() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dev/articles/"+strconv.FormatInt(article.ID, 10)+"/clear-ai-summary", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/dev/articles/:id/clear-ai-summary status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		AISummary            string `json:"ai_summary"`
		DisplaySummary       string `json:"display_summary"`
		DisplaySummaryStatus string `json:"display_summary_status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal clear current summary response error = %v", err)
	}
	if resp.AISummary != "" {
		t.Fatalf("ai_summary = %q, want empty", resp.AISummary)
	}
	if !strings.Contains(resp.DisplaySummary, "RSS 摘要") {
		t.Fatalf("display_summary = %q, want raw rss fallback", resp.DisplaySummary)
	}
	if resp.DisplaySummaryStatus != service.DisplaySummaryFallback {
		t.Fatalf("display_summary_status = %q, want %q", resp.DisplaySummaryStatus, service.DisplaySummaryFallback)
	}
}

func TestRefreshCurrentArticleAISummaryEndpoint(t *testing.T) {
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"刷新后的 AI 摘要"}}]}`))
	}))
	defer aiMock.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	if err := repo.SetSetting("ai_api_key", "test-key"); err != nil {
		t.Fatalf("SetSetting(ai_api_key) error = %v", err)
	}
	if err := repo.SetSetting("ai_base_url", aiMock.URL); err != nil {
		t.Fatalf("SetSetting(ai_base_url) error = %v", err)
	}
	if err := repo.SetSetting("ai_model", "test-model"); err != nil {
		t.Fatalf("SetSetting(ai_model) error = %v", err)
	}

	_, err = repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{
			Title:       "A1",
			Link:        "https://example.com/1",
			Summary:     "<p>RSS 原始摘要</p>",
			FullContent: "<article><p>正文内容足够长，可以重新生成 AI 摘要。</p></article>",
		},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	server := NewServer(repo, t.TempDir())
	article := repo.ListArticles()[0]
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dev/articles/"+strconv.FormatInt(article.ID, 10)+"/refresh-ai-summary", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/dev/articles/:id/refresh-ai-summary status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		AISummary            string `json:"ai_summary"`
		AISummaryStatus      string `json:"ai_summary_status"`
		DisplaySummary       string `json:"display_summary"`
		DisplaySummaryStatus string `json:"display_summary_status"`
		SummaryDebug         struct {
			Strategy            string `json:"strategy"`
			QueryMode           string `json:"query_mode"`
			ChunkCount          int    `json:"chunk_count"`
			WindowCount         int    `json:"window_count"`
			RewritePassed       bool   `json:"rewrite_passed"`
			FinalSentenceClosed bool   `json:"final_sentence_closed"`
			UsedAI              bool   `json:"used_ai"`
		} `json:"summary_debug"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal refresh ai summary response error = %v", err)
	}
	if resp.AISummary != "刷新后的 AI 摘要。" {
		t.Fatalf("ai_summary = %q, want refreshed ai summary", resp.AISummary)
	}
	if resp.AISummaryStatus != service.AISummaryReady {
		t.Fatalf("ai_summary_status = %q, want %q", resp.AISummaryStatus, service.AISummaryReady)
	}
	if !strings.Contains(resp.DisplaySummary, "刷新后的 AI 摘要") {
		t.Fatalf("display_summary = %q, want refreshed ai summary in display layer", resp.DisplaySummary)
	}
	if resp.DisplaySummaryStatus != service.DisplaySummaryReady {
		t.Fatalf("display_summary_status = %q, want %q", resp.DisplaySummaryStatus, service.DisplaySummaryReady)
	}
	if resp.SummaryDebug.Strategy == "" {
		t.Fatalf("summary_debug.strategy is empty, want debug info")
	}
	if resp.SummaryDebug.QueryMode == "" {
		t.Fatalf("summary_debug.query_mode is empty, want query mode info")
	}
	if !resp.SummaryDebug.UsedAI {
		t.Fatalf("summary_debug.used_ai = false, want true")
	}
	if !resp.SummaryDebug.FinalSentenceClosed {
		t.Fatalf("summary_debug.final_sentence_closed = false, want true")
	}
}

func TestClearRecentAISummariesEndpoint(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	server := NewServer(repo, t.TempDir())

	_, err = repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>摘要一</p>"},
		{Title: "A2", Link: "https://example.com/2", Summary: "<p>摘要二</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}
	for _, article := range repo.ListArticles() {
		if err := repo.UpdateArticleSummaryState(article.ID, "AI 摘要", service.DisplaySummaryReady, "<p>AI 摘要</p>", service.DisplaySummaryReady); err != nil {
			t.Fatalf("UpdateArticleSummaryState() error = %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/dev/articles/clear-recent-ai-summaries", nil)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/dev/articles/clear-recent-ai-summaries status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var resp struct {
		Cleared int `json:"cleared"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal clear recent summaries response error = %v", err)
	}
	if resp.Cleared != 2 {
		t.Fatalf("cleared = %d, want 2", resp.Cleared)
	}
	for _, article := range repo.ListArticles() {
		if article.AISummary != "" {
			t.Fatalf("article %d ai_summary = %q, want empty", article.ID, article.AISummary)
		}
		if article.DisplaySummaryStatus != service.DisplaySummaryFallback {
			t.Fatalf("article %d display_summary_status = %q, want fallback", article.ID, article.DisplaySummaryStatus)
		}
	}
}
