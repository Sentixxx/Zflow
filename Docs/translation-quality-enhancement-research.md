# 翻译质量增强方案调研

> 基于 2024–2025 学术论文和开源实践的调研，聚焦 LLM 驱动的文章翻译场景，为 Zflow 沉浸式翻译功能的下一步优化提供方向参考。

## 1. 现状与约束

Zflow 当前翻译链路（参见 `Docs/ai-translation-chain.md`）：

1. 前端提取文章 HTML，`buildTranslationTemplate()` 拆分出可翻译段落
2. 后端逐段调用 OpenAI 兼容 Chat Completions，NDJSON 流式返回
3. 前端按段增量渲染原文/译文对照

**主要瓶颈：**

- 逐段独立翻译，段间缺乏上下文连贯性（术语不一致、指代断裂）
- 单次生成，无质量校验或自我纠正机制
- Prompt 固定，不感知文章领域/风格/术语

---

## 2. 核心研究发现

### 2.1 Agentic Reflection 工作流

**来源：** Andrew Ng — [translation-agent](https://github.com/andrewyng/translation-agent)

三步式反思翻译流程：

```
Translate → Reflect → Refine
```

1. **Translate**：LLM 生成初始翻译
2. **Reflect**：同一 LLM 审视翻译，生成改进建议（术语一致性、风格适配、漏译检查）
3. **Refine**：根据建议重新生成优化译文

**关键设计点：**

- 通过 Prompt 注入国家/地区参数（如"墨西哥口语西班牙语"而非通用西班牙语），控制方言和语体
- 通过 Glossary 注入领域术语表，保证关键术语翻译一致
- 文档级评估优于句级 BLEU：人工评测常优于商用 MT，但句级 BLEU 有时偏低

**对 Zflow 的适用性：** 高。RSS 文章篇幅适中（通常 < 5000 字），三步流程的额外延迟可接受。核心收益在术语一致性和风格自然度上。

### 2.2 MAPS：多维知识引导翻译

**来源：** He et al., TACL 2024 — [Exploring Human-Like Translation Strategy with Large Language Models](https://arxiv.org/abs/2305.04118)

MAPS（Multi-Aspect Prompting and Selection）模拟人类译员的准备过程：

1. **Knowledge Mining**：提示 LLM 从源文本中提取三类翻译知识
   - **Keywords**：关键术语及其推荐译法
   - **Topics**：文章主题/领域（科技、政治、体育等）
   - **Demonstrations**：相关翻译示例
2. **Knowledge Integration**：将上述知识注入翻译 Prompt
3. **Quality Selection**：使用 COMET-QE 等质量评估模型过滤低质量知识

**实验结果：** 幻觉减少 59%，歧义、误译、遗漏等多类错误显著下降。

**对 Zflow 的适用性：** 中高。Keywords 和 Topics 提取成本低（一次 LLM 调用），可在翻译前作为 preamble 注入。Demonstrations 需要翻译记忆库支持，初期可省略。

### 2.3 系统化自我纠正（TEaR）

**来源：** Feng et al., 2024 — [Improving LLM-based Machine Translation with Systematic Self-Refinement](https://arxiv.org/abs/2402.16379)

三阶段流水线：

1. **Translation**：生成初始译文
2. **Evaluation**：LLM 评估译文质量，定位具体错误（术语、流畅度、忠实度）
3. **Refinement**：基于错误反馈修正译文

**核心发现：**

- 不同评估策略对最终修正效果影响巨大，评估 Prompt 的设计比翻译 Prompt 更关键
- 通用 LLM 同时具备翻译和评估能力，无需独立 QE 模型
- 跨模型组合（Model A 翻译 + Model B 评估）有时优于单模型自评

### 2.4 文档级上下文感知翻译

**来源：** 多篇综述，核心参考：
- [Beyond the Sentence: A Survey on Context-Aware MT with LLMs](https://arxiv.org/abs/2506.07583) (Appicharla et al., 2025)
- [Document-Level Machine Translation with LLMs](https://arxiv.org/abs/2304.02210)

**关键发现：**

- 整段翻译（paragraph-level）质量显著优于逐句翻译：误译、语法错误、风格不一致均减少
- 上下文窗口策略：前 N 段已译文本作为上下文注入当前段的翻译 Prompt，保持术语和风格连贯
- 篇章结构（discourse structure）信息可通过 DAG 建模段落间依赖关系，指导翻译顺序

**对 Zflow 的适用性：** 高。当前逐段独立翻译是最大质量瓶颈。简单的滑动窗口上下文（前 2-3 段译文）即可大幅提升连贯性，实现成本极低。

### 2.5 MQM 错误标注引导后编辑

**来源：** Ki & Carpuat, NAACL 2024 — [Guiding LLMs to Post-Edit MT with Error Annotations](https://arxiv.org/abs/2404.07851)

- 使用 MQM（Multidimensional Quality Metrics）标注翻译错误类型和位置
- 仅靠 Prompting 注入细粒度错误反馈效果不明确
- **Fine-tuning 后效果显著**：TER、BLEU、COMET 三项指标均提升
- 中英、英德、英俄三个语言对验证有效

**对 Zflow 的适用性：** 低（需要 fine-tuning）。但 MQM 错误分类体系可用于设计评估 Prompt。

### 2.6 术语一致性控制

**来源：** [Efficient Terminology Integration for LLM-based Translation](https://aclanthology.org/2024.wmt-1.51.pdf) (WMT 2024)；综合调研

**核心问题：** LLM 缺乏确定性行为，同一术语在同一文档中可能被翻译为不同词汇。

**应对策略：**

- **Glossary Prompting**：在系统 Prompt 中注入术语表 `{source_term: target_term}`
- **Translate-then-Refine**：先翻译再做术语一致性检查，修正不一致用词
- **翻译记忆（TM）**：复用历史翻译片段，对低资源语言 ROI 尤其高

**对 Zflow 的适用性：** 中。RSS 阅读器场景下用户通常不维护术语表，但可以从 feed 元数据（分类、标签）自动推断领域，并使用预设领域术语表。

---

## 3. 推荐实施路径

根据 **实现成本 / 质量收益** 排序，分三个阶段：

### Phase 1 — 低成本高收益（Prompt 层优化）

| 优化项 | 方法 | 预期收益 | 改动范围 |
|--------|------|----------|----------|
| 滑动窗口上下文 | 翻译第 N 段时，Prompt 注入前 2-3 段的原文+译文 | 术语一致性、指代连贯 | 后端翻译 Prompt |
| 领域感知 Prompt | 从 feed 标签/分类推断领域，注入 system prompt | 术语准确度 | 后端翻译 Prompt |
| 目标语言区域化 | 支持 `zh-CN` / `zh-TW` / `es-MX` 等区域变体 | 方言适配 | 设置页 + Prompt |

### Phase 2 — 中等成本（Agentic 工作流）

| 优化项 | 方法 | 预期收益 | 改动范围 |
|--------|------|----------|----------|
| Reflect + Refine | 翻译完成后增加反思-修正轮次（可选开关） | 减少误译/幻觉 | 后端新增流程步骤 |
| Keywords 预提取 | 翻译前先提取关键术语及推荐译法，注入后续段落 | 术语一致性 | 后端新增预处理步骤 |
| 质量自评 | 翻译后 LLM 自评 1-5 分，低分段自动触发重译 | 减少低质量输出 | 后端 + 前端标记 |

### Phase 3 — 高成本探索（架构级变更）

| 优化项 | 方法 | 预期收益 | 改动范围 |
|--------|------|----------|----------|
| 用户术语表 | 允许用户维护 feed 级/全局术语表 | 深度定制 | 数据库 + 设置页 + Prompt |
| 翻译记忆 | 缓存已翻译片段，相似段落复用 | 一致性 + 成本节约 | 数据库 + 向量检索 |
| 跨模型评估 | Model A 翻译 + Model B 评估/修正 | 质量上限 | 多模型配置 |
| 批量段落翻译 | 合并短段落为批次一次翻译 | 减少 API 调用 + 上下文连贯 | 后端分段逻辑重构 |

---

## 4. Phase 1 具体设计草案

### 4.1 滑动窗口上下文

当前后端逐段翻译 Prompt 大致为：

```
Translate the following text to {target_lang}:

{paragraph}
```

改为：

```
You are translating an article about {topic}. Maintain consistent
terminology and style with the preceding context.

### Previous context (for reference only, do NOT re-translate):
Original: {prev_source_1}
Translation: {prev_translation_1}

Original: {prev_source_2}
Translation: {prev_translation_2}

### Translate the following:
{current_paragraph}
```

窗口大小建议 2-3 段。首段无上下文时退化为当前行为。

### 4.2 领域感知

从 feed 元数据提取领域信号：

```go
// 优先级：feed.category > feed.title 关键词匹配 > "general"
domain := inferDomain(feed)
```

注入 system prompt：

```
You are a professional translator specializing in {domain} content.
```

预设领域映射示例：

| Feed 信号 | 领域 | System Prompt 关键词 |
|-----------|------|---------------------|
| technology, programming, AI | 科技 | technology and software engineering |
| finance, economy, market | 财经 | finance and economics |
| science, research, arxiv | 学术 | academic and scientific research |
| news, politics | 新闻 | journalism and current affairs |
| 默认 | 通用 | general content |

---

## 5. LLM API 测试方案调研

### 5.1 业界实践

LLM 集成代码的测试面临三个核心挑战：**非确定性**（同一 prompt 不同返回）、**高成本**（每次调用消耗 token）、**慢速**（网络延迟）。业界主流应对策略：

| 层级 | 策略 | 测什么 | 不测什么 |
|------|------|--------|----------|
| 纯函数 | 直接调用，零依赖 | Prompt 构建、上下文裁剪、参数校验 | — |
| HTTP Mock | `httptest.NewServer` 模拟 API 端点 | 请求结构、header、流式解析 | LLM 实际生成质量 |
| Prompt 快照 | 在 mock 中捕获发送的完整 prompt | 关键片段是否存在（防回归） | 语义正确性 |
| LLM-as-Judge | 调用真实 LLM 评估输出质量 | 翻译质量、一致性 | CI 中不适合（成本+慢） |

**核心原则**（来源：[Mocking OpenAI](https://laszlo.substack.com/p/mocking-openai-unit-testing-in-the)）：

- Mock HTTP 层而非客户端层：`httptest` 拦截所有出站请求，无需修改业务代码
- 测试发出的 prompt 是否正确，而非 LLM 返回什么：prompt 结构是我们的代码，LLM 行为是上游的
- 确定性断言优先：JSON 合法性、必含字段、长度约束等零成本检查覆盖大部分回归

**三层测试架构**（来源：[Rethinking Testing for LLM Applications](https://arxiv.org/abs/2508.20737)）：

1. **System Shell 层**：传统单元测试直接适用
2. **Prompt Orchestration 层**：需要"语义重新解释"传统方法 — 即 prompt 快照测试
3. **LLM Inference Core 层**：需要全新范式 — LLM-as-Judge / 人工评估

### 5.2 在 Zflow 中的落地

采用 Go 标准库 `httptest` + 项目已有的 `NewTestSQLiteFeedRepository()` 测试基础设施：

```
ai_handlers_test.go
├── 纯函数测试（11 个用例）
│   ├── buildTranslationContext: 空/单段/首段钉住/超长截断/跳过空白
│   ├── selectContextIndices: 小/大 history
│   └── buildTranslationSystemPrompt: 全字段/无摘要/空/长摘要截断
└── httptest 集成测试（3 个用例）
    ├── PromptContainsArticleContext: 捕获请求体，断言标题+来源
    ├── SlidingWindowProgression: 3段翻译，验证上下文递增
    └── AnthropicProtocol: 验证 header + body 结构差异
```

关键设计决策：

- mock server 同时被摘要管线和翻译管线命中 → 翻译前清空捕获列表，隔离两个管线的请求
- 使用 `sync.Mutex` 保护捕获的请求列表（handler 可能并发调用）
- 提取 `patchAI()`、`doRequest()`、`getFirstArticleID()` 三个 helper 避免测试样板代码

---

## 6. 参考文献

1. He, Z. et al. "Exploring Human-Like Translation Strategy with Large Language Models." *TACL*, 2024. [arXiv:2305.04118](https://arxiv.org/abs/2305.04118)
2. Feng, Z. et al. "Improving LLM-based Machine Translation with Systematic Self-Refinement." 2024. [arXiv:2402.16379](https://arxiv.org/abs/2402.16379)
3. Ki, D. & Carpuat, M. "Guiding Large Language Models to Post-Edit Machine Translation with Error Annotations." *NAACL Findings*, 2024. [arXiv:2404.07851](https://arxiv.org/abs/2404.07851)
4. Gain, B. et al. "Bridging the Linguistic Divide: A Survey on Leveraging LLMs for Machine Translation." 2025. [arXiv:2504.01919](https://arxiv.org/abs/2504.01919)
5. Appicharla, R. et al. "Beyond the Sentence: A Survey on Context-Aware Machine Translation with LLMs." 2025. [arXiv:2506.07583](https://arxiv.org/abs/2506.07583)
6. Ng, A. et al. "Translation Agent: Agentic Translation using Reflection Workflow." 2024. [GitHub](https://github.com/andrewyng/translation-agent)
7. "Efficient Terminology Integration for LLM-based Translation." *WMT*, 2024. [ACL Anthology](https://aclanthology.org/2024.wmt-1.51.pdf)
8. "GRAFT: A Graph-based Flow-aware Agentic Framework for Document-level Machine Translation." 2025. [arXiv:2507.03311](https://arxiv.org/abs/2507.03311)
9. "Mocking OpenAI — Unit testing in the age of LLMs." [Substack](https://laszlo.substack.com/p/mocking-openai-unit-testing-in-the)
10. "Rethinking Testing for LLM Applications: Characteristics, Challenges, and a Lightweight Interaction Protocol." 2025. [arXiv:2508.20737](https://arxiv.org/abs/2508.20737)
11. "Mocking External APIs in Agent Tests." [Scenario / LangWatch](https://langwatch.ai/scenario/testing-guides/mocks/)
