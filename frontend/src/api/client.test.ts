import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClient } from "./client";

beforeEach(() => {
  // Silence logger debug/info output during tests; warn/error still surface so we
  // explicitly mock them when we assert on behaviour.
  vi.spyOn(console, "debug").mockImplementation(() => {});
  vi.spyOn(console, "info").mockImplementation(() => {});
  vi.spyOn(console, "warn").mockImplementation(() => {});
  vi.spyOn(console, "error").mockImplementation(() => {});
});

afterEach(() => {
  vi.restoreAllMocks();
});

function createStream(chunks: string[]) {
  const encoder = new TextEncoder();
  return new ReadableStream<Uint8Array>({
    start(controller) {
      chunks.forEach((chunk) => controller.enqueue(encoder.encode(chunk)));
      controller.close();
    },
  });
}

function jsonResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
    ...init,
  });
}

describe("ApiClient request error handling", () => {
  it("throws the server error field for non-200 JSON responses", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ error: "feed not found" }, { status: 404 }),
    );
    await expect(client.listFeeds()).rejects.toThrowError("feed not found");
  });

  it("falls back to HTTP status text when error body is missing", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("<html>500</html>", { status: 500 }),
    );
    await expect(client.listFeeds()).rejects.toThrowError("HTTP 500");
  });

  it("surfaces HTTP 401 with server error verbatim", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ error: "unauthorized" }, { status: 401 }),
    );
    await expect(client.getArticle(1)).rejects.toThrowError("unauthorized");
  });

  it("does not panic when 200 response body is not valid JSON", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("not-json", { status: 200 }),
    );
    // malformed body is swallowed to `{}` — listFeeds returns default empty list
    await expect(client.listFeeds()).resolves.toEqual([]);
  });

  it("propagates network errors (e.g. AbortError) from fetch", async () => {
    const client = new ApiClient("http://example.com");
    const err = new DOMException("aborted", "AbortError");
    vi.spyOn(globalThis, "fetch").mockRejectedValue(err);
    await expect(client.listFeeds()).rejects.toBe(err);
  });

  it("propagates generic (non-Error) rejection values via String()", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockRejectedValue("boom");
    await expect(client.listFeeds()).rejects.toBe("boom");
  });
});

describe("ApiClient buildURL / baseURL handling", () => {
  it("strips trailing slash from baseURL", async () => {
    const client = new ApiClient("http://example.com/");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ feeds: [] }));
    await client.listFeeds();
    expect(String(fetchMock.mock.calls[0]?.[0])).toBe("http://example.com/api/v1/feeds");
  });

  it("uses the most recent baseURL per instance (late-bound through constructor)", async () => {
    const c1 = new ApiClient("http://first.example");
    const c2 = new ApiClient("http://second.example");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ feeds: [] }));
    await c1.listFeeds();
    await c2.listFeeds();
    expect(String(fetchMock.mock.calls[0]?.[0])).toContain("first.example");
    expect(String(fetchMock.mock.calls[1]?.[0])).toContain("second.example");
  });
});

describe("listArticles / listArticlesPage query wiring", () => {
  it("appends sort=latest and feed_id to the list query", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ articles: [] }));
    await client.listArticlesInScope({ page: 1, limit: 20, sort: "latest", feedID: 42 });
    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).toContain("sort=latest");
    expect(url).toContain("feed_id=42");
    expect(url).toContain("page=1");
  });

  it("omits query string entirely when no options are given", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ articles: [] }));
    await client.listArticles();
    expect(String(fetchMock.mock.calls[0]?.[0])).toBe("http://example.com/api/v1/articles");
  });

  it("returns empty list when server omits the articles field", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(jsonResponse({}));
    await expect(client.listArticles()).resolves.toEqual([]);
  });

  it("hasMore=false when exactly limit items are returned", async () => {
    const client = new ApiClient("http://example.com");
    const articles = Array.from({ length: 2 }, (_, idx) => ({ id: idx + 1 }));
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ articles }));
    const result = await client.listArticlesPage(1, 2);
    expect(result.articles).toHaveLength(2);
    expect(result.hasMore).toBe(false);
    // limit+1 pattern: frontend should have asked for 3
    expect(String(fetchMock.mock.calls[0]?.[0])).toContain("limit=3");
  });

  it("hasMore=true drops the extra sentinel row returned by the backend", async () => {
    const client = new ApiClient("http://example.com");
    const articles = Array.from({ length: 3 }, (_, idx) => ({ id: idx + 1 }));
    vi.spyOn(globalThis, "fetch").mockResolvedValue(jsonResponse({ articles }));
    const result = await client.listArticlesPage(1, 2, "latest", { feedID: 1 });
    expect(result.articles).toHaveLength(2);
    expect(result.hasMore).toBe(true);
  });

  it("preserves string IDs that exceed JS Number.MAX_SAFE_INTEGER", async () => {
    const client = new ApiClient("http://example.com");
    // Backend contract: IDs past 2^53-1 come as strings to avoid precision loss.
    const hugeId = "9007199254740993"; // MAX_SAFE_INTEGER + 2
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ articles: [{ id: hugeId, title: "t" }] }),
    );
    const rows = (await client.listArticles()) as unknown as Array<{ id: string }>;
    expect(rows[0]?.id).toBe(hugeId);
    expect(typeof rows[0]?.id).toBe("string");
  });
});

