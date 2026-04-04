import { describe, expect, it } from "vitest";
import type { InfiniteData } from "@tanstack/react-query";
import type { Article } from "@/types";
import { buildSingleArticlesPageData, flattenArticlePages, mapArticlePages, shouldHydrateArticlePool } from "./article-pages";

type ArticlesPage = {
  articles: Article[];
  hasMore: boolean;
};

function makeArticle(id: number, overrides: Partial<Article> = {}): Article {
  return {
    id,
    feed_id: 1,
    title: `article-${id}`,
    link: "",
    summary: "",
    published_at: "",
    created_at: "",
    is_read: false,
    is_favorite: false,
    ...overrides,
  };
}

function makeData(): InfiniteData<ArticlesPage, unknown> {
  return {
    pageParams: [1, 2],
    pages: [
      { articles: [makeArticle(1), makeArticle(2)], hasMore: true },
      { articles: [makeArticle(3)], hasMore: false },
    ],
  };
}

describe("article-pages helpers", () => {
  it("wraps a full article list as a single page cache", () => {
    const data = buildSingleArticlesPageData([makeArticle(1), makeArticle(2)]);
    expect(data.pages).toHaveLength(1);
    expect(data.pages[0].hasMore).toBe(false);
    expect(data.pages[0].articles.map((article) => article.id)).toEqual([1, 2]);
  });

  it("flattens every page in order", () => {
    expect(flattenArticlePages(makeData()).map((article) => article.id)).toEqual([1, 2, 3]);
  });

  it("updates matching articles without dropping other pages", () => {
    const updated = mapArticlePages(makeData(), (article) => (article.id === 2 ? { ...article, is_read: true } : article));
    expect(updated.pages).toHaveLength(2);
    expect(updated.pages[0].articles[1].is_read).toBe(true);
    expect(updated.pages[1].articles[0].id).toBe(3);
  });

  it("only hydrates remaining pages after the first page is already visible", () => {
    expect(shouldHydrateArticlePool({ loadedCount: 20, hasNextPage: true, isFetching: false, isFetchingNextPage: false })).toBe(true);
    expect(shouldHydrateArticlePool({ loadedCount: 0, hasNextPage: true, isFetching: false, isFetchingNextPage: false })).toBe(false);
    expect(shouldHydrateArticlePool({ loadedCount: 20, hasNextPage: false, isFetching: false, isFetchingNextPage: false })).toBe(false);
    expect(shouldHydrateArticlePool({ loadedCount: 20, hasNextPage: true, isFetching: true, isFetchingNextPage: false })).toBe(false);
    expect(shouldHydrateArticlePool({ loadedCount: 20, hasNextPage: true, isFetching: false, isFetchingNextPage: true })).toBe(false);
  });
});
