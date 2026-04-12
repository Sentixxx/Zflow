package mock

import (
	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
)

type MockFeedRepository struct {
	CloseFunc                             func() error
	ListFunc                              func() []model.Feed
	ListFoldersFunc                       func() []model.Folder
	CreateFolderFunc                      func(name string, parentID *int64) (model.Folder, error)
	UpdateFolderFunc                      func(id int64, name string, parentID *int64) (model.Folder, bool, error)
	DeleteFolderFunc                      func(id int64) (bool, error)
	AddInFolderFunc                       func(url, title string, items []repository.ArticleSeed, fetchErr string, folderID *int64, etag string, lastModified string) (model.Feed, error)
	UpdateFeedFolderFunc                  func(id int64, folderID *int64) (model.Feed, bool, error)
	UpdateFeedRetentionDaysFunc           func(id int64, retentionDays int) (model.Feed, bool, error)
	DeleteFeedFunc                        func(id int64) (bool, error)
	GetFeedFunc                           func(id int64) (model.Feed, bool, error)
	GetFeedByURLFunc                      func(rawURL string) (model.Feed, bool, error)
	CreateFeedPlaceholderFunc             func(url string, title string, folderID *int64) (model.Feed, error)
	UpdateFeedAfterRefreshFunc            func(feedID int64, title string, items []repository.ArticleSeed, fetchErr, etag, lastModified string) error
	UpdateFeedScriptFunc                  func(id int64, script string, lang string) (model.Feed, bool, error)
	UpdateFeedTitleFunc                   func(id int64, title string) (model.Feed, bool, error)
	UpdateFeedIconFunc                    func(id int64, iconPath string) (model.Feed, bool, error)
	GetSettingFunc                        func(key string) (string, bool, error)
	SetSettingFunc                        func(key, value string) error
	ListArticlesFunc                      func() []model.Article
	ListArticleListItemsFunc              func(query repository.ArticleListQuery) ([]model.Article, bool)
	ListArticlesNeedingScoreRefreshFunc   func(featureVersion int, limit int) []model.Article
	ListArticlesMissingDisplaySummaryFunc func(feedID int64, limit int) []model.Article
	DeleteArticleFunc                     func(id int64) (bool, error)
	GetArticleFunc                        func(id int64) (model.Article, bool)
	UpdateArticleFullContentFunc          func(id int64, content string) error
	UpdateArticleSummaryStateFunc         func(id int64, aiSummary string, aiStatus string, displaySummary string, displayStatus string) error
	UpdateArticleDisplaySummaryFunc       func(id int64, summary string, status string) error
	UpdateArticleScoresFunc               func(id int64, scores model.RecommendationScores) error
	UpdateArticleFeaturesFunc             func(id int64, features model.ArticleFeatures) error
	MarkArticleReadFunc                   func(id int64, read bool) (model.Article, bool, error)
	MarkArticleFavoriteFunc               func(id int64, favorite bool) (model.Article, bool, error)
	PurgeExpiredArticlesFunc              func(retentionDays int) (int, error)
}

func (m *MockFeedRepository) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockFeedRepository) List() []model.Feed {
	if m.ListFunc != nil {
		return m.ListFunc()
	}
	return nil
}

func (m *MockFeedRepository) ListFolders() []model.Folder {
	if m.ListFoldersFunc != nil {
		return m.ListFoldersFunc()
	}
	return nil
}

func (m *MockFeedRepository) CreateFolder(name string, parentID *int64) (model.Folder, error) {
	if m.CreateFolderFunc != nil {
		return m.CreateFolderFunc(name, parentID)
	}
	return model.Folder{}, nil
}

func (m *MockFeedRepository) UpdateFolder(id int64, name string, parentID *int64) (model.Folder, bool, error) {
	if m.UpdateFolderFunc != nil {
		return m.UpdateFolderFunc(id, name, parentID)
	}
	return model.Folder{}, false, nil
}

func (m *MockFeedRepository) DeleteFolder(id int64) (bool, error) {
	if m.DeleteFolderFunc != nil {
		return m.DeleteFolderFunc(id)
	}
	return false, nil
}

func (m *MockFeedRepository) AddInFolder(url, title string, items []repository.ArticleSeed, fetchErr string, folderID *int64, etag string, lastModified string) (model.Feed, error) {
	if m.AddInFolderFunc != nil {
		return m.AddInFolderFunc(url, title, items, fetchErr, folderID, etag, lastModified)
	}
	return model.Feed{}, nil
}

func (m *MockFeedRepository) UpdateFeedFolder(id int64, folderID *int64) (model.Feed, bool, error) {
	if m.UpdateFeedFolderFunc != nil {
		return m.UpdateFeedFolderFunc(id, folderID)
	}
	return model.Feed{}, false, nil
}

func (m *MockFeedRepository) UpdateFeedRetentionDays(id int64, retentionDays int) (model.Feed, bool, error) {
	if m.UpdateFeedRetentionDaysFunc != nil {
		return m.UpdateFeedRetentionDaysFunc(id, retentionDays)
	}
	return model.Feed{}, false, nil
}

func (m *MockFeedRepository) DeleteFeed(id int64) (bool, error) {
	if m.DeleteFeedFunc != nil {
		return m.DeleteFeedFunc(id)
	}
	return false, nil
}

func (m *MockFeedRepository) GetFeed(id int64) (model.Feed, bool, error) {
	if m.GetFeedFunc != nil {
		return m.GetFeedFunc(id)
	}
	return model.Feed{}, false, nil
}

func (m *MockFeedRepository) GetFeedByURL(rawURL string) (model.Feed, bool, error) {
	if m.GetFeedByURLFunc != nil {
		return m.GetFeedByURLFunc(rawURL)
	}
	return model.Feed{}, false, nil
}