describe("ApiClient mutating endpoints", () => {
  it("createFeed sends url + folder_id in the POST body", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ id: 1 }));
    await client.createFeed("https://blog.example/rss", 3);
    const [, init] = fetchMock.mock.calls[0] ?? [];
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({
      url: "https://blog.example/rss",
      folder_id: 3,
    });
  });

  it("createFeed defaults folder_id to null when omitted", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ id: 1 }));
    await client.createFeed("https://blog.example/rss");
    const [, init] = fetchMock.mock.calls[0] ?? [];
    expect(JSON.parse(String(init?.body))).toEqual({
      url: "https://blog.example/rss",
      folder_id: null,
    });
  });

  it("createFolder nests under parent_id", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ id: 2 }));
    await client.createFolder("news", 7);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(String(url)).toBe("http://example.com/api/v1/folders");
    expect(JSON.parse(String(init?.body))).toEqual({ name: "news", parent_id: 7 });
  });

  it("updateFolder patches existing folder", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ id: 2 }));
    await client.updateFolder(2, "renamed", null);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(String(url)).toBe("http://example.com/api/v1/folders/2");
    expect(init?.method).toBe("PATCH");
    expect(JSON.parse(String(init?.body))).toEqual({ name: "renamed", parent_id: null });
  });

  it("deleteFolder issues DELETE", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({}));
    await client.deleteFolder(5);
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("DELETE");
  });

  it("listFolders returns [] when backend omits folders", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(jsonResponse({}));
    await expect(client.listFolders()).resolves.toEqual([]);
  });

  it("updateFeedFolder forwards folder_id via updateFeedSettings", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ id: 10 }));
    await client.updateFeedFolder(10, 42);
    const [url, init] = fetchMock.mock.calls[0] ?? [];
    expect(String(url)).toBe("http://example.com/api/v1/feeds/10");
    expect(init?.method).toBe("PATCH");
    expect(JSON.parse(String(init?.body))).toEqual({ folder_id: 42 });
  });

  it("deleteFeed / refreshFeed / updateFeedScript / updateFeedTitle target the correct paths", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({}));
    await client.deleteFeed(1);
    await client.refreshFeed(2);
    await client.updateFeedScript(3, "echo hi", "shell");
    await client.updateFeedTitle(4, "new title");
    const calls = fetchMock.mock.calls.map((c) => [String(c[0]), c[1]?.method] as const);
    expect(calls).toEqual([
      ["http://example.com/api/v1/feeds/1", "DELETE"],
      ["http://example.com/api/v1/feeds/2/refresh", "POST"],
      ["http://example.com/api/v1/feeds/3/script", "PATCH"],
      ["http://example.com/api/v1/feeds/4/title", "PATCH"],
    ]);
    expect(JSON.parse(String(fetchMock.mock.calls[2]?.[1]?.body))).toEqual({
      script: "echo hi",
      script_lang: "shell",
    });
  });

  it("article mutation helpers (read/favorite/readable/refresh-cache/clear/refresh AI)", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ id: 1 }));
    await client.setArticleRead(1, true);
    await client.setArticleFavorite(1, false);
    await client.extractArticleReadable(1);
    await client.refreshArticleCache(1);
    await client.clearArticleAISummary(1);
    await client.refreshArticleAISummary(1);
    await client.clearRecentAISummaries();
    const paths = fetchMock.mock.calls.map((c) => String(c[0]));
    expect(paths).toContain("http://example.com/api/v1/articles/1/read");
    expect(paths).toContain("http://example.com/api/v1/articles/1/favorite");
    expect(paths).toContain("http://example.com/api/v1/articles/1/readability");
    expect(paths).toContain("http://example.com/api/v1/articles/1/refresh-cache");
    expect(paths).toContain("http://example.com/api/v1/dev/articles/1/clear-ai-summary");
    expect(paths).toContain("http://example.com/api/v1/dev/articles/1/refresh-ai-summary");
    expect(paths).toContain("http://example.com/api/v1/dev/articles/clear-recent-ai-summaries");
  });
});

