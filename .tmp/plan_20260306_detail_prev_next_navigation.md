# Plan 2026-03-06 Detail Prev Next Navigation

## Goal
优化主界面阅读流：在详情区支持“上一篇/下一篇”快速切换，减少来回点击列表。

## Design
1. 基于当前 `filteredAndSortedArticles` 计算选中文章索引。
2. 推导 `prevArticleID/nextArticleID`，无可用项时禁用按钮。
3. 在详情浮动快捷操作加入前后切换按钮，保持现有“回到顶部/翻译”。

## Steps
1. 修改 `frontend/src/pages/ReaderPage.tsx` 计算并传递前后导航能力。
2. 修改 `frontend/src/components/article-detail/ArticleDetailContent.tsx` 和 `ArticleFloatingActions.tsx` 接收并触发前后切换。
3. 运行 `npm run build`。
4. 回写 task/change 与 `.phrase/docs/CHANGE.md`。
