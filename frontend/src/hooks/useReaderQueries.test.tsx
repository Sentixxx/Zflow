/* @vitest-environment jsdom */
import { describe, expect, it, vi } from "vitest";
import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { buildArticlesQueryKey } from "./articles-query-key";
import { useReaderQueries } from "./useReaderQueries";

// 仅针对 useReaderQueries 的 key/staleTime/refetchInterval 契约与 getNextPageParam 分支，
// 不真实发请求 —— 通过 vi.stubGlobal('fetch') 让 ApiClient 的 request 不会打外部。

type GlobalWithAct = typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };

function mountHook<T>(callback: () => T) {
  (globalThis as GlobalWithAct).IS_REACT_ACT_ENVIRONMENT = true;
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
  });
  const container = document.createElement("div");
  const root = createRoot(container);
  const ref = { current: undefined as T | undefined };
  const Probe = () => {
    ref.current = callback();
    return null;
  };
  act(() => {
    root.render(createElement(QueryClientProvider, { client }, createElement(Probe)));
  });
  return {
    get current() {
      if (ref.current === undefined) {
        throw new Error("hook not rendered yet");
      }
      return ref.current;
    },
    unmount: () => {
      act(() => {
        root.unmount();
      });
      client.clear();
    },
  };
}

describe("useReaderQueries", () => {
  it("feeds/folders queryKey 带上 apiBase，保持跨实例稳定", () => {
    // 阻止任何真实 fetch 流出
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network disabled in test")));
    const handle = mountHook(() => useReaderQueries("http://a.test", "latest"));
    const { feedsQuery, foldersQuery, articlesQueryKey } = handle.current;
    expect(feedsQuery).toBeDefined();
    expect(foldersQuery).toBeDefined();
    expect(articlesQueryKey).toEqual(buildArticlesQueryKey("http://a.test", "latest", { feedId: null, folderId: null }));
    handle.unmount();
    vi.unstubAllGlobals();
  });

  it("scope 改变 articlesQueryKey 也随之改变（保证切换 feed 时会重新 fetch）", () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network disabled in test")));
    const a = mountHook(() => useReaderQueries("http://a.test", "latest", { feedId: 7, folderId: null }));
    const keyA = a.current.articlesQueryKey;
    a.unmount();
    const b = mountHook(() => useReaderQueries("http://a.test", "latest", { feedId: 8, folderId: null }));
    const keyB = b.current.articlesQueryKey;
    b.unmount();
    expect(keyA).not.toEqual(keyB);
    expect(keyA[4]).toBe(7);
    expect(keyB[4]).toBe(8);
    vi.unstubAllGlobals();
  });

  it("getNextPageParam: hasMore=true 返回下一页号，hasMore=false 返回 undefined", async () => {
    // 后端按 limit+1 pattern 探测 hasMore：返回 21 篇→hasMore=true，返回 1 篇→hasMore=false。
    const stubArticle = (id: number) => ({
      id,
      feed_id: 1,
      title: `t${id}`,
      link: "",
      is_read: false,
      is_favorite: false,
      created_at: "",
    });
    const pagePayloads = [
      { articles: Array.from({ length: 21 }, (_, i) => stubArticle(i + 1)) },
      { articles: [stubArticle(99)] },
    ];
    let callIndex = 0;
    const fetchMock = vi.fn(async (input: string | URL | Request) => {
      const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
      if (url.includes("/articles")) {
        const body = pagePayloads[callIndex] ?? { articles: [] };
        callIndex += 1;
        return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      if (url.includes("/feeds")) {
        return new Response(JSON.stringify({ feeds: [] }), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      if (url.includes("/folders")) {
        return new Response(JSON.stringify({ folders: [] }), { status: 200, headers: { "Content-Type": "application/json" } });
      }
      return new Response("{}", { status: 200 });
    });
    vi.stubGlobal("fetch", fetchMock);

    const handle = mountHook(() => useReaderQueries("http://ipg.test", "latest"));
    // 等待首次 fetch 完成
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 30));
    });
    const q = handle.current.articlesInfiniteQuery;
    expect(q.hasNextPage).toBe(true);
    // fetch next page；hasMore=false 后 hasNextPage 应变为 false
    await act(async () => {
      await q.fetchNextPage();
      await new Promise((resolve) => setTimeout(resolve, 30));
    });
    expect(handle.current.articlesInfiniteQuery.hasNextPage).toBe(false);
    handle.unmount();
    vi.unstubAllGlobals();
  });

  it("staleTime / refetchInterval 遵循 CLAUDE.md 30s / 60s 约定", () => {
    // 通过读源码的 defaultOptions 不可行；改为直接从 QueryClient cache 里读 query.options。
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("network disabled in test")));
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
    });
    (globalThis as GlobalWithAct).IS_REACT_ACT_ENVIRONMENT = true;
    const container = document.createElement("div");
    const root = createRoot(container);
    const ref = { current: undefined as ReturnType<typeof useReaderQueries> | undefined };
    const Probe = () => {
      ref.current = useReaderQueries("http://b.test", "latest");
      return null;
    };
    act(() => {
      root.render(createElement(QueryClientProvider, { client }, createElement(Probe)));
    });

    type ObservedOptions = { staleTime?: number; refetchInterval?: number };
    const readOptions = (queryKey: unknown[]): ObservedOptions => {
      const entry = client.getQueryCache().find({ queryKey });
      return (entry?.options ?? {}) as unknown as ObservedOptions;
    };
    const feedsOpts = readOptions(["feeds", "http://b.test"]);
    const foldersOpts = readOptions(["folders", "http://b.test"]);
    const articlesOpts = readOptions([...buildArticlesQueryKey("http://b.test", "latest")]);
    expect(feedsOpts.staleTime).toBe(30_000);
    expect(feedsOpts.refetchInterval).toBe(60_000);
    expect(foldersOpts.staleTime).toBe(30_000);
    expect(foldersOpts.refetchInterval).toBe(60_000);
    expect(articlesOpts.staleTime).toBe(30_000);
    expect(articlesOpts.refetchInterval).toBe(60_000);

    act(() => {
      root.unmount();
    });
    client.clear();
    vi.unstubAllGlobals();
  });
});
