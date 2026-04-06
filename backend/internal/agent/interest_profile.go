package agent

import (
	"context"
	"fmt"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

const (
	interestProfileName              = "interest_profile"
	interestMatchThreshold           = 0.80
	interestFavoriteWeightMultiplier = 3.0
	maxSourceArticlesPerProfile      = 50
)

// InterestProfileAgent extracts user interest vectors from reading behavior
// (read/favorite) and maintains interest profiles for relevance boosting.
type InterestProfileAgent struct {
	deps   Deps
	logger *logger.ModuleLogger
}

func NewInterestProfileAgent(deps Deps) *InterestProfileAgent {
	return &InterestProfileAgent{
		deps:   deps,
		logger: logger.NewModuleFromEnv("agent"),
	}
}

func (a *InterestProfileAgent) Name() string { return interestProfileName }

func (a *InterestProfileAgent) Run(ctx context.Context) error {
	articles := a.deps.FeedRepo.ListArticles()
	if len(articles) == 0 {
		return nil
	}

	var signals []signalArticle
	for _, art := range articles {
		if art.IsFavorite {
			signals = append(signals, signalArticle{article: art, weight: interestFavoriteWeightMultiplier})
		} else if art.IsRead {
			signals = append(signals, signalArticle{article: art, weight: 1.0})
		}
	}

	if len(signals) == 0 {
		return nil
	}

	processed := 0
	for _, sa := range signals {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := a.processSignal(ctx, sa); err != nil {
			a.logger.Warn("run", interestProfileName, "failed", "process signal article", "article_id", fmt.Sprint(sa.article.ID), "error", err.Error())
			continue
		}
		processed++
	}

	a.logger.Info("run", interestProfileName, "ok", "processed signals", "count", fmt.Sprint(processed))
	return nil
}

type signalArticle struct {
	article model.Article
	weight  float64
}

func (a *InterestProfileAgent) processSignal(ctx context.Context, sa signalArticle) error {
	embedding, err := a.deps.VectorRepo.GetEmbedding(ctx, "article", sa.article.ID)
	if err != nil {
		return fmt.Errorf("get article embedding: %w", err)
	}
	if embedding == nil {
		return nil
	}

	profile, found, err := a.deps.AgentRepo.FindMatchingInterestProfile(ctx, embedding, interestMatchThreshold)
	if err != nil {
		return fmt.Errorf("find matching profile: %w", err)
	}

	if found {
		return a.reinforceProfile(ctx, profile, embedding, sa)
	}

	label := fmt.Sprintf("interest from: %s", truncateString(sa.article.Title, 60))
	_, err = a.deps.AgentRepo.CreateInterestProfile(ctx, label, embedding, sa.weight, []int64{sa.article.ID})
	if err != nil {
		return fmt.Errorf("create interest profile: %w", err)
	}
	a.logger.Debug("run", interestProfileName, "ok", "created new interest profile", "article_id", fmt.Sprint(sa.article.ID), "label", label)
	return nil
}

func (a *InterestProfileAgent) reinforceProfile(ctx context.Context, profile model.InterestProfile, newEmbedding []float32, sa signalArticle) error {
	for _, id := range profile.SourceArticleIDs {
		if id == sa.article.ID {
			return nil
		}
	}

	currentEmbedding, err := a.deps.AgentRepo.GetInterestProfileEmbedding(ctx, profile.ID)
	if err != nil || currentEmbedding == nil {
		return err
	}

	totalWeight := profile.Weight + sa.weight
	merged := make([]float32, len(currentEmbedding))
	for i := range merged {
		merged[i] = float32((float64(currentEmbedding[i])*profile.Weight + float64(newEmbedding[i])*sa.weight) / totalWeight)
	}

	articleIDs := append(profile.SourceArticleIDs, sa.article.ID)
	if len(articleIDs) > maxSourceArticlesPerProfile {
		articleIDs = articleIDs[len(articleIDs)-maxSourceArticlesPerProfile:]
	}

	return a.deps.AgentRepo.UpdateInterestProfileEmbedding(ctx, profile.ID, merged, totalWeight, articleIDs)
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
