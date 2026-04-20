/* @vitest-environment node */
import { describe, expect, it, vi } from "vitest";
import type { ApiClient } from "@/api";
import type { Article } from "@/types";
import type { TranslateStreamEvent } from "@/api/client";
import { useArticleActions, type TranslationParagraph } from "./useArticleActions";

// useArticleActions 不调用任何 React hook，是纯 dispatcher 工厂，可直接函数调用。
// 这里用一个可控 stub 替换 client，覆盖成功 / 失败 / 去重 / 乐观更新回滚四条关键分支。

type Setter<T> = (updater: T | ((prev: T) => T)) => void;

function makeArticle(overrides: Partial<Article> = {}): Article {
  return {
    id: 1,
    feed_id: 1,
    title: "demo",
    link: "",
    summary: "",
    published_at: "",
    created_at: "",
    is_read: false,
    is_favorite: false,
    ...overrides,
  };
}

type Harness = {
  selectedArticle: Article | null;
  articles: Article[];
  messages: Array<{ text: string; isError: boolean }>;
  translationParagraphs: Record<number, TranslationParagraph[]>;
  translationVisible: Record<number, boolean>;
  translationRunning: Record<number, boolean>;
  isExtractingReadable: boolean;
  isRefreshingArticleCache: boolean;
};

type HarnessOptions = {
  selectedArticle?: Article | null;
  articles?: Article[];
  translationRunning?: Record<number, boolean>;
  translationVisible?: Record<number, boolean>;
  translationParagraphs?: Record<number, TranslationParagraph[]>;
  isExtractingReadable?: boolean;
  isRefreshingArticleCache?: boolean;
};

function buildHarness(client: Partial<ApiClient>, opts: HarnessOptions = {}) {
  const state: Harness = {
    selectedArticle: "selectedArticle" in opts ? (opts.selectedArticle ?? null) : makeArticle(),
    articles: opts.articles ?? [makeArticle()],
    messages: [],
    translationParagraphs: opts.translationParagraphs ?? {},
    translationVisible: opts.translationVisible ?? {},
    translationRunning: opts.translationRunning ?? {},
    isExtractingReadable: opts.isExtractingReadable ?? false,
    isRefreshingArticleCache: opts.isRefreshingArticleCache ?? false,
  };

  const applySelected: Setter<Article | null> = (updater) => {
    const next = typeof updater === "function" ? (updater as (prev: Article | null) => Article | null)(state.selectedArticle) : updater;
    state.selectedArticle = next;
  };
  const applyArticles: Setter<Article[]> = (updater) => {
    const next = typeof updater === "function" ? (updater as (prev: Article[]) => Article[])(state.articles) : updater;
    state.articles = next;
  };
  const applyTranslationParagraphs: Setter<Record<number, TranslationParagraph[]>> = (updater) => {
    const next =
      typeof updater === "function"
        ? (updater as (prev: Record<number, TranslationParagraph[]>) => Record<number, TranslationParagraph[]>)(state.translationParagraphs)
        : updater;
    state.translationParagraphs = next;
  };
  const applyTranslationVisible: Setter<Record<number, boolean>> = (updater) => {
    const next =
      typeof updater === "function"
        ? (updater as (prev: Record<number, boolean>) => Record<number, boolean>)(state.translationVisible)
        : updater;
    state.translationVisible = next;
  };
  const applyTranslationRunning: Setter<Record<number, boolean>> = (updater) => {
    const next =
      typeof updater === "function"
        ? (updater as (prev: Record<number, boolean>) => Record<number, boolean>)(state.translationRunning)
        : updater;
    state.translationRunning = next;
  };
  const applyIsExtracting: Setter<boolean> = (updater) => {
    const next = typeof updater === "function" ? (updater as (prev: boolean) => boolean)(state.isExtractingReadable) : updater;
    state.isExtractingReadable = next;
  };
  const applyIsRefreshingCache: Setter<boolean> = (updater) => {
    const next = typeof updater === "function" ? (updater as (prev: boolean) => boolean)(state.isRefreshingArticleCache) : updater;
    state.isRefreshingArticleCache = next;
  };

  const actions = useArticleActions({
    client: client as ApiClient,
    selectedArticle: state.selectedArticle,
    setSelectedArticle: applySelected as never,
    setArticles: applyArticles as never,
    setMessage: (message, isError) => state.messages.push({ text: message, isError: Boolean(isError) }),
    isExtractingReadable: state.isExtractingReadable,
    setIsExtractingReadable: applyIsExtracting as never,
    isRefreshingArticleCache: state.isRefreshingArticleCache,
    setIsRefreshingArticleCache: applyIsRefreshingCache as never,
    aiTargetLang: "zh-CN",
    translationSources: ["p1", "p2"],
    translationParagraphsByArticleID: state.translationParagraphs,
    setTranslationParagraphsByArticleID: applyTranslationParagraphs as never,
    translationVisibleByArticleID: state.translationVisible,
    setTranslationVisibleByArticleID: applyTranslationVisible as never,
    translationRunningByArticleID: state.translationRunning,
    setTranslationRunningByArticleID: applyTranslationRunning as never,
  });

  return { state, actions };
}

