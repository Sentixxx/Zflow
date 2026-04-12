# 计划书：gitignore 二进制产物补齐

## 背景
- 根 `.gitignore` 仅忽略 `.DS_Store`、`.tmp` 与 `.worktrees/`。
- `git check-ignore -v backend/server backend/migrate-sqlite-to-pg` 当前无输出，说明后端根目录可执行产物未被忽略。
- `git ls-tree -r --long HEAD` 显示 `backend/server` 与 `backend/migrate-sqlite-to-pg` 曾作为大体积可执行文件被提交。

## 目标
- 补齐仓库根级忽略规则，避免后端本地构建产物再次进入版本控制。
- 保持规则最小且可解释，不影响源码、脚本或真实文档文件。
- 按当前 phase 文档规范补充 task/change/CHANGE 索引。

## 执行步骤
1. 在当前 phase `task_rss_llm_reader.md` 新增一个原子任务，描述“维护者修复二进制产物忽略规则缺口”。
2. 更新根 `.gitignore`，忽略后端根目录常见本地产物：
   - 精确忽略 `backend/server` 与 `backend/migrate-sqlite-to-pg`
   - 补充常见二进制/测试输出后缀，避免同类问题复发
3. 回写 phase `change_rss_llm_reader.md` 与全局 `.phrase/docs/CHANGE.md`。
4. 用 `git check-ignore -v` 验证目标路径已命中忽略规则，并用 `git diff --check` 确认补丁无格式错误。

## 风险与边界
- 只改忽略规则，不直接删除 Git 历史中的二进制对象。
- 当前工作区已有 `backend/server` 与 `backend/migrate-sqlite-to-pg` 的删除记录，属于历史已跟踪文件；本次修复仅防止后续再次误纳入。
