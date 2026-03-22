# TECH-REFER: Summary Memory Pipeline (2026-03-22)

## Context
- 当前 `entries` 仅保存 RSS 原始 `summary` 与可选 `full_content`，缺少面向推荐的统一摘要资产。
- 后续推荐系统希望演进为“有记忆系统的智能体”，因此需要稳定的中间表示，而不是每次推荐都重新从原文临时抽取。

## Proposed Approach
- 采用三层摘要结构：
  - 原始层 `raw_summary`：保留 RSS/script 产出的原始摘要，用于 UI 回显与问题排查。
  - 规范化层 `canonical_summary`：对正文/原始摘要做清洗、截断、去模板化后的统一摘要，作为评分与 embedding 的稳定输入。
  - 记忆层 `memory_summary`：围绕“这篇内容对用户为何重要”生成的结构化摘要，供推荐智能体写入长期记忆。
- 采用“抽取先行、生成补强”的流水线：
  - 先做 deterministic 抽取：正文清洗、段落切分、语言检测、关键词/实体/主题候选。
  - 再做 LLM 生成：生成 canonical summary、memory summary、topic tags、novelty hooks、actionable signals。
- 采用异步回填，不阻塞阅读主链路：
  - 抓取入库后立即可读。
  - 摘要任务进入后台队列；失败时保留 `pending/failed` 状态并支持重试。

## Data Model
- `entry_summaries`
  - `entry_id`
  - `source_type` (`rss`/`full_content`/`llm`)
  - `canonical_summary`
  - `memory_summary`
  - `topics_json`
  - `entities_json`
  - `signals_json`
  - `quality_score`
  - `novelty_score`
  - `generated_at`
  - `model_name`
  - `status`
  - `error_message`
- `memory_events`
  - `id`
  - `entry_id`
  - `event_type` (`exposed`/`opened`/`favorited`/`dismissed`/`feedback`)
  - `payload_json`
  - `created_at`
- `user_memory_snapshots`
  - `id`
  - `summary_window`
  - `profile_summary`
  - `preference_vector_ref`
  - `updated_at`

## Service Boundaries
- `handler`
  - 提供摘要查询、重算触发、记忆反馈写入接口。
- `service`
  - 编排正文抽取、摘要生成、信号合并、记忆更新。
  - 保证降级：无 `full_content` 时回退到 `summary/title`。
- `repository`
  - 负责 `entry_summaries`、`memory_events`、`user_memory_snapshots` 的持久化与查询。

## Recommendation Contract
- 推荐排序不直接依赖原始 RSS `summary`。
- 推荐输入最少包含：
  - `canonical_summary`
  - `memory_summary`
  - `topics/entities/signals`
  - 最近用户记忆快照
  - 最近交互事件窗口
- 推荐解释复用 `memory_summary + matched signals` 输出，避免解释层再单独调用一次模型。

## Trade-offs
- 优点：摘要与记忆可复用，推荐、解释、画像共用同一份中间资产。
- 代价：需要新增表、后台任务和回填逻辑，首轮实现复杂度上升。
- 结论：这是值得的；否则推荐上线时会把摘要、画像、解释三套逻辑各写一遍。

## Risks & Mitigations
- LLM 成本高：先提供 deterministic baseline，再对高价值文章做生成增强。
- 摘要漂移：记录 `model_name/generated_at` 并支持批量重算。
- 长文不稳定：先做段落切片与 token budget 控制，不把整篇正文直接扔给模型。
- 记忆污染：只把用户显式行为和高置信摘要写入长期记忆，短期噪声保留在事件窗口。

## Validation
- 单元测试：清洗/切片/降级/状态迁移/记忆聚合。
- 集成测试：摘要生成失败不影响文章读取；回填成功后推荐输入完整。
- 手动验证：查看文章摘要详情、推荐解释、用户记忆快照变化。
