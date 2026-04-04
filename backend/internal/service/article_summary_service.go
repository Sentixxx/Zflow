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
	"sort"
	"strings"
	"unicode"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
	logpkg "github.com/Sentixxx/Zflow/backend/pkg/logger"
)

const (
	DisplaySummaryReady    = "ready"
	DisplaySummaryFallback = "fallback"
	DisplaySummaryFailed   = "failed"
	AISummaryReady         = "ready"
	AISummaryFailed        = "failed"
	AISummaryCleared       = "cleared"
	aiSummaryChunkLimit    = 1200
	aiSummaryChunkMaxCount = 6
	aiSummarySourceBudget  = aiSummaryChunkLimit * aiSummaryChunkMaxCount
	aiSummaryWindowBands   = 3
)

var (
	summaryMarkdownImagePattern     = regexp.MustCompile(`(?is)!\[[^\]]*\]\([^)]+\)`)
	summaryHTMLFigurePattern        = regexp.MustCompile(`(?is)<figure\b[^>]*>.*?</figure>`)
	summaryHTMLImagePattern         = regexp.MustCompile(`(?is)<img\b[^>]*>`)
	summaryHTMLBlockBreakPattern    = regexp.MustCompile(`(?is)</(p|div|li|h[1-6]|blockquote|section|article|pre|tr|ul|ol)>|<br\s*/?>`)
	summaryHTMLTagPattern           = regexp.MustCompile(`(?s)<[^>]*>`)
	summaryParagraphCollapsePattern = regexp.MustCompile(`\n{3,}`)
	summaryParagraphSplitPattern    = regexp.MustCompile(`\n\s*\n+`)
)

type SummaryAIConfig struct {
	Protocol string
	APIKey   string
	BaseURL  string
	Model    string
}

type summaryBuildResult struct {
	AISummary       string
	AISummaryStatus string
	DisplaySummary  string
	DisplayStatus   string
	Debug           model.SummaryDebug
}

type summaryChunkPlan struct {
	Chunks      []string
	Strategy    string
	WindowCount int
}

type summaryPromptSpec struct {
	SystemPrompt string
	UserPrompt   string
	QueryMode    string
	AllowRewrite bool
}

type summaryPromptResult struct {
	Text          string
	Closed        bool
	RewritePassed bool
}

type ArticleSummaryService struct {
	store          repository.FeedRepository
	httpClient     func() *http.Client
	loadAIConfig   func() (SummaryAIConfig, error)
	defaultBaseURL string
	defaultModel   string
	logger         *logpkg.ModuleLogger
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
		logger:         logpkg.NewModuleFromEnv("service"),
	}
}

func (s *ArticleSummaryService) BackfillFeed(feedID int64, limit int) error {
	articles := s.store.ListArticlesMissingDisplaySummary(feedID, limit)
	for _, article := range articles {
		if err := s.refreshSummaryState(context.Background(), article); err != nil {
			return fmt.Errorf("refresh article %d summary state: %w", article.ID, err)
		}
	}
	return nil
}

func (s *ArticleSummaryService) RefreshArticle(articleID int64) error {
	article, ok := s.store.GetArticle(articleID)
	if !ok {
		return ErrArticleNotFound
	}
	return s.refreshSummaryState(context.Background(), article)
}

