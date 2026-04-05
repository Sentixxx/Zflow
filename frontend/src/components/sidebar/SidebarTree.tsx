import type { Article, Feed, Folder } from "@/types";
import { feedHost } from "@/lib/feed-utils";
import { RssFallbackIcon } from "@/components/ui";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";

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
    const isDragging = draggingFeedID === feed.id;
    const isSelected = selectedFeedID === feed.id;
    const host = feedHost(feed.url);
    const iconSrc = feed.icon_url
      ? `${apiBase.replace(/\/$/, "")}${feed.icon_url}`
      : feedIconURLByHost.get(host) || "";

    return (
      <div key={`feed-${feed.id}`} className="relative group">
        <button
          className={cn(
            "w-full text-left px-2 py-1.5 text-sm transition-colors border-b border-border/50 pr-8",
            isSelected ? "bg-accent text-accent-foreground border-l-2 border-l-primary" : "hover:bg-muted/60",
            isDragging && "opacity-40"
          )}
          style={{ paddingLeft }}
          onClick={() => { if (!isRenaming) onSelectFeed(feed.id); }}
          draggable={!isRenaming}
          onDragStart={(event) => { if (isRenaming) return; onFeedDragStart(event, feed.id); }}
          onDragEnd={onFeedDragEnd}
        >
          {isRenaming ? (
            <div onClick={(event) => event.stopPropagation()}>
              <input
                className="w-full rounded border border-input bg-background px-2 py-1 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
                value={renamingFeedTitle}
                onChange={(event) => onRenamingFeedTitleChange(event.target.value)}
                autoFocus
                onBlur={() => onRenameFeed(feed.id)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") { event.preventDefault(); onRenameFeed(feed.id); }
                }}
              />
            </div>
          ) : (
            <>
              <div className="flex items-center gap-1.5 mb-0.5">
                {iconSrc ? (
                  <>
                    <img
                      className="w-3.5 h-3.5 rounded-sm object-cover flex-shrink-0"
                      src={iconSrc} alt="" loading="lazy"
                      onError={(event) => {
                        event.currentTarget.style.display = "none";
                        const fallback = event.currentTarget.nextElementSibling as HTMLElement | null;
                        if (fallback) fallback.style.display = "inline-flex";
                      }}
                    />
                    <span className="hidden w-3.5 h-3.5 text-orange-500 flex-shrink-0" aria-hidden="true">
                      <RssFallbackIcon />
                    </span>
                  </>
                ) : (
                  <span className="inline-flex w-3.5 h-3.5 text-orange-500 flex-shrink-0" aria-hidden="true">
                    <RssFallbackIcon />
                  </span>
                )}
                <span className="font-medium truncate">{feed.title || "(未命名源)"}</span>
              </div>
              <div className="text-xs text-muted-foreground truncate">
                {feed.url} · {feed.item_count} · {feed.last_fetch_status}
                {feed.last_fetch_status === "failed" && feed.last_fetch_error ? ` · ${feed.last_fetch_error}` : ""}
              </div>
            </>
          )}
        </button>
        {!isRenaming && (
          <button
            className="absolute right-1 top-1/2 -translate-y-1/2 w-6 h-6 rounded-md opacity-0 group-hover:opacity-100 hover:bg-muted flex items-center justify-center text-muted-foreground transition-opacity"
            onClick={(event) => onOpenFeedContextMenu(event, feed)}
            title="管理订阅源"
            aria-label={`管理订阅源 ${feed.title || feed.url}`}
          >
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
    const isSelected = selectedFolderID === folder.id;
    const isDragTarget = dragOverFolderID === folder.id;
    const paddingLeft = 8 + depth * 14;

    return (
      <div key={`folder-${folder.id}`}>
        <div
          className={cn("relative group", isDragTarget && "bg-accent/40")}
          onDragOver={(event) => onFolderDragOver(event, folder.id)}
          onDragLeave={() => onFolderDragLeave(folder.id)}
          onDrop={(event) => onFolderDrop(event, folder.id)}
        >
          <button
            className={cn(
              "w-full text-left px-2 py-1.5 text-sm font-semibold border-b border-border/50 pr-8 flex items-center gap-1.5 transition-colors",
              isSelected ? "bg-accent text-accent-foreground border-l-2 border-l-primary" : "hover:bg-muted/60"
            )}
            style={{ paddingLeft }}
            onClick={() => onSelectFolder(folder.id)}
          >
            <span
              className={cn(
                "w-3 text-muted-foreground text-xs transition-transform duration-150",
                hasChildren ? "cursor-pointer" : "opacity-30",
                expanded && hasChildren && "rotate-90"
              )}
              onClick={(event) => {
                event.preventDefault();
                event.stopPropagation();
                if (hasChildren) onToggleFolderCollapsed(folder.id);
              }}
            >
              ▸
            </span>
            <span className="truncate">{folder.name}</span>
          </button>
          <button
            className="absolute right-1 top-1/2 -translate-y-1/2 w-6 h-6 rounded-md opacity-0 group-hover:opacity-100 hover:bg-muted flex items-center justify-center text-muted-foreground transition-opacity"
            onClick={(event) => onOpenFolderContextMenu(event, folder)}
            title="管理分类"
            aria-label={`管理分类 ${folder.name}`}
          >
            ⋯
          </button>
        </div>
        {expanded && (
          <div>
            {folderFeeds.map((feed) => renderFeedNode(feed, paddingLeft + 18))}
            {children.map((child) => renderFolderNode(child, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
      {/* Mode tabs */}
      <div className="grid grid-cols-2 gap-1.5 px-2 pt-2 pb-1.5 shrink-0">
        {(["subscriptions", "favorites"] as const).map((mode) => (
          <button
            key={mode}
            onClick={() => onSwitchSidebarMode(mode)}
            className={cn(
              "rounded-md px-2 py-1.5 text-sm font-medium transition-colors border",
              sidebarMode === mode
                ? "bg-accent text-accent-foreground border-primary/30"
                : "bg-transparent text-muted-foreground border-transparent hover:border-border hover:bg-muted/50"
            )}
          >
            {mode === "subscriptions" ? "订阅源" : "收藏"}
          </button>
        ))}
      </div>

      <Separator />

      {/* List content */}
      <div className="flex-1 min-h-0 overflow-auto pb-14">
        {sidebarMode === "subscriptions" ? (
          <>
            <div className="flex items-center justify-between px-3 py-2">
              <span className="text-xs font-bold uppercase tracking-widest text-muted-foreground">订阅列表</span>
              <button
                onClick={onCreateRootFolder}
                className="text-xs px-2 py-1 rounded border border-border text-muted-foreground hover:bg-muted/60 transition-colors"
              >
                新建分类
              </button>
            </div>
            {/* All feeds item */}
            <button
              className={cn(
                "w-full text-left px-3 py-1.5 text-sm font-medium border-b border-border/50 transition-colors",
                selectedFeedID == null && selectedFolderID == null
                  ? "bg-accent text-accent-foreground border-l-2 border-l-primary"
                  : "hover:bg-muted/60"
              )}
              onClick={() => onSelectFeed(null)}
            >
              全部订阅源
            </button>
            {rootFolders.map((folder) => renderFolderNode(folder))}
            {uncategorizedFeeds.length > 0 && (
              <div
                className={cn(
                  "px-3 py-1 text-xs text-muted-foreground border-b border-border/50 transition-colors",
                  dragOverUncategorized && "bg-accent/40"
                )}
                onDragOver={onUncategorizedDragOver}
                onDragLeave={onUncategorizedDragLeave}
                onDrop={onUncategorizedDrop}
              >
                未分类（可拖拽到这里取消分类）
              </div>
            )}
            {uncategorizedFeeds.map((feed) => renderFeedNode(feed, 8))}
            {feeds.length === 0 && (
              <div className="px-3 py-4 text-sm text-muted-foreground">暂无订阅</div>
            )}
          </>
        ) : (
          <>
            <div className="px-3 py-2">
              <span className="text-xs font-bold uppercase tracking-widest text-muted-foreground">收藏文章</span>
            </div>
            {favoriteArticles.map((article) => (
              <button
                key={`favorite-${article.id}`}
                className={cn(
                  "w-full text-left px-3 py-2 text-sm border-b border-border/50 flex items-start gap-2 transition-colors",
                  selectedArticleID === article.id
                    ? "bg-accent text-accent-foreground border-l-2 border-l-primary"
                    : "hover:bg-muted/60"
                )}
                onClick={() => onSelectArticle(article.id)}
                title={article.title || "(无标题)"}
              >
                <span className="text-amber-400 shrink-0 mt-0.5" aria-hidden="true">☆</span>
                <span className="truncate">{article.title || "(无标题)"}</span>
              </button>
            ))}
            {favoriteArticles.length === 0 && (
              <div className="px-3 py-4 text-sm text-muted-foreground">暂无收藏</div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
