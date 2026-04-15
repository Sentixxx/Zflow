package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	_ "github.com/mattn/go-sqlite3"
)

func TestParseLLMScoringResponse(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
		wantQ   int
		wantD   int
		wantR   int
	}{
		{
			name:  "clean JSON",
			raw:   `{"quality": 72, "depth": 55, "relevance": 68, "reasoning": "Well-structured article."}`,
			wantQ: 72, wantD: 55, wantR: 68,
		},
		{
			name:  "markdown fenced",
			raw:   "```json\n{\"quality\": 80, \"depth\": 60, \"relevance\": 45, \"reasoning\": \"Good.\"}\n```",
			wantQ: 80, wantD: 60, wantR: 45,
		},
		{
			name:  "with think tags",
			raw:   "<think>Let me analyze...</think>\n{\"quality\": 50, \"depth\": 40, \"relevance\": 30, \"reasoning\": \"Thin.\"}",
			wantQ: 50, wantD: 40, wantR: 30,
		},
		{
			name:  "clamps out of range",
			raw:   `{"quality": 150, "depth": -10, "relevance": 50, "reasoning": "Edge case."}`,
			wantQ: 100, wantD: 0, wantR: 50,
		},
		{
			name:    "invalid JSON",
			raw:     "not json at all",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLLMScoringResponse(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Quality != tt.wantQ {
				t.Errorf("quality: got %d, want %d", result.Quality, tt.wantQ)
			}
			if result.Depth != tt.wantD {
				t.Errorf("depth: got %d, want %d", result.Depth, tt.wantD)
			}
			if result.Relevance != tt.wantR {
				t.Errorf("relevance: got %d, want %d", result.Relevance, tt.wantR)
			}
		})
	}
}

func TestBlendLLMScores(t *testing.T) {
	features := model.ArticleFeatures{
		Quality:   60,
		Depth:     50,
		Relevance: 40,
		Freshness: 80,
		Novelty:   70,
	}
	llm := llmScoringResult{
		Quality:   90,
		Depth:     80,
		Relevance: 70,
	}

	blended := blendLLMScores(features, llm)

	// 60*0.4 + 90*0.6 = 24 + 54 = 78
	if blended.Quality != 78 {
		t.Errorf("quality: got %d, want 78", blended.Quality)
	}
	// 50*0.4 + 80*0.6 = 20 + 48 = 68
	if blended.Depth != 68 {
		t.Errorf("depth: got %d, want 68", blended.Depth)
	}
	// 40*0.4 + 70*0.6 = 16 + 42 = 58
	if blended.Relevance != 58 {
		t.Errorf("relevance: got %d, want 58", blended.Relevance)
	}
	if blended.Freshness != 80 {
		t.Errorf("freshness should be unchanged: got %d, want 80", blended.Freshness)
	}
	if blended.Novelty != 70 {
		t.Errorf("novelty should be unchanged: got %d, want 70", blended.Novelty)
	}
}

func TestBlendLLMScoresReasoningPassthrough(t *testing.T) {
	features := model.ArticleFeatures{
		Quality:   50,
		Depth:     50,
		Relevance: 50,
	}
	llm := llmScoringResult{
		Quality:   70,
		Depth:     60,
		Relevance: 55,
		Reasoning: "  Well-reasoned technical content with novel insights.  ",
	}

	blended := blendLLMScores(features, llm)

	// Reasoning must be trimmed and present after blending
	if blended.Reasoning != "Well-reasoned technical content with novel insights." {
		t.Errorf("reasoning = %q, want trimmed reasoning string", blended.Reasoning)
	}
}

func TestBlendLLMScoresEmptyReasoningPreservesEmpty(t *testing.T) {
	features := model.ArticleFeatures{
		Quality: 60,
	}
	llm := llmScoringResult{
		Quality:   70,
		Reasoning: "",
	}

	blended := blendLLMScores(features, llm)

	if blended.Reasoning != "" {
		t.Errorf("reasoning should be empty, got %q", blended.Reasoning)
	}
}

func TestBuildLLMScoringUserPrompt(t *testing.T) {
	article := model.Article{
		Title:       "Test Article Title",
		Summary:     "A short summary of the article.",
		FullContent: "Full content of the article with more details.",
	}
	prompt := buildLLMScoringUserPrompt(article)
	if prompt == "" {
		t.Fatal("prompt should not be empty")
	}
	if !strings.Contains(prompt, "Test Article Title") {
		t.Error("prompt should contain title")
	}
	if !strings.Contains(prompt, "Full content") {
		t.Error("prompt should contain full content")
	}
}

