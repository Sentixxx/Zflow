# Plan 2026-03-06 Main Interaction Product Optimization

## Product Issues
1. 用户切换导航范围/筛选后，详情可能停留在已不属于当前列表的旧文章，认知错位。
2. 用户从分类/收藏/筛选状态回到默认全量视图路径较长。

## Decisions
1. 增加“上下文纠偏”：当选中文章不在当前过滤结果中，自动清空详情并回到列表上下文。
2. 在列表头增加“当前范围提示条 + 一键恢复默认视图”入口，统一重置范围/筛选/排序。

## Steps
1. 修改 `frontend/src/pages/ReaderPage.tsx`：补充上下文摘要、重置函数、纠偏 effect、UI 入口。
2. 修改 `frontend/src/styles.css`：新增上下文提示条样式与移动端适配。
3. 运行 `npm run build` 验证。
4. 回写 task/change 和 `.phrase/docs/CHANGE.md`。
