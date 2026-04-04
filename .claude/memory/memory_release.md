# Release Memory

## Commit Message Rules (Hard)
- Subject 必须描述具体改动，禁止写 `implement taskNNN` 一类抽象表述。
- Body 必须有三段：背景、关键改动、行为变化。
- Body 必须是真实换行文本，禁止出现字面 `\\n`。
- 组装 commit message 时优先使用多段 `-m` 或 `-F` 文件，避免转义错误。
- 每次提交默认追加 `Co-Authored-By`，作者名需按“当前助手身份”动态填写。
- `Co-Authored-By` 模板：`Co-Authored-By: <CURRENT_ASSISTANT_NAME> <noreply@assistant.local>`。
- 若当前助手为 Codex，则使用：`Co-Authored-By: codex <noreply@openai.com>`（https://github.com/codex）。

## Push Rules
- 改写历史后，推送统一使用 `git push --force-with-lease`。
- 未获用户允许时不推送。

## Plan-First Rule
- 做出任何文件修改前，先在仓库目录输出计划书临时文件，并在修改前先阅读该计划书。

## External Info Persistence
- 发布/收尾阶段若整理到对论文编写有价值的外部信息，必须持久化保存概要与来源地址。