func (s *ArticleSummaryService) RefreshArticleWithDebug(articleID int64) (model.Article, error) {
	article, ok := s.store.GetArticle(articleID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	result := s.buildSummaryStateResult(context.Background(), article)
	if err := s.store.UpdateArticleSummaryState(article.ID, result.AISummary, result.AISummaryStatus, result.DisplaySummary, result.DisplayStatus); err != nil {
		return model.Article{}, fmt.Errorf("update article %d summary state: %w", article.ID, err)
	}
	updated, ok := s.store.GetArticle(article.ID)
	if !ok {
		return model.Article{}, ErrArticleNotFound
	}
	updated.SummaryDebug = &result.Debug
	return updated, nil
}

func (s *ArticleSummaryService) RefreshRecentArticles(limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	articles := s.store.ListArticles()
	if len(articles) == 0 {
		return 0, nil
	}
	if len(articles) > limit {
		articles = articles[:limit]
	}
	s.logger.Info("refresh", "summary", "started", "summary regeneration started", "article_count", len(articles), "limit", limit)
	refreshed := 0
	for _, article := range articles {
		if err := s.clearArticleAISummaryState(article); err != nil {
			s.logger.Error("reset", "summary", "failed", "clear ai summary before regeneration failed", "article_id", article.ID, "error", err.Error())
			return refreshed, fmt.Errorf("clear article %d ai summary: %w", article.ID, err)
		}
		s.logger.Info("reset", "summary", "ok", "cleared ai summary before regeneration", "article_id", article.ID)

		if err := s.refreshSummaryState(context.Background(), article); err != nil {
			s.logger.Error("refresh", "summary", "failed", "regenerate summary state failed", "article_id", article.ID, "error", err.Error())
			return refreshed, fmt.Errorf("refresh article %d summary state: %w", article.ID, err)
		}
		s.logger.Info("refresh", "summary", "ok", "regenerated summary state", "article_id", article.ID)
		refreshed++
	}
	s.logger.Info("refresh", "summary", "ok", "summary regeneration completed", "refreshed", refreshed)
	return refreshed, nil
}

func (s *ArticleSummaryService) ClearArticleAI(articleID int64) error {
	article, ok := s.store.GetArticle(articleID)
	if !ok {
		return ErrArticleNotFound
	}
	return s.clearArticleAISummaryState(article)
}

func (s *ArticleSummaryService) ClearRecentArticleAI(limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	articles := s.store.ListArticles()
	if len(articles) == 0 {
		return 0, nil
	}
	if len(articles) > limit {
		articles = articles[:limit]
	}
	s.logger.Info("delete", "summary", "started", "clear ai summaries started", "article_count", len(articles), "limit", limit)
	cleared := 0
	for _, article := range articles {
		if err := s.clearArticleAISummaryState(article); err != nil {
			s.logger.Error("delete", "summary", "failed", "clear ai summary failed", "article_id", article.ID, "error", err.Error())
			return cleared, err
		}
		cleared++
	}
	s.logger.Info("delete", "summary", "ok", "clear ai summaries completed", "cleared", cleared)
	return cleared, nil
}

func (s *ArticleSummaryService) refreshSummaryState(ctx context.Context, article model.Article) error {
	result := s.buildSummaryStateResult(ctx, article)
	if err := s.store.UpdateArticleSummaryState(article.ID, result.AISummary, result.AISummaryStatus, result.DisplaySummary, result.DisplayStatus); err != nil {
		return fmt.Errorf("update article %d summary state: %w", article.ID, err)
	}
	return nil
}

func (s *ArticleSummaryService) clearArticleAISummaryState(article model.Article) error {
	displaySummary, displayStatus := buildFallbackDisplaySummary(article, false)
	return s.store.UpdateArticleSummaryState(article.ID, "", AISummaryCleared, displaySummary, displayStatus)
}

func (s *ArticleSummaryService) buildSummaryState(ctx context.Context, article model.Article) (string, string, string, string) {
	result := s.buildSummaryStateResult(ctx, article)
	return result.AISummary, result.AISummaryStatus, result.DisplaySummary, result.DisplayStatus
}

func (s *ArticleSummaryService) buildSummaryStateResult(ctx context.Context, article model.Article) summaryBuildResult {
	localSummary, localStatus := BuildDisplaySummary(article)
	aiSummary, aiStatus, debug := s.buildAISummary(ctx, article)
	if strings.TrimSpace(aiSummary) != "" {
		return summaryBuildResult{
			AISummary:       aiSummary,
			AISummaryStatus: aiStatus,
			DisplaySummary:  wrapSummaryHTML([]string{aiSummary}),
			DisplayStatus:   DisplaySummaryReady,
			Debug:           debug,
		}
	}
	if strings.TrimSpace(localSummary) != "" {
		debug.UsedAI = false
		if strings.TrimSpace(debug.Strategy) == "" {
			debug.Strategy = "fallback"
		}
		return summaryBuildResult{
			AISummary:       "",
			AISummaryStatus: aiStatus,
			DisplaySummary:  localSummary,
			DisplayStatus:   localStatus,
			Debug:           debug,
		}
	}
	debug.UsedAI = false
	if strings.TrimSpace(debug.Strategy) == "" {
		debug.Strategy = "fallback"
	}
	return summaryBuildResult{
		AISummary:       "",
		AISummaryStatus: aiStatus,
		DisplaySummary:  "",
		DisplayStatus:   DisplaySummaryFailed,
		Debug:           debug,
	}
}

func (s *ArticleSummaryService) buildAISummary(ctx context.Context, article model.Article) (string, string, model.SummaryDebug) {
	if s.loadAIConfig == nil || s.httpClient == nil {
		return "", "", model.SummaryDebug{Strategy: "fallback"}
	}
	cfg, err := s.loadAIConfig()
	if err != nil || strings.TrimSpace(cfg.APIKey) == "" {
		return "", "", model.SummaryDebug{Strategy: "fallback"}
	}
	aiSummary, debug, err := s.generateSummaryWithAI(ctx, article, cfg)
	if err != nil || strings.TrimSpace(aiSummary) == "" {
		debug.UsedAI = false
		if strings.TrimSpace(debug.Strategy) == "" {
			debug.Strategy = "fallback"
		}
		return "", AISummaryFailed, debug
	}
	debug.UsedAI = true
	return aiSummary, AISummaryReady, debug
}

func BuildDisplaySummary(article model.Article) (string, string) {
	return buildFallbackDisplaySummary(article, true)
}

func buildFallbackDisplaySummary(article model.Article, preferFullContent bool) (string, string) {
	primaryBlocks := normalizeSummaryBlocks(article.FullContent, 420, 3)
	if !preferFullContent || len(primaryBlocks) == 0 {
		primaryBlocks = normalizeSummaryBlocks(article.Summary, 320, 2)
		if len(primaryBlocks) == 0 && preferFullContent {
			primaryBlocks = normalizeSummaryBlocks(article.FullContent, 420, 3)
		}
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
	sourceBlocks := extractSummarySourceBlocks(raw)
	if len(sourceBlocks) == 0 || maxChars <= 0 || maxBlocks <= 0 {
		return nil
	}

	blocks := make([]string, 0, maxBlocks)
	usedChars := 0
	for _, normalized := range sourceBlocks {
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

func extractSummarySourceBlocks(raw string) []string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}

	text = summaryMarkdownImagePattern.ReplaceAllString(text, " ")
	if strings.Contains(text, "<") && strings.Contains(text, ">") {
		text = summaryHTMLFigurePattern.ReplaceAllString(text, "\n\n")
		text = summaryHTMLImagePattern.ReplaceAllString(text, " ")
		text = summaryHTMLBlockBreakPattern.ReplaceAllString(text, "\n\n")
		text = summaryHTMLTagPattern.ReplaceAllString(text, " ")
		text = html.UnescapeString(text)
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = summaryParagraphCollapsePattern.ReplaceAllString(text, "\n\n")

	rawBlocks := summaryParagraphSplitPattern.Split(text, -1)
	blocks := make([]string, 0, len(rawBlocks))
	for _, block := range rawBlocks {
		normalized := strings.Join(strings.Fields(strings.TrimSpace(block)), " ")
		if normalized == "" {
			continue
		}
		if isImageNoiseBlock(normalized) {
			continue
		}
		blocks = append(blocks, normalized)
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

func (s *ArticleSummaryService) generateSummaryWithAI(ctx context.Context, article model.Article, cfg SummaryAIConfig) (string, model.SummaryDebug, error) {
	plan := planSummaryContentChunks(article.FullContent, aiSummaryChunkLimit, aiSummaryChunkMaxCount)
	debug := model.SummaryDebug{
		Strategy:    plan.Strategy,
		QueryMode:   "single_pass_main_point",
		ChunkCount:  len(plan.Chunks),
		WindowCount: plan.WindowCount,
	}
	if len(plan.Chunks) > 1 {
		partialSummaries := make([]string, 0, len(plan.Chunks))
		for _, chunk := range plan.Chunks {
			partialResult, err := s.generateSummaryWithPrompt(ctx, article, cfg, buildSummaryChunkPrompt(article, chunk))
			if err != nil {
				return "", debug, err
			}
			partial := partialResult.Text
			if strings.TrimSpace(partial) == "" {
				return "", debug, errors.New("empty partial summary content")
			}
			partialSummaries = append(partialSummaries, partial)
		}
		debug.QueryMode = "aggregate_main_point"
		finalResult, err := s.generateSummaryWithPrompt(ctx, article, cfg, buildSummaryAggregatePrompt(article, partialSummaries, plan.Chunks))
		debug.FinalSentenceClosed = finalResult.Closed
		debug.RewritePassed = finalResult.RewritePassed
		return finalResult.Text, debug, err
	}
	singleBody := ""
	if len(plan.Chunks) == 1 {
		singleBody = plan.Chunks[0]
	}
	finalResult, err := s.generateSummaryWithPrompt(ctx, article, cfg, buildSummaryPrompt(article, singleBody))
	debug.FinalSentenceClosed = finalResult.Closed
	debug.RewritePassed = finalResult.RewritePassed
	return finalResult.Text, debug, err
}

func (s *ArticleSummaryService) generateSummaryWithPrompt(ctx context.Context, article model.Article, cfg SummaryAIConfig, spec summaryPromptSpec) (summaryPromptResult, error) {
	switch normalizeAIProtocol(cfg.Protocol) {
	case "anthropic":
		return s.generateSummaryWithAnthropic(ctx, article, cfg, spec)
	default:
		return s.generateSummaryWithOpenAI(ctx, article, cfg, spec)
	}
}

func (s *ArticleSummaryService) generateSummaryWithOpenAI(ctx context.Context, article model.Article, cfg SummaryAIConfig, spec summaryPromptSpec) (summaryPromptResult, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(firstNonEmpty(cfg.BaseURL, s.defaultBaseURL)), "/")
	model := strings.TrimSpace(firstNonEmpty(cfg.Model, s.defaultModel))
	if baseURL == "" || model == "" {
		return summaryPromptResult{}, errors.New("ai summary config is incomplete")
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
				Content: spec.SystemPrompt,
			},
			{
				Role:    "user",
				Content: spec.UserPrompt,
			},
		},
		"temperature": 0.7,
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return summaryPromptResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(payloadBytes))
	if err != nil {
		return summaryPromptResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(cfg.APIKey))

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return summaryPromptResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return summaryPromptResult{}, fmt.Errorf("summary upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return summaryPromptResult{}, err
	}
	if len(out.Choices) == 0 {
		return summaryPromptResult{}, errors.New("empty summary response")
	}
	result := strings.TrimSpace(stripThinkTags(out.Choices[0].Message.Content))
	if result == "" {
		return summaryPromptResult{}, errors.New("empty summary content")
	}
	normalized, closed := normalizeAISummaryResult(result)
	promptResult := summaryPromptResult{Text: normalized, Closed: closed}
	if spec.AllowRewrite && shouldRewriteAISummary(normalized) {
		rewritten, err := s.generateSummaryWithOpenAI(ctx, article, cfg, buildSummaryRewritePrompt(article, normalized, spec.QueryMode))
		if err == nil && strings.TrimSpace(rewritten.Text) != "" {
			rewritten.RewritePassed = true
			return rewritten, nil
		}
	}
	return promptResult, nil
}

func (s *ArticleSummaryService) generateSummaryWithAnthropic(ctx context.Context, article model.Article, cfg SummaryAIConfig, spec summaryPromptSpec) (summaryPromptResult, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(firstNonEmpty(cfg.BaseURL, "https://api.minimaxi.com/anthropic")), "/")
	model := strings.TrimSpace(firstNonEmpty(cfg.Model, s.defaultModel))
	if baseURL == "" || model == "" {
		return summaryPromptResult{}, errors.New("ai summary config is incomplete")
	}

	reqPayload := map[string]any{
		"model":      model,
		"max_tokens": 1200,
		"system":     spec.SystemPrompt,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]string{
					{
						"type": "text",
						"text": spec.UserPrompt,
					},
				},
			},
		},
		"temperature": 0.7,
	}
	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return summaryPromptResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/messages", bytes.NewReader(payloadBytes))
	if err != nil {
		return summaryPromptResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", strings.TrimSpace(cfg.APIKey))
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := s.httpClient().Do(req)
	if err != nil {
		return summaryPromptResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return summaryPromptResult{}, fmt.Errorf("summary upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&out); err != nil {
		return summaryPromptResult{}, err
	}
	for _, block := range out.Content {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			normalized, closed := normalizeAISummaryResult(block.Text)
			promptResult := summaryPromptResult{Text: normalized, Closed: closed}
			if spec.AllowRewrite && shouldRewriteAISummary(normalized) {
				rewritten, err := s.generateSummaryWithAnthropic(ctx, article, cfg, buildSummaryRewritePrompt(article, normalized, spec.QueryMode))
				if err == nil && strings.TrimSpace(rewritten.Text) != "" {
					rewritten.RewritePassed = true
					return rewritten, nil
				}
			}
			return promptResult, nil
		}
	}
	return summaryPromptResult{}, errors.New("empty summary content")
}

