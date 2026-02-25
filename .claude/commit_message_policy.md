# Commit Message Policy

## Required Format
- Subject 使用 Conventional Commits（如 `feat:`, `fix:`, `docs:`, `test:`, `chore:`）。
- Subject 必须描述“具体改动内容”，禁止写 `implement taskNNN` 这种任务号式标题。
- Body 必须说明：
  - 问题背景/动机（为什么改）。
  - 关键改动点（改了什么）。
  - 行为变化与影响面（结果是什么）。
- 每次提交默认在正文末尾追加 `Co-Authored-By`，并按当前助手身份动态填写。
- 推荐模板：`Co-Authored-By: <CURRENT_ASSISTANT_NAME> <noreply@assistant.local>`。

## Example
```
fix: Anubis cookie 复用时 UA 与 policyRule hash 不匹配

Anubis 服务端在验证 JWT cookie 时会检查 policyRule hash 是否
与当前请求匹配的策略规则一致。Solver 提交解题时使用 Chrome UA，
但后续请求（定时刷新、预览、图标下载）使用 Gist UA，导致
policyRule hash 不匹配，cookie 被拒绝，每次都重新解题。

修改为：当 host 存在缓存的 Anubis cookie 时，该 host 的所有
请求统一使用 Chrome UA，确保与 solver 提交时的策略规则一致。
无 Anubis cookie 的 host 仍使用 Gist UA 诚实标识。
```
