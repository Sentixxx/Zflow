import { useEffect, useRef, useState } from "react";
import { SORT_MODE_LABELS } from "@/lib/article-list";
import type { ReadFilter, SortMode } from "@/lib/article-list";

type ArticleListToolbarProps = {
  readFilter: ReadFilter;
  sortMode: SortMode;
  onToggleReadFilter: () => void;
  onSortModeChange: (mode: SortMode) => void;
};

const SORT_MODE_OPTIONS: SortMode[] = ["latest", "oldest", "recommend", "quality", "relevance", "novelty"];

export function ArticleListToolbar({ readFilter, sortMode, onToggleReadFilter, onSortModeChange }: ArticleListToolbarProps) {
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!menuOpen) {
      return;
    }

    const handlePointerDown = (event: PointerEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };
    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenuOpen(false);
      }
    };

    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("keydown", handleEscape);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("keydown", handleEscape);
    };
  }, [menuOpen]);

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
      <div className={`list-sort-menu ${menuOpen ? "is-open" : ""}`} ref={menuRef}>
        <span className="list-sort-caption">排序</span>
        <button
          type="button"
          className="list-sort-trigger"
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          aria-label={`文章排序方式：${SORT_MODE_LABELS[sortMode]}`}
          onClick={() => setMenuOpen((open) => !open)}
        >
          <span className="list-sort-current">{SORT_MODE_LABELS[sortMode]}</span>
          <span className="list-sort-chevron" aria-hidden="true">
            ▾
          </span>
        </button>
        {menuOpen && (
          <div className="list-sort-popover" role="menu" aria-label="文章排序方式">
            {SORT_MODE_OPTIONS.map((mode) => (
              <button
                key={mode}
                type="button"
                role="menuitemradio"
                aria-checked={mode === sortMode}
                className={`list-sort-option ${mode === sortMode ? "is-active" : ""}`}
                onClick={() => {
                  onSortModeChange(mode);
                  setMenuOpen(false);
                }}
              >
                <span>{SORT_MODE_LABELS[mode]}</span>
                {mode === sortMode ? <span className="list-sort-option-mark">✓</span> : null}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
