package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/internal/service"
)

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

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := service.NewFeedService(repo)
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

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := service.NewFeedService(repo)
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
		} `json:"articles"`
	}
	if err := json.Unmarshal(rrList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal list response error = %v", err)
	}
	if len(listResp.Articles) != 1 {
		t.Fatalf("articles len = %d, want 1", len(listResp.Articles))
	}

	articleID := listResp.Articles[0].ID
	reqDetail := httptest.NewRequest(http.MethodGet, "/api/v1/articles/"+strconv.FormatInt(articleID, 10), nil)
	rrDetail := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrDetail, reqDetail)
	if rrDetail.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/articles/:id status = %d, want %d", rrDetail.Code, http.StatusOK)
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

func TestCORSPreflightAndHeaders(t *testing.T) {
	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feedService := service.NewFeedService(repo)
	server := NewServer(feedService, t.TempDir())

	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/articles", nil)
	preflight.Header.Set("Origin", "http://localhost:5173")
	preflight.Header.Set("Access-Control-Request-Method", "GET")
	rrPreflight := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrPreflight, preflight)

	if rrPreflight.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS /api/v1/articles status = %d, want %d", rrPreflight.Code, http.StatusNoContent)
	}
	if rrPreflight.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", rrPreflight.Header().Get("Access-Control-Allow-Origin"))
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", rr.Header().Get("Access-Control-Allow-Origin"))
	}
}
