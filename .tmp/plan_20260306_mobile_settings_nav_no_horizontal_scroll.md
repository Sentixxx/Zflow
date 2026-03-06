# Plan 2026-03-06 Mobile Settings Nav

## Goal
修复移动端设置切换横向滚动问题，改为无横向滚动且易点按的布局。

## Steps
1. 调整 `frontend/src/styles.css` 中 `@media (max-width: 900px)` 的 `.settings-nav/.settings-tab`：
   - 去掉横向滚动
   - 使用可换行布局（两列）
2. 确认 `@media (max-width: 640px)` 下仍可正常显示。
3. 运行 `npm run build` 验证。
4. 回写 phase `task/change` 与 `.phrase/docs/CHANGE.md`。
