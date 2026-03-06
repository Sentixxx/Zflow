# Plan: quick add feed button in content navigation (task128)

## Goal
在“内容导航”区域提供快速添加订阅源入口，减少多步操作。

## Scope
1. 新增并完成 task128。
2. 修改 frontend/src/pages/ReaderPage.tsx：
   - 在sidebar header增加“+订阅”按钮
   - 点击后打开设置弹窗并切换到订阅管理tab
3. 修改 frontend/src/styles.css：
   - 新增sidebar header操作区和快速按钮样式
4. npm run build
5. 回写 task/change/CHANGE
