import { useMemo } from "react";
import type { Article } from "@/types";
import type { ApiClient } from "@/api";
import { useQueryClient } from "@tanstack/react-query";
import type { InfiniteData, QueryKey, UseInfiniteQueryResult } from "@tanstack/react-query";
import type { MessageSetter } from "@/hooks/useFeeds";
import { flattenArticlePages, replaceArticlePages, type ArticlesPage } from "./article-pages";

export function useEntries(
  _client: ApiClient,
  articlesInfiniteQuery: UseInfiniteQueryResult<InfiniteData<ArticlesPage, unknown>, Error>,
  articlesQueryKey: QueryKey,
  setMessage: MessageSetter,
) {
  const queryClient = useQueryClient();
  const articles = useMemo(() => flattenArticlePages(articlesInfiniteQuery.data), [articlesInfiniteQuery.data]);

  const loadArticles = async (options?: { silentStatus?: boolean }): Promise<Article[] | null> => {
    try {
      const result = await articlesInfiniteQuery.refetch();
      const data = flattenArticlePages(result.data);
      if (!options?.silentStatus) {
        setMessage("文章列表已刷新");
      }
      return data;
    } catch (e) {
      setMessage((e as Error).message, true);
      return null;
    }
  };

  const setArticles = (updater: Article[] | ((current: Article[]) => Article[])) => {
    queryClient.setQueryData<InfiniteData<ArticlesPage, unknown> | undefined>(articlesQueryKey, (current) => {
      if (!current) {
        return current;
      }
      const nextArticles = typeof updater === "function" ? updater(flattenArticlePages(current)) : updater;
      return replaceArticlePages(current, nextArticles);
    });
  };

  const fetchNextArticlePage = async () => {
    if (!articlesInfiniteQuery.hasNextPage || articlesInfiniteQuery.isFetchingNextPage) {
      return false;
    }
    await articlesInfiniteQuery.fetchNextPage();
    return true;
  };

  const upsertArticle = (nextArticle: Article) => {
    setArticles((current) => current.map((entry) => (entry.id === nextArticle.id ? nextArticle : entry)));
  };

  return {
    articles,
    setArticles,
    loadArticles,
    fetchNextArticlePage,
    hasNextArticlePage: Boolean(articlesInfiniteQuery.hasNextPage),
    isFetchingNextArticlePage: articlesInfiniteQuery.isFetchingNextPage,
    upsertArticle,
  };
}
