/* @vitest-environment jsdom */
import { describe, expect, it, vi } from "vitest";
import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { InfiniteData, UseInfiniteQueryResult } from "@tanstack/react-query";
import type { ApiClient } from "@/api";
import type { Article } from "@/types";
import { useEntries } from "./useEntries";
import { buildSingleArticlesPageData, type ArticlesPage } from "./article-pages";

type GlobalWithAct = typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };

function makeArticle(id: number, overrides: Partial<Article> = {}): Article {
  return {
    id,
    feed_id: 1,
    title: `a${id}`,
    link: "",
    is_read: false,
    is_favorite: false,
    created_at: "",
    ...overrides,
  };
}

type InfiniteQueryStub = UseInfiniteQueryResult<InfiniteData<ArticlesPage, unknown>, Error>;

function makeInfiniteQuery(overrides: Partial<InfiniteQueryStub> = {}): InfiniteQueryStub {
  const defaults: Partial<InfiniteQueryStub> = {
    data: undefined,
    hasNextPage: false,
    isFetchingNextPage: false,
    refetch: vi.fn().mockResolvedValue({ data: undefined }),
    fetchNextPage: vi.fn().mockResolvedValue(undefined),
  };
  return { ...defaults, ...overrides } as InfiniteQueryStub;
}

function mountEntries(
  articlesQuery: InfiniteQueryStub,
  articlesQueryKey: readonly unknown[],
  setMessage = vi.fn(),
  primeData?: InfiniteData<ArticlesPage, unknown>,
) {
  (globalThis as GlobalWithAct).IS_REACT_ACT_ENVIRONMENT = true;
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (primeData) {
    queryClient.setQueryData(articlesQueryKey, primeData);
  }
  const container = document.createElement("div");
  const root = createRoot(container);
  const ref = { current: undefined as ReturnType<typeof useEntries> | undefined };
  const Probe = () => {
    ref.current = useEntries({} as ApiClient, articlesQuery, articlesQueryKey as never, setMessage);
    return null;
  };
  act(() => {
    root.render(createElement(QueryClientProvider, { client: queryClient }, createElement(Probe)));
  });
  return {
    get current() {
      if (!ref.current) {
        throw new Error("useEntries hook did not render");
      }
      return ref.current;
    },
    queryClient,
    unmount: () => {
      act(() => root.unmount());
      queryClient.clear();
    },
  };
}

describe("useEntries", () => {
  it("articles 从 infinite data 扁平化；空响应不 panic 返回 []", () => {
    const q = makeInfiniteQuery();
    const handle = mountEntries(q, ["articles", "a"]);
    expect(handle.current.articles).toEqual([]);
    expect(handle.current.hasNextArticlePage).toBe(false);
    handle.unmount();
  });

  it("articles 形状：多页合并后保持原顺序", () => {
    const data: InfiniteData<ArticlesPage, unknown> = {
      pageParams: [1, 2],
      pages: [
        { articles: [makeArticle(1), makeArticle(2)], hasMore: true },
        { articles: [makeArticle(3)], hasMore: false },
      ],
    };
    const q = makeInfiniteQuery({ data, hasNextPage: false });
    const handle = mountEntries(q, ["articles", "b"]);
    expect(handle.current.articles.map((a) => a.id)).toEqual([1, 2, 3]);
    handle.unmount();
  });

  it("loadArticles 成功 → setMessage 提示；silentStatus 时不提示", async () => {
    const refetch = vi.fn().mockResolvedValue({
      data: buildSingleArticlesPageData([makeArticle(10)]),
    });
    const q = makeInfiniteQuery({ refetch });
    const setMessage = vi.fn();
    const handle = mountEntries(q, ["articles", "c"], setMessage);

    const result = await handle.current.loadArticles();
    expect(result?.map((a) => a.id)).toEqual([10]);
    expect(setMessage).toHaveBeenCalledWith("文章列表已刷新");

    setMessage.mockClear();
    await handle.current.loadArticles({ silentStatus: true });
    expect(setMessage).not.toHaveBeenCalled();
    handle.unmount();
  });

  it("loadArticles 失败 → setMessage(error, true) 并返回 null", async () => {
    const refetch = vi.fn().mockRejectedValue(new Error("fetch fail"));
    const q = makeInfiniteQuery({ refetch });
    const setMessage = vi.fn();
    const handle = mountEntries(q, ["articles", "d"], setMessage);
    const result = await handle.current.loadArticles();
    expect(result).toBeNull();
    expect(setMessage).toHaveBeenCalledWith("fetch fail", true);
    handle.unmount();
  });

  it("fetchNextArticlePage: 无下一页或正在 fetch 时直接返回 false", async () => {
    const q1 = makeInfiniteQuery({ hasNextPage: false });
    const h1 = mountEntries(q1, ["articles", "e1"]);
    expect(await h1.current.fetchNextArticlePage()).toBe(false);
    expect(q1.fetchNextPage).not.toHaveBeenCalled();
    h1.unmount();

    const q2 = makeInfiniteQuery({ hasNextPage: true, isFetchingNextPage: true });
    const h2 = mountEntries(q2, ["articles", "e2"]);
    expect(await h2.current.fetchNextArticlePage()).toBe(false);
    expect(q2.fetchNextPage).not.toHaveBeenCalled();
    h2.unmount();
  });

  it("fetchNextArticlePage: 有下一页时调用 fetchNextPage 返回 true", async () => {
    const q = makeInfiniteQuery({ hasNextPage: true, isFetchingNextPage: false });
    const handle = mountEntries(q, ["articles", "f"]);
    const ok = await handle.current.fetchNextArticlePage();
    expect(ok).toBe(true);
    expect(q.fetchNextPage).toHaveBeenCalledTimes(1);
    handle.unmount();
  });

  it("setArticles/upsertArticle 写入 queryClient cache，空 cache 时保持不变", () => {
    const q = makeInfiniteQuery();
    // cache 为空时 setArticles 不应抛错，也不应写入任何数据
    const handle1 = mountEntries(q, ["articles", "g1"]);
    handle1.current.setArticles([makeArticle(1)]);
    expect(handle1.queryClient.getQueryData(["articles", "g1"])).toBeUndefined();
    handle1.unmount();

    // cache 预置一页，upsertArticle 能精准替换单篇
    const seed = buildSingleArticlesPageData([makeArticle(1), makeArticle(2)]);
    const handle2 = mountEntries(q, ["articles", "g2"], vi.fn(), seed);
    handle2.current.upsertArticle(makeArticle(2, { title: "updated" }));
    const cached = handle2.queryClient.getQueryData<InfiniteData<ArticlesPage, unknown>>(["articles", "g2"]);
    const found = cached?.pages[0].articles.find((a) => a.id === 2);
    expect(found?.title).toBe("updated");
    handle2.unmount();
  });
});
