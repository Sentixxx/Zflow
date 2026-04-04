import { useRef } from "react";
import type { UIEvent } from "react";
import type { Article, Feed } from "@/types";
import { formatArticleTime, formatRecommendationSummary } from "@/lib/article-list";
import { RssFallbackIcon } from "@/components/ui/RssFallbackIcon";
import { useVirtualizer } from "@tanstack/react-virtual";

type ArticleListProps = {
  articles: Article[];
  selectedArticleID: number | null;
  feedByID: Map<number, Feed>;
  feedNameByID: Map<number, string>;
  apiBase: string;
  listBounce: boolean;
  onScroll: (event: UIEvent<HTMLDivElement>) => void;
  onSelectArticle: (id: number) => void;
};

export function ArticleList({
  articles,
  selectedArticleID,
  feedByID,
  feedNameByID,
  apiBase,
  listBounce,
  onScroll,
  onSelectArticle,
}: ArticleListProps) {
  if (articles.length === 0) {
    return <div className="item">暂无文章</div>;
  }

  const parentRef = useRef<HTMLDivElement | null>(null);
  const rowVirtualizer = useVirtualizer({
    count: articles.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 96,
    overscan: 8,
  });

  return (
    <div
      ref={parentRef}
      className={`list article-list ${listBounce ? "bounce" : ""}`}
      onScroll={onScroll}
      data-virtualized="true"
    >
      <div style={{ height: rowVirtualizer.getTotalSize(), position: "relative" }}>
        {rowVirtualizer.getVirtualItems().map((virtualRow) => {
          const article = articles[virtualRow.index];
          const sourceFeed = feedByID.get(article.feed_id);
          const iconSrc = sourceFeed?.icon_url ? `${apiBase.replace(/\/$/, "")}${sourceFeed.icon_url}` : "";
          return (
            <button
              key={article.id}
              ref={(node) => {
                if (node) {
                  rowVirtualizer.measureElement(node);
                }
              }}
              data-index={virtualRow.index}
              className={`item article ${selectedArticleID === article.id ? "active" : ""}`}
              onClick={() => onSelectArticle(article.id)}
              title={article.is_read ? "已读文章，点击查看详情" : "未读文章，点击查看并自动标记已读"}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                transform: `translateY(${virtualRow.start}px)`,
              }}
            >
              <div className="article-source">
                {iconSrc ? (
                  <>
                    <img
                      className="feed-icon article-source-icon"
                      src={iconSrc}
                      alt=""
                      loading="lazy"
                      onError={(event) => {
                        event.currentTarget.style.display = "none";
                        const fallback = event.currentTarget.nextElementSibling as HTMLElement | null;
                        if (fallback) {
                          fallback.style.display = "inline-flex";
                        }
                      }}
                    />
                    <span className="feed-icon-fallback article-source-icon-fallback" style={{ display: "none" }} aria-hidden="true">
                      <RssFallbackIcon />
                    </span>
                  </>
                ) : (
                  <span className="feed-icon-fallback article-source-icon-fallback" aria-hidden="true">
                    <RssFallbackIcon />
                  </span>
                )}
                <span>{feedNameByID.get(article.feed_id) || `订阅源 #${article.feed_id}`}</span>
              </div>
              <div>
                <strong className="article-title">
                  {article.is_favorite && <span className="article-favorite-star" aria-hidden="true">☆</span>}
                  {article.title || "(无标题)"}
                </strong>
                <span className={`pill ${article.is_read ? "read" : "unread"}`} title={article.is_read ? "已读状态" : "未读状态"}>
                  {article.is_read ? "已读" : "未读"}
                </span>
              </div>
              <div className="meta article-meta-line">
                <span>{formatArticleTime(article.published_at || article.created_at)}</span>
                <span className="article-score-summary">{formatRecommendationSummary(article)}</span>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}
