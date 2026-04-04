# Frontend Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修复 2026-04-04 前端审查中的 Critical + Important 问题，提升时区显示、流式解析稳健性、状态管理、性能与可维护性。

**Architecture:** 以 TDD 驱动小步修改：先补测试，再最小化实现。将重复的 settings 状态抽离为 hook，Reader/Settings 页面共享 bootstrap 逻辑，文章列表改为虚拟化渲染，分页使用 limit+1。

**Tech Stack:** React + TypeScript + TanStack Query + Vitest + @tanstack/react-virtual

---

### Task 1: 使用本地时区格式化文章日期

**Files:**
- Modify: `frontend/src/lib/article-list.test.ts`
- Modify: `frontend/src/lib/article-list.ts`

- [ ] **Step 1: Write the failing test**

```ts
it("formats dates using Intl DateTimeFormat", () => {
  const formatter = { format: vi.fn(() => "2026/02/27") } as Intl.DateTimeFormat;
  const dtfSpy = vi.spyOn(Intl, "DateTimeFormat").mockReturnValue(formatter);
  const result = formatArticleTime("2026-02-26T23:30:00Z");
  expect(dtfSpy).toHaveBeenCalledWith(undefined, { year: "numeric", month: "2-digit", day: "2-digit" });
  expect(result).toBe("2026/02/27");
  dtfSpy.mockRestore();
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npm run test -- src/lib/article-list.test.ts`  
Expected: FAIL with "expected spy to have been called" (Intl.DateTimeFormat not used)

- [ ] **Step 3: Write minimal implementation**

```ts
const formatter = new Intl.DateTimeFormat(undefined, { year: "numeric", month: "2-digit", day: "2-digit" });
return formatter.format(new Date(ts));
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npm run test -- src/lib/article-list.test.ts`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/article-list.ts frontend/src/lib/article-list.test.ts
git commit -m "fix: format article dates in local timezone"
```

---

### Task 2: 流式解析容错 + limit+1 分页

**Files:**
- Create: `frontend/src/api/client.test.ts`
- Modify: `frontend/src/api/client.ts`

- [ ] **Step 1: Write failing tests**

```ts
import { describe, expect, it, vi, afterEach } from "vitest";
import { ApiClient } from "./client";

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

it("skips invalid stream lines instead of throwing", async () => {
  const client = new ApiClient("http://example.com");
  const onEvent = vi.fn();
  const stream = createStream(['{"type":"start","article_id":1,"target_lang":"zh-CN","total":1}\\n', "bad-json\\n"]);
  vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(stream, { status: 200 }));

  await expect(client.translateArticleStream(1, "zh-CN", onEvent)).resolves.toBeUndefined();
  expect(onEvent).toHaveBeenCalledTimes(1);
});

it("uses limit+1 to detect hasMore", async () => {
  const client = new ApiClient("http://example.com");
  const articles = Array.from({ length: 3 }, (_, idx) => ({ id: idx + 1 })) as any[];
  vi.spyOn(globalThis, "fetch").mockResolvedValue(
    new Response(JSON.stringify({ articles, has_more: false }), { status: 200 }),
  );

  const result = await client.listArticlesPage(1, 2);
  expect(result.articles).toHaveLength(2);
  expect(result.hasMore).toBe(true);
});
```

- [ ] **Step 2: Run tests to verify failures**

Run: `npm run test -- src/api/client.test.ts`  
Expected: FAIL on translateArticleStream (throws) and hasMore (false)

- [ ] **Step 3: Write minimal implementation**

```ts
const parseLine = (raw: string) => {
  try {
    return JSON.parse(raw) as TranslateStreamEvent;
  } catch (error) {
    apiLogger.warn("translate:stream:invalid", { error: error instanceof Error ? error.message : String(error) });
    return null;
  }
};
```

```ts
const requestedLimit = limit + 1;
const search = buildArticleListQuery({ page, limit: requestedLimit, sort, feedID: scope?.feedID, folderID: scope?.folderID });
const data = await this.request<{ articles?: Article[] }>(`/api/v1/articles?${search}`);
const rows = data.articles ?? [];
return { articles: rows.slice(0, limit), hasMore: rows.length > limit };
```

- [ ] **Step 4: Run tests to verify pass**

Run: `npm run test -- src/api/client.test.ts`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/api/client.test.ts
git commit -m "fix: harden translate stream parsing and paging"
```

---

### Task 3: 缓存文件夹树遍历 + 过滤函数 memoize

**Files:**
- Create: `frontend/src/lib/folder-tree.ts`
- Create: `frontend/src/lib/folder-tree.test.ts`
- Modify: `frontend/src/pages/ReaderPage.tsx`

- [ ] **Step 1: Write failing tests**

