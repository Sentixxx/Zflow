package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/internal/repository/mock"
)

// stubVectorRepo is a local in-memory implementation of repository.VectorRepository
// used by the embedding service tests. Keeping it local avoids widening the public
// repository/mock package surface just for these tests.
type stubVectorRepo struct {
	mu            sync.Mutex
	embeddings    map[string][]float32 // key: sourceType|sourceID
	storedModel   map[string]string
	storeCount    int
	hasCount      int
	searchCount   int
	storeErr      error
	hasErr        error
	searchErr     error
	searchResults []repository.VectorMatch
}

func newStubVectorRepo() *stubVectorRepo {
	return &stubVectorRepo{
		embeddings:  make(map[string][]float32),
		storedModel: make(map[string]string),
	}
}

func vecKey(sourceType string, id int64) string {
	return fmt.Sprintf("%s|%d", sourceType, id)
}

func (s *stubVectorRepo) StoreEmbedding(_ context.Context, sourceType string, sourceID int64, model string, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storeCount++
	if s.storeErr != nil {
		return s.storeErr
	}
	k := vecKey(sourceType, sourceID)
	cp := make([]float32, len(embedding))
	copy(cp, embedding)
	s.embeddings[k] = cp
	s.storedModel[k] = model
	return nil
}

func (s *stubVectorRepo) SearchSimilar(_ context.Context, _ string, _ []float32, _ int) ([]repository.VectorMatch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.searchCount++
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return s.searchResults, nil
}

func (s *stubVectorRepo) FindSimilarToSource(_ context.Context, _ string, _ int64, _ float64, _ int) ([]repository.VectorMatch, error) {
	return nil, nil
}

func (s *stubVectorRepo) DeleteEmbedding(_ context.Context, sourceType string, sourceID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.embeddings, vecKey(sourceType, sourceID))
	return nil
}

func (s *stubVectorRepo) HasEmbedding(_ context.Context, sourceType string, sourceID int64) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasCount++
	if s.hasErr != nil {
		return false, s.hasErr
	}
	_, ok := s.embeddings[vecKey(sourceType, sourceID)]
	return ok, nil
}

func (s *stubVectorRepo) GetEmbedding(_ context.Context, sourceType string, sourceID int64) ([]float32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.embeddings[vecKey(sourceType, sourceID)]
	if !ok {
		return nil, errors.New("not found")
	}
	return v, nil
}

func (s *stubVectorRepo) snapshotCounts() (store, has, search int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.storeCount, s.hasCount, s.searchCount
}

// mockEmbeddingServer creates an httptest.Server that mimics the OpenAI-compatible
// embeddings endpoint. It records every call count and returns a configurable vector.
type mockEmbeddingServer struct {
	srv       *httptest.Server
	callCount int
	mu        sync.Mutex
	dim       int
	status    int
	bodyOK    []byte
	bodyBad   []byte
}

func newMockEmbeddingServer(t *testing.T, dim int) *mockEmbeddingServer {
	t.Helper()
	m := &mockEmbeddingServer{dim: dim, status: http.StatusOK}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.callCount++
		status := m.status
		badBody := m.bodyBad
		m.mu.Unlock()

		if !strings.HasSuffix(r.URL.Path, "/embeddings") {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "bad path", http.StatusNotFound)
			return
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "Bearer ") {
			t.Errorf("Authorization header = %q, want Bearer prefix", got)
		}

		// Decode input to keep the assertions meaningful.
		var body embeddingAPIRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode body: %v", err)
		}

		if status != http.StatusOK {
			w.WriteHeader(status)
			if len(badBody) > 0 {
				_, _ = w.Write(badBody)
			} else {
				_, _ = w.Write([]byte(`{"error":{"message":"forced failure"}}`))
			}
			return
		}

		vec := make([]float32, dim)
		for i := range vec {
			vec[i] = 0.01 * float32(i+1)
		}
		resp := embeddingAPIResponse{}
		resp.Data = append(resp.Data, struct {
			Embedding []float32 `json:"embedding"`
		}{Embedding: vec})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	return m
}

func (m *mockEmbeddingServer) setFail(status int, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = status
	m.bodyBad = body
}

