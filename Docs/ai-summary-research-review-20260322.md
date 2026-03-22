# AI 摘要研究综述（2026-03-22）

## 1. 文档目的
- 本文整理了本次围绕 AI 文章摘要系统查阅的核心论文与工程启发。
- 聚焦问题：
  - 长文摘要应该采用什么范式
  - 摘要长度控制应放在哪一层
  - 为什么会出现“摘要被截断”“结尾停在半句”“只覆盖开头不覆盖结论”
  - 对 Zflow 当前 RSS/Readability/LLM 摘要链路的直接启发是什么
- 本文面向后续论文写作与系统设计，尽量采用“研究问题 -> 论文结论 -> 工程含义”的结构。

## 2. 研究主题概览

### 2.1 长文摘要范式
- 平铺式长上下文摘要通常不稳定，尤其容易弱化文档中段和结尾的关键信息。
- 更稳妥的路径是层级摘要（hierarchical summarization）：
  - 先对局部片段提炼信息
  - 再做聚合摘要
  - 聚合阶段尽量保留原文锚点，而不是只看压缩后的中间摘要

### 2.2 长度控制范式
- 摘要长度控制不应主要依赖生成后的字符截断。
- 更合理的方法是：
  - 在信息选择阶段控制“哪些内容必须被覆盖”
  - 在 prompt 中把长度作为软约束（soft guidance）
  - 让模型优先保证完整性和自然收尾

### 2.3 当前工程风险
- 过短的阶段摘要会造成不可逆信息损失。
- 过强的“concise / 2-4 句”约束会把完整性牺牲掉。
- 最终聚合如果只看阶段摘要，不回看原文片段，容易放大遗漏和失真。
- 这些问题会共同导致：
  - 摘要停在半句
  - 只总结开头导语
  - 中后段结论被压没

## 3. 核心论文分组综述

### 3.1 层级摘要与长文结构建模

#### Liu and Lapata, 2019
- 论文：Hierarchical Transformers for Multi-Document Summarization
- 链接：https://aclanthology.org/P19-1500/
- 核心结论：
  - 层级式编码优于把长输入简单平铺拼接。
  - 文本单元之间的结构关系需要被保留，否则摘要模型更容易丢掉跨段依赖。
- 对本文的启发：
  - 长文摘要不应只做“截块 + 压缩”，而应尽量保留局部结构和全局聚合两层语义。

#### Huang et al., 2021
- 论文：Efficient Attentions for Long Document Summarization
- 链接：https://aclanthology.org/2021.naacl-main.112/
- 核心结论：
  - 长文摘要需要更有效的注意力机制来定位 salient regions。
  - 如果长输入中的显著区域不能被稳定识别，摘要质量和忠实性都会下降。
- 对本文的启发：
  - 长文链路中“选什么内容给模型看”比单纯“给更多 token”更重要。

#### Xiao et al., 2022
- 论文：PRIMERA: Pyramid-based Masked Sentence Pre-training for Multi-document Summarization
- 链接：https://aclanthology.org/2022.acl-long.360/
- 核心结论：
  - Pyramid / hierarchical pretraining 对跨段聚合和多源信息整合有明显帮助。
- 对本文的启发：
  - 生产系统中的 chunk-then-aggregate 方案本质上就在模拟金字塔式摘要流程。
  - 聚合阶段必须有足够上下文，不能只吃极短中间摘要。

#### Liu et al., 2022
- 论文：Two-Stage Movie Script Summarization
- 链接：https://aclanthology.org/2022.creativesumm-1.9/
- 核心结论：
  - 对超长叙事文本，两阶段或多阶段摘要比一遍压过去更实用。
- 对本文的启发：
  - 对特别长的文章，分块和多窗口聚合是合理的，不必追求单次全量摘要。

### 3.2 长度控制与可控摘要

#### Chan, Wang, King, 2021
- 论文：Controllable Summarization with Constrained Markov Decision Process
- 链接：https://aclanthology.org/2021.tacl-1.72/
- 核心结论：
  - 长度控制本身是独立优化目标。
  - 过度强化长度约束会压缩掉信息量和可读性。
- 对本文的启发：
  - “短”不能成为压倒性目标；否则用户最终看到的不是摘要，而是受损摘要。

#### Fan, Grangier, Auli, 2018
- 论文：Controllable Abstractive Summarization
- 链接：https://aclanthology.org/W18-2706/
- 核心结论：
  - 长度等摘要属性可以通过显式控制信号引导，但控制信号必须进入生成过程本身。
- 对本文的启发：
  - 若要控制摘要风格，应在 prompt/规划阶段处理，而不是在结果上后裁剪。

