# Plan: mobile bottom navigation for single-pane reading (task123)

## Goal
解决移动端三栏叠在一起的问题：在窄屏引入底部导航，改为单屏切换导航/列表/详情。

## Scope
1. 新增并完成 task123。
2. 修改 frontend/src/pages/ReaderPage.tsx：
   - 新增 mobile tab 状态（nav/list/detail）
   - 窄屏时按 tab 仅渲染一个主面板
   - 选中文章后自动切到 detail；切换订阅/分类后回到 list
   - 新增底部导航按钮组
3. 修改 frontend/src/styles.css：
   - 增加移动端底部导航样式
   - 增加窄屏下面板显隐样式
4. npm run build 验证。
5. 回写 task/change/CHANGE。
