package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	logpkg "github.com/Sentixxx/Zflow/backend/pkg/logger"
	readability "github.com/go-shiori/go-readability"
)

var (
	ErrArticleNotFound      = errors.New("article not found")
	ErrArticleLinkEmpty     = errors.New("article link is empty")
	ErrReadabilityFetchFail = errors.New("readability fetch failed")
	ErrSaveArticleContent   = errors.New("failed to save article content")
	ErrInvalidArticleSort   = errors.New("invalid article sort")
)

type ArticleSortMode string

const (
	ArticleSortLatest    ArticleSortMode = "latest"
	ArticleSortOldest    ArticleSortMode = "oldest"
	ArticleSortRecommend ArticleSortMode = "recommend"
	ArticleSortQuality   ArticleSortMode = "quality"
	ArticleSortRelevance ArticleSortMode = "relevance"
	ArticleSortNovelty   ArticleSortMode = "novelty"
)

var articleTokenPattern = regexp.MustCompile(`[[:alnum:]]{2,}`)

type ArticleService struct {
	store             repository.FeedRepository
	readabilityClient func() *http.Client
	aiClient          func() *http.Client
	loadScoringAI     func() (ScoringAIConfig, error)
	logger            *logpkg.ModuleLogger
}

func NewArticleService(store repository.FeedRepository, readabilityClient func() *http.Client, opts ...ArticleServiceOption) *ArticleService {
	svc := &ArticleService{
		store:             store,
		readabilityClient: readabilityClient,
		logger:            logpkg.NewModuleFromEnv("service"),
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

type ArticleServiceOption func(*ArticleService)

func WithScoringAI(aiClient func() *http.Client, loadConfig func() (ScoringAIConfig, error)) ArticleServiceOption {
	return func(s *ArticleService) {
		s.aiClient = aiClient
		s.loadScoringAI = loadConfig
	}
}

func NormalizeArticleSortMode(raw string) (ArticleSortMode, error) {
	switch ArticleSortMode(strings.TrimSpace(raw)) {
	case "", ArticleSortLatest:
		return ArticleSortLatest, nil
	case ArticleSortOldest:
		return ArticleSortOldest, nil
	case ArticleSortRecommend:
		return ArticleSortRecommend, nil
	case ArticleSortQuality:
		return ArticleSortQuality, nil
	case ArticleSortRelevance:
		return ArticleSortRelevance, nil
	case ArticleSortNovelty:
		return ArticleSortNovelty, nil
	default:
		return "", ErrInvalidArticleSort
	}
}

func (u *ArticleService) List(page int, limit int, sortMode ArticleSortMode, feedID *int64, folderID *int64) ([]model.Article, bool) {
	scopeFeedIDs, scoped := u.resolveArticleScopeFeedIDs(feedID, folderID)
	articles, hasMore := u.store.ListArticleListItems(repository.ArticleListQuery{
		Page:    page,
		Limit:   limit,
		Sort:    string(sortMode),
		Scoped:  scoped,
		FeedIDs: scopeFeedIDs,
	})
	return attachStoredRecommendationScoresToArticles(articles), hasMore
}

func (u *ArticleService) resolveArticleScopeFeedIDs(feedID *int64, folderID *int64) ([]int64, bool) {
	if feedID != nil {
		return []int64{*feedID}, true
	}
	if folderID == nil {
		return nil, false
	}

	folders := u.store.ListFolders()
	descendants := make(map[int64]struct{})
	stack := []int64{*folderID}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := descendants[current]; ok {
			continue
		}
		descendants[current] = struct{}{}
		for _, folder := range folders {
			if folder.ParentID != nil && *folder.ParentID == current {
				stack = append(stack, folder.ID)
			}
		}
	}

	feedIDs := make([]int64, 0)
	for _, feed := range u.store.List() {
		if feed.FolderID == nil {
			continue
		}
		if _, ok := descendants[*feed.FolderID]; ok {
			feedIDs = append(feedIDs, feed.ID)
		}
	}
	return feedIDs, true
}

func (u *ArticleService) Get(id int64) (model.Article, bool) {
	article, ok := u.store.GetArticle(id)
	if !ok {
		return model.Article{}, false
	}
	return attachStoredRecommendationScores(article), true
}

func (u *ArticleService) Delete(id int64) (bool, error) {
	return u.store.DeleteArticle(id)
}

func (u *ArticleService) MarkRead(id int64, read bool) (model.Article, bool, error) {
	article, ok, err := u.store.MarkArticleRead(id, read)
	if !ok || err != nil {
		return article, ok, err
	}
	return attachStoredRecommendationScores(article), ok, nil
}

func (u *ArticleService) MarkFavorite(id int64, favorite bool) (model.Article, bool, error) {
	article, ok, err := u.store.MarkArticleFavorite(id, favorite)
	if !ok || err != nil {
		return article, ok, err
	}
	return attachStoredRecommendationScores(article), ok, nil
}

func (u *ArticleService) ExtractReadable(ctx context.Context, articleID int64) (model.Article, error) {
	article, ok := u.store.GetArticle(articleID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	if strings.TrimSpace(article.Link) == "" {
		return model.Article{}, ErrArticleLinkEmpty
	}
	content, err := u.fetchReadableContent(ctx, article.Link)
	if err != nil {
		return model.Article{}, fmt.Errorf("%w: %v", ErrReadabilityFetchFail, err)
	}
	if err := u.store.UpdateArticleFullContent(articleID, content); err != nil {
		return model.Article{}, fmt.Errorf("%w: %v", ErrSaveArticleContent, err)
	}
	article.FullContent = content
	features := RecomputeArticleFeatures(article)
	if err := u.store.UpdateArticleFeatures(articleID, features); err != nil {
		return model.Article{}, fmt.Errorf("%w: %v", ErrSaveArticleContent, err)
	}
	if err := u.store.UpdateArticleScores(articleID, recommendationScoresFromFeatures(features)); err != nil {
		return model.Article{}, fmt.Errorf("%w: %v", ErrSaveArticleContent, err)
	}
	updated, ok := u.store.GetArticle(articleID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	return attachStoredRecommendationScores(updated), nil
}

func (u *ArticleService) RefreshCache(ctx context.Context, articleID int64) (model.Article, error) {
	article, ok := u.store.GetArticle(articleID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	if strings.TrimSpace(article.Link) != "" {
		content, err := u.fetchReadableContent(ctx, article.Link)
		if err != nil {
			return model.Article{}, fmt.Errorf("%w: %v", ErrReadabilityFetchFail, err)
		}
		if err := u.store.UpdateArticleFullContent(articleID, content); err != nil {
			return model.Article{}, fmt.Errorf("%w: %v", ErrSaveArticleContent, err)
		}
		article.FullContent = content
		features := RecomputeArticleFeatures(article)
		if err := u.store.UpdateArticleFeatures(articleID, features); err != nil {
			return model.Article{}, fmt.Errorf("%w: %v", ErrSaveArticleContent, err)
		}
		if err := u.store.UpdateArticleScores(articleID, recommendationScoresFromFeatures(features)); err != nil {
			return model.Article{}, fmt.Errorf("%w: %v", ErrSaveArticleContent, err)
		}
	}
	updated, ok := u.store.GetArticle(articleID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	return attachStoredRecommendationScores(updated), nil
}

func attachStoredRecommendationScores(article model.Article) model.Article {
	if article.ArticleFeatures != nil && article.ArticleFeatures.FeatureVersion >= articleFeatureVersion {
		if article.RecommendationScores == nil || !hasMeaningfulScores(*article.RecommendationScores) {
			scores := recommendationScoresFromFeatures(*article.ArticleFeatures)
			article.RecommendationScores = &scores
		}
		// Expose depth/freshness/reasoning directly on Article for API consumers.
		// These dimensions are stored in article_features but not in the entries table,
		// so we surface them here rather than changing the scores aggregate struct.
		article.ScoreDepth = article.ArticleFeatures.Depth
		article.ScoreFreshness = article.ArticleFeatures.Freshness
		article.ScoreReasoning = article.ArticleFeatures.Reasoning
		return article
	}
	return article
}

func attachStoredRecommendationScoresToArticles(articles []model.Article) []model.Article {
	for i := range articles {
		articles[i] = attachStoredRecommendationScores(articles[i])
	}
	return articles
}

func (u *ArticleService) persistArticleScoring(articleID int64, features model.ArticleFeatures, scores model.RecommendationScores) {
	if articleID <= 0 {
		return
	}
	_ = u.store.UpdateArticleFeatures(articleID, features)
	_ = u.store.UpdateArticleScores(articleID, scores)
}

func (u *ArticleService) RefreshStaleScores(ctx context.Context, limit int) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	articles := u.store.ListArticlesNeedingScoreRefresh(articleFeatureVersion, limit)
	if len(articles) == 0 {
		return 0, nil
	}
	features := scoreArticles(articles)

	// Try to load AI config for LLM-enhanced scoring
	var aiCfg *ScoringAIConfig
	var aiClient *http.Client
	if u.loadScoringAI != nil && u.aiClient != nil {
		cfg, err := u.loadScoringAI()
		if err == nil && cfg.APIKey != "" && cfg.Model != "" {
			aiCfg = &cfg
			aiClient = u.aiClient()
		}
	}

	refreshed := 0
	for i := range articles {
		if err := ctx.Err(); err != nil {
			return refreshed, err
		}
		feat := features[i]

		// LLM enhancement: only for valid/degraded articles with enough content
		if aiCfg != nil && feat.GateStatus != model.ArticleGateInvalid {
			llmResult, err := callLLMForScoring(ctx, aiClient, *aiCfg, articles[i], u.logger)
			if err != nil {
				u.logger.Debug("scoring", "llm", "failed", "LLM scoring failed, using rule-based", "article_id", articles[i].ID, "error", err.Error())
			} else {
				feat = blendLLMScores(feat, llmResult)
				// Recompute composite with blended scores
				composite := int(float64(feat.Quality)*0.33 +
					float64(feat.Relevance)*0.27 +
					float64(feat.Depth)*0.20 +
					float64(feat.Freshness)*0.08 +
					float64(feat.Novelty)*0.12)
				switch feat.GateStatus {
				case model.ArticleGateInvalid:
					composite = min(composite, 30)
				case model.ArticleGateDegraded:
					composite = min(composite, 70)
				}
				feat.Composite = clampScore(composite)
			}
		}

		scores := recommendationScoresFromFeatures(feat)
		if err := u.store.UpdateArticleFeatures(articles[i].ID, feat); err != nil {
			return refreshed, err
		}
		if err := u.store.UpdateArticleScores(articles[i].ID, scores); err != nil {
			return refreshed, err
		}
		refreshed++
	}
	return refreshed, nil
}

func sortArticles(articles []model.Article, sortMode ArticleSortMode) {
	mode := sortMode
	if mode == "" {
		mode = ArticleSortLatest
	}

	slices.SortStableFunc(articles, func(a, b model.Article) int {
		aTime, _ := articleTimestamp(a)
		bTime, _ := articleTimestamp(b)
		aScore := scoreForSort(a, mode)
		bScore := scoreForSort(b, mode)

		switch mode {
		case ArticleSortOldest:
			if cmp := compareTimes(aTime, bTime); cmp != 0 {
				return cmp
			}
		case ArticleSortRecommend, ArticleSortQuality, ArticleSortRelevance, ArticleSortNovelty:
			if aScore != bScore {
				return compareIntsDesc(aScore, bScore)
			}
			if cmp := compareTimesDesc(aTime, bTime); cmp != 0 {
				return cmp
			}
		default:
			if cmp := compareTimesDesc(aTime, bTime); cmp != 0 {
				return cmp
			}
		}

		return compareInt64Desc(a.ID, b.ID)
	})
}

func scoreForSort(article model.Article, sortMode ArticleSortMode) int {
	scores := model.RecommendationScores{}
	if article.ArticleFeatures != nil && article.ArticleFeatures.FeatureVersion >= articleFeatureVersion {
		scores = recommendationScoresFromFeatures(*article.ArticleFeatures)
	} else if article.RecommendationScores != nil {
		scores = *article.RecommendationScores
	}
	switch sortMode {
	case ArticleSortQuality:
		return scores.Quality
	case ArticleSortRelevance:
		return scores.Relevance
	case ArticleSortNovelty:
		return scores.Novelty
	case ArticleSortRecommend:
		return scores.Composite
	default:
		return 0
	}
}

func hasMeaningfulScores(scores model.RecommendationScores) bool {
	return scores.Quality > 0 || scores.Relevance > 0 || scores.Novelty > 0 || scores.Composite > 0
}

func uniqueTokens(text string) []string {
	raw := articleTokenPattern.FindAllString(strings.ToLower(text), -1)
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, token := range raw {
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func articleTimestamp(article model.Article) (time.Time, bool) {
	for _, raw := range []string{strings.TrimSpace(article.PublishedAt), strings.TrimSpace(article.CreatedAt)} {
		if raw == "" {
			continue
		}
		if ts, ok := parseArticleTime(raw); ok {
			return ts, true
		}
	}
	return time.Time{}, false
}

func parseArticleTime(raw string) (time.Time, bool) {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC850,
	}
	for _, layout := range formats {
		ts, err := time.Parse(layout, raw)
		if err == nil {
			return ts.UTC(), true
		}
	}
	return time.Time{}, false
}

func articleAgeHours(article model.Article) float64 {
	ts, ok := articleTimestamp(article)
	if !ok {
		return 24 * 365
	}
	delta := time.Since(ts)
	if delta < 0 {
		return 0
	}
	return delta.Hours()
}

func normalizeByCap(value int, cap int) float64 {
	if cap <= 0 || value <= 0 {
		return 0
	}
	if value > cap {
		value = cap
	}
	return float64(value) / float64(cap)
}

func clampScore(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func compareTimes(a, b time.Time) int {
	switch {
	case a.Before(b):
		return -1
	case a.After(b):
		return 1
	default:
		return 0
	}
}

func compareTimesDesc(a, b time.Time) int {
	return compareTimes(b, a)
}

func compareIntsDesc(a, b int) int {
	switch {
	case a > b:
		return -1
	case a < b:
		return 1
	default:
		return 0
	}
}

func compareInt64Desc(a, b int64) int {
	switch {
	case a > b:
		return -1
	case a < b:
		return 1
	default:
		return 0
	}
}

func (u *ArticleService) fetchReadableContent(ctx context.Context, rawURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Zflow/0.1 (+https://github.com/Sentixxx/Zflow)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := u.readabilityClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.Contains(contentType, "application/pdf") || bytes.HasPrefix(rawBody, []byte("%PDF-")) {
		return "", errors.New("unsupported readability content type: pdf")
	}
	doc, err := readability.FromReader(bytes.NewReader(rawBody), parsedURL)
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		content = strings.TrimSpace(doc.TextContent)
	}
	if content == "" {
		return "", errors.New("empty readable content")
	}
	return content, nil
}
