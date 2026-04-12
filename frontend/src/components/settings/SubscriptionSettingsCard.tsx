import { useEffect, useState } from "react";
import type { Feed, Folder } from "@/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Loader2 } from "lucide-react";

type SubscriptionSettingsCardProps = {
  feeds: Feed[];
  feedURL: string;
  onFeedURLChange: (value: string) => void;
  newFeedFolderID: number | null;
  onNewFeedFolderIDChange: (value: number | null) => void;
  articleRetentionDays: string;
  folders: Folder[];
  onCreateRootFolder: () => void;
  onAddFeed: () => void;
  onSaveFeedRetentionDays: (feedID: number, retentionDays: number) => Promise<boolean>;
  onRefreshFeeds: () => void;
  onRefreshFeedsFromNetwork: () => void;
  onRefreshArticles: () => void;
  isRefreshingFeeds: boolean;
  isRefreshingArticles: boolean;
};

export function SubscriptionSettingsCard({
  feeds,
  feedURL,
  onFeedURLChange,
  newFeedFolderID,
  onNewFeedFolderIDChange,
  articleRetentionDays,
  folders,
  onCreateRootFolder,
  onAddFeed,
  onSaveFeedRetentionDays,
  onRefreshFeeds,
  onRefreshFeedsFromNetwork,
  onRefreshArticles,
  isRefreshingFeeds,
  isRefreshingArticles,
}: SubscriptionSettingsCardProps) {
  const [retentionDrafts, setRetentionDrafts] = useState<Record<number, string>>({});

  useEffect(() => {
    setRetentionDrafts(
      Object.fromEntries(feeds.map((feed) => [feed.id, String(feed.retention_days ?? 0)])),
    );
  }, [feeds]);

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
            {isRefreshingFeeds && <Loader2 className="w-4 h-4 mr-1.5 animate-spin" />}
            远端抓取
          </Button>
          <Button
            variant="outline"
            onClick={onRefreshArticles}
            disabled={isRefreshingArticles || isRefreshingFeeds}
          >
            {isRefreshingArticles && <Loader2 className="w-4 h-4 mr-1.5 animate-spin" />}
            刷新文章
          </Button>
        </div>
      </div>

      <div>
        <h4 className="text-base font-semibold mb-3">订阅源保留覆盖</h4>
        <p className="text-xs text-muted-foreground mb-3">填 `0` 表示跟随全局设置。当前全局默认：{articleRetentionDays} 天。</p>
        <div className="space-y-3">
          {feeds.map((feed) => (
            <div key={feed.id} className="rounded-md border border-border p-3">
              <div className="mb-2">
                <div className="text-sm font-medium">{feed.title || "(未命名源)"}</div>
                <div className="text-xs text-muted-foreground truncate">{feed.url}</div>
              </div>
              <div className="flex items-center gap-2">
                <Input
                  type="number"
                  min={0}
                  max={3650}
                  value={retentionDrafts[feed.id] ?? "0"}
                  onChange={(e) => setRetentionDrafts((current) => ({ ...current, [feed.id]: e.target.value }))}
                  className="w-28"
                />
                <Button
                  variant="outline"
                  onClick={() => void onSaveFeedRetentionDays(feed.id, Number(retentionDrafts[feed.id] ?? "0"))}
                >
                  保存
                </Button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