```ts
import { describe, expect, it } from "vitest";
import { buildDescendantFolderIDs } from "./folder-tree";
import type { Folder } from "@/types";

const folders: Folder[] = [
  { id: 1, name: "root", parent_id: null },
  { id: 2, name: "child", parent_id: 1 },
  { id: 3, name: "grand", parent_id: 2 },
];

it("builds descendant set including self", () => {
  const map = buildDescendantFolderIDs(folders);
  expect(Array.from(map.get(1) ?? [])).toEqual([1, 2, 3]);
  expect(Array.from(map.get(2) ?? [])).toEqual([2, 3]);
});
```

- [ ] **Step 2: Run tests to verify failure**

Run: `npm run test -- src/lib/folder-tree.test.ts`  
Expected: FAIL (module not found)

- [ ] **Step 3: Write minimal implementation**

```ts
import type { Folder } from "@/types";

export function buildDescendantFolderIDs(folders: Folder[]): Map<number, Set<number>> {
  const childrenByParent = new Map<number, number[]>();
  folders.forEach((folder) => {
    if (folder.parent_id == null) return;
    const list = childrenByParent.get(folder.parent_id) ?? [];
    list.push(folder.id);
    childrenByParent.set(folder.parent_id, list);
  });

  const cache = new Map<number, Set<number>>();
  const visit = (id: number): Set<number> => {
    const hit = cache.get(id);
    if (hit) return hit;
    const next = new Set<number>([id]);
    (childrenByParent.get(id) ?? []).forEach((childID) => {
      visit(childID).forEach((desc) => next.add(desc));
    });
    cache.set(id, next);
    return next;
  };

  folders.forEach((folder) => visit(folder.id));
  return cache;
}
```

- [ ] **Step 4: Use memoized results in ReaderPage**

```ts
const descendantFolderIDs = useMemo(() => buildDescendantFolderIDs(folders), [folders]);
const filterArticlesByScope = useCallback((items: Article[], feedID: number | null, folderID: number | null) => {
  if (sidebarMode === "favorites") return items.filter((article) => article.is_favorite);
  if (feedID != null) return items.filter((article) => article.feed_id === feedID);
  if (folderID != null) {
    const allowed = descendantFolderIDs.get(folderID);
    if (!allowed || allowed.size === 0) return [];
    return items.filter((article) => allowed.has(feedByID.get(article.feed_id)?.folder_id ?? -1));
  }
  return items;
}, [sidebarMode, descendantFolderIDs, feedByID]);
```

- [ ] **Step 5: Run tests**

Run: `npm run test -- src/lib/folder-tree.test.ts`  
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/folder-tree.ts frontend/src/lib/folder-tree.test.ts frontend/src/pages/ReaderPage.tsx
git commit -m "refactor: memoize folder scope filtering"
```

---

### Task 4: 抽离 settings 状态 + Reader/Settings 共享 bootstrap

**Files:**
- Create: `frontend/src/hooks/useSettingsState.ts`
- Create: `frontend/src/hooks/useReaderBootstrap.ts`
- Modify: `frontend/src/pages/ReaderPage.tsx`
- Modify: `frontend/src/pages/SettingsPage.tsx`
- Modify: `frontend/src/hooks/useFeeds.ts`
- Modify: `frontend/src/hooks/useReaderQueries.ts`

- [ ] **Step 1: Write settings state hook**

```ts
import { useState } from "react";
import type { ScriptLang } from "@/components";

export function useSettingsState() {
  const [networkProxyURL, setNetworkProxyURL] = useState("");
  const [aiProtocol, setAIProtocol] = useState<"openai" | "anthropic">("openai");
  const [aiAPIKey, setAIAPIKey] = useState("");
  const [aiAPIKeyMasked, setAIAPIKeyMasked] = useState("");
  const [aiAPIKeyConfigured, setAIAPIKeyConfigured] = useState(false);
  const [aiBaseURL, setAIBaseURL] = useState("");
  const [aiModel, setAIModel] = useState("");
  const [aiTargetLang, setAITargetLang] = useState("zh-CN");
  const [articleRetentionDays, setArticleRetentionDays] = useState("90");
  const [scriptFeedID, setScriptFeedID] = useState<number | null>(null);
  const [scriptContent, setScriptContent] = useState("");
  const [scriptLang, setScriptLang] = useState<ScriptLang>("shell");
  const [scriptDirty, setScriptDirty] = useState(false);

  return {
    networkProxyURL,
    aiProtocol,
    aiAPIKey,
    aiAPIKeyMasked,
    aiAPIKeyConfigured,
    aiBaseURL,
    aiModel,
    aiTargetLang,
    articleRetentionDays,
    scriptFeedID,
    scriptContent,
    scriptLang,
    scriptDirty,
    setNetworkProxyURL,
    setAIProtocol,
    setAIAPIKey,
    setAIAPIKeyMasked,
    setAIAPIKeyConfigured,
    setAIBaseURL,
    setAIModel,
    setAITargetLang,
    setArticleRetentionDays,
    setScriptFeedID,
    setScriptContent,
    setScriptLang,
    setScriptDirty,
  };
}
```

- [ ] **Step 2: Build bootstrap hook with shared logic**

```ts
export function useReaderBootstrap(apiBase: string, sortMode: SortMode = "latest") {
  const { client, feedsQuery, foldersQuery, articlesQueryKey, articlesInfiniteQuery } = useReaderQueries(apiBase, sortMode);
  const [status, setStatus] = useState("准备就绪");
  const [error, setError] = useState("");

  const setMessage = (message: string, isError = false) => {
    if (isError) {
      setError(message);
      setStatus("");
      return;
    }
    setError("");
    setStatus(message);
  };

  const { feeds, folders, loadFeeds, loadFolders } = useFeeds(client, feedsQuery, foldersQuery, setMessage);
  const { articles, setArticles, loadArticles, fetchNextArticlePage, hasNextArticlePage } = useEntries(
    client,
    articlesInfiniteQuery,
    articlesQueryKey,
    setMessage,
  );

  return {
    client,
    feeds,
    folders,
    loadFeeds,
    loadFolders,
    articles,
    setArticles,
    loadArticles,
    fetchNextArticlePage,
    hasNextArticlePage,
    setMessage,
    status,
    error,
  };
}
```

- [ ] **Step 3: Update useFeeds to avoid duplicate state**

```ts
const feeds = feedsQuery.data ?? [];
const folders = foldersQuery.data ?? [];