func (m *MockFeedRepository) CreateFeedPlaceholder(url string, title string, folderID *int64) (model.Feed, error) {
	if m.CreateFeedPlaceholderFunc != nil {
		return m.CreateFeedPlaceholderFunc(url, title, folderID)
	}
	return model.Feed{}, nil
}

func (m *MockFeedRepository) UpdateFeedAfterRefresh(feedID int64, title string, items []repository.ArticleSeed, fetchErr, etag, lastModified string) error {
	if m.UpdateFeedAfterRefreshFunc != nil {
		return m.UpdateFeedAfterRefreshFunc(feedID, title, items, fetchErr, etag, lastModified)
	}
	return nil
}

func (m *MockFeedRepository) UpdateFeedScript(id int64, script string, lang string) (model.Feed, bool, error) {
	if m.UpdateFeedScriptFunc != nil {
		return m.UpdateFeedScriptFunc(id, script, lang)
	}
	return model.Feed{}, false, nil
}

func (m *MockFeedRepository) UpdateFeedTitle(id int64, title string) (model.Feed, bool, error) {
	if m.UpdateFeedTitleFunc != nil {
		return m.UpdateFeedTitleFunc(id, title)
	}
	return model.Feed{}, false, nil
}

func (m *MockFeedRepository) UpdateFeedIcon(id int64, iconPath string) (model.Feed, bool, error) {
	if m.UpdateFeedIconFunc != nil {
		return m.UpdateFeedIconFunc(id, iconPath)
	}
	return model.Feed{}, false, nil
}

func (m *MockFeedRepository) GetSetting(key string) (string, bool, error) {
	if m.GetSettingFunc != nil {
		return m.GetSettingFunc(key)
	}
	return "", false, nil
}

func (m *MockFeedRepository) SetSetting(key, value string) error {
	if m.SetSettingFunc != nil {
		return m.SetSettingFunc(key, value)
	}
	return nil
}

func (m *MockFeedRepository) ListArticles() []model.Article {
	if m.ListArticlesFunc != nil {
		return m.ListArticlesFunc()
	}
	return nil
}

func (m *MockFeedRepository) ListArticleListItems(query repository.ArticleListQuery) ([]model.Article, bool) {
	if m.ListArticleListItemsFunc != nil {
		return m.ListArticleListItemsFunc(query)
	}
	return nil, false
}

func (m *MockFeedRepository) ListArticlesNeedingScoreRefresh(featureVersion int, limit int) []model.Article {
	if m.ListArticlesNeedingScoreRefreshFunc != nil {
		return m.ListArticlesNeedingScoreRefreshFunc(featureVersion, limit)
	}
	return nil
}

func (m *MockFeedRepository) ListArticlesMissingDisplaySummary(feedID int64, limit int) []model.Article {
	if m.ListArticlesMissingDisplaySummaryFunc != nil {
		return m.ListArticlesMissingDisplaySummaryFunc(feedID, limit)
	}
	return nil
}

func (m *MockFeedRepository) DeleteArticle(id int64) (bool, error) {
	if m.DeleteArticleFunc != nil {
		return m.DeleteArticleFunc(id)
	}
	return false, nil
}

func (m *MockFeedRepository) GetArticle(id int64) (model.Article, bool) {
	if m.GetArticleFunc != nil {
		return m.GetArticleFunc(id)
	}
	return model.Article{}, false
}

func (m *MockFeedRepository) UpdateArticleFullContent(id int64, content string) error {
	if m.UpdateArticleFullContentFunc != nil {
		return m.UpdateArticleFullContentFunc(id, content)
	}
	return nil
}

func (m *MockFeedRepository) UpdateArticleSummaryState(id int64, aiSummary string, aiStatus string, displaySummary string, displayStatus string) error {
	if m.UpdateArticleSummaryStateFunc != nil {
		return m.UpdateArticleSummaryStateFunc(id, aiSummary, aiStatus, displaySummary, displayStatus)
	}
	return nil
}

func (m *MockFeedRepository) UpdateArticleDisplaySummary(id int64, summary string, status string) error {
	if m.UpdateArticleDisplaySummaryFunc != nil {
		return m.UpdateArticleDisplaySummaryFunc(id, summary, status)
	}
	return nil
}

func (m *MockFeedRepository) UpdateArticleScores(id int64, scores model.RecommendationScores) error {
	if m.UpdateArticleScoresFunc != nil {
		return m.UpdateArticleScoresFunc(id, scores)
	}
	return nil
}

func (m *MockFeedRepository) UpdateArticleFeatures(id int64, features model.ArticleFeatures) error {
	if m.UpdateArticleFeaturesFunc != nil {
		return m.UpdateArticleFeaturesFunc(id, features)
	}
	return nil
}

func (m *MockFeedRepository) MarkArticleRead(id int64, read bool) (model.Article, bool, error) {
	if m.MarkArticleReadFunc != nil {
		return m.MarkArticleReadFunc(id, read)
	}
	return model.Article{}, false, nil
}

func (m *MockFeedRepository) MarkArticleFavorite(id int64, favorite bool) (model.Article, bool, error) {
	if m.MarkArticleFavoriteFunc != nil {
		return m.MarkArticleFavoriteFunc(id, favorite)
	}
	return model.Article{}, false, nil
}

func (m *MockFeedRepository) PurgeExpiredArticles(retentionDays int) (int, error) {
	if m.PurgeExpiredArticlesFunc != nil {
		return m.PurgeExpiredArticlesFunc(retentionDays)
	}
	return 0, nil
}
