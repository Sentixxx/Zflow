# Temp Plan: Background Score Refresh (2026-04-04)

## Goal
- 将文章评分缺失或版本过期的补写，从文章列表读路径迁移到后台周期任务。

## Scope
- `GET /api/v1/articles` 与 `GET /api/v1/articles/{id}` 不再同步触发评分补写。
- 新增后台周期刷新入口，按批次扫描并补写 `article_features` 缺失或 `feature_version` 落后的文章。
- 保持显式内容变更路径（如正文刷新）可即时重算对应文章评分。

## Steps
1. 先写服务层失败测试，验证列表读取不会再同步补写评分。
2. 新增仓储查询，按批获取缺失或旧版本评分的文章。
3. 新增 `ArticleService` 后台刷新方法，并让列表/详情改为纯读。
4. 新增后台 scheduler，在服务启动后独立 goroutine 周期执行批量评分刷新。
5. 跑 Go 定向测试，再用 curl 复测文章列表接口耗时。
6. 回写 phase 文档与变更索引。

## Risks
- 后台刷新批次必须限流，避免继续抢占 SQLite 写锁。
- 排序和列表展示在评分缺失时要能平稳回退，不能因短暂空值崩掉。
