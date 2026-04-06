package repository

import "context"

type VectorMatch struct {
	SourceType string
	SourceID   int64
	Score      float64 // cosine similarity (0-1, higher = more similar)
}

type VectorRepository interface {
	StoreEmbedding(ctx context.Context, sourceType string, sourceID int64, model string, embedding []float32) error
	SearchSimilar(ctx context.Context, sourceType string, queryEmbedding []float32, limit int) ([]VectorMatch, error)
	FindSimilarToSource(ctx context.Context, sourceType string, sourceID int64, threshold float64, limit int) ([]VectorMatch, error)
	DeleteEmbedding(ctx context.Context, sourceType string, sourceID int64) error
	HasEmbedding(ctx context.Context, sourceType string, sourceID int64) (bool, error)
	GetEmbedding(ctx context.Context, sourceType string, sourceID int64) ([]float32, error)
}
