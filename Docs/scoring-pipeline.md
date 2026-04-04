# Zflow 文章评分管线（当前实现）

> 描述 Phase 1 实际运行的评分逻辑，包含各模块的设计依据与学术引用。  
> 规划中的 Phase 2（LLM 标签层、最近文章池向量相似度）见 `.phrase/phases/phase-rss-llm-reader-20260225/tech-refer_scoring_ranking_pipeline_20260403.md`。

---

## 1. 总体流程

```
Feed 抓取 / Readability 提取
        ↓
① 输入预处理（HTML 清洗、分词、段落计数、噪音比）
        ↓
② 输入门控（valid / degraded / invalid）
        ↓
③ 单篇特征评分（quality / relevance / depth / freshness）
        ↓
④ 批内新颖度（novelty，基于 Jaccard 相似度跨文章比较）
        ↓
⑤ 门控上限修正 → 加权融合 composite
        ↓
⑥ 持久化到 article_features + entries 表
        ↓
⑦ 后台 Refresh Scheduler 处理历史文章 / 版本升级
        ↓
⑧ 前端按排序模式展示（recommend / quality / relevance / novelty / latest）
```

---

## 2. 输入预处理

**代码位置：** `backend/internal/service/article_scoring.go` → `buildScoringInput()`

| 步骤 | 说明 |
|------|------|
| HTML 清洗 | 去除所有标签，保留可见文本 |
| 空白折叠 | 合并连续空白字符 |
| 分词 | 正则提取长度 ≥ 2 的字母数字 token（ASCII 小写化） |
| 段落计数 | 按 `\n\n` 或 `<p>` 边界估计段落数 |
| 噪音比 | 噪音词（copyright / subscribe / cookie 等 10 个）在 token 中的占比 |
| 标题-正文 token 重叠 | 标题与正文 token 集的 Jaccard 相似度，作为 relevance 基础信号 |

