# Temp Plan: Auto Commit Memory (2026-04-04)

## Goal
- 新增一条长期记忆：当完成代码编辑并判断已形成合适提交边界时，默认自行提交并推送到 Git。

## Scope
- 新增独立 memory 文件记录该规则。
- 更新 `MEMORY_INDEX.md`，让 `coding` 与 `release` 阶段都加载这条记忆。
- 不修改与该规则无关的流程文档。

## Steps
1. 新增 `memory_autocommit.md`，明确自动 commit/push 的触发条件与边界。
2. 更新 `MEMORY_INDEX.md` 阶段映射，确保 coding/release 阶段会加载该 memory。
3. 检查当前工作区状态，避免误把无关改动混入后续提交。

## Risks
- 自动推送只应在形成清晰提交边界时触发，不能把未完成或混杂改动一起推送。
- 如涉及改写历史，仍需遵守 `force-with-lease` 规则。
