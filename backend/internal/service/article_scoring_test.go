package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/internal/repository/mock"
)

// nowRFC3339 is a stable "recent" timestamp helper to keep freshness deterministic
// (freshness depends on wall clock, so tests use "now" and only check it's high).
func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// TestRecomputeArticleFeaturesInputGating covers the three ArticleGateStatus branches
// produced by scoreBaseFeatures: invalid (empty/tiny), degraded (short/noisy), valid.
func TestRecomputeArticleFeaturesInputGating(t *testing.T) {
	tests := []struct {
		name       string
		article    model.Article
		wantGate   model.ArticleGateStatus
		maxCompose int // composite is capped by gate status
	}{
		{
			name:       "fully empty article is invalid",
			article:    model.Article{},
			wantGate:   model.ArticleGateInvalid,
			maxCompose: 30,
		},
		{
			name: "extremely short content is invalid",
			article: model.Article{
				Title:   "x",
				Summary: "short", // 5 chars, far below 40 char threshold
			},
			wantGate:   model.ArticleGateInvalid,
			maxCompose: 30,
		},
		{
			name: "moderate length triggers degraded gate",
			// ~93 visible chars with 18 unique tokens — above invalid threshold
			// (40 chars / 8 tokens) but below valid threshold (120 chars / 20 tokens).
			article: model.Article{
				Title:       "Notes on cache",
				Summary:     "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi rho sigma",
				PublishedAt: nowRFC3339(),
			},
			wantGate:   model.ArticleGateDegraded,
			maxCompose: 70,
		},
		{
			name: "rich article is valid",
			// Needs >=120 visible chars, >=20 unique tokens, and noise ratio < 0.12.
			// Use a vocabulary-rich body so uniqueTokens() produces enough distinct
			// tokens (repeated phrases collapse to a single unique token).
			article: model.Article{
				Title: "Detailed Engineering Analysis of Distributed Consensus",
				Summary: "A careful summary describing Raft Paxos leader election commit " +
					"replication log truncation quorum safety liveness invariants membership changes.",
				FullContent: "Distributed consensus protocols such as Raft and Paxos rely on " +
					"majority quorums to make progress. Nodes coordinate through elections, " +
					"replicate log entries, persist state to stable storage, and recover after " +
					"crashes using snapshots. Membership changes require joint configuration " +
					"handoffs. Clients observe linearizable reads when leases overlap safely.",
				Link:        "https://example.com/a",
				CoverURL:    "https://example.com/c.jpg",
				PublishedAt: nowRFC3339(),
			},
			wantGate:   model.ArticleGateValid,
			maxCompose: 100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			features := RecomputeArticleFeatures(tc.article)
			if features.GateStatus != tc.wantGate {
				t.Fatalf("GateStatus = %q, want %q", features.GateStatus, tc.wantGate)
			}
			// All five dims must be in [0, 100]
			checkScoreRange(t, "Quality", features.Quality)
			checkScoreRange(t, "Relevance", features.Relevance)
			checkScoreRange(t, "Depth", features.Depth)
			checkScoreRange(t, "Freshness", features.Freshness)
			checkScoreRange(t, "Novelty", features.Novelty)
			checkScoreRange(t, "Composite", features.Composite)
			if features.Composite > tc.maxCompose {
				t.Errorf("Composite = %d, gate %q caps at %d", features.Composite, features.GateStatus, tc.maxCompose)
			}
			if features.FeatureVersion != articleFeatureVersion {
				t.Errorf("FeatureVersion = %d, want %d", features.FeatureVersion, articleFeatureVersion)
			}
			if features.ScoredAt == "" {
				t.Error("ScoredAt must be populated after scoring")
			}
			// Rule-only path must produce no reasoning text.
			if features.Reasoning != "" {
				t.Errorf("Reasoning = %q on rule-only path, want empty", features.Reasoning)
			}
		})
	}
}

func checkScoreRange(t *testing.T, name string, v int) {
	t.Helper()
	if v < 0 || v > 100 {
		t.Errorf("%s = %d, want within [0, 100]", name, v)
	}
}

