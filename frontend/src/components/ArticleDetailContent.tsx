import type { Article } from "../types";
import { ArticleDetailTopBar } from "./ArticleDetailTopBar";

type ArticleDetailContentProps = {
  article: Article | null;
  sanitizedSummaryHTML: string;
  sanitizedFullContentHTML: string;
  canMarkUnread: boolean;
  canOpenSourceSite: boolean;
  canExtractReadable: boolean;
  isExtractingReadable: boolean;
  sourceSiteURL: string;
  onMarkUnread: () => void;
  onOpenSourceSite: () => void;
  onExtractReadable: () => void;
};

export function ArticleDetailContent({
  article,
  sanitizedSummaryHTML,
  sanitizedFullContentHTML,
  canMarkUnread,
  canOpenSourceSite,
  canExtractReadable,
  isExtractingReadable,
  sourceSiteURL,
  onMarkUnread,
  onOpenSourceSite,
  onExtractReadable,
}: ArticleDetailContentProps) {
  return (
    <>
      <ArticleDetailTopBar
        canMarkUnread={canMarkUnread}
        canOpenSourceSite={canOpenSourceSite}
        canExtractReadable={canExtractReadable}
        isExtractingReadable={isExtractingReadable}
        sourceSiteURL={sourceSiteURL}
        onMarkUnread={onMarkUnread}
        onOpenSourceSite={onOpenSourceSite}
        onExtractReadable={onExtractReadable}
      />
      <div className="detail">
        {!article && <p className="detail-empty">请选择一篇文章查看详情</p>}
        {article && (
          <>
            <h3 className="detail-title">{article.title || "(无标题)"}</h3>
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
            <h4 className="detail-section-title">{sanitizedFullContentHTML ? "正文" : "摘要"}</h4>
            {sanitizedFullContentHTML ? (
              <div className="detail-summary" dangerouslySetInnerHTML={{ __html: sanitizedFullContentHTML }} />
            ) : sanitizedSummaryHTML ? (
              <div className="detail-summary" dangerouslySetInnerHTML={{ __html: sanitizedSummaryHTML }} />
            ) : (
              <p className="detail-summary">(无摘要)</p>
            )}
          </>
        )}
      </div>
    </>
  );
}