func (m *mockEmbeddingServer) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *mockEmbeddingServer) close() {
	m.srv.Close()
}

// makeEmbeddingService wires a service with the provided vector and feed repos plus
// a test config pointing at the given mock server (or empty URL / empty key variants).
func makeEmbeddingService(vec *stubVectorRepo, feed repository.FeedRepository, cfg EmbeddingConfig) *EmbeddingService {
	return NewEmbeddingService(
		vec,
		feed,
		func() *http.Client { return http.DefaultClient },
		func() (EmbeddingConfig, error) { return cfg, nil },
	)
}

// --- Tests ---

// TestBuildEmbeddingTextCombinesFields verifies title/summary/content concatenation
// and the maxEmbeddingChars truncation guard.
func TestBuildEmbeddingTextCombinesFields(t *testing.T) {
	a := model.Article{Title: "T", Summary: "S", FullContent: "C"}
	got := buildEmbeddingText(a)
	if got != "T\n\nS\n\nC" {
		t.Errorf("buildEmbeddingText = %q, want %q", got, "T\n\nS\n\nC")
	}
	if buildEmbeddingText(model.Article{}) != "" {
		t.Error("empty article should produce empty text")
	}
	huge := strings.Repeat("x", maxEmbeddingChars+500)
	out := buildEmbeddingText(model.Article{FullContent: huge})
	if len(out) != maxEmbeddingChars {
		t.Errorf("truncation: got len %d, want %d", len(out), maxEmbeddingChars)
	}
}

// TestEmbedPendingArticlesNoAPIKey guarantees the service is a no-op when API key
// is missing, regardless of articles in the repository.
func TestEmbedPendingArticlesNoAPIKey(t *testing.T) {
	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		ListArticlesFunc: func() []model.Article {
			return []model.Article{{ID: 1, Title: "A", FullContent: "body"}}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{APIKey: ""})
	n, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("embedded count = %d, want 0 when API key absent", n)
	}
	if store, _, _ := vec.snapshotCounts(); store != 0 {
		t.Errorf("no key path must not store vectors, got store=%d", store)
	}
}

// TestEmbedPendingArticlesConfigError verifies config-loading errors are surfaced.
func TestEmbedPendingArticlesConfigError(t *testing.T) {
	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{}
	svc := NewEmbeddingService(vec, feed,
		func() *http.Client { return http.DefaultClient },
		func() (EmbeddingConfig, error) { return EmbeddingConfig{}, errors.New("cfg broken") },
	)
	_, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err == nil || !strings.Contains(err.Error(), "load embedding config") {
		t.Fatalf("expected config error, got %v", err)
	}
}

// TestEmbedPendingArticlesCallsAPIAndStores covers the happy path: one article without
// an existing embedding triggers one API call and one StoreEmbedding.
func TestEmbedPendingArticlesCallsAPIAndStores(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 8)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			return []model.Article{{ID: 42, Title: "Hello", FullContent: "World body text."}}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		Model:   "m",
	})
	n, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("embedded = %d, want 1", n)
	}
	if mockSrv.count() != 1 {
		t.Errorf("API call count = %d, want 1", mockSrv.count())
	}
	store, _, _ := vec.snapshotCounts()
	if store != 1 {
		t.Errorf("StoreEmbedding call count = %d, want 1", store)
	}
	if stored, _ := vec.GetEmbedding(context.Background(), "article", 42); len(stored) != 8 {
		t.Errorf("stored vec dim = %d, want 8", len(stored))
	}
}

// TestEmbedPendingArticlesSkipsExisting models a "cache hit": an article already has
// an embedding, so HasEmbedding reports true and no external call is made.
func TestEmbedPendingArticlesSkipsExisting(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	// Pre-populate the vector store to simulate a prior successful embedding.
	_ = vec.StoreEmbedding(context.Background(), "article", 7, "prev-model", []float32{0.1, 0.2, 0.3, 0.4})

	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			return []model.Article{{ID: 7, Title: "Already Embedded", FullContent: "body"}}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		Model:   "m",
	})
	n, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("embedded = %d, want 0 when already present", n)
	}
	if mockSrv.count() != 0 {
		t.Errorf("API should not be called; count = %d", mockSrv.count())
	}
}

