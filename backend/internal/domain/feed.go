package domain

type Feed struct {
	ID               int64  `json:"id"`
	URL              string `json:"url"`
	Title            string `json:"title"`
	FolderID         *int64 `json:"folder_id,omitempty"`
	CustomScript     string `json:"custom_script,omitempty"`
	CustomScriptLang string `json:"custom_script_lang,omitempty"`
	IconURL          string `json:"icon_url,omitempty"`
	IconPath         string `json:"-"`
	IconFetchedAt    string `json:"-"`
	ItemCount        int    `json:"item_count"`
	LastFetchedAt    string `json:"last_fetched_at"`
	LastFetchStatus  string `json:"last_fetch_status"`
	LastFetchError   string `json:"last_fetch_error,omitempty"`
	ETag             string `json:"etag,omitempty"`
	LastModified     string `json:"last_modified,omitempty"`
	CreatedAt        string `json:"created_at"`
}
