package store

import "github.com/Sentixxx/Zflow/backend/internal/repository"

// Deprecated: use repository.ArticleSeed.
type ArticleSeed = repository.ArticleSeed

// Deprecated: use repository.SQLiteFeedRepository.
type FeedStore = repository.SQLiteFeedRepository

// Deprecated: use repository.ErrFeedExists.
var ErrFeedExists = repository.ErrFeedExists

// Deprecated: use repository.ErrFolderNameEmpty.
var ErrFolderNameEmpty = repository.ErrFolderNameEmpty

// Deprecated: use repository.NewSQLiteFeedRepository.
func NewFeedStore(path string) (*FeedStore, error) {
	return repository.NewSQLiteFeedRepository(path)
}
