# Frontend Code Review — 2026-04-04

## 总览

对 `frontend/` 全部代码进行类型安全、性能、安全性、状态管理和 CLAUDE.md 合规性审查。

---

## 做得好的地方

- **类型安全** — 全局零 `any`，所有 props 和 API 响应均显式类型化
- **TanStack Query 配置** — `staleTime: 30s`、`refetchInterval: 60s`，使用 `useInfiniteQuery` + `getNextPageParam`，完全合规
- **dangerouslySetInnerHTML** — 均通过 DOMPurify `sanitizeRichHTML` 预处理，合规
- **外部链接** — `target="_blank"` 均附带 `rel="noreferrer"`，合规
- **Console 纪律** — 所有日志通过 `src/lib/logger.ts`，页面组件无散落 `console.*`
- **Import 路径** — 跨模块一致使用 `@/` 别名
- **Zustand 范围** — 仅存放真正共享的状态（selectedFeedID、selectedFolderID、readFilter、sortMode），局部交互状态留在组件内
- **暗色/亮色主题** — 支持 `prefers-color-scheme`
- **Hook 拆分策略** — `useReaderQueries`、`useEntries`、`useArticleActions` 等有效分解了复杂度

---

## 严重问题 (Critical)

### 1. 日期格式化硬编码 UTC+8

**文件:** `frontend/src/lib/article-list.ts` (~line 132-137)

```typescript
const utc8Ms = ts + 8 * 60 * 60 * 1000;
const date = new Date(utc8Ms);
```

`formatArticleTime` 硬编码 UTC+8 偏移。非中国时区用户会看到错误日期。作为开源自部署工具，应使用 `Intl.DateTimeFormat` 或 `toLocaleDateString()` 适配浏览器本地时区。

### 2. 重复 ApiClient 实例

**文件:**
- `frontend/src/pages/ReaderPage.tsx` (~line 96)
- `frontend/src/hooks/useReaderQueries.ts` (~line 8)

`ReaderPage` 和 `useReaderQueries` 各自创建独立的 `ApiClient`。ReaderPage 未使用 hook 返回的 client，导致两个实例并存。虽功能无害（无状态），但造成混淆。

**建议:** 统一为单一 client 来源，由 hook 提供或由页面注入。

### 3. 流式 JSON.parse 无 try/catch

**文件:** `frontend/src/api/client.ts` (~line 241-242)

```typescript
const parsed = JSON.parse(trimmed) as TranslateStreamEvent;
```

`translateArticleStream` 中逐行 `JSON.parse` 无异常捕获。网络异常导致的不完整 chunk 会抛出未处理错误，终止整个流。

**建议:** 包裹 try/catch，跳过异常行并记录日志。

---

## 重要问题 (Important)

### 4. ReaderPage 30+ useState，设置状态应抽离

**文件:** `frontend/src/pages/ReaderPage.tsx` (~line 52-95)

组件管理 30+ 个 state。设置相关状态（`networkProxyURL`、`aiProtocol`、`aiAPIKey`、`aiBaseURL`、`aiModel`、`aiTargetLang`、`articleRetentionDays`、`scriptFeedID`、`scriptContent`、`scriptLang`、`scriptDirty`）应移入 `useSettingsActions` 或专用 settings store。当前导致 `SettingsModal`/`SettingsView` 接收 40+ props。

**建议:** 将设置状态合并为 `useSettingsState` hook 或 Zustand settings slice。

### 5. useFeeds 复制 Query 数据到 useState

**文件:** `frontend/src/hooks/useFeeds.ts` (~line 15-27)

```typescript
const [feeds, setFeeds] = useState<Feed[]>([]);
useEffect(() => {
  if (feedsQuery.data) setFeeds(feedsQuery.data);
}, [feedsQuery.data]);
```

将 TanStack Query 缓存数据复制到 `useState`，形成双数据源。refetch 后 useState 可能保留旧数据。

