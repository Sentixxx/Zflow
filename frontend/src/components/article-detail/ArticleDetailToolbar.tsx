import { useEffect, useRef, useState } from "react";
import sourceSiteIcon from "../../assets/source-site.svg";
import readabilityIcon from "../../assets/readability.svg";
import markUnreadIcon from "../../assets/mark-unread.svg";
import { ToolbarIconButton } from "@/components/ui/ToolbarIconButton";

type ArticleDetailToolbarProps = {
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
  onMarkUnread: () => void;
  onToggleFavorite: () => void;
  onOpenSourceSite: () => void;
  onExtractReadable: () => void;
  onRefreshArticleCache: () => void;
  onToggleReadableMode: () => void;
};

export function ArticleDetailToolbar({
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
  onMarkUnread,
  onToggleFavorite,
  onOpenSourceSite,
  onExtractReadable,
  onRefreshArticleCache,
  onToggleReadableMode,
}: ArticleDetailToolbarProps) {
  const [moreOpen, setMoreOpen] = useState(false);
  const moreRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!moreOpen) {
      return;
    }
    const onPointerDown = (event: MouseEvent) => {
      if (!moreRef.current) {
        return;
      }
      if (!moreRef.current.contains(event.target as Node)) {
        setMoreOpen(false);
      }
    };
    window.addEventListener("mousedown", onPointerDown);
    return () => window.removeEventListener("mousedown", onPointerDown);
  }, [moreOpen]);

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
        onClick={onToggleFavorite}
        disabled={!canToggleFavorite}
        title={isFavorite ? "取消收藏" : "收藏文章"}
        ariaLabel={isFavorite ? "取消收藏" : "收藏文章"}
      >
        <span className="detail-icon-slot detail-star-slot" aria-hidden="true">
          {isFavorite ? "★" : "☆"}
        </span>
      </ToolbarIconButton>
      <ToolbarIconButton
        onClick={canToggleReadableMode ? onToggleReadableMode : onExtractReadable}
        disabled={isExtractingReadable || (!canToggleReadableMode && !canExtractReadable)}
        title={
          isExtractingReadable
            ? "正在抓取原文..."
            : canToggleReadableMode
              ? readableModeEnabled
                ? "退出阅读模式"
                : "进入阅读模式"
              : canExtractReadable
                ? "使用 Readability 抓取原文"
                : "当前文章缺少可用链接"
        }
        ariaLabel={canToggleReadableMode ? "切换阅读模式" : "抓取原文"}
      >
        <span className="detail-icon-slot" aria-hidden="true">
          <img className="detail-icon-image icon-readability" src={readabilityIcon} alt="" />
        </span>
      </ToolbarIconButton>
      <ToolbarIconButton
        onClick={onOpenSourceSite}
        disabled={!canOpenSourceSite}
        title={canOpenSourceSite ? `在新标签页打开原文：${sourceSiteURL}` : "当前文章缺少可用链接"}
        ariaLabel="打开原文链接"
      >
        <span className="detail-icon-slot" aria-hidden="true">
          <img className="detail-icon-image icon-source-site" src={sourceSiteIcon} alt="" />
        </span>
      </ToolbarIconButton>
      <div className="detail-toolbar-more" ref={moreRef}>
        <ToolbarIconButton onClick={() => setMoreOpen((current) => !current)} title="更多工具" ariaLabel="更多工具">
          <span className="detail-icon-slot" aria-hidden="true">
            <span className="detail-more-glyph">...</span>
          </span>
        </ToolbarIconButton>
        {moreOpen && (
          <div className="detail-toolbar-more-menu" role="menu" aria-label="更多文章工具">
            <button
              className="detail-toolbar-menu-item"
              type="button"
              onClick={() => {
                onRefreshArticleCache();
                setMoreOpen(false);
              }}
              disabled={!canRefreshArticleCache || isRefreshingArticleCache}
              title={isRefreshingArticleCache ? "正在刷新缓存..." : "刷新当前文章缓存"}
            >
              {isRefreshingArticleCache ? "刷新中..." : "刷新当前文章缓存"}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
