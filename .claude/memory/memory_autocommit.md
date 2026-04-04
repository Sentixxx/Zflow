# Auto Commit Memory

- 当一次代码编辑已经形成清晰、可验证、可独立说明的提交边界时，不等待额外提醒，直接执行 `git commit`。
- 完成提交后，若用户未明确禁止推送，则默认继续执行 `git push`；如历史被改写，则使用 `git push --force-with-lease`。
- 自动提交前必须先确认本次提交不会混入无关改动；若工作区存在 unrelated changes，需要只提交当前任务相关文件，或先停下来说明风险。
- 提交信息继续遵守 `.claude/commit_message_policy.md` 与 `memory_release.md`：Conventional Commits、三段式正文、真实换行、追加 `Co-Authored-By`。
