import { useState } from "react";
import type { SortMode } from "@/lib/article-list";
import type { RefreshFailure } from "@/components";
import { useReaderQueries } from "@/hooks/useReaderQueries";
import type { ArticleScope } from "@/hooks/articles-query-key";
import { useFeeds } from "@/hooks/useFeeds";
import { useEntries } from "@/hooks/useEntries";
import { refreshFeedsBatch } from "@/services/feed-refresh-service";

export function useReaderBootstrap(
  apiBase: string,
  sortMode: SortMode = "latest",
  scope: ArticleScope = { feedId: null, folderId: null },
) {
  const { client, feedsQuery, foldersQuery, articlesQueryKey, articlesInfiniteQuery } = useReaderQueries(apiBase, sortMode, scope);
  const [status, setStatus] = useState("准备就绪");
  const [error, setError] = useState("");

  const setMessage = (message: string, isError = false) => {
    if (isError) {
      setError(message);
      setStatus("");
      return;
    }
    setError("");
    setStatus(message);
  };

  const { feeds, folders, loadFeeds, loadFolders } = useFeeds(client, feedsQuery, foldersQuery, setMessage);
  const { articles, setArticles, loadArticles, fetchNextArticlePage, hasNextArticlePage } = useEntries(
    client,
    articlesInfiniteQuery,
    articlesQueryKey,
    setMessage,
  );
  const [feedURL, setFeedURL] = useState("");
  const [newFeedFolderID, setNewFeedFolderID] = useState<number | null>(null);
  const [isRefreshingArticles, setIsRefreshingArticles] = useState(false);
  const [isRefreshingFeeds, setIsRefreshingFeeds] = useState(false);
  const [refreshFailures, setRefreshFailures] = useState<RefreshFailure[]>([]);

  const refreshFeedsFromNetwork = async () => {
    if (isRefreshingFeeds) {
      return;
    }
    setIsRefreshingFeeds(true);
    setRefreshFailures([]);
    try {
      setMessage("正在远端抓取订阅源...");
      const currentFeeds = await client.listFeeds();
      if (currentFeeds.length === 0) {
        setMessage("暂无订阅源可刷新");
        return;
      }
      const { successCount, failedCount, failures } = await refreshFeedsBatch(currentFeeds, (feedID) =>
        client.refreshFeed(feedID),
      );
      setRefreshFailures(failures);
      await Promise.all([loadFeeds({ silentStatus: true }), loadArticles({ silentStatus: true })]);
      if (failedCount > 0) {
        setMessage(`订阅源刷新完成：成功 ${successCount}，失败 ${failedCount}`);
      } else {
        setRefreshFailures([]);
        setMessage(`订阅源刷新完成：成功 ${successCount}`);
      }
    } catch (e) {
      setMessage((e as Error).message, true);
    } finally {
      setIsRefreshingFeeds(false);
    }
  };

  const handleRefreshArticles = async () => {
    if (isRefreshingArticles) {
      return;
    }
    setIsRefreshingArticles(true);
    setMessage("正在刷新文章...");
    try {
      const data = await loadArticles({ silentStatus: true });
      if (data) {
        setMessage("文章列表已刷新");
      }
    } finally {
      setIsRefreshingArticles(false);
    }
  };

  const handleRefreshFeeds = async () => {
    await loadFeeds();
  };

  const addFeed = async () => {
    const url = feedURL.trim();
    if (!url) {
      setMessage("请输入 RSS/Atom URL", true);
      return;
    }
    try {
      setMessage("正在添加订阅并抓取...");
      await client.createFeed(url, newFeedFolderID);
      setFeedURL("");
      await Promise.all([loadFeeds(), loadFolders(), loadArticles()]);
      setMessage("订阅添加成功");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  const createRootFolder = async () => {
    const name = window.prompt("分类名称", "新分类");
    if (!name || !name.trim()) {
      return;
    }
    try {
      const folder = await client.createFolder(name.trim());
      await Promise.all([loadFolders(), loadFeeds()]);
      setNewFeedFolderID(folder.id);
      setMessage("分类已创建");
    } catch (e) {
      setMessage((e as Error).message, true);
    }
  };

  return {
    client,
    feeds,
    folders,
    articlesInfiniteQuery,
    loadFeeds,
    loadFolders,
    articles,
    setArticles,
    loadArticles,
    fetchNextArticlePage,
    hasNextArticlePage,
    feedURL,
    setFeedURL,
    newFeedFolderID,
    setNewFeedFolderID,
    isRefreshingFeeds,
    isRefreshingArticles,
    refreshFailures,
    setRefreshFailures,
    refreshFeedsFromNetwork,
    handleRefreshArticles,
    handleRefreshFeeds,
    addFeed,
    createRootFolder,
    setMessage,
    status,
    error,
  };
}
