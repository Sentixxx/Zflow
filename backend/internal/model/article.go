package model

type Article struct {
	ID          int64  `json:"id"`
	FeedID      int64  `json:"feed_id"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Summary     string `json:"summary,omitempty"`
	FullContent string `json:"full_content,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	IsRead      bool   `json:"is_read"`
	CreatedAt   string `json:"created_at"`
}
