# 2026-03-22 task169 通用易读型摘要 prompt 与 query 模式优化

## Checklist
- [x] 读取 `.rules`、memory 索引、`memory_coding.md`、`memory_docs.md` 与相关 lessons
- [x] 输出并阅读临时计划书 `.tmp/plan_20260322_generic_readable_summary_prompt.md`
- [x] 重写 AI 摘要 prompt 与 query 模式，统一为通用单段主旨型摘要
- [x] 增加摘要二次压缩重写路径，消除“超长压缩复述”而不恢复硬截断
- [x] 扩展 `summary_debug` 并同步前端顶栏调试信息
- [x] 更新摘要研究文档与 phase 技术参考
- [x] 运行后端测试与前端构建验证
- [x] 回写 `task/change/lessons` 文档

## Review
- 目标：把 AI 摘要从“长段压缩复述”改成适配多种输入类型的“扫读即懂”摘要，同时保留无硬截断和长文层级聚合。
- 结果：后端 prompt 改成通用主旨优先风格，单篇/分块/聚合三条路径都引入 query-guided 提问；最终结果如果明显过长或细节堆叠，会走二次语义重写而不是字符裁剪。`summary_debug` 新增 `query_mode` 与 `rewrite_passed`，前端顶栏可直接看到本次摘要是否经过扫读版压缩。
- 文档：研究综述和 phase 技术参考补充了 query-guided summarization、通用 prompt 和重写式压缩的研究与工程启发。
- 验证：`GOCACHE=/tmp/zflow-go-build go test ./internal/service ./internal/handler`、`npm run build` 通过。

# 2026-03-22 task168 摘要论文研究综述文档落盘

## Checklist
- [x] 读取 `.rules`、memory 索引、`memory_docs.md`
- [x] 输出并阅读临时计划书 `.tmp/plan_20260322_summary_paper_review_doc.md`
- [x] 整理本次查阅的摘要论文与研究结论
- [x] 在 `Docs/` 下落盘成可直接用于论文写作的研究综述
- [x] 回写 `task/change/lessons` 文档

## Review
- 目标：把此前查阅的摘要相关论文整理成一份可直接复用的研究文档，供后续论文写作和系统设计引用。
- 结果：新增 `Docs/ai-summary-research-review-20260322.md`，覆盖长文摘要范式、长度控制、聚合失真、对 Zflow 的工程启发与参考文献清单。

# 2026-03-22 task167 AI 摘要去长度限制化与层级聚合适配

## Checklist
- [x] 读取 `.rules`、memory 索引、`memory_coding.md` 与相关 lessons
- [x] 输出并阅读临时计划书 `.tmp/plan_20260322_summary_no_length_limit_and_debug.md`
- [x] 去除最终摘要与中间阶段摘要的固定长度限制
- [x] 放宽摘要 prompt 与上游输出预算，增加句边界收尾处理
- [x] 调整长文聚合输入为“阶段摘要 + 原文片段”联合聚合
- [x] 为单篇刷新摘要接口补充 `summary_debug` 调试信息并适配顶栏消息
- [x] 运行后端测试与前端构建验证
- [x] 将相关研究结论沉淀到记忆文档
- [x] 回写 `task/change/lessons` 文档

## Review
- 目标：彻底消除摘要链路里的硬长度限制，并把长文摘要从“过度压缩”改成“层级覆盖 + 自然收尾”。
- 结果：后端移除了最终与阶段摘要的固定字符限制，Anthropic 路径提高输出预算，聚合阶段会同时看到阶段摘要和原文片段；单句无标点时自动补句号，多句半截时回退到最后完整句。单篇刷新接口新增 `summary_debug`，顶栏状态会显示摘要策略、chunks/windows 和是否完整收尾。
- 记忆：将长文摘要与长度控制的论文结论回写到 `.phrase/phases/phase-rss-llm-reader-20260225/tech-refer_summary_memory_pipeline_20260322.md` 和 `tasks/lessons.md`。
- 验证：`GOCACHE=/tmp/zflow-go-build go test ./internal/service ./internal/handler`、`npm run build` 通过。

# 2026-03-22 task166 当前文章摘要显式刷新与顶栏详细状态

