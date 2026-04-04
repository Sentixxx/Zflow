# Docker Compose Deployment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Dockerfiles, nginx config, and a docker-compose setup so users can run Zflow with a single `docker compose up --build`, exposing only the frontend.

**Architecture:** Build frontend to `dist` and serve it via Nginx; Nginx proxies `/api` to the backend container on the internal network. Backend stores data under `/data` with host volume mount. Only the frontend publishes port `80`.

**Tech Stack:** Docker, docker-compose, Nginx, Go build, Node/Vite build.

---

## File Map

- Create: `backend/Dockerfile`
- Create: `backend/.dockerignore`
- Create: `frontend/Dockerfile`
- Create: `frontend/nginx.conf`
- Create: `frontend/.dockerignore`
- Create: `docker-compose.yml`
- Modify: `Docs/README.md` (if exists) or `README.md` to add run instructions (only if present)
- Modify: `.phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md`
- Modify: `.phrase/phases/phase-rss-llm-reader-20260225/change_rss_llm_reader.md`
- Modify: `.phrase/docs/CHANGE.md`

---

### Task 1: Backend Dockerfile + .dockerignore

**Files:**
- Create: `backend/Dockerfile`
- Create: `backend/.dockerignore`

- [ ] **Step 1: Write failing validation (manual)**

```bash
docker build -t zflow-backend ./backend
```

Expected: FAIL because Dockerfile is missing.

- [ ] **Step 2: Implement Dockerfile**

```Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/zflow ./cmd/server

FROM alpine:3.20
RUN addgroup -S zflow && adduser -S zflow -G zflow
WORKDIR /app
COPY --from=builder /out/zflow /app/zflow
EXPOSE 8080
ENV ZFLOW_ADDR=:8080
ENV DATA_DIR=/data
ENV ZFLOW_DB_PATH=/data/zflow.db
USER zflow
ENTRYPOINT ["/app/zflow"]
```

- [ ] **Step 3: Implement .dockerignore**

```gitignore
data
dist
node_modules
.git
.gitignore
.DS_Store
```

- [ ] **Step 4: Re-run validation**

```bash
docker build -t zflow-backend ./backend
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/Dockerfile backend/.dockerignore
git commit -m "feat: add backend docker image build"
```

---

### Task 2: Frontend Dockerfile + Nginx config + .dockerignore

**Files:**
- Create: `frontend/Dockerfile`
- Create: `frontend/nginx.conf`
- Create: `frontend/.dockerignore`

- [ ] **Step 1: Write failing validation (manual)**

```bash
docker build -t zflow-frontend ./frontend
```

Expected: FAIL because Dockerfile/nginx.conf is missing.

- [ ] **Step 2: Implement Nginx config**

```nginx
server {
  listen 80;
  server_name _;

  root /usr/share/nginx/html;
  index index.html;

  location /api/ {
    proxy_pass http://backend:8080/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
  }

  location / {
    try_files $uri /index.html;
  }
}
```

- [ ] **Step 3: Implement Dockerfile**

```Dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json package-lock.json* pnpm-lock.yaml* ./
RUN if [ -f package-lock.json ]; then npm ci; \
    elif [ -f pnpm-lock.yaml ]; then npm install -g pnpm && pnpm install --frozen-lockfile; \
    else npm install; fi
COPY . .
RUN npm run build

FROM nginx:1.27-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
```

- [ ] **Step 4: Implement .dockerignore**

```gitignore
node_modules
dist
.git
.gitignore
.DS_Store
```

- [ ] **Step 5: Re-run validation**

```bash
docker build -t zflow-frontend ./frontend
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/Dockerfile frontend/nginx.conf frontend/.dockerignore
git commit -m "feat: add frontend nginx docker image"
```

---

### Task 3: docker-compose.yml

**Files:**
- Create: `docker-compose.yml`

- [ ] **Step 1: Write failing validation (manual)**

```bash
docker compose -f docker-compose.yml config
```

Expected: FAIL because file missing.

- [ ] **Step 2: Implement docker-compose.yml**

```yaml
services:
  backend:
    build:
      context: ./backend
    environment:
      DATA_DIR: /data
      ZFLOW_DB_PATH: /data/zflow.db
      ZFLOW_ADDR: :8080
      ZFLOW_ALLOWED_ORIGINS: http://localhost,http://127.0.0.1
    volumes:
      - ./data:/data
    expose:
      - "8080"

  frontend:
    build:
      context: ./frontend
    ports:
      - "80:80"
    depends_on:
      - backend
```

- [ ] **Step 3: Re-run validation**

```bash
docker compose -f docker-compose.yml config
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml
git commit -m "feat: add docker compose deployment"
```

---

### Task 4: Docs + Phase Records

**Files:**
- Modify: `.phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md`
- Modify: `.phrase/phases/phase-rss-llm-reader-20260225/change_rss_llm_reader.md`
- Modify: `.phrase/docs/CHANGE.md`
- Modify: `README.md` or `Docs/README.md` (only if present)

- [ ] **Step 1: Add taskNNN and changeNNN entries**

Add a new `taskNNN` for Docker deployment with validation:  
`验证:docker compose build + docker compose up + curl http://localhost/healthz`.

- [ ] **Step 2: Update README (if present)**

Add a “Docker Compose” section with:
- `docker compose up --build`
- Data volume `./data`
- `ZFLOW_ALLOWED_ORIGINS` note for LAN access

- [ ] **Step 3: Commit**

```bash
git add .phrase/phases/phase-rss-llm-reader-20260225/task_rss_llm_reader.md \
  .phrase/phases/phase-rss-llm-reader-20260225/change_rss_llm_reader.md \
  .phrase/docs/CHANGE.md \
  README.md Docs/README.md
git commit -m "docs: document docker compose deployment"
```

---

## Plan Self-Review

- Spec coverage: Dockerfiles, nginx config, compose, env/volumes, docs updates covered by Tasks 1–4.
- No placeholders: all steps include concrete code blocks and commands.
- Consistency: Nginx proxies `/api` to `backend:8080`, backend listens on `:8080`, data at `/data`.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-04-docker-compose-deployment.md`. Two execution options:

1. Subagent-Driven (recommended) — I dispatch a fresh subagent per task, review between tasks, fast iteration  
2. Inline Execution — Execute tasks in this session using executing-plans, batch execution with checkpoints  

Which approach?
