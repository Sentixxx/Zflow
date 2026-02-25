package domain

type Feed struct {
	ID              int64  `json:"id"`
	URL             string `json:"url"`
	Title           string `json:"title"`
	ItemCount       int    `json:"item_count"`
	LastFetchedAt   string `json:"last_fetched_at"`
	LastFetchStatus string `json:"last_fetch_status"`
	LastFetchError  string `json:"last_fetch_error,omitempty"`
	CreatedAt       string `json:"created_at"`
}
