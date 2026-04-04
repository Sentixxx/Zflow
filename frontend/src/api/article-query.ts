import type { SortMode } from "@/lib/article-list";

type BuildArticleListQueryOptions = {
  page?: number;
  limit?: number;
  sort?: SortMode;
  feedID?: number | null;
  folderID?: number | null;
};

export function buildArticleListQuery({ page, limit, sort, feedID, folderID }: BuildArticleListQueryOptions): string {
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
  if (typeof feedID === "number") {
    search.set("feed_id", String(feedID));
  }
  if (typeof folderID === "number") {
    search.set("folder_id", String(folderID));
  }
  return search.toString();
}
