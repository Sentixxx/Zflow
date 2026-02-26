import { ArticleDetailToolbar } from "./ArticleDetailToolbar";

type ArticleDetailTopBarProps = {
  title: string;
  canMarkUnread: boolean;
  canOpenSourceSite: boolean;
  canExtractReadable: boolean;
  isExtractingReadable: boolean;
  sourceSiteURL: string;
  onMarkUnread: () => void;
  onOpenSourceSite: () => void;
  onExtractReadable: () => void;
};

export function ArticleDetailTopBar({
  title,
  canMarkUnread,
  canOpenSourceSite,
  canExtractReadable,
  isExtractingReadable,
  sourceSiteURL,
  onMarkUnread,
  onOpenSourceSite,
  onExtractReadable,
}: ArticleDetailTopBarProps) {
  return (
    <div className="detail-panel-head">
      <h2 className="detail-panel-title">{title}</h2>
      <ArticleDetailToolbar
        canMarkUnread={canMarkUnread}
        canOpenSourceSite={canOpenSourceSite}
        canExtractReadable={canExtractReadable}
        isExtractingReadable={isExtractingReadable}
        sourceSiteURL={sourceSiteURL}
        onMarkUnread={onMarkUnread}
        onOpenSourceSite={onOpenSourceSite}
        onExtractReadable={onExtractReadable}
      />
    </div>
  );
}
