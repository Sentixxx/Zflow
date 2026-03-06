# Plan: restore initial list visual style (task126)

## Goal
把列表视觉回退到最初版本：纯扁平连续流，去除当前“奇怪”的卡片化残留观感。

## Scope
1. 新增并完成 task126。
2. 仅修改 frontend/src/styles.css 中列表相关块：
   - item / item.article
   - feed-item / folder-item
   - list 容器与移动端 item 规则
   - 恢复最初的分隔线+左侧active强调样式
3. npm run build 验证
4. 回写 task/change/CHANGE
