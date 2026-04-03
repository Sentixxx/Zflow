import type { SortMode } from "@/lib/article-list";

type BuildArticleListQueryOptions = {
  page?: number;
  limit?: number;
  sort?: SortMode;
};

export function buildArticleListQuery({ page, limit, sort }: BuildArticleListQueryOptions): string {
  const search = new URLSearchParams();
  if (typeof page === "number") {
    search.set("page", String(page));
  }
  if (typeof limit === "number") {
    search.set("limit", String(limit));
  }
  if (sort) {
    search.set("sort", sort);
  }
  return search.toString();
}
