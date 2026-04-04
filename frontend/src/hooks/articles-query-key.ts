import type { SortMode } from "@/lib/article-list";

export const ARTICLE_PAGE_SIZE = 20;

export function buildArticlesQueryKey(apiBase: string, sortMode: SortMode = "latest") {
  return ["articles", apiBase, ARTICLE_PAGE_SIZE, sortMode] as const;
}
