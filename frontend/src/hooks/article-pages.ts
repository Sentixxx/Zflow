import type { InfiniteData } from "@tanstack/react-query";
import type { Article } from "@/types";

export type ArticlesPage = {
  articles: Article[];
  hasMore: boolean;
};

export function buildSingleArticlesPageData(articles: Article[]): InfiniteData<ArticlesPage, unknown> {
  return {
    pageParams: [1],
    pages: [
      {
        articles,
        hasMore: false,
      },
    ],
  };
}

export function shouldHydrateArticlePool(options: {
  loadedCount: number;
  hasNextPage: boolean;
  isFetching: boolean;
  isFetchingNextPage: boolean;
}): boolean {
  return options.loadedCount > 0 && options.hasNextPage && !options.isFetching && !options.isFetchingNextPage;
}

export function flattenArticlePages(data: InfiniteData<ArticlesPage, unknown> | undefined): Article[] {
  if (!data) {
    return [];
  }
  return data.pages.flatMap((page) => page.articles);
}

export function mapArticlePages(
  data: InfiniteData<ArticlesPage, unknown>,
  mapper: (article: Article) => Article,
): InfiniteData<ArticlesPage, unknown> {
  return {
    ...data,
    pages: data.pages.map((page) => ({
      ...page,
      articles: page.articles.map(mapper),
    })),
  };
}

export function replaceArticlePages(
  data: InfiniteData<ArticlesPage, unknown>,
  nextArticles: Article[],
): InfiniteData<ArticlesPage, unknown> {
  let cursor = 0;
  return {
    ...data,
    pages: data.pages.map((page, index) => {
      const remaining = nextArticles.length - cursor;
      const size = index === data.pages.length - 1 ? Math.max(remaining, 0) : Math.min(page.articles.length, Math.max(remaining, 0));
      const articles = nextArticles.slice(cursor, cursor + size);
      cursor += size;
      return {
        ...page,
        articles,
      };
    }),
  };
}
