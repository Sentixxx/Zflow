package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
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

type stubScoreRunner struct {
	count     atomic.Int64
	returnErr bool
}

func (r *stubScoreRunner) RefreshStaleScores(_ context.Context, _ int) (int, error) {
	r.count.Add(1)
	if r.returnErr {
		return 0, errors.New("stub refresh error")
	}
	return 1, nil
}

func TestArticleScoreRefreshScheduler_When_ZeroParams_Should_UseDefaults(t *testing.T) {
	runner := &stubScoreRunner{}
	s := NewArticleScoreRefreshScheduler(runner, 0, 0)
	if s.interval != time.Minute {
		t.Fatalf("interval = %s, want 1m", s.interval)
	}
	if s.batchSize != 50 {
		t.Fatalf("batchSize = %d, want 50", s.batchSize)
	}
}

func TestArticleScoreRefreshScheduler_When_CtxCancelled_Should_Stop(t *testing.T) {
	runner := &stubScoreRunner{}
	s := NewArticleScoreRefreshScheduler(runner, 10*time.Millisecond, 10)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	time.Sleep(35 * time.Millisecond)
	cancel()
	<-done

	// Must have run at least twice (startup + >=1 tick)
	if got := runner.count.Load(); got < 2 {
		t.Fatalf("refresh count = %d, want >= 2", got)
	}
}

func TestArticleScoreRefreshScheduler_When_RunnerErrors_Should_NotPanic(t *testing.T) {
	runner := &stubScoreRunner{returnErr: true}
	s := NewArticleScoreRefreshScheduler(runner, 10*time.Millisecond, 5)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	// Must not panic even when runner returns an error on every call
	s.Start(ctx)

	if runner.count.Load() < 1 {
		t.Fatal("runner was never called")
	}
}
