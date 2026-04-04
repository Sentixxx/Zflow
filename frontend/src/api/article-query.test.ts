import { describe, expect, it } from "vitest";
import { buildArticleListQuery } from "./article-query";

describe("buildArticleListQuery", () => {
  it("includes sort mode alongside page and limit", () => {
    expect(buildArticleListQuery({ page: 2, limit: 20, sort: "recommend" })).toBe("page=2&limit=20&sort=recommend");
  });

  it("includes feed and folder scope when provided", () => {
    expect(buildArticleListQuery({ page: 1, limit: 20, sort: "latest", feedID: 12, folderID: 34 })).toBe(
      "page=1&limit=20&sort=latest&feed_id=12&folder_id=34",
    );
  });
});
