import { useEffect, useState } from "react";
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
  const [feeds, setFeeds] = useState<Feed[]>([]);
  const [folders, setFolders] = useState<Folder[]>([]);

  useEffect(() => {
    if (feedsQuery.data) {
      setFeeds(feedsQuery.data);
    }
  }, [feedsQuery.data]);

  useEffect(() => {
    if (foldersQuery.data) {
      setFolders(foldersQuery.data);
    }
  }, [foldersQuery.data]);

  const loadFeeds = async (options?: { silentStatus?: boolean }) => {
    try {
      const data = (await feedsQuery.refetch()).data ?? (await client.listFeeds());
      setFeeds(data);
      if (!options?.silentStatus) {
        setMessage("订阅列表已刷新");
      }
      return data;
    } catch (e) {
      setMessage((e as Error).message, true);
      return null;
    }
  };

  const loadFolders = async () => {
    try {
      const data = (await foldersQuery.refetch()).data ?? (await client.listFolders());
      setFolders(data);
      return data;
    } catch (e) {
      setMessage((e as Error).message, true);
      return null;
    }
  };

  return {
    feeds,
    folders,
    setFeeds,
    setFolders,
    loadFeeds,
    loadFolders,
  };
}