// TestEmbedPendingArticlesSkipsEmptyText ensures articles with no text content
// (empty title/summary/body) are silently skipped instead of crashing the loop.
func TestEmbedPendingArticlesSkipsEmptyText(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			return []model.Article{{ID: 1}, {ID: 2, Title: "  ", Summary: "  "}}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		Model:   "m",
	})
	n, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("embedded = %d, want 0 for empty-text articles", n)
	}
	if mockSrv.count() != 0 {
		t.Errorf("API must not be called for empty text; got %d", mockSrv.count())
	}
}

// TestEmbedPendingArticlesAPIFailureDoesNotStore checks the failure-resilience
// contract: an API 5xx must not crash the loop, must not store a partial vector,
// and must leave the vector store clean for retry.
func TestEmbedPendingArticlesAPIFailureDoesNotStore(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()
	mockSrv.setFail(http.StatusInternalServerError, []byte(`{"error":{"message":"upstream down"}}`))

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			return []model.Article{{ID: 99, Title: "X", FullContent: "content"}}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		Model:   "m",
	})
	n, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err != nil {
		// Contract: batch path swallows per-article errors and logs them.
		t.Fatalf("EmbedPendingArticles should swallow per-article errors, got: %v", err)
	}
	if n != 0 {
		t.Errorf("embedded = %d, want 0 on API failure", n)
	}
	store, _, _ := vec.snapshotCounts()
	if store != 0 {
		t.Errorf("StoreEmbedding count = %d, want 0 on API failure", store)
	}
}

// TestEmbedPendingArticlesFallsBackToListArticles verifies the secondary list path:
// when ListArticlesNeedingScoreRefresh returns nothing, the service falls back to
// scanning ListArticles.
func TestEmbedPendingArticlesFallsBackToListArticles(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article { return nil },
		ListArticlesFunc: func() []model.Article {
			return []model.Article{{ID: 5, Title: "Fallback", FullContent: "body"}}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		Model:   "m",
	})
	n, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Errorf("embedded = %d, want 1 via fallback list", n)
	}
}

// TestEmbedPendingArticlesRespectsBatchSize ensures the batch cap is honored.
func TestEmbedPendingArticlesRespectsBatchSize(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			return []model.Article{
				{ID: 1, Title: "A", FullContent: "a body"},
				{ID: 2, Title: "B", FullContent: "b body"},
				{ID: 3, Title: "C", FullContent: "c body"},
			}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		Model:   "m",
	})
	n, err := svc.EmbedPendingArticles(context.Background(), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Errorf("embedded = %d, want 2 (batch cap)", n)
	}
	if mockSrv.count() != 2 {
		t.Errorf("API call count = %d, want 2", mockSrv.count())
	}
}

// TestEmbedSingleArticleHappyPath covers the single-article path end to end.
func TestEmbedSingleArticleHappyPath(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 6)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		GetArticleFunc: func(id int64) (model.Article, bool) {
			return model.Article{ID: id, Title: "T", FullContent: "body"}, true
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		// Model left empty on purpose to hit the default-model branch.
	})
	if err := svc.EmbedSingleArticle(context.Background(), 11); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockSrv.count() != 1 {
		t.Errorf("API count = %d, want 1", mockSrv.count())
	}
	if stored, _ := vec.GetEmbedding(context.Background(), "article", 11); len(stored) != 6 {
		t.Errorf("stored dim = %d, want 6", len(stored))
	}
	// Default model must be applied when config left empty.
	if got := vec.storedModel[vecKey("article", 11)]; got != defaultEmbeddingModel {
		t.Errorf("stored model = %q, want %q", got, defaultEmbeddingModel)
	}
}

// TestEmbedSingleArticleNotFound verifies explicit error when the feed repo reports
// missing.
func TestEmbedSingleArticleNotFound(t *testing.T) {
	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		GetArticleFunc: func(_ int64) (model.Article, bool) { return model.Article{}, false },
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{APIKey: "k", BaseURL: "http://unused"})
	err := svc.EmbedSingleArticle(context.Background(), 404)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

