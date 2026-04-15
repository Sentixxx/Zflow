import type { SortMode } from "@/lib/article-list";

export const ARTICLE_PAGE_SIZE = 20;

export type ArticleScope = {
  feedId: number | null;
  folderId: number | null;
};

export function buildArticlesQueryKey(
  apiBase: string,
  sortMode: SortMode = "latest",
  scope: ArticleScope = { feedId: null, folderId: null },
) {
  // scope 的 feedId/folderId 加入 key，确保切换 feed/folder 时 React Query 重新 fetch
  return ["articles", apiBase, ARTICLE_PAGE_SIZE, sortMode, scope.feedId, scope.folderId] as const;
}
