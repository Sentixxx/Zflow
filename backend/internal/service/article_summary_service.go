package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
)

const (
	DisplaySummaryReady    = "ready"
	DisplaySummaryFallback = "fallback"
	DisplaySummaryFailed   = "failed"
)

type SummaryAIConfig struct {
	Protocol string
	APIKey  string
	BaseURL string
	Model   string
}

type ArticleSummaryService struct {
	store          repository.FeedRepository
	httpClient     func() *http.Client
	loadAIConfig   func() (SummaryAIConfig, error)
	defaultBaseURL string
	defaultModel   string
}

func NewArticleSummaryService(
	store repository.FeedRepository,
	httpClient func() *http.Client,
	loadAIConfig func() (SummaryAIConfig, error),
	defaultBaseURL string,
	defaultModel string,
) *ArticleSummaryService {
	return &ArticleSummaryService{
		store:          store,
		httpClient:     httpClient,
		loadAIConfig:   loadAIConfig,
		defaultBaseURL: strings.TrimSpace(defaultBaseURL),
		defaultModel:   strings.TrimSpace(defaultModel),
	}
}

func (s *ArticleSummaryService) BackfillFeed(feedID int64, limit int) error {
	articles := s.store.ListArticlesMissingDisplaySummary(feedID, limit)
	for _, article := range articles {
		summary, status := s.buildDisplaySummary(context.Background(), article)
		if err := s.store.UpdateArticleDisplaySummary(article.ID, summary, status); err != nil {
			return fmt.Errorf("update article %d display summary: %w", article.ID, err)
		}
	}
	return nil
}

func (s *ArticleSummaryService) RefreshArticle(articleID int64) error {
	article, ok := s.store.GetArticle(articleID)
	if !ok {
		return ErrArticleNotFound
	}
	summary, status := s.buildDisplaySummary(context.Background(), article)
	if err := s.store.UpdateArticleDisplaySummary(article.ID, summary, status); err != nil {
		return fmt.Errorf("update article %d display summary: %w", article.ID, err)
	}
	return nil
}

func (s *ArticleSummaryService) buildDisplaySummary(ctx context.Context, article model.Article) (string, string) {
	localSummary, localStatus := BuildDisplaySummary(article)
	if strings.TrimSpace(localSummary) == "" {
		return "", DisplaySummaryFailed
	}
	if s.loadAIConfig == nil || s.httpClient == nil {
		return localSummary, localStatus
	}
	cfg, err := s.loadAIConfig()
	if err != nil || strings.TrimSpace(cfg.APIKey) == "" {
		return localSummary, localStatus
	}
	aiSummary, err := s.generateSummaryWithAI(ctx, article, cfg)
	if err != nil {
		return localSummary, localStatus
	}
	if strings.TrimSpace(aiSummary) == "" {
		return localSummary, localStatus
	}
	return wrapSummaryHTML([]string{aiSummary}), DisplaySummaryReady
}

func BuildDisplaySummary(article model.Article) (string, string) {
	primaryBlocks := normalizeSummaryBlocks(article.FullContent, 420, 3)
	if len(primaryBlocks) == 0 {
		primaryBlocks = normalizeSummaryBlocks(article.Summary, 320, 2)
	}
	if len(primaryBlocks) == 0 {
		primaryBlocks = normalizeSummaryBlocks(article.Title, 160, 1)
	}
	if len(primaryBlocks) == 0 {
		return "", DisplaySummaryFailed
	}

	return wrapSummaryHTML(primaryBlocks), DisplaySummaryFallback
}

func normalizeSummaryBlocks(raw string, maxChars int, maxBlocks int) []string {
	text := strings.TrimSpace(raw)
	if text == "" || maxChars <= 0 || maxBlocks <= 0 {
		return nil
	}

	if strings.Contains(text, "<") && strings.Contains(text, ">") {
		blockBreaks := regexp.MustCompile(`(?is)</(p|div|li|h[1-6]|blockquote|section|article|pre|tr|ul|ol)>|<br\s*/?>`)
		text = blockBreaks.ReplaceAllString(text, "\n\n")
		tags := regexp.MustCompile(`(?s)<[^>]*>`)
		text = tags.ReplaceAllString(text, " ")
		text = html.UnescapeString(text)
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	rawBlocks := regexp.MustCompile(`\n\s*\n+`).Split(text, -1)
	blocks := make([]string, 0, maxBlocks)
	usedChars := 0
	for _, block := range rawBlocks {
		normalized := strings.Join(strings.Fields(strings.TrimSpace(block)), " ")
		if normalized == "" {
			continue
		}
		if isBoilerplateSummaryBlock(normalized) {
			continue
		}
		if usedChars >= maxChars {
			break
		}
		remaining := maxChars - usedChars
		if remaining <= 0 {
			break
		}
		if len([]rune(normalized)) > remaining {
			normalized = truncateRunes(normalized, remaining)
		}
		blocks = append(blocks, normalized)
		usedChars += len([]rune(normalized))
		if len(blocks) >= maxBlocks {
			break
		}
	}
	return blocks
}

func isBoilerplateSummaryBlock(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return true
	}
	boilerplates := []string{
		"read more",
		"阅读全文",
		"继续阅读",
		"点击查看全文",
		"the post",
		"appeared first on",
	}
	for _, item := range boilerplates {
		if normalized == item || strings.HasPrefix(normalized, item+" ") {
			return true
		}
	}
	return false
}

