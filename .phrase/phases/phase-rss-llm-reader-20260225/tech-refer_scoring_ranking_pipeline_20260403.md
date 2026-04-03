# TECH REFER: Scoring + Ranking Pipeline (Phase 1)

## Summary
- 目标：把当前单篇启发式评分升级为可解释、可回退、可迭代的分层推荐管线。
- 第一阶段只做四层：`输入门控 -> Readability 兜底 -> 单篇特征抽取 -> 最近文章池新颖度 -> 融合排序`。
- LLM 只输出结构化弱特征（标签、证据、置信度），不直接输出最终总分。

## Scope
- In:
  - 输入有效性门控与降级策略。
  - 当正文不足时触发一次 Readability 兜底。
  - 单篇规则特征：`quality`、`relevance`、`depth`、`freshness`。
  - 单篇 LLM 特征：主题标签、意图标签、质量信号标签。
  - 最近文章池上的真正 `novelty` 计算（近重复/簇拥挤度）。
  - 基于上述特征的 `composite` 融合排序。
- Out:
  - 训练端到端 ranker。
  - 用户可见的正式标签系统。
  - 个性化偏好建模。
  - 在线多轮 LLM 反思链路。

## Problems In Current Pipeline
- `quality` 过度依赖标题/摘要/正文长度，容易奖励灌水。
- `relevance` 主要是标题与正文 token overlap，容易被关键词堆叠欺骗。
- 当前 `novelty` 实际接近“新鲜度 + 长度”，不是跨文章新颖度。
- 输入校验过弱，抓取残片、模板噪音和极短文本也可能进入评分。
- 标签层为空，排序无法利用主题/意图/质量信号等稀疏高价值特征。

## Proposed Architecture

### 1. Input Gate
- 输入字段：`title`、`summary`、`full_content`、`link`、`published_at`。
- 预处理：
  - HTML 清洗。
  - 空白折叠。
  - 提取可见文本与 token。
- 最低门槛：
  - 标题或正文至少一个非空。
  - 正文可见字符数达到最小阈值。
  - token 数达到最小阈值。
  - URL/导航/版权/模板噪音占比不过高。
- 输出：
  - `valid`
  - `degraded`
  - `invalid`
- 行为：
  - `invalid` 不进入正常高分路径。
  - `degraded` 可排序，但总分受限并携带低置信度。

### 2. Readability Fallback
- 当输入门控失败且尚未做过正文抽取时，触发一次 Readability。
- 仅重试一次，避免无限补救链路。
- Readability 成功后重新走输入门控。
- 若仍失败，则维持 `degraded/invalid` 状态，不再继续深度分析。

### 3. Single-Article Features
- 规则特征：
  - `quality`：结构完整度、正文有效占比、噪音控制、元数据完整性。
  - `relevance`：标题与正文的一致性、摘要与正文的一致性。
  - `depth`：信息密度、段落展开度、独特 token 比例、句段丰富度。
  - `freshness`：时间衰减后的新鲜度。
- LLM 特征：
  - `topic_tags`
  - `intent_tags`
  - `quality_signal_tags`
  - 每个标签包含 `name`、`confidence`、`evidence_spans`
- 约束：
  - LLM 不输出最终 `composite`。
  - 标签只做内部排序特征，不视为真值分类。

### 4. Recent-Pool Novelty
- 维护一个最近文章窗口，作为新颖度比较池。
- 对当前文章与窗口做语义相似度比较，得到：
  - 最近近重复率
  - 最近高相似文章数
  - 所在语义簇拥挤度
- `novelty` 只由这层计算，不再由单篇长度/封面等特征近似替代。
- 最近池只影响 `novelty/diversity`，不反向污染单篇 `quality/relevance/depth`。

### 5. Fusion Ranking
- 最终维度：
  - `quality`
  - `relevance`
  - `depth`
  - `freshness`
  - `novelty`
  - `composite`
