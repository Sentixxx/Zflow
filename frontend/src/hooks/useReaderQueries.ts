import { useMemo } from "react";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { ApiClient } from "@/api";
import type { SortMode } from "@/lib/article-list";
import { ARTICLE_PAGE_SIZE, buildArticlesQueryKey, type ArticleScope } from "./articles-query-key";

export function useReaderQueries(
  apiBase: string,
  sortMode: SortMode = "latest",
  scope: ArticleScope = { feedId: null, folderId: null },
) {
  const client = useMemo(() => new ApiClient(apiBase), [apiBase]);
  const articlesQueryKey = buildArticlesQueryKey(apiBase, sortMode, scope);

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
    queryKey: articlesQueryKey,
    initialPageParam: 1,
    queryFn: async ({ pageParam }) =>
      client.listArticlesPage(pageParam, ARTICLE_PAGE_SIZE, sortMode, {
        feedID: scope.feedId,
        folderID: scope.folderId,
      }),
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
    articlesQueryKey,
    articlesInfiniteQuery,
  };
}
