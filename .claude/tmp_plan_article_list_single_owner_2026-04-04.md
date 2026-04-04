# Temp Plan: article-list-single-owner-fix (2026-04-04)

## Goal
- 消除文章列表“手动刷新数组”和“React Query 分页缓存”双写导致的抽风。
- 让文章列表只由 query 缓存持有，页面侧只消费统一结果。

## Steps
1. 先补最小失败测试，锁定刷新逻辑必须走 query refetch，而不是再单独拼一份数组状态。
2. 重构 `useEntries`：
   - 去掉本地 `articles` state。
   - `loadArticles()` 改成触发 `articlesInfiniteQuery.refetch()` 并从 query 结果返回。
   - `setArticles` / `upsertArticle` 改为通过 query cache / query data 更新，而不是本地 state。
3. 收口调用点：
   - `ReaderPage`、`SettingsPage`、`useArticleActions`、`useSettingsActions`、`useSidebarFeedActions` 继续通过统一 hook 更新。
4. 验证：
   - 前端单测。
   - `npm run build`。

## Constraint
- 不改变现有 API 行为，只修正前端数据 owner。
