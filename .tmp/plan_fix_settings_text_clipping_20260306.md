# Plan: fix settings text clipping regression (task135)

## Goal
修复设置卡片文字显示不全问题，优先保证内容完整可见。

## Scope
1. 新增并完成 task135。
2. 回调会导致裁切的样式：
   - settings-modal-body 固定高度
   - settings-page-inner / settings-section-card 的 100% 高度约束
3. 保留必要的视觉优化，不再牺牲可读性。
4. npm run build
5. 回写 task/change/CHANGE
