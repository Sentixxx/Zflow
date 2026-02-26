import { useMemo } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { ApiClient } from "@/api";

const ARTICLE_PAGE_SIZE = 20;

export function useReaderQueries(apiBase: string) {
  const client = useMemo(() => new ApiClient(apiBase), [apiBase]);

  const feedsQuery = useQuery({
    queryKey: ["feeds", apiBase],
    queryFn: () => client.listFeeds(),
    staleTime: 30_000,
    refetchInterval: 60_000,
  });

  const foldersQuery = useQuery({
    queryKey: ["folders", apiBase],
    queryFn: () => client.listFolders(),
    staleTime: 30_000,
    refetchInterval: 60_000,
  });

  const articlesInfiniteQuery = useInfiniteQuery({
    queryKey: ["articles", apiBase, ARTICLE_PAGE_SIZE],
    initialPageParam: 1,
    queryFn: async ({ pageParam }) => client.listArticlesPage(pageParam, ARTICLE_PAGE_SIZE),
    getNextPageParam: (lastPage, allPages) => {
      if (!lastPage.hasMore) {
        return undefined;
      }
      return allPages.length + 1;
    },
    staleTime: 30_000,
    refetchInterval: 60_000,
  });

  return {
    client,
    feedsQuery,
    foldersQuery,
    articlesInfiniteQuery,
  };
}
