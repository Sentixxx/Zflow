# Memory Index

## Purpose
- 将长期规则按阶段拆分，避免每次加载全部记忆导致 token 浪费。

## Loading Rule
- 每次只加载当前阶段所需记忆文件。
- 阶段切换时再切换记忆，不跨阶段全量读取。

## Stage Mappings
- `init`（立项/PRFAQ）: `memory_init.md`
- `coding`（实现/重构/修复/评审）: `memory_coding.md`
- `docs`（文档/文案）: `memory_docs.md`
- `release`（提交/推送/发布）: `memory_release.md`