- 初始融合原则：
  - 规则特征提供稳定骨架。
  - `novelty` 提供跨文章去重/多样性约束。
  - 标签通过弱特征加权影响排序：
    - 例如 `deep-analysis` 提升 depth 相关权重。
    - `breaking-news` 提升 freshness 权重。
    - `seo-slop`、`roundup` 降低 quality 上限。
- `degraded/invalid` 输入会触发总分上限或惩罚项。

## Data Model Direction
- `entries` 继续保存基础分数字段。
- 新增或拆分持久化对象：
  - `article_features`
  - `article_tags`
  - 近期文章相似度索引或最近窗口缓存
- 兼容策略：
  - 旧 `novelty` 排序入口保留一个兼容周期。
  - 内部语义逐步切换为真正的跨文章新颖度。

## LLM Tag Schema (v1)
```json
{
  "topic_tags": [
    { "name": "distributed-systems", "confidence": 0.91, "evidence_spans": ["..."] }
  ],
  "intent_tags": [
    { "name": "postmortem", "confidence": 0.83, "evidence_spans": ["..."] }
  ],
  "quality_signal_tags": [
    { "name": "deep-analysis", "confidence": 0.78, "evidence_spans": ["..."] },
    { "name": "seo-slop", "confidence": 0.08, "evidence_spans": ["..."] }
  ]
}
```

## Failure Handling
- LLM 超时/失败：保留规则特征，标签层为空，不阻断主流程。
- Readability 失败：保留原始文本并走门控降级。
- 最近池不可用：`novelty` 回退到中性值，不让排序完全失效。

## Validation Plan
- 单元测试：
  - 输入门控对空文本、短文本、模板噪音、Readability 兜底的判定。
  - 最近池近重复时 `novelty` 降低。
  - 标签特征缺失时排序仍稳定。
- 手动验证：
  - 同主题近似文章不再长期堆满前排。
  - 深度文章在非最新排序下更容易靠前。
  - 明显残片/模板页不会得到高分。

## External References
- BestBlogs 实践仓库
  - 概要：采用“初评 -> 深度分析”分层 workflow，说明先过滤再深度分析是有效结构；但其大量 LLM 节点更像内容生产流，不适合直接照搬到实时排序主路径。
  - 来源：https://github.com/ginobefun/BestBlogs
- BestBlogs Dify 工作流实践文档
  - 概要：展示了结构化输出标签、摘要、评分与复核思路，适合作为“标签与评分分层”的参考，不适合作为最终排序执行模型。
  - 来源：https://github.com/ginobefun/BestBlogs/blob/main/flows/Dify/BestBlogs.dev%20%E5%9F%BA%E4%BA%8E%20Dify%20Workflow%20%E7%9A%84%E6%96%87%E7%AB%A0%E6%99%BA%E8%83%BD%E5%88%86%E6%9E%90%E5%AE%9E%E8%B7%B5.md
- MMR
  - 概要：经典多样性重排方法，适合作为最近文章池去重/多样性抑制的理论基础。
  - 来源：https://aclanthology.org/X98-1025/
- Sentence-BERT
  - 概要：提供低成本句向量与语义相似度能力，适合最近文章池相似度与近重复检测。
  - 来源：https://aclanthology.org/D19-1410/
- Recovering Lexically and Semantically Reused Texts
  - 概要：直接对应近似复用文本检测问题，可为跨文章新颖度与近重复识别提供参考。
  - 来源：https://aclanthology.org/2021.starsem-1.5/
- A Neural Local Coherence Model for Text Quality Assessment
  - 概要：说明质量评估应关注局部连贯性等结构信号，而不是单纯长度。
  - 来源：https://aclanthology.org/D18-1464/
- Weakly Supervised Text Classification using Supervision Signals from a Language Model
  - 概要：支持将 LLM 输出视为弱监督/弱特征而非最终真值，符合本方案的标签定位。
  - 来源：https://aclanthology.org/2022.findings-naacl.176/
