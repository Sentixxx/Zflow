package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/pkg/logger"
)

const (
	topicBriefName      = "topic_brief"
	briefMinScore       = 60
	briefMaxArticles    = 30
	briefMaxSourceChars = 6000
)

// TopicBriefAgent generates periodic briefs (daily/weekly/monthly) by
// summarising high-score articles and cluster summaries via LLM.
type TopicBriefAgent struct {
	deps   Deps
	level  string // "daily", "weekly", "monthly"
	logger *logger.ModuleLogger
}

func NewTopicBriefAgent(deps Deps, level string) *TopicBriefAgent {
	return &TopicBriefAgent{
		deps:   deps,
		level:  level,
		logger: logger.NewModuleFromEnv("agent"),
	}
}

func (a *TopicBriefAgent) Name() string { return topicBriefName + "_" + a.level }

func (a *TopicBriefAgent) Run(ctx context.Context) error {
	if a.deps.LLMCall == nil {
		return fmt.Errorf("LLM not configured, skipping brief generation")
	}

	switch a.level {
	case "daily":
		return a.generateDailyBrief(ctx)
	case "weekly":
		return a.generateWeeklyBrief(ctx)
	case "monthly":
		return a.generateMonthlyBrief(ctx)
	default:
		return fmt.Errorf("unknown brief level: %s", a.level)
	}
}

func (a *TopicBriefAgent) generateDailyBrief(ctx context.Context) error {
	now := time.Now().UTC()
	periodStart := now.Truncate(24 * time.Hour).Format("2006-01-02")
	periodEnd := periodStart

	// Collect high-score articles from today
	articles := a.deps.FeedRepo.ListArticles()
	var selected []model.Article
	for _, art := range articles {
		if len(selected) >= briefMaxArticles {
			break
		}
		score := 0
		if art.RecommendationScores != nil {
			score = art.RecommendationScores.Composite
		}
		if score >= briefMinScore {
			selected = append(selected, art)
		}
	}

	if len(selected) == 0 {
		a.logger.Debug("run", a.Name(), "ok", "no high-score articles for daily brief")
		return nil
	}

	// Collect cluster summaries
	clusters, _ := a.deps.AgentRepo.ListActiveClusters(ctx)

	// Build source material
	sourceText := a.buildSourceText(selected, clusters)

	// Generate brief via LLM
	systemPrompt := "You are a concise news analyst. Generate a daily brief in markdown that summarises the key topics and articles. Group related items. Use bullet points. Write in the same language as the source material."
	userPrompt := fmt.Sprintf("Generate a daily brief for %s based on these articles and topic clusters:\n\n%s", periodStart, sourceText)

	content, err := a.deps.LLMCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("LLM call: %w", err)
	}

	articleIDs := make([]int64, len(selected))
	for i, art := range selected {
		articleIDs[i] = art.ID
	}
	clusterIDs := make([]int64, len(clusters))
	for i, c := range clusters {
		clusterIDs[i] = c.ID
	}

	slug := fmt.Sprintf("daily-%s", periodStart)
	title := fmt.Sprintf("Daily Brief: %s", periodStart)

	_, err = a.deps.AgentRepo.CreateTopicBrief(ctx, model.TopicBrief{
		Title:            title,
		Slug:             slug,
		Content:          content,
		Level:            "daily",
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		SourceClusterIDs: clusterIDs,
		SourceArticleIDs: articleIDs,
	})
	if err != nil {
		return fmt.Errorf("create daily brief: %w", err)
	}

	a.logger.Info("run", a.Name(), "ok", "daily brief generated", "articles", fmt.Sprint(len(selected)))
	return nil
}

