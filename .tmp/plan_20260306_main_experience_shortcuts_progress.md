# Plan 2026-03-06 Main Experience Shortcuts + Progress

## Goal
继续优化主界面阅读体验：降低切换成本并增强当前位置感知。

## Scope
1. 增加全局快捷键（非输入态生效）：
   - J: 下一篇
   - K: 上一篇
   - Esc: 返回列表/清空当前详情
2. 详情头部增加当前位置提示：`第 N / M 条`，并附快捷键提示。

## Steps
1. 修改 `frontend/src/pages/ReaderPage.tsx` 计算位置文本并注册键盘事件。
2. 修改 `frontend/src/components/article-detail/ArticleDetailContent.tsx` 与 `ArticleDetailTopBar.tsx` 透传并渲染位置文案。
3. 修改 `frontend/src/styles.css` 增加详情标题副文案样式。
4. 执行 `npm run build`，回写 task/change。
