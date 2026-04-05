import type { Article } from "@/types";
import { useRef } from "react";
import { formatRecommendationSummary } from "@/lib/article-list";
import { ArticleDetailTopBar } from "./ArticleDetailTopBar";
import { ArticleFloatingActions } from "./ArticleFloatingActions";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

function getSummaryStatusMeta(status: string | undefined): { label: string; tone: "ready" | "fallback" | "neutral" } {
  switch ((status || "").trim()) {
    case "ready":
      return { label: "AI 摘要", tone: "ready" };
    case "fallback":
      return { label: "快速摘要", tone: "fallback" };
    default:
      return { label: "摘要", tone: "neutral" };
  }
}

type ArticleDetailContentProps = {
  article: Article | null;
  sanitizedSummaryHTML: string;
  sanitizedFullContentHTML: string;
  canMarkUnread: boolean;
  canToggleFavorite: boolean;
  isFavorite: boolean;
  canOpenSourceSite: boolean;
  canExtractReadable: boolean;
  isExtractingReadable: boolean;
  canRefreshArticleCache: boolean;
  isRefreshingArticleCache: boolean;
  isTranslatingArticle: boolean;
  sourceSiteURL: string;
  detailProgressText: string;
  canGoPrev: boolean;
  canGoNext: boolean;
  translationParagraphs: Array<{
    index: number;
    source: string;
    translated: string;
    status: "pending" | "done";
  }>;
  onMarkUnread: () => void;
  onToggleFavorite: () => void;
  onOpenSourceSite: () => void;
  onExtractReadable: () => void;
  onRefreshArticleCache: () => void;
  onTranslateArticle: () => void;
  onGoPrev: () => void;
  onGoNext: () => void;
};

