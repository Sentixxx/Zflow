# Plan: remove card containers for three reader panes (task127)

## Goal
让“内容导航 / 全部文章 / 文章详情”三栏恢复为同一页面连续布局，不使用独立卡片容器。

## Scope
1. 新增并完成 task127。
2. 仅修改 frontend/src/styles.css：
   - sidebar/list-panel/detail-panel 不使用卡片背景、边框、圆角、阴影
   - 窄屏下这三栏也不使用卡片式边框
3. npm run build
4. 回写 task/change/CHANGE
