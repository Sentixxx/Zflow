import type { Article } from "@/types";
import { useMemo, useRef } from "react";
import { formatRecommendationSummary } from "@/lib/article-list";
import { renderTranslatedHTML, splitTranslatedTextBlocks } from "@/lib/translation";
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
  translationTemplateHTML: string;
  canMarkUnread: boolean;
  canToggleFavorite: boolean;
  isFavorite: boolean;
  canOpenSourceSite: boolean;
  canExtractReadable: boolean;
  isExtractingReadable: boolean;
  canRefreshArticleCache: boolean;
  isRefreshingArticleCache: boolean;
  isTranslatingArticle: boolean;
  isTranslationVisible: boolean;
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
  translationTemplateHTML,
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
  isTranslationVisible,
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
  const hasTranslation = translationParagraphs.length > 0;
  const showTranslation = isTranslationVisible;
  const showReadableContent = hasUsableFullContent && !showTranslation;
  const hasSummaryCard = Boolean((sanitizedSummaryHTML || "").trim());
  const summaryStatusMeta = getSummaryStatusMeta(article?.display_summary_status);
  const renderedTranslatedHTML = useMemo(
    () => renderTranslatedHTML(translationTemplateHTML, translationParagraphs, isTranslatingArticle),
    [translationTemplateHTML, translationParagraphs, isTranslatingArticle],
  );

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
        className="article-scroll-pane flex-1 min-h-0 overflow-y-auto"
      >
        {!article && (
          <p className="text-sm text-muted-foreground text-center mt-20">请选择一篇文章查看详情</p>
        )}

        {article && (
          <div className="article-content-wrapper">
            {/* Meta row */}
            <div className="flex items-center gap-2.5 mb-5 text-xs text-muted-foreground flex-wrap">
              <span>{article.published_at || "-"}</span>
              <span className="text-border">·</span>
              {article.is_read ? (
                <Badge variant="read">已读</Badge>
              ) : (
                <Badge variant="unread">未读</Badge>
              )}
              {article.link && (
                <>
                  <span className="text-border">·</span>
                  <a
                    href={article.link}
                    target="_blank"
                    rel="noreferrer"
                    className="text-primary/80 hover:text-primary underline underline-offset-2 truncate max-w-[260px] transition-colors"
                  >
                    {article.link}
                  </a>
                </>
              )}
            </div>

            {/* AI Summary card */}
            {hasSummaryCard && (
              <section
                className="mb-7 rounded-xl border border-border/70 bg-muted/20 overflow-hidden"
                aria-label="文章摘要"
              >
                <div className="flex items-center justify-between px-5 py-3 border-b border-border/50">
                  <span className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">摘要</span>
                  <Badge
                    variant={summaryStatusMeta.tone === "ready" ? "unread" : "secondary"}
                  >
                    {summaryStatusMeta.label}
                  </Badge>
                </div>
                <div
                  className="px-5 py-4 prose-article [&_p]:text-[15px] [&_p]:leading-[1.8]"
                  dangerouslySetInnerHTML={{ __html: sanitizedSummaryHTML }}
                />
              </section>
            )}

            {/* Body section divider */}
            {(showReadableContent || showTranslation || hasSummaryCard) && (
              <div className="flex items-center gap-3 mb-5">
                <span className="text-xs font-semibold uppercase tracking-widest text-muted-foreground whitespace-nowrap">
                  {showTranslation ? "原文 / 译文" : "正文"}
                </span>
                <div className="flex-1 h-px bg-border/60" />
              </div>
            )}

            {/* Body content */}
            {showTranslation && renderedTranslatedHTML ? (
              <div
                className="prose-article"
                dangerouslySetInnerHTML={{ __html: renderedTranslatedHTML }}
              />
            ) : showTranslation ? (
              <div className="space-y-6">
                {translationParagraphs.map((item) => (
                  <div key={item.index} className="space-y-2">
                    <p className="text-[15px] leading-[1.85] text-muted-foreground break-words [overflow-wrap:anywhere]">
                      {item.source || "(原文段落加载中...)"}
                    </p>
                    {item.status === "done" ? (
                      <div className="space-y-3 border-l-2 border-primary/50 pl-4">
                        {splitTranslatedTextBlocks(item.translated).map((block, blockIndex) => (
                          <div
                            key={`${item.index}-${blockIndex}`}
                            className={cn(
                              "text-base leading-[1.85] whitespace-pre-wrap break-words [overflow-wrap:anywhere]",
                              blockIndex > 0 && "pt-1",
                            )}
                          >
                            {block}
                          </div>
                        ))}
                      </div>
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
              <p className="text-sm text-muted-foreground italic">
                点击工具栏「抓取正文」后，全文将展示于此处。
              </p>
            ) : (
              <p className="text-sm text-muted-foreground italic">(暂无内容)</p>
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
          hasTranslation={hasTranslation}
          isTranslationVisible={isTranslationVisible}
        />
      )}
    </>
  );
}
