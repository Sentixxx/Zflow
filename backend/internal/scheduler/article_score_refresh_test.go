package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	schedulermock "github.com/Sentixxx/Zflow/backend/internal/scheduler/mock"
)

// This file complements feed_refresh_test.go (which already contains three
// ArticleScoreRefreshScheduler tests) by tightening two contracts that were
// previously only covered incidentally:
//   1. Explicit interval/batchSize values are preserved by the constructor
//      (no default substitution on legitimate positive input).
//   2. The ticker actually produces multiple calls over time (not just the
//      startup runOnce), with the call count scaling with elapsed time.
//   3. runOnce's success branch (err==nil, debug-log path) is exercised.
//
// Tests here must not duplicate the three existing cases in feed_refresh_test.go.

// TestArticleScoreRefreshScheduler_ExplicitValuesKept ensures positive params
// bypass the default-substitution branch in the constructor.
func TestArticleScoreRefreshScheduler_ExplicitValuesKept(t *testing.T) {
	runner := &schedulermock.MockArticleScoreRefreshRunner{}
	s := NewArticleScoreRefreshScheduler(runner, 250*time.Millisecond, 42)
	if s.interval != 250*time.Millisecond {
		t.Fatalf("interval = %s, want 250ms", s.interval)
	}
	if s.batchSize != 42 {
		t.Fatalf("batchSize = %d, want 42", s.batchSize)
	}
}

// TestArticleScoreRefreshScheduler_NegativeValuesUseDefaults covers the <=0
// branch in NewArticleScoreRefreshScheduler for both parameters.
func TestArticleScoreRefreshScheduler_NegativeValuesUseDefaults(t *testing.T) {
	runner := &schedulermock.MockArticleScoreRefreshRunner{}
	s := NewArticleScoreRefreshScheduler(runner, -1*time.Second, -3)
	if s.interval != time.Minute {
		t.Fatalf("interval = %s, want 1m", s.interval)
	}
	if s.batchSize != 50 {
		t.Fatalf("batchSize = %d, want 50", s.batchSize)
	}
}

// TestArticleScoreRefreshScheduler_TickerFiresMultipleTimes asserts that the
// ticker branch keeps calling the runner beyond the startup runOnce. Using a
// busy-wait on atomic count instead of a fixed sleep minimizes CI flakiness.
func TestArticleScoreRefreshScheduler_TickerFiresMultipleTimes(t *testing.T) {
	runner := &schedulermock.MockArticleScoreRefreshRunner{}
	s := NewArticleScoreRefreshScheduler(runner, 5*time.Millisecond, 4)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	// Wait up to 1s for >=3 calls (startup + >=2 ticks). This proves the
	// `<-ticker.C` branch is actually reached more than once, not just the
	// pre-loop runOnce.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if runner.Count.Load() >= 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	if got := runner.Count.Load(); got < 3 {
		t.Fatalf("refresh count = %d, want >= 3 (startup + >=2 ticks)", got)
	}
}

// TestArticleScoreRefreshScheduler_SuccessPathReturnsPositive exercises the
// success branch of runOnce where refreshed > 0 drives the debug log path.
// We use a local runner (not the shared mock) so the assertion about the
// returned `refreshed` value is explicit and local.
func TestArticleScoreRefreshScheduler_SuccessPathReturnsPositive(t *testing.T) {
	runner := &countingArticleScoreRunner{refreshed: 7}
	s := NewArticleScoreRefreshScheduler(runner, time.Hour, 25)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()
	cancel()
	<-done

	if got := runner.count.Load(); got != 1 {
		t.Fatalf("runner call count = %d, want 1 (startup only)", got)
	}
	if got := runner.lastBatchSize.Load(); got != 25 {
		t.Fatalf("batchSize propagated = %d, want 25", got)
	}
}

// countingArticleScoreRunner is a local test double recording batchSize.
// It exists because the shared mock does not expose the batchSize argument,
// and mock/*.go is outside the allowed write surface for this task.
type countingArticleScoreRunner struct {
	count         atomic.Int64
	lastBatchSize atomic.Int64
	refreshed     int
}

func (r *countingArticleScoreRunner) RefreshStaleScores(_ context.Context, limit int) (int, error) {
	r.count.Add(1)
	r.lastBatchSize.Store(int64(limit))
	return r.refreshed, nil
}
