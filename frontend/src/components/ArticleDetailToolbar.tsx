import sourceSiteIcon from "../assets/source-site.svg";
import readabilityIcon from "../assets/readability.svg";
import markUnreadIcon from "../assets/mark-unread.svg";
import { ToolbarIconButton } from "./ToolbarIconButton";

type ArticleDetailToolbarProps = {
  canMarkUnread: boolean;
  canOpenSourceSite: boolean;
  canExtractReadable: boolean;
  isExtractingReadable: boolean;
  sourceSiteURL: string;
  onMarkUnread: () => void;
  onOpenSourceSite: () => void;
  onExtractReadable: () => void;
};

export function ArticleDetailToolbar({
  canMarkUnread,
  canOpenSourceSite,
  canExtractReadable,
  isExtractingReadable,
  sourceSiteURL,
  onMarkUnread,
  onOpenSourceSite,
  onExtractReadable,
}: ArticleDetailToolbarProps) {
  return (
    <div className="detail-toolbar">
      <ToolbarIconButton
        onClick={onMarkUnread}
        disabled={!canMarkUnread}
        title={canMarkUnread ? "标记为未读" : "当前已是未读"}
        ariaLabel="标记未读"
      >
        <span className="detail-icon-slot" aria-hidden="true">
          <img className="detail-icon-image icon-mark-unread" src={markUnreadIcon} alt="" />
        </span>
      </ToolbarIconButton>
      <ToolbarIconButton
        onClick={onExtractReadable}
        disabled={!canExtractReadable || isExtractingReadable}
        title={
          isExtractingReadable
            ? "正在抓取原文..."
            : canExtractReadable
              ? "使用 Readability 抓取原文"
              : "当前文章缺少可用链接"
        }
        ariaLabel="抓取原文"
      >
        <span className="detail-icon-slot" aria-hidden="true">
          <img className="detail-icon-image icon-readability" src={readabilityIcon} alt="" />
        </span>
      </ToolbarIconButton>
      <ToolbarIconButton
        onClick={onOpenSourceSite}
        disabled={!canOpenSourceSite}
        title={canOpenSourceSite ? `在新标签页打开来源网站：${sourceSiteURL}` : "当前文章缺少可用来源网站"}
        ariaLabel="打开来源网站"
      >
        <span className="detail-icon-slot" aria-hidden="true">
          <img className="detail-icon-image icon-source-site" src={sourceSiteIcon} alt="" />
        </span>
      </ToolbarIconButton>
    </div>
  );
}