func normalizeAISummaryResult(text string) (string, bool) {
	normalized := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if normalized == "" {
		return "", false
	}
	closed := isClosedSentence(normalized)
	if closed {
		return normalized, true
	}
	if completed := trimToLastCompleteSentence(normalized); completed != "" {
		return completed, true
	}
	return normalized + "。", true
}

func buildSummaryPrompt(article model.Article, body string) summaryPromptSpec {
	var builder strings.Builder
	builder.WriteString("请把下面内容概括成一段适合扫读的中文摘要。输入可能是文章、帖子、论坛回复、经验贴、评论串或混合文本。\n")
	builder.WriteString("目标：让读者扫一眼就知道主要内容。\n")
	builder.WriteString("要求：\n")
	builder.WriteString("1. 输出单段中文摘要，通常 2 到 4 句，先说最主要内容，再补最关键的背景、变化、结论或立场。\n")
	builder.WriteString("2. 只保留最有用的支撑信息，不要罗列过多地名、价格、时间、枝节细节。\n")
	builder.WriteString("3. 不要重复标题，不要写成“这篇文章/作者介绍了”，不要输出项目符号。\n")
	builder.WriteString("4. 如果输入是讨论或回复，优先概括核心观点、争议点和最终态度；如果是资讯或分析，优先概括主事件、判断和影响。\n")
	builder.WriteString("5. 必须自然收尾，不能停在半句。\n\n")
	builder.WriteString("你需要先回答这些问题，再写最终摘要：\n")
	builder.WriteString("A. 这段内容最主要在说什么？\n")
	builder.WriteString("B. 读者扫一眼最需要知道什么？\n")
	builder.WriteString("C. 最后给出的结论、态度或结果是什么？\n\n")
	builder.WriteString("标题：\n")
	builder.WriteString(strings.TrimSpace(article.Title))
	builder.WriteString("\n\n原始摘要：\n")
	builder.WriteString(strings.TrimSpace(strings.Join(normalizeSummaryBlocks(article.Summary, 500, 3), "\n\n")))
	builder.WriteString("\n\n正文片段：\n")
	builder.WriteString(strings.TrimSpace(body))
	return summaryPromptSpec{
		SystemPrompt: buildSummarySystemPrompt(),
		UserPrompt:   strings.TrimSpace(builder.String()),
		QueryMode:    "single_pass_main_point",
		AllowRewrite: true,
	}
}