// TestEmbedSingleArticleEmptyTextNoCall confirms no external call is issued when
// the article has no text to embed.
func TestEmbedSingleArticleEmptyTextNoCall(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		GetArticleFunc: func(_ int64) (model.Article, bool) { return model.Article{ID: 1}, true },
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
	})
	if err := svc.EmbedSingleArticle(context.Background(), 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mockSrv.count() != 0 {
		t.Errorf("API must not be called for empty-text article")
	}
	if store, _, _ := vec.snapshotCounts(); store != 0 {
		t.Errorf("StoreEmbedding count = %d, want 0", store)
	}
}

// TestEmbedSingleArticleAPIErrorWraps verifies an upstream 4xx is wrapped and returned.
func TestEmbedSingleArticleAPIErrorWraps(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()
	mockSrv.setFail(http.StatusBadRequest, []byte(`{"error":{"message":"bad"}}`))

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		GetArticleFunc: func(id int64) (model.Article, bool) {
			return model.Article{ID: id, Title: "T", FullContent: "body"}, true
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{APIKey: "k", BaseURL: mockSrv.srv.URL, Model: "m"})
	err := svc.EmbedSingleArticle(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error from upstream 400")
	}
	if !strings.Contains(err.Error(), "call embedding API") {
		t.Errorf("error = %v, want to contain 'call embedding API'", err)
	}
	if store, _, _ := vec.snapshotCounts(); store != 0 {
		t.Errorf("StoreEmbedding must not be called on API failure, got %d", store)
	}
}

// TestEmbeddingDimensionConsistency asserts repeated calls against the same mock
// endpoint return vectors of the same dimension — guards against provider drift
// between requests.
func TestEmbeddingDimensionConsistency(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 12)
	defer mockSrv.close()

	svc := makeEmbeddingService(newStubVectorRepo(), &mock.MockFeedRepository{},
		EmbeddingConfig{APIKey: "k", BaseURL: mockSrv.srv.URL, Model: "m"})
	cfg := EmbeddingConfig{APIKey: "k", BaseURL: mockSrv.srv.URL, Model: "m"}

	v1, err := svc.callEmbeddingAPI(context.Background(), cfg, "first text")
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	v2, err := svc.callEmbeddingAPI(context.Background(), cfg, "second longer text")
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if len(v1) != len(v2) {
		t.Errorf("dimension drift: %d vs %d", len(v1), len(v2))
	}
	if len(v1) != 12 {
		t.Errorf("v1 dim = %d, want 12", len(v1))
	}
}

// TestSearchSimilarArticlesNoKey verifies the nil result when the API key is absent.
func TestSearchSimilarArticlesNoKey(t *testing.T) {
	vec := newStubVectorRepo()
	svc := makeEmbeddingService(vec, &mock.MockFeedRepository{}, EmbeddingConfig{})
	got, err := svc.SearchSimilarArticles(context.Background(), "query", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil results with no API key, got %+v", got)
	}
	if _, _, search := vec.snapshotCounts(); search != 0 {
		t.Errorf("vector search should not run without API key, got count=%d", search)
	}
}

// TestSearchSimilarArticlesCallsAPIAndRepo wires the happy path.
func TestSearchSimilarArticlesCallsAPIAndRepo(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	vec.searchResults = []repository.VectorMatch{{SourceType: "article", SourceID: 99, Score: 0.92}}
	svc := makeEmbeddingService(vec, &mock.MockFeedRepository{}, EmbeddingConfig{
		APIKey: "k", BaseURL: mockSrv.srv.URL, Model: "m",
	})
	got, err := svc.SearchSimilarArticles(context.Background(), "query text", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].SourceID != 99 {
		t.Errorf("unexpected search result: %+v", got)
	}
	if mockSrv.count() != 1 {
		t.Errorf("API count = %d, want 1", mockSrv.count())
	}
	if _, _, search := vec.snapshotCounts(); search != 1 {
		t.Errorf("repo search count = %d, want 1", search)
	}
}

