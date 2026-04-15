import { describe, expect, it } from "vitest";
import { feedHost, buildFeedIconURLByHost } from "./feed-utils";
import type { Feed } from "@/types";

function feed(overrides: Partial<Feed> & Pick<Feed, "id">): Feed {
  return {
    id: overrides.id,
    url: overrides.url ?? "",
    title: overrides.title ?? `Feed ${overrides.id}`,
    icon_url: overrides.icon_url,
    last_fetch_error: overrides.last_fetch_error,
    item_count: overrides.item_count ?? 0,
    retention_days: overrides.retention_days ?? 0,
    custom_script: overrides.custom_script,
    custom_script_lang: overrides.custom_script_lang,
    created_at: overrides.created_at ?? "2026-04-01T00:00:00Z",
    last_fetched_at: overrides.last_fetched_at ?? "2026-04-01T00:00:00Z",
    last_fetch_status: overrides.last_fetch_status ?? "ok",
    folder_id: overrides.folder_id ?? null,
  };
}

describe("feedHost", () => {
  it("extracts lowercase host from a valid https URL", () => {
    expect(feedHost("https://Blog.Example.COM/rss")).toBe("blog.example.com");
  });

  it("extracts host from an http URL", () => {
    expect(feedHost("http://feeds.example.org/rss.xml")).toBe("feeds.example.org");
  });

  it("returns empty string for invalid URL", () => {
    expect(feedHost("not-a-url")).toBe("");
  });

  it("returns empty string for undefined input", () => {
    expect(feedHost(undefined)).toBe("");
  });

  it("returns empty string for empty string", () => {
    expect(feedHost("")).toBe("");
  });
});

describe("buildFeedIconURLByHost", () => {
  it("maps host to prefixed icon URL", () => {
    const feeds = [
      feed({ id: 1, url: "https://example.com/rss", icon_url: "/icons/example.ico" }),
    ];
    const map = buildFeedIconURLByHost(feeds, "http://localhost:8080");
    expect(map.get("example.com")).toBe("http://localhost:8080/icons/example.ico");
  });

  it("deduplicates hosts, first feed with icon wins", () => {
    const feeds = [
      feed({ id: 1, url: "https://example.com/feed1", icon_url: "/icons/first.ico" }),
      feed({ id: 2, url: "https://example.com/feed2", icon_url: "/icons/second.ico" }),
    ];
    const map = buildFeedIconURLByHost(feeds, "http://localhost:8080");
    // Only the first feed's icon should be stored for this host
    expect(map.get("example.com")).toBe("http://localhost:8080/icons/first.ico");
    expect(map.size).toBe(1);
  });

  it("skips feeds with no icon_url", () => {
    const feeds = [
      feed({ id: 1, url: "https://example.com/rss" }),
    ];
    const map = buildFeedIconURLByHost(feeds, "http://localhost:8080");
    expect(map.size).toBe(0);
  });

  it("strips trailing slash from apiBase", () => {
    const feeds = [
      feed({ id: 1, url: "https://example.com/rss", icon_url: "/icons/ex.ico" }),
    ];
    const map = buildFeedIconURLByHost(feeds, "http://localhost:8080/");
    expect(map.get("example.com")).toBe("http://localhost:8080/icons/ex.ico");
  });

  it("handles multiple different hosts", () => {
    const feeds = [
      feed({ id: 1, url: "https://alpha.com/rss", icon_url: "/icons/a.ico" }),
      feed({ id: 2, url: "https://beta.com/rss", icon_url: "/icons/b.ico" }),
    ];
    const map = buildFeedIconURLByHost(feeds, "http://localhost:8080");
    expect(map.get("alpha.com")).toBe("http://localhost:8080/icons/a.ico");
    expect(map.get("beta.com")).toBe("http://localhost:8080/icons/b.ico");
  });
});
