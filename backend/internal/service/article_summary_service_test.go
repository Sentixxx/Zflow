package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sentixxx/Zflow/backend/internal/model"
	"github.com/Sentixxx/Zflow/backend/internal/repository"
)

func TestBuildDisplaySummaryPrefersFullContent(t *testing.T) {
	summary, status := BuildDisplaySummary(model.Article{
		Title:       "Title",
		Summary:     "<p>RSS summary</p>",
		FullContent: "<article><p>Full content paragraph one.</p><p>Full content paragraph two.</p></article>",
	})
	if status != DisplaySummaryFallback {
		t.Fatalf("status = %q, want %q", status, DisplaySummaryFallback)
	}
	if !strings.Contains(summary, "Full content paragraph one.") {
		t.Fatalf("summary = %q, want full content", summary)
	}
	if strings.Contains(summary, "RSS summary") {
		t.Fatalf("summary = %q, want full content preferred over rss summary", summary)
	}
}

func TestBuildDisplaySummaryFallsBackToRawSummary(t *testing.T) {
	summary, status := BuildDisplaySummary(model.Article{
		Title:   "Title",
		Summary: "<p>RSS summary only</p>",
	})
	if status != DisplaySummaryFallback {
		t.Fatalf("status = %q, want %q", status, DisplaySummaryFallback)
	}
	if !strings.Contains(summary, "RSS summary only") {
		t.Fatalf("summary = %q, want rss summary", summary)
	}
}

func TestSplitSummaryContentChunksRespectsThreshold(t *testing.T) {
	raw := "<p>" + strings.Repeat("A", 700) + "</p><p>" + strings.Repeat("B", 700) + "</p><p>" + strings.Repeat("C", 700) + "</p>"
	chunks := splitSummaryContentChunks(raw, 1200, 6)
	if len(chunks) != 3 {
		t.Fatalf("chunks len = %d, want 3", len(chunks))
	}
	for _, chunk := range chunks {
		if len([]rune(chunk)) > 1200 {
			t.Fatalf("chunk len = %d, want <= 1200", len([]rune(chunk)))
		}
	}
}

func TestSelectSummaryPromptBlocksCoversMiddleAndTailContent(t *testing.T) {
	raw := strings.Join([]string{
		"<p>简短导语。</p>",
		"<p>这是一段信息量一般的背景介绍，主要解释写作缘起。</p>",
		"<p>真正的关键变化发生在中段：产品从单机模式切到协作模式，团队因此重写了权限系统与同步链路。</p>",
		"<p>这里继续展开核心影响：新架构让离线写入冲突减少，同时把平均同步时延压到了 200 毫秒以内。</p>",
		"<p>结尾部分给出最终判断：如果你的团队过去因为权限复杂和同步冲突放弃这类工具，这次改版值得重新评估。</p>",
	}, "")

	blocks, _ := selectTieredSummaryPromptBlocks(extractSummarySourceBlocks(raw), 1200)
	joined := strings.Join(blocks, "\n")
	if !strings.Contains(joined, "真正的关键变化发生在中段") {
		t.Fatalf("selected blocks = %q, want middle key block", joined)
	}
	if !strings.Contains(joined, "结尾部分给出最终判断") {
		t.Fatalf("selected blocks = %q, want tail conclusion block", joined)
	}
}

func TestExtractSummarySourceBlocksSkipsImageNoise(t *testing.T) {
	raw := strings.Join([]string{
		"<article>",
		"<p>这里是正常正文，说明真正的论点。</p>",
		"<figure><img src=\"https://cdn.example.com/cover.png\" alt=\"封面\" /><figcaption>图 1：封面图片，图片来源：示例网站</figcaption></figure>",
		"<p>![封面图](https://cdn.example.com/cover.png)</p>",
		"<p>结尾继续给出关键判断。</p>",
		"</article>",
	}, "")

	blocks := extractSummarySourceBlocks(raw)
	joined := strings.Join(blocks, "\n")
	if strings.Contains(joined, "图片来源") || strings.Contains(joined, "封面图") {
		t.Fatalf("blocks = %q, want image noise removed", joined)
	}
	if !strings.Contains(joined, "这里是正常正文") || !strings.Contains(joined, "结尾继续给出关键判断") {
		t.Fatalf("blocks = %q, want normal text kept", joined)
	}
}

