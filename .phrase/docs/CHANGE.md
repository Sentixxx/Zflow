# CHANGE INDEX

## phase-rss-llm-reader-20260225
- change025 日期:2026-02-25 | 文件:.rules | 操作:Add | 影响:项目核心规范 | 说明:新增统一规范入口并覆盖架构/API/提交/分层记忆约束 | 关联:task014
- change026 日期:2026-02-25 | 文件:.phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md | 操作:Modify | 影响:task014 | 说明:新增并完成项目规范落地任务 | 关联:task014
- change022 日期:2026-02-25 | 文件:backend/internal/api/server.go | 操作:Modify | 影响:HTTP中间件 | 说明:新增全局CORS中间件并处理OPTIONS预检请求 | 关联:task013
- change023 日期:2026-02-25 | 文件:backend/internal/api/server_test.go | 操作:Modify | 影响:API测试 | 说明:新增CORS预检与响应头测试覆盖 | 关联:task013
- change024 日期:2026-02-25 | 文件:.phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md | 操作:Modify | 影响:task013 | 说明:新增并完成前端跨域联调任务 | 关联:task013
- change021 日期:2026-02-25 | 文件:frontend/index.html | 操作:Add | 影响:联调界面 | 说明:新增最小前端页用于订阅创建、文章列表/详情和已读切换联调 | 关联:task002
- change014 日期:2026-02-25 | 文件:backend/internal/domain/article.go | 操作:Add | 影响:文章领域模型 | 说明:新增Article结构用于列表/详情与已读状态 | 关联:task002
- change015 日期:2026-02-25 | 文件:backend/internal/store/store.go | 操作:Modify | 影响:持久化与查询接口 | 说明:扩展FeedStore以保存文章并支持详情与已读更新 | 关联:task002
- change016 日期:2026-02-25 | 文件:backend/internal/feedparser/parser.go | 操作:Modify | 影响:抓取解析管线 | 说明:解析结果从计数升级为结构化文章条目 | 关联:task002
- change017 日期:2026-02-25 | 文件:backend/internal/api/server.go | 操作:Modify | 影响:文章API | 说明:新增GET /api/v1/articles、GET /api/v1/articles/{id}、PATCH /api/v1/articles/{id}/read | 关联:task002
- change018 日期:2026-02-25 | 文件:backend/internal/api/server_test.go | 操作:Modify | 影响:API测试 | 说明:新增文章列表/详情/已读状态回归测试 | 关联:task002
- change019 日期:2026-02-25 | 文件:backend/internal/feedparser/parser_test.go | 操作:Modify | 影响:解析测试 | 说明:更新为验证结构化字段解析 | 关联:task002
- change020 日期:2026-02-25 | 文件:.phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md | 操作:Modify | 影响:task002 | 说明:task002标记完成并补充单元测试验证 | 关联:task002
- change006 日期:2026-02-25 | 文件:backend/go.mod | 操作:Add | 影响:Go模块初始化 | 说明:初始化Go后端模块 | 关联:task001
- change007 日期:2026-02-25 | 文件:backend/cmd/server/main.go | 操作:Add | 影响:服务启动入口 | 说明:新增HTTP服务入口与数据目录配置 | 关联:task001
- change008 日期:2026-02-25 | 文件:backend/internal/api/server.go | 操作:Add | 影响:订阅API | 说明:实现POST/GET /api/v1/feeds与健康检查 | 关联:task001
- change009 日期:2026-02-25 | 文件:backend/internal/store/store.go | 操作:Add | 影响:订阅持久化 | 说明:实现JSON文件存储与去重写入 | 关联:task001
- change010 日期:2026-02-25 | 文件:backend/internal/feedparser/parser.go | 操作:Add | 影响:RSS/Atom解析 | 说明:实现首抓解析并提取标题与条目数 | 关联:task001
- change011 日期:2026-02-25 | 文件:backend/internal/api/server_test.go | 操作:Add | 影响:API测试 | 说明:新增创建订阅与查询列表测试 | 关联:task001
- change012 日期:2026-02-25 | 文件:backend/internal/feedparser/parser_test.go | 操作:Add | 影响:解析测试 | 说明:新增RSS与Atom解析单元测试 | 关联:task001
- change013 日期:2026-02-25 | 文件:.phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md | 操作:Modify | 影响:task001 | 说明:task001标记完成并保留验证方式 | 关联:task001
- change002 日期:2026-02-25 | 文件:.phrase/phases/phase-rss-llm-reader-20260225/adr_backend_go_20260225.md | 操作:Add | 影响:后端技术决策 | 说明:新增ADR并锁定后端语言为Go | 关联:task001
- change003 日期:2026-02-25 | 文件:.phrase/phases/phase-rss-llm-reader-20260225/spec_rss_llm_reader.md | 操作:Modify | 影响:Goals/Technical Constraints | 说明:补充后端统一使用Go与API解耦约束 | 关联:task001
- change004 日期:2026-02-25 | 文件:.phrase/phases/phase-rss-llm-reader-20260225/plan_rss_llm_reader.md | 操作:Modify | 影响:Milestones/Priorities | 说明:新增Go后端基础里程碑并提升优先级 | 关联:task001
- change005 日期:2026-02-25 | 文件:.phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md | 操作:Modify | 影响:task001 | 说明:明确task001由Go API服务承载 | 关联:task001
- change001 日期:2026-02-25 | 文件:.phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md | 操作:Add | 影响:task001-task012 | 说明:新增首批任务拆解清单 | 关联:task001
