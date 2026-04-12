package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Sentixxx/Zflow/backend/internal/repository"
)

// ---------------------------------------------------------------------------
// Pure function tests: buildTranslationContext
// ---------------------------------------------------------------------------

func TestBuildTranslationContext_Empty(t *testing.T) {
	got := buildTranslationContext(nil)
	if got != "(none)" {
		t.Fatalf("buildTranslationContext(nil) = %q, want %q", got, "(none)")
	}
}

func TestBuildTranslationContext_SinglePair(t *testing.T) {
	history := []translationPair{
		{Source: "Hello world", Translated: "你好世界"},
	}
	got := buildTranslationContext(history)
	if !strings.Contains(got, "Hello world") || !strings.Contains(got, "你好世界") {
		t.Fatalf("single pair context missing content: %q", got)
	}
	if !strings.Contains(got, "Segment 1") {
		t.Fatalf("single pair context missing segment number: %q", got)
	}
}

func TestBuildTranslationContext_PinsFirstParagraph(t *testing.T) {
	// Build 8 segments; the first one should always be present
	// even though only 5 pairs fit in the window.
	history := make([]translationPair, 8)
	history[0] = translationPair{
		Source:     "ANCHOR_TERM: distributed consensus protocol",
		Translated: "锚定术语：分布式共识协议",
	}
	for i := 1; i < 8; i++ {
		history[i] = translationPair{
			Source:     "Paragraph " + strconv.Itoa(i+1),
			Translated: "第" + strconv.Itoa(i+1) + "段",
		}
	}

	got := buildTranslationContext(history)

	// First segment must be pinned
	if !strings.Contains(got, "ANCHOR_TERM") {
		t.Fatalf("first segment not pinned in context: %q", got)
	}
	if !strings.Contains(got, "锚定术语") {
		t.Fatalf("first segment translation not pinned: %q", got)
	}
	// Most recent segment must also be present
	if !strings.Contains(got, "Paragraph 8") {
		t.Fatalf("most recent segment missing: %q", got)
	}
	// Middle segments (2-3) should have been evicted
	if strings.Contains(got, "Paragraph 2") {
		t.Fatalf("early middle segment should be evicted: %q", got)
	}
}

func TestBuildTranslationContext_CharLimit(t *testing.T) {
	// Each pair ~ 500 chars; with maxChars=2400, at most ~4-5 fit.
	history := make([]translationPair, 10)
	longText := strings.Repeat("word ", 80) // ~400 chars
	for i := 0; i < 10; i++ {
		history[i] = translationPair{
			Source:     longText + strconv.Itoa(i),
			Translated: longText + "翻译" + strconv.Itoa(i),
		}
	}
	got := buildTranslationContext(history)
	// Should not exceed reasonable length (2400 + some overhead from last entry)
	if len(got) > 3000 {
		t.Fatalf("context too long: %d chars", len(got))
	}
	// First segment always pinned
	if !strings.Contains(got, "word word") {
		t.Fatalf("context appears empty: %q", got[:100])
	}
}

func TestBuildTranslationContext_SkipsEmptyPairs(t *testing.T) {
	history := []translationPair{
		{Source: "Real content", Translated: "真实内容"},
		{Source: "", Translated: ""},
		{Source: "   ", Translated: "   "},
		{Source: "More content", Translated: "更多内容"},
	}
	got := buildTranslationContext(history)
	if !strings.Contains(got, "Real content") {
		t.Fatalf("non-empty pair missing: %q", got)
	}
	if !strings.Contains(got, "More content") {
		t.Fatalf("second non-empty pair missing: %q", got)
	}
	// Should not contain empty segment entries
	if strings.Contains(got, "Segment 2") || strings.Contains(got, "Segment 3") {
		t.Fatalf("empty pairs should be skipped: %q", got)
	}
}

func TestSelectContextIndices_SmallHistory(t *testing.T) {
	history := make([]translationPair, 3)
	indices := selectContextIndices(history, 5)
	if len(indices) != 3 || indices[0] != 0 || indices[1] != 1 || indices[2] != 2 {
		t.Fatalf("small history indices = %v, want [0 1 2]", indices)
	}
}

func TestSelectContextIndices_LargeHistory(t *testing.T) {
	history := make([]translationPair, 10)
	indices := selectContextIndices(history, 5)
	// Should be: [0, 6, 7, 8, 9] — first + last 4
	if len(indices) != 5 {
		t.Fatalf("indices len = %d, want 5", len(indices))
	}
	if indices[0] != 0 {
		t.Fatalf("first index = %d, want 0", indices[0])
	}
	if indices[len(indices)-1] != 9 {
		t.Fatalf("last index = %d, want 9", indices[len(indices)-1])
	}
}

// ---------------------------------------------------------------------------
// Pure function tests: buildTranslationSystemPrompt
// ---------------------------------------------------------------------------

