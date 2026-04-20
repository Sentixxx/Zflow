package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// fakeEmbeddingRunner is an in-file test double for EmbeddingRunner.
// Count tracks call count; LastBatchSize records the batchSize from the most
// recent call; ReturnErr forces a non-nil error on every call; Embedded
// overrides the returned count when >= 0 (default 1 for success path).
type fakeEmbeddingRunner struct {
	count         atomic.Int64
	lastBatchSize atomic.Int64
	returnErr     bool
	embedded      int
}

func (r *fakeEmbeddingRunner) EmbedPendingArticles(_ context.Context, batchSize int) (int, error) {
	r.count.Add(1)
	r.lastBatchSize.Store(int64(batchSize))
	if r.returnErr {
		return 0, errors.New("stub embedding error")
	}
	if r.embedded > 0 {
		return r.embedded, nil
	}
	return 1, nil
}

// TestNewEmbeddingRefreshScheduler_Defaults verifies that zero-valued interval
// and batchSize fall back to the documented defaults (5m / 20).
func TestNewEmbeddingRefreshScheduler_Defaults(t *testing.T) {
	runner := &fakeEmbeddingRunner{}
	s := NewEmbeddingRefreshScheduler(runner, 0, 0)
	if s.interval != 5*time.Minute {
		t.Fatalf("interval = %s, want 5m", s.interval)
	}
	if s.batchSize != 20 {
		t.Fatalf("batchSize = %d, want 20", s.batchSize)
	}
}

// TestNewEmbeddingRefreshScheduler_NegativeValuesUseDefaults verifies the
// defensive branches: negative inputs are treated as "unset".
func TestNewEmbeddingRefreshScheduler_NegativeValuesUseDefaults(t *testing.T) {
	runner := &fakeEmbeddingRunner{}
	s := NewEmbeddingRefreshScheduler(runner, -1*time.Second, -5)
	if s.interval != 5*time.Minute {
		t.Fatalf("interval = %s, want 5m", s.interval)
	}
	if s.batchSize != 20 {
		t.Fatalf("batchSize = %d, want 20", s.batchSize)
	}
}

// TestNewEmbeddingRefreshScheduler_ExplicitValuesKept verifies that explicit
// positive params are preserved verbatim (no default substitution).
func TestNewEmbeddingRefreshScheduler_ExplicitValuesKept(t *testing.T) {
	runner := &fakeEmbeddingRunner{}
	s := NewEmbeddingRefreshScheduler(runner, 30*time.Second, 7)
	if s.interval != 30*time.Second {
		t.Fatalf("interval = %s, want 30s", s.interval)
	}
	if s.batchSize != 7 {
		t.Fatalf("batchSize = %d, want 7", s.batchSize)
	}
}

// TestEmbeddingRefreshScheduler_StartRunsOnceAndTicks verifies the normal path:
// Start() triggers an immediate runOnce and then keeps firing on ticker events.
// Also checks that batchSize is propagated into the runner call.
func TestEmbeddingRefreshScheduler_StartRunsOnceAndTicks(t *testing.T) {
	runner := &fakeEmbeddingRunner{embedded: 3}
	s := NewEmbeddingRefreshScheduler(runner, 5*time.Millisecond, 11)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	// Poll for >=3 calls (startup + >=2 ticks) with a generous deadline so
	// that slow CI does not flake. Max wait 1s, typical ~20ms.
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if runner.count.Load() >= 3 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	if got := runner.count.Load(); got < 3 {
		t.Fatalf("embed count = %d, want >= 3 (startup + >=2 ticks)", got)
	}
	if got := runner.lastBatchSize.Load(); got != 11 {
		t.Fatalf("batchSize propagated = %d, want 11", got)
	}
}

// TestEmbeddingRefreshScheduler_CtxCancelledStops verifies that Start returns
// promptly once the context is cancelled, and no further runner calls happen
// after cancellation.
func TestEmbeddingRefreshScheduler_CtxCancelledStops(t *testing.T) {
	runner := &fakeEmbeddingRunner{}
	// Long interval so the only call is the startup runOnce.
	s := NewEmbeddingRefreshScheduler(runner, time.Hour, 5)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Start did not return within 500ms after ctx cancel")
	}

	if got := runner.count.Load(); got != 1 {
		t.Fatalf("embed count after cancel = %d, want 1 (startup only)", got)
	}
}

// TestEmbeddingRefreshScheduler_RunnerErrorDoesNotPanic verifies that
// repeated runner errors are absorbed by runOnce (warn-log path) and the loop
// keeps ticking without panicking.
func TestEmbeddingRefreshScheduler_RunnerErrorDoesNotPanic(t *testing.T) {
	runner := &fakeEmbeddingRunner{returnErr: true}
	s := NewEmbeddingRefreshScheduler(runner, 5*time.Millisecond, 3)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	// Must not panic even when every call returns an error.
	s.Start(ctx)

	if got := runner.count.Load(); got < 1 {
		t.Fatalf("runner was never called, count=%d", got)
	}
}

// TestEmbeddingRefreshScheduler_ZeroEmbeddedSkipsDebugLog covers the
// `embedded > 0` false branch in runOnce: the runner returns (0, nil), so the
// debug log path is skipped. Behavior contract: no panic, call still counted.
func TestEmbeddingRefreshScheduler_ZeroEmbeddedSkipsDebugLog(t *testing.T) {
	runner := &zeroEmbeddingRunner{}
	s := NewEmbeddingRefreshScheduler(runner, time.Hour, 5)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()
	cancel()
	<-done

	if runner.count.Load() != 1 {
		t.Fatalf("zero-runner count = %d, want 1", runner.count.Load())
	}
}

// zeroEmbeddingRunner always returns (0, nil) — used to exercise the
// `embedded > 0` false branch in runOnce.
type zeroEmbeddingRunner struct {
	count atomic.Int64
}

func (r *zeroEmbeddingRunner) EmbedPendingArticles(_ context.Context, _ int) (int, error) {
	r.count.Add(1)
	return 0, nil
}
