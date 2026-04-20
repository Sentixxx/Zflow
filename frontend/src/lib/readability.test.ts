import { describe, expect, it } from "vitest";
import type { Article } from "@/types";
import { canRequestReadability, shouldAutoFetchReadability } from "./readability";

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
    created_at: input.created_at ?? "2026-04-05T00:00:00Z",
    recommendation_scores: input.recommendation_scores,
    cover_url: input.cover_url ?? "",
  };
}

describe("canRequestReadability", () => {
  it("allows readability when article exists even if link is empty", () => {
    expect(canRequestReadability(article({ id: 1, link: "" }))).toBe(true);
  });

  it("allows readability when article has a concrete link", () => {
    expect(canRequestReadability(article({ id: 1, link: "https://example.com/a" }))).toBe(true);
  });

  it("disables readability when article is missing", () => {
    expect(canRequestReadability(null)).toBe(false);
  });
});

describe("shouldAutoFetchReadability", () => {
  it("auto fetches when article has no readable content even if link is empty", () => {
    const target = article({ id: 2, link: "", full_content: "" });
    expect(
      shouldAutoFetchReadability({ article: target, hasUsableFullContent: false, isExtractingReadable: false }),
    ).toBe(true);
  });

  it("skips auto fetch while readability is in progress", () => {
    const target = article({ id: 3, link: "", full_content: "" });
    expect(
      shouldAutoFetchReadability({ article: target, hasUsableFullContent: false, isExtractingReadable: true }),
    ).toBe(false);
  });

  it("skips auto fetch when the article already has usable full content", () => {
    // Even without an in-flight extraction, a present full_content should
    // short-circuit the auto-fetch decision — avoids clobbering existing data.
    const target = article({ id: 4, full_content: "<p>body</p>" });
    expect(
      shouldAutoFetchReadability({ article: target, hasUsableFullContent: true, isExtractingReadable: false }),
    ).toBe(false);
  });

  it("prefers the usable-content branch over the in-progress branch", () => {
    // Both flags true: usable content wins (order of guards), still returns false.
    const target = article({ id: 5, full_content: "<p>body</p>" });
    expect(
      shouldAutoFetchReadability({ article: target, hasUsableFullContent: true, isExtractingReadable: true }),
    ).toBe(false);
  });

  it("returns false when article is null regardless of other flags", () => {
    expect(
      shouldAutoFetchReadability({ article: null, hasUsableFullContent: false, isExtractingReadable: false }),
    ).toBe(false);
    expect(
      shouldAutoFetchReadability({ article: null, hasUsableFullContent: true, isExtractingReadable: true }),
    ).toBe(false);
  });
});
