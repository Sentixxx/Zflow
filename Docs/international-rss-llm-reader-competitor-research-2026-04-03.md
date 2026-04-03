# 国际竞品研究：RSS + LLM Reader

日期：2026-04-03

## 结论先行

国际市场里，真正 relevant 的不是单一一种产品，而是四条不同路线：

- `Feedly`、`Inoreader`：更像“信息操作系统”，核心是抓取、监控、过滤、自动化。
- `Readwise Reader`：更像“AI 阅读工作台”，核心是阅读、标注、摘要、知识沉淀。
- `NewsBlur`：更像“可训练的 RSS”，核心是显式可解释过滤。
- `Miniflux`、`FreshRSS`：更像“自托管基础盘”，核心是低成本、自主可控、可长期归档。

从公开讨论看，用户反复抱怨的不是“AI 不够炫”，而是更基础的问题：未读和归档策略不顺手、大订阅量下性能与稳定性不够、组织与批量处理效率低、搜索与去重不够强、AI 功能价格过高或不够可控。这说明 RSS + LLM 产品真正的分水岭，不在“有没有摘要按钮”，而在“基础阅读器能力是否足够扎实，AI 是否真正嵌进主流程”。

## 研究范围与方法

- 研究对象：`Feedly`、`Inoreader`、`Readwise Reader`、`NewsBlur`、`Miniflux`、`FreshRSS`
- 证据类型：
  - 官方站点、帮助中心、功能页、博客、定价页
  - Reddit、官方论坛、GitHub issues 等公开讨论
- 说明：
  - 公开讨论只能反映高频抱怨样本，不等于总体统计结论
  - 无明确发布日期的官方页面统一写“访问日期：2026-04-03”

## 国际竞品对比表

| 产品 | 定位 | 核心能力 | AI 与过滤 | 主要短板 | 主要来源 |
| --- | --- | --- | --- | --- | --- |
| Feedly | 从通用 RSS 阅读器延展到研究/情报工作台 | RSS/新闻源聚合、Boards、团队共享、趋势与公司卡片 | Feedly AI、AI Feeds、Ask AI，偏大规模过滤与情报提炼 | 30/31 天未读限制、旧文处理弱、已读交互 bug、AI 价格高 | Feedly docs、pricing、Reddit |
| Inoreader | 内容中枢 + 自动化平台 | RSS、newsletter、monitoring feeds、rules、filters、导出/API | Inoreader Intelligence、摘要、标签建议、重复过滤、自动规则 | 导入/加源流程不直观、部分源失败率高、价格争议、旧文保留策略一般 | Inoreader blog、pricing、Reddit |
| Readwise Reader | AI 阅读工作台 / read-it-later + RSS + 知识库 | RSS、网页、PDF、newsletter、视频、高亮、检索、多端同步 | Ghostreader：摘要、解释、问答、自定义 prompt | 组织与批处理效率不足、RSS 主流程不够强、重度使用下变慢 | Readwise docs、官网、Reddit |
| NewsBlur | 可训练、可解释、可自托管的 RSS 产品 | RSS、River of News、全文搜索、训练器、第三方客户端同步 | 显式训练作者/标题/标签/正文/URL，新增自然语言分类 | 大账户/重度场景稳定性问题、更新/计数异常、部分 feed 抓取报错 | NewsBlur features、blog、forum |
| Miniflux | 极简、自托管、强主见 RSS 阅读器 | Go 单二进制、全文搜索、规则改写、全文抓取、Webhook、API | 过滤主要靠规则与基础自动化，不靠复杂 AI | 组织能力弱、去重不足、原生移动体验弱、功能边界刻意收敛 | Miniflux features、opinionated、GitHub、Reddit |
| FreshRSS | 全能型自托管 RSS 聚合器 | 多用户、扩展系统、WebSub、XPath 抓取、CLI、客户端 API | 扩展性强，但 AI 不是主轴 | 移动/API 兼容摩擦、大规模订阅下错误治理一般、抓取调度可能过激、生态碎片化 | FreshRSS docs、GitHub issues、Reddit |

## 国际用户痛点表