// TestRecomputeArticleFeaturesSummaryFallback verifies that when FullContent is empty
// the scorer still builds rule features from Summary. This is the readability-missing
// fallback path used by freshly ingested RSS items.
func TestRecomputeArticleFeaturesSummaryFallback(t *testing.T) {
	// Vocabulary-rich summary so uniqueTokens produces enough distinct tokens
	// to clear the invalid gate even without FullContent.
	summary := "Thoughtful analysis of consensus protocol tradeoffs covering Raft, Paxos, " +
		"leader election, log replication, quorum safety, liveness, lease invariants, " +
		"membership reconfiguration, snapshot compaction, crash recovery and linearizable reads."
	article := model.Article{
		Title:       "Consensus Protocol Deep Dive",
		Summary:     summary,
		FullContent: "", // missing
		Link:        "https://example.com/x",
		PublishedAt: nowRFC3339(),
	}
	features := RecomputeArticleFeatures(article)
	if features.GateStatus == model.ArticleGateInvalid {
		t.Fatalf("expected fallback to summary to keep article non-invalid, got %q", features.GateStatus)
	}
	if features.Quality <= 0 || features.Depth <= 0 || features.Relevance <= 0 {
		t.Errorf("scores should be positive on summary-only fallback, got q=%d d=%d r=%d",
			features.Quality, features.Depth, features.Relevance)
	}
}

// TestRecomputeArticleFeaturesDeterministic guards against hidden randomness: the
// same input must produce the same score output across calls (ScoredAt excluded).
func TestRecomputeArticleFeaturesDeterministic(t *testing.T) {
	article := model.Article{
		Title:   "Stable Input",
		Summary: "Deterministic summary describing the article body with varied vocabulary.",
		// Varied body so tokens don't collapse after uniqueTokens dedup.
		FullContent: "Determinism matters for recomputing features without hidden randomness. " +
			"Distributed systems rely on reproducible decisions, consistent hashing, stable " +
			"leader leases, and monotonic counters. Engineering tradeoffs include latency, " +
			"throughput, availability, and consistency across replicas and regions.",
		Link:        "https://example.com/stable",
		PublishedAt: nowRFC3339(),
	}
	first := RecomputeArticleFeatures(article)
	second := RecomputeArticleFeatures(article)

	// Zero out ScoredAt for comparison — it's wall-clock derived.
	first.ScoredAt = ""
	second.ScoredAt = ""
	if first != second {
		t.Errorf("rule scoring not deterministic:\nfirst  = %+v\nsecond = %+v", first, second)
	}
	if first.ContentFingerprint == "" {
		t.Error("ContentFingerprint must be set")
	}
	if first.ContentFingerprint != second.ContentFingerprint {
		t.Error("ContentFingerprint must be stable for same input")
	}
}

// TestScoreArticlesNoveltyPenalizesNearDuplicates builds a two-article batch where
// one article is a near-duplicate of the other; its Novelty must be lower than that
// of an isolated unique article.
func TestScoreArticlesNoveltyPenalizesNearDuplicates(t *testing.T) {
	body := strings.Repeat("Detailed engineering analysis of the consensus paper. ", 20)
	// Two near-identical articles should both receive a novelty penalty.
	batch := []model.Article{
		{
			Title:       "Consensus Paper Review",
			Summary:     "Review of the consensus paper.",
			FullContent: body,
			PublishedAt: nowRFC3339(),
		},
		{
			Title:       "Consensus Paper Review",
			Summary:     "Review of the consensus paper.",
			FullContent: body,
			PublishedAt: nowRFC3339(),
		},
	}
	duplicateFeatures := scoreArticles(batch)

	// Same input, isolated — acts as the novelty baseline.
	soloFeatures := scoreArticles([]model.Article{batch[0]})

	if len(duplicateFeatures) != 2 || len(soloFeatures) != 1 {
		t.Fatalf("unexpected feature slice lengths dup=%d solo=%d", len(duplicateFeatures), len(soloFeatures))
	}
	if duplicateFeatures[0].Novelty >= soloFeatures[0].Novelty {
		t.Errorf("near-duplicate novelty %d should be strictly lower than solo %d",
			duplicateFeatures[0].Novelty, soloFeatures[0].Novelty)
	}
	// The two duplicates should be penalized symmetrically.
	if duplicateFeatures[0].Novelty != duplicateFeatures[1].Novelty {
		t.Errorf("symmetric duplicates should share novelty, got %d vs %d",
			duplicateFeatures[0].Novelty, duplicateFeatures[1].Novelty)
	}
}

