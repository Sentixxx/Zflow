package scheduler

import (
	"context"
	"time"

	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

type FeedRefreshRunner interface {
	RefreshAllFeeds(ctx context.Context) error
}

type FeedRefreshScheduler struct {
	runner   FeedRefreshRunner
	interval time.Duration
	logger   *logger.ModuleLogger
}

func NewFeedRefreshScheduler(runner FeedRefreshRunner, interval time.Duration) *FeedRefreshScheduler {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &FeedRefreshScheduler{
		runner:   runner,
		interval: interval,
		logger:   logger.NewModuleFromEnv("scheduler"),
	}
}

func (s *FeedRefreshScheduler) Start(ctx context.Context) {
	// Run once at startup so users see fresh data quickly.
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("refresh", "feed", "cancelled", "feed refresh scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *FeedRefreshScheduler) runOnce(ctx context.Context) {
	if err := s.runner.RefreshAllFeeds(ctx); err != nil {
		s.logger.Warn("refresh", "feed", "failed", "scheduled refresh finished with error", "error", err.Error())
		return
	}
	s.logger.Debug("refresh", "feed", "ok", "scheduled refresh finished")
}
