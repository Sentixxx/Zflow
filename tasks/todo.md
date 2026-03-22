# 2026-03-22 摘要系统规划与 task144 实现

## Checklist
- [x] 读取 `.rules`、memory 索引与当前 phase 文档
- [x] 输出并阅读临时计划书 `.tmp/plan_20260322_summary_system_planning.md`
- [x] 审核现有文章模型、AI 能力与数据层缺口
- [x] 为摘要系统补充 phase 规格、计划与任务拆解
- [x] 产出面向“有记忆的推荐智能体”的实现规划
- [x] 输出并阅读 task144 执行计划书 `.tmp/plan_20260322_implement_task144_display_summary.md`
- [x] 为文章增加 `display_summary` 持久化字段与本地摘要生成 service
- [x] 在 feed 创建/刷新与正文缓存刷新后回填用户可见摘要
- [x] 更新详情页为优先展示 `display_summary`
- [x] 运行 `go test ./...` 与 `npm run build`
- [x] 固定前端包管理器为 `npm` 并补充 corepack 误用说明
- [x] 在文章详情中将摘要改为独立圆弧卡片展示
- [x] 让 display_summary 在有 AI 配置时优先走 OpenAI 兼容模型生成，并兼容 MiniMax
- [x] 修复正文抓取沿用 8 秒全局 client 导致的 Readability 超时
- [x] 修复正文详情右侧空白和宽度异常的样式问题
- [x] 将摘要与正文收敛成单一路径阅读，不再需要切换阅读模式
- [x] 在缺少正文时进入详情后自动触发 Readability 抓取，并避免旧请求回切当前文章

## Review
- 现状缺口：只有 RSS 原始摘要与全文缓存，没有可复用的“规范化摘要/记忆摘要”层。
- 关键决策：摘要拆成原始层、规范化层、记忆层；推荐系统消费记忆摘要与事件，而不是直接消费 UI 摘要字段。
- task144 结果：详情页已优先展示后端生成的 `display_summary`，原始 `summary` 仍作为兜底。
- 实现边界：本轮仅实现本地快速摘要，不接入大模型摘要生成，也不改列表预览。
- 环境修复：前端已显式声明 `packageManager: npm@11.6.2`，避免使用者误走 `corepack + pnpm/yarn` 触发 keyid 校验错误。
- UI 调整：文章详情的摘要已提升为独立圆弧卡片，固定在元信息之后、正文内容之前。
- AI 接入：摘要已支持复用现有 AI 设置走 OpenAI 兼容 `chat/completions`，可直接配置 MiniMax `https://api.minimaxi.com/v1` + `MiniMax-M2.7`。
- 正文抓取：Readability 已切到独立长超时 client，不再被 feed 抓取的 8 秒全局超时误伤。
- 详情布局：移除了 `.detail` 容器自身的固定最大宽度，右侧详情面板不再出现人为留白带。
- 阅读体验：详情页现在固定先看摘要，再在下方直接继续看正文；Readability 按钮改成抓取/重抓正文，不再承担模式切换。
- 自动抓取：当详情文章缺少 `full_content` 且存在链接时，页面会自动触发一次正文抓取，用户不需要再手动补点一次按钮。
- 并发安全：正文抓取、收藏、未读和缓存刷新在异步返回时不再强行覆盖当前已切换到的另一篇文章详情。
- 下一步：如继续推进，可在 `task145` 上引入模型增强摘要，并把 `display_summary` 作为用户视图、把推荐摘要另行分层。
