import { describe, expect, it, vi, afterEach } from "vitest";
import { filterAndSortArticles, formatArticleTime, formatRecommendationSummary, getRecommendationScores } from "./article-list";
import type { Article, RecommendationScores } from "@/types";

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
    recommendation_scores: input.recommendation_scores,
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

  it("sorts by explicit recommendation scores when available", () => {
    const high: RecommendationScores = { quality: 92, relevance: 80, novelty: 75, composite: 88 };
    const low: RecommendationScores = { quality: 30, relevance: 45, novelty: 40, composite: 36 };
    const rows = [
      article({ id: 1, published_at: "2026-02-25T00:00:00Z", recommendation_scores: low }),
      article({ id: 2, published_at: "2026-02-24T00:00:00Z", recommendation_scores: high }),
    ];
    const result = filterAndSortArticles(rows, "all", "recommend", new Set());
    expect(result.map((x) => x.id)).toEqual([2, 1]);
  });

  it("keeps backend order when preserveIncomingOrder is enabled", () => {
    const rows = [
      article({ id: 1, published_at: "2026-02-25T00:00:00Z", recommendation_scores: { quality: 10, relevance: 10, novelty: 10, composite: 10 } }),
      article({ id: 2, published_at: "2026-02-24T00:00:00Z", recommendation_scores: { quality: 99, relevance: 99, novelty: 99, composite: 99 } }),
    ];
    const result = filterAndSortArticles(rows, "all", "recommend", new Set(), true);
    expect(result.map((x) => x.id)).toEqual([1, 2]);
  });
});

describe("recommendation helpers", () => {
  it("falls back to heuristic scores when article has no backend scores", () => {
    const scores = getRecommendationScores(
      article({
        id: 8,
        title: "OpenAI releases a detailed model update",
        summary: "A detailed update covers model behavior, deployment safety, and release process.",
        full_content: "A detailed update covers model behavior, deployment safety, and release process with examples and rollout notes.",
        published_at: "2026-02-26T07:00:00Z",
      }),
    );
    expect(scores.composite).toBeGreaterThan(0);
    expect(scores.quality).toBeGreaterThan(0);
  });

  it("formats recommendation summary for quick display", () => {
    const text = formatRecommendationSummary(
      article({
        id: 9,
        recommendation_scores: { quality: 88, relevance: 81, novelty: 73, composite: 84 },
      }),
    );
    expect(text).toContain("综 84");
    expect(text).toContain("质 88");
  });
});