// TestSearchSimilarArticlesAPIError verifies error propagation wrapping.
func TestSearchSimilarArticlesAPIError(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()
	mockSrv.setFail(http.StatusTooManyRequests, []byte(`{"error":{"message":"rate limited"}}`))

	svc := makeEmbeddingService(newStubVectorRepo(), &mock.MockFeedRepository{}, EmbeddingConfig{
		APIKey: "k", BaseURL: mockSrv.srv.URL, Model: "m",
	})
	_, err := svc.SearchSimilarArticles(context.Background(), "q", 1)
	if err == nil {
		t.Fatal("expected error from 429")
	}
	if !strings.Contains(err.Error(), "embed query") {
		t.Errorf("error = %v, want wrap 'embed query'", err)
	}
}

// TestCallEmbeddingAPIBaseURLTrimming confirms trailing slashes in BaseURL are
// stripped before building the request.
func TestCallEmbeddingAPIBaseURLTrimming(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 2)
	defer mockSrv.close()

	// Append stray slashes to the base URL.
	cfg := EmbeddingConfig{APIKey: "k", BaseURL: mockSrv.srv.URL + "//", Model: "m"}
	svc := makeEmbeddingService(newStubVectorRepo(), &mock.MockFeedRepository{}, cfg)

	if _, err := svc.callEmbeddingAPI(context.Background(), cfg, "text"); err != nil {
		t.Fatalf("call failed: %v", err)
	}
	if mockSrv.count() != 1 {
		t.Errorf("API count = %d, want 1", mockSrv.count())
	}
}

// TestCallEmbeddingAPIEmptyData covers the "empty data" server response branch.
func TestCallEmbeddingAPIEmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	cfg := EmbeddingConfig{APIKey: "k", BaseURL: srv.URL, Model: "m"}
	svc := makeEmbeddingService(newStubVectorRepo(), &mock.MockFeedRepository{}, cfg)
	_, err := svc.callEmbeddingAPI(context.Background(), cfg, "text")
	if err == nil || !strings.Contains(err.Error(), "empty data") {
		t.Fatalf("expected empty-data error, got %v", err)
	}
}

// TestCallEmbeddingAPIMalformedJSON exercises the parse-response branch.
func TestCallEmbeddingAPIMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	cfg := EmbeddingConfig{APIKey: "k", BaseURL: srv.URL, Model: "m"}
	svc := makeEmbeddingService(newStubVectorRepo(), &mock.MockFeedRepository{}, cfg)
	_, err := svc.callEmbeddingAPI(context.Background(), cfg, "text")
	if err == nil || !strings.Contains(err.Error(), "parse response") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

// TestCallEmbeddingAPIErrorField exercises the server-level "error" object branch.
func TestCallEmbeddingAPIErrorField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	defer srv.Close()

	cfg := EmbeddingConfig{APIKey: "k", BaseURL: srv.URL, Model: "m"}
	svc := makeEmbeddingService(newStubVectorRepo(), &mock.MockFeedRepository{}, cfg)
	_, err := svc.callEmbeddingAPI(context.Background(), cfg, "text")
	if err == nil || !strings.Contains(err.Error(), "embedding API error") {
		t.Fatalf("expected embedding API error, got %v", err)
	}
}

// TestEmbedPendingArticlesDefaultBatchSize exercises the "batchSize <= 0 ->
// default" branch of the pending loop.
func TestEmbedPendingArticlesDefaultBatchSize(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			return []model.Article{{ID: 1, Title: "T", FullContent: "body"}}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		Model:   "m",
	})
	// Passing 0 should fall back to maxEmbeddingBatch without error.
	n, err := svc.EmbedPendingArticles(context.Background(), 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if n != 1 {
		t.Errorf("embedded = %d, want 1", n)
	}
}

// TestEmbedPendingArticlesDefaultModelWhenConfigBlank verifies that the default
// model name is used when cfg.Model is empty, and forwarded to StoreEmbedding.
func TestEmbedPendingArticlesDefaultModelWhenConfigBlank(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			return []model.Article{{ID: 8, Title: "T", FullContent: "body"}}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		// Model intentionally empty -> should use defaultEmbeddingModel.
	})
	n, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if n != 1 {
		t.Errorf("embedded = %d, want 1", n)
	}
	if got := vec.storedModel[vecKey("article", 8)]; got != defaultEmbeddingModel {
		t.Errorf("stored model = %q, want %q", got, defaultEmbeddingModel)
	}
}