func buildSummaryChunkPrompt(article model.Article, chunk string) summaryPromptSpec {
	var builder strings.Builder
	builder.WriteString("请阅读下面这一部分内容，并提炼一段供最终摘要合并使用的中文阶段摘要。输入可能来自文章、帖子、回复串或混合文本。\n")
	builder.WriteString("要求：\n")
	builder.WriteString("1. 输出单段，通常 1 到 3 句。\n")
	builder.WriteString("2. 只保留这一部分的主旨、最关键的事实/观点/变化，以及应被最终摘要保留的结论。\n")
	builder.WriteString("3. 删除枝节性例子和重复信息，不要把它写成这部分内容的压缩复述。\n")
	builder.WriteString("4. 不要重复标题，不要输出项目符号，必须自然收尾。\n\n")
	builder.WriteString("你需要先回答这些问题，再写阶段摘要：\n")
	builder.WriteString("A. 这一部分最主要在说什么？\n")
	builder.WriteString("B. 哪个事实、观点或变化最值得留到最终摘要？\n")
	builder.WriteString("C. 哪些细节只是例子，不值得全部带入最终摘要？\n\n")
	builder.WriteString("标题：\n")
	builder.WriteString(strings.TrimSpace(article.Title))
	builder.WriteString("\n\n原始摘要：\n")
	builder.WriteString(strings.TrimSpace(strings.Join(normalizeSummaryBlocks(article.Summary, 500, 3), "\n\n")))
	builder.WriteString("\n\n当前正文片段：\n")
	builder.WriteString(strings.TrimSpace(chunk))
	return summaryPromptSpec{
		SystemPrompt: buildSummarySystemPrompt(),
		UserPrompt:   strings.TrimSpace(builder.String()),
		QueryMode:    "chunk_main_point",
		AllowRewrite: false,
	}
}

