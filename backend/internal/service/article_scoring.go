package service

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
)

const articleFeatureVersion = 1

var htmlTagPattern = regexp.MustCompile(`<[^>]+>`)

type scoringInput struct {
	article           model.Article
	title             string
	summary           string
	fullContent       string
	body              string
	visibleBody       string
	titleTokens       []string
	bodyTokens        []string
	bodyTokenSet      map[string]struct{}
	paragraphCount    int
	noiseRatio        float64
	overlapRatio      float64
	fingerprintSource string
}

func ArticleFeaturesForArticle(article model.Article) model.ArticleFeatures {
	if article.ArticleFeatures != nil && article.ArticleFeatures.FeatureVersion >= articleFeatureVersion {
		return *article.ArticleFeatures
	}
	return RecomputeArticleFeatures(article)
}

func RecomputeArticleFeatures(article model.Article) model.ArticleFeatures {
	return scoreArticles([]model.Article{article})[0]
}

func RecommendationScoresForArticle(article model.Article) model.RecommendationScores {
	if article.ArticleFeatures != nil && article.ArticleFeatures.FeatureVersion >= articleFeatureVersion {
		return recommendationScoresFromFeatures(*article.ArticleFeatures)
	}
	if article.RecommendationScores != nil && hasMeaningfulScores(*article.RecommendationScores) {
		return *article.RecommendationScores
	}
	return recommendationScoresFromFeatures(scoreArticles([]model.Article{article})[0])
}

func RecommendationScoresForSeed(seed repository.ArticleSeed) model.RecommendationScores {
	if seed.ArticleFeatures != nil && seed.ArticleFeatures.FeatureVersion >= articleFeatureVersion {
		return recommendationScoresFromFeatures(*seed.ArticleFeatures)
	}
	return recommendationScoresFromFeatures(scoreSeedFeatures([]repository.ArticleSeed{seed})[0])
}

func AttachRecommendationScoresToSeeds(items []repository.ArticleSeed) []repository.ArticleSeed {
	if len(items) == 0 {
		return items
	}
	features := scoreSeedFeatures(items)
	scored := make([]repository.ArticleSeed, 0, len(items))
	for i, item := range items {
		next := item
		feature := features[i]
		scores := recommendationScoresFromFeatures(feature)
		next.ArticleFeatures = &feature
		next.RecommendationScores = &scores
		scored = append(scored, next)
	}
	return scored
}

func recommendationScoresFromFeatures(features model.ArticleFeatures) model.RecommendationScores {
	return model.RecommendationScores{
		Quality:   clampScore(features.Quality),
		Relevance: clampScore(features.Relevance),
		Novelty:   clampScore(features.Novelty),
		Composite: clampScore(features.Composite),
	}
}

func scoreSeedFeatures(items []repository.ArticleSeed) []model.ArticleFeatures {
	articles := make([]model.Article, 0, len(items))
	for _, item := range items {
		articles = append(articles, model.Article{
			Title:       item.Title,
			Link:        item.Link,
			Summary:     item.Summary,
			FullContent: item.FullContent,
			CoverURL:    item.CoverURL,
			PublishedAt: item.PublishedAt,
		})
	}
	return scoreArticles(articles)
}

