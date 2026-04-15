package mock

import (
	"context"
	"errors"
	"sync/atomic"
)

// MockArticleScoreRefreshRunner is a test double for scheduler.ArticleScoreRefreshRunner.
// Count tracks how many times RefreshStaleScores was called.
// ReturnErr controls whether the mock returns a non-nil error.
type MockArticleScoreRefreshRunner struct {
	Count     atomic.Int64
	ReturnErr bool
}

func (m *MockArticleScoreRefreshRunner) RefreshStaleScores(_ context.Context, _ int) (int, error) {
	m.Count.Add(1)
	if m.ReturnErr {
		return 0, errors.New("stub refresh error")
	}
	return 1, nil
}
