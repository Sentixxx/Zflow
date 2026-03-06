import type { Article } from "@/types";
import { useEffect, useRef, useState } from "react";
import { ArticleDetailTopBar } from "./ArticleDetailTopBar";
import { ArticleFloatingActions } from "./ArticleFloatingActions";

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
  const [readableModeEnabled, setReadableModeEnabled] = useState<boolean>(false);
  const normalizedFull = (sanitizedFullContentHTML || "").trim();
  const looksLikePDFGarbage = /^%PDF-\d/i.test(normalizedFull) || (normalizedFull.includes("xref") && normalizedFull.includes("endobj"));
  const hasUsableFullContent = Boolean(normalizedFull) && !looksLikePDFGarbage;
  const contentClassName = hasUsableFullContent ? "detail-summary detail-readable" : "detail-summary";
  const panelTitle = article ? article.title || "(无标题)" : "请选择一篇文章查看详情";
  const hasTranslation = translationParagraphs.length > 0 || isTranslatingArticle;
  const showReadableContent = hasUsableFullContent && readableModeEnabled && !hasTranslation;

  useEffect(() => {
    if (!article) {
      setReadableModeEnabled(false);
      return;
    }
    setReadableModeEnabled(hasUsableFullContent);
  }, [article?.id, hasUsableFullContent]);

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
        canToggleReadableMode={hasUsableFullContent}
        readableModeEnabled={readableModeEnabled}
        sourceSiteURL={sourceSiteURL}
        contextText={detailProgressText}
        onMarkUnread={onMarkUnread}
        onToggleFavorite={onToggleFavorite}
        onOpenSourceSite={onOpenSourceSite}
        onExtractReadable={onExtractReadable}
        onRefreshArticleCache={onRefreshArticleCache}
        onToggleReadableMode={() => setReadableModeEnabled((current) => !current)}
      />
      <div className="detail" ref={detailRef}>
        {!article && <p className="detail-empty">请选择一篇文章查看详情</p>}
        {article && (
          <>
            <div className="detail-sequence-nav" aria-label="文章顺序导航">
              <button className="detail-sequence-btn" onClick={onGoPrev} disabled={!canGoPrev}>
                上一篇
              </button>
              <span className="detail-sequence-progress">{detailProgressText || "第 - / - 条"}</span>
              <button className="detail-sequence-btn" onClick={onGoNext} disabled={!canGoNext}>
                下一篇
              </button>
            </div>
            <p className="meta-row article-meta">
              <span>🗓 {article.published_at || "-"}</span>
              <span>{article.is_read ? "已读" : "未读"}</span>
            </p>
            <p className="meta detail-link">
              链接：
              {article.link ? (
                <a href={article.link} target="_blank" rel="noreferrer">
                  {article.link}
                </a>
              ) : (
                "-"
              )}
            </p>
            <h4 className="detail-section-title">{hasTranslation ? "正文（原文 / 译文）" : showReadableContent ? "正文" : "摘要"}</h4>
            {hasTranslation ? (
              <div className="detail-summary detail-translation-inline">
                {translationParagraphs.map((item) => (
                  <div className="translation-inline-block" key={item.index}>
                    <p className="translation-source-inline">{item.source || "(原文段落加载中...)"}</p>
                    {item.status === "done" ? (
                      <p className="translation-target-inline">{item.translated}</p>
                    ) : (
                      <div className="translation-pending" aria-live="polite">
                        <span className="translation-loading-dot" aria-hidden="true" />
                        <span>第 {item.index} 段翻译中...</span>
                      </div>
                    )}
                  </div>
                ))}
                {isTranslatingArticle && translationParagraphs.length === 0 && (
                  <div className="translation-pending" aria-live="polite">
                    <span className="translation-loading-dot" aria-hidden="true" />
                    <span>正在拆分段落并启动翻译...</span>
                  </div>
                )}
              </div>
            ) : showReadableContent ? (
              <div className={contentClassName} dangerouslySetInnerHTML={{ __html: sanitizedFullContentHTML }} />
            ) : sanitizedSummaryHTML ? (
              <div className={contentClassName} dangerouslySetInnerHTML={{ __html: sanitizedSummaryHTML }} />
            ) : (
              <p className="detail-summary">(无摘要)</p>
            )}
          </>
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