func truncateRunes(text string, max int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= max {
		return string(runes)
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return strings.TrimSpace(string(runes[:max-1])) + "…"
}

func wrapSummaryHTML(blocks []string) string {
	if len(blocks) == 0 {
		return ""
	}
	var builder strings.Builder
	for _, block := range blocks {
		builder.WriteString("<p>")
		builder.WriteString(html.EscapeString(block))
		builder.WriteString("</p>")
	}
	return builder.String()
}

func (s *ArticleSummaryService) generateSummaryWithAI(ctx context.Context, article model.Article, cfg SummaryAIConfig) (string, error) {
	switch normalizeAIProtocol(cfg.Protocol) {
	case "anthropic":
		return s.generateSummaryWithAnthropic(ctx, article, cfg)
	default:
		return s.generateSummaryWithOpenAI(ctx, article, cfg)
	}
}

func (s *ArticleSummaryService) generateSummaryWithOpenAI(ctx context.Context, article model.Article, cfg SummaryAIConfig) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(firstNonEmpty(cfg.BaseURL, s.defaultBaseURL)), "/")
	model := strings.TrimSpace(firstNonEmpty(cfg.Model, s.defaultModel))
	if baseURL == "" || model == "" {
		return "", errors.New("ai summary config is incomplete")
	}

	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	reqPayload := map[string]any{
		"model": model,
		"messages": []chatMessage{
			{
				Role:    "system",
				Content: "You write concise article summaries for readers. Return only the final summary in Chinese, 2 to 4 sentences, no markdown, no bullet points, no preamble.",
			},
			{
				Role:    "user",
				Content: buildSummaryPrompt(article),
			},
		},
		"temperature": 0.7,
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("summary upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("empty summary response")
	}
	result := strings.TrimSpace(stripThinkTags(out.Choices[0].Message.Content))
	if result == "" {
		return "", errors.New("empty summary content")
	}
	return truncateRunes(strings.Join(strings.Fields(result), " "), 220), nil
}

func (s *ArticleSummaryService) generateSummaryWithAnthropic(ctx context.Context, article model.Article, cfg SummaryAIConfig) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(firstNonEmpty(cfg.BaseURL, "https://api.minimaxi.com/anthropic")), "/")
	model := strings.TrimSpace(firstNonEmpty(cfg.Model, s.defaultModel))
	if baseURL == "" || model == "" {
		return "", errors.New("ai summary config is incomplete")
	}

	reqPayload := map[string]any{
		"model":      model,
		"max_tokens": 300,
		"system":     "You write concise article summaries for readers. Return only the final summary in Chinese, 2 to 4 sentences, no markdown, no bullet points, no preamble.",
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]string{
					{
						"type": "text",
						"text": buildSummaryPrompt(article),
					},
				},
			},
		},
		"temperature": 0.7,
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", strings.TrimSpace(cfg.APIKey))
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("summary upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return "", err
	}
	for _, block := range out.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return truncateRunes(strings.Join(strings.Fields(block.Text), " "), 220), nil
		}
	}
	return "", errors.New("empty summary content")
}

func buildSummaryPrompt(article model.Article) string {
	var builder strings.Builder
	builder.WriteString("请为读者生成一段中文文章摘要，要求：\n")
	builder.WriteString("1. 2到4句话。\n")
	builder.WriteString("2. 直接说明文章讲了什么、为什么值得读。\n")
	builder.WriteString("3. 不要出现“这篇文章”之类空话，不要重复标题，不要输出项目符号。\n\n")
	builder.WriteString("标题：\n")
	builder.WriteString(strings.TrimSpace(article.Title))
	builder.WriteString("\n\n原始摘要：\n")
	builder.WriteString(strings.TrimSpace(strings.Join(normalizeSummaryBlocks(article.Summary, 500, 3), "\n\n")))
	builder.WriteString("\n\n正文片段：\n")
	builder.WriteString(strings.TrimSpace(strings.Join(normalizeSummaryBlocks(article.FullContent, 1400, 5), "\n\n")))
	return strings.TrimSpace(builder.String())
}

func stripThinkTags(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	thinkBlock := regexp.MustCompile(`(?is)<think>.*?</think>`)
	return strings.TrimSpace(thinkBlock.ReplaceAllString(trimmed, ""))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeAIProtocol(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "anthropic":
		return "anthropic"
	default:
		return "openai"
	}
}
