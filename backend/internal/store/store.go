package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/domain"
)

var ErrFeedExists = errors.New("feed already exists")

type fileData struct {
	NextFeedID    int64            `json:"next_feed_id"`
	NextArticleID int64            `json:"next_article_id"`
	NextID        int64            `json:"next_id,omitempty"`
	Feeds         []domain.Feed    `json:"feeds"`
	Articles      []domain.Article `json:"articles"`
}

type FeedStore struct {
	mu            sync.Mutex
	path          string
	nextFeedID    int64
	nextArticleID int64
	feeds         []domain.Feed
	articles      []domain.Article
}

func NewFeedStore(path string) (*FeedStore, error) {
	s := &FeedStore{path: path, nextFeedID: 1, nextArticleID: 1}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

type ArticleSeed struct {
	Title       string
	Link        string
	Summary     string
	PublishedAt string
}

func (s *FeedStore) List() []domain.Feed {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]domain.Feed, len(s.feeds))
	copy(out, s.feeds)
	return out
}

func (s *FeedStore) Add(url, title string, items []ArticleSeed, fetchErr string) (domain.Feed, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, f := range s.feeds {
		if f.URL == url {
			return domain.Feed{}, ErrFeedExists
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	status := "ok"
	if fetchErr != "" {
		status = "error"
	}

	feed := domain.Feed{
		ID:              s.nextFeedID,
		URL:             url,
		Title:           title,
		ItemCount:       0,
		LastFetchedAt:   now,
		LastFetchStatus: status,
		LastFetchError:  fetchErr,
		CreatedAt:       now,
	}
	s.nextFeedID++
	s.feeds = append(s.feeds, feed)
	existingKeys := s.buildExistingDedupKeySet()
	insertedCount := 0
	for _, item := range items {
		cleaned := cleanSeed(item)
		if cleaned.Title == "" && cleaned.Link == "" {
			continue
		}
		key := dedupKey(cleaned)
		if _, exists := existingKeys[key]; exists {
			continue
		}
		existingKeys[key] = struct{}{}

		article := domain.Article{
			ID:          s.nextArticleID,
			FeedID:      feed.ID,
			Title:       cleaned.Title,
			Link:        cleaned.Link,
			Summary:     cleaned.Summary,
			PublishedAt: cleaned.PublishedAt,
			IsRead:      false,
			CreatedAt:   now,
		}
		s.nextArticleID++
		s.articles = append(s.articles, article)
		insertedCount++
	}

	for i := range s.feeds {
		if s.feeds[i].ID == feed.ID {
			s.feeds[i].ItemCount = insertedCount
			feed.ItemCount = insertedCount
			break
		}
	}

	if err := s.save(); err != nil {
		return domain.Feed{}, err
	}
	return feed, nil
}

func (s *FeedStore) ListArticles() []domain.Article {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]domain.Article, len(s.articles))
	copy(out, s.articles)
	return out
}

func (s *FeedStore) GetArticle(id int64) (domain.Article, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, article := range s.articles {
		if article.ID == id {
			return article, true
		}
	}
	return domain.Article{}, false
}

func (s *FeedStore) MarkArticleRead(id int64, read bool) (domain.Article, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.articles {
		if s.articles[i].ID != id {
			continue
		}
		s.articles[i].IsRead = read
		if err := s.save(); err != nil {
			return domain.Article{}, true, err
		}
		return s.articles[i], true, nil
	}
	return domain.Article{}, false, nil
}

func (s *FeedStore) load() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	if data.NextFeedID > 0 {
		s.nextFeedID = data.NextFeedID
	} else if data.NextID > 0 {
		s.nextFeedID = data.NextID
	}
	if data.NextArticleID > 0 {
		s.nextArticleID = data.NextArticleID
	}
	s.feeds = data.Feeds
	s.articles = data.Articles
	return nil
}

func (s *FeedStore) save() error {
	data := fileData{
		NextFeedID:    s.nextFeedID,
		NextArticleID: s.nextArticleID,
		Feeds:         s.feeds,
		Articles:      s.articles,
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, raw, 0o644)
}

func (s *FeedStore) buildExistingDedupKeySet() map[string]struct{} {
	keys := make(map[string]struct{}, len(s.articles))
	for _, article := range s.articles {
		key := dedupKey(ArticleSeed{
			Title:   article.Title,
			Link:    article.Link,
			Summary: article.Summary,
		})
		keys[key] = struct{}{}
	}
	return keys
}

func cleanSeed(seed ArticleSeed) ArticleSeed {
	seed.Title = strings.TrimSpace(seed.Title)
	seed.Link = strings.TrimSpace(seed.Link)
	seed.Summary = strings.TrimSpace(seed.Summary)
	seed.PublishedAt = strings.TrimSpace(seed.PublishedAt)
	return seed
}

func dedupKey(seed ArticleSeed) string {
	normalizedLink := normalizeForKey(seed.Link)
	if normalizedLink != "" {
		return hashText("link:" + normalizedLink)
	}

	normalizedTitle := normalizeForKey(seed.Title)
	normalizedSummary := normalizeForKey(seed.Summary)
	return hashText("text:" + normalizedTitle + "|" + normalizedSummary)
}

func normalizeForKey(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return ""
	}
	return strings.Join(strings.Fields(v), " ")
}

func hashText(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}
