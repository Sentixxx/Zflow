package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

type EmbeddingRunner interface {
	EmbedPendingArticles(ctx context.Context, batchSize int) (int, error)
}

type EmbeddingRefreshScheduler struct {
	runner    EmbeddingRunner
	interval  time.Duration
	batchSize int
	logger    *logger.ModuleLogger
}

func NewEmbeddingRefreshScheduler(runner EmbeddingRunner, interval time.Duration, batchSize int) *EmbeddingRefreshScheduler {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 20
	}
	return &EmbeddingRefreshScheduler{
		runner:    runner,
		interval:  interval,
		batchSize: batchSize,
		logger:    logger.NewModuleFromEnv("scheduler"),
	}
}

func (s *EmbeddingRefreshScheduler) Start(ctx context.Context) {
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("refresh", "embedding", "cancelled", "embedding refresh scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *EmbeddingRefreshScheduler) runOnce(ctx context.Context) {
	embedded, err := s.runner.EmbedPendingArticles(ctx, s.batchSize)
	if err != nil {
		s.logger.Warn("refresh", "embedding", "failed", "scheduled embedding refresh finished with error", "error", err.Error())
		return
	}
	if embedded > 0 {
		s.logger.Debug("refresh", "embedding", "ok", "scheduled embedding refresh finished", "embedded", fmt.Sprint(embedded))
	}
}
