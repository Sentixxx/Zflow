import { useRef } from "react";
import type { UIEvent } from "react";
import type { Article, Feed } from "@/types";
import { formatArticleTime, formatRecommendationSummary } from "@/lib/article-list";
import { RssFallbackIcon } from "@/components/ui/RssFallbackIcon";
import { Badge } from "@/components/ui/badge";
import { useVirtualizer } from "@tanstack/react-virtual";
import { cn } from "@/lib/utils";

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
  const parentRef = useRef<HTMLDivElement | null>(null);
  const rowVirtualizer = useVirtualizer({
    count: articles.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 96,
    overscan: 8,
  });

  if (articles.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-sm text-muted-foreground p-4">
        暂无文章
      </div>
    );
  }

  return (
    <div
      ref={parentRef}
      className={cn("flex-1 min-h-0 overflow-auto", listBounce && "animate-[bounce_0.15s_ease-out]")}
      onScroll={onScroll}
      data-virtualized="true"
    >
      <div style={{ height: rowVirtualizer.getTotalSize(), position: "relative" }}>
        {rowVirtualizer.getVirtualItems().map((virtualRow) => {
          const article = articles[virtualRow.index];
          const sourceFeed = feedByID.get(article.feed_id);
          const iconSrc = sourceFeed?.icon_url ? `${apiBase.replace(/\/$/, "")}${sourceFeed.icon_url}` : "";
          const isSelected = selectedArticleID === article.id;

          return (
            <button
              key={article.id}
              ref={(node) => {
                if (node) rowVirtualizer.measureElement(node);
              }}
              data-index={virtualRow.index}
              onClick={() => onSelectArticle(article.id)}
              title={article.is_read ? "已读文章，点击查看详情" : "未读文章，点击查看并自动标记已读"}
              className={cn(
                "w-full text-left px-3 py-3 border-b border-border transition-colors cursor-pointer",
                "bg-transparent hover:bg-muted/50",
                isSelected && "bg-accent border-l-[3px] border-l-primary",
                !article.is_read && !isSelected && "border-l-[3px] border-l-primary/30"
              )}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                transform: `translateY(${virtualRow.start}px)`,
              }}
            >
              {/* Source row */}
              <div className="flex items-center gap-1.5 mb-1">
                {iconSrc ? (
                  <>
                    <img
                      className="w-3.5 h-3.5 rounded-sm object-cover flex-shrink-0"
                      src={iconSrc}
                      alt=""
                      loading="lazy"
                      onError={(event) => {
                        event.currentTarget.style.display = "none";
                        const fallback = event.currentTarget.nextElementSibling as HTMLElement | null;
                        if (fallback) fallback.style.display = "inline-flex";
                      }}
                    />
                    <span className="hidden w-4 h-4 text-orange-500 flex-shrink-0" aria-hidden="true">
                      <RssFallbackIcon />
                    </span>
                  </>
                ) : (
                  <span className="inline-flex w-4 h-4 text-orange-500 flex-shrink-0" aria-hidden="true">
                    <RssFallbackIcon />
                  </span>
                )}
                <span className="text-xs text-muted-foreground truncate">
                  {feedNameByID.get(article.feed_id) || `订阅源 #${article.feed_id}`}
                </span>
              </div>

              {/* Title row */}
              <div className="flex items-start gap-2 mb-1">
                <span className={cn("text-sm font-medium leading-snug flex-1 min-w-0", article.is_read && "text-muted-foreground")}>
                  {article.is_favorite && <span className="mr-1 text-amber-400" aria-hidden="true">☆</span>}
                  {article.title || "(无标题)"}
                </span>
                {!article.is_read && (
                  <Badge variant="unread" className="flex-shrink-0 mt-0.5">未读</Badge>
                )}
              </div>

              {/* Meta row */}
              <div className="flex items-center justify-between text-xs text-muted-foreground">
                <span>{formatArticleTime(article.published_at || article.created_at)}</span>
                <span className="text-primary/80">{formatRecommendationSummary(article)}</span>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}
