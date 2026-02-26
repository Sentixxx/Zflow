import { ArticleDetailToolbar } from "./ArticleDetailToolbar";

type ArticleDetailTopBarProps = {
  title: string;
  canMarkUnread: boolean;
  canOpenSourceSite: boolean;
  canExtractReadable: boolean;
  isExtractingReadable: boolean;
  canRefreshArticleCache: boolean;
  isRefreshingArticleCache: boolean;
  canToggleReadableMode: boolean;
  readableModeEnabled: boolean;
  sourceSiteURL: string;
  onMarkUnread: () => void;
  onOpenSourceSite: () => void;
  onExtractReadable: () => void;
  onRefreshArticleCache: () => void;
  onToggleReadableMode: () => void;
};

export function ArticleDetailTopBar({
  title,
  canMarkUnread,
  canOpenSourceSite,
  canExtractReadable,
  isExtractingReadable,
  canRefreshArticleCache,
  isRefreshingArticleCache,
  canToggleReadableMode,
  readableModeEnabled,
  sourceSiteURL,
  onMarkUnread,
  onOpenSourceSite,
  onExtractReadable,
  onRefreshArticleCache,
  onToggleReadableMode,
}: ArticleDetailTopBarProps) {
  return (
    <div className="detail-panel-head">
      <h2 className="detail-panel-title">{title}</h2>
      <ArticleDetailToolbar
        canMarkUnread={canMarkUnread}
        canOpenSourceSite={canOpenSourceSite}
        canExtractReadable={canExtractReadable}
        isExtractingReadable={isExtractingReadable}
        canRefreshArticleCache={canRefreshArticleCache}
        isRefreshingArticleCache={isRefreshingArticleCache}
        canToggleReadableMode={canToggleReadableMode}
        readableModeEnabled={readableModeEnabled}
        sourceSiteURL={sourceSiteURL}
        onMarkUnread={onMarkUnread}
        onOpenSourceSite={onOpenSourceSite}
        onExtractReadable={onExtractReadable}
        onRefreshArticleCache={onRefreshArticleCache}
        onToggleReadableMode={onToggleReadableMode}
      />
    </div>
  );
}