describe("translateArticle (non-streaming)", () => {
  it("defaults to zh-CN when no target language is provided", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ translated_text: "你好", target_lang: "zh-CN", article_id: 1 }),
    );
    const result = await client.translateArticle(1);
    expect(result.translated_text).toBe("你好");
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      target_lang: "zh-CN",
    });
  });

  it("honours an explicit target language", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ translated_text: "hola", target_lang: "es", article_id: 1 }),
    );
    await client.translateArticle(1, "es");
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1]?.body))).toEqual({
      target_lang: "es",
    });
  });
});

describe("translateArticleStream", () => {
  it("skips invalid stream lines instead of throwing", async () => {
    const client = new ApiClient("http://example.com");
    const onEvent = vi.fn();
    const stream = createStream([
      '{"type":"start","article_id":1,"target_lang":"zh-CN","total":1}\n',
      "bad-json\n",
    ]);
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(stream, { status: 200 }));

    await expect(client.translateArticleStream(1, "zh-CN", [], onEvent)).resolves.toBeUndefined();
    expect(onEvent).toHaveBeenCalledTimes(1);
  });

  it("flushes a trailing event that arrives without a terminating newline", async () => {
    const client = new ApiClient("http://example.com");
    const onEvent = vi.fn();
    const stream = createStream([
      '{"type":"start","article_id":1,"target_lang":"zh-CN","total":1}\n',
      '{"type":"done","article_id":1,"total":1}',
    ]);
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(stream, { status: 200 }));
    await client.translateArticleStream(1, "zh-CN", ["summary"], onEvent);
    expect(onEvent).toHaveBeenCalledTimes(2);
    expect(onEvent.mock.calls[1]?.[0]).toMatchObject({ type: "done", article_id: 1 });
  });

  it("drops an invalid trailing fragment silently", async () => {
    const client = new ApiClient("http://example.com");
    const onEvent = vi.fn();
    const stream = createStream([
      '{"type":"start","article_id":1,"target_lang":"zh-CN","total":1}\n',
      "trailing-garbage",
    ]);
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(stream, { status: 200 }));
    await expect(client.translateArticleStream(1, "zh-CN", [], onEvent)).resolves.toBeUndefined();
    expect(onEvent).toHaveBeenCalledTimes(1);
  });

  it("ignores empty lines between events", async () => {
    const client = new ApiClient("http://example.com");
    const onEvent = vi.fn();
    const stream = createStream([
      '{"type":"start","article_id":1,"target_lang":"zh-CN","total":1}\n',
      "\n",
      '{"type":"done","article_id":1,"total":1}\n',
    ]);
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(stream, { status: 200 }));
    await client.translateArticleStream(1, "zh-CN", [], onEvent);
    expect(onEvent).toHaveBeenCalledTimes(2);
  });

  it("throws the server error when the stream request fails", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ error: "translation disabled" }, { status: 400 }),
    );
    await expect(
      client.translateArticleStream(1, "zh-CN", [], () => {}),
    ).rejects.toThrowError("translation disabled");
  });

  it("throws HTTP status when the failed stream response has no error body", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("oops", { status: 503 }),
    );
    await expect(
      client.translateArticleStream(1, "zh-CN", [], () => {}),
    ).rejects.toThrowError("HTTP 503");
  });

  it("throws when the 200 response has no body", async () => {
    const client = new ApiClient("http://example.com");
    // Construct a Response whose body is null (simulates pre-fetch stream).
    const resp = new Response(null, { status: 200 });
    // Node's Response may still expose an empty body; force null for the branch.
    Object.defineProperty(resp, "body", { value: null });
    vi.spyOn(globalThis, "fetch").mockResolvedValue(resp);
    await expect(
      client.translateArticleStream(1, "zh-CN", [], () => {}),
    ).rejects.toThrowError("stream body is empty");
  });

  it("propagates fetch abort errors", async () => {
    const client = new ApiClient("http://example.com");
    const err = new DOMException("aborted", "AbortError");
    vi.spyOn(globalThis, "fetch").mockRejectedValue(err);
    await expect(
      client.translateArticleStream(1, "zh-CN", [], () => {}),
    ).rejects.toBe(err);
  });
});