func TestArticleSummaryServiceBackfillFeed(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feed, err := repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>Hello world</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		nil,
		nil,
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.BackfillFeed(feed.ID, 20); err != nil {
		t.Fatalf("BackfillFeed() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}
	if articles[0].DisplaySummary == "" {
		t.Fatalf("display_summary is empty, want generated summary")
	}
	if articles[0].DisplaySummaryStatus == "" {
		t.Fatalf("display_summary_status is empty, want generated status")
	}
}

func TestArticleSummaryServiceUsesAIWhenConfigured(t *testing.T) {
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"这篇文章总结了关键变化，并点出了对读者最重要的影响。"}}]}`))
	}))
	defer aiMock.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feed, err := repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>Hello world</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		func() *http.Client { return aiMock.Client() },
		func() (SummaryAIConfig, error) {
			return SummaryAIConfig{
				APIKey:  "test-key",
				BaseURL: aiMock.URL,
				Model:   "MiniMax-M2.7",
			}, nil
		},
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.BackfillFeed(feed.ID, 20); err != nil {
		t.Fatalf("BackfillFeed() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}
	if !strings.Contains(articles[0].DisplaySummary, "关键变化") {
		t.Fatalf("display_summary = %q, want ai generated summary", articles[0].DisplaySummary)
	}
	if articles[0].DisplaySummaryStatus != DisplaySummaryReady {
		t.Fatalf("display_summary_status = %q, want %q", articles[0].DisplaySummaryStatus, DisplaySummaryReady)
	}
}

func TestArticleSummaryServiceDoesNotTruncateFinalAISummary(t *testing.T) {
	longSentence := strings.Repeat("这是一段完整的 AI 摘要内容，用来验证系统不会在保存阶段对最终结果做字符级硬截断。", 2)
	longSummary := longSentence + " " + longSentence
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"` + longSummary + `"}}]}`))
	}))
	defer aiMock.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feed, err := repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>Hello world</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		func() *http.Client { return aiMock.Client() },
		func() (SummaryAIConfig, error) {
			return SummaryAIConfig{
				APIKey:  "test-key",
				BaseURL: aiMock.URL,
				Model:   "MiniMax-M2.7",
			}, nil
		},
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.BackfillFeed(feed.ID, 20); err != nil {
		t.Fatalf("BackfillFeed() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}
	if !strings.Contains(articles[0].DisplaySummary, longSummary) {
		t.Fatalf("display_summary = %q, want full ai summary without truncation", articles[0].DisplaySummary)
	}
}

func TestBuildSummaryPromptUsesGenericReadableInstructions(t *testing.T) {
	spec := buildSummaryPrompt(model.Article{
		Title:   "标题",
		Summary: "<p>原始摘要</p>",
	}, "正文内容")

	if !strings.Contains(spec.UserPrompt, "文章、帖子、论坛回复、经验贴、评论串或混合文本") {
		t.Fatalf("user prompt = %q, want generic input types", spec.UserPrompt)
	}
	if !strings.Contains(spec.UserPrompt, "扫一眼就知道主要内容") {
		t.Fatalf("user prompt = %q, want glanceable summary objective", spec.UserPrompt)
	}
	if spec.QueryMode != "single_pass_main_point" {
		t.Fatalf("query mode = %q, want single_pass_main_point", spec.QueryMode)
	}
	if !spec.AllowRewrite {
		t.Fatalf("allow rewrite = false, want true")
	}
}

func TestShouldRewriteAISummaryDetectsVerboseDetailDump(t *testing.T) {
	verbose := "作者先解释整体判断，然后按城市、路线、价格、时间和体验逐项展开，信息很多，密度很高，读起来像压缩版正文。接着又补了多个例子、多个地点和多个细节，导致主线开始发散。最后虽然给了结论，但前面已经堆了太多信息，扫读时很难快速把握重点。"
	if !shouldRewriteAISummary(verbose) {
		t.Fatalf("shouldRewriteAISummary(verbose) = false, want true")
	}
	compact := "作者认为新疆正处于国内游红利期，并结合低预算长线旅行经历说明机票和中转策略如何显著降低成本。文章以北疆、南疆和伊犁的路线举例，核心结论是新疆值得趁窗口期尽早去一次。"
	if shouldRewriteAISummary(compact) {
		t.Fatalf("shouldRewriteAISummary(compact) = true, want false")
	}
}

func TestArticleSummaryServiceRewritesVerboseFinalAISummary(t *testing.T) {
	callCount := 0
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if callCount == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"作者先解释新疆旅行为什么便宜，再按乌鲁木齐、富蕴、阿勒泰、莎车、喀什、伊犁和赛里木湖一路展开，期间穿插大量机票价格、中转技巧、景点体验和路线细节。文章还补充了房地产泡沫、经济下行、西部发展等背景，并反复说明航司补贴、酒店打折和基建升级如何让旅行成本下降。整体信息很多，但读者扫一眼不容易抓住主线。"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"作者认为新疆正处于国内游成本下降、体验提升的窗口期，并结合低预算长线旅行经历说明特价机票和中转策略如何降低出行成本。文章用北疆、南疆和伊犁的路线作例子，核心结论是新疆值得趁价格红利仍在时尽早去一次。"}}]}`))
	}))
	defer aiMock.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feed, err := repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>Hello world</p>", FullContent: "<p>正文内容。</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		func() *http.Client { return aiMock.Client() },
		func() (SummaryAIConfig, error) {
			return SummaryAIConfig{
				APIKey:  "test-key",
				BaseURL: aiMock.URL,
				Model:   "MiniMax-M2.7",
			}, nil
		},
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.BackfillFeed(feed.ID, 20); err != nil {
		t.Fatalf("BackfillFeed() error = %v", err)
	}

	article := repo.ListArticles()[0]
	if !strings.Contains(article.AISummary, "窗口期") {
		t.Fatalf("ai_summary = %q, want rewritten concise summary", article.AISummary)
	}
	if strings.Contains(article.AISummary, "乌鲁木齐、富蕴、阿勒泰") {
		t.Fatalf("ai_summary = %q, want verbose route dump removed by rewrite", article.AISummary)
	}
	if callCount != 2 {
		t.Fatalf("callCount = %d, want 2 (draft + rewrite)", callCount)
	}
}

func TestArticleSummaryServiceRefreshRecentArticlesClearsOldSummaryBeforeRegeneration(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	_, err = repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>新的摘要内容</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}
	articleID := articles[0].ID
	if err := repo.UpdateArticleDisplaySummary(articleID, "<p>旧摘要</p>", DisplaySummaryReady); err != nil {
		t.Fatalf("UpdateArticleDisplaySummary(old) error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		nil,
		nil,
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	refreshed, err := svc.RefreshRecentArticles(100)
	if err != nil {
		t.Fatalf("RefreshRecentArticles() error = %v", err)
	}
	if refreshed != 1 {
		t.Fatalf("refreshed = %d, want 1", refreshed)
	}

	updated, ok := repo.GetArticle(articleID)
	if !ok {
		t.Fatalf("GetArticle(%d) ok = false, want true", articleID)
	}
	if strings.Contains(updated.DisplaySummary, "旧摘要") {
		t.Fatalf("display_summary = %q, want regenerated content instead of stale summary", updated.DisplaySummary)
	}
	if !strings.Contains(updated.DisplaySummary, "新的摘要内容") {
		t.Fatalf("display_summary = %q, want regenerated summary from current article content", updated.DisplaySummary)
	}
	if updated.DisplaySummaryStatus != DisplaySummaryFallback {
		t.Fatalf("display_summary_status = %q, want %q", updated.DisplaySummaryStatus, DisplaySummaryFallback)
	}
}

func TestArticleSummaryServiceClearArticleAIFallsBackToRawSummary(t *testing.T) {
	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	_, err = repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{
			Title:       "A1",
			Link:        "https://example.com/1",
			Summary:     "<p>RSS 原始摘要</p>",
			FullContent: "<article><p>正文更长，理论上可以生成本地快速摘要。</p></article>",
		},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	article := repo.ListArticles()[0]
	if err := repo.UpdateArticleSummaryState(article.ID, "模型生成摘要", DisplaySummaryReady, "<p>模型生成摘要</p>", DisplaySummaryReady); err != nil {
		t.Fatalf("UpdateArticleSummaryState() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		nil,
		nil,
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.ClearArticleAI(article.ID); err != nil {
		t.Fatalf("ClearArticleAI() error = %v", err)
	}

	updated, ok := repo.GetArticle(article.ID)
	if !ok {
		t.Fatalf("GetArticle(%d) ok = false, want true", article.ID)
	}
	if updated.AISummary != "" || updated.AISummaryStatus != AISummaryCleared {
		t.Fatalf("ai summary state = %+v, want ai_summary cleared with status %q", updated, AISummaryCleared)
	}
	if !strings.Contains(updated.DisplaySummary, "RSS 原始摘要") {
		t.Fatalf("display_summary = %q, want fallback to raw rss summary", updated.DisplaySummary)
	}
	if updated.DisplaySummaryStatus != DisplaySummaryFallback {
		t.Fatalf("display_summary_status = %q, want %q", updated.DisplaySummaryStatus, DisplaySummaryFallback)
	}
}

func TestArticleSummaryServiceChunksLongContentForAI(t *testing.T) {
	callPrompts := make([]string, 0, 4)
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request error = %v", err)
		}
		if len(payload.Messages) == 0 {
			t.Fatal("messages is empty")
		}
		prompt := payload.Messages[len(payload.Messages)-1].Content
		callPrompts = append(callPrompts, prompt)

		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(prompt, "阶段摘要：") {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"这篇长文先说明背景，再给出关键变化与最终结论。"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"这一部分主要说明了局部重点，并保留了结尾结论。"}}]}`))
	}))
	defer aiMock.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	longContent := "<article><p>" + strings.Repeat("A", 700) + "</p><p>" + strings.Repeat("B", 700) + "</p><p>" + strings.Repeat("C", 700) + "</p></article>"
	feed, err := repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>Hello world</p>", FullContent: longContent},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		func() *http.Client { return aiMock.Client() },
		func() (SummaryAIConfig, error) {
			return SummaryAIConfig{
				APIKey:  "test-key",
				BaseURL: aiMock.URL,
				Model:   "MiniMax-M2.7",
			}, nil
		},
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.BackfillFeed(feed.ID, 20); err != nil {
		t.Fatalf("BackfillFeed() error = %v", err)
	}

	if len(callPrompts) != 4 {
		t.Fatalf("ai call count = %d, want 4", len(callPrompts))
	}
	if !strings.Contains(callPrompts[len(callPrompts)-1], "阶段摘要：") {
		t.Fatalf("final prompt = %q, want aggregate prompt", callPrompts[len(callPrompts)-1])
	}
	if !strings.Contains(callPrompts[len(callPrompts)-1], "对应原文片段：") {
		t.Fatalf("final prompt = %q, want source chunks for grounded aggregation", callPrompts[len(callPrompts)-1])
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}
	if !strings.Contains(articles[0].DisplaySummary, "关键变化与最终结论") {
		t.Fatalf("display_summary = %q, want aggregated ai summary", articles[0].DisplaySummary)
	}
}

