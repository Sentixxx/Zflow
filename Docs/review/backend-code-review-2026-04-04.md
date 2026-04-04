# Backend Code Review — 2026-04-04

## 总览

对 `backend/` 全部代码进行架构合规性、安全性、性能和代码质量审查。

---

## 做得好的地方

- 分层架构整体遵守良好（handler → service → repository → db）
- SQL 全部使用参数化查询，无注入风险
- 依赖注入贯穿始终，无全局可变状态
- 结构化日志一致，使用 `module/action/resource/result` 字段
- 优雅停机处理正确（SIGTERM/SIGINT + 8s shutdown window）
- 输入校验到位：body size limit、URL 校验、proxy scheme 校验、retention 边界检查
- HTTP client 线程安全（`clientMu` RWMutex 保护 proxy 重配置）
- 测试使用 `httptest` 黑盒测试

---

## 严重问题 (Critical)

### 1. API key 明文返回

**文件:** `backend/internal/handler/ai_handlers.go` (GET /api/v1/settings/ai)

GET 请求返回完整 `APIKey` 明文 JSON。任何 XSS 漏洞或浏览器插件都能窃取 key。违反项目规则 "Never log tokens/passwords/API keys"。

**建议:** GET 响应中掩码显示（如 `sk-****abcd`），仅 PATCH 接受完整 key。

### 2. 自定义脚本无沙箱

**文件:** `backend/internal/handler/server.go` (`runScript`, ~line 386-415)

`runScript` 直接执行用户提供的 shell/Python/Node.js 脚本，仅有 12s 超时保护：
- 无文件系统隔离
- 无网络限制
- 无权限降级
- 无资源限制（CPU/内存）
- `stdout.Len() > 1<<20` 检查在脚本执行完毕后才触发，无法限制执行期间内存

**建议:** 至少加安全注释说明风险。理想情况使用 OS 级沙箱（如 Linux `bwrap`）或设置子进程资源限制。

### 3. 文章列表全表加载

**文件:** `backend/internal/service/article_service.go` (List, ~line 73-99)

`ArticleService.List()` 每次请求执行 `SELECT ... FROM entries ORDER BY e.id DESC`，无 LIMIT/WHERE，将所有文章（含 `full_content`、`summary`、`ai_summary`）加载到内存后再过滤分页。

订阅量大时会造成严重的内存压力和延迟。

**建议:** 将过滤（`feed_id`、`folder_id`）和分页（`LIMIT`/`OFFSET`）下推到 SQL。列表查询排除 `full_content` 和 `ai_summary`。

---

## 重要问题 (Important)

### 4. ensureColumn 用 fmt.Sprintf 拼 SQL

**文件:** `backend/internal/repository/sqlite_feed_repository_impl.go` (~line 200-221)

```go
rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
_, err = s.db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, ddl))
```

值全是硬编码常量，非用户输入。但 PRAGMA/ALTER TABLE 不支持 `?` 参数化，属于必要偏离。

**建议:** 加注释说明原因：`// NOTE: table/column names cannot be parameterized in SQLite PRAGMA/ALTER; all values are hardcoded literals, never user input.`

### 5. 无 mock/ 子目录

CLAUDE.md 要求 "Mock files go in mock/ subdirectory of the interface's package"。目前测试使用真实 SQLite 实例，无 `repository/mock/` 包。Service 和 handler 层无法做真正的隔离单元测试。

**建议:** 创建 `backend/internal/repository/mock/` 并实现 `MockFeedRepository`。

### 6. CORS 允许所有来源

**文件:** `backend/internal/handler/server.go` (~line 980)

```go
w.Header().Set("Access-Control-Allow-Origin", "*")
```

任何网页都能调用 Zflow API，结合无认证机制，可读取 feed 数据或修改设置。

**建议:** 限制为前端来源，或通过环境变量配置允许的 origin。

### 7. Server 结构体混入业务逻辑

**文件:** `backend/internal/handler/server.go`

`Server` 结构体直接执行 feed 刷新、icon 下载、feed 解析、脚本执行等。按架构规则，handler 层应仅 "validate params, enforce body size limits, call service"。

涉及方法：`refreshFeedByID`、`fetchAndParse`、`applyScriptToItems`、`tryRefreshFeedIcon`、`RefreshAllFeeds`。

**建议:** 抽取 `FeedService` 处理 feed 刷新编排、icon 管理和脚本执行。

### 8. filterArticlesByScope 循环内重复查库

**文件:** `backend/internal/service/article_service.go` (~line 116-148)

按 `folderID` 过滤时，`ListFolders()` 在循环内每次调用都查库。

**建议:** 循环外调用一次 `ListFolders()`，构建父子映射后遍历。

---

## 建议改进 (Suggestions)

### 9. 正则表达式应预编译

**文件:** `backend/internal/service/article_summary_service.go` (~line 328-340)

`extractSummarySourceBlocks` 内多个 `regexp.MustCompile` 在每次调用时重新编译，应提为包级变量。

### 10. Go 1.21+ 内置 min 函数

**文件:** `backend/internal/service/article_scoring.go` (~line 383-388)

自定义 `min` 函数遮蔽了 Go 1.21+ 内置函数，可直接删除。

### 11. feed 刷新缺少总超时

`fetchAndParse` 和 `discoverIconURLsFromHTML` 各自发 HTTP 请求，单次 feed 刷新可能触发多次请求（icon 候选循环），总耗时可能远超预期。

**建议:** 使用 `context.WithTimeout` 限制单次 feed 刷新总耗时。

### 12. logger 每次从环境变量创建

**文件:** `backend/pkg/logger/logger.go`

`NewModuleFromEnv` 每次调用都读环境变量。虽非 bug，但更好的做法是从 config 层传入 logger。

---

## 架构合规性汇总

| 规则 | 状态 | 备注 |
|------|------|------|
| handler → service → repository → db | 部分通过 | feed 刷新/icon 逻辑在 handler 层 |
| 参数化 SQL，禁止拼接 | 通过 | ensureColumn 用 fmt.Sprintf 但值为硬编码 |
| Mock 放在 mock/ 子目录 | 未通过 | 无 mock 包 |
| httptest 黑盒测试 | 通过 | |
| Schema 变更需迁移路径 | 通过 | ensureColumn 提供前向兼容迁移 |
| 日志字段 snake_case | 通过 | |
| 禁止日志输出 token/密码/API key | 部分通过 | API key 通过 HTTP 响应暴露 |
| 构造函数注入依赖 | 通过 | |
| 无全局可变状态 | 通过 | |

---

## 修复优先级

1. **API key 明文暴露** — 安全，Critical
2. **文章列表全表加载** — 性能，Critical
3. **handler 业务逻辑抽到 service** — 架构，Important
4. **CORS 限制来源** — 安全，Important
5. **其余 Important 和 Suggestion 项**
