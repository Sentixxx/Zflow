# Release Memory

## Commit Message Rules (Hard)
- Subject 必须描述具体改动，禁止写 `implement taskNNN` 一类抽象表述。
- Body 必须有三段：背景、关键改动、行为变化。
- Body 必须是真实换行文本，禁止出现字面 `\\n`。
- 组装 commit message 时优先使用多段 `-m` 或 `-F` 文件，避免转义错误。

## Push Rules
- 改写历史后，推送统一使用 `git push --force-with-lease`。
- 未获用户允许时不推送。

