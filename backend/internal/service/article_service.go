package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	readability "github.com/go-shiori/go-readability"
)

var (
	ErrArticleNotFound      = errors.New("article not found")
	ErrArticleLinkEmpty     = errors.New("article link is empty")
	ErrReadabilityFetchFail = errors.New("readability fetch failed")
	ErrSaveArticleContent   = errors.New("failed to save article content")
)

type ArticleService struct {
	store      repository.FeedRepository
	httpClient func() *http.Client
}

func NewArticleService(store repository.FeedRepository, httpClient func() *http.Client) *ArticleService {
	return &ArticleService{
		store:      store,
		httpClient: httpClient,
	}
}

func (u *ArticleService) List(page int, limit int) ([]model.Article, bool) {
	all := u.store.ListArticles()
	if limit <= 0 {
		return all, false
	}
	if page < 1 {
		page = 1
	}

	start := (page - 1) * limit
	if start >= len(all) {
		return []model.Article{}, false
	}
	endExclusive := start + limit + 1
	if endExclusive > len(all) {
		endExclusive = len(all)
	}
	window := all[start:endExclusive]
	hasMore := len(window) > limit
	if hasMore {
		window = window[:limit]
	}
	return window, hasMore
}

func (u *ArticleService) Get(id int64) (model.Article, bool) {
	return u.store.GetArticle(id)
}

func (u *ArticleService) Delete(id int64) (bool, error) {
	return u.store.DeleteArticle(id)
}

func (u *ArticleService) MarkRead(id int64, read bool) (model.Article, bool, error) {
	return u.store.MarkArticleRead(id, read)
}

func (u *ArticleService) ExtractReadable(ctx context.Context, articleID int64) (model.Article, error) {
	article, ok := u.store.GetArticle(articleID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	if strings.TrimSpace(article.Link) == "" {
		return model.Article{}, ErrArticleLinkEmpty
	}
	content, err := u.fetchReadableContent(ctx, article.Link)
	if err != nil {
		return model.Article{}, fmt.Errorf("%w: %v", ErrReadabilityFetchFail, err)
	}
	if err := u.store.UpdateArticleFullContent(articleID, content); err != nil {
		return model.Article{}, fmt.Errorf("%w: %v", ErrSaveArticleContent, err)
	}
	updated, ok := u.store.GetArticle(articleID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	return updated, nil
}

func (u *ArticleService) RefreshCache(ctx context.Context, articleID int64) (model.Article, error) {
	article, ok := u.store.GetArticle(articleID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	if strings.TrimSpace(article.Link) != "" {
		content, err := u.fetchReadableContent(ctx, article.Link)
		if err != nil {
			return model.Article{}, fmt.Errorf("%w: %v", ErrReadabilityFetchFail, err)
		}
		if err := u.store.UpdateArticleFullContent(articleID, content); err != nil {
			return model.Article{}, fmt.Errorf("%w: %v", ErrSaveArticleContent, err)
		}
	}
	updated, ok := u.store.GetArticle(articleID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	return updated, nil
}

func (u *ArticleService) fetchReadableContent(ctx context.Context, rawURL string) (string, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Zflow/0.1 (+https://github.com/Sentixxx/Zflow)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	resp, err := u.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	rawBody, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", err
	}
	contentType := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if strings.Contains(contentType, "application/pdf") || bytes.HasPrefix(rawBody, []byte("%PDF-")) {
		return "", errors.New("unsupported readability content type: pdf")
	}
	doc, err := readability.FromReader(bytes.NewReader(rawBody), parsedURL)
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(doc.Content)
	if content == "" {
		content = strings.TrimSpace(doc.TextContent)
	}
	if content == "" {
		return "", errors.New("empty readable content")
	}
	return content, nil
}