// TestRecommendationScoresForArticleLegacyFallback verifies the three-tier fallback
// in RecommendationScoresForArticle:
//  1. ArticleFeatures present and current-version -> use it.
//  2. No features but persisted RecommendationScores -> reuse persisted scores.
//  3. Neither -> compute dynamically from raw article fields.
func TestRecommendationScoresForArticleLegacyFallback(t *testing.T) {
	base := model.Article{
		Title:       "Legacy Article",
		Summary:     "A summary.",
		FullContent: strings.Repeat("Legacy body content for scoring. ", 20),
		Link:        "https://example.com/legacy",
		PublishedAt: nowRFC3339(),
	}

	t.Run("current features win", func(t *testing.T) {
		article := base
		article.ArticleFeatures = &model.ArticleFeatures{
			Quality:        77,
			Relevance:      66,
			Novelty:        55,
			Composite:      71,
			FeatureVersion: articleFeatureVersion,
		}
		// Persisted scores set to a different value — must be ignored when
		// current-version features are present.
		article.RecommendationScores = &model.RecommendationScores{Quality: 10, Relevance: 10, Novelty: 10, Composite: 10}
		scores := RecommendationScoresForArticle(article)
		if scores.Quality != 77 || scores.Composite != 71 {
			t.Errorf("features path: scores = %+v, want quality=77 composite=71", scores)
		}
	})

	t.Run("persisted scores reused when no features", func(t *testing.T) {
		article := base
		article.RecommendationScores = &model.RecommendationScores{Quality: 81, Relevance: 72, Novelty: 63, Composite: 75}
		scores := RecommendationScoresForArticle(article)
		if scores.Quality != 81 || scores.Composite != 75 {
			t.Errorf("persisted path: scores = %+v, want quality=81 composite=75", scores)
		}
	})

	t.Run("dynamic recompute when nothing persisted", func(t *testing.T) {
		article := base
		scores := RecommendationScoresForArticle(article)
		// Must produce meaningful positive scores for a real article body.
		if scores.Quality <= 0 || scores.Composite <= 0 {
			t.Errorf("dynamic path: expected positive scores, got %+v", scores)
		}
		checkScoreRange(t, "dynamic.Quality", scores.Quality)
		checkScoreRange(t, "dynamic.Composite", scores.Composite)
	})

	t.Run("stale feature version forces recompute", func(t *testing.T) {
		article := base
		article.ArticleFeatures = &model.ArticleFeatures{
			Quality:        99,
			Composite:      99,
			FeatureVersion: 0, // stale
		}
		scores := RecommendationScoresForArticle(article)
		// Must not blindly echo the stale 99 — recompute kicks in.
		if scores.Quality == 99 && scores.Composite == 99 {
			t.Errorf("stale features should be discarded, got %+v", scores)
		}
	})
}

// TestRecommendationScoresForSeedLegacyFallback mirrors the article path for the
// seed variant used during feed ingestion.
func TestRecommendationScoresForSeedLegacyFallback(t *testing.T) {
	seed := repository.ArticleSeed{
		Title:       "Seed Title",
		Link:        "https://example.com/seed",
		Summary:     "Seed summary content.",
		FullContent: strings.Repeat("Seed body paragraph. ", 20),
		PublishedAt: nowRFC3339(),
	}

	t.Run("current features win", func(t *testing.T) {
		s := seed
		s.ArticleFeatures = &model.ArticleFeatures{
			Quality:        65,
			Relevance:      55,
			Novelty:        45,
			Composite:      58,
			FeatureVersion: articleFeatureVersion,
		}
		scores := RecommendationScoresForSeed(s)
		if scores.Quality != 65 || scores.Composite != 58 {
			t.Errorf("features path: scores = %+v, want quality=65 composite=58", scores)
		}
	})

	t.Run("dynamic recompute when no features", func(t *testing.T) {
		scores := RecommendationScoresForSeed(seed)
		if scores.Quality <= 0 {
			t.Errorf("dynamic path should produce positive quality, got %+v", scores)
		}
	})
}

