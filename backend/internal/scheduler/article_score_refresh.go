package scheduler

import (
	"context"
	"time"

	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

type ArticleScoreRefreshRunner interface {
	RefreshStaleScores(ctx context.Context, limit int) (int, error)
}

type ArticleScoreRefreshScheduler struct {
	runner    ArticleScoreRefreshRunner
	interval  time.Duration
	batchSize int
	logger    *logger.ModuleLogger
}

func NewArticleScoreRefreshScheduler(runner ArticleScoreRefreshRunner, interval time.Duration, batchSize int) *ArticleScoreRefreshScheduler {
	if interval <= 0 {
		interval = time.Minute
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &ArticleScoreRefreshScheduler{
		runner:    runner,
		interval:  interval,
		batchSize: batchSize,
		logger:    logger.NewModuleFromEnv("scheduler"),
	}
}

func (s *ArticleScoreRefreshScheduler) Start(ctx context.Context) {
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("refresh", "article_score", "cancelled", "article score refresh scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *ArticleScoreRefreshScheduler) runOnce(ctx context.Context) {
	refreshed, err := s.runner.RefreshStaleScores(ctx, s.batchSize)
	if err != nil {
		s.logger.Warn("refresh", "article_score", "failed", "scheduled article score refresh finished with error", "error", err.Error())
		return
	}
	s.logger.Debug("refresh", "article_score", "ok", "scheduled article score refresh finished", "refreshed", refreshed)
}
