# Plan: mobile display optimization (task122)

## Goal
提升移动端显示效果：避免信息挤压、提升触控可用性、减少固定元素遮挡。

## Scope
1. 新增并完成 task122。
2. 调整 frontend/src/styles.css 的移动端规则：
   - 顶部栏与状态文案布局（避免重叠）
   - 按钮触控尺寸与间距
   - panel/list/detail 在小屏下的内边距与可读性
   - 浮动按钮和底部安全区适配
   - 设置弹窗在窄屏下的导航与可滚动体验
3. npm run build 验证。
4. 回写 change 与全局 CHANGE。
