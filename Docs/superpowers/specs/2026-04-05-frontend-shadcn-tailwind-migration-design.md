# 前端样式系统重构：迁移到 Tailwind + shadcn/ui

**日期**: 2026-04-05
**作者**: Claude (Sonnet 4.6) + @Sentixxx
**状态**: 已确认，待拆分实现计划
**类型**: Breaking Change — 前端样式层整体重写

## 背景与动机

### 当前痛点

用户反馈前端页面存在以下问题：

1. **组件风格不一致** — 不同页面/组件的视觉语言割裂
2. **页面有点丑** — 缺乏统一的设计语言
3. **尺寸不规整** — 相似元素的尺寸、圆角、字号取值随意

### 代码层面的证据（审计结果）

通过对 `frontend/src/styles.css`（2652 行）的审计，确认痛点来自**缺乏设计令牌系统**：

| 维度 | 当前状况 | 问题 |
|---|---|---|
| `border-radius` | `0, 3, 4, 8, 9, 10, 12, 13, 14, 16, 22, 24, 999`（13 种取值） | 无规律，组件之间圆角不协调 |
| `font-size` | `11, 12, 13, 14, 15, 16, 17, 18, 20, 22, 24, 30` + `em` 混用 | 字号层级混乱 |
| 交互元素高度 | `30, 34, 36, 38, 40, 42, 44`（7 种） | TopBar 按钮 40px、列表工具栏按钮 42px、排序触发器 36px、输入框 38px —— 大小不一致 |
| 选中态颜色 | `.item.article.active`=紫 `#8b5cf6`、`.feed-item.active`=蓝 `#0ea5e9`、`.folder-item.active`=深蓝 `#2563eb` | 硬编码，三处"选中"颜色各不相同，未走 CSS 变量 |

此外：

- 自研的 tooltip（`.icon-btn::after` 伪元素）、下拉菜单（`.list-sort-popover`）、弹窗（`SettingsModal`）在定位、焦点管理、可访问性上质量参差
- `src/components/ui/` 仅有 2 个自定义组件，CLAUDE.md 所声称的 "shadcn/ui" 实际并未实装

## 目标

1. 建立**统一的设计令牌系统**（radius / font-size / spacing / control-height / 颜色）
2. 引入 **Tailwind CSS + shadcn/ui** 作为长期样式与组件基建
3. **彻底替换**现有 `styles.css` 中的自定义 class（breaking change，不保留兼容层）
4. 用 shadcn 组件替换自研的交互组件（Button / Dialog / DropdownMenu / Tooltip / Tabs / Card / Badge / Input / Select）
5. 保持现有业务逻辑、API、路由、i18n、Zustand store 不变
6. 保持暗色模式跟随系统设置（`prefers-color-scheme`）

## 非目标

- 不修改后端 API 合约
- 不重写业务 hooks（`useReaderQueries`、`useArticleActions` 等）
- 不重写路由 / store / i18n
- 不引入新的动画库
- 不做布局层的大改动（三栏结构保持不变）
- 不引入手动暗色切换（`.dark` 类写一份占位，当前仍跟随系统）

## 架构决策

### 决策 1：选型 = Tailwind + shadcn/ui（非 Radix-only）

**替代方案：**

- **A. 只引 Radix primitives + 保留现有 CSS** — 改动最小但无法解决"设计令牌不统一"的根本问题
- **B. Tailwind + shadcn/ui** ✅ — 同时解决令牌系统和组件一致性
- **C. 只做设计令牌不引组件库** — 无法提升自研交互组件的质量

**选择 B 的理由：**

- shadcn 组件 copy-paste 到仓库，**无运行时依赖锁定**，符合自托管项目价值观
- shadcn 原生基于 CSS 变量做主题，天然支持明暗模式
- 同步解决"令牌不统一"和"自研弹窗 / 下拉 / tooltip 质量参差"两个问题
- 社区生态最成熟

### 决策 2：Breaking Change，不保留兼容层