## Checklist
- [x] 读取 `.rules`、memory 索引、`memory_coding.md` 与相关 lessons
- [x] 输出并阅读临时计划书 `.tmp/plan_20260322_refresh_current_article_summary.md`
- [x] 复用后端摘要 service 补单篇刷新当前文章摘要开发接口
- [x] 在设置页开发调试区新增“刷新当前文章摘要”按钮与进行中禁用态
- [x] 调整顶栏状态文案，显示当前文章标题与摘要来源状态
- [x] 运行相关后端测试与前端构建验证
- [x] 回写 `task/change/lessons` 文档

## Review
- 目标：让开发阶段可以显式重拉当前文章摘要，并避免重复触发时页面没有反馈。
- 结果：新增单篇刷新当前文章摘要接口和按钮，前端本地维护“刷新中”状态，重复点击会被拦住；顶栏状态改为显示当前文章标题和摘要来源，不再只给出笼统提示。
- 验证：`GOCACHE=/tmp/zflow-go-build go test ./internal/handler`、`npm run build` 通过。

# 2026-03-22 task165 AI 摘要存储分层与开发调试清除

## Checklist
- [x] 读取 `.rules`、memory 索引、`memory_coding.md` 与相关 lessons
- [x] 输出并阅读临时计划书 `.tmp/plan_20260322_ai_summary_storage_and_dev_clear.md`
- [x] 拆分 AI 摘要存储层为 `ai_summary` 与 `display_summary`
- [x] 优化 AI 摘要分块：按总长分层并过滤图片噪音
- [x] 新增单篇/最近 100 篇 AI 摘要开发清除入口
- [x] 调整重生成逻辑为基于 AI 层清空后重建展示层
- [x] 同步前端类型、设置页开发调试区与详情页摘要来源状态
- [x] 运行后端测试与前端构建验证
- [x] 回写 `task/change/lessons` 文档

## Review
- 目标：把 AI 摘要从“单一展示字段”升级为“AI 原文层 + 展示层”，并为开发调试提供单篇/批量清除与稳定重生成入口。
- 结果：后端新增 `ai_summary` 独立持久化字段，`display_summary` 改为派生展示层；开发调试区支持清除当前文章/最近 100 篇 AI 摘要，清除后即时回退到 RSS 摘要。
- 分块策略：AI 摘要输入会先过滤图片噪音，再按文章总长分层处理，短文直出，中长文走连续窗口分块，超长文走多窗口带聚合。
- 验证：`GOCACHE=/tmp/zflow-go-build go test ./internal/service ./internal/handler`、`GOCACHE=/tmp/zflow-go-build go test ./cmd/server`、`npm run build` 通过。

# 2026-03-22 task164 移除 AI 摘要结果限长

## Checklist
- [x] 读取 `.rules`、memory 索引、`memory_coding.md` 与当前 phase 文档
- [x] 输出并阅读临时计划书 `.tmp/plan_20260322_remove_ai_summary_length_limit.md`
- [x] 定位 AI 摘要结果的硬截断逻辑与相关测试
- [x] 移除 AI 摘要结果的人为限长，保留本地 fallback 摘要现有约束
- [x] 运行相关后端测试验证行为
- [x] 回写 `task/change/lessons` 文档

## Review
- 现象：AI 摘要在保存前被硬截到固定字符数，导致句子被截断成半句。
- 修复原则：只移除 AI 摘要结果的硬截断，不扩大其他摘要路径的展示体积，也不改前端契约。
- 实现结果：最终展示给用户的 AI 摘要不再走固定字符裁断，只做空白归一化；长文分块摘要的内部阶段上限仍保留，用于控制聚合 prompt 体积。
- 验证结果：`GOCACHE=/tmp/zflow-go-build go test ./internal/service` 通过，新增回归测试锁定“长 AI 摘要完整保留”行为。

# 2026-03-22 task163 摘要重生成先清空旧结果并补日志

## Checklist
- [x] 读取 `.rules`、memory 索引、`memory_coding.md` 与当前 phase 文档
- [x] 输出并阅读临时计划书 `.tmp/plan_20260322_reset_summary_before_regenerate.md`
- [x] 检查摘要重生成入口、service 与仓储更新接口
- [x] 调整重生成流程为先清空旧 `display_summary` 状态再逐篇重算
- [x] 为摘要重生成补充结构化日志，记录开始、重置和完成结果
- [x] 运行相关后端测试验证行为
- [x] 回写 `task/change/lessons` 文档