describe("useArticleActions", () => {
  it("markUnread: API 成功后同步 selectedArticle 和列表并提示文案", async () => {
    const updated = makeArticle({ is_read: false });
    const client = { setArticleRead: vi.fn().mockResolvedValue(updated) };
    const { state, actions } = buildHarness(client, {
      selectedArticle: makeArticle({ is_read: true }),
      articles: [makeArticle({ is_read: true })],
    });

    await actions.markUnread();

    expect(client.setArticleRead).toHaveBeenCalledWith(1, false);
    expect(state.selectedArticle?.is_read).toBe(false);
    expect(state.articles[0].is_read).toBe(false);
    expect(state.messages[state.messages.length - 1]).toEqual({ text: "文章已标记为未读", isError: false });
  });

  it("markUnread: 无 selectedArticle 时直接 return，不调用 API", async () => {
    const client = { setArticleRead: vi.fn() };
    const { state, actions } = buildHarness(client, { selectedArticle: null });
    await actions.markUnread();
    expect(client.setArticleRead).not.toHaveBeenCalled();
    expect(state.messages).toHaveLength(0);
  });

  it("markUnread: API 失败时把 error message 通过 setMessage 暴露", async () => {
    const client = { setArticleRead: vi.fn().mockRejectedValue(new Error("network down")) };
    const { state, actions } = buildHarness(client);
    await actions.markUnread();
    expect(state.messages).toEqual([{ text: "network down", isError: true }]);
  });

  it("toggleFavorite: 成功分支翻转 is_favorite 并更新 favorited_at", async () => {
    const updated = makeArticle({ is_favorite: true, favorited_at: "2026-04-20T00:00:00Z" });
    const client = { setArticleFavorite: vi.fn().mockResolvedValue(updated) };
    const { state, actions } = buildHarness(client, {
      selectedArticle: makeArticle({ is_favorite: false }),
      articles: [makeArticle({ is_favorite: false })],
    });

    await actions.toggleFavorite();

    expect(client.setArticleFavorite).toHaveBeenCalledWith(1, true);
    expect(state.selectedArticle?.is_favorite).toBe(true);
    expect(state.articles[0].favorited_at).toBe("2026-04-20T00:00:00Z");
    expect(state.messages[state.messages.length - 1]).toEqual({ text: "已加入收藏", isError: false });
  });

  it("toggleFavorite: 取消收藏也走同一 setMessage 分支", async () => {
    const updated = makeArticle({ is_favorite: false });
    const client = { setArticleFavorite: vi.fn().mockResolvedValue(updated) };
    const { state, actions } = buildHarness(client, {
      selectedArticle: makeArticle({ is_favorite: true }),
    });
    await actions.toggleFavorite();
    expect(state.messages[state.messages.length - 1]).toEqual({ text: "已取消收藏", isError: false });
  });

  it("toggleFavorite: API 失败时 selectedArticle 不被覆盖（无乐观更新）", async () => {
    const client = { setArticleFavorite: vi.fn().mockRejectedValue(new Error("boom")) };
    const { state, actions } = buildHarness(client, {
      selectedArticle: makeArticle({ is_favorite: false }),
    });
    await actions.toggleFavorite();
    expect(state.selectedArticle?.is_favorite).toBe(false);
    expect(state.messages).toEqual([{ text: "boom", isError: true }]);
  });

  it("extractReadableContent: 运行中拒绝并发调用", async () => {
    const client = { extractArticleReadable: vi.fn().mockResolvedValue(makeArticle()) };
    const { actions } = buildHarness(client, { isExtractingReadable: true });
    await actions.extractReadableContent();
    expect(client.extractArticleReadable).not.toHaveBeenCalled();
  });

  it("extractReadableContent: 成功分支发出开始/完成两条 message 并 finally 复位", async () => {
    const updated = makeArticle({ full_content: "<p>ok</p>" });
    const client = { extractArticleReadable: vi.fn().mockResolvedValue(updated) };
    const { state, actions } = buildHarness(client);
    await actions.extractReadableContent();
    expect(state.messages.map((m) => m.text)).toEqual(["正在使用 Readability 抓取原文...", "原文抓取完成"]);
    expect(state.isExtractingReadable).toBe(false);
  });

  it("extractReadableContent: 失败也必须 finally 复位 isExtractingReadable", async () => {
    const client = { extractArticleReadable: vi.fn().mockRejectedValue(new Error("timeout")) };
    const { state, actions } = buildHarness(client);
    await actions.extractReadableContent();
    expect(state.isExtractingReadable).toBe(false);
    expect(state.messages[state.messages.length - 1]).toEqual({ text: "timeout", isError: true });
  });

  it("refreshCurrentArticleCache: 成功后清空 translation 三件套并复位 loading", async () => {
    const updated = makeArticle();
    const client = { refreshArticleCache: vi.fn().mockResolvedValue(updated) };
    const { state, actions } = buildHarness(client, {
      translationParagraphs: { 1: [{ index: 1, source: "s", translated: "t", status: "done" }] },
      translationVisible: { 1: true },
      translationRunning: { 1: true },
    });
    await actions.refreshCurrentArticleCache();
    expect(state.translationParagraphs[1]).toEqual([]);
    expect(state.translationVisible[1]).toBe(false);
    expect(state.translationRunning[1]).toBe(false);
    expect(state.isRefreshingArticleCache).toBe(false);
  });

  it("translateArticle: 运行中再次点击只切换可见性，不重复发流", async () => {
    const client = { translateArticleStream: vi.fn() };
    const { state, actions } = buildHarness(client, {
      translationRunning: { 1: true },
      translationVisible: { 1: false },
    });
    await actions.translateArticle();
    expect(client.translateArticleStream).not.toHaveBeenCalled();
    expect(state.translationVisible[1]).toBe(true);
    expect(state.messages[state.messages.length - 1]?.text).toBe("已切换到译文模式");
  });

  it("translateArticle: 已有翻译缓存时也只切可见性", async () => {
    const client = { translateArticleStream: vi.fn() };
    const { state, actions } = buildHarness(client, {
      translationParagraphs: { 1: [{ index: 1, source: "s", translated: "t", status: "done" }] },
      translationVisible: { 1: true },
    });
    await actions.translateArticle();
    expect(client.translateArticleStream).not.toHaveBeenCalled();
    expect(state.translationVisible[1]).toBe(false);
    expect(state.messages[state.messages.length - 1]?.text).toBe("已切回原文模式");
  });

  it("translateArticle: 首次翻译走 start → chunk → done 流程并填充段落", async () => {
    const client = {
      translateArticleStream: vi.fn(
        async (_id: number, _lang: string, _sources: string[], onEvent: (event: TranslateStreamEvent) => void) => {
          onEvent({ type: "start", article_id: 1, target_lang: "zh-CN", total: 2, sources: ["p1", "p2"] });
          onEvent({ type: "chunk", article_id: 1, target_lang: "zh-CN", total: 2, index: 1, source: "p1", translated: "段1" });
          onEvent({ type: "chunk", article_id: 1, target_lang: "zh-CN", total: 2, index: 2, source: "p2", translated: "段2" });
        },
      ),
    };
    const { state, actions } = buildHarness(client);
    await actions.translateArticle();
    expect(state.translationParagraphs[1]).toHaveLength(2);
    expect(state.translationParagraphs[1][0].translated).toBe("段1");
    expect(state.translationParagraphs[1][1].status).toBe("done");
    expect(state.translationRunning[1]).toBe(false); // finally 复位
    expect(state.messages[state.messages.length - 1]?.text).toBe("AI 翻译完成");
  });

  it("translateArticle: stream error 事件被抛出并走 setMessage error 分支", async () => {
    const client = {
      translateArticleStream: vi.fn(
        async (_id: number, _lang: string, _sources: string[], onEvent: (event: TranslateStreamEvent) => void) => {
          onEvent({ type: "error", article_id: 1, error: "stream fail" });
        },
      ),
    };
    const { state, actions } = buildHarness(client);
    await actions.translateArticle();
    expect(state.messages[state.messages.length - 1]).toEqual({ text: "stream fail", isError: true });
    expect(state.translationRunning[1]).toBe(false);
  });
});
