# Temp Plan: MCP Push Default Memory (2026-04-04)

## Goal
- 修正长期偏好：未来默认优先使用 GitHub MCP 提交到远端；本次仅临时优先真实 `git push`。

## Scope
- 仅更新自动提交相关 memory。
- 不改其它流程文档。

## Steps
1. 将 `memory_autocommit.md` 中的推送优先级改为“默认 MCP，当前这次例外走 git push”。
2. 保持与现有 auto-commit / release 规则一致。
