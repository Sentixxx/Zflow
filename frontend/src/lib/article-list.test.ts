import { describe, expect, it, vi, afterEach } from "vitest";
import { filterAndSortArticles, formatArticleTime } from "./article-list";
import type { Article } from "@/types";

function article(input: Partial<Article> & Pick<Article, "id">): Article {
  return {
    id: input.id,
    feed_id: input.feed_id ?? 1,
    title: input.title ?? `article-${input.id}`,
    link: input.link ?? "",
    summary: input.summary ?? "",
    full_content: input.full_content ?? "",
    published_at: input.published_at ?? "",
    is_read: input.is_read ?? false,
    is_favorite: input.is_favorite ?? false,
    favorited_at: input.favorited_at ?? "",
    created_at: input.created_at ?? "2026-02-25T00:00:00Z",
    cover_url: input.cover_url ?? "",
  };
}

afterEach(() => {
  vi.useRealTimers();
});

describe("formatArticleTime", () => {
  it("formats short relative time", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-02-26T08:00:00Z"));
    expect(formatArticleTime("2026-02-26T07:59:45Z")).toBe("刚刚");
    expect(formatArticleTime("2026-02-26T07:30:00Z")).toContain("分钟前");
  });
});

describe("filterAndSortArticles", () => {
  it("keeps sticky read entries in unread mode", () => {
    const rows = [
      article({ id: 1, is_read: false, published_at: "2026-02-25T00:00:00Z" }),
      article({ id: 2, is_read: true, published_at: "2026-02-26T00:00:00Z" }),
    ];
    const result = filterAndSortArticles(rows, "unread", "latest", new Set([2]));
    expect(result.map((x) => x.id)).toEqual([2, 1]);
  });
});
