# Frontend (React + Vite)

## Dev

```bash
cd frontend
npm install
npm run dev
```

前端包管理器固定为 `npm`，与仓库中的 `package-lock.json` 保持一致。
不要使用 `pnpm`、`yarn` 或依赖 `corepack` 自动推断包管理器，否则在部分 Node/Corepack 组合下可能触发签名 key 校验错误。

默认地址：`http://localhost:5173`

## Build

```bash
npm run build
```

## API

页面内可配置 `API Base URL`，默认 `http://localhost:8080`。
