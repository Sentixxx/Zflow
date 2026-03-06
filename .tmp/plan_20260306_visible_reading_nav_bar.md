# Plan 2026-03-06 Visible Reading Nav Bar

## Goal
继续优化跨平台阅读体验：提供显式、可触达的上一篇/下一篇切换控件。

## Design
1. 在详情区顶部操作栏下新增“顺序导航条”（上一篇 / 当前位置 / 下一篇）。
2. 与现有 prev/next 逻辑复用，边界按钮禁用。
3. 移动端按钮放大并保持单行布局，避免误触。

## Steps
1. 修改 `frontend/src/components/article-detail/ArticleDetailContent.tsx` 添加可见导航条。
2. 修改 `frontend/src/styles.css` 添加导航条样式和移动端适配。
3. 执行 `npm run build`。
4. 回写 task/change 与 `.phrase/docs/CHANGE.md`。
