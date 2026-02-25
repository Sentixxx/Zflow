# TECH REFER: Feed Script Contract (v1)

## Summary
- 目标：在 `抓取 -> 入库 -> 展示` 管线中插入“每个订阅源可配置脚本”的可扩展层。
- 核心原则：脚本交互使用结构化 JSON 契约；HTML 仅作为输出字段之一，不使用“裸 stdout=HTML”的隐式协议。
- 默认语言支持：`shell`、`python`、`javascript`。

## Scope
- In:
  - 每个 Feed 可配置脚本内容 + 脚本语言。
  - 刷新订阅时按 item 调用脚本。
  - 脚本返回结构化 JSON，可覆盖标题、摘要 HTML、全文 HTML 等字段。
  - 失败回退策略与可观测日志。
- Out:
  - 沙箱隔离（容器级）不在 v1 实现。
  - 远程脚本市场/签名分发不在 v1 实现。

## Data Model
- Feed
  - `custom_script: string`
  - `custom_script_lang: "shell" | "python" | "javascript"`
- Entry
  - `summary: string`（RSS 原始摘要）
  - `full_content: string`（脚本输出全文 HTML）

## API
- `PATCH /api/v1/feeds/{id}/script`
  - Request:
```json
{
  "script": "string",
  "script_lang": "shell|python|javascript"
}
```
  - Response: feed 对象（包含脚本字段）

## Script I/O Contract (v1)
- 输入：stdin JSON（每条 item 一次）
```json
{
  "version": "v1",
  "feed": {
    "id": 123,
    "url": "https://example.com/feed.xml"
  },
  "item": {
    "title": "Title",
    "link": "https://example.com/article",
    "summary": "<p>short summary</p>",
    "published_at": "2026-02-25T10:00:00Z"
  }
}
```

- 输出：stdout JSON
```json
{
  "ok": true,
  "title": "optional title override",
  "summary_html": "<p>optional summary html</p>",
  "content_html": "<article>optional full html</article>",
  "excerpt_text": "optional plain excerpt",
  "meta": {
    "author": "optional",
    "tags": ["optional"]
  },
  "debug": "optional debug message"
}
```

## Merge Rules
- `ok != true`：视为失败，回退原始 RSS 字段。
- `title` 非空：覆盖 entry.title。
- `content_html` 非空：写入 entry.full_content，前端优先展示。
- `summary_html` 非空：覆盖 entry.summary。
- `excerpt_text` 仅作辅助文本，不直接覆盖 HTML 字段（v1）。

## Runtime Semantics
- 退出码：
  - `0` + 有效 JSON + `ok=true` -> 应用结果
  - 非 `0` 或 JSON 解析失败 -> 回退
- 超时：单 item 脚本执行超时（建议 12s）。
- 日志：记录 feed_id、item.link、错误摘要，不中断整个 feed 刷新。
- 兼容：若脚本为空，跳过脚本阶段，按现有管线入库。

## Language Launchers
- `shell` -> `/bin/sh -lc <script>`
- `python` -> `python3 -c <script>`
- `javascript` -> `node -e <script>`

## Frontend Settings Requirements
- 设置卡片新增脚本编辑区：
  - 选择订阅源
  - 选择语言（shell/python/javascript）
  - 编辑/上传脚本
  - 保存按钮调用 `/api/v1/feeds/{id}/script`
- 提示文案必须明确：
  - “stdin 为 JSON v1”
  - “stdout 必须返回 JSON”
  - “`content_html` 为最终全文 HTML 字段”

## Security Notes (v1)
- 风险：用户脚本是任意代码执行，仅适用于自托管可信环境。
- 最小限制建议：
  - 执行超时
  - 输出大小上限
  - 错误重试次数限制
  - 失败后自动回退
- 后续可演进：
  - 脚本执行隔离（容器/沙箱）
  - 能力白名单（网络/文件系统）

## Example Scripts
- Python:
```python
import json,sys
inp=json.load(sys.stdin)
item=inp["item"]
out={
  "ok": True,
  "content_html": f"<article><h1>{item['title']}</h1><p>Link: <a href='{item['link']}'>{item['link']}</a></p></article>"
}
print(json.dumps(out, ensure_ascii=False))
```

- JavaScript:
```javascript
const fs = require("fs");
const inp = JSON.parse(fs.readFileSync(0, "utf8"));
const item = inp.item;
const out = {
  ok: true,
  summary_html: `<p>${item.title}</p>`,
  content_html: `<article><a href="${item.link}">${item.link}</a></article>`
};
process.stdout.write(JSON.stringify(out));
```