func TestArticleSummaryServicePromptIncludesBodyBeyondOpening(t *testing.T) {
	callPrompts := make([]string, 0, 2)
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request error = %v", err)
		}
		prompt := payload.Messages[len(payload.Messages)-1].Content
		callPrompts = append(callPrompts, prompt)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"这篇文章总结了产品从单机到协作的改造，并说明了性能收益。"}}]}`))
	}))
	defer aiMock.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	fullContent := strings.Join([]string{
		"<article>",
		"<p>简短导语。</p>",
		"<p>这是一段信息量一般的背景介绍，主要解释写作缘起。</p>",
		"<p>真正的关键变化发生在中段：产品从单机模式切到协作模式，团队因此重写了权限系统与同步链路。</p>",
		"<p>这里继续展开核心影响：新架构让离线写入冲突减少，同时把平均同步时延压到了 200 毫秒以内。</p>",
		"<p>结尾部分给出最终判断：如果你的团队过去因为权限复杂和同步冲突放弃这类工具，这次改版值得重新评估。</p>",
		"</article>",
	}, "")
	feed, err := repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>背景导语</p>", FullContent: fullContent},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		func() *http.Client { return aiMock.Client() },
		func() (SummaryAIConfig, error) {
			return SummaryAIConfig{
				APIKey:  "test-key",
				BaseURL: aiMock.URL,
				Model:   "MiniMax-M2.7",
			}, nil
		},
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.BackfillFeed(feed.ID, 20); err != nil {
		t.Fatalf("BackfillFeed() error = %v", err)
	}

	if len(callPrompts) == 0 {
		t.Fatal("no AI prompt captured")
	}
	prompt := callPrompts[0]
	if !strings.Contains(prompt, "真正的关键变化发生在中段") {
		t.Fatalf("prompt = %q, want middle key content", prompt)
	}
	if !strings.Contains(prompt, "结尾部分给出最终判断") {
		t.Fatalf("prompt = %q, want tail conclusion content", prompt)
	}
}

func TestArticleSummaryServiceUsesAnthropicWhenConfigured(t *testing.T) {
	aiMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Fatalf("x-api-key = %q, want test-key", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Fatal("anthropic-version header is empty")
		}
		var payload struct {
			MaxTokens int `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode anthropic request error = %v", err)
		}
		if payload.MaxTokens < 1000 {
			t.Fatalf("max_tokens = %d, want >= 1000", payload.MaxTokens)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"这篇文章用简洁方式说明了变化重点，并给出了读者该关注的核心影响。"}]}`))
	}))
	defer aiMock.Close()

	repo, err := repository.NewTestSQLiteFeedRepository(filepath.Join(t.TempDir(), "feeds.db"))
	if err != nil {
		t.Fatalf("NewSQLiteFeedRepository() error = %v", err)
	}
	feed, err := repo.AddInFolder("https://example.com/feed", "Feed", []repository.ArticleSeed{
		{Title: "A1", Link: "https://example.com/1", Summary: "<p>Hello world</p>"},
	}, "", nil, "", "")
	if err != nil {
		t.Fatalf("AddInFolder() error = %v", err)
	}

	svc := NewArticleSummaryService(
		repo,
		func() *http.Client { return aiMock.Client() },
		func() (SummaryAIConfig, error) {
			return SummaryAIConfig{
				Protocol: "anthropic",
				APIKey:   "test-key",
				BaseURL:  aiMock.URL,
				Model:    "MiniMax-Text-01",
			}, nil
		},
		"https://api.openai.com/v1",
		"gpt-4o-mini",
	)
	if err := svc.BackfillFeed(feed.ID, 20); err != nil {
		t.Fatalf("BackfillFeed() error = %v", err)
	}

	articles := repo.ListArticles()
	if len(articles) != 1 {
		t.Fatalf("ListArticles len = %d, want 1", len(articles))
	}
	if !strings.Contains(articles[0].DisplaySummary, "变化重点") {
		t.Fatalf("display_summary = %q, want anthropic generated summary", articles[0].DisplaySummary)
	}
	if articles[0].DisplaySummaryStatus != DisplaySummaryReady {
		t.Fatalf("display_summary_status = %q, want %q", articles[0].DisplaySummaryStatus, DisplaySummaryReady)
	}
}

func TestArticleSummaryServiceFallsBackWhenAIUnavailable(t *testing.T) {
	summary, status := BuildDisplaySummary(model.Article{
		Title:   "Title",
		Summary: "<p>RSS summary only</p>",
	})
	if status != DisplaySummaryFallback {
		t.Fatalf("status = %q, want %q", status, DisplaySummaryFallback)
	}
	if !strings.Contains(summary, "RSS summary only") {
		t.Fatalf("summary = %q, want fallback local summary", summary)
	}
}

func TestNormalizeAISummaryResultTrimsIncompleteTail(t *testing.T) {
	result, closed := normalizeAISummaryResult("新疆正处于国内游红利期，出行成本更低。作者从北疆一路写到伊犁，最后停在赛里木湖")
	if !closed {
		t.Fatalf("closed = false, want true after trimming to last complete sentence")
	}
	if result != "新疆正处于国内游红利期，出行成本更低。" {
		t.Fatalf("result = %q, want trimmed complete sentence", result)
	}
}
