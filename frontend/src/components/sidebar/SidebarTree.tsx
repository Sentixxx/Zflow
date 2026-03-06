import type { Article, Feed, Folder } from "@/types";
import { feedHost } from "@/lib/feed-utils";
import { RssFallbackIcon } from "@/components/ui";

type SidebarMode = "subscriptions" | "favorites";

type SidebarTreeProps = {
  sidebarMode: SidebarMode;
  selectedFeedID: number | null;
  selectedFolderID: number | null;
  selectedArticleID: number | null;
  rootFolders: Folder[];
  childFoldersByParent: Map<number, Folder[]>;
  feedsByFolder: Map<number, Feed[]>;
  uncategorizedFeeds: Feed[];
  feeds: Feed[];
  favoriteArticles: Article[];
  collapsedFolders: Record<number, boolean>;
  dragOverFolderID: number | null;
  dragOverUncategorized: boolean;
  draggingFeedID: number | null;
  renamingFeedID: number | null;
  renamingFeedTitle: string;
  apiBase: string;
  feedIconURLByHost: Map<string, string>;
  onSwitchSidebarMode: (mode: SidebarMode) => void;
  onCreateRootFolder: () => void;
  onSelectFeed: (feedID: number | null) => void;
  onSelectFolder: (folderID: number) => void;
  onSelectArticle: (articleID: number) => void;
  onToggleFolderCollapsed: (folderID: number) => void;
  onOpenFeedContextMenu: (event: React.MouseEvent, feed: Feed) => void;
  onOpenFolderContextMenu: (event: React.MouseEvent, folder: Folder) => void;
  onFeedDragStart: (event: React.DragEvent<HTMLButtonElement>, feedID: number) => void;
  onFeedDragEnd: () => void;
  onFolderDragOver: (event: React.DragEvent, folderID: number) => void;
  onFolderDragLeave: (folderID: number) => void;
  onFolderDrop: (event: React.DragEvent, folderID: number) => void;
  onUncategorizedDragOver: (event: React.DragEvent) => void;
  onUncategorizedDragLeave: () => void;
  onUncategorizedDrop: (event: React.DragEvent) => void;
  onRenamingFeedTitleChange: (value: string) => void;
  onRenameFeed: (feedID: number) => void;
};

