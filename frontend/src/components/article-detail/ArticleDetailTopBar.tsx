import { ArticleDetailToolbar } from "./ArticleDetailToolbar";

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
  canToggleReadableMode: boolean;
  readableModeEnabled: boolean;
  sourceSiteURL: string;
  contextText?: string;
  onMarkUnread: () => void;
  onToggleFavorite: () => void;
  onOpenSourceSite: () => void;
  onExtractReadable: () => void;
  onRefreshArticleCache: () => void;
  onToggleReadableMode: () => void;
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
  canToggleReadableMode,
  readableModeEnabled,
  sourceSiteURL,
  contextText,
  onMarkUnread,
  onToggleFavorite,
  onOpenSourceSite,
  onExtractReadable,
  onRefreshArticleCache,
  onToggleReadableMode,
}: ArticleDetailTopBarProps) {
  return (
    <div className="detail-panel-head">
      <div className="detail-panel-title-wrap">
        <h2 className="detail-panel-title">{title}</h2>
        {contextText && <p className="detail-panel-context">{contextText}</p>}
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
        canToggleReadableMode={canToggleReadableMode}
        readableModeEnabled={readableModeEnabled}
        sourceSiteURL={sourceSiteURL}
        onMarkUnread={onMarkUnread}
        onToggleFavorite={onToggleFavorite}
        onOpenSourceSite={onOpenSourceSite}
        onExtractReadable={onExtractReadable}
        onRefreshArticleCache={onRefreshArticleCache}
        onToggleReadableMode={onToggleReadableMode}
      />
    </div>
  );
}