func TestBuildTranslationSystemPrompt_Full(t *testing.T) {
	ctx := articleContext{
		Title:     "Rust vs Go Performance Benchmark",
		AISummary: "This article compares runtime performance of Rust and Go across various workloads.",
		FeedTitle: "Ars Technica",
	}
	got := buildTranslationSystemPrompt(ctx)

	for _, want := range []string{
		"precise translator",
		"Rust vs Go Performance Benchmark",
		"Ars Technica",
		"compares runtime performance",
		"Article context",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("system prompt missing %q in: %q", want, got)
		}
	}
}

func TestBuildTranslationSystemPrompt_NoSummary(t *testing.T) {
	ctx := articleContext{
		Title:     "Breaking News",
		FeedTitle: "Reuters",
	}
	got := buildTranslationSystemPrompt(ctx)
	if !strings.Contains(got, "Breaking News") {
		t.Errorf("missing title: %q", got)
	}
	if !strings.Contains(got, "Reuters") {
		t.Errorf("missing feed title: %q", got)
	}
	if strings.Contains(got, "Summary:") {
		t.Errorf("should not contain Summary field when empty: %q", got)
	}
}

func TestBuildTranslationSystemPrompt_Empty(t *testing.T) {
	got := buildTranslationSystemPrompt(articleContext{})
	if !strings.Contains(got, "precise translator") {
		t.Errorf("base prompt missing: %q", got)
	}
	if strings.Contains(got, "Article context") {
		t.Errorf("should not contain article context block when empty: %q", got)
	}
}

func TestBuildTranslationSystemPrompt_LongSummaryTruncated(t *testing.T) {
	ctx := articleContext{
		Title:     "Test",
		AISummary: strings.Repeat("很长的摘要内容", 200), // >> 600 chars
	}
	got := buildTranslationSystemPrompt(ctx)
	if !strings.Contains(got, "Summary:") {
		t.Errorf("summary field missing: %q", got)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "...") {
		t.Errorf("long summary should be truncated with '...': %q", got[len(got)-50:])
	}
}

// ---------------------------------------------------------------------------
// httptest integration: prompt capture and verification
// ---------------------------------------------------------------------------

// capturedRequest stores the decoded request body from a mock LLM call.
type capturedRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func TestTranslateStream_PromptContainsArticleContext(t *testing.T) {
	var mu sync.Mutex
	var captured []capturedRequest

	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req capturedRequest
		_ = json.Unmarshal(body, &req)
		mu.Lock()
		captured = append(captured, req)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"译文"}}]}`))
	}))
	defer aiMock.Close()

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Tech Research Weekly</title>
    <item>
      <title>WebAssembly Reaches Native Speed</title>
      <link>https://example.com/wasm</link>
      <description><![CDATA[<p>First paragraph about Wasm.</p><p>Second paragraph about benchmarks.</p>]]></description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(repo, t.TempDir())

	// Save AI settings pointing to mock
	patchAI(t, server, aiMock.URL)

	// Create feed + fetch articles
	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	doRequest(t, server, http.MethodPost, "/api/v1/feeds", createBody, http.StatusCreated)

	articleID := getFirstArticleID(t, server)

	// Clear any captured requests from feed creation (summary generation)
	mu.Lock()
	captured = nil
	mu.Unlock()

	// Stream translate
	doRequest(t, server, http.MethodPost,
		"/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/translate/stream",
		[]byte(`{"target_lang":"zh-CN","sources":["First paragraph about Wasm","Second paragraph about benchmarks"]}`),
		http.StatusOK,
	)

	mu.Lock()
	defer mu.Unlock()

	if len(captured) < 1 {
		t.Fatal("no LLM calls captured")
	}

	// Verify system prompt contains article title and feed source
	firstCall := captured[0]
	if len(firstCall.Messages) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(firstCall.Messages))
	}
	systemPrompt := firstCall.Messages[0].Content
	if !strings.Contains(systemPrompt, "WebAssembly Reaches Native Speed") {
		t.Errorf("system prompt missing article title: %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Tech Research Weekly") {
		t.Errorf("system prompt missing feed title: %q", systemPrompt)
	}
}

func TestTranslateStream_SlidingWindowProgression(t *testing.T) {
	var mu sync.Mutex
	var captured []capturedRequest

	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req capturedRequest
		_ = json.Unmarshal(body, &req)
		mu.Lock()
		callIdx := len(captured)
		captured = append(captured, req)
		mu.Unlock()

		content := "翻译段落" + strconv.Itoa(callIdx+1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + content + `"}}]}`))
	}))
	defer aiMock.Close()

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <item>
      <title>Sliding Window Test</title>
      <link>https://example.com/sw</link>
      <description>Test article</description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(repo, t.TempDir())
	patchAI(t, server, aiMock.URL)

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	doRequest(t, server, http.MethodPost, "/api/v1/feeds", createBody, http.StatusCreated)
	articleID := getFirstArticleID(t, server)

	// Clear any captured requests from feed creation (summary generation)
	mu.Lock()
	captured = nil
	mu.Unlock()

	// Translate 3 paragraphs
	doRequest(t, server, http.MethodPost,
		"/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/translate/stream",
		[]byte(`{"target_lang":"zh-CN","sources":["First paragraph establishes key terms","Second paragraph continues discussion","Third paragraph concludes"]}`),
		http.StatusOK,
	)

	mu.Lock()
	defer mu.Unlock()

	if len(captured) != 3 {
		t.Fatalf("expected 3 LLM calls, got %d", len(captured))
	}

	// First call: no prior context
	userMsg1 := captured[0].Messages[1].Content
	if !strings.Contains(userMsg1, "(none)") {
		t.Errorf("first call should have no prior context, got: %q", userMsg1)
	}

	// Second call: should contain first paragraph translation
	userMsg2 := captured[1].Messages[1].Content
	if !strings.Contains(userMsg2, "翻译段落1") {
		t.Errorf("second call missing first paragraph translation in context: %q", userMsg2)
	}

	// Third call: should contain both prior translations
	userMsg3 := captured[2].Messages[1].Content
	if !strings.Contains(userMsg3, "翻译段落1") {
		t.Errorf("third call missing first paragraph translation: %q", userMsg3)
	}
	if !strings.Contains(userMsg3, "翻译段落2") {
		t.Errorf("third call missing second paragraph translation: %q", userMsg3)
	}
}