func buildSummaryAggregatePrompt(article model.Article, partialSummaries []string, sourceChunks []string) summaryPromptSpec {
	var builder strings.Builder
	builder.WriteString("请根据下面的阶段摘要和对应原文片段，合并成一段适合扫读的中文最终摘要。输入可能是文章、帖子、论坛回复、评论串或混合文本。\n")
	builder.WriteString("目标：让读者扫一眼就知道全文主要内容，而不是看到压缩版正文。\n")
	builder.WriteString("要求：\n")
	builder.WriteString("1. 输出单段中文摘要，通常 2 到 4 句，先说主线，再补最关键的背景、变化、结论或立场。\n")
	builder.WriteString("2. 如果多个阶段摘要说的是同一主线，只保留一次，不要堆砌重复信息。\n")
	builder.WriteString("3. 只保留最能说明问题的一个或少数几个例子，不要枚举路线、地名、价格或枝节细节。\n")
	builder.WriteString("4. 优先覆盖最终结论、态度和对读者最值得知道的部分，避免只总结开头。\n")
	builder.WriteString("5. 不要重复标题，不要输出项目符号，不要提到“第几部分”，必须自然收尾。\n\n")
	builder.WriteString("你需要先回答这些问题，再写最终摘要：\n")
	builder.WriteString("A. 全文最主要在说什么？\n")
	builder.WriteString("B. 读者扫一眼最需要知道的 1 到 2 个重点是什么？\n")
	builder.WriteString("C. 全文最后落到什么结论、判断或态度上？\n\n")
	builder.WriteString("标题：\n")
	builder.WriteString(strings.TrimSpace(article.Title))
	builder.WriteString("\n\n原始摘要：\n")
	builder.WriteString(strings.TrimSpace(strings.Join(normalizeSummaryBlocks(article.Summary, 500, 3), "\n\n")))
	builder.WriteString("\n\n阶段摘要：\n")
	for index, partial := range partialSummaries {
		builder.WriteString(fmt.Sprintf("%d. %s\n", index+1, strings.TrimSpace(partial)))
	}
	builder.WriteString("\n对应原文片段：\n")
	for index, chunk := range sourceChunks {
		builder.WriteString(fmt.Sprintf("%d. %s\n", index+1, strings.TrimSpace(chunk)))
	}
	return summaryPromptSpec{
		SystemPrompt: buildSummarySystemPrompt(),
		UserPrompt:   strings.TrimSpace(builder.String()),
		QueryMode:    "aggregate_main_point",
		AllowRewrite: true,
	}
}

