# Plan: Frontend theme color + texture refinement (task120)

## Goal
基于用户反馈，修正“过于不自然”的观感：引入明确主题色并降低生硬玻璃感，让页面更有层次但不花哨。

## Scope
1. 新增并完成 task120。
2. 在 frontend/src/styles.css 做第二轮视觉修正：
   - 新增主题色变量（primary/warm）并贯穿 active/hover 状态
   - 降低过度透明和发灰感，提升实体卡片质感
   - 增加轻微纹理/光感背景（极低对比）
   - 优化顶部栏、列表项、详情阅读卡、设置弹窗的颜色与阴影节奏
3. 运行 npm run build。
4. 回写 change_* 与 .phrase/docs/CHANGE.md。

## Constraints
- 仅样式层修改，不改组件逻辑。
- 不引入新依赖。

## Verification
- npm run build
- 手动检查：主题色存在感、列表/详情/设置页质感、移动端布局。