| 痛点主题 | 代表产品 | 典型表现 | 证据来源 |
| --- | --- | --- | --- |
| 未读与归档策略 | Feedly、Inoreader | 旧文不能长期保留为未读，低频订阅补读体验差 | Feedly Reddit 2025-11-20、2026-02-01；Inoreader Reddit 2023-12-06 |
| 组织与批量处理 | Readwise Reader、Miniflux | 文件夹、排序、批量归档、多分类能力不足 | Readwise Reddit 2025-05-26；Miniflux GitHub issue 2024-03-29 |
| 搜索、去重、过滤 | Readwise Reader、Miniflux、Inoreader | 全库搜索弱、重复内容治理弱、过滤链路复杂或不稳定 | Readwise Reddit 2024-12-28、2025-05-01；Miniflux GitHub issue 2020-09-14 |
| 大订阅量下性能与稳定性 | Readwise Reader、FreshRSS、NewsBlur | 多 feeds / 多视图时变慢、更新异常、错误处理不足 | Readwise Reddit 2025-05-07；FreshRSS GitHub issue 2023-05-14；NewsBlur forum 2025-10-24 |
| 移动端与多端一致性 | FreshRSS、Miniflux、NewsBlur | 社区客户端能力不齐、同步和访问稳定性不足 | FreshRSS Reddit 2020-08-01；Miniflux Reddit 2020-05-10；NewsBlur forum 2025-06-24 |
| AI 成本、配额、可控性 | Feedly、Readwise Reader | AI 有吸引力，但成本高、价值感和控制权争议大 | Feedly pricing + Reddit 2025-11-20；Readwise docs + Reddit 2024-03-18 |

## 对自托管 RSS + LLM 路线的启示

- 国际竞品已经证明两件事：
  - 用户愿意为“过滤噪音”和“加速理解”付费。
  - 但如果基础阅读器体验不稳，AI 会被视为外挂而不是主价值。
- 自托管路线的机会不在“做一个更便宜的 Feedly”，而在“把成本、规则、数据、模型调用都变得更可控”。
- 对比这些产品后，最值得持续盯的不是某个具体功能，而是四类能力能否同时成立：
  - 长期可归档的未读与收藏策略
  - 高密度但不混乱的组织与批处理
  - 中文/多源场景下稳定的搜索、去重、过滤
  - 可解释、可关闭、可替换模型的 AI 流程

## 来源清单

### 官方来源

- 来源：Saving AI Feeds in Feedly，2025-07-24，https://docs.feedly.com/article/769-saving-ai-feeds-feedly
- 来源：Meet Feedly AI for Market Intelligence，2024-01-16，https://feedly.com/new-features/posts/meet-feedly-ai-for-market-intelligence
- 来源：Feedly Market Intelligence Pricing，访问日期：2026-04-03，https://feedly.com/market-intelligence/pricing
- 来源：Inoreader 2025: Intelligence and automation in one content hub，2025-12-23，https://www.inoreader.com/blog/2025/12/inoreader-2025-intelligence-and-automation-in-one-content-hub.html
- 来源：Inoreader Intelligence and article summaries are here，2025-03-11，https://www.inoreader.com/sk/blog/2025/03/inoreader-intelligence-and-article-summaries-are-here.html
- 来源：Inoreader Pricing / Spotlights，访问日期：2026-04-03，https://www.inoreader.com/pricing/feature/Spotlights
- 来源：Readwise Reader Docs，访问日期：2026-04-03，https://docs.readwise.io/reader
- 来源：Ghostreader Overview，访问日期：2026-04-03，https://docs.readwise.io/reader/guides/ghostreader/overview
- 来源：Ghostreader FAQ，访问日期：2026-04-03，https://docs.readwise.io/reader/docs/faqs/ghostreader
- 来源：Readwise Reader 官网，访问日期：2026-04-03，https://readwise.io/read
- 来源：NewsBlur FAQ，访问日期：2026-04-03，https://www.newsblur.com/faq
- 来源：NewsBlur Features，访问日期：2026-04-03，https://www.newsblur.com/features
- 来源：Natural language text and image classifiers，2026-04-02，https://blog.newsblur.com/2026/04/02/natural-language-text-and-image-classifiers/
- 来源：Intelligence trainer overhaul，2026-01-22，https://blog.newsblur.com/2026/01/22/intelligence-trainer-overhaul/
- 来源：Miniflux Features，访问日期：2026-04-03，https://miniflux.app/features.html
- 来源：Why Miniflux is opinionated，访问日期：2026-04-03，https://miniflux.app/opinionated.html
- 来源：FreshRSS 文档首页，访问日期：2026-04-03，https://freshrss.github.io/FreshRSS/en/
- 来源：FreshRSS Mobile access，访问日期：2026-04-03，https://freshrss.github.io/FreshRSS/en/users/06_Mobile_access.html
- 来源：FreshRSS Extensions，访问日期：2026-04-03，https://freshrss.github.io/FreshRSS/en/admins/15_extensions.html