func buildSummaryRewritePrompt(article model.Article, draft string, queryMode string) summaryPromptSpec {
	var builder strings.Builder
	builder.WriteString("下面是一段已经生成的中文摘要，但它偏长、偏密、像压缩版正文，不够适合扫读。\n")
	builder.WriteString("请把它重写成更易读的最终摘要，要求：\n")
	builder.WriteString("1. 输出单段中文摘要，通常 2 到 4 句，先说主旨，再补最重要的背景、变化、结论或立场。\n")
	builder.WriteString("2. 保留主线和结论，删除次要细节、枚举和重复信息。\n")
	builder.WriteString("3. 不要重复标题，不要写成“这篇文章/作者认为”，不要输出项目符号。\n")
	builder.WriteString("4. 必须自然收尾，不能留下半句。\n\n")
	builder.WriteString("标题：\n")
	builder.WriteString(strings.TrimSpace(article.Title))
	builder.WriteString("\n\n当前摘要草稿：\n")
	builder.WriteString(strings.TrimSpace(draft))
	return summaryPromptSpec{
		SystemPrompt: buildSummarySystemPrompt(),
		UserPrompt:   strings.TrimSpace(builder.String()),
		QueryMode:    queryMode,
		AllowRewrite: false,
	}
}

func buildSummarySystemPrompt() string {
	return "You summarize heterogeneous text inputs for Chinese readers. The input may be an article, forum reply, thread, guide, essay, note, review, or mixed text. Write one readable Chinese paragraph that helps the reader understand the main point at a glance. Lead with the central idea, keep only the most important support, conclusion, or stance, avoid list-like detail dumps, and end naturally. Return only the final summary in Chinese with no markdown, no bullets, and no preamble."
}

func shouldRewriteAISummary(text string) bool {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return false
	}
	sentenceCount := countSummarySentences(normalized)
	runeCount := len([]rune(normalized))
	detailSeparators := strings.Count(normalized, "，") + strings.Count(normalized, "、") + strings.Count(normalized, "；")
	if sentenceCount > 4 {
		return true
	}
	if runeCount > 280 && sentenceCount >= 3 {
		return true
	}
	if runeCount > 160 && sentenceCount >= 3 && detailSeparators >= 4 {
		return true
	}
	if sentenceCount >= 3 && detailSeparators >= 8 {
		return true
	}
	if runeCount > 220 && detailSeparators >= 6 {
		return true
	}
	return false
}

func countSummarySentences(text string) int {
	count := 0
	for _, r := range text {
		switch r {
		case '。', '！', '？', '!', '?', ';', '；':
			count++
		}
	}
	if count == 0 && strings.TrimSpace(text) != "" {
		return 1
	}
	return count
}