// TestAttachRecommendationScoresToSeedsEmpty guards the empty-input fast path and
// the happy-path attach behavior.
func TestAttachRecommendationScoresToSeeds(t *testing.T) {
	t.Run("empty input returns same slice", func(t *testing.T) {
		out := AttachRecommendationScoresToSeeds(nil)
		if len(out) != 0 {
			t.Errorf("expected empty output, got %d items", len(out))
		}
	})

	t.Run("each seed gets features and scores", func(t *testing.T) {
		seeds := []repository.ArticleSeed{
			{
				Title:       "A",
				Link:        "https://example.com/a",
				Summary:     "Article A summary.",
				FullContent: strings.Repeat("Body A paragraph. ", 20),
				PublishedAt: nowRFC3339(),
			},
			{
				Title:       "B",
				Link:        "https://example.com/b",
				Summary:     "Article B summary.",
				FullContent: strings.Repeat("Body B paragraph. ", 20),
				PublishedAt: nowRFC3339(),
			},
		}
		out := AttachRecommendationScoresToSeeds(seeds)
		if len(out) != 2 {
			t.Fatalf("len(out) = %d, want 2", len(out))
		}
		for i, s := range out {
			if s.ArticleFeatures == nil {
				t.Errorf("seed %d: ArticleFeatures not attached", i)
				continue
			}
			if s.ArticleFeatures.FeatureVersion != articleFeatureVersion {
				t.Errorf("seed %d: FeatureVersion = %d, want %d", i, s.ArticleFeatures.FeatureVersion, articleFeatureVersion)
			}
			if s.RecommendationScores == nil {
				t.Errorf("seed %d: RecommendationScores not attached", i)
				continue
			}
			checkScoreRange(t, "seed.Composite", s.RecommendationScores.Composite)
		}
	})
}

// TestArticleFeaturesForArticlePassesThroughCurrent verifies that ArticleFeaturesForArticle
// returns the stored features as-is when they carry the current feature version.
func TestArticleFeaturesForArticleCurrentPassthrough(t *testing.T) {
	stored := model.ArticleFeatures{
		Quality:        50,
		Relevance:      45,
		Depth:          40,
		Freshness:      60,
		Novelty:        55,
		Composite:      50,
		Reasoning:      "persisted",
		FeatureVersion: articleFeatureVersion,
	}
	article := model.Article{
		Title:           "X",
		ArticleFeatures: &stored,
	}
	got := ArticleFeaturesForArticle(article)
	if got != stored {
		t.Errorf("expected passthrough of stored features, got %+v want %+v", got, stored)
	}
}

// TestArticleFeaturesForArticleRecomputesOnStale confirms that when the stored features
// carry a stale FeatureVersion, the helper recomputes them.
func TestArticleFeaturesForArticleStaleRecomputes(t *testing.T) {
	stored := model.ArticleFeatures{
		Quality:        99,
		Composite:      99,
		FeatureVersion: 0, // stale
	}
	article := model.Article{
		Title:           "X",
		Summary:         "A reasonable summary for recompute.",
		FullContent:     strings.Repeat("Detailed body paragraph. ", 20),
		Link:            "https://example.com/x",
		PublishedAt:     nowRFC3339(),
		ArticleFeatures: &stored,
	}
	got := ArticleFeaturesForArticle(article)
	if got.FeatureVersion != articleFeatureVersion {
		t.Errorf("FeatureVersion = %d, want %d", got.FeatureVersion, articleFeatureVersion)
	}
	if got.Quality == 99 && got.Composite == 99 {
		t.Errorf("stale features must be recomputed, got %+v", got)
	}
}

