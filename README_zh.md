**中文** | [English](README.md)

# Zflow

开源、可自托管的 RSS 阅读器，支持 LLM 驱动的内容评分、摘要生成与反信息茧房探索推荐。

默认零遥测。

## 功能特性

- **RSS/Atom 订阅** — 添加源、文件夹分类、拖拽排序
- **三栏阅读器** — 侧栏导航、虚拟滚动文章列表、全文提取文章详情
- **LLM 评分** — 通过可配置 AI 后端对每篇文章给出质量、相关性、新颖性评分
- **AI 翻译** — 流式逐段翻译，原文/译文一键切换
- **AI 摘要** — 分层摘要（RSS 原始 + AI 生成），支持一键刷新
- **向量检索** — 基于 sqlite-vec 的语义相似度搜索与主题聚类
- **知识简报** — 自动生成日报、周报、月报
- **深色/浅色模式** — 跟随系统偏好，完整 CSS 变量主题
- **移动端适配** — 底部导航栏、触控友好尺寸、安全区域支持
- **OPML 导入/导出** — 可从任意 RSS 阅读器迁移
- **键盘无障碍** — 跳转到内容、焦点指示器、ESC 关闭对话框

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go + Echo, SQLite (WAL 模式) via `modernc.org/sqlite` (无 CGO) |
| 前端 | React 18 + TypeScript + Vite |
| 状态管理 | TanStack Query + Zustand |
| UI 组件 | Tailwind CSS + shadcn/ui |
| 路由 | Wouter |
| 向量 | sqlite-vec |
| 部署 | Docker Compose |

## 快速开始

### Docker Compose（推荐）

```bash
git clone https://github.com/Sentixxx/Zflow.git
cd Zflow
docker compose up -d
```

浏览器打开 `http://localhost` 即可使用。

### 本地开发

**后端：**

```bash
cd backend
go build ./cmd/server
./server
# API 地址 http://localhost:8080
```

**前端：**

```bash
cd frontend
npm install
npm run dev
# 开发服务器 http://localhost:5173
```

## 配置

环境变量（统一前缀 `ZFLOW_`，兼容通用变量回退）：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `PORT` / `ZFLOW_ADDR` | `8080` | 后端监听地址 |
| `DATA_DIR` / `ZFLOW_DATA_DIR` | `./data` | SQLite 数据库与数据目录 |
| `ZFLOW_LOG_LEVEL` | `info` | 日志级别：debug, info, warn, error |
| `ZFLOW_LOG_FORMAT` | `text` | 日志格式：text, json |

### AI 设置

在 UI 的设置页面配置：

- **Chat/Translation** — OpenAI 兼容或 Anthropic SDK 端点，用于摘要、翻译、简报生成
- **Embedding** — 独立端点，用于向量检索与主题聚类

API Key 保存后不会以明文返回。

## API

基础地址：`http://localhost:8080/api/v1`

响应格式：`{ error?: string, data?: T }`

时间字段统一使用 RFC3339 UTC。

## 架构

```
backend/
  cmd/server/          # 启动入口
  internal/
    handler/           # HTTP 协议适配层（无 SQL）
    service/           # 业务编排层（无 HTTP 协议细节）
    repository/        # 数据访问层（无业务规则）
    model/             # 领域模型
    db/                # SQLite 初始化与迁移
    scheduler/         # 后台定时抓取（默认 15 分钟）
    feedparser/        # 宽容 RSS/Atom 解析器，支持 Conditional GET
    config/            # 环境变量加载
  pkg/logger/          # 结构化日志

frontend/
  src/
    api/               # API 客户端 + 流式响应处理
    components/        # UI 组件（shadcn/ui + 业务组件）
    hooks/             # TanStack Query + 状态编排
    stores/            # Zustand 全局状态
    lib/               # 纯函数（评分算法、HTML 清洗、国际化）
    pages/             # ReaderPage, SettingsPage, BriefsPage
```

依赖方向：`handler -> service -> repository -> db`

所有依赖通过构造函数注入，禁止全局可变状态。

## 测试

```bash
# 后端
cd backend && go test ./...

# 前端
cd frontend && npm run build   # TypeScript 检查 + Vite 构建
cd frontend && npm run test    # Vitest
```

## 许可证

[MIT](LICENSE)