#### Liu, Jia, Zhu, 2022
- 论文：Length Control in Abstractive Summarization by Pretraining Information Selection
- 链接：https://aclanthology.org/2022.acl-long.474/
- 核心结论：
  - 长度控制更适合在信息选择阶段完成，而不是只在解码或后处理阶段强压。
- 对本文的启发：
  - “先决定该覆盖什么，再决定如何自然地写出来”优于“先生成，再截短”。

#### He et al., 2022
- 论文：CTRLsum: Towards Generic Controllable Text Summarization
- 链接：https://aclanthology.org/2022.emnlp-main.396/
- 核心结论：
  - 通过关键词/提示显式控制摘要关注点，是一种可实用化的泛化控制方式。
- 对本文的启发：
  - 对生产系统来说，可以用 prompt 明确要求覆盖“背景、主体变化、结论”等元素，而不是只说“简洁”。

### 3.3 长文聚合中的失真与位置偏置

#### Ou and Lapata, 2025
- 论文：Context-Aware Hierarchical Merging for Long Document Summarization
- 链接：https://aclanthology.org/2025.findings-acl.289/
- 核心结论：
  - 如果最终聚合只依赖中间摘要，遗漏和幻觉会被放大。
  - 更好的做法是让最终合并阶段同时看到源上下文。
- 对本文的启发：
  - 最终聚合 prompt 应同时包含阶段摘要和对应原文窗口，形成 re-grounding。

#### Jain et al., 2025
- 论文：On Positional Bias of Faithfulness for Long-form Summarization
- 链接：https://aclanthology.org/2025.naacl-long.442/
- 核心结论：
  - 长文摘要存在明显位置偏置，模型更容易偏向开头和结尾，或者干脆弱化中段。
- 对本文的启发：
  - 长文选块不能只看前言，也不能只抽稀疏高分段；需要保护中段与结尾覆盖。

### 3.4 通用 Prompt 与 Query-Guided Summarization

#### OpenAI Prompting Guide
- 文档：https://platform.openai.com/docs/guides/text?api-mode=responses
- 核心结论：
  - Prompt 需要先定义任务目标和输出形态，再给内容；仅靠“be concise”这类单句约束不稳定。
  - 如果目标是让用户快速理解内容，应该把“可读性”“先说主旨”“单段输出”“自然收尾”写成显式约束。
- 对本文的启发：
  - 生产摘要 prompt 应把“扫读可懂”设成目标，而不是只把“完整”或“简洁”设成目标。

#### He et al., 2022（CTRLsum）
- 论文：CTRLsum: Towards Generic Controllable Text Summarization
- 链接：https://aclanthology.org/2022.emnlp-main.396/
- 核心结论：
  - Query、关键词和控制信号可以作为通用摘要生成的引导条件，而不必为每个题材训练一套专门模板。
- 对本文的启发：
  - 对工程系统而言，更通用的做法不是给每类文章写死模板，而是在 prompt 中明确“需要回答的问题”，例如主旨、最重要信息、最终结论。

#### Fan et al., 2018
- 论文：Controllable Abstractive Summarization
- 链接：https://aclanthology.org/W18-2706/
- 核心结论：
  - 摘要风格控制应进入生成过程，而不是依赖生成后的结果修剪。
- 对本文的启发：
  - 如果想让摘要更像“扫读版”，应增加重写式压缩，而不是在最终字符串上做裁剪。

## 4. 研究结论汇总

### 4.1 长文摘要的推荐范式
- 对生产系统来说，最稳妥的是层级式摘要：
  - 短文：单阶段摘要
  - 中长文：连续窗口分块 + 聚合
  - 超长文：多窗口带 + 聚合
- 不建议把全部长文都交给单次 prompt 平铺处理。

### 4.2 长度控制的正确位置
- 长度控制应主要发生在：
  - 信息选择阶段
  - prompt 约束阶段
- 不应主要发生在：
  - 结果生成后的字符截断
  - 过低的 `max_tokens`
  - 过短的阶段摘要限制

### 4.3 为什么会产生“半句截断”
- 常见根因不是“前端显示裁剪”，而是链路内多层过度压缩：
  - prompt 强制过短
  - 中间阶段摘要过短
  - 上游输出预算不足
  - 最终再做硬截断
- 这些因素叠加后，即使最终存储层不截断，结果仍会表现为“像被截断”。

### 4.4 为什么最终聚合要保留原文锚点
- 中间摘要天然会丢细节。
- 如果聚合阶段只看中间摘要，模型无法校正这些损失。
- 因此长文聚合阶段最好同时输入：
  - article title
  - source summary / rss summary
  - partial summaries
  - corresponding source chunks

