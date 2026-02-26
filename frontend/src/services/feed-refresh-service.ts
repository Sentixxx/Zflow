import type { Feed } from "@/types";
import type { RefreshFailure } from "@/components";

export type FeedRefreshSummary = {
  successCount: number;
  failedCount: number;
  failures: RefreshFailure[];
};

export async function refreshFeedsBatch(
  feeds: Feed[],
  refreshOne: (feedID: number) => Promise<void>,
): Promise<FeedRefreshSummary> {
  const settled = await Promise.allSettled(feeds.map((feed) => refreshOne(feed.id)));
  const successCount = settled.filter((item) => item.status === "fulfilled").length;
  const failedCount = settled.length - successCount;
  const failures: RefreshFailure[] = settled.flatMap((item, index) => {
    if (item.status === "fulfilled") {
      return [];
    }
    const feed = feeds[index];
    const reason = item.reason instanceof Error ? item.reason.message : String(item.reason || "未知错误");
    return [
      {
        feedID: feed.id,
        feedTitle: feed.title || feed.url || `#${feed.id}`,
        reason,
      },
    ];
  });

  return { successCount, failedCount, failures };
}