describe("blob export / import flows", () => {
  it("exportProfile returns the raw Blob on success", async () => {
    const client = new ApiClient("http://example.com");
    const blob = new Blob(["{}"], { type: "application/json" });
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(blob, { status: 200 }));
    const got = await client.exportProfile();
    expect(got).toBeInstanceOf(Blob);
  });

  it("exportProfile throws HTTP error when backend rejects", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("", { status: 500 }));
    await expect(client.exportProfile()).rejects.toThrowError("HTTP 500");
  });

  it("exportOPML returns the XML Blob on success", async () => {
    const client = new ApiClient("http://example.com");
    const blob = new Blob(["<opml/>"], { type: "text/xml" });
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(blob, { status: 200 }));
    const got = await client.exportOPML();
    expect(got).toBeInstanceOf(Blob);
  });

  it("exportOPML throws when backend responds non-200", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response("", { status: 500 }));
    await expect(client.exportOPML()).rejects.toThrowError("HTTP 500");
  });

  it("importOPML posts XML body and returns parsed counts", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ imported_feeds: 3, updated_feeds: 1 }));
    const result = await client.importOPML("<opml/>");
    expect(result).toEqual({ imported_feeds: 3, updated_feeds: 1 });
    const [, init] = fetchMock.mock.calls[0] ?? [];
    expect(init?.method).toBe("POST");
    expect(init?.body).toBe("<opml/>");
    expect(
      (init?.headers as Record<string, string> | undefined)?.["Content-Type"],
    ).toMatch(/^text\/xml/);
  });

  it("importOPML surfaces the backend error field on failure", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      jsonResponse({ error: "invalid opml" }, { status: 400 }),
    );
    await expect(client.importOPML("<bad/>")).rejects.toThrowError("invalid opml");
  });

  it("importOPML falls back to HTTP status when error body is absent", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("oops", { status: 502 }),
    );
    await expect(client.importOPML("<bad/>")).rejects.toThrowError("HTTP 502");
  });

  it("importProfile forwards the raw JSON body unchanged", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ imported_feeds: 0 }));
    await client.importProfile('{"foo":1}');
    const [, init] = fetchMock.mock.calls[0] ?? [];
    expect(init?.body).toBe('{"foo":1}');
    expect(init?.method).toBe("POST");
  });
});

describe("settings endpoints", () => {
  it("getNetworkSettings / updateNetworkSettings", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ proxy_url: "http://proxy.example" }));
    await client.getNetworkSettings();
    await client.updateNetworkSettings("http://proxy.example");
    expect(fetchMock.mock.calls[0]?.[1]?.method ?? "GET").toBe("GET");
    expect(fetchMock.mock.calls[1]?.[1]?.method).toBe("PATCH");
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toEqual({
      proxy_url: "http://proxy.example",
    });
  });

  it("getAISettings / getDataSettings / updateDataSettings / regenerateSummaries", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ retention_days: 30, refreshed: 4 }));
    await client.getAISettings();
    await client.getDataSettings();
    await client.updateDataSettings(45);
    await client.regenerateSummaries();
    expect(String(fetchMock.mock.calls[0]?.[0])).toContain("/api/v1/settings/ai");
    expect(String(fetchMock.mock.calls[1]?.[0])).toContain("/api/v1/settings/data");
    expect(fetchMock.mock.calls[2]?.[1]?.method).toBe("PATCH");
    expect(JSON.parse(String(fetchMock.mock.calls[2]?.[1]?.body))).toEqual({
      retention_days: 45,
    });
    expect(String(fetchMock.mock.calls[3]?.[0])).toContain("regenerate-summaries");
    expect(fetchMock.mock.calls[3]?.[1]?.method).toBe("POST");
  });

  it("updateAISettings sends separated embedding fields", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ protocol: "openai" }));

    await client.updateAISettings({
      protocol: "openai",
      api_key: "chat-key",
      base_url: "https://chat.example/v1",
      model: "chat-model",
      target_lang: "zh-CN",
      embedding_api_key: "embed-key",
      embedding_base_url: "https://embed.example/v1",
      embedding_model: "text-embedding-3-small",
    });

    const [, init] = fetchMock.mock.calls[0] ?? [];
    expect(JSON.parse(String(init?.body))).toMatchObject({
      embedding_api_key: "embed-key",
      embedding_base_url: "https://embed.example/v1",
      embedding_model: "text-embedding-3-small",
    });
  });
});

