/**
 * feed-refresh-service.ts contract tests.
 *
 * Invariants from spec_rss_llm_reader.md (Acceptance Criteria):
 * - Batch refresh must aggregate success/failure per-feed.
 * - Failure records must include feedTitle for user-visible error reporting.
 * - All-success case must produce zero failures.
 */
import { describe, expect, it, vi } from "vitest";
import { refreshFeedsBatch } from "./feed-refresh-service";
import type { Feed } from "@/types";

function feed(id: number, title: string, url: string): Feed {
  return {
    id,
    url,
    title,
    item_count: 0,
    last_fetched_at: "2026-04-01T00:00:00Z",
    last_fetch_status: "ok",
    created_at: "2026-04-01T00:00:00Z",
  };
}

describe("refreshFeedsBatch", () => {
  it("returns zero failures when all feeds succeed", async () => {
    const feeds = [
      feed(1, "Alpha Feed", "https://alpha.com/rss"),
      feed(2, "Beta Feed", "https://beta.com/rss"),
    ];
    const refreshOne = vi.fn().mockResolvedValue(undefined);
    const result = await refreshFeedsBatch(feeds, refreshOne);

    expect(result.successCount).toBe(2);
    expect(result.failedCount).toBe(0);
    expect(result.failures).toHaveLength(0);
    expect(refreshOne).toHaveBeenCalledTimes(2);
  });

  it("aggregates failures when some feeds error", async () => {
    const feeds = [
      feed(1, "OK Feed", "https://ok.com/rss"),
      feed(2, "Bad Feed", "https://bad.com/rss"),
      feed(3, "Also OK Feed", "https://alsook.com/rss"),
    ];
    const refreshOne = vi.fn().mockImplementation(async (feedID: number) => {
      if (feedID === 2) {
        throw new Error("connection timeout");
      }
    });

    const result = await refreshFeedsBatch(feeds, refreshOne);

    expect(result.successCount).toBe(2);
    expect(result.failedCount).toBe(1);
    expect(result.failures).toHaveLength(1);
    expect(result.failures[0].feedID).toBe(2);
    expect(result.failures[0].feedTitle).toBe("Bad Feed");
    expect(result.failures[0].reason).toContain("connection timeout");
  });

  it("uses feed title in failure record, falls back to URL when title is empty", async () => {
    const feedNoTitle: Feed = {
      id: 10,
      url: "https://notitle.com/rss",
      title: "",
      item_count: 0,
      last_fetched_at: "2026-04-01T00:00:00Z",
      last_fetch_status: "ok",
      created_at: "2026-04-01T00:00:00Z",
    };
    const refreshOne = vi.fn().mockRejectedValue(new Error("fetch error"));

    const result = await refreshFeedsBatch([feedNoTitle], refreshOne);

    expect(result.failures).toHaveLength(1);
    // Spec: title || url || `#${id}` fallback chain
    expect(result.failures[0].feedTitle).toBe("https://notitle.com/rss");
  });

  it("falls back to #id when both title and url are empty", async () => {
    const feedNoTitleNoURL: Feed = {
      id: 99,
      url: "",
      title: "",
      item_count: 0,
      last_fetched_at: "2026-04-01T00:00:00Z",
      last_fetch_status: "ok",
      created_at: "2026-04-01T00:00:00Z",
    };
    const refreshOne = vi.fn().mockRejectedValue("plain string error");

    const result = await refreshFeedsBatch([feedNoTitleNoURL], refreshOne);

    expect(result.failures).toHaveLength(1);
    expect(result.failures[0].feedTitle).toBe("#99");
  });

  it("handles all feeds failing", async () => {
    const feeds = [
      feed(1, "A", "https://a.com/rss"),
      feed(2, "B", "https://b.com/rss"),
    ];
    const refreshOne = vi.fn().mockRejectedValue(new Error("no network"));

    const result = await refreshFeedsBatch(feeds, refreshOne);

    expect(result.successCount).toBe(0);
    expect(result.failedCount).toBe(2);
    expect(result.failures).toHaveLength(2);
  });
});