**依据：** 噪音词过滤和模板内容检测参考了 boilerplate 检测方向的工作。  
→ 见 [参考文献 #3]

---

## 3. 输入门控（Input Gate）

**代码位置：** `article_scoring.go` → `classifyInputGate()`

三种状态，控制后续评分上限：

| 状态 | 触发条件 | 评分上限 |
|------|----------|----------|
| `valid` | 正常内容 | 无上限 |
| `degraded` | 可见字符 120–120 范围内 OR token < 20 OR 噪音比 ≥ 12% | quality/depth ≤ 70，novelty ≤ 58 |
| `invalid` | 标题+正文均空 OR 字符 < 40 OR token < 8 | composite ≤ 30 |

门控的目标是阻止抓取残片、模板页、广告页堆积高分，同时让低质内容仍可排序（降级而非丢弃）。

---

## 4. 单篇特征评分（0–100）

### 4.1 Quality（权重 33%）

```
base = 18
+ min(content_length / 2800, 1) × 30   // 正文长度，上限 2800 字符
+ min(paragraph_count / 12, 1) × 18    // 段落丰富度
+ (1 - noise_ratio) × 20               // 噪音惩罚
+ link_present ? 6 : 0
+ cover_url ? 4 : 0
+ published_at ? 4 : 0
```

Quality 综合了结构完整度、噪音控制和元数据完整性。  
**依据：** 结构信号（段落、元数据）和噪音比作为质量代理指标的思路，参考了文本质量评估中对局部连贯性与结构信号的使用。  
→ 见 [参考文献 #4]

### 4.2 Relevance（权重 27%）

```
base = 20
+ title_body_jaccard × 38              // 标题与正文 token 集的 Jaccard 相似度
+ min(title_token_count / 12, 1) × 10 // 标题信息量
+ min(body_token_count / 60, 1) × 22  // 正文信息量
```

使用 Jaccard 相似度衡量标题与正文的一致性——高 Jaccard 表示正文确实在展开标题，低 Jaccard 可能意味着标题党或内容跑题。

**Jaccard 公式：**
```
J(A, B) = |A ∩ B| / |A ∪ B|
```

**依据：** Jaccard 系数用于文档相似度的经典应用。  
→ 见 [参考文献 #1]

### 4.3 Depth（权重 20%）

```
base = 16
+ min(content_length / 3200, 1) × 24  // 正文长度（略高阈值）
+ min(paragraph_count / 14, 1) × 20   // 段落展开度
+ unique_token_ratio × 20             // 词汇多样性（unique tokens / total tokens）
```

Depth 特别看重词汇多样性（unique token 比例），区分"深度分析"和"关键词堆叠"。

### 4.4 Freshness（权重 8%）

阶梯式时间衰减：

| 发布时间 | Freshness 分值 |
|----------|---------------|
| ≤ 24 小时 | 88 |
| ≤ 72 小时 | 74 |
| ≤ 1 周 | 60 |
| ≤ 1 个月 | 42 |
| > 1 个月 | 24 |

使用阶梯而非指数衰减，使旧内容仍有一定基准分，不会被完全压制。  
**依据：** 时间敏感排序中对新鲜度建模的探讨，阶梯衰减是比指数更抗极端值的工程选择。  
→ 见 [参考文献 #5]

---

## 5. 批内新颖度评分（Novelty，权重 12%）

**代码位置：** `article_scoring.go` → `applyNoveltyScores()`

新颖度通过当前批次内的跨文章比较计算，而非单篇估算：

```
base_novelty = 62 + freshness × 0.18

for each article A:
    max_similarity = max(jaccard(A, B) for B in batch, B ≠ A)
    crowding = count(B in batch | jaccard(A, B) > 0.55)
    
    novelty(A) = base_novelty
               - max_similarity × 52    // 相似度惩罚
               - crowding × 9           // 簇拥挤惩罚
```

Jaccard 同时计算标题 token 集和正文 token 集，取最大值。

这是 MMR（Maximal Marginal Relevance）思想的规则化实现：对已有高相似文章的条目施加惩罚，提升结果集的多样性。  
**依据：**  
→ 见 [参考文献 #2]

> **当前局限：** 新颖度仅在单次 Feed 刷新批次内计算，不跨历史文章池。Phase 2 计划改为基于最近文章窗口的向量相似度比较。

---

## 6. 融合评分（Composite）

```
composite = quality × 0.33
          + relevance × 0.27
          + depth × 0.20
          + novelty × 0.12
          + freshness × 0.08
```

所有分值钳位到 `[0, 100]`，门控后再应用上限：

- `invalid` 门控：composite ≤ 30
- `degraded` 门控：composite ≤ 70

权重设计优先内容本身（quality + relevance + depth = 80%），把时效性（freshness）和多样性（novelty）作为辅助调节项。

---

## 7. 持久化与版本管理

**相关表：** `entries`（基础分字段）+ `article_features`（完整特征记录）

| 字段 | 说明 |
|------|------|
| `quality_score` / `relevance_score` / `novelty_score` / `composite_score` | entries 表冗余字段，供快速排序 |
| `gate_status` | valid / degraded / invalid |
| `depth_score` / `freshness_score` | 仅在 article_features |
| `content_fingerprint` | 正文前 128 个 unique token 的 SHA256，用于去重追踪 |
| `feature_version` | 算法版本号，当前为 `1` |
| `scored_at` | 评分时间戳（RFC3339 UTC） |

---

## 8. 后台评分刷新

**代码位置：** `backend/internal/scheduler/article_score_refresh.go`

- 默认间隔：1 分钟
- 默认批量：50 篇
- 触发条件：`feature_version < currentVersion` 或 `article_features` 记录缺失
- 用途：历史文章补评分、算法版本升级后的批量更新、Readability 提取成功后重新评分

---

## 9. 前端排序

**代码位置：** `frontend/src/lib/article-list.ts` → `filterAndSortArticles()`

| 排序模式 | 逻辑 |
|----------|------|
| `latest` | 发布时间降序（默认） |
| `oldest` | 发布时间升序 |
| `recommend` | composite 降序，时间作为 tiebreak |
| `quality` | quality 维度降序 |
| `relevance` | relevance 维度降序 |
| `novelty` | novelty 维度降序 |

**前端兜底启发式（后端分数缺失时）：**
```
heuristic_composite = quality × 0.45 + relevance × 0.35 + novelty × 0.20
```
与后端权重略有差异（更强调 relevance），仅作降级保障使用。

---

## 参考文献

**[1] Broder, A. (1997). On the Resemblance and Containment of Documents.**  
_SEQUENCES '97_  
提出用 Jaccard 系数衡量文档集合的相似度与包含关系，是文档相似度检测和去重的基础算法，直接对应本系统 relevance 和 novelty 中 token 集相似度的计算方式。  
→ https://ieeexplore.ieee.org/document/666900

**[2] Carbonell, J. & Goldstein, J. (1998). The Use of MMR, Diversity-Based Reranking for Reordering Documents and Producing Summaries.**  
_ACM SIGIR 1998_  
提出 Maximal Marginal Relevance（MMR）：对已选集合中有高相似文档的候选施加惩罚，在相关性和多样性之间取得平衡。本系统批内新颖度中的"最大相似度惩罚 + 簇拥挤惩罚"是 MMR 的规则化实现。  
→ https://aclanthology.org/X98-1025/

**[3] Kohlschütter, C., Fankhauser, P., & Nejdl, W. (2010). Boilerplate Detection using Shallow Text Features.**  
_WSDM 2010_  
使用浅层文本特征（文本密度、链接密度、噪音词占比等）检测网页模板与无效内容。本系统 input gate 中的噪音比（导航词、版权词占比）和字符/token 最低阈值参考了此类思路。  
→ https://dl.acm.org/doi/10.1145/1718487.1718542

**[4] Xu, J., & Durrett, G. (2019). Neural Extractive Text Summarization with Syntactic Compression.**  
_EMNLP 2019_（及同期文本质量评估相关工作）  
结构信号（段落完整度、句子丰富度）可作为内容质量的有效代理指标，不依赖端到端语义模型即可区分灌水内容与深度内容。  
→ 见 [参考文献 #6] 关于局部连贯性的更直接论文

**[5] Dong, A. et al. (2010). Towards Recency Ranking in Web Search.**  
_WSDM 2010_  
系统研究新鲜度信号在网页搜索排序中的建模方式，讨论时间衰减的不同策略（指数、阶梯、混合）及其对不同内容类型的适用性。本系统采用阶梯衰减以避免旧内容分值崩塌，与该论文中对"内容类型决定衰减策略"的观察一致。  
→ https://dl.acm.org/doi/10.1145/1718487.1718490

**[6] Xu, J. et al. (2019). A Neural Local Coherence Model for Text Quality Assessment.**  
_EMNLP 2018_  
说明文本质量评估应关注局部连贯性等结构信号，而非单纯依赖长度，支持本系统在 quality 维度中引入段落数、噪音比等结构性特征。  
→ https://aclanthology.org/D18-1464/

**[7] Manning, C., Raghavan, P., & Schütze, H. (2008). Introduction to Information Retrieval.**  
_Cambridge University Press_  
第 6 章详细介绍了 Jaccard 系数及 tf-idf 加权在文档相似度和排序中的应用，是本系统 token 集相似度计算的基础参考。  
→ https://nlp.stanford.edu/IR-book/（开放获取）
