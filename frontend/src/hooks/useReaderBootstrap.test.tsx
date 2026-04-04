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

type BootstrapSnapshot = {
  client: ApiClient;
  feeds: string[];
  folders: string[];
  articles: string[];
  hasNextArticlePage: boolean;
};

describe("useReaderBootstrap", () => {
  it("wires shared reader data and actions", () => {
    const globalWithAct = globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
    globalWithAct.IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);
    const snapshotRef = { current: null as BootstrapSnapshot | null };

    const Probe = () => {
      snapshotRef.current = useReaderBootstrap("http://example.com", "latest") as unknown as BootstrapSnapshot;
      return null;
    };

    act(() => {
      root.render(<Probe />);
    });

    const snapshot = snapshotRef.current as BootstrapSnapshot;
    expect(snapshot.client).toBe(mockClient);
    expect(snapshot.feeds).toEqual(["feed"]);
    expect(snapshot.folders).toEqual(["folder"]);
    expect(snapshot.articles).toEqual(["article"]);
    expect(snapshot.hasNextArticlePage).toBe(true);
  });
});
