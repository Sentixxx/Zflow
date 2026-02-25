package service

import "github.com/Sentixxx/Zflow/backend/internal/repository"

type FeedService interface {
	repository.FeedRepository
}

type feedService struct {
	repository.FeedRepository
}

func NewFeedService(repo repository.FeedRepository) FeedService {
	return &feedService{FeedRepository: repo}
}
