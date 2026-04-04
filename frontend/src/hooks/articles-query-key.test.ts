import { describe, expect, it } from "vitest";
import { buildArticlesQueryKey } from "./articles-query-key";

describe("buildArticlesQueryKey", () => {
  it("depends only on apiBase and sortMode so subscription switching stays instant", () => {
    expect(buildArticlesQueryKey("http://127.0.0.1:8080", "latest")).toEqual(["articles", "http://127.0.0.1:8080", 20, "latest"]);
  });
});
