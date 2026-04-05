import { ArticleDetailToolbar } from "./ArticleDetailToolbar";
import { Separator } from "@/components/ui/separator";

type ArticleDetailTopBarProps = {
  title: string;
  canMarkUnread: boolean;
  canToggleFavorite: boolean;
  isFavorite: boolean;
  canOpenSourceSite: boolean;
  canExtractReadable: boolean;
  isExtractingReadable: boolean;
  canRefreshArticleCache: boolean;
  isRefreshingArticleCache: boolean;
  hasReadableContent: boolean;
  sourceSiteURL: string;
  contextText?: string;
  recommendationText?: string;
  onMarkUnread: () => void;
  onToggleFavorite: () => void;
  onOpenSourceSite: () => void;
  onExtractReadable: () => void;
  onRefreshArticleCache: () => void;
};

export function ArticleDetailTopBar({
  title,
  canMarkUnread,
  canToggleFavorite,
  isFavorite,
  canOpenSourceSite,
  canExtractReadable,
  isExtractingReadable,
  canRefreshArticleCache,
  isRefreshingArticleCache,
  hasReadableContent,
  sourceSiteURL,
  contextText,
  recommendationText,
  onMarkUnread,
  onToggleFavorite,
  onOpenSourceSite,
  onExtractReadable,
  onRefreshArticleCache,
}: ArticleDetailTopBarProps) {
  return (
    <>
      <div className="flex items-start justify-between gap-3 px-4 pt-3 pb-2">
        <div className="min-w-0 flex-1">
          <h2 className="text-base font-semibold leading-snug truncate">{title}</h2>
          {contextText && (
            <p className="text-xs text-muted-foreground mt-0.5">{contextText}</p>
          )}
          {recommendationText && (
            <p className="text-xs text-primary/80 mt-0.5">{recommendationText}</p>
          )}
        </div>
        <ArticleDetailToolbar
          canMarkUnread={canMarkUnread}
          canToggleFavorite={canToggleFavorite}
          isFavorite={isFavorite}
          canOpenSourceSite={canOpenSourceSite}
          canExtractReadable={canExtractReadable}
          isExtractingReadable={isExtractingReadable}
          canRefreshArticleCache={canRefreshArticleCache}
          isRefreshingArticleCache={isRefreshingArticleCache}
          hasReadableContent={hasReadableContent}
          sourceSiteURL={sourceSiteURL}
          onMarkUnread={onMarkUnread}
          onToggleFavorite={onToggleFavorite}
          onOpenSourceSite={onOpenSourceSite}
          onExtractReadable={onExtractReadable}
          onRefreshArticleCache={onRefreshArticleCache}
        />
      </div>
      <Separator />
    </>
  );
}
