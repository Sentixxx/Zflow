/* @vitest-environment jsdom */
import { describe, expect, it, vi } from "vitest";
import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import type { ApiClient } from "@/api";
import type { Article, Feed } from "@/types";
import { useSettingsActions } from "./useSettingsActions";

type GlobalWithAct = typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean };
type Returned = ReturnType<typeof useSettingsActions>;

type DraftState = {
  networkProxyURL: string;
  aiProtocol: "openai" | "anthropic";
  aiAPIKey: string;
  aiAPIKeyMasked: string;
  aiAPIKeyConfigured: boolean;
  aiBaseURL: string;
  aiModel: string;
  aiTargetLang: string;
  embeddingAPIKey: string;
  embeddingAPIKeyMasked: string;
  embeddingAPIKeyConfigured: boolean;
  embeddingBaseURL: string;
  embeddingModel: string;
  articleRetentionDays: string;
  scriptFeedID: number | null;
  scriptContent: string;
  scriptLang: "shell" | "python" | "javascript";
  scriptDirty: boolean;
};

function defaultDraft(): DraftState {
  return {
    networkProxyURL: "",
    aiProtocol: "openai",
    aiAPIKey: "",
    aiAPIKeyMasked: "",
    aiAPIKeyConfigured: false,
    aiBaseURL: "",
    aiModel: "",
    aiTargetLang: "zh-CN",
    embeddingAPIKey: "",
    embeddingAPIKeyMasked: "",
    embeddingAPIKeyConfigured: false,
    embeddingBaseURL: "",
    embeddingModel: "",
    articleRetentionDays: "7",
    scriptFeedID: null,
    scriptContent: "",
    scriptLang: "shell",
    scriptDirty: false,
  };
}

type HarnessOpts = {
  draft?: Partial<DraftState>;
  feeds?: Feed[];
  selectedArticleID?: number | null;
};

function mount(
  client: Partial<ApiClient>,
  opts: HarnessOpts = {},
) {
  (globalThis as GlobalWithAct).IS_REACT_ACT_ENVIRONMENT = true;
  const draft = { ...defaultDraft(), ...opts.draft };
  const messages: Array<{ text: string; isError: boolean }> = [];
  const loadFeeds = vi.fn().mockResolvedValue(undefined);
  const loadFolders = vi.fn().mockResolvedValue(undefined);
  const loadArticles = vi.fn().mockResolvedValue(undefined);
  const onSelectedArticleUpdated = vi.fn();

  const mkSetter = <K extends keyof DraftState>(key: K) =>
    ((updater: DraftState[K] | ((prev: DraftState[K]) => DraftState[K])) => {
      const next = typeof updater === "function" ? (updater as (prev: DraftState[K]) => DraftState[K])(draft[key]) : updater;
      draft[key] = next;
    }) as never;

  const container = document.createElement("div");
  const root = createRoot(container);
  const ref = { current: undefined as Returned | undefined };
  const Probe = () => {
    ref.current = useSettingsActions({
      client: client as ApiClient,
      feeds: opts.feeds ?? [],
      selectedArticleID: opts.selectedArticleID ?? null,
      selectedArticleTitle: "demo title",
      selectedArticleSummaryStatus: "ready",
      apiBase: "http://api.test",
      networkProxyURL: draft.networkProxyURL,
      aiProtocol: draft.aiProtocol,
      aiAPIKey: draft.aiAPIKey,
      setAIAPIKeyMasked: mkSetter("aiAPIKeyMasked"),
      setAIAPIKeyConfigured: mkSetter("aiAPIKeyConfigured"),
      aiBaseURL: draft.aiBaseURL,
      aiModel: draft.aiModel,
      aiTargetLang: draft.aiTargetLang,
      embeddingAPIKey: draft.embeddingAPIKey,
      setEmbeddingAPIKeyMasked: mkSetter("embeddingAPIKeyMasked"),
      setEmbeddingAPIKeyConfigured: mkSetter("embeddingAPIKeyConfigured"),
      embeddingBaseURL: draft.embeddingBaseURL,
      embeddingModel: draft.embeddingModel,
      articleRetentionDays: draft.articleRetentionDays,
      scriptFeedID: draft.scriptFeedID,
      scriptContent: draft.scriptContent,
      scriptLang: draft.scriptLang,
      setNetworkProxyURL: mkSetter("networkProxyURL"),
      setAIProtocol: mkSetter("aiProtocol"),
      setAIAPIKey: mkSetter("aiAPIKey"),
      setAIBaseURL: mkSetter("aiBaseURL"),
      setAIModel: mkSetter("aiModel"),
      setAITargetLang: mkSetter("aiTargetLang"),
      setEmbeddingAPIKey: mkSetter("embeddingAPIKey"),
      setEmbeddingBaseURL: mkSetter("embeddingBaseURL"),
      setEmbeddingModel: mkSetter("embeddingModel"),
      setArticleRetentionDays: mkSetter("articleRetentionDays"),
      setScriptFeedID: mkSetter("scriptFeedID"),
      setScriptContent: mkSetter("scriptContent"),
      setScriptLang: mkSetter("scriptLang"),
      setScriptDirty: mkSetter("scriptDirty"),
      loadFeeds,
      loadFolders,
      loadArticles,
      onSelectedArticleUpdated,
      setMessage: (message, isError) => messages.push({ text: message, isError: Boolean(isError) }),
    });
    return null;
  };
  act(() => {
    root.render(createElement(Probe));
  });

  return {
    get actions() {
      if (!ref.current) throw new Error("hook not ready");
      return ref.current;
    },
    draft,
    messages,
    loadFeeds,
    loadFolders,
    loadArticles,
    onSelectedArticleUpdated,
    unmount: () => {
      act(() => root.unmount());
    },
  };
}

