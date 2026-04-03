import { useEffect, useMemo, useState } from "react";
import type { Article } from "@/types";
import type { ApiClient } from "@/api";
import type { InfiniteData, UseInfiniteQueryResult } from "@tanstack/react-query";
import type { MessageSetter } from "@/hooks/useFeeds";
import type { SortMode } from "@/lib/article-list";

type ArticlesPage = {
  articles: Article[];
  hasMore: boolean;
};

export function useEntries(
  client: ApiClient,
  articlesInfiniteQuery: UseInfiniteQueryResult<InfiniteData<ArticlesPage, unknown>, Error>,
  setMessage: MessageSetter,
  sortMode: SortMode = "latest",
) {
  const [articles, setArticles] = useState<Article[]>([]);

  const mergedFromQuery = useMemo(() => {
    if (!articlesInfiniteQuery.data) {
      return [] as Article[];
    }
    return articlesInfiniteQuery.data.pages.flatMap((page) => page.articles);
  }, [articlesInfiniteQuery.data]);

  useEffect(() => {
    if (articlesInfiniteQuery.data) {
      setArticles(mergedFromQuery);
    }
  }, [mergedFromQuery, articlesInfiniteQuery.data]);

  const loadArticles = async (options?: { silentStatus?: boolean }): Promise<Article[] | null> => {
    try {
      const data = await client.listArticles(undefined, undefined, sortMode);
      setArticles(data);
      if (!options?.silentStatus) {
        setMessage("文章列表已刷新");
      }
      return data;
    } catch (e) {
      setMessage((e as Error).message, true);
      return null;
    }
  };

  const fetchNextArticlePage = async () => {
    if (!articlesInfiniteQuery.hasNextPage || articlesInfiniteQuery.isFetchingNextPage) {
      return false;
    }
    await articlesInfiniteQuery.fetchNextPage();
    return true;
  };

  const upsertArticle = (nextArticle: Article) => {
    setArticles((current) => {
      const idx = current.findIndex((entry) => entry.id === nextArticle.id);
      if (idx < 0) {
        return [nextArticle, ...current];
      }
      const copy = [...current];
      copy[idx] = nextArticle;
      return copy;
    });
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
