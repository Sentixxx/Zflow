package model

type SummaryDebug struct {
	Strategy            string `json:"strategy,omitempty"`
	QueryMode           string `json:"query_mode,omitempty"`
	ChunkCount          int    `json:"chunk_count,omitempty"`
	WindowCount         int    `json:"window_count,omitempty"`
	RewritePassed       bool   `json:"rewrite_passed,omitempty"`
	FinalSentenceClosed bool   `json:"final_sentence_closed,omitempty"`
	UsedAI              bool   `json:"used_ai,omitempty"`
}

type ArticleGateStatus string

const (
	ArticleGateValid    ArticleGateStatus = "valid"
	ArticleGateDegraded ArticleGateStatus = "degraded"
	ArticleGateInvalid  ArticleGateStatus = "invalid"
)

type RecommendationScores struct {
	Quality   int `json:"quality"`
	Relevance int `json:"relevance"`
	Novelty   int `json:"novelty"`
	Composite int `json:"composite"`
}

type ArticleSourceField struct {
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	ValueHTML string `json:"value_html,omitempty"`
}

type ArticleSourcePayload struct {
	FeedType    string               `json:"feed_type,omitempty"`
	Title       string               `json:"title,omitempty"`
	Link        string               `json:"link,omitempty"`
	Summary     string               `json:"summary,omitempty"`
	PublishedAt string               `json:"published_at,omitempty"`
	Fields      []ArticleSourceField `json:"fields,omitempty"`
}

type ArticleFeatures struct {
	GateStatus         ArticleGateStatus `json:"-"`
	Quality            int               `json:"-"`
	Relevance          int               `json:"-"`
	Depth              int               `json:"-"`
	Freshness          int               `json:"-"`
	Novelty            int               `json:"-"`
	Composite          int               `json:"-"`
	Reasoning          string            `json:"-"`
	ContentFingerprint string            `json:"-"`
	FeatureVersion     int               `json:"-"`
	ScoredAt           string            `json:"-"`
}

type Article struct {
	ID                   int64                 `json:"id"`
	FeedID               int64                 `json:"feed_id"`
	Title                string                `json:"title"`
	Link                 string                `json:"link"`
	Summary              string                `json:"summary,omitempty"`
	AISummary            string                `json:"ai_summary,omitempty"`
	AISummaryStatus      string                `json:"ai_summary_status,omitempty"`
	AISummaryAt          string                `json:"ai_summary_updated_at,omitempty"`
	DisplaySummary       string                `json:"display_summary,omitempty"`
	DisplaySummaryStatus string                `json:"display_summary_status,omitempty"`
	DisplaySummaryAt     string                `json:"display_summary_updated_at,omitempty"`
	FullContent          string                `json:"full_content,omitempty"`
	SourcePayload        *ArticleSourcePayload `json:"source_payload,omitempty"`
	CoverURL             string                `json:"cover_url,omitempty"`
	PublishedAt          string                `json:"published_at,omitempty"`
	IsRead               bool                  `json:"is_read"`
	IsFavorite           bool                  `json:"is_favorite"`
	FavoritedAt          string                `json:"favorited_at,omitempty"`
	CreatedAt            string                `json:"created_at"`
	SummaryDebug         *SummaryDebug         `json:"summary_debug,omitempty"`
	RecommendationScores *RecommendationScores `json:"recommendation_scores,omitempty"`
	// ScoreDepth, ScoreFreshness, ScoreReasoning expose five-dim scoring fields
	// that are populated from ArticleFeatures when serving article detail responses.
	ScoreDepth      int    `json:"score_depth,omitempty"`
	ScoreFreshness  int    `json:"score_freshness,omitempty"`
	ScoreReasoning  string `json:"score_reasoning,omitempty"`
	ArticleFeatures *ArticleFeatures `json:"-"`
}
