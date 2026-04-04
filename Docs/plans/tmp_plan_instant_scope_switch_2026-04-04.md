# Temp Plan: instant-scope-switch-single-owner (2026-04-04)

## Goal
- 恢复订阅源/分类切换的即时性。
- 保持文章列表由 React Query 单一 owner 持有，不回退到本地数组 + query 双写。

## Steps
1. 先补测试：
   - 文章 query key 只受 `apiBase + sortMode` 影响，不受 feed/folder scope 影响。
2. 实现：
   - `useReaderQueries` 的文章主查询回到全局文章池。
   - `ReaderPage` 切换订阅/分类只做本地过滤，不触发冷查询。
   - `useEntries` 继续只围绕 query cache 做 refetch 和局部更新。
3. 验证：
   - 前端 Vitest。
   - `npm run build`。