function makeFeed(overrides: Partial<Feed> = {}): Feed {
  return {
    id: 1,
    url: "https://a/rss",
    title: "A",
    item_count: 0,
    last_fetched_at: "",
    last_fetch_status: "ok",
    created_at: "",
    ...overrides,
  };
}

function makeArticle(overrides: Partial<Article> = {}): Article {
  return {
    id: 1,
    feed_id: 1,
    title: "demo",
    link: "",
    is_read: false,
    is_favorite: false,
    created_at: "",
    ...overrides,
  };
}

describe("useSettingsActions", () => {
  it("saveAISettings 成功路径：回写服务器返回的字段并提示保存", async () => {
    const server = {
      protocol: "anthropic",
      api_key: "real-key",
      api_key_masked: "re******ey",
      api_key_configured: true,
      base_url: "https://anthropic.test",
      model: "claude",
      target_lang: "zh-CN",
      embedding_api_key: "emb",
      embedding_api_key_masked: "em****",
      embedding_api_key_configured: true,
      embedding_base_url: "https://emb.test",
      embedding_model: "text",
    };
    const client = { updateAISettings: vi.fn().mockResolvedValue(server) };
    const h = mount(client, {
      draft: { aiProtocol: "anthropic", aiAPIKey: "  real-key  ", aiBaseURL: "https://anthropic.test" },
    });

    await act(async () => {
      await h.actions.saveAISettings();
    });

    // trim 在进 client 前已执行
    expect(client.updateAISettings).toHaveBeenCalledWith(expect.objectContaining({ api_key: "real-key" }));
    expect(h.draft.aiAPIKeyMasked).toBe("re******ey");
    expect(h.draft.aiAPIKeyConfigured).toBe(true);
    expect(h.messages[h.messages.length - 1]).toEqual({ text: "AI 设置已保存", isError: false });
    h.unmount();
  });

  it("saveAISettings 失败：setMessage 写 error、本地 draft 不会被 server 数据覆盖", async () => {
    const client = { updateAISettings: vi.fn().mockRejectedValue(new Error("server 500")) };
    const h = mount(client, {
      draft: { aiProtocol: "openai", aiAPIKey: "draft-key", aiModel: "draft-model" },
    });
    await act(async () => {
      await h.actions.saveAISettings();
    });
    expect(h.messages[h.messages.length - 1]).toEqual({ text: "server 500", isError: true });
    // 失败时 draft 保持不变
    expect(h.draft.aiAPIKey).toBe("draft-key");
    expect(h.draft.aiModel).toBe("draft-model");
    h.unmount();
  });

  it("saveAISettings: 服务器回返 protocol 与请求不一致 → 标红并提前 return，不回写 draft", async () => {
    const client = {
      updateAISettings: vi.fn().mockResolvedValue({
        protocol: "openai", // 请求的是 anthropic
        api_key: "other-key",
      }),
    };
    const h = mount(client, {
      draft: { aiProtocol: "anthropic", aiAPIKey: "draft" },
    });
    await act(async () => {
      await h.actions.saveAISettings();
    });
    expect(h.messages[h.messages.length - 1]).toEqual({ text: "AI 协议保存失败，请重试", isError: true });
    // 前置 return 保证 api_key 不被 other-key 覆盖
    expect(h.draft.aiAPIKey).toBe("draft");
    h.unmount();
  });

  it("saveDataSettings 校验：非法天数直接失败，不调用 API", async () => {
    const client = { updateDataSettings: vi.fn() };
    const h = mount(client, { draft: { articleRetentionDays: "0" } });
    await act(async () => {
      await h.actions.saveDataSettings();
    });
    expect(client.updateDataSettings).not.toHaveBeenCalled();
    expect(h.messages[h.messages.length - 1]).toEqual({ text: "文章保留天数需为 1-3650 的整数", isError: true });
    h.unmount();
  });

  it("saveDataSettings 合法路径：调用 client 并把服务端 value 回写 draft", async () => {
    const client = { updateDataSettings: vi.fn().mockResolvedValue({ retention_days: 14 }) };
    const h = mount(client, { draft: { articleRetentionDays: "14" } });
    await act(async () => {
      await h.actions.saveDataSettings();
    });
    expect(client.updateDataSettings).toHaveBeenCalledWith(14);
    expect(h.draft.articleRetentionDays).toBe("14");
    expect(h.messages[h.messages.length - 1]?.text).toBe("数据保留策略已保存");
    h.unmount();
  });

  it("refreshCurrentArticleAISummary: 无 selectedArticle 时报错并不调用 API", async () => {
    const client = { refreshArticleAISummary: vi.fn(), getArticle: vi.fn() };
    const h = mount(client, { selectedArticleID: null });
    await act(async () => {
      await h.actions.refreshCurrentArticleAISummary();
    });
    expect(client.refreshArticleAISummary).not.toHaveBeenCalled();
    expect(h.messages[h.messages.length - 1]).toEqual({ text: "当前没有选中的文章", isError: true });
    h.unmount();
  });

  it("refreshCurrentArticleAISummary: 成功路径触发 onSelectedArticleUpdated + loadArticles(silent) + 额外 getArticle 刷新", async () => {
    const updatedArticle = makeArticle({ id: 42, display_summary_status: "ready" });
    const client = {
      refreshArticleAISummary: vi.fn().mockResolvedValue(updatedArticle),
      getArticle: vi.fn().mockResolvedValue(updatedArticle),
    };
    const h = mount(client, { selectedArticleID: 42 });
    await act(async () => {
      await h.actions.refreshCurrentArticleAISummary();
    });
    expect(h.onSelectedArticleUpdated).toHaveBeenCalledWith(updatedArticle);
    expect(h.loadArticles).toHaveBeenCalledWith({ silentStatus: true });
    expect(client.getArticle).toHaveBeenCalledWith(42);
    h.unmount();
  });

  it("refreshCurrentArticleAISummary: 并发再调提示仍在刷新中（第二次调用被 dedup）", async () => {
    let resolveFirst: (value: Article) => void = () => {};
    const firstPromise = new Promise<Article>((resolve) => {
      resolveFirst = resolve;
    });
    const client = {
      refreshArticleAISummary: vi.fn().mockReturnValue(firstPromise),
      getArticle: vi.fn().mockResolvedValue(makeArticle({ id: 7 })),
    };
    const h = mount(client, { selectedArticleID: 7 });

    // 第一次不等待；第二次立刻触发，应该被 dedup
    let first: Promise<void> | undefined;
    act(() => {
      first = h.actions.refreshCurrentArticleAISummary();
    });
    await act(async () => {
      await h.actions.refreshCurrentArticleAISummary();
    });
    expect(client.refreshArticleAISummary).toHaveBeenCalledTimes(1);
    expect(h.messages.some((m) => m.text.startsWith("当前文章摘要仍在刷新中"))).toBe(true);

    // 收尾
    resolveFirst(makeArticle({ id: 7 }));
    await act(async () => {
      await first;
    });
    h.unmount();
  });

  it("saveFeedScript: 未选 feed 时直接失败", async () => {
    const client = { updateFeedScript: vi.fn() };
    const h = mount(client, { draft: { scriptFeedID: null } });
    await act(async () => {
      await h.actions.saveFeedScript();
    });
    expect(client.updateFeedScript).not.toHaveBeenCalled();
    expect(h.messages[h.messages.length - 1]).toEqual({ text: "请先选择订阅源", isError: true });
    h.unmount();
  });

  it("saveFeedScript: 选中 feed 时调用 API 并 loadFeeds", async () => {
    const client = { updateFeedScript: vi.fn().mockResolvedValue(makeFeed()) };
    const h = mount(client, {
      draft: { scriptFeedID: 9, scriptContent: "echo hi", scriptLang: "shell", scriptDirty: true },
    });
    await act(async () => {
      await h.actions.saveFeedScript();
    });
    expect(client.updateFeedScript).toHaveBeenCalledWith(9, "echo hi", "shell");
    expect(h.draft.scriptDirty).toBe(false);
    expect(h.loadFeeds).toHaveBeenCalled();
    h.unmount();
  });

  it("selectScriptFeed: 从 feeds 列表填入 script 字段；传 null 清空", () => {
    const feeds = [makeFeed({ id: 3, custom_script: "print()", custom_script_lang: "python" })];
    const h = mount({}, { draft: { scriptFeedID: null }, feeds });
    act(() => {
      h.actions.selectScriptFeed(3);
    });
    expect(h.draft.scriptContent).toBe("print()");
    expect(h.draft.scriptLang).toBe("python");

    act(() => {
      h.actions.selectScriptFeed(null);
    });
    expect(h.draft.scriptContent).toBe("");
    expect(h.draft.scriptLang).toBe("shell");
    h.unmount();
  });

  it("handleSaveAPIBase: 写入 localStorage 并提示", () => {
    const h = mount({});
    act(() => {
      h.actions.handleSaveAPIBase();
    });
    expect(localStorage.getItem("zflow_api_base")).toBe("http://api.test");
    expect(h.messages[h.messages.length - 1]?.text).toBe("API Base 已保存");
    h.unmount();
  });
});