// TestFreshnessScoreBuckets exercises the four freshness age buckets.
func TestFreshnessScoreBuckets(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name string
		age  time.Duration
		want int
	}{
		{"within 24h", 2 * time.Hour, 88},
		{"within 72h", 48 * time.Hour, 74},
		{"within 7d", 5 * 24 * time.Hour, 60},
		{"within 30d", 20 * 24 * time.Hour, 42},
		{"older than 30d", 120 * 24 * time.Hour, 24},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			article := model.Article{PublishedAt: now.Add(-tc.age).Format(time.RFC3339)}
			got := freshnessScore(article)
			if got != tc.want {
				t.Errorf("freshnessScore(age=%v) = %d, want %d", tc.age, got, tc.want)
			}
		})
	}
}

// TestScoreArticlesEmptyInput covers the early return in scoreArticles.
func TestScoreArticlesEmptyInput(t *testing.T) {
	if out := scoreArticles(nil); out != nil {
		t.Errorf("scoreArticles(nil) = %v, want nil", out)
	}
	if out := scoreArticles([]model.Article{}); out != nil {
		t.Errorf("scoreArticles([]) = %v, want nil", out)
	}
}

// TestHasMeaningfulScores covers the zero-score guard used by the legacy fallback path.
func TestHasMeaningfulScores(t *testing.T) {
	if hasMeaningfulScores(model.RecommendationScores{}) {
		t.Error("zero-value scores must not be considered meaningful")
	}
	if !hasMeaningfulScores(model.RecommendationScores{Quality: 1}) {
		t.Error("any positive dimension should be meaningful")
	}
	if !hasMeaningfulScores(model.RecommendationScores{Composite: 5}) {
		t.Error("composite alone should be enough to be meaningful")
	}
}

// TestRefreshStaleScoresContextCancelled verifies that a pre-cancelled context is
// surfaced without touching the repo. This is the rule-path ctx-gate contract.
func TestRefreshStaleScoresContextCancelled(t *testing.T) {
	var listCalls int32
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article {
			atomic.AddInt32(&listCalls, 1)
			return nil
		},
	}
	svc := NewArticleService(feed, func() *http.Client { return http.DefaultClient })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	n, err := svc.RefreshStaleScores(ctx, 10)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if n != 0 {
		t.Errorf("refreshed = %d, want 0", n)
	}
	if atomic.LoadInt32(&listCalls) != 0 {
		t.Errorf("cancelled ctx must not call repo, got %d call(s)", listCalls)
	}
}

// TestRefreshStaleScoresNoArticles covers the empty-list short-circuit.
func TestRefreshStaleScoresNoArticles(t *testing.T) {
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article { return nil },
	}
	svc := NewArticleService(feed, func() *http.Client { return http.DefaultClient })
	n, err := svc.RefreshStaleScores(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if n != 0 {
		t.Errorf("refreshed = %d, want 0", n)
	}
}

