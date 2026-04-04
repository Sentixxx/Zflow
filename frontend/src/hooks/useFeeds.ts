import type { Feed, Folder } from "@/types";
import type { ApiClient } from "@/api";
import type { UseQueryResult } from "@tanstack/react-query";

export type MessageSetter = (message: string, isError?: boolean) => void;

export function useFeeds(
  client: ApiClient,
  feedsQuery: UseQueryResult<Feed[], Error>,
  foldersQuery: UseQueryResult<Folder[], Error>,
  setMessage: MessageSetter,
) {
  const feeds = feedsQuery.data ?? [];
  const folders = foldersQuery.data ?? [];

  const loadFeeds = async (options?: { silentStatus?: boolean }) => {
    try {
      const data = (await feedsQuery.refetch()).data ?? (await client.listFeeds());
      if (!options?.silentStatus) {
        setMessage("订阅列表已刷新");
      }
      return data;
    } catch (e) {
      setMessage((e as Error).message, true);
      return null;
    }
  };

  const loadFolders = async (options?: { silentStatus?: boolean }) => {
    try {
      const data = (await foldersQuery.refetch()).data ?? (await client.listFolders());
      if (!options?.silentStatus) {
        setMessage("分类列表已刷新");
      }
      return data;
    } catch (e) {
      setMessage((e as Error).message, true);
      return null;
    }
  };

  return {
    feeds,
    folders,
    loadFeeds,
    loadFolders,
  };
}
