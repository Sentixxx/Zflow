package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	logpkg "github.com/Sentixxx/Zflow/backend/pkg/logger"
)

// maxScoreReasoningLen is the hard upper bound (in Unicode code points) for persisted
// LLM reasoning text. LLM outputs are unbounded in principle; without a cap, a
// misbehaving model could write many kilobytes into TEXT columns and then have those
// bytes echoed back verbatim by every endpoint that returns a full Article.
const maxScoreReasoningLen = 1024

// ScoringAIConfig holds the AI provider configuration for LLM-enhanced scoring.
type ScoringAIConfig struct {
	Protocol string
	APIKey   string
	BaseURL  string
	Model    string
}

// llmScoringResult is the structured output expected from the LLM.
type llmScoringResult struct {
	Quality   int    `json:"quality"`
	Depth     int    `json:"depth"`
	Relevance int    `json:"relevance"`
	Reasoning string `json:"reasoning"`
}

const llmScoringSystemPrompt = `You are an article quality evaluator for a personal RSS reader. Evaluate the given article and return a JSON object with three integer scores (0-100) and a brief reasoning.

Scoring dimensions:
- quality (0-100): Content quality. High: well-structured, original insights, clear writing, backed by evidence. Low: SEO spam, clickbait, thin content, template boilerplate, listicles without substance.
- depth (0-100): Information density and analytical depth. High: thorough analysis, multiple perspectives, technical detail, novel arguments. Low: surface-level summary, rehashed common knowledge, padding.
- relevance (0-100): How informative and useful the article is for a technically-minded reader. High: actionable knowledge, important developments, in-depth tutorials, significant research. Low: promotional content, event announcements, trivial updates.

Rules:
- Return ONLY a valid JSON object, no markdown fences, no extra text.
- Be strict: most articles should score 40-70. Reserve 80+ for genuinely excellent content.
- Do not inflate scores. A mediocre article with good formatting is still mediocre.
- reasoning should be 1-2 sentences explaining the key factors.

Output format:
{"quality": <int>, "depth": <int>, "relevance": <int>, "reasoning": "<string>"}`

func buildLLMScoringUserPrompt(article model.Article) string {
	var b strings.Builder
	b.WriteString("Title: ")
	b.WriteString(strings.TrimSpace(article.Title))
	b.WriteByte('\n')

	summary := strings.TrimSpace(article.Summary)
	if summary != "" {
		b.WriteString("Summary: ")
		if len(summary) > 500 {
			summary = summary[:500]
		}
		b.WriteString(summary)
		b.WriteByte('\n')
	}

	body := strings.TrimSpace(article.FullContent)
	if body == "" {
		body = summary
	}
	if body != "" {
		visible := normalizedVisibleText(body)
		if len(visible) > 3000 {
			visible = visible[:3000]
		}
		b.WriteString("Content:\n")
		b.WriteString(visible)
	}
	return b.String()
}

func callLLMForScoring(ctx context.Context, httpClient *http.Client, cfg ScoringAIConfig, article model.Article, logger *logpkg.ModuleLogger) (llmScoringResult, error) {
	userPrompt := buildLLMScoringUserPrompt(article)
	if strings.TrimSpace(userPrompt) == "" {
		return llmScoringResult{}, fmt.Errorf("empty scoring input for article %d", article.ID)
	}

	switch cfg.Protocol {
	case "anthropic":
		return callLLMScoringAnthropic(ctx, httpClient, cfg, userPrompt)
	default:
		return callLLMScoringOpenAI(ctx, httpClient, cfg, userPrompt)
	}
}

func callLLMScoringOpenAI(ctx context.Context, httpClient *http.Client, cfg ScoringAIConfig, userPrompt string) (llmScoringResult, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	type chatMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := map[string]any{
		"model": cfg.Model,
		"messages": []chatMsg{
			{Role: "system", Content: llmScoringSystemPrompt},
			{Role: "user", Content: userPrompt},
		},
		"temperature": 0.3,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return llmScoringResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return llmScoringResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))

	resp, err := httpClient.Do(req)
	if err != nil {
		return llmScoringResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return llmScoringResult{}, fmt.Errorf("scoring upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return llmScoringResult{}, err
	}
	if len(out.Choices) == 0 {
		return llmScoringResult{}, fmt.Errorf("empty scoring response")
	}
	return parseLLMScoringResponse(out.Choices[0].Message.Content)
}

func callLLMScoringAnthropic(ctx context.Context, httpClient *http.Client, cfg ScoringAIConfig, userPrompt string) (llmScoringResult, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}

	payload := map[string]any{
		"model":      cfg.Model,
		"max_tokens": 300,
		"system":     llmScoringSystemPrompt,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]string{
					{"type": "text", "text": userPrompt},
				},
			},
		},
		"temperature": 0.3,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return llmScoringResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return llmScoringResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", strings.TrimSpace(cfg.APIKey))
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := httpClient.Do(req)
	if err != nil {
		return llmScoringResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return llmScoringResult{}, fmt.Errorf("scoring upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return llmScoringResult{}, err
	}
	for _, block := range out.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			return parseLLMScoringResponse(block.Text)
		}
	}
	return llmScoringResult{}, fmt.Errorf("empty scoring content")
}

func parseLLMScoringResponse(raw string) (llmScoringResult, error) {
	text := strings.TrimSpace(raw)
	// Strip markdown fences if present
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	// Strip <think> tags if present
	if idx := strings.Index(text, "</think>"); idx >= 0 {
		text = strings.TrimSpace(text[idx+len("</think>"):])
	}

	var result llmScoringResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return llmScoringResult{}, fmt.Errorf("parse scoring JSON: %w (raw: %.200s)", err, text)
	}
	result.Quality = clampScore(result.Quality)
	result.Depth = clampScore(result.Depth)
	result.Relevance = clampScore(result.Relevance)
	return result, nil
}

// blendLLMScores merges rule-based features with LLM scores.
// The blend ratio is 40% rule-based, 60% LLM for the three dimensions the LLM evaluates.
// Freshness and novelty remain purely rule-based.
// LLM reasoning is persisted with a hard cap of maxScoreReasoningLen runes to prevent
// unbounded LLM output from inflating stored TEXT size and being echoed back by every
// endpoint that returns a full Article object.
func blendLLMScores(features model.ArticleFeatures, llm llmScoringResult) model.ArticleFeatures {
	features.Quality = blendScore(features.Quality, llm.Quality, 0.4, 0.6)
	features.Depth = blendScore(features.Depth, llm.Depth, 0.4, 0.6)
	features.Relevance = blendScore(features.Relevance, llm.Relevance, 0.4, 0.6)
	reasoning := strings.TrimSpace(llm.Reasoning)
	if utf8.RuneCountInString(reasoning) > maxScoreReasoningLen {
		// Truncate at a rune boundary to avoid splitting multibyte characters, then
		// append an ellipsis so callers can detect truncation if needed.
		runes := []rune(reasoning)
		reasoning = string(runes[:maxScoreReasoningLen]) + "…"
	}
	features.Reasoning = reasoning
	return features
}

func blendScore(rule, llm int, ruleWeight, llmWeight float64) int {
	return clampScore(int(float64(rule)*ruleWeight + float64(llm)*llmWeight + 0.5))
}