- `styles.css` 从 ~2652 行缩减到 ~100 行（仅保留 `@tailwind` 指令、主题 CSS 变量、极少数真正无法用 Tailwind 表达的规则）
- 所有现有自定义 class（`.topbar`, `.panel`, `.sidebar`, `.item.article` 等）**全部删除**
- 所有 `.tsx` 组件的 `className` **全部重写**为 Tailwind 工具类
- 不提供新旧并存的过渡期

### 决策 3：CSS 变量采用 shadcn 标准约定（HSL 三元组）

- 放弃现有的双变量体系（`--ink`, `--paper`, `--theme-primary`, `--line` 等语义层）
- 统一使用 shadcn 约定：`--background`, `--foreground`, `--primary`, `--muted`, `--accent`, `--border`, `--ring` 等
- 值使用 HSL 空格分隔格式（`--background: 220 30% 98%;`），方便 Tailwind 透明度修饰符

### 决策 4：设计令牌通过 Tailwind 主题扩展暴露

在 `tailwind.config.js → theme.extend` 中定义：

```js
fontSize: {
  xs:   ['11px', { lineHeight: '16px' }],
  sm:   ['12px', { lineHeight: '18px' }],
  base: ['13px', { lineHeight: '20px' }],
  md:   ['14px', { lineHeight: '22px' }],
  lg:   ['16px', { lineHeight: '24px' }],
  xl:   ['20px', { lineHeight: '28px' }],
},
height: {
  'control-sm': '32px',
  'control-md': '36px',
  'control-lg': '40px',
},
borderRadius: {
  lg: 'var(--radius)',         // 10px
  md: 'calc(var(--radius) - 2px)', // 8px
  sm: 'calc(var(--radius) - 4px)', // 6px
},
```

原则：
- 圆角仅允许 `sm / md / lg / pill`（999px 映射为 `rounded-full`）四档
- 字号仅允许 `xs / sm / base / md / lg / xl` 六档
- 控件高度仅允许 `control-sm / control-md / control-lg` 三档
- 间距沿用 Tailwind 默认 4px 步进

### 决策 5：暗色模式保持 `@media (prefers-color-scheme)`

- 不使用 shadcn 默认的 `.dark` class 切换策略
- 在 `styles.css` 里写 `@media (prefers-color-scheme: dark)` 查询覆盖 `:root` 变量
- 未来需要"手动切换"时再新增 `.dark { ... }` 规则，本次不写占位

## 具体设计

### 依赖清单

```
dependencies（生产）:
  tailwindcss-animate
  class-variance-authority
  clsx
  tailwind-merge
  lucide-react
  @radix-ui/react-dialog
  @radix-ui/react-dropdown-menu
  @radix-ui/react-tooltip
  @radix-ui/react-tabs
  @radix-ui/react-popover
  @radix-ui/react-separator
  @radix-ui/react-slot
  @radix-ui/react-label

devDependencies:
  tailwindcss
  postcss
  autoprefixer
```

### 新增/修改的配置文件

| 文件 | 动作 |
|---|---|
| `frontend/tailwind.config.js` | 新增 |
| `frontend/postcss.config.js` | 新增 |
| `frontend/components.json` | 新增（shadcn CLI） |
| `frontend/src/lib/utils.ts` | 新增（`cn()` 函数） |
| `frontend/src/styles.css` | 重写（2652 → ~100 行） |
| `frontend/package.json` | 新增依赖 |