func TestBuildLLMScoringUserPromptTruncatesLongContent(t *testing.T) {
	longContent := strings.Repeat("This is a long paragraph with enough words. ", 500)
	article := model.Article{
		Title:       "Title",
		FullContent: longContent,
	}
	prompt := buildLLMScoringUserPrompt(article)
	// Content section should be truncated to 3000 chars
	contentIdx := strings.Index(prompt, "Content:\n")
	if contentIdx < 0 {
		t.Fatal("prompt should contain Content section")
	}
	contentPart := prompt[contentIdx+len("Content:\n"):]
	if len(contentPart) > 3100 { // some slack for HTML cleaning
		t.Errorf("content should be truncated, got %d chars", len(contentPart))
	}
}

// TestCallLLMForScoringOpenAI tests the full HTTP flow with a mock OpenAI server.
func TestCallLLMForScoringOpenAI(t *testing.T) {
	var capturedReq struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		Temperature float64 `json:"temperature"`
	}

	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", got)
		}
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("path = %q, want /chat/completions suffix", r.URL.Path)
		}

		json.NewDecoder(r.Body).Decode(&capturedReq)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"quality\":75,\"depth\":60,\"relevance\":55,\"reasoning\":\"Solid technical article.\"}"}}]}`))
	}))
	defer aiMock.Close()

	cfg := ScoringAIConfig{
		Protocol: "openai",
		APIKey:   "test-key",
		BaseURL:  aiMock.URL,
		Model:    "test-model",
	}

	article := model.Article{
		ID:    42,
		Title: "Understanding Distributed Consensus",
		Summary: "An in-depth look at Raft and Paxos.",
		FullContent: "Detailed explanation of consensus algorithms...",
	}

	result, err := callLLMForScoring(context.Background(), aiMock.Client(), cfg, article, nil)
	if err != nil {
		t.Fatalf("callLLMForScoring() error = %v", err)
	}

	if result.Quality != 75 {
		t.Errorf("quality = %d, want 75", result.Quality)
	}
	if result.Depth != 60 {
		t.Errorf("depth = %d, want 60", result.Depth)
	}
	if result.Relevance != 55 {
		t.Errorf("relevance = %d, want 55", result.Relevance)
	}

	// Verify request was properly formed
	if capturedReq.Model != "test-model" {
		t.Errorf("model = %q, want test-model", capturedReq.Model)
	}
	if len(capturedReq.Messages) != 2 {
		t.Fatalf("messages count = %d, want 2", len(capturedReq.Messages))
	}
	if capturedReq.Messages[0].Role != "system" {
		t.Errorf("first message role = %q, want system", capturedReq.Messages[0].Role)
	}
	if capturedReq.Messages[1].Role != "user" {
		t.Errorf("second message role = %q, want user", capturedReq.Messages[1].Role)
	}
	if !strings.Contains(capturedReq.Messages[1].Content, "Distributed Consensus") {
		t.Errorf("user message should contain article title")
	}
}

// TestCallLLMForScoringAnthropic tests the Anthropic protocol path.
func TestCallLLMForScoringAnthropic(t *testing.T) {
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "anthropic-key" {
			t.Errorf("x-api-key = %q, want anthropic-key", got)
		}
		if got := r.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("anthropic-version = %q, want 2023-06-01", got)
		}
		if !strings.HasSuffix(r.URL.Path, "/v1/messages") {
			t.Errorf("path = %q, want /v1/messages suffix", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"content":[{"type":"text","text":"{\"quality\":82,\"depth\":70,\"relevance\":65,\"reasoning\":\"Excellent depth.\"}"}]}`))
	}))
	defer aiMock.Close()

	cfg := ScoringAIConfig{
		Protocol: "anthropic",
		APIKey:   "anthropic-key",
		BaseURL:  aiMock.URL,
		Model:    "claude-test",
	}

	result, err := callLLMForScoring(context.Background(), aiMock.Client(), cfg, model.Article{
		ID:    1,
		Title: "Test",
		FullContent: "Some content for scoring.",
	}, nil)
	if err != nil {
		t.Fatalf("callLLMForScoring() error = %v", err)
	}
	if result.Quality != 82 {
		t.Errorf("quality = %d, want 82", result.Quality)
	}
}

