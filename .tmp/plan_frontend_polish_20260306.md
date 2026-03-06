# Plan: Frontend natural visual polish (task119)

## Goal
在不改业务逻辑的前提下，提升阅读页整体视觉自然度与层次感，覆盖顶部栏、三栏面板、列表卡片、详情阅读区、弹窗与设置页。

## Scope
1. 新增并完成 task119（front-end UI polish）。
2. 调整 frontend/src/styles.css 的语义变量和核心组件样式：
   - 背景与色板（light/dark）
   - 顶部栏与操作按钮
   - 三栏 panel 玻璃感/边框/阴影
   - 列表工具按钮和文章/订阅项 hover/active
   - 详情区头部、正文卡片化阅读体验
   - 浮动按钮、上下文菜单、确认弹窗、设置弹窗
3. 运行 npm run build 验证。
4. 回写 task/change/CHANGE 索引。

## Constraints
- 不改 API、状态逻辑、路由行为。
- 不引入大范围无关重构。
- 维持现有 class 命名，优先纯 CSS 修改。

## Verification
- npm run build
- 手动检查：顶部按钮、侧栏/中栏/详情栏层次、详情阅读排版、设置弹窗。