### 主题变量（`styles.css` 重写后）

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    --background: 220 30% 98%;
    --foreground: 220 20% 17%;
    --card: 0 0% 100%;
    --card-foreground: 220 20% 17%;
    --popover: 0 0% 100%;
    --popover-foreground: 220 20% 17%;
    --primary: 176 78% 26%;
    --primary-foreground: 0 0% 100%;
    --secondary: 220 20% 94%;
    --secondary-foreground: 220 20% 17%;
    --muted: 220 20% 94%;
    --muted-foreground: 220 12% 45%;
    --accent: 176 50% 92%;
    --accent-foreground: 176 78% 20%;
    --destructive: 0 72% 50%;
    --destructive-foreground: 0 0% 100%;
    --border: 216 25% 88%;
    --input: 216 25% 88%;
    --ring: 176 78% 40%;
    --radius: 0.625rem;
  }

  @media (prefers-color-scheme: dark) {
    :root {
      --background: 222 40% 8%;
      --foreground: 210 20% 94%;
      --card: 220 30% 12%;
      --card-foreground: 210 20% 94%;
      --popover: 220 30% 12%;
      --popover-foreground: 210 20% 94%;
      --primary: 172 66% 54%;
      --primary-foreground: 220 30% 10%;
      --secondary: 220 20% 18%;
      --secondary-foreground: 210 20% 94%;
      --muted: 220 20% 18%;
      --muted-foreground: 215 15% 65%;
      --accent: 176 40% 20%;
      --accent-foreground: 172 66% 80%;
      --destructive: 0 62% 50%;
      --destructive-foreground: 0 0% 100%;
      --border: 216 20% 24%;
      --input: 216 20% 24%;
      --ring: 172 66% 50%;
    }
  }

  * { @apply border-border; }
  body {
    @apply bg-background text-foreground;
    font-family: "PingFang SC", "Noto Sans SC", "IBM Plex Sans", sans-serif;
  }
}
```

### shadcn 组件引入与替换映射

| shadcn 组件 | 替换现有实现 |
|---|---|
| `Button` | `.icon-btn`, `.list-icon-btn`, `.mini-btn`, `.settings-entry`, `button.secondary`, `.sidebar-mode-tab` 等所有按钮 |
| `Dialog` | `SettingsModal` 的自研弹窗外壳 |
| `DropdownMenu` | `.list-sort-popover` 及排序菜单、节点操作菜单 |
| `Tooltip` | `.icon-btn::after / ::before` 自研 tooltip、`.list-icon-btn::after` |
| `Tabs` | `.sidebar-mode-tabs`（订阅/收藏切换）、SettingsModal 内部的 tab 导航 |
| `Input` | 所有原生 `<input>`（feed-rename-input, 设置表单等） |
| `Select` | 所有原生 `<select>` |
| `Card` | `.detail-summary-card`, 各 SettingsCard 外壳 |
| `Badge` | 文章条目的"已读/未读" pill、摘要状态徽章 |
| `Separator` | `.tree-divider` 等分隔线 |
| `Label` | 所有原生 `<label>` |
| `Popover` | 非菜单型悬浮面板（如有） |

### 自研保留组件（结构特殊，不套 shadcn，仅用 Tailwind 重写样式）

- `SidebarTree`（树形折叠 + 拖拽）
- `ArticleList`（TanStack 虚拟滚动，`@tanstack/react-virtual`）
- `ArticleDetailContent`（富文本正文、摘要、译文三态）
- 三栏布局容器（`ReaderPage` 内的 grid / flex 外壳）
- `TopBar`, `ArticleListToolbar`, `ArticleDetailToolbar`, `ArticleDetailTopBar`（容器型，用 Tailwind 重新布局）

### 迁移顺序（从底到顶，风险递增）

1. **基础设施**：装依赖 → 写 `tailwind.config.js` / `postcss.config.js` / `components.json` / `src/lib/utils.ts` → 重写 `styles.css` 只保留主题变量 → 验证 `npm run dev` 能启动
2. **引入 shadcn 原子组件**：`npx shadcn@latest add button input label select badge separator tooltip`
3. **引入 shadcn 复合组件**：`npx shadcn@latest add dialog dropdown-menu tabs card popover`
4. **重写叶子组件**：`ToolbarIconButton`、各 `SettingsCard` 表单元素 → 使用 shadcn `Button / Input / Label`
5. **重写交互组件**：
   - `SettingsModal` → `Dialog` + `Tabs`
   - `ArticleListToolbar` 排序菜单 → `DropdownMenu`
   - 所有 icon 按钮的 tooltip → `Tooltip`
6. **重写容器**：`TopBar`, `SidebarTree`, `ArticleList`（保留虚拟滚动）, `ArticleDetailContent`, `ReaderPage`
7. **清理 `styles.css`**：删除所有被 Tailwind 取代的规则，确认只剩主题变量层（~100 行）
8. **测试回归**：
   - `npm run build`（TypeScript + Vite 构建）
   - `npm run test`（Vitest — 注意 `ArticleList.test.tsx`, `useReaderBootstrap.test.tsx`, `useSettingsState.test.tsx` 中若有 class 断言则改为 role/text 断言）
   - 手动冒烟：三栏布局、暗色模式、移动端断点、各交互（拖拽、排序、搜索、设置弹窗、翻译、摘要、收藏）

### 测试策略

- **单元测试**：Vitest 现有测试继续通过。若有 `toHaveClass('...')` 或类似 class 断言，改用 `getByRole`, `getByText`, `toBeVisible` 等更稳健的查询
- **视觉回归**：迁移过程中按组件对照旧 UI 截图，人工确认无明显退化
- **自动化浏览器验证**：使用 `chrome-devtools-mcp` 启动 dev server、导航到关键页面、截图对比、触发交互（点击、hover、表单填写），在每个迁移步骤后跑一次冒烟
- **可访问性检查**：迁移完成后通过 `chrome-devtools-mcp` 的 a11y 能力抽查焦点、ARIA、键盘导航
- **构建验证**：每完成一个迁移步骤，运行 `npm run build` 确保 TypeScript 类型和 Tailwind purge 都正常
- **暗色模式**：切换系统外观或使用 `chrome-devtools-mcp` emulate 验证

## 风险与缓解

| 风险 | 可能性 | 影响 | 缓解措施 |
|---|---|---|---|
| 全量重写漏掉某些交互细节（拖拽悬浮态、tooltip 定位、transition 时间） | 中 | 中 | 按组件对比旧截图，迁移一个验证一个 |
| Radix Dialog 的 scroll lock 与现有 `body { overflow: hidden }` 冲突 | 低 | 低 | shadcn 默认配置已处理 |
| 虚拟滚动列表 `@tanstack/react-virtual` 在 Tailwind 重写后定位 bug | 中 | 高 | 虚拟滚动的关键样式（`position: absolute`, `transform: translateY`）用 Tailwind 任意值语法保留，或用 `style` prop 注入 |
| 现有 Vitest 测试中的 class 断言会失效 | 中 | 低 | 预先扫描测试文件，迁移时同步修改断言方式 |
| 移动端断点不一致（现有 `max-width: 768px`，Tailwind 默认 `md: 768px` 是 min-width 语义） | 低 | 低 | 统一使用 Tailwind 的移动优先断点语义重写所有响应式规则 |
| `styles.css` 中某些难以用 Tailwind 表达的规则（如 `animation keyframes`, `::before/::after` 伪元素内容） | 中 | 低 | 这些规则保留在 `styles.css` 的 `@layer base` 或 `@layer components` 中，不强行 Tailwind 化 |
| shadcn 组件的 `class-variance-authority` 默认样式和现有视觉不完全一致 | 高 | 中 | 通过修改 shadcn 组件源码（因为是 copy-paste 到本地的），直接调整 variant 样式 |
| 首次 Tailwind JIT 扫描使构建变慢 | 低 | 低 | 可接受，增量构建快 |

## 验收标准

1. `npm run build` 通过，无 TypeScript 错误，无 Vite 构建错误
2. `npm run test` 全部通过
3. `styles.css` 行数 ≤ 150 行（从 2652 行削减 ≥ 94%）
4. 三栏布局、明暗主题、移动端断点在手动测试中无明显退化
5. 所有交互（侧边栏拖拽、排序菜单、设置弹窗、文章翻译、AI 摘要、收藏、已读切换）功能完整
6. 浏览器 DevTools 中：不存在 `class="item article"`, `class="icon-btn"`, `class="sidebar"` 等旧自定义 class 的痕迹（全部替换为 Tailwind 或 shadcn 组件 class）
7. 新增的 shadcn 组件位于 `src/components/ui/`，符合 CLAUDE.md 的 "shadcn/ui" 约定

## 后续可扩展项（不在本次范围）

- 手动暗色切换开关（基础设施已就绪，只需加一个 toggle）
- 设计令牌文档站（Storybook 或轻量 demo 页面）
- 自定义主题（多配色方案）
- 动效细化（`framer-motion` 或 `tailwindcss-animate` 扩展）