const loadFeeds = async (options?: { silentStatus?: boolean }) => {
  const data = (await feedsQuery.refetch()).data ?? (await client.listFeeds());
  if (!options?.silentStatus) setMessage("订阅列表已刷新");
  return data;
};
```

- [ ] **Step 4: Update ReaderPage/SettingsPage to use hooks**

```ts
const { client, feeds, folders, loadFeeds, loadFolders, articles, setArticles, loadArticles, fetchNextArticlePage, hasNextArticlePage, setMessage, status, error } =
  useReaderBootstrap(apiBase, sortMode);

const settingsState = useSettingsState();
```

- [ ] **Step 5: Ensure ApiClient only comes from useReaderQueries**

```ts
// remove new ApiClient(apiBase) in pages; use the hook client
```

- [ ] **Step 6: Run tests**

Run: `npm run test`  
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add frontend/src/hooks/useSettingsState.ts frontend/src/hooks/useReaderBootstrap.ts frontend/src/hooks/useFeeds.ts frontend/src/hooks/useReaderQueries.ts frontend/src/pages/ReaderPage.tsx frontend/src/pages/SettingsPage.tsx
git commit -m "refactor: extract settings state and bootstrap logic"
```

---

### Task 5: 文章列表虚拟化

**Files:**
- Modify: `frontend/package.json`
- Modify: `frontend/src/components/article-list/ArticleList.tsx`
- Modify: `frontend/src/pages/ReaderPage.tsx`

- [ ] **Step 1: Add dependency**

```json
"@tanstack/react-virtual": "^3.13.6"
```

- [ ] **Step 2: Update ArticleList to use virtualizer**

```tsx
const parentRef = useRef<HTMLDivElement | null>(null);
const rowVirtualizer = useVirtualizer({
  count: articles.length,
  getScrollElement: () => parentRef.current,
  estimateSize: () => 96,
  overscan: 8,
});
```

```tsx
<div ref={parentRef} className={`list article-list ${listBounce ? "bounce" : ""}`} onScroll={onScroll}>
  <div style={{ height: rowVirtualizer.getTotalSize(), position: "relative" }}>
    {rowVirtualizer.getVirtualItems().map((virtualRow) => {
      const article = articles[virtualRow.index];
      return (
        <button
          key={article.id}
          ref={rowVirtualizer.measureElement}
          style={{ position: "absolute", top: 0, left: 0, width: "100%", transform: `translateY(${virtualRow.start}px)` }}
        >
          ...
        </button>
      );
    })}
  </div>
</div>
```

- [ ] **Step 3: Wire ReaderPage scroll handler**

```tsx
<ArticleList
  listBounce={listBounce}
  onScroll={onArticleListScroll}
  ...
/>;
```

- [ ] **Step 4: Run build**

Run: `npm run build`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/package.json frontend/src/components/article-list/ArticleList.tsx frontend/src/pages/ReaderPage.tsx
git commit -m "feat: virtualize article list rendering"
```

---

### Task 6: 文档更新与收尾

**Files:**
- Modify: `.phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md`
- Modify: `.phrase/phases/phase-rss-llm-reader-20260225/change_rss_llm_reader.md`
- Modify: `.phrase/docs/CHANGE.md`

- [ ] **Step 1: Add taskNNN entries for fixes and mark done**
- [ ] **Step 2: Add changeNNN entries for file edits**
- [ ] **Step 3: Update global CHANGE index**
- [ ] **Step 4: Run verification**

Run: `npm run test`  
Run: `npm run build`

- [ ] **Step 5: Commit**

```bash
git add .phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md .phrase/phases/phase-rss-llm-reader-20260225/change_rss_llm_reader.md .phrase/docs/CHANGE.md
git commit -m "docs: record frontend review fixes"
```
