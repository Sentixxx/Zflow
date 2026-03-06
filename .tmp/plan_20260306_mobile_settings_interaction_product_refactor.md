# Plan 2026-03-06 Mobile Settings Interaction Refactor

## Problem
当前移动端设置切换交互不自然：横向滚动和网格tab都偏“桌面思维”，心智负担高。

## Product Decision
移动端改为「单入口选择器」：
- 顶部使用 select 选择设置分组（订阅/脚本/连接/AI/数据）
- 内容保持单列滚动
- 桌面端继续左侧tab，不改变已有高效路径

## Steps
1. 修改 `frontend/src/components/settings/SettingsView.tsx`：新增移动端 tab 选择器。
2. 修改 `frontend/src/styles.css`：移动端隐藏侧栏 tab、显示选择器，优化 spacing。
3. 运行 `npm run build`。
4. 回写 `task/change` 与 `.phrase/docs/CHANGE.md`。