### 4.5 为什么需要通用 Query 模式
- 生产系统面对的输入并不总是标准资讯文章，还可能是：
  - 帖子
  - 论坛回复
  - 评论串
  - 经验贴
  - 混合文本
- 如果 prompt 预设“这是篇文章”，就容易出现：
  - 语气不匹配
  - 过度复述结构细节
  - 忽略观点/态度类输入的结论
- 更稳妥的做法是使用 query-guided summarization：
  - 先问“最主要在说什么”
  - 再问“读者扫一眼最需要知道什么”
  - 最后问“结论/态度/结果是什么”
- 这种 query 不是搜索引擎查询，而是模型在生成前的信息选择框架。

## 5. 对 Zflow 摘要链路的直接启发

### 5.1 已落地的工程决策
- 最终 AI 摘要不再做固定长度截断。
- 中间阶段摘要不再做超短字符级硬截断。
- 最终聚合阶段同时读取阶段摘要与原文片段。
- 句边界完整性作为摘要质量信号之一暴露到调试链路。

### 5.2 推荐保留的长期原则
- 长度是软约束，不是硬上限。
- 摘要完整性优先于“极短”。
- 通用 prompt 不应假设输入一定是新闻或标准文章。
- Query 应服务主旨提炼与结论收束，而不是服务题材分类。
- 长文要尽量保留结构，而不是只保留若干离散高分块。
- 聚合阶段需要 re-grounding。
- 开发调试接口要能说明本次摘要走的是哪条策略链路。

## 6. 可直接引用的研究陈述
- 长文摘要更适合采用层级式处理，而非简单的平铺长上下文输入。
- 摘要长度控制的有效位置是信息选择与生成约束阶段，而不是生成后的结果裁剪。
- 过度强化摘要的简短性会明显损害信息完整性与自然收尾。
- 中间阶段摘要如果被过度压缩，会造成不可逆的信息损失，并在最终聚合时放大为遗漏和不完整结尾。
- 对长文摘要而言，最终聚合阶段同时保留原文锚点有助于缓解遗漏和失真。
- 对异构输入做通用摘要时，query-guided prompt 比题材特化模板更稳，因为它把模型注意力放在主旨、重点和结论上。

## 7. 使用建议
- 如果后续你要写论文中的 related work，可以按 3 条主线组织：
  - 长文摘要范式：hierarchical summarization / long-context summarization
  - 长度控制：controllable summarization / length control
  - 聚合失真：context-aware merging / positional bias / faithfulness
- 如果要补一条更贴近系统实现的 related work，也可以单独列：
  - 通用控制式摘要：controllable summarization / query-guided summarization / prompt-based controllability
- 如果要写系统设计部分，可以直接把 Zflow 的实现动机表述为：
  - 从“硬截断摘要”升级为“层级摘要 + 通用 query-guided prompt + 软长度控制 + 原文回锚聚合”的生产摘要流水线。

## 8. 参考文献清单
- Liu, Y., & Lapata, M. (2019). Hierarchical Transformers for Multi-Document Summarization. ACL 2019. https://aclanthology.org/P19-1500/
- Huang, L., et al. (2021). Efficient Attentions for Long Document Summarization. NAACL 2021. https://aclanthology.org/2021.naacl-main.112/
- Xiao, W., et al. (2022). PRIMERA: Pyramid-based Masked Sentence Pre-training for Multi-document Summarization. ACL 2022. https://aclanthology.org/2022.acl-long.360/
- Liu, Y., Jia, X., & Zhu, K. (2022). Length Control in Abstractive Summarization by Pretraining Information Selection. ACL 2022. https://aclanthology.org/2022.acl-long.474/
- Liu, et al. (2022). Two-Stage Movie Script Summarization. CreativeSumm 2022. https://aclanthology.org/2022.creativesumm-1.9/
- Chan, H. P., Wang, L., & King, I. (2021). Controllable Summarization with Constrained Markov Decision Process. TACL 2021. https://aclanthology.org/2021.tacl-1.72/
- Fan, A., Grangier, D., & Auli, M. (2018). Controllable Abstractive Summarization. WNMT/NGT 2018. https://aclanthology.org/W18-2706/
- He, J., et al. (2022). CTRLsum: Towards Generic Controllable Text Summarization. EMNLP 2022. https://aclanthology.org/2022.emnlp-main.396/
- Ou, J., & Lapata, M. (2025). Context-Aware Hierarchical Merging for Long Document Summarization. Findings ACL 2025. https://aclanthology.org/2025.findings-acl.289/
- Jain, et al. (2025). On Positional Bias of Faithfulness for Long-form Summarization. NAACL 2025. https://aclanthology.org/2025.naacl-long.442/
- OpenAI. Text generation prompting guide. https://platform.openai.com/docs/guides/text?api-mode=responses
