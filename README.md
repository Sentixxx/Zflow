[中文](README_zh.md) | **English**

# Zflow

Open-source, self-hosted RSS reader with LLM-powered scoring, summarization, and anti-filter-bubble exploration.

Zero telemetry by default.

## Features

- **RSS/Atom subscriptions** — add feeds, organize into folders, drag-and-drop reorder
- **Three-pane reader** — sidebar navigation, article list with virtual scroll, article detail with full-text extraction
- **LLM scoring** — quality, relevance, and novelty scores for every article via configurable AI backend
- **AI translation** — streaming paragraph-level translation with original/translated toggle
- **AI summarization** — layered summaries (RSS original, AI-generated) with one-click refresh
- **Embedding search** — sqlite-vec powered semantic similarity for topic clustering
- **Knowledge briefs** — auto-generated daily/weekly/monthly digests from your feeds
- **Dark/light mode** — follows system preference, fully themed with CSS variables
- **Mobile responsive** — bottom navigation, touch-friendly targets, safe area support
- **OPML import/export** — migrate from any RSS reader
- **Keyboard accessible** — skip-to-content, focus indicators, ESC to close dialogs

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go + Echo, SQLite (WAL mode) via `modernc.org/sqlite` (no CGO) |
| Frontend | React 18 + TypeScript + Vite |
| State | TanStack Query + Zustand |
| UI | Tailwind CSS + shadcn/ui |
| Routing | Wouter |
| Vectors | sqlite-vec for embeddings |
| Deploy | Docker Compose |

## Quick Start

### Docker Compose (recommended)

```bash
git clone https://github.com/Sentixxx/Zflow.git
cd Zflow
docker compose up -d
```

Open `http://localhost` in your browser.

### Local Development

**Backend:**

```bash
cd backend
go build ./cmd/server
./server
# API at http://localhost:8080
```

**Frontend:**

```bash
cd frontend
npm install
npm run dev
# Dev server at http://localhost:5173
```

## Configuration

Environment variables (prefix `ZFLOW_` or use common fallbacks):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` / `ZFLOW_ADDR` | `8080` | Backend listen address |
| `DATA_DIR` / `ZFLOW_DATA_DIR` | `./data` | SQLite database and data directory |
| `ZFLOW_LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `ZFLOW_LOG_FORMAT` | `text` | Log format: text, json |

### AI Settings

Configure via the Settings page in the UI:

- **Chat/Translation** — OpenAI-compatible or Anthropic SDK endpoint for summaries, translation, briefs
- **Embedding** — separate endpoint for vector search and topic clustering

API keys are never returned in plaintext after saving.

## API

Base URL: `http://localhost:8080/api/v1`

Response envelope: `{ error?: string, data?: T }`

All timestamps in RFC3339 UTC.

## Architecture

```
backend/
  cmd/server/          # Entry point
  internal/
    handler/           # HTTP layer (no SQL)
    service/           # Business logic (no HTTP)
    repository/        # Data access (no business rules)
    model/             # Domain objects
    db/                # SQLite init + migrations
    scheduler/         # Background feed refresh (15-min default)
    feedparser/        # Lenient RSS/Atom parser with Conditional GET
    config/            # Environment variable loading
  pkg/logger/          # Structured logging

frontend/
  src/
    api/               # API client + streaming
    components/        # UI components (shadcn/ui + feature-scoped)
    hooks/             # TanStack Query + state orchestration
    stores/            # Zustand global state
    lib/               # Pure functions (scoring, sanitization, i18n)
    pages/             # ReaderPage, SettingsPage, BriefsPage
```

Dependency direction: `handler -> service -> repository -> db`

All dependencies injected via constructors. No global mutable state.

## Testing

```bash
# Backend
cd backend && go test ./...

# Frontend
cd frontend && npm run build   # TypeScript check + Vite build
cd frontend && npm run test    # Vitest
```

## License

[MIT](LICENSE)
