# CORS Same-Host LAN Fix Plan

- 目标: 修复前端通过局域网 host 访问时，后端因严格 allowlist 不返回 `Access-Control-Allow-Origin` 导致浏览器请求被拦截的问题。
- 根因: 当前默认 CORS 仅允许 `localhost/127.0.0.1`，但前端默认会在非 localhost 情况下请求 `http://<当前host>:8080`；同 host 的局域网 origin 未被放行。
- 方案:
  - 先补 handler 失败测试，覆盖 `Origin=http://192.168.x.x:5173` 且请求 Host 为同一 IP 时应放行。
  - 再将 `allowOrigin` 改为：显式 allowlist 命中时放行；否则若 origin host 与当前请求 host 相同，则放行。
  - 保持跨 host 的恶意 origin 仍被拦截，不回退成 `*`。
- 验证:
  - `go test ./internal/handler -run 'TestCORSPreflightAndHeaders|TestCORSAllowsConfiguredOrigins|TestCORSAllowsSameHostLanOrigin'`
  - `curl -H 'Origin: http://192.168.1.9:5173' http://127.0.0.1:8080/healthz` 应返回 `Access-Control-Allow-Origin`
