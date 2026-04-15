package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	schedulermock "github.com/Sentixxx/Zflow/backend/internal/scheduler/mock"
)

type stubRunner struct {
	count atomic.Int64
}

func (r *stubRunner) RefreshAllFeeds(_ context.Context) error {
	r.count.Add(1)
	return nil
}

func TestNewFeedRefreshSchedulerDefaultsInterval(t *testing.T) {
	runner := &stubRunner{}
	s := NewFeedRefreshScheduler(runner, 0)
	if s.interval != 15*time.Minute {
		t.Fatalf("interval = %s, want 15m", s.interval)
	}
}

func TestStartRunsImmediatelyAndThenTicks(t *testing.T) {
	runner := &stubRunner{}
	s := NewFeedRefreshScheduler(runner, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	time.Sleep(35 * time.Millisecond)
	cancel()
	<-done

	if got := runner.count.Load(); got < 2 {
		t.Fatalf("refresh count = %d, want >= 2", got)
	}
}

// --- ArticleScoreRefreshScheduler ---

func TestArticleScoreRefreshScheduler_When_ZeroParams_Should_UseDefaults(t *testing.T) {
	runner := &schedulermock.MockArticleScoreRefreshRunner{}
	s := NewArticleScoreRefreshScheduler(runner, 0, 0)
	if s.interval != time.Minute {
		t.Fatalf("interval = %s, want 1m", s.interval)
	}
	if s.batchSize != 50 {
		t.Fatalf("batchSize = %d, want 50", s.batchSize)
	}
}

func TestArticleScoreRefreshScheduler_When_CtxCancelled_Should_Stop(t *testing.T) {
	runner := &schedulermock.MockArticleScoreRefreshRunner{}
	s := NewArticleScoreRefreshScheduler(runner, 10*time.Millisecond, 10)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	// 120ms 给高负载 CI 的 ticker 抖动留 12x 余量（interval=10ms）
	time.Sleep(120 * time.Millisecond)
	cancel()
	<-done

	// Must have run at least twice (startup + >=1 tick)
	if got := runner.Count.Load(); got < 2 {
		t.Fatalf("refresh count = %d, want >= 2", got)
	}
}

func TestArticleScoreRefreshScheduler_When_RunnerErrors_Should_NotPanic(t *testing.T) {
	runner := &schedulermock.MockArticleScoreRefreshRunner{ReturnErr: true}
	s := NewArticleScoreRefreshScheduler(runner, 10*time.Millisecond, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	// Must not panic even when runner returns an error on every call
	s.Start(ctx)

	if runner.Count.Load() < 1 {
		t.Fatal("runner was never called")
	}
}