func (a *TopicBriefAgent) generateWeeklyBrief(ctx context.Context) error {
	now := time.Now().UTC()
	weekStart := now.AddDate(0, 0, -int(now.Weekday()))
	periodStart := weekStart.Format("2006-01-02")
	periodEnd := now.Format("2006-01-02")

	// Collect daily briefs from this week
	dailyBriefs, err := a.deps.AgentRepo.ListTopicBriefs(ctx, "daily", 7)
	if err != nil {
		return fmt.Errorf("list daily briefs: %w", err)
	}
	if len(dailyBriefs) == 0 {
		a.logger.Debug("run", a.Name(), "ok", "no daily briefs for weekly brief")
		return nil
	}

	var sourceBuilder strings.Builder
	var allArticleIDs []int64
	var allClusterIDs []int64
	for _, b := range dailyBriefs {
		fmt.Fprintf(&sourceBuilder, "## %s\n%s\n\n", b.Title, b.Content)
		allArticleIDs = append(allArticleIDs, b.SourceArticleIDs...)
		allClusterIDs = append(allClusterIDs, b.SourceClusterIDs...)
	}

	sourceText := sourceBuilder.String()
	if len(sourceText) > briefMaxSourceChars {
		sourceText = sourceText[:briefMaxSourceChars]
	}

	systemPrompt := "You are a concise news analyst. Compress multiple daily briefs into one weekly summary in markdown. Remove redundancy, keep core insights. Write in the same language as the source material."
	userPrompt := fmt.Sprintf("Generate a weekly brief for %s to %s from these daily briefs:\n\n%s", periodStart, periodEnd, sourceText)

	content, err := a.deps.LLMCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("LLM call: %w", err)
	}

	slug := fmt.Sprintf("weekly-%s", periodStart)
	title := fmt.Sprintf("Weekly Brief: %s ~ %s", periodStart, periodEnd)

	_, err = a.deps.AgentRepo.CreateTopicBrief(ctx, model.TopicBrief{
		Title:            title,
		Slug:             slug,
		Content:          content,
		Level:            "weekly",
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		SourceClusterIDs: dedup(allClusterIDs),
		SourceArticleIDs: dedup(allArticleIDs),
	})
	if err != nil {
		return fmt.Errorf("create weekly brief: %w", err)
	}

	a.logger.Info("run", a.Name(), "ok", "weekly brief generated", "source_briefs", fmt.Sprint(len(dailyBriefs)))
	return nil
}

func (a *TopicBriefAgent) generateMonthlyBrief(ctx context.Context) error {
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	periodStart := monthStart.Format("2006-01-02")
	periodEnd := now.Format("2006-01-02")

	weeklyBriefs, err := a.deps.AgentRepo.ListTopicBriefs(ctx, "weekly", 5)
	if err != nil {
		return fmt.Errorf("list weekly briefs: %w", err)
	}
	if len(weeklyBriefs) == 0 {
		a.logger.Debug("run", a.Name(), "ok", "no weekly briefs for monthly brief")
		return nil
	}

	var sourceBuilder strings.Builder
	var allArticleIDs []int64
	var allClusterIDs []int64
	for _, b := range weeklyBriefs {
		fmt.Fprintf(&sourceBuilder, "## %s\n%s\n\n", b.Title, b.Content)
		allArticleIDs = append(allArticleIDs, b.SourceArticleIDs...)
		allClusterIDs = append(allClusterIDs, b.SourceClusterIDs...)
	}

	sourceText := sourceBuilder.String()
	if len(sourceText) > briefMaxSourceChars {
		sourceText = sourceText[:briefMaxSourceChars]
	}

	systemPrompt := "You are a concise news analyst. Compress multiple weekly briefs into one monthly summary in markdown. Identify recurring themes. Write in the same language as the source material."
	userPrompt := fmt.Sprintf("Generate a monthly brief for %s to %s from these weekly briefs:\n\n%s", periodStart, periodEnd, sourceText)

	content, err := a.deps.LLMCall(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("LLM call: %w", err)
	}

	slug := fmt.Sprintf("monthly-%s", periodStart[:7])
	title := fmt.Sprintf("Monthly Brief: %s", periodStart[:7])

	_, err = a.deps.AgentRepo.CreateTopicBrief(ctx, model.TopicBrief{
		Title:            title,
		Slug:             slug,
		Content:          content,
		Level:            "monthly",
		PeriodStart:      periodStart,
		PeriodEnd:        periodEnd,
		SourceClusterIDs: dedup(allClusterIDs),
		SourceArticleIDs: dedup(allArticleIDs),
	})
	if err != nil {
		return fmt.Errorf("create monthly brief: %w", err)
	}

	a.logger.Info("run", a.Name(), "ok", "monthly brief generated", "source_briefs", fmt.Sprint(len(weeklyBriefs)))
	return nil
}

func (a *TopicBriefAgent) buildSourceText(articles []model.Article, clusters []model.TopicCluster) string {
	var b strings.Builder
	remaining := briefMaxSourceChars

	if len(clusters) > 0 {
		b.WriteString("# Topic Clusters\n\n")
		for _, c := range clusters {
			if c.Summary != "" {
				line := fmt.Sprintf("- **%s** (%d articles): %s\n", c.Title, c.ArticleCount, c.Summary)
				if len(line) > remaining {
					break
				}
				b.WriteString(line)
				remaining -= len(line)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("# Articles\n\n")
	for _, art := range articles {
		summary := art.DisplaySummary
		if summary == "" {
			summary = art.AISummary
		}
		if summary == "" {
			summary = art.Summary
		}
		if len(summary) > 300 {
			summary = summary[:300] + "..."
		}
		line := fmt.Sprintf("- **%s**: %s\n", art.Title, summary)
		if len(line) > remaining {
			break
		}
		b.WriteString(line)
		remaining -= len(line)
	}

	return b.String()
}

func dedup(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}
	return result
}