export function ArticleDetailContent({
  article,
  sanitizedSummaryHTML,
  sanitizedFullContentHTML,
  canMarkUnread,
  canToggleFavorite,
  isFavorite,
  canOpenSourceSite,
  canExtractReadable,
  isExtractingReadable,
  canRefreshArticleCache,
  isRefreshingArticleCache,
  isTranslatingArticle,
  sourceSiteURL,
  detailProgressText,
  canGoPrev,
  canGoNext,
  translationParagraphs,
  onMarkUnread,
  onToggleFavorite,
  onOpenSourceSite,
  onExtractReadable,
  onRefreshArticleCache,
  onTranslateArticle,
  onGoPrev,
  onGoNext,
}: ArticleDetailContentProps) {
  const detailRef = useRef<HTMLDivElement | null>(null);
  const normalizedFull = (sanitizedFullContentHTML || "").trim();
  const looksLikePDFGarbage =
    /^%PDF-\d/i.test(normalizedFull) ||
    (normalizedFull.includes("xref") && normalizedFull.includes("endobj"));
  const hasUsableFullContent = Boolean(normalizedFull) && !looksLikePDFGarbage;
  const panelTitle = article ? article.title || "(无标题)" : "请选择一篇文章查看详情";
  const hasTranslation = translationParagraphs.length > 0 || isTranslatingArticle;
  const showReadableContent = hasUsableFullContent && !hasTranslation;
  const hasSummaryCard = Boolean((sanitizedSummaryHTML || "").trim());
  const summaryStatusMeta = getSummaryStatusMeta(article?.display_summary_status);

  return (
    <>
      <ArticleDetailTopBar
        title={panelTitle}
        canMarkUnread={canMarkUnread}
        canToggleFavorite={canToggleFavorite}
        isFavorite={isFavorite}
        canOpenSourceSite={canOpenSourceSite}
        canExtractReadable={canExtractReadable}
        isExtractingReadable={isExtractingReadable}
        canRefreshArticleCache={canRefreshArticleCache}
        isRefreshingArticleCache={isRefreshingArticleCache}
        hasReadableContent={hasUsableFullContent}
        sourceSiteURL={sourceSiteURL}
        contextText={detailProgressText}
        recommendationText={article ? `推荐分：${formatRecommendationSummary(article)}` : undefined}
        onMarkUnread={onMarkUnread}
        onToggleFavorite={onToggleFavorite}
        onOpenSourceSite={onOpenSourceSite}
        onExtractReadable={onExtractReadable}
        onRefreshArticleCache={onRefreshArticleCache}
      />

      <div
        ref={detailRef}
        className="flex-1 min-h-0 overflow-y-auto px-5 py-4"
      >
        {!article && (
          <p className="text-sm text-muted-foreground text-center mt-16">请选择一篇文章查看详情</p>
        )}

        {article && (
          <div className="max-w-prose mx-auto">
            {/* Navigation row */}
            <div className="flex items-center justify-between gap-3 mb-4" aria-label="文章顺序导航">
              <button
                className="text-sm text-muted-foreground hover:text-foreground disabled:opacity-40 transition-colors"
                onClick={onGoPrev}
                disabled={!canGoPrev}
              >
                ← 上一篇
              </button>
              <span className="text-xs text-muted-foreground">{detailProgressText || "第 - / - 条"}</span>
              <button
                className="text-sm text-muted-foreground hover:text-foreground disabled:opacity-40 transition-colors"
                onClick={onGoNext}
                disabled={!canGoNext}
              >
                下一篇 →
              </button>
            </div>

            {/* Meta row */}
            <div className="flex items-center gap-3 mb-3 text-xs text-muted-foreground flex-wrap">
              <span>🗓 {article.published_at || "-"}</span>
              {article.is_read ? (
                <Badge variant="read">已读</Badge>
              ) : (
                <Badge variant="unread">未读</Badge>
              )}
              {article.link && (
                <a
                  href={article.link}
                  target="_blank"
                  rel="noreferrer"
                  className="text-primary underline underline-offset-2 truncate max-w-[240px]"
                >
                  {article.link}
                </a>
              )}
            </div>

            {/* AI Summary card */}
            {hasSummaryCard && (
              <section
                className="mb-4 rounded-lg border bg-muted/30 overflow-hidden"
                aria-label="文章摘要"
              >
                <div className="flex items-center justify-between px-4 py-2.5 border-b bg-background/60">
                  <h4 className="text-sm font-semibold">文章摘要</h4>
                  <Badge
                    variant={summaryStatusMeta.tone === "ready" ? "unread" : "secondary"}
                    className="text-xs"
                  >
                    {summaryStatusMeta.label}
                  </Badge>
                </div>
                <div
                  className="px-4 py-3 prose-article text-sm"
                  dangerouslySetInnerHTML={{ __html: sanitizedSummaryHTML }}
                />
              </section>
            )}

            {/* Body */}
            <h4 className="text-sm font-semibold text-muted-foreground mb-3">
              {hasTranslation ? "正文（原文 / 译文）" : "正文"}
            </h4>

            {hasTranslation ? (
              <div className="space-y-4">
                {translationParagraphs.map((item) => (
                  <div key={item.index} className="space-y-1">
                    <p className="text-sm text-muted-foreground leading-relaxed">
                      {item.source || "(原文段落加载中...)"}
                    </p>
                    {item.status === "done" ? (
                      <p className="text-sm leading-relaxed border-l-2 border-primary/40 pl-3">
                        {item.translated}
                      </p>
                    ) : (
                      <div className="flex items-center gap-2 text-xs text-muted-foreground" aria-live="polite">
                        <span className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" aria-hidden="true" />
                        <span>第 {item.index} 段翻译中...</span>
                      </div>
                    )}
                  </div>
                ))}
                {isTranslatingArticle && translationParagraphs.length === 0 && (
                  <div className="flex items-center gap-2 text-xs text-muted-foreground" aria-live="polite">
                    <span className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" aria-hidden="true" />
                    <span>正在拆分段落并启动翻译...</span>
                  </div>
                )}
              </div>
            ) : showReadableContent ? (
              <div
                className="prose-article"
                dangerouslySetInnerHTML={{ __html: sanitizedFullContentHTML }}
              />
            ) : hasSummaryCard ? (
              <p className={cn("text-sm text-muted-foreground italic")}>
                上方已展示摘要；点击工具栏中的正文按钮后，会把抓取到的正文直接接在这里继续阅读。
              </p>
            ) : (
              <p className="text-sm text-muted-foreground italic">(无摘要)</p>
            )}
          </div>
        )}
      </div>

      {article && (
        <ArticleFloatingActions
          onPrev={onGoPrev}
          onNext={onGoNext}
          canGoPrev={canGoPrev}
          canGoNext={canGoNext}
          onScrollTop={() => {
            detailRef.current?.scrollTo({ top: 0, behavior: "smooth" });
          }}
          onTranslate={onTranslateArticle}
          isTranslating={isTranslatingArticle}
        />
      )}
    </>
  );
}