// TestEmbedPendingArticlesStoreErrorContinuesBatch ensures a single StoreEmbedding
// failure doesn't abort the batch; other articles still get processed.
func TestEmbedPendingArticlesStoreErrorContinuesBatch(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	vec.storeErr = errors.New("disk full")

	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			return []model.Article{
				{ID: 1, Title: "A", FullContent: "body"},
				{ID: 2, Title: "B", FullContent: "body"},
			}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		Model:   "m",
	})
	n, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if n != 0 {
		t.Errorf("embedded = %d, want 0 when store fails", n)
	}
	// Each article should have triggered one API call and one store attempt.
	if mockSrv.count() != 2 {
		t.Errorf("API count = %d, want 2", mockSrv.count())
	}
}

// TestEmbedSingleArticleConfigError surfaces config-load failures from the single
// article path (covers the wrapper at line 125-127).
func TestEmbedSingleArticleConfigError(t *testing.T) {
	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{}
	svc := NewEmbeddingService(vec, feed,
		func() *http.Client { return http.DefaultClient },
		func() (EmbeddingConfig, error) { return EmbeddingConfig{}, errors.New("cfg broken") },
	)
	err := svc.EmbedSingleArticle(context.Background(), 1)
	if err == nil || !strings.Contains(err.Error(), "load embedding config") {
		t.Fatalf("expected config error, got %v", err)
	}
}

// TestEmbedSingleArticleNoAPIKey ensures the single-article path is a no-op (no
// store, no error) when the API key is absent.
func TestEmbedSingleArticleNoAPIKey(t *testing.T) {
	vec := newStubVectorRepo()
	feed := &mock.MockFeedRepository{
		GetArticleFunc: func(_ int64) (model.Article, bool) { return model.Article{ID: 1, Title: "t"}, true },
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{APIKey: ""})
	if err := svc.EmbedSingleArticle(context.Background(), 1); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if store, _, _ := vec.snapshotCounts(); store != 0 {
		t.Errorf("store count = %d, want 0", store)
	}
}

// TestSearchSimilarArticlesConfigError surfaces config-load failures for search.
func TestSearchSimilarArticlesConfigError(t *testing.T) {
	svc := NewEmbeddingService(newStubVectorRepo(), &mock.MockFeedRepository{},
		func() *http.Client { return http.DefaultClient },
		func() (EmbeddingConfig, error) { return EmbeddingConfig{}, errors.New("cfg broken") },
	)
	_, err := svc.SearchSimilarArticles(context.Background(), "q", 1)
	if err == nil || !strings.Contains(err.Error(), "load embedding config") {
		t.Fatalf("expected config error, got %v", err)
	}
}

// TestVectorRepoAccessor verifies the accessor returns the injected repo.
func TestVectorRepoAccessor(t *testing.T) {
	vec := newStubVectorRepo()
	svc := makeEmbeddingService(vec, &mock.MockFeedRepository{}, EmbeddingConfig{})
	if svc.VectorRepo() != vec {
		t.Error("VectorRepo() must return the injected repo instance")
	}
}

// TestEmbedPendingArticlesHasEmbeddingError ensures HasEmbedding failures are logged
// and the article is skipped without aborting the whole batch.
func TestEmbedPendingArticlesHasEmbeddingError(t *testing.T) {
	mockSrv := newMockEmbeddingServer(t, 4)
	defer mockSrv.close()

	vec := newStubVectorRepo()
	vec.hasErr = errors.New("index unreachable")

	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			return []model.Article{{ID: 1, Title: "T", FullContent: "body"}}
		},
	}
	svc := makeEmbeddingService(vec, feed, EmbeddingConfig{
		APIKey:  "k",
		BaseURL: mockSrv.srv.URL,
		Model:   "m",
	})
	n, err := svc.EmbedPendingArticles(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("embedded = %d, want 0 when HasEmbedding errors", n)
	}
	if mockSrv.count() != 0 {
		t.Errorf("API must not fire when HasEmbedding failed")
	}
}
