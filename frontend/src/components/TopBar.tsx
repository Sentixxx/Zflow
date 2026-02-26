type TopBarProps = {
  statusText: string;
  isError: boolean;
  isRefreshingArticles: boolean;
  isRefreshingFeeds: boolean;
  onRefreshArticles: () => void;
  onRefreshFeeds: () => void;
};

export function TopBar({
  statusText,
  isError,
  isRefreshingArticles,
  isRefreshingFeeds,
  onRefreshArticles,
  onRefreshFeeds,
}: TopBarProps) {
  return (
    <header className="topbar">
      <div className="brand">
        <span className="brand-mark">▸</span>
        <h1>Zflow</h1>
      </div>
      <div className={`top-status ${isError ? "error" : ""}`}>{statusText}</div>
      <div className="top-actions">
        <button
          className={`icon-btn ${isRefreshingArticles ? "loading" : ""}`}
          onClick={onRefreshArticles}
          disabled={isRefreshingArticles || isRefreshingFeeds}
          title={isRefreshingArticles ? "正在刷新文章..." : "刷新文章列表"}
          data-tooltip={isRefreshingArticles ? "正在刷新文章..." : "刷新文章列表"}
          aria-label={isRefreshingArticles ? "正在刷新文章" : "刷新文章列表"}
        >
          <span className="icon-btn-glyph">⟳</span>
        </button>
        <button
          className={`icon-btn ${isRefreshingFeeds ? "loading" : ""}`}
          onClick={onRefreshFeeds}
          disabled={isRefreshingFeeds || isRefreshingArticles}
          title={isRefreshingFeeds ? "正在远端抓取订阅源..." : "远端抓取订阅源"}
          data-tooltip={isRefreshingFeeds ? "正在远端抓取订阅源..." : "远端抓取订阅源"}
          aria-label={isRefreshingFeeds ? "正在远端抓取订阅源" : "远端抓取订阅源"}
        >
          <span className="icon-btn-glyph">◎</span>
        </button>
      </div>
    </header>
  );
}