func splitSummaryContentChunks(raw string, chunkLimit int, maxChunks int) []string {
	return planSummaryContentChunks(raw, chunkLimit, maxChunks).Chunks
}

func planSummaryContentChunks(raw string, chunkLimit int, maxChunks int) summaryChunkPlan {
	blocks := extractSummarySourceBlocks(raw)
	if len(blocks) == 0 || chunkLimit <= 0 || maxChunks <= 0 {
		return summaryChunkPlan{}
	}
	totalChars := totalSummaryBlockChars(blocks)
	switch {
	case totalChars <= chunkLimit:
		return summaryChunkPlan{
			Chunks:      []string{strings.Join(blocks, "\n\n")},
			Strategy:    "single_pass",
			WindowCount: 1,
		}
	case totalChars <= aiSummarySourceBudget:
		chunks := packSummaryBlocksIntoChunks(blocks, chunkLimit, maxChunks)
		return summaryChunkPlan{
			Chunks:      chunks,
			Strategy:    "chunk_aggregate",
			WindowCount: 1,
		}
	default:
		selected, windowCount := selectTieredSummaryPromptBlocks(blocks, chunkLimit*maxChunks)
		chunks := packSummaryBlocksIntoChunks(selected, chunkLimit, maxChunks)
		return summaryChunkPlan{
			Chunks:      chunks,
			Strategy:    "multi_window_aggregate",
			WindowCount: windowCount,
		}
	}
}

func packSummaryBlocksIntoChunks(blocks []string, chunkLimit int, maxChunks int) []string {
	chunks := make([]string, 0, maxChunks)
	current := make([]string, 0, 4)
	currentLen := 0
	for _, block := range blocks {
		blockLen := len([]rune(block))
		if blockLen == 0 {
			continue
		}
		additional := blockLen
		if len(current) > 0 {
			additional += 2
		}
		if len(current) > 0 && currentLen+additional > chunkLimit {
			chunks = append(chunks, strings.Join(current, "\n\n"))
			if len(chunks) >= maxChunks {
				return chunks
			}
			current = current[:0]
			currentLen = 0
		}
		if blockLen > chunkLimit {
			block = truncateRunes(block, chunkLimit)
			blockLen = len([]rune(block))
		}
		current = append(current, block)
		if currentLen == 0 {
			currentLen = blockLen
		} else {
			currentLen += 2 + blockLen
		}
	}
	if len(current) > 0 && len(chunks) < maxChunks {
		chunks = append(chunks, strings.Join(current, "\n\n"))
	}
	return chunks
}

func selectTieredSummaryPromptBlocks(blocks []string, maxChars int) ([]string, int) {
	if len(blocks) == 0 || maxChars <= 0 {
		return nil, 0
	}
	scored := scoreSummaryBlocks(blocks)
	selected := make([]scoredBlock, 0, len(scored))
	seen := make(map[int]struct{}, len(scored))
	windowCount := 0
	bandSize := (len(scored) + aiSummaryWindowBands - 1) / aiSummaryWindowBands
	if bandSize <= 0 {
		bandSize = len(scored)
	}
	perBandBudget := maxChars / aiSummaryWindowBands
	if perBandBudget <= 0 {
		perBandBudget = maxChars
	}
	for band := 0; band < aiSummaryWindowBands; band++ {
		start := band * bandSize
		if start >= len(scored) {
			break
		}
		end := start + bandSize
		if end > len(scored) {
			end = len(scored)
		}
		bandAdded := false
		for _, candidate := range bestContinuousWindow(scored, start, end, perBandBudget) {
			if _, exists := seen[candidate.Index]; exists {
				continue
			}
			seen[candidate.Index] = struct{}{}
			selected = append(selected, candidate)
			bandAdded = true
		}
		if bandAdded {
			windowCount++
		}
	}
	if len(selected) == 0 {
		selected = append(selected, bestContinuousWindow(scored, 0, len(scored), maxChars)...)
		windowCount = 1
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Index < selected[j].Index
	})
	return fitSummaryBlocksWithinBudget(selected, maxChars), windowCount
}

type scoredBlock struct {
	Index int
	Text  string
	Score int
}

func scoreSummaryBlocks(blocks []string) []scoredBlock {
	scored := make([]scoredBlock, 0, len(blocks))
	for index, block := range blocks {
		scored = append(scored, scoredBlock{
			Index: index,
			Text:  block,
			Score: scoreSummaryPromptBlock(block, index, len(blocks)),
		})
	}
	return scored
}

