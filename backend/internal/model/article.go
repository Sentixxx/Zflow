package model

type Article struct {
	ID          int64  `json:"id"`
	FeedID      int64  `json:"feed_id"`
	Title       string `json:"title"`
	Link        string `json:"link"`
	Summary     string `json:"summary,omitempty"`
	FullContent string `json:"full_content,omitempty"`
	CoverURL    string `json:"cover_url,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	IsRead      bool   `json:"is_read"`
	IsFavorite  bool   `json:"is_favorite"`
	FavoritedAt string `json:"favorited_at,omitempty"`
	CreatedAt   string `json:"created_at"`
}