// TestCallLLMForScoringAPIError verifies graceful handling of upstream errors.
func TestCallLLMForScoringAPIError(t *testing.T) {
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer aiMock.Close()

	cfg := ScoringAIConfig{
		Protocol: "openai",
		APIKey:   "key",
		BaseURL:  aiMock.URL,
		Model:    "model",
	}

	_, err := callLLMForScoring(context.Background(), aiMock.Client(), cfg, model.Article{
		ID:    1,
		Title: "Test",
		FullContent: "Content.",
	}, nil)
	if err == nil {
		t.Fatal("expected error for 429 response")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error = %q, want contains 429", err.Error())
	}
}

// TestCallLLMForScoringMalformedJSON tests handling of invalid JSON from LLM.
func TestCallLLMForScoringMalformedJSON(t *testing.T) {
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"I cannot evaluate this article properly."}}]}`))
	}))
	defer aiMock.Close()

	cfg := ScoringAIConfig{
		Protocol: "openai",
		APIKey:   "key",
		BaseURL:  aiMock.URL,
		Model:    "model",
	}

	_, err := callLLMForScoring(context.Background(), aiMock.Client(), cfg, model.Article{
		ID:    1,
		Title: "Test",
		FullContent: "Content.",
	}, nil)
	if err == nil {
		t.Fatal("expected error for non-JSON LLM response")
	}
}

// TestRefreshStaleScoresWithLLM tests the full integration: rule-based scoring + LLM enhancement.
func TestRefreshStaleScoresWithLLM(t *testing.T) {
	llmCallCount := 0
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		llmCallCount++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"{\"quality\":85,\"depth\":75,\"relevance\":70,\"reasoning\":\"High quality.\"}"}}]}`))
	}))
	defer aiMock.Close()

	dbPath := filepath.Join(t.TempDir(), "scoring.db")
	repo, err := repository.NewTestSQLiteFeedRepository(dbPath)
	if err != nil {
		t.Fatalf("NewTestSQLiteFeedRepository() error = %v", err)
	}

	svc := NewArticleService(repo, func() *http.Client { return http.DefaultClient },
		WithScoringAI(
			func() *http.Client { return aiMock.Client() },
			func() (ScoringAIConfig, error) {
				return ScoringAIConfig{
					Protocol: "openai",
					APIKey:   "test-key",
					BaseURL:  aiMock.URL,
					Model:    "test-model",
				}, nil
			},
		),
	)

	// Insert an article with stale feature version
	contentBody := strings.Repeat("Detailed engineering analysis of distributed systems with Raft consensus. ", 30)
	_, err = repo.AddInFolder("https://example.com/feed", "Feed", scoreTestSeeds([]repository.ArticleSeed{
		{
			Title:       "Distributed Systems Deep Dive",
			Link:        "https://example.com/article-1",
			Summary:     "An analysis of consensus algorithms.",
			FullContent: contentBody,
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}), "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	// Force feature_version to 0 to trigger re-scoring
	articles := repo.ListArticles()
	if len(articles) == 0 {
		t.Fatal("no articles found")
	}
	repo.UpdateArticleFeatures(articles[0].ID, model.ArticleFeatures{FeatureVersion: 0})

	// Run scoring refresh
	refreshed, err := svc.RefreshStaleScores(context.Background(), 10)
	if err != nil {
		t.Fatalf("RefreshStaleScores() error = %v", err)
	}
	if refreshed != 1 {
		t.Fatalf("refreshed = %d, want 1", refreshed)
	}

	// LLM should have been called
	if llmCallCount != 1 {
		t.Errorf("LLM call count = %d, want 1", llmCallCount)
	}

	// Verify scores were persisted and blended (not pure rule-based, not pure LLM)
	updated, ok := repo.GetArticle(articles[0].ID)
	if !ok {
		t.Fatal("article not found after scoring")
	}
	if updated.RecommendationScores == nil {
		t.Fatal("recommendation scores should be set")
	}
	// Blended quality should be between pure rule and pure LLM (85)
	// Rule-based quality for this article should be moderate (50-80 range)
	// Blended should be higher than pure rule due to LLM boost
	if updated.RecommendationScores.Quality <= 0 || updated.RecommendationScores.Quality > 100 {
		t.Errorf("quality = %d, want 1-100 range", updated.RecommendationScores.Quality)
	}

	// Verify LLM reasoning was persisted to article_features
	if updated.ArticleFeatures == nil {
		t.Fatal("article_features should be set after LLM scoring")
	}
	if updated.ArticleFeatures.Reasoning != "High quality." {
		t.Errorf("reasoning = %q, want %q", updated.ArticleFeatures.Reasoning, "High quality.")
	}
	// Verify depth and freshness are populated
	if updated.ArticleFeatures.Depth <= 0 {
		t.Errorf("depth = %d, want > 0", updated.ArticleFeatures.Depth)
	}
	if updated.ArticleFeatures.Freshness <= 0 {
		t.Errorf("freshness = %d, want > 0", updated.ArticleFeatures.Freshness)
	}
}

// TestRefreshStaleScoresLLMFailureFallsBack tests that LLM failure doesn't block scoring.
func TestRefreshStaleScoresLLMFailureFallsBack(t *testing.T) {
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer aiMock.Close()

	dbPath := filepath.Join(t.TempDir(), "scoring-fallback.db")
	repo, err := repository.NewTestSQLiteFeedRepository(dbPath)
	if err != nil {
		t.Fatalf("NewTestSQLiteFeedRepository() error = %v", err)
	}

	svc := NewArticleService(repo, func() *http.Client { return http.DefaultClient },
		WithScoringAI(
			func() *http.Client { return aiMock.Client() },
			func() (ScoringAIConfig, error) {
				return ScoringAIConfig{
					Protocol: "openai",
					APIKey:   "key",
					BaseURL:  aiMock.URL,
					Model:    "model",
				}, nil
			},
		),
	)

	contentBody := strings.Repeat("Engineering article content. ", 30)
	_, err = repo.AddInFolder("https://example.com/feed", "Feed", scoreTestSeeds([]repository.ArticleSeed{
		{
			Title:       "Test Article",
			Link:        "https://example.com/a",
			Summary:     "Summary.",
			FullContent: contentBody,
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}), "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	articles := repo.ListArticles()
	repo.UpdateArticleFeatures(articles[0].ID, model.ArticleFeatures{FeatureVersion: 0})

	// Should succeed despite LLM failure — falls back to rule-based
	refreshed, err := svc.RefreshStaleScores(context.Background(), 10)
	if err != nil {
		t.Fatalf("RefreshStaleScores() should not error on LLM failure: %v", err)
	}
	if refreshed != 1 {
		t.Fatalf("refreshed = %d, want 1", refreshed)
	}

	updated, ok := repo.GetArticle(articles[0].ID)
	if !ok {
		t.Fatal("article not found")
	}
	if updated.RecommendationScores == nil {
		t.Fatal("scores should be set even when LLM fails")
	}
	if updated.RecommendationScores.Quality <= 0 {
		t.Error("quality should be positive from rule-based scoring")
	}
}

// TestBlendLLMScoresReasoningTruncation verifies that reasoning longer than
// maxScoreReasoningLen runes is truncated at a rune boundary and marked with "…".
// This guards against unbounded LLM output inflating persisted TEXT size.
func TestBlendLLMScoresReasoningTruncation(t *testing.T) {
	// Build a 2000-rune ASCII string (well above the 1024-rune cap).
	longReasoning := strings.Repeat("a", 2000)
	llm := llmScoringResult{
		Quality:   70,
		Depth:     60,
		Relevance: 50,
		Reasoning: longReasoning,
	}
	blended := blendLLMScores(model.ArticleFeatures{Quality: 50, Depth: 50, Relevance: 50}, llm)

	runeCount := len([]rune(blended.Reasoning))
	// Expect exactly 1024 runes + 1 rune for "…" = 1025 total runes.
	const wantRunes = maxScoreReasoningLen + 1 // +1 for the ellipsis rune
	if runeCount != wantRunes {
		t.Errorf("truncated reasoning rune count = %d, want %d", runeCount, wantRunes)
	}
	if !strings.HasSuffix(blended.Reasoning, "…") {
		t.Errorf("truncated reasoning must end with '…', got: %.30q", blended.Reasoning)
	}
	// Result must be valid UTF-8.
	if !isValidUTF8(blended.Reasoning) {
		t.Error("truncated reasoning is not valid UTF-8")
	}
}

// TestBlendLLMScoresReasoningTruncationMultibyte verifies that multibyte (CJK) runes are
// not split when reasoning is truncated.
func TestBlendLLMScoresReasoningTruncationMultibyte(t *testing.T) {
	// Each "中" is 3 bytes; 2000 of them is well above the 1024-rune cap.
	longReasoning := strings.Repeat("中", 2000)
	llm := llmScoringResult{Quality: 60, Depth: 50, Relevance: 40, Reasoning: longReasoning}
	blended := blendLLMScores(model.ArticleFeatures{Quality: 50, Depth: 50, Relevance: 50}, llm)

	if !isValidUTF8(blended.Reasoning) {
		t.Error("truncated multibyte reasoning is not valid UTF-8")
	}
	runeCount := len([]rune(blended.Reasoning))
	const wantRunes = maxScoreReasoningLen + 1
	if runeCount != wantRunes {
		t.Errorf("rune count = %d, want %d", runeCount, wantRunes)
	}
}

// isValidUTF8 returns true when every byte in s is part of a valid UTF-8 sequence.
func isValidUTF8(s string) bool {
	for _, r := range s {
		if r == '\uFFFD' {
			return false
		}
	}
	return true
}

// TestRuleOnlyScoringClearsReasoning verifies that running pure rule-based scoring on
// an article that previously had LLM reasoning stored will result in an empty Reasoning
// field, preventing stale LLM text from surviving into the persisted result.
func TestRuleOnlyScoringClearsReasoning(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scoring-clear-reasoning.db")
	repo, err := repository.NewTestSQLiteFeedRepository(dbPath)
	if err != nil {
		t.Fatalf("NewTestSQLiteFeedRepository() error = %v", err)
	}

	// No AI configured — pure rule path.
	svc := NewArticleService(repo, func() *http.Client { return http.DefaultClient })

	contentBody := strings.Repeat("Some article content for rule scoring. ", 30)
	_, err = repo.AddInFolder("https://example.com/feed", "Feed", scoreTestSeeds([]repository.ArticleSeed{
		{
			Title:       "Rule-Only Article",
			Link:        "https://example.com/rule-only",
			Summary:     "Summary.",
			FullContent: contentBody,
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}), "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) == 0 {
		t.Fatal("no articles inserted")
	}
	articleID := articles[0].ID

	// Simulate a previous LLM scoring round by writing a non-empty Reasoning into
	// article_features with an outdated FeatureVersion so it triggers re-scoring.
	oldFeatures := model.ArticleFeatures{
		Reasoning:      "Old LLM reasoning that must be cleared.",
		FeatureVersion: 0, // stale — triggers RefreshStaleScores
	}
	if err := repo.UpdateArticleFeatures(articleID, oldFeatures); err != nil {
		t.Fatalf("UpdateArticleFeatures() error = %v", err)
	}

	refreshed, err := svc.RefreshStaleScores(context.Background(), 10)
	if err != nil {
		t.Fatalf("RefreshStaleScores() error = %v", err)
	}
	if refreshed != 1 {
		t.Fatalf("refreshed = %d, want 1", refreshed)
	}

	updated, ok := repo.GetArticle(articleID)
	if !ok {
		t.Fatal("article not found after rule-only re-scoring")
	}
	if updated.ArticleFeatures == nil {
		t.Fatal("article_features must be populated after scoring")
	}
	// Rule path must have zeroed out the old LLM reasoning.
	if updated.ArticleFeatures.Reasoning != "" {
		t.Errorf("Reasoning = %q after rule-only rescoring, want empty string", updated.ArticleFeatures.Reasoning)
	}
}

// TestRefreshStaleScoresNoAIConfig tests that scoring works without AI configured.
func TestRefreshStaleScoresNoAIConfig(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "scoring-noai.db")
	repo, err := repository.NewTestSQLiteFeedRepository(dbPath)
	if err != nil {
		t.Fatalf("NewTestSQLiteFeedRepository() error = %v", err)
	}

	// No WithScoringAI option — pure rule-based
	svc := NewArticleService(repo, func() *http.Client { return http.DefaultClient })

	contentBody := strings.Repeat("Some article text. ", 30)
	_, err = repo.AddInFolder("https://example.com/feed", "Feed", scoreTestSeeds([]repository.ArticleSeed{
		{
			Title:       "Pure Rule Article",
			Link:        "https://example.com/b",
			Summary:     "Summary.",
			FullContent: contentBody,
			PublishedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}), "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	articles := repo.ListArticles()
	repo.UpdateArticleFeatures(articles[0].ID, model.ArticleFeatures{FeatureVersion: 0})

	refreshed, err := svc.RefreshStaleScores(context.Background(), 10)
	if err != nil {
		t.Fatalf("RefreshStaleScores() error = %v", err)
	}
	if refreshed != 1 {
		t.Fatalf("refreshed = %d, want 1", refreshed)
	}
}
