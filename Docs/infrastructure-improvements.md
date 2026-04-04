# 基建改进建议

记录于 2026-04-04，基于前后端代码 review 结果。

---

## 高优先级

### 1. 数据库迁移管理
当前用 `ensureColumn` 做前向兼容迁移，是 ad-hoc 方案。随着 schema 越来越复杂，维护会越来越困难。
**建议:** 引入版本化迁移（如 `golang-migrate` 或手写版本号表），支持回滚和状态追踪。

### 2. 文章列表查询性能
`ArticleService.List()` 全表加载后在内存过滤分页，订阅量增长后首先暴露。
**建议:** 将 feed_id/folder_id 过滤和 LIMIT/OFFSET 下推到 SQL，列表查询排除 full_content/ai_summary 字段。
**参考:** `Docs/review/backend-code-review-2026-04-04.md` Critical #3

### 3. API 认证
目前完全无认证，依赖网络隔离。暴露在公网时无保护。
**建议:** 至少支持 Bearer token 或 HTTP Basic Auth，覆盖公网自部署场景。

---

## 中优先级

### 4. Docker 化部署
无 `Dockerfile` 或 `docker-compose.yml`。自部署工具核心用户群依赖 Docker，缺失会提高使用门槛。
**建议:** 前后端 + 数据目录挂载打包成单镜像。

### 5. CI/CD 缺少 release 流程
CI 只有 test + build，无自动打包发布。用户使用新版本需自行编译。
**建议:** 添加 GitHub Release workflow，发布前后端二进制和 Docker 镜像。

### 6. 前端 Error Boundary
任何渲染错误会导致白屏无提示。
**建议:** 在应用根节点包裹 React Error Boundary，提供友好的错误恢复 UI。
**参考:** `Docs/review/frontend-code-review-2026-04-04.md` Suggestion #12

---

## 低优先级

### 7. 测试覆盖率
- 后端 repository 层几乎无单元测试，依赖集成测试
- 前端组件层无测试，部分 hook/lib 有覆盖

### 8. 健康检查端点
无 `/healthz` 端点，Docker/K8s 部署无法做存活检测。

### 9. 配置热重载
日志级别、调度间隔等需重启生效，对自部署工具稍显不便。

---

## 建议行动顺序

1. 文章列表 SQL 下推（核心功能性能，影响日常使用）
2. Docker 化（降低用户获取门槛）
3. API 认证（公网安全基线）
4. 版本化数据库迁移（长期可维护性）
5. CI/CD release 流程
6. Error Boundary + 健康检查端点
