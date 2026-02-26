import { ArticleDetailToolbar } from "./ArticleDetailToolbar";

type ArticleDetailTopBarProps = {
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
      <h2>文章内容</h2>
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