// TestRefreshStaleScoresRulePathPersistsFeaturesAndScores exercises the full rule
// path with an in-memory mock: no LLM wired, so features and scores must both be
// persisted via the repository update hooks.
func TestRefreshStaleScoresRulePathPersistsFeaturesAndScores(t *testing.T) {
	var featUpdates, scoreUpdates int32
	articles := []model.Article{
		{
			ID:    42,
			Title: "Detailed Engineering Analysis of Distributed Consensus",
			Summary: "Summary describing Raft, Paxos, leader election, log replication, quorum safety, " +
				"liveness, lease invariants, membership reconfiguration, snapshot compaction.",
			FullContent: "Distributed consensus protocols rely on majority quorums to make progress. " +
				"Nodes coordinate through elections, replicate log entries, persist state to stable " +
				"storage, and recover after crashes using snapshots. Membership changes require " +
				"joint configuration handoffs while clients observe linearizable reads via leases.",
			PublishedAt: nowRFC3339(),
		},
	}
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article { return articles },
		UpdateArticleFeaturesFunc: func(id int64, f model.ArticleFeatures) error {
			atomic.AddInt32(&featUpdates, 1)
			if id != 42 {
				t.Errorf("UpdateArticleFeatures id = %d, want 42", id)
			}
			if f.FeatureVersion != articleFeatureVersion {
				t.Errorf("feature version = %d, want %d", f.FeatureVersion, articleFeatureVersion)
			}
			return nil
		},
		UpdateArticleScoresFunc: func(id int64, s model.RecommendationScores) error {
			atomic.AddInt32(&scoreUpdates, 1)
			if s.Composite <= 0 {
				t.Errorf("composite = %d, want positive", s.Composite)
			}
			return nil
		},
	}
	svc := NewArticleService(feed, func() *http.Client { return http.DefaultClient })

	n, err := svc.RefreshStaleScores(context.Background(), 5)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if n != 1 {
		t.Errorf("refreshed = %d, want 1", n)
	}
	if got := atomic.LoadInt32(&featUpdates); got != 1 {
		t.Errorf("UpdateArticleFeatures calls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&scoreUpdates); got != 1 {
		t.Errorf("UpdateArticleScores calls = %d, want 1", got)
	}
}

// TestRefreshStaleScoresUpdateFeaturesError bubbles repository write failures.
func TestRefreshStaleScoresUpdateFeaturesError(t *testing.T) {
	articles := []model.Article{
		{ID: 1, Title: "T", Summary: "Some content with enough words.",
			FullContent:                                       "Body " + strings.Repeat("word ", 60),
			PublishedAt: nowRFC3339()},
	}
	boom := errors.New("write failed")
	feed := &mock.MockFeedRepository{
		ListArticlesNeedingScoreRefreshFunc: func(_, _ int) []model.Article { return articles },
		UpdateArticleFeaturesFunc:           func(_ int64, _ model.ArticleFeatures) error { return boom },
	}
	svc := NewArticleService(feed, func() *http.Client { return http.DefaultClient })
	_, err := svc.RefreshStaleScores(context.Background(), 5)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}

