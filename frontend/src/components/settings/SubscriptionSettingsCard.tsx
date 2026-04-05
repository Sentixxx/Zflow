import type { Folder } from "@/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

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
    <div className="space-y-5">
      <div>
        <h4 className="text-base font-semibold mb-4">添加订阅</h4>
        <div className="space-y-3">
          <div className="space-y-1.5">
            <Label htmlFor="feedUrl">RSS/Atom URL</Label>
            <Input
              id="feedUrl"
              value={feedURL}
              placeholder="https://example.com/feed.xml"
              onChange={(e) => onFeedURLChange(e.target.value)}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="folderSelect">归类到文件夹</Label>
            <div className="flex gap-2">
              <select
                id="folderSelect"
                value={newFeedFolderID ?? ""}
                onChange={(e) => onNewFeedFolderIDChange(e.target.value ? Number(e.target.value) : null)}
                className="flex-1 rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
              >
                <option value="">未分类</option>
                {folders.map((folder) => (
                  <option key={folder.id} value={folder.id}>
                    {folder.name}
                  </option>
                ))}
              </select>
              <Button variant="outline" onClick={onCreateRootFolder}>
                新建分类
              </Button>
            </div>
          </div>
        </div>
      </div>

      <div>
        <h4 className="text-base font-semibold mb-3">操作</h4>
        <div className="flex flex-wrap gap-2">
          <Button onClick={onAddFeed}>添加并首抓</Button>
          <Button variant="outline" onClick={onRefreshFeeds}>刷新订阅</Button>
          <Button
            variant="outline"
            onClick={onRefreshFeedsFromNetwork}
            disabled={isRefreshingFeeds || isRefreshingArticles}
          >
            远端抓取
          </Button>
          <Button
            variant="outline"
            onClick={onRefreshArticles}
            disabled={isRefreshingArticles || isRefreshingFeeds}
          >
            刷新文章
          </Button>
        </div>
      </div>
    </div>
  );
}
