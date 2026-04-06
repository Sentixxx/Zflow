package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

const (
	defaultEmbeddingModel = "text-embedding-3-small"
	maxEmbeddingBatch     = 20
	maxEmbeddingChars     = 8000
)

type EmbeddingConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

type EmbeddingService struct {
	vectorRepo repository.VectorRepository
	feedRepo   repository.FeedRepository
	httpClient func() *http.Client
	loadConfig func() (EmbeddingConfig, error)
	logger     *logger.ModuleLogger
}

func NewEmbeddingService(
	vectorRepo repository.VectorRepository,
	feedRepo repository.FeedRepository,
	httpClient func() *http.Client,
	loadConfig func() (EmbeddingConfig, error),
) *EmbeddingService {
	return &EmbeddingService{
		vectorRepo: vectorRepo,
		feedRepo:   feedRepo,
		httpClient: httpClient,
		loadConfig: loadConfig,
		logger:     logger.NewModuleFromEnv("embedding"),
	}
}

func (s *EmbeddingService) VectorRepo() repository.VectorRepository {
	return s.vectorRepo
}

func (s *EmbeddingService) EmbedPendingArticles(ctx context.Context, batchSize int) (int, error) {
	if batchSize <= 0 {
		batchSize = maxEmbeddingBatch
	}

	cfg, err := s.loadConfig()
	if err != nil {
		return 0, fmt.Errorf("load embedding config: %w", err)
	}
	if cfg.APIKey == "" {
		return 0, nil
	}

	articles := s.feedRepo.ListArticlesNeedingScoreRefresh(0, 0)
	if len(articles) == 0 {
		articles = s.feedRepo.ListArticles()
	}

	var pending []model.Article
	for _, a := range articles {
		if len(pending) >= batchSize {
			break
		}
		has, err := s.vectorRepo.HasEmbedding(ctx, "article", a.ID)
		if err != nil {
			s.logger.Error("embed", "article", "failed", "check embedding exists", "article_id", fmt.Sprint(a.ID), "error", err.Error())
			continue
		}
		if !has {
			pending = append(pending, a)
		}
	}

	if len(pending) == 0 {
		return 0, nil
	}

	embedded := 0
	for _, a := range pending {
		text := buildEmbeddingText(a)
		if text == "" {
			continue
		}

		vec, err := s.callEmbeddingAPI(ctx, cfg, text)
		if err != nil {
			s.logger.Error("embed", "article", "failed", "call embedding API", "article_id", fmt.Sprint(a.ID), "error", err.Error())
			continue
		}

		modelName := cfg.Model
		if modelName == "" {
			modelName = defaultEmbeddingModel
		}
		if err := s.vectorRepo.StoreEmbedding(ctx, "article", a.ID, modelName, vec); err != nil {
			s.logger.Error("embed", "article", "failed", "store embedding", "article_id", fmt.Sprint(a.ID), "error", err.Error())
			continue
		}
		embedded++
	}

	if embedded > 0 {
		s.logger.Info("embed", "article", "ok", "embedded articles", "count", fmt.Sprint(embedded))
	}
	return embedded, nil
}

func (s *EmbeddingService) EmbedSingleArticle(ctx context.Context, articleID int64) error {
	cfg, err := s.loadConfig()
	if err != nil {
		return fmt.Errorf("load embedding config: %w", err)
	}
	if cfg.APIKey == "" {
		return nil
	}

	article, ok := s.feedRepo.GetArticle(articleID)
	if !ok {
		return fmt.Errorf("article %d not found", articleID)
	}

	text := buildEmbeddingText(article)
	if text == "" {
		return nil
	}

	vec, err := s.callEmbeddingAPI(ctx, cfg, text)
	if err != nil {
		return fmt.Errorf("call embedding API: %w", err)
	}

	modelName := cfg.Model
	if modelName == "" {
		modelName = defaultEmbeddingModel
	}
	return s.vectorRepo.StoreEmbedding(ctx, "article", articleID, modelName, vec)
}

func (s *EmbeddingService) SearchSimilarArticles(ctx context.Context, query string, limit int) ([]repository.VectorMatch, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load embedding config: %w", err)
	}
	if cfg.APIKey == "" {
		return nil, nil
	}

	vec, err := s.callEmbeddingAPI(ctx, cfg, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	return s.vectorRepo.SearchSimilar(ctx, "article", vec, limit)
}

func buildEmbeddingText(a model.Article) string {
	var parts []string
	if t := strings.TrimSpace(a.Title); t != "" {
		parts = append(parts, t)
	}
	if s := strings.TrimSpace(a.Summary); s != "" {
		parts = append(parts, s)
	}
	if c := strings.TrimSpace(a.FullContent); c != "" {
		parts = append(parts, c)
	}
	text := strings.Join(parts, "\n\n")
	if len(text) > maxEmbeddingChars {
		text = text[:maxEmbeddingChars]
	}
	return text
}

type embeddingAPIRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingAPIResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *EmbeddingService) callEmbeddingAPI(ctx context.Context, cfg EmbeddingConfig, text string) ([]float32, error) {
	modelName := cfg.Model
	if modelName == "" {
		modelName = defaultEmbeddingModel
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	reqBody := embeddingAPIRequest{
		Model: modelName,
		Input: text,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp embeddingAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("embedding API error: %s", apiResp.Error.Message)
	}
	if len(apiResp.Data) == 0 || len(apiResp.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding API returned empty data")
	}

	return apiResp.Data[0].Embedding, nil
}
