# router

HTTP 路由装配层：

- 负责创建 `ServeMux` 并组装路由。
- 通过接口解耦具体 handler 实现。
- 统一在此层拼接中间件包装后的顶层 `http.Handler`。
