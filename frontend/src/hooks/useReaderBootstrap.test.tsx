/* @vitest-environment jsdom */
import { describe, expect, it, vi } from "vitest";
import { act } from "react";
import { createRoot } from "react-dom/client";
import type { ApiClient } from "@/api";

const mockClient = {} as ApiClient;
const mockLoadFeeds = vi.fn();
const mockLoadFolders = vi.fn();
const mockLoadArticles = vi.fn();
const mockSetArticles = vi.fn();
const mockFetchNext = vi.fn();

vi.mock("./useReaderQueries", () => ({
  useReaderQueries: () => ({
    client: mockClient,
    feedsQuery: { data: [] },
    foldersQuery: { data: [] },
    articlesQueryKey: ["articles"],
    articlesInfiniteQuery: {
      data: undefined,
      hasNextPage: false,
      isFetching: false,
      isFetchingNextPage: false,
      refetch: vi.fn(),
      fetchNextPage: vi.fn(),
    },
  }),
}));

vi.mock("./useFeeds", () => ({
  useFeeds: () => ({
    feeds: ["feed"],
    folders: ["folder"],
    loadFeeds: mockLoadFeeds,
    loadFolders: mockLoadFolders,
  }),
}));

vi.mock("./useEntries", () => ({
  useEntries: () => ({
    articles: ["article"],
    setArticles: mockSetArticles,
    loadArticles: mockLoadArticles,
    fetchNextArticlePage: mockFetchNext,
    hasNextArticlePage: true,
  }),
}));

import { useReaderBootstrap } from "./useReaderBootstrap";

describe("useReaderBootstrap", () => {
  it("wires shared reader data and actions", () => {
    globalThis.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);
    let snapshot: ReturnType<typeof useReaderBootstrap> | null = null;

    const Probe = () => {
      snapshot = useReaderBootstrap("http://example.com", "latest");
      return null;
    };

    act(() => {
      root.render(<Probe />);
    });

    expect(snapshot?.client).toBe(mockClient);
    expect(snapshot?.feeds).toEqual(["feed"]);
    expect(snapshot?.folders).toEqual(["folder"]);
    expect(snapshot?.articles).toEqual(["article"]);
    expect(snapshot?.hasNextArticlePage).toBe(true);
  });
});
