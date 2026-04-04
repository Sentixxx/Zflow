# Temp Plan: First Paint List Slimming (2026-04-04)

## Goal
- 改善阅读页首次打开速度，让文章列表首屏更快可见。

## Scope
- 后端 `/api/v1/articles` 列表响应只返回列表视图必需字段。
- 详情接口 `/api/v1/articles/{id}` 继续返回完整文章内容。
- 前端列表继续保持现有展示，不再依赖列表接口中的摘要、全文等重字段。

## Steps
1. 先写后端 handler 回归测试，验证列表接口不再返回 `summary/display_summary/full_content/ai_summary/summary_debug` 等重字段。
2. 最小实现后端列表 DTO / 裁剪逻辑，保证详情接口不受影响。
3. 跑 Go 相关测试确认 RED -> GREEN。
4. 视需要补前端类型/测试，确保列表与详情接线不因字段缺失出错。
5. 回写 phase task/change 与全局 CHANGE 索引。

## Risks
- 排序与列表 UI 仍依赖 `recommendation_scores`，不能误删。
- 详情打开路径依赖 `getArticle(id)` 返回完整文章，不能破坏。