export function SidebarTree({
  sidebarMode,
  selectedFeedID,
  selectedFolderID,
  selectedArticleID,
  rootFolders,
  childFoldersByParent,
  feedsByFolder,
  uncategorizedFeeds,
  feeds,
  favoriteArticles,
  collapsedFolders,
  dragOverFolderID,
  dragOverUncategorized,
  draggingFeedID,
  renamingFeedID,
  renamingFeedTitle,
  apiBase,
  feedIconURLByHost,
  onSwitchSidebarMode,
  onCreateRootFolder,
  onSelectFeed,
  onSelectFolder,
  onSelectArticle,
  onToggleFolderCollapsed,
  onOpenFeedContextMenu,
  onOpenFolderContextMenu,
  onFeedDragStart,
  onFeedDragEnd,
  onFolderDragOver,
  onFolderDragLeave,
  onFolderDrop,
  onUncategorizedDragOver,
  onUncategorizedDragLeave,
  onUncategorizedDrop,
  onRenamingFeedTitleChange,
  onRenameFeed,
}: SidebarTreeProps) {
  const renderFeedNode = (feed: Feed, paddingLeft: number) => {
    const isRenaming = renamingFeedID === feed.id;
    const host = feedHost(feed.url);
    const iconSrc = feed.icon_url ? `${apiBase.replace(/\/$/, "")}${feed.icon_url}` : feedIconURLByHost.get(host) || "";
    return (
      <div key={`feed-${feed.id}`} className={`tree-row feed-row ${isRenaming ? "editing" : ""}`}>
        <button
          className={`item feed-item ${selectedFeedID === feed.id ? "active" : ""} ${draggingFeedID === feed.id ? "dragging" : ""}`}
          style={{ paddingLeft }}
          onClick={() => {
            if (!isRenaming) {
              onSelectFeed(feed.id);
            }
          }}
          draggable={!isRenaming}
          onDragStart={(event) => {
            if (isRenaming) return;
            onFeedDragStart(event, feed.id);
          }}
          onDragEnd={onFeedDragEnd}
        >
          {isRenaming ? (
            <div className="feed-rename-row" onClick={(event) => event.stopPropagation()}>
              <input
                className="feed-rename-input"
                value={renamingFeedTitle}
                onChange={(event) => onRenamingFeedTitleChange(event.target.value)}
                autoFocus
                onBlur={() => {
                  onRenameFeed(feed.id);
                }}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    onRenameFeed(feed.id);
                  }
                }}
              />
            </div>
          ) : (
            <div className="feed-title-row">
              {iconSrc ? (
                <>
                  <img
                    className="feed-icon"
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
                  <span className="feed-icon-fallback" style={{ display: "none" }} aria-hidden="true">
                    <RssFallbackIcon />
                  </span>
                </>
              ) : (
                <span className="feed-icon-fallback" aria-hidden="true">
                  <RssFallbackIcon />
                </span>
              )}
              <strong>{feed.title || "(未命名源)"}</strong>
            </div>
          )}
          <div className="meta">
            {feed.url} · items={feed.item_count} · {feed.last_fetch_status}
            {feed.last_fetch_status === "failed" && feed.last_fetch_error ? ` · 错误: ${feed.last_fetch_error}` : ""}
          </div>
        </button>
        {!isRenaming && (
          <button className="node-action-btn" onClick={(event) => onOpenFeedContextMenu(event, feed)} title="管理订阅源" aria-label={`管理订阅源 ${feed.title || feed.url}`}>
            ⋯
          </button>
        )}
      </div>
    );
  };

  const renderFolderNode = (folder: Folder, depth = 0) => {
    const children = childFoldersByParent.get(folder.id) || [];
    const folderFeeds = feedsByFolder.get(folder.id) || [];
    const hasChildren = children.length > 0 || folderFeeds.length > 0;
    const expanded = !collapsedFolders[folder.id];
    const paddingLeft = 8 + depth * 14;
    return (
      <div key={`folder-${folder.id}`}>
        <div
          className={`tree-row ${selectedFolderID === folder.id ? "active" : ""} ${dragOverFolderID === folder.id ? "drop-target" : ""}`}
          onDragOver={(event) => onFolderDragOver(event, folder.id)}
          onDragLeave={() => onFolderDragLeave(folder.id)}
          onDrop={(event) => onFolderDrop(event, folder.id)}
        >
          <button className={`item folder-item ${selectedFolderID === folder.id ? "active" : ""}`} onClick={() => onSelectFolder(folder.id)} style={{ paddingLeft }}>
            <span
              className={`folder-caret ${expanded ? "expanded" : ""} ${hasChildren ? "" : "disabled"}`}
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
                if (hasChildren) {
                  onToggleFolderCollapsed(folder.id);
                }
              }}
            >
              ▸
            </span>
            <span className="folder-name">{folder.name}</span>
          </button>
          <button className="node-action-btn" onClick={(event) => onOpenFolderContextMenu(event, folder)} title="管理分类" aria-label={`管理分类 ${folder.name}`}>
            ⋯
          </button>
        </div>
        <div className={`folder-children ${expanded ? "expanded" : "collapsed"}`}>
          <div className="folder-children-inner">
            {folderFeeds.map((feed) => renderFeedNode(feed, paddingLeft + 18))}
            {children.map((child) => renderFolderNode(child, depth + 1))}
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="sidebar-content">
      <div className="sidebar-mode-tabs">
        <button className={`sidebar-mode-tab ${sidebarMode === "subscriptions" ? "active" : ""}`} onClick={() => onSwitchSidebarMode("subscriptions")}>
          订阅源
        </button>
        <button className={`sidebar-mode-tab ${sidebarMode === "favorites" ? "active" : ""}`} onClick={() => onSwitchSidebarMode("favorites")}>
          收藏
        </button>
      </div>
      {sidebarMode === "subscriptions" ? (
        <>
          <div className="section-head">
            <h3 className="section-title">订阅列表</h3>
            <button className="mini-btn" onClick={onCreateRootFolder}>
              新建分类
            </button>
          </div>
          <div className="list">
            <button className={`item feed-item ${selectedFeedID == null && selectedFolderID == null ? "active" : ""}`} onClick={() => onSelectFeed(null)}>
              <strong>全部订阅源</strong>
            </button>
            {rootFolders.map((folder) => renderFolderNode(folder))}
            {uncategorizedFeeds.length > 0 && (
              <div
                className={`tree-divider ${dragOverUncategorized ? "drop-target" : ""}`}
                onDragOver={onUncategorizedDragOver}
                onDragLeave={onUncategorizedDragLeave}
                onDrop={onUncategorizedDrop}
              >
                未分类（可拖拽到这里取消分类）
              </div>
            )}
            {uncategorizedFeeds.map((feed) => renderFeedNode(feed, 8))}
            {feeds.length === 0 && <div className="item">暂无订阅</div>}
          </div>
        </>
      ) : (
        <>
          <div className="section-head">
            <h3 className="section-title">收藏文章</h3>
          </div>
          <div className="list">
            {favoriteArticles.map((article) => (
              <button
                key={`favorite-${article.id}`}
                className={`item favorite-entry ${selectedArticleID === article.id ? "active" : ""}`}
                onClick={() => onSelectArticle(article.id)}
                title={article.title || "(无标题)"}
              >
                <span className="favorite-entry-star" aria-hidden="true">
                  ☆
                </span>
                <span className="favorite-entry-title">{article.title || "(无标题)"}</span>
              </button>
            ))}
            {favoriteArticles.length === 0 && <div className="item">暂无收藏</div>}
          </div>
        </>
      )}
    </div>
  );
}
