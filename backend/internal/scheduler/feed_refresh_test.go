package scheduler

import (
	"context"
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
