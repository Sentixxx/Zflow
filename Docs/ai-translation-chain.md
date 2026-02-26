# AI Translation Chain

## 1. 目标

打通「文章详情 -> 一键翻译 -> 后端调用 LLM -> 前端展示结果」的完整链路，保证用户在当前阅读视图内直接看到翻译结果。

## 2. 端到端流程

1. 用户在设置页填写 AI 参数（API Key / Base URL / Model / 默认目标语言）。
2. 用户在文章详情右下悬浮工具栏点击翻译按钮。
3. 前端调用 `POST /api/v1/articles/{id}/translate/stream`，默认语言使用“AI 设置”中的目标语言。
4. 后端按优先级提取可翻译文本：`full_content -> summary -> title`，并切分为段落。
5. 后端对每段逐个调用 OpenAI 兼容 Chat Completions，并以 NDJSON 流式返回 `start/chunk/done` 事件。
6. 前端按段落增量渲染「原文/译文」对照，尚未翻译段落显示加载动画。

## 3. 后端接口

- 路径：`POST /api/v1/articles/{id}/translate`
- 请求体（可选）：

```json
{
  "target_lang": "zh-CN"
}
```

- 响应体：

```json
{
  "article_id": 123,
  "target_lang": "zh-CN",
  "translated_text": "...",
  "source_char_count": 1024
}
```

## 4. AI 设置接口

- 读取：`GET /api/v1/settings/ai`
- 保存：`PATCH /api/v1/settings/ai`

```json
{
  "api_key": "sk-...",
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4o-mini",
  "target_lang": "zh-CN"
}
```

## 5. 流式翻译接口

- 路径：`POST /api/v1/articles/{id}/translate/stream`
- 类型：`application/x-ndjson`
- 事件：
  - `start`：总段落数
  - `chunk`：单段原文 + 译文
  - `done`：流结束
  - `error`：翻译失败

## 6. 前端接入点

- API 层：`frontend/src/api/client.ts` `translateArticle(id, targetLang?)`
- 页面编排：`frontend/src/App.tsx`
- 详情内容：`frontend/src/components/article-detail/ArticleDetailContent.tsx`
- 悬浮入口：`frontend/src/components/article-detail/ArticleFloatingActions.tsx`

## 7. 验证步骤

1. 打开设置页，在 `AI 设置` 填写并保存 API Key/Base URL/模型/默认目标语言。
2. 打开任意文章详情，点击右下翻译图标。
3. 预期：
   - 翻译请求期间按钮禁用；
   - 详情区逐段出现中英对照；
   - 未翻译段落显示加载动画；
   - 失败时显示错误状态提示。

## 8. 常见问题

- 问题：翻译返回空内容。  
  排查：确认文章是否存在可翻译源文本（`full_content/summary/title`）以及上游模型返回是否为空。

- 问题：接口报鉴权失败。  
  排查：确认设置页中 `API Key` 已保存。