// TestNormalizeArticleSortMode covers every accepted sort-mode literal plus
// whitespace tolerance and the invalid rejection path — these flow directly into
// how stored scoring features are consumed downstream.
func TestNormalizeArticleSortMode(t *testing.T) {
	tests := []struct {
		in      string
		want    ArticleSortMode
		wantErr bool
	}{
		{"", ArticleSortLatest, false},
		{"latest", ArticleSortLatest, false},
		{"oldest", ArticleSortOldest, false},
		{"recommend", ArticleSortRecommend, false},
		{"quality", ArticleSortQuality, false},
		{"relevance", ArticleSortRelevance, false},
		{"novelty", ArticleSortNovelty, false},
		{"  latest  ", ArticleSortLatest, false},
		{"bogus", "", true},
	}
	for _, tc := range tests {
		got, err := NormalizeArticleSortMode(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizeArticleSortMode(%q) err = nil, want error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeArticleSortMode(%q) err = %v, want nil", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("NormalizeArticleSortMode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestScoreForSortReadsFromFeaturesOrScores checks the per-dimension extraction for
// downstream sorters. It must prefer current-version ArticleFeatures over persisted
// RecommendationScores, and fall back when features are stale.
func TestScoreForSortReadsFromFeaturesOrScores(t *testing.T) {
	curFeat := &model.ArticleFeatures{
		Quality: 88, Relevance: 77, Novelty: 66, Composite: 71,
		FeatureVersion: articleFeatureVersion,
	}
	stalePersisted := &model.RecommendationScores{Quality: 22, Relevance: 21, Novelty: 20, Composite: 19}

	withFeat := model.Article{ArticleFeatures: curFeat, RecommendationScores: stalePersisted}
	if got := scoreForSort(withFeat, ArticleSortQuality); got != 88 {
		t.Errorf("Quality via features = %d, want 88", got)
	}
	if got := scoreForSort(withFeat, ArticleSortRelevance); got != 77 {
		t.Errorf("Relevance via features = %d, want 77", got)
	}
	if got := scoreForSort(withFeat, ArticleSortNovelty); got != 66 {
		t.Errorf("Novelty via features = %d, want 66", got)
	}
	if got := scoreForSort(withFeat, ArticleSortRecommend); got != 71 {
		t.Errorf("Composite via features = %d, want 71", got)
	}
	// Non-score sort modes return 0.
	if got := scoreForSort(withFeat, ArticleSortLatest); got != 0 {
		t.Errorf("latest sort should yield 0, got %d", got)
	}

	// Stale features -> persisted scores win.
	staleFeat := &model.ArticleFeatures{Quality: 9, FeatureVersion: 0}
	onlyPersisted := model.Article{ArticleFeatures: staleFeat, RecommendationScores: stalePersisted}
	if got := scoreForSort(onlyPersisted, ArticleSortQuality); got != 22 {
		t.Errorf("Quality via persisted (stale feat) = %d, want 22", got)
	}

	// Nothing stored -> 0.
	empty := model.Article{}
	if got := scoreForSort(empty, ArticleSortRecommend); got != 0 {
		t.Errorf("empty article should sort to 0, got %d", got)
	}
}

// TestSortArticlesExercisesComparators drives every switch arm of sortArticles to
// keep the comparator helpers covered alongside the scoring-consumption layer.
func TestSortArticlesExercisesComparators(t *testing.T) {
	now := time.Now().UTC()
	mk := func(id int64, quality int, published time.Time) model.Article {
		return model.Article{
			ID:                   id,
			PublishedAt:          published.Format(time.RFC3339),
			RecommendationScores: &model.RecommendationScores{Quality: quality, Composite: quality},
		}
	}

	articles := []model.Article{
		mk(1, 30, now.Add(-3*time.Hour)),
		mk(2, 80, now.Add(-1*time.Hour)),
		mk(3, 80, now.Add(-2*time.Hour)), // tied quality with id=2, older
	}

	// ArticleSortQuality should surface quality desc, then newer first on ties.
	copyQ := append([]model.Article(nil), articles...)
	sortArticles(copyQ, ArticleSortQuality)
	if copyQ[0].ID != 2 || copyQ[1].ID != 3 || copyQ[2].ID != 1 {
		t.Errorf("quality sort order = [%d,%d,%d], want [2,3,1]", copyQ[0].ID, copyQ[1].ID, copyQ[2].ID)
	}

	// ArticleSortOldest should surface oldest first.
	copyO := append([]model.Article(nil), articles...)
	sortArticles(copyO, ArticleSortOldest)
	if copyO[0].ID != 1 {
		t.Errorf("oldest sort head = %d, want 1", copyO[0].ID)
	}

	// Default (latest) path.
	copyL := append([]model.Article(nil), articles...)
	sortArticles(copyL, "")
	if copyL[0].ID != 2 {
		t.Errorf("latest sort head = %d, want 2", copyL[0].ID)
	}
}

// TestContentNoiseRatio covers the utility function for boilerplate detection.
func TestContentNoiseRatio(t *testing.T) {
	tests := []struct {
		name string
		text string
		zero bool // expect exactly 0
		high bool // expect noise >= 0.5
	}{
		{"empty yields ratio 1", "", false, true}, // defined as 1.0 when no words
		{"all real content", "this paragraph talks about distributed systems and consensus", true, false},
		{"boilerplate heavy", "copyright privacy cookie subscribe terms rights reserved sign login", false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ratio := contentNoiseRatio(tc.text)
			if tc.zero && ratio != 0 {
				t.Errorf("ratio = %v, want 0", ratio)
			}
			if tc.high && ratio < 0.5 {
				t.Errorf("ratio = %v, want >= 0.5", ratio)
			}
			if ratio < 0 || ratio > 1 {
				t.Errorf("ratio = %v out of [0,1]", ratio)
			}
		})
	}
}
