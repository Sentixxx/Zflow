package repository

import (
	"context"

	"github.com/Sentixxx/Zflow/backend/internal/model"
)

type AgentRepository interface {
	// Agent runs
	CreateAgentRun(ctx context.Context, agentType string, inputSummary string) (model.AgentRun, error)
	CompleteAgentRun(ctx context.Context, id int64, outputSummary string, itemsProcessed, itemsCreated int) error
	FailAgentRun(ctx context.Context, id int64, errMsg string) error
	ListAgentRuns(ctx context.Context, agentType string, limit int) ([]model.AgentRun, error)

	// Interest profiles
	ListInterestProfiles(ctx context.Context) ([]model.InterestProfile, error)
	GetInterestProfile(ctx context.Context, id int64) (model.InterestProfile, bool, error)
	GetInterestProfileEmbedding(ctx context.Context, id int64) ([]float32, error)
	CreateInterestProfile(ctx context.Context, label string, embedding []float32, weight float64, articleIDs []int64) (model.InterestProfile, error)
	UpdateInterestProfileEmbedding(ctx context.Context, id int64, embedding []float32, weight float64, articleIDs []int64) error
	DeleteInterestProfile(ctx context.Context, id int64) error
	FindMatchingInterestProfile(ctx context.Context, embedding []float32, threshold float64) (model.InterestProfile, bool, error)
	ScoreArticleAgainstInterests(ctx context.Context, articleEmbedding []float32) (float64, error)

	// Topic clusters
	ListActiveClusters(ctx context.Context) ([]model.TopicCluster, error)
	GetCluster(ctx context.Context, id int64) (model.TopicCluster, bool, error)
	CreateCluster(ctx context.Context, title string, centroidEmbedding []float32) (model.TopicCluster, error)
	UpdateClusterCentroid(ctx context.Context, id int64, centroidEmbedding []float32, articleCount int) error
	UpdateClusterSummary(ctx context.Context, id int64, title, summary string) error
	ArchiveCluster(ctx context.Context, id int64) error
	AddClusterMember(ctx context.Context, clusterID, articleID int64, similarity float64, isRepresentative bool) error
	ListClusterMembers(ctx context.Context, clusterID int64) ([]model.TopicClusterMember, error)
	GetArticleCluster(ctx context.Context, articleID int64) (*model.TopicCluster, error)
	FindMatchingCluster(ctx context.Context, embedding []float32, threshold float64) (model.TopicCluster, bool, error)
	SetRepresentativeArticle(ctx context.Context, clusterID, articleID int64) error

	// Topic briefs
	CreateTopicBrief(ctx context.Context, brief model.TopicBrief) (model.TopicBrief, error)
	GetTopicBrief(ctx context.Context, id int64) (model.TopicBrief, bool, error)
	ListTopicBriefs(ctx context.Context, level string, limit int) ([]model.TopicBrief, error)
	GetLatestBrief(ctx context.Context, level string) (model.TopicBrief, bool, error)
}
