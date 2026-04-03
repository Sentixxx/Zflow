import { describe, expect, it } from "vitest";
import { buildArticleListQuery } from "./article-query";

describe("buildArticleListQuery", () => {
  it("includes sort mode alongside page and limit", () => {
    expect(buildArticleListQuery({ page: 2, limit: 20, sort: "recommend" })).toBe("page=2&limit=20&sort=recommend");
  });
});