**建议:** 直接使用 `feedsQuery.data ?? []`，移除冗余 state。

### 6. filterArticlesByScope 未 memoize

**文件:** `frontend/src/pages/ReaderPage.tsx` (~line 180)

该函数在组件体内声明，捕获闭包变量（`sidebarMode`、`feeds`、`childFoldersByParent`），每次渲染创建新闭包。虽在 `useMemo` 中调用，但也在 `rebuildStickyUnreadIDs` 中无 memoize 调用。

CLAUDE.md: "Expensive computation must use `useMemo` or pure function caching."

### 7. collectDescendantFolderIDs 无缓存

**文件:** `frontend/src/pages/ReaderPage.tsx` (~line 165)

每次调用遍历整棵文件夹树。深层文件夹结构下，每个渲染周期的 `filteredAndSortedArticles` 计算都会触发遍历。

**建议:** memoize 文件夹树遍历结果。

### 8. 文章列表无虚拟化

**文件:** `frontend/src/components/article-list/ArticleList.tsx`

CLAUDE.md: "Long lists must use pagination, infinite scroll, or virtualization (no full synchronous render)."

当前通过 `.map()` 同步渲染所有 `pagedArticles`。虽有 `bufferedCount`/`visibleCount` 两级缓冲，但滚动后 DOM 节点可累积至数百个。

**建议:** 引入 `@tanstack/react-virtual` 实现虚拟化。

### 9. ReaderPage 与 SettingsPage 大量代码重复

**文件:**
- `frontend/src/pages/ReaderPage.tsx`
- `frontend/src/pages/SettingsPage.tsx`

`SettingsPage` 复制了 ReaderPage 的大量状态声明、`setMessage`、`refreshFeedsFromNetwork`、`handleRefreshArticles`、`addFeed`、`createRootFolder` 和 bootstrap `useEffect`。修改一处而遗漏另一处将产生 bug。

**建议:** 抽取共享逻辑到 hook（如 `useAppBootstrap`）。

### 10. limit+1 hasMore 检测模式未在前端使用

CLAUDE.md: "detect `hasMore` via `limit+1` pattern."

当前依赖后端返回的 `data.has_more` 布尔值，而非前端请求 `limit+1` 项后判断 `results.length > limit`。虽功能正确，但偏离文档约定。

---

## 建议改进 (Suggestions)

### 11. 文件夹操作使用 window.prompt/confirm

**文件:** `frontend/src/hooks/useSidebarFeedActions.ts` (~line 115, 131, 146)

浏览器原生 `prompt()`/`confirm()` 与整体 UI 品质不匹配。建议替换为自定义模态框。

### 12. 缺少 React Error Boundary

应用无全局错误边界。任何组件渲染错误将导致整个 UI 崩溃且无法恢复。

---

## 合规性汇总

| 规则 | 状态 |
|------|------|
| 无 `any` 类型 | 通过 |
| TanStack Query 配置 (staleTime/refetchInterval) | 通过 |
| dangerouslySetInnerHTML 净化 | 通过 |
| 外部链接 rel 属性 | 通过 |
| Console 日志纪律 | 通过 |
| @/ Import 别名 | 通过 |
| Zustand 范围纪律 | 通过 |
| 暗色/亮色主题 | 通过 |
| 长列表虚拟化 | 需改进 |
| useMemo 缓存昂贵计算 | 需改进（部分缺失） |
| DRY / 代码重复 | 需改进 |
| 时区处理 | Bug |

---

## 修复优先级

1. **硬编码 UTC+8 时区** — Bug，Critical
2. **流式 JSON.parse 无异常处理** — 健壮性，Critical
3. **ReaderPage/SettingsPage 代码重复** — 可维护性，Important
4. **设置状态从 ReaderPage 抽离** — 架构，Important
5. **useFeeds 双数据源** — 数据一致性，Important
6. **文章列表虚拟化** — 性能，Important
7. **其余 Important 和 Suggestion 项**
