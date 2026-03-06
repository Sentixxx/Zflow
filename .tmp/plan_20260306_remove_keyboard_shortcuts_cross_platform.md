# Plan 2026-03-06 Remove Keyboard Shortcuts for Cross-Platform

## Goal
移除主界面键盘快捷键依赖，确保跨平台体验一致，不依赖物理键盘输入。

## Steps
1. 删除 `ReaderPage.tsx` 中 `isEditableTarget` 与全局 keydown 监听逻辑。
2. 调整详情位置提示文案，去掉“快捷键 J/K”提示，只保留第 N / M 条。
3. 运行 `npm run build`。
4. 回写 task/change 与 `.phrase/docs/CHANGE.md`。
