package repository

import "github.com/Sentixxx/Zflow/backend/internal/model"

type FeedRepository interface {
	Close() error
	List() []model.Feed
	ListFolders() []model.Folder
	CreateFolder(name string, parentID *int64) (model.Folder, error)
	UpdateFolder(id int64, name string, parentID *int64) (model.Folder, bool, error)
	DeleteFolder(id int64) (bool, error)
	AddInFolder(url, title string, items []ArticleSeed, fetchErr string, folderID *int64, etag string, lastModified string) (model.Feed, error)
	UpdateFeedFolder(id int64, folderID *int64) (model.Feed, bool, error)
	DeleteFeed(id int64) (bool, error)
	GetFeed(id int64) (model.Feed, bool, error)
	GetFeedByURL(rawURL string) (model.Feed, bool, error)
	CreateFeedPlaceholder(url string, title string, folderID *int64) (model.Feed, error)
	UpdateFeedAfterRefresh(feedID int64, title string, items []ArticleSeed, fetchErr, etag, lastModified string) error
	UpdateFeedScript(id int64, script string, lang string) (model.Feed, bool, error)
	UpdateFeedTitle(id int64, title string) (model.Feed, bool, error)
	UpdateFeedIcon(id int64, iconPath string) (model.Feed, bool, error)
	GetSetting(key string) (string, bool, error)
	SetSetting(key, value string) error
	ListArticles() []model.Article
	DeleteArticle(id int64) (bool, error)
	GetArticle(id int64) (model.Article, bool)
	UpdateArticleFullContent(id int64, content string) error
	MarkArticleRead(id int64, read bool) (model.Article, bool, error)
	MarkArticleFavorite(id int64, favorite bool) (model.Article, bool, error)
	PurgeExpiredArticles(retentionDays int) (int, error)
}
