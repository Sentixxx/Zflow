# Temp Plan: subscription-scope-switch-fix (2026-04-04)

## Goal
- 把订阅源/分类切换从“全量文章刷新 + 前端本地筛选”改成“作用域查询”。
- 消除 `loadArticles()` 与 `useInfiniteQuery()` 同时写 `articles` 带来的竞争覆盖。

## Steps
1. 先补测试：
   - 后端 `GET /api/v1/articles` 支持 `feed_id` / `folder_id`，并验证分类查询包含子分类订阅。
   - 前端查询参数与 hook 作用域 key 覆盖 feed/folder 切换。
2. 最小实现：
   - 后端 handler 解析 `feed_id` / `folder_id`。
   - service 层在排序前按作用域过滤文章。
   - 前端 query key 和 API 请求带上作用域参数。
   - `selectFeed` / `selectFolder` 不再主动全量 `loadArticles()`，只更新作用域状态。
3. 验证：
   - 跑相关前后端测试。
   - 补 phase task/change/CHANGE 索引。

## Constraints
- 保持现有排序与分页接口行为不变。
- 文件夹筛选继续包含子分类下的订阅源，避免用户行为回退。
