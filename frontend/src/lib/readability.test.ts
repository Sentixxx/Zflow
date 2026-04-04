import { describe, expect, it } from "vitest";
import type { Article } from "@/types";
import { canRequestReadability } from "./readability";

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

  it("disables readability when article is missing", () => {
    expect(canRequestReadability(null)).toBe(false);
  });
});
