import type { ReadFilter, SortMode } from "../lib/article-list";

type ArticleListToolbarProps = {
  readFilter: ReadFilter;
  sortMode: SortMode;
  onToggleReadFilter: () => void;
  onToggleSortMode: () => void;
};

export function ArticleListToolbar({ readFilter, sortMode, onToggleReadFilter, onToggleSortMode }: ArticleListToolbarProps) {
  return (
    <div className="list-toolbar">
      <button
        className={`list-icon-btn ${readFilter === "unread" ? "is-unread" : "is-read"}`}
        onClick={onToggleReadFilter}
        title={readFilter === "unread" ? "仅显示未读" : "显示全部（含已读）"}
        data-tooltip={readFilter === "unread" ? "仅显示未读" : "显示全部（含已读）"}
        aria-label={readFilter === "unread" ? "仅显示未读" : "显示全部（含已读）"}
      >
        <span className="glyph">{readFilter === "unread" ? "○" : "●"}</span>
      </button>
      <button
        className={`list-icon-btn ${sortMode === "latest" ? "sort-latest" : "sort-oldest"}`}
        onClick={onToggleSortMode}
        title={sortMode === "latest" ? "最新优先" : "最早优先"}
        data-tooltip={sortMode === "latest" ? "最新优先" : "最早优先"}
        aria-label={sortMode === "latest" ? "最新优先" : "最早优先"}
      >
        <span className="glyph">{sortMode === "latest" ? "↓" : "↑"}</span>
      </button>
    </div>
  );
}
