# Docker Compose 部署设计

## Summary
提供一套面向自部署用户的 Docker Compose 方案：前端静态资源由 Nginx 托管并反向代理 `/api` 到后端容器，外部仅暴露前端端口；后端仅在 Compose 内部网络可访问，并把数据目录挂载到宿主机。

## Goals
- 提供 `docker-compose.yml` + `frontend`/`backend` Dockerfile，支持一条命令启动。
- 外部只暴露前端端口（默认 `80`），后端只在内部网络监听 `:8080`。
- 数据目录可挂载到宿主机（默认 `./data` -> `/data`）。
- Nginx 负责 `/` 静态资源与 `/api` 反向代理。

## Non-Goals
- 不引入 Kubernetes/Helm。
- 不提供多环境复杂编排（只做最小可用 compose）。
- 不在 Docker 内运行 Vite dev server。

## Architecture
- `backend` 容器：Go 二进制，监听 `:8080`，仅 `expose 8080`，不对外发布端口。
- `frontend` 容器：多阶段构建 `frontend/dist`，最终由 Nginx 托管；Nginx 反代 `/api` 到 `backend:8080`。
- `frontend` 对外发布 `80:80`。

## Data & Volumes
- 后端数据目录挂载：
  - 宿主机 `./data` -> 容器 `/data`
  - 通过 `DATA_DIR=/data` 与 `ZFLOW_DB_PATH=/data/zflow.db` 配置。

## Environment Variables
- `backend`：
  - `DATA_DIR=/data`
  - `ZFLOW_DB_PATH=/data/zflow.db`
  - `ZFLOW_ADDR=:8080`
  - `ZFLOW_ALLOWED_ORIGINS` 默认填 `http://localhost,http://127.0.0.1`
    - 用户如果用局域网 IP 访问前端，需要将该变量改成对应的 `http://<lan-ip>`。
- `frontend`：无环境变量依赖，统一由 Nginx 反代 `/api`。

## Nginx Behavior
- `/` -> `frontend/dist`
- `/api/*` -> `http://backend:8080/api/*`
- 设置 `Host` 与 `X-Forwarded-*` 头，保持后端日志与代理行为一致。

## docker-compose.yml Layout (High Level)
- `services.frontend`：build `./frontend`，发布端口 `80:80`。
- `services.backend`：build `./backend`，仅 `expose: 8080`。
- `networks`：默认 bridge，内部互通。
- `volumes`：`./data:/data`。

## Acceptance Criteria
- `docker compose up --build` 后，浏览器访问 `http://localhost` 能加载前端并正常调用 `/api`。
- 后端不对外暴露端口，`localhost:8080` 不直接对外发布（仅内部容器访问）。
- 数据持久化到宿主机 `./data`。