func bestContinuousWindow(scored []scoredBlock, start int, end int, maxChars int) []scoredBlock {
	if len(scored) == 0 || start < 0 || end > len(scored) || start >= end || maxChars <= 0 {
		return nil
	}
	bestScore := -1 << 30
	bestStart := start
	bestEnd := start
	for left := start; left < end; left++ {
		totalChars := 0
		totalScore := 0
		for right := left; right < end; right++ {
			blockLen := len([]rune(scored[right].Text))
			if right > left {
				blockLen += 2
			}
			if totalChars+blockLen > maxChars {
				break
			}
			totalChars += blockLen
			totalScore += scored[right].Score
			if totalScore > bestScore {
				bestScore = totalScore
				bestStart = left
				bestEnd = right + 1
			}
		}
	}
	if bestEnd <= bestStart {
		return nil
	}
	return append([]scoredBlock(nil), scored[bestStart:bestEnd]...)
}

func fitSummaryBlocksWithinBudget(picks []scoredBlock, maxChars int) []string {
	result := make([]string, 0, len(picks))
	usedChars := 0
	for _, pick := range picks {
		text := pick.Text
		if len(result) > 0 {
			if usedChars+2 >= maxChars {
				break
			}
		}
		remainingChars := maxChars - usedChars
		if len(result) > 0 {
			remainingChars -= 2
		}
		if remainingChars <= 0 {
			break
		}
		if len([]rune(text)) > remainingChars {
			text = truncateRunes(text, remainingChars)
		}
		result = append(result, text)
		usedChars += len([]rune(text))
		if len(result) > 1 {
			usedChars += 2
		}
	}
	return result
}

func isClosedSentence(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	last := []rune(trimmed)[len([]rune(trimmed))-1]
	switch last {
	case '。', '！', '？', '.', '!', '?', '"', '”', '）', ')':
		return true
	default:
		return false
	}
}

func trimToLastCompleteSentence(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	lastBoundary := -1
	for idx, r := range runes {
		switch r {
		case '。', '！', '？', '.', '!', '?':
			lastBoundary = idx
		}
	}
	if lastBoundary < 0 {
		return ""
	}
	return strings.TrimSpace(string(runes[:lastBoundary+1]))
}

func scoreSummaryPromptBlock(text string, index int, total int) int {
	runes := []rune(strings.TrimSpace(text))
	length := len(runes)
	if length == 0 {
		return 0
	}

	score := length
	if length > 220 {
		score = 220 + (length-220)/4
	}
	if length < 40 {
		score -= 40 - length
	}

	punctuationBonus := 0
	for _, token := range []string{"。", "！", "？", "；", ":"} {
		punctuationBonus += strings.Count(text, token) * 8
	}
	score += punctuationBonus
	if total > 0 {
		if index >= total/2 {
			score += 12
		}
		if index >= (total*2)/3 {
			score += 16
		}
	}
	for _, token := range []string{"总结", "结论", "关键", "核心", "最终", "原因", "影响", "但是", "然而", "因此", "值得注意"} {
		score += strings.Count(strings.ToLower(text), token) * 18
	}

	linkPenalty := strings.Count(strings.ToLower(text), "http") * 24
	return score - linkPenalty
}

func totalSummaryBlockChars(blocks []string) int {
	total := 0
	for idx, block := range blocks {
		if idx > 0 {
			total += 2
		}
		total += len([]rune(block))
	}
	return total
}

func isImageNoiseBlock(text string) bool {
	normalized := strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
	if normalized == "" {
		return true
	}
	for _, prefix := range []string{"图 ", "图：", "图:", "图源", "图片来源", "image:", "photo by", "source:", "fig.", "figure ", "caption:"} {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	if regexp.MustCompile(`(?i)\bhttps?://\S+\.(png|jpg|jpeg|gif|webp|svg)\b`).MatchString(normalized) {
		return true
	}
	if regexp.MustCompile(`(?i)^\[?image\]?[\s:：-]*`).MatchString(normalized) {
		return true
	}
	if regexp.MustCompile(`(?i)^!\[[^\]]*\]`).MatchString(normalized) {
		return true
	}
	if regexp.MustCompile(`(?i)^(图|figure|fig)\s*\d+`).MatchString(normalized) {
		return true
	}
	if strings.Contains(normalized, "图片") || strings.Contains(normalized, "配图") || strings.Contains(normalized, "截图") {
		letters := 0
		for _, r := range normalized {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				letters++
			}
		}
		if letters <= 48 {
			return true
		}
	}
	return false
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
