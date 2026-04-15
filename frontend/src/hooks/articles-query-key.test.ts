import { describe, expect, it } from "vitest";
import { buildArticlesQueryKey } from "./articles-query-key";

describe("buildArticlesQueryKey", () => {
  it("uses apiBase and sortMode as key parts when no scope is provided", () => {
    expect(buildArticlesQueryKey("http://127.0.0.1:8080", "latest")).toEqual([
      "articles",
      "http://127.0.0.1:8080",
      20,
      "latest",
      null,
      null,
    ]);
  });

  it("includes feedId in key so switching feed triggers re-fetch", () => {
    const key1 = buildArticlesQueryKey("http://127.0.0.1:8080", "latest", { feedId: 1, folderId: null });
    const key2 = buildArticlesQueryKey("http://127.0.0.1:8080", "latest", { feedId: 2, folderId: null });
    expect(key1).not.toEqual(key2);
    expect(key1[4]).toBe(1);
    expect(key2[4]).toBe(2);
  });

  it("includes folderId in key so switching folder triggers re-fetch", () => {
    const key1 = buildArticlesQueryKey("http://127.0.0.1:8080", "latest", { feedId: null, folderId: 10 });
    const key2 = buildArticlesQueryKey("http://127.0.0.1:8080", "latest", { feedId: null, folderId: 20 });
    expect(key1).not.toEqual(key2);
    expect(key1[5]).toBe(10);
    expect(key2[5]).toBe(20);
  });

  it("feed scope and folder scope produce different keys even with same numeric value", () => {
    const feedKey = buildArticlesQueryKey("http://127.0.0.1:8080", "latest", { feedId: 5, folderId: null });
    const folderKey = buildArticlesQueryKey("http://127.0.0.1:8080", "latest", { feedId: null, folderId: 5 });
    expect(feedKey).not.toEqual(folderKey);
  });

  it("null scope and default scope produce the same key (both null)", () => {
    const withDefault = buildArticlesQueryKey("http://127.0.0.1:8080", "latest");
    const withNulls = buildArticlesQueryKey("http://127.0.0.1:8080", "latest", { feedId: null, folderId: null });
    expect(withDefault).toEqual(withNulls);
  });

  it("sortMode change produces different key regardless of scope", () => {
    const latestKey = buildArticlesQueryKey("http://127.0.0.1:8080", "latest", { feedId: 1, folderId: null });
    const oldestKey = buildArticlesQueryKey("http://127.0.0.1:8080", "oldest", { feedId: 1, folderId: null });
    expect(latestKey).not.toEqual(oldestKey);
  });
});
