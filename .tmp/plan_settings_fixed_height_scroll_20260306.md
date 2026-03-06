# Plan: stable settings card height with internal scroll (task136)

## Goal
设置卡片切换保持固定高度，同时确保内容完整可读。

## Scope
1. 新增并完成 task136。
2. 修改 frontend/src/styles.css：
   - settings modal 在桌面固定高度
   - settings-section-card 固定高度并启用内部滚动
   - 保持移动端不强制固定高度
3. npm run build
4. 回写 task/change/CHANGE
