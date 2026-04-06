package agent

import (
	"context"
	"fmt"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

const (
	topicClusterName            = "topic_cluster"
	clusterAssignThreshold      = 0.82
	clusterPairwiseThreshold    = 0.78
	maxPairwiseScanBatch        = 200
)

// TopicClusterAgent clusters similar articles across feeds by cosine
// similarity of their embeddings. Articles above the assign threshold are
// folded into existing clusters; unclustered articles are scanned pairwise
// to form new clusters.
type TopicClusterAgent struct {
	deps   Deps
	logger *logger.ModuleLogger
}

func NewTopicClusterAgent(deps Deps) *TopicClusterAgent {
	return &TopicClusterAgent{
		deps:   deps,
		logger: logger.NewModuleFromEnv("agent"),
	}
}

func (a *TopicClusterAgent) Name() string { return topicClusterName }

func (a *TopicClusterAgent) Run(ctx context.Context) error {
	articles := a.deps.FeedRepo.ListArticles()
	if len(articles) == 0 {
		return nil
	}

	assigned := 0
	var unclustered []model.Article

	// Phase 1: try to assign each article to an existing cluster
	for _, art := range articles {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Skip if already in a cluster
		existing, err := a.deps.AgentRepo.GetArticleCluster(ctx, art.ID)
		if err != nil {
			a.logger.Warn("run", topicClusterName, "failed", "check article cluster", "article_id", fmt.Sprint(art.ID), "error", err.Error())
			continue
		}
		if existing != nil {
			continue
		}

		embedding, err := a.deps.VectorRepo.GetEmbedding(ctx, "article", art.ID)
		if err != nil || embedding == nil {
			unclustered = append(unclustered, art)
			continue
		}

		cluster, found, err := a.deps.AgentRepo.FindMatchingCluster(ctx, embedding, clusterAssignThreshold)
		if err != nil {
			a.logger.Warn("run", topicClusterName, "failed", "find matching cluster", "article_id", fmt.Sprint(art.ID), "error", err.Error())
			unclustered = append(unclustered, art)
			continue
		}

		if found {
			if err := a.addToCluster(ctx, cluster, art, embedding); err != nil {
				a.logger.Warn("run", topicClusterName, "failed", "add to cluster", "article_id", fmt.Sprint(art.ID), "error", err.Error())
			} else {
				assigned++
			}
		} else {
			unclustered = append(unclustered, art)
		}
	}

	// Phase 2: pairwise scan of unclustered articles to form new clusters
	created := a.pairwiseScan(ctx, unclustered)

	a.logger.Info("run", topicClusterName, "ok", "clustering complete", "assigned", fmt.Sprint(assigned), "new_clusters", fmt.Sprint(created))
	return nil
}

func (a *TopicClusterAgent) addToCluster(ctx context.Context, cluster model.TopicCluster, art model.Article, embedding []float32) error {
	// Compute similarity with cluster centroid for record
	matches, err := a.deps.VectorRepo.SearchSimilar(ctx, "article", embedding, 1)
	similarity := 0.0
	if err == nil && len(matches) > 0 {
		similarity = matches[0].Score
	}

	if err := a.deps.AgentRepo.AddClusterMember(ctx, cluster.ID, art.ID, similarity, false); err != nil {
		return err
	}

	// Update centroid: simple running average
	return a.updateCentroid(ctx, cluster.ID, embedding)
}

func (a *TopicClusterAgent) updateCentroid(ctx context.Context, clusterID int64, newEmbedding []float32) error {
	members, err := a.deps.AgentRepo.ListClusterMembers(ctx, clusterID)
	if err != nil {
		return err
	}

	// Recompute centroid as average of all member embeddings
	count := 0
	centroid := make([]float32, len(newEmbedding))
	for _, m := range members {
		emb, err := a.deps.VectorRepo.GetEmbedding(ctx, "article", m.ArticleID)
		if err != nil || emb == nil {
			continue
		}
		for i := range centroid {
			centroid[i] += emb[i]
		}
		count++
	}
	if count == 0 {
		return nil
	}
	for i := range centroid {
		centroid[i] /= float32(count)
	}

	return a.deps.AgentRepo.UpdateClusterCentroid(ctx, clusterID, centroid, count)
}

func (a *TopicClusterAgent) pairwiseScan(ctx context.Context, articles []model.Article) int {
	if len(articles) < 2 {
		return 0
	}

	// Limit batch size
	scan := articles
	if len(scan) > maxPairwiseScanBatch {
		scan = scan[:maxPairwiseScanBatch]
	}

	// Load embeddings
	type articleWithEmb struct {
		article   model.Article
		embedding []float32
	}
	var items []articleWithEmb
	for _, art := range scan {
		emb, err := a.deps.VectorRepo.GetEmbedding(ctx, "article", art.ID)
		if err != nil || emb == nil {
			continue
		}
		items = append(items, articleWithEmb{article: art, embedding: emb})
	}

	if len(items) < 2 {
		return 0
	}

	// Track which articles have been assigned to a new cluster
	assigned := make(map[int64]bool)
	created := 0

	for i := 0; i < len(items); i++ {
		if assigned[items[i].article.ID] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return created
		}

		// Find all articles similar to items[i]
		var group []articleWithEmb
		group = append(group, items[i])
		for j := i + 1; j < len(items); j++ {
			if assigned[items[j].article.ID] {
				continue
			}
			sim := cosineSimilarity(items[i].embedding, items[j].embedding)
			if sim >= clusterPairwiseThreshold {
				group = append(group, items[j])
			}
		}

		if len(group) < 2 {
			continue
		}

		// Compute centroid
		centroid := make([]float32, len(items[0].embedding))
		for _, g := range group {
			for k := range centroid {
				centroid[k] += g.embedding[k]
			}
		}
		for k := range centroid {
			centroid[k] /= float32(len(group))
		}

		// Create cluster
		title := fmt.Sprintf("Topic: %s (+%d)", truncateString(group[0].article.Title, 50), len(group)-1)
		cluster, err := a.deps.AgentRepo.CreateCluster(ctx, title, centroid)
		if err != nil {
			a.logger.Warn("run", topicClusterName, "failed", "create cluster", "error", err.Error())
			continue
		}

		// Add members; pick highest composite_score as representative
		bestScore := -1
		bestArticleID := int64(0)
		for _, g := range group {
			sim := cosineSimilarity(g.embedding, centroid)
			_ = a.deps.AgentRepo.AddClusterMember(ctx, cluster.ID, g.article.ID, float64(sim), false)
			assigned[g.article.ID] = true

			score := 0
			if g.article.RecommendationScores != nil {
				score = g.article.RecommendationScores.Composite
			}
			if score > bestScore {
				bestScore = score
				bestArticleID = g.article.ID
			}
		}

		if bestArticleID > 0 {
			_ = a.deps.AgentRepo.SetRepresentativeArticle(ctx, cluster.ID, bestArticleID)
		}

		created++
	}

	return created
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt(normA) * sqrt(normB))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method
	z := x
	for i := 0; i < 20; i++ {
		z = (z + x/z) / 2
	}
	return z
}