func TestTranslateStream_AnthropicProtocol(t *testing.T) {
	var mu sync.Mutex
	var capturedHeaders http.Header
	var capturedBody []byte

	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedHeaders = r.Header.Clone()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"人类学译文"}]}`))
	}))
	defer aiMock.Close()

	feedXML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Anthropic Feed</title>
    <item>
      <title>Claude Research Paper</title>
      <link>https://example.com/claude</link>
      <description>Test content</description>
      <pubDate>Wed, 25 Feb 2026 11:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedXML.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	server := NewServer(repo, t.TempDir())

	// Save AI settings with Anthropic protocol
	reqSaveAI := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/ai", bytes.NewReader([]byte(`{
		"api_key":"test-anthropic-key",
		"base_url":"`+aiMock.URL+`",
		"model":"claude-sonnet-4-20250514",
		"target_lang":"zh-CN",
		"protocol":"anthropic"
	}`)))
	rrSaveAI := httptest.NewRecorder()
	server.Handler().ServeHTTP(rrSaveAI, reqSaveAI)
	if rrSaveAI.Code != http.StatusOK {
		t.Fatalf("PATCH settings status = %d, body = %s", rrSaveAI.Code, rrSaveAI.Body.String())
	}

	createBody, _ := json.Marshal(map[string]string{"url": feedXML.URL})
	doRequest(t, server, http.MethodPost, "/api/v1/feeds", createBody, http.StatusCreated)
	articleID := getFirstArticleID(t, server)

	doRequest(t, server, http.MethodPost,
		"/api/v1/articles/"+strconv.FormatInt(articleID, 10)+"/translate/stream",
		[]byte(`{"target_lang":"zh-CN","sources":["Test paragraph"]}`),
		http.StatusOK,
	)

	mu.Lock()
	defer mu.Unlock()

	// Verify Anthropic-specific headers
	if capturedHeaders.Get("X-Api-Key") != "test-anthropic-key" {
		t.Errorf("expected x-api-key header, got headers: %v", capturedHeaders)
	}
	if capturedHeaders.Get("Authorization") != "" {
		t.Errorf("anthropic protocol should not use Authorization header")
	}

	// Verify body contains system prompt with article context
	var body map[string]any
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	systemPrompt, _ := body["system"].(string)
	if !strings.Contains(systemPrompt, "Claude Research Paper") {
		t.Errorf("anthropic system prompt missing article title: %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, "Anthropic Feed") {
		t.Errorf("anthropic system prompt missing feed title: %q", systemPrompt)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func patchAI(t *testing.T, server *Server, mockURL string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings/ai", bytes.NewReader([]byte(`{
		"api_key":"test-key",
		"base_url":"`+mockURL+`",
		"model":"test-model",
		"target_lang":"zh-CN"
	}`)))
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("patchAI: status = %d, body = %s", rr.Code, rr.Body.String())
	}
}

func doRequest(t *testing.T, server *Server, method, path string, body []byte, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != wantStatus {
		t.Fatalf("%s %s: status = %d, want %d, body = %s", method, path, rr.Code, wantStatus, rr.Body.String())
	}
	return rr
}

func getFirstArticleID(t *testing.T, server *Server) int64 {
	t.Helper()
	rr := doRequest(t, server, http.MethodGet, "/api/v1/articles", nil, http.StatusOK)
	var listResp struct {
		Articles []struct {
			ID int64 `json:"id"`
		} `json:"articles"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("unmarshal articles: %v", err)
	}
	if len(listResp.Articles) == 0 {
		t.Fatal("no articles found")
	}
	return listResp.Articles[0].ID
}
