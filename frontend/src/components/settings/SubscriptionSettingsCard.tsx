import type { Folder } from "@/types";

type SubscriptionSettingsCardProps = {
  feedURL: string;
  onFeedURLChange: (value: string) => void;
  newFeedFolderID: number | null;
  onNewFeedFolderIDChange: (value: number | null) => void;
  folders: Folder[];
  onCreateRootFolder: () => void;
  onAddFeed: () => void;
  onRefreshFeeds: () => void;
  onRefreshFeedsFromNetwork: () => void;
  onRefreshArticles: () => void;
  isRefreshingFeeds: boolean;
  isRefreshingArticles: boolean;
};

export function SubscriptionSettingsCard({
  feedURL,
  onFeedURLChange,
  newFeedFolderID,
  onNewFeedFolderIDChange,
  folders,
  onCreateRootFolder,
  onAddFeed,
  onRefreshFeeds,
  onRefreshFeedsFromNetwork,
  onRefreshArticles,
  isRefreshingFeeds,
  isRefreshingArticles,
}: SubscriptionSettingsCardProps) {
  return (
    <div className="settings-page-inner settings-section-card">
      <h4 className="section-title">添加订阅</h4>
      <label htmlFor="feedUrl">RSS/Atom URL</label>
      <input id="feedUrl" value={feedURL} placeholder="https://example.com/feed.xml" onChange={(e) => onFeedURLChange(e.target.value)} />
      <label htmlFor="folderSelect">归类到文件夹</label>
      <div className="row">
        <select id="folderSelect" value={newFeedFolderID ?? ""} onChange={(e) => onNewFeedFolderIDChange(e.target.value ? Number(e.target.value) : null)}>
          <option value="">未分类</option>
          {folders.map((folder) => (
            <option key={folder.id} value={folder.id}>
              {folder.name}
            </option>
          ))}
        </select>
        <button className="secondary" onClick={onCreateRootFolder}>
          新建分类
        </button>
      </div>
      <div className="row settings-actions">
        <button onClick={onAddFeed}>添加并首抓</button>
        <button className="secondary" onClick={onRefreshFeeds}>
          刷新订阅
        </button>
        <button className="secondary" onClick={onRefreshFeedsFromNetwork} disabled={isRefreshingFeeds || isRefreshingArticles}>
          远端抓取
        </button>
        <button className="secondary" onClick={onRefreshArticles} disabled={isRefreshingArticles || isRefreshingFeeds}>
          刷新文章
        </button>
      </div>
    </div>
  );
}