## Review
- 现象：批量重生成摘要时直接覆盖结果，旧摘要不会先清空，过程也缺少足够日志，维护时看不到进度和失败位置。
- 修复原则：保持接口与前端按钮不变，只修正后端执行语义和可观测性，避免引入新的维护入口复杂度。
- 实现结果：批量重生成会先锁定最近文章集合，逐篇清空旧 `display_summary/display_summary_status`，再按当前内容与 AI 配置重新生成，并输出开始/清空/完成日志。
- 验证结果：`GOCACHE=/tmp/zflow-go-build go test ./internal/service ./internal/handler` 通过，新增回归测试确保旧摘要不会残留。

# 2026-03-22 task162 启动端口冲突日志修复

## Checklist
- [x] 读取 `.rules`、memory 索引、`memory_coding.md` 与当前 phase 文档
- [x] 输出并阅读临时计划书 `.tmp/plan_20260322_fix_server_bind_logging.md`
- [x] 确认后端启动链路与端口配置优先级
- [x] 修正 HTTP 启动时序，避免未成功监听前打印 `server started`
- [x] 为端口占用失败补充更明确的错误信息
- [x] 运行 `go test ./...` 验证后端无回归
- [x] 回写 `task/change/lessons` 文档

## Review
- 现象：`ListenAndServe` 失败时日志仍先输出 `server started`，造成“服务已启动后又异常停止”的假象。
- 根因：启动日志写在真正 `bind` 端口之前，观测语义错误，不是网络层随机问题。
- 修复原则：保留默认 `:8080` 契约，不做自动换随机端口的破坏性补丁；改为先监听成功，再宣告启动完成。
- 实现结果：启动入口改为显式 `net.Listen` 后再 `Serve`，端口被占用时会直接报出带 `PORT/ZFLOW_ADDR` 提示的错误，不再出现伪 `server started`。
- 验证结果：`GOCACHE=/tmp/zflow-go-build go test ./...` 通过，新增启动入口测试覆盖可用端口和端口占用两条路径。

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
- [x] 打通 AI 摘要协议配置，让 OpenAI/Anthropic 两类兼容服务都能从设置页直接生效
- [x] 修复 AI 协议保存被静默回退到 OpenAI 的问题，并为 Anthropic 持久化补回归测试
- [x] 为超长文章的 AI 摘要增加分批输入阈值，避免单次 prompt 过大
- [x] 在摘要卡片中显示 AI 摘要状态，区分模型摘要与本地回退摘要
- [x] 将 AI 摘要输入改为全篇覆盖式采样，避免只盯正文开头导致复述前言
- [x] 提供批量重生成摘要入口，让旧文章也能按新策略重跑

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
- AI 摘要：前端设置现在可显式选择 `OpenAI 兼容` 或 `Anthropic SDK`，保存后后端摘要 service 会按协议走 `/chat/completions` 或 `/v1/messages`。
- 协议验证：新增 Anthropic 摘要单测，锁住请求头和返回解析，避免出现“代码里有分支、产品里不可用”的假接入。
- 保存修复：后端不再把未知协议静默吃成 `openai`；如果协议保存或读回不一致，前端会直接报错，不再伪装成“保存成功”。
- 长文阈值：AI 摘要现在会在正文超过约 `1200` 字符/块时自动分批；每篇最多取 `6` 块，先做局部摘要，再合并成最终摘要，避免长文一次性塞爆 prompt。
- 摘要状态：详情页摘要卡片头部现在会直接显示 `AI 摘要` 或 `快速摘要`，用户能立刻知道当前看到的是模型结果还是本地回退。
- 通用优化：AI 摘要输入不再只截正文最前面几段，而是按“开头/中段/结尾覆盖 + 信息量优先”采样正文块，减少任何站点前言把摘要带偏的概率。
- 批量重跑：数据管理页新增“重生成最近 100 篇摘要”按钮，可把已有文章按当前 AI 配置和新采样策略重新生成一遍 `display_summary`。
- 下一步：如继续推进，可在 `task145` 上引入模型增强摘要，并把 `display_summary` 作为用户视图、把推荐摘要另行分层。
