package model

import "time"

type AgentRun struct {
	ID             int64      `json:"id"`
	AgentType      string     `json:"agent_type"`
	Status         string     `json:"status"` // running, completed, failed
	InputSummary   string     `json:"input_summary"`
	OutputSummary  string     `json:"output_summary"`
	ItemsProcessed int        `json:"items_processed"`
	ItemsCreated   int        `json:"items_created"`
	Error          string     `json:"error,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

type InterestProfile struct {
	ID               int64     `json:"id"`
	Label            string    `json:"label"`
	Weight           float64   `json:"weight"`
	SourceArticleIDs []int64   `json:"source_article_ids"`
	LastReinforcedAt *time.Time `json:"last_reinforced_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type TopicCluster struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	ArticleCount int       `json:"article_count"`
	Status       string    `json:"status"` // active, archived
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TopicClusterMember struct {
	ClusterID        int64     `json:"cluster_id"`
	ArticleID        int64     `json:"article_id"`
	Similarity       float64   `json:"similarity"`
	IsRepresentative bool      `json:"is_representative"`
	AddedAt          time.Time `json:"added_at"`
}

type TopicBrief struct {
	ID               int64     `json:"id"`
	Title            string    `json:"title"`
	Slug             string    `json:"slug"`
	Content          string    `json:"content"`
	Level            string    `json:"level"` // daily, weekly, monthly
	PeriodStart      string    `json:"period_start"`
	PeriodEnd        string    `json:"period_end"`
	SourceClusterIDs []int64   `json:"source_cluster_ids"`
	SourceArticleIDs []int64   `json:"source_article_ids"`
	ParentBriefID    *int64    `json:"parent_brief_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
