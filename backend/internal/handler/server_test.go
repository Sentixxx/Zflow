package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Sentixxx/Zflow/backend/internal/repository"
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

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
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

func TestAISettingsGetAndPatch(t *testing.T) {
	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
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
		"target_lang":"ja"
	}`)))
	rrPatch := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrPatch, reqPatch)
	if rrPatch.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/settings/ai status = %d, want %d, body=%s", rrPatch.Code, http.StatusOK, rrPatch.Body.String())
	}

	rrGet2 := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrGet2, reqGet)
	if rrGet2.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/settings/ai status(after patch) = %d, want %d", rrGet2.Code, http.StatusOK)
	}
	var resp struct {
		APIKey     string `json:"api_key"`
		BaseURL    string `json:"base_url"`
		Model      string `json:"model"`
		TargetLang string `json:"target_lang"`
	}
	if err := json.Unmarshal(rrGet2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal ai settings response error = %v", err)
	}
	if resp.APIKey != "test-ai-key" || resp.BaseURL != "https://example-ai.local/v1" || resp.Model != "test-model" || resp.TargetLang != "ja" {
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

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
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
		FullContent string `json:"full_content"`
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

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
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

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
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
		FullContent string `json:"full_content"`
	}
	if err := json.Unmarshal(rrRefresh.Body.Bytes(), &detailResp); err != nil {
		t.Fatalf("unmarshal refresh-cache response error = %v", err)
	}
	if !strings.Contains(detailResp.FullContent, "Cache refresh readability text") {
		t.Fatalf("full_content = %q, want contains refreshed readability text", detailResp.FullContent)
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

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
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

	repo, err := repository.NewSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.json"))
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
	reqStream := httptest.NewRequest(http.MethodPost, "/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/translate/stream", bytes.NewReader([]byte(`{"target_lang":"zh-CN"}`)))
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