### 公开讨论与问题样本

- 来源：It’s time to ditch Feedly?，2025-11-20，https://www.reddit.com/r/feedly/comments/1p1uy8p/its_time_to_ditch_feedly/
- 来源：Remove or filter out old articles，2026-02-01，https://www.reddit.com/r/feedly/comments/1qt04wg/remove_or_filter_out_old_articles/
- 来源：Feedly 已读操作问题讨论，2024-08-28，https://www.reddit.com/r/feedly/comments/1f3d4xi
- 来源：Inoreader cannot add subreddits，2026-03-27，https://www.reddit.com/r/InoReader/comments/1s5fgn1/cannot_add_subreddits/
- 来源：Inoreader 源失败与更新问题讨论，2025-12-31，https://www.reddit.com/r/InoReader/comments/1hqdf00
- 来源：Inoreader 价格与体验讨论，2024-06-01，https://www.reddit.com/r/InoReader/comments/1d51abd
- 来源：Inoreader 未读策略讨论，2023-12-06，https://www.reddit.com/r/InoReader/comments/18by5m8
- 来源：Readwise 搜索体验讨论，2024-12-28，https://www.reddit.com/r/readwise/comments/1hnvkrg/searching_in_readwise_reader_is_not_great/
- 来源：Readwise 缺少 full feed search，2025-05-01，https://www.reddit.com/r/readwise/comments/1kc8u4e/love_readwise_but_the_lack_of_full_feed_search_is/
- 来源：Readwise folder reorder 讨论，2025-05-26，https://www.reddit.com/r/readwise/comments/1kvp9uu/reorder_folders_in_readwise_reader_feed/
- 来源：Readwise 大量视图与条目下变慢，2025-05-07，https://www.reddit.com/r/readwise/comments/1kgxo7f/readwise_reader_sluggishness/
- 来源：Readwise 值不值得作为主 RSS reader，2024-03-18，https://www.reddit.com/r/readwise/comments/1bh5in8/is_readwise_readwise_reader_worth_it/
- 来源：NewsBlur OPML 下载 502，2025-04-14，https://forum.newsblur.com/t/opml-download-leads-to-http-502-for-1-500-feeds/11635
- 来源：NewsBlur iOS SSL error，2025-06-24，https://forum.newsblur.com/t/cant-access-newsblur-on-ios-due-to-ssl-error/11793
- 来源：NewsBlur news counter 异常，2025-10-23，https://forum.newsblur.com/t/issue-with-the-news-counter/13226
- 来源：NewsBlur feeds not updating，2025-10-24，https://forum.newsblur.com/t/newsblur-not-updating-feeds/13229
- 来源：I ditched Feedly and self-hosted Miniflux instead，2025-04-07，https://www.reddit.com/r/selfhosted/comments/1jtffa0/i_ditched_feedly_and_selfhosted_miniflux_instead/
- 来源：Miniflux multiple categories request，2024-03-29，https://github.com/miniflux/v2/issues/2575
- 来源：Miniflux duplicate entries request，2020-09-14，https://github.com/miniflux/v2/issues/797
- 来源：Miniflutt Android client discussion，2020-05-10，https://www.reddit.com/r/selfhosted/comments/gh18cd/miniflutt_an_android_client_for_miniflux/
- 来源：FreshRSS Android login/sync issue，2020-05-08，https://github.com/FreshRSS/FreshRSS/issues/2959
- 来源：FreshRSS all feeds show error，2023-05-14，https://github.com/FreshRSS/FreshRSS/issues/5402
- 来源：FreshRSS aggressive fetching / OpenRSS blocked，2024-11-03，https://github.com/FreshRSS/FreshRSS/issues/6973
- 来源：FreshRSS mobile apps discussion，2020-08-01，https://www.reddit.com/r/selfhosted/comments/i1snyr/what_mobile_app_do_you_use_with_your_freshrss/