describe("topics / briefs / interests / agents", () => {
  it("listTopics unwraps `data` envelope and defaults to []", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(jsonResponse({}));
    await expect(client.listTopics()).resolves.toEqual([]);
  });

  it("getTopic returns a synthetic empty cluster when server omits data", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(jsonResponse({}));
    const result = await client.getTopic(99);
    expect(result.members).toEqual([]);
    expect(result.cluster.id).toBe(0);
  });

  it("getArticleCluster scans topics and returns the matching cluster", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi.spyOn(globalThis, "fetch");
    fetchMock.mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/topics")) {
        return jsonResponse({
          data: [
            { id: 1, title: "a", summary: "", article_count: 0, status: "", created_at: "", updated_at: "" },
            { id: 2, title: "b", summary: "", article_count: 0, status: "", created_at: "", updated_at: "" },
          ],
        });
      }
      if (url.endsWith("/api/v1/topics/1")) {
        return jsonResponse({ data: { cluster: { id: 1 }, members: [] } });
      }
      if (url.endsWith("/api/v1/topics/2")) {
        return jsonResponse({
          data: {
            cluster: { id: 2, title: "b", summary: "", article_count: 0, status: "", created_at: "", updated_at: "" },
            members: [{ article_id: 77 }],
          },
        });
      }
      return jsonResponse({}, { status: 404 });
    });
    const cluster = await client.getArticleCluster(77);
    expect(cluster?.id).toBe(2);
  });

  it("getArticleCluster returns null when no topic contains the article", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/api/v1/topics")) {
        return jsonResponse({ data: [{ id: 1 }] });
      }
      return jsonResponse({ data: { cluster: { id: 1 }, members: [] } });
    });
    await expect(client.getArticleCluster(999)).resolves.toBeNull();
  });

  it("getArticleCluster swallows errors and returns null", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("network"));
    await expect(client.getArticleCluster(1)).resolves.toBeNull();
  });

  it("listBriefs defaults to daily / limit 20 and encodes both in the URL", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ data: [] }));
    await client.listBriefs();
    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).toContain("level=daily");
    expect(url).toContain("limit=20");
  });

  it("listBriefs honours explicit level and limit", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ data: [] }));
    await client.listBriefs("weekly", 5);
    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).toContain("level=weekly");
    expect(url).toContain("limit=5");
  });

  it("getBrief returns null when server omits data", async () => {
    const client = new ApiClient("http://example.com");
    vi.spyOn(globalThis, "fetch").mockResolvedValue(jsonResponse({}));
    await expect(client.getBrief(1)).resolves.toBeNull();
  });

  it("listInterests / updateInterestWeight / deleteInterest path + body", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ data: { id: 5, weight: 0.8 } }));
    await client.listInterests();
    await client.updateInterestWeight(5, 0.8);
    await client.deleteInterest(5);
    const calls = fetchMock.mock.calls.map((c) => [String(c[0]), c[1]?.method] as const);
    expect(calls[0]?.[0]).toContain("/api/v1/interests");
    expect(calls[1]?.[1]).toBe("PATCH");
    expect(JSON.parse(String(fetchMock.mock.calls[1]?.[1]?.body))).toEqual({ weight: 0.8 });
    expect(calls[2]?.[1]).toBe("DELETE");
  });

  it("listAgentRuns appends type and limit when specified", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ data: [] }));
    await client.listAgentRuns("scoring", 5);
    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).toContain("type=scoring");
    expect(url).toContain("limit=5");
  });

  it("listAgentRuns omits the type parameter when not provided", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({ data: [] }));
    await client.listAgentRuns();
    const url = String(fetchMock.mock.calls[0]?.[0]);
    expect(url).not.toContain("type=");
    expect(url).toContain("limit=20");
  });

  it("triggerAgent POSTs to /trigger/{type}", async () => {
    const client = new ApiClient("http://example.com");
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(jsonResponse({}));
    await client.triggerAgent("scoring");
    expect(String(fetchMock.mock.calls[0]?.[0])).toBe(
      "http://example.com/api/v1/agents/trigger/scoring",
    );
    expect(fetchMock.mock.calls[0]?.[1]?.method).toBe("POST");
  });
});
