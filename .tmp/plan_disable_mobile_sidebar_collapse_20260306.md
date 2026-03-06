# Plan: disable sidebar collapse on mobile (task131)

## Goal
移动端不提供侧栏折叠能力；折叠只保留在电脑端。

## Scope
1. 新增并完成 task131。
2. ReaderPage.tsx:
   - 仅桌面显示侧栏折叠按钮
   - 进入移动端时强制 sidebarCollapsed=false
3. styles.css:
   - 移动端隐藏侧栏折叠按钮相关占位影响（如需）
4. npm run build
5. 回写 task/change/CHANGE
