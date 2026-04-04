# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Zflow is an open-source, self-hosted RSS reader with LLM-powered scoring, summarization, and recommendation features. Zero telemetry by default.

- **Backend:** Go + Echo, SQLite via `modernc.org/sqlite` (no CGO)
- **Frontend:** React 18 + TypeScript + Vite, TanStack Query, Zustand, Wouter
- **Package manager:** npm (enforced — do not use pnpm or yarn)

---

## Commands

### Backend
```bash
cd backend
go build ./cmd/server        # Build binary
go test ./...                # Run all tests (required before commit)
go test -v ./internal/handler -run TestFooBar  # Run single test
```

### Frontend
```bash
cd frontend
npm install                  # Install dependencies
npm run dev                  # Dev server at localhost:5173
npm run build                # TypeScript check + Vite build (required before commit)
npm run test                 # Run Vitest once
npm run test:watch           # Watch mode
npm run test -- foo.test.ts  # Run a single test file
```

### Acceptance baseline
- Backend changes: `go test ./...` must pass
- Frontend changes: `npm run build` must pass; also run `npm run test` for modules with existing Vitest coverage

---

## Architecture

### Backend Layers (strict dependency direction)

```
handler → service → repository → db
```

- **`handler/`** — HTTP protocol adapter. Validates params, enforces body size limits, calls service. No SQL.
- **`service/`** — Business logic orchestration. No HTTP protocol details.
- **`repository/`** — SQL execution only. No business rules.
- **`model/`** — Shared domain objects used across service/repository.
- **`db/`** — SQLite init and schema migrations (WAL mode, foreign keys enabled).
- **`scheduler/`** — Background feed refresh (default 15-minute interval).
- **`feedparser/`** — Lenient RSS/Atom parser with Conditional GET support (ETag/If-Modified-Since).
- **`config/`** — Env var loading (prefix `ZFLOW_`, with fallbacks to `PORT`, `DATA_DIR`, etc.).
- **`pkg/logger/`** — Structured logging. Never use `fmt.Print`/`log.Printf` in business code.

All dependencies are injected via constructors. No global mutable state.

### Frontend Layers

```
ReaderPage (orchestrator)
 └── useReaderQueries (TanStack Query setup — feeds, folders, articles infinite query)
 └── useFeeds / useEntries (data fetch + transform hooks)
 └── useArticleActions (mark read/favorite, translate, summarize)
 └── useReaderStore (Zustand) — selectedFeedID, selectedFolderID, readFilter, sortMode
 └── Components (presentational only)
```

**Key directories:**
- `src/api/` — API client + type definitions + streaming response handling
- `src/components/ui/` — Generic UI components (shadcn/ui)
- `src/components/{feature}/` — Feature-scoped components (sidebar, article-detail, settings)
- `src/hooks/` — State-orchestration hooks
- `src/lib/` — Pure functions (scoring algorithms, date formatting, routing, HTML sanitization)
- `src/services/` — Complex async flows (batch translation, request dedup/cancel)
- `src/stores/` — Zustand stores
- `src/types/` — DTO and domain type definitions
- `src/i18n/` — Internationalization config

Use `@/` path alias for all imports (e.g. `@/components/...`).

### API contract
- Base: `http://localhost:8080/api/v1`
- Response envelope: `{ error?: string, data?: T }`
- All timestamps: RFC3339 UTC
- Frontend API base URL stored in `localStorage` key `zflow_api_base`

---

## Key Rules from `.rules`

### Backend
- Mock files go in `mock/` subdirectory of the interface's package — no centralized `testutil/mock_*`.
- Prefer black-box HTTP tests via `httptest`.
- No string-concatenated SQL; use parameterized queries throughout.
- Schema changes require a migration path (no breaking changes to existing data).
- Log fields: `module`, `action`, `resource`, `result` in `snake_case`. Never log tokens/passwords/API keys.

### Frontend
- No `any` types — all props and API responses must be explicitly typed.
- Long lists must use pagination, infinite scroll, or virtualization (no full synchronous render).
- Expensive computation must use `useMemo` or pure function caching.
- `dangerouslySetInnerHTML` only with DOMPurify-sanitized content.
- External links default to `rel="noreferrer"`.
- TanStack Query: `staleTime: 30s`, `refetchInterval: 60s`; use `useInfiniteQuery` + `getNextPageParam` for pagination; detect `hasMore` via `limit+1` pattern.
- Global state (Zustand) only for truly shared data; local interaction state stays in components.
- Use `src/lib/logger.ts` for all frontend logging — no scattered `console.*` in page components.
- CSS: use semantic color variables for theming; support `light/dark` following `prefers-color-scheme`.

### Interface consistency
- Any backend DTO change must be reflected in frontend `types/` and `api/` call layer.
- If an ID exceeds JS safe integer range, backend must return it as a string.

### Git commits
- Conventional Commits: `feat/fix/docs/test/chore`
- Subject must describe the actual change (not "implement taskXXX").
- Body must include: background, key changes, behavior change.
- Append `Co-Authored-By: <agent name> <noreply@assistant.local>`.

---

## MCP GitHub Push vs Local Git

**Never use `mcp__github__push_files` to push code changes when working in a git worktree.**

Using both MCP push and local `git commit` creates two parallel commit histories with different SHAs, even if the file content is identical. The branches will diverge and cannot be fast-forward merged.

**Rules:**
- File changes: always use local Edit/Write + `git commit` + `git push`
- If `git push` fails due to auth: fix the auth (switch remote to SSH, run `gh auth setup-git`) — do not work around it with MCP
- MCP GitHub tools are only for: creating branches, creating PRs, reading PR/issue data

**If divergence already happened:** fix with `git push --force-with-lease` to make remote match local (local is authoritative).

---

## Document-Driven Workflow

All implementation is traceable to a task in `.phrase/phases/`. On completion, update:
- `task_*.md` in the relevant phase
- `change_*.md` in the relevant phase
- `.phrase/docs/CHANGE.md`

Non-reversible decisions must update the ADR.

## Plan Files

All AI-generated working plans (Codex, Claude, etc.) go in `Docs/plans/`. Do not drop plan files in the repo root or in tool-specific directories (`.claude/`, `.codex/`).