func scoreArticles(articles []model.Article) []model.ArticleFeatures {
	if len(articles) == 0 {
		return nil
	}
	inputs := make([]scoringInput, 0, len(articles))
	for _, article := range articles {
		inputs = append(inputs, buildScoringInput(article))
	}
	features := make([]model.ArticleFeatures, 0, len(inputs))
	for _, input := range inputs {
		features = append(features, scoreBaseFeatures(input))
	}
	applyNoveltyScores(inputs, features)
	for i := range features {
		features[i].Quality = clampScore(features[i].Quality)
		features[i].Relevance = clampScore(features[i].Relevance)
		features[i].Depth = clampScore(features[i].Depth)
		features[i].Freshness = clampScore(features[i].Freshness)
		features[i].Novelty = clampScore(features[i].Novelty)
		composite := int(float64(features[i].Quality)*0.33 +
			float64(features[i].Relevance)*0.27 +
			float64(features[i].Depth)*0.20 +
			float64(features[i].Freshness)*0.08 +
			float64(features[i].Novelty)*0.12)
		switch features[i].GateStatus {
		case model.ArticleGateInvalid:
			composite = min(composite, 30)
		case model.ArticleGateDegraded:
			composite = min(composite, 70)
		}
		features[i].Composite = clampScore(composite)
		features[i].FeatureVersion = articleFeatureVersion
		if strings.TrimSpace(features[i].ScoredAt) == "" {
			features[i].ScoredAt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	return features
}

func buildScoringInput(article model.Article) scoringInput {
	title := strings.TrimSpace(article.Title)
	summary := strings.TrimSpace(article.Summary)
	fullContent := strings.TrimSpace(article.FullContent)
	body := fullContent
	if body == "" {
		body = summary
	}
	visibleBody := normalizedVisibleText(body)
	titleTokens := uniqueTokens(title)
	bodyTokens := uniqueTokens(visibleBody)
	bodyTokenSet := make(map[string]struct{}, len(bodyTokens))
	for _, token := range bodyTokens {
		bodyTokenSet[token] = struct{}{}
	}
	overlapCount := 0
	for _, token := range titleTokens {
		if _, ok := bodyTokenSet[token]; ok {
			overlapCount++
		}
	}
	overlapRatio := 0.0
	if len(titleTokens) > 0 {
		overlapRatio = float64(overlapCount) / float64(len(titleTokens))
	}
	return scoringInput{
		article:           article,
		title:             title,
		summary:           summary,
		fullContent:       fullContent,
		body:              body,
		visibleBody:       visibleBody,
		titleTokens:       titleTokens,
		bodyTokens:        bodyTokens,
		bodyTokenSet:      bodyTokenSet,
		paragraphCount:    visibleParagraphCount(body),
		noiseRatio:        contentNoiseRatio(visibleBody),
		overlapRatio:      overlapRatio,
		fingerprintSource: fingerprintSource(title, visibleBody),
	}
}

func scoreBaseFeatures(input scoringInput) model.ArticleFeatures {
	visibleChars := len([]rune(input.visibleBody))
	tokenCount := len(input.bodyTokens)
	gateStatus := model.ArticleGateValid
	switch {
	case input.title == "" && input.visibleBody == "":
		gateStatus = model.ArticleGateInvalid
	case visibleChars < 40 || tokenCount < 8:
		gateStatus = model.ArticleGateInvalid
	case visibleChars < 120 || tokenCount < 20 || input.noiseRatio >= 0.12:
		gateStatus = model.ArticleGateDegraded
	}

	quality := 18 +
		int(normalizeByCap(visibleChars, 2800)*30+0.5) +
		int(normalizeByCap(input.paragraphCount, 12)*18+0.5) +
		int((1.0-input.noiseRatio)*20+0.5)
	if input.article.Link != "" {
		quality += 6
	}
	if input.article.CoverURL != "" {
		quality += 4
	}
	if input.article.PublishedAt != "" {
		quality += 4
	}

	relevance := 20 +
		int(input.overlapRatio*38.0+0.5) +
		int(normalizeByCap(len(input.titleTokens), 12)*10+0.5) +
		int(normalizeByCap(tokenCount, 60)*22+0.5)

	uniqueRatio := 0.0
	rawTokens := articleTokenPattern.FindAllString(strings.ToLower(input.visibleBody), -1)
	if len(rawTokens) > 0 {
		uniqueRatio = float64(len(input.bodyTokens)) / float64(len(rawTokens))
	}
	depth := 16 +
		int(normalizeByCap(visibleChars, 3200)*24+0.5) +
		int(normalizeByCap(input.paragraphCount, 14)*20+0.5) +
		int(uniqueRatio*20.0+0.5)

	freshness := freshnessScore(input.article)

	switch gateStatus {
	case model.ArticleGateInvalid:
		quality = min(quality, 24)
		relevance = min(relevance, 26)
		depth = min(depth, 20)
	case model.ArticleGateDegraded:
		quality = min(quality, 62)
		depth = min(depth, 58)
	}

	return model.ArticleFeatures{
		GateStatus:         gateStatus,
		Quality:            quality,
		Relevance:          relevance,
		Depth:              depth,
		Freshness:          freshness,
		Novelty:            55,
		ContentFingerprint: hashFingerprint(input.fingerprintSource),
	}
}

func applyNoveltyScores(inputs []scoringInput, features []model.ArticleFeatures) {
	for i := range features {
		maxSimilarity := 0.0
		crowdedCount := 0
		for j := range inputs {
			if i == j {
				continue
			}
			similarity := tokenSimilarity(inputs[i].bodyTokenSet, inputs[j].bodyTokenSet)
			titleSimilarity := tokenSimilarity(tokensToSet(inputs[i].titleTokens), tokensToSet(inputs[j].titleTokens))
			if titleSimilarity > similarity {
				similarity = titleSimilarity
			}
			if similarity > maxSimilarity {
				maxSimilarity = similarity
			}
			if similarity >= 0.55 {
				crowdedCount++
			}
		}
		novelty := 62 + int(float64(features[i].Freshness)*0.18+0.5)
		novelty -= int(maxSimilarity*52.0 + 0.5)
		novelty -= crowdedCount * 9
		switch features[i].GateStatus {
		case model.ArticleGateInvalid:
			novelty = min(novelty, 25)
		case model.ArticleGateDegraded:
			novelty = min(novelty, 58)
		}
		features[i].Novelty = clampScore(novelty)
	}
}

func normalizedVisibleText(raw string) string {
	cleaned := htmlTagPattern.ReplaceAllString(raw, " ")
	return strings.Join(strings.Fields(strings.TrimSpace(cleaned)), " ")
}

func visibleParagraphCount(raw string) int {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	count := 0
	for _, part := range parts {
		if strings.TrimSpace(normalizedVisibleText(part)) != "" {
			count++
		}
	}
	if count == 0 && strings.TrimSpace(normalizedVisibleText(raw)) != "" {
		return 1
	}
	return count
}

func contentNoiseRatio(text string) float64 {
	noiseTokens := []string{"copyright", "subscribe", "privacy", "cookie", "cookies", "terms", "rights", "reserved", "sign", "login"}
	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return 1
	}
	noise := 0
	for _, word := range words {
		for _, token := range noiseTokens {
			if word == token {
				noise++
				break
			}
		}
	}
	return float64(noise) / float64(len(words))
}

func freshnessScore(article model.Article) int {
	ageHours := articleAgeHours(article)
	switch {
	case ageHours <= 24:
		return 88
	case ageHours <= 72:
		return 74
	case ageHours <= 24*7:
		return 60
	case ageHours <= 24*30:
		return 42
	default:
		return 24
	}
}

func fingerprintSource(title, body string) string {
	parts := append(uniqueTokens(title), uniqueTokens(body)...)
	if len(parts) == 0 {
		return ""
	}
	if len(parts) > 128 {
		parts = parts[:128]
	}
	return strings.Join(parts, " ")
}

func hashFingerprint(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func tokenSimilarity(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	union := len(a)
	for token := range b {
		if _, ok := a[token]; ok {
			intersection++
			continue
		}
		union++
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func tokensToSet(tokens []string) map[string]struct{} {
	if len(tokens) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		out[token] = struct{}{}
	}
	return out
}
