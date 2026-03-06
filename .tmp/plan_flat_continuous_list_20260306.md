# Plan: continuous flat list style (task125)

## Goal
将列表从分离卡片改回连续扁平流：卡片之间无缝衔接，形成整页连贯阅读感。

## Scope
1. 新增并完成 task125。
2. 修改 frontend/src/styles.css：
   - 去除列表项之间间隙、圆角和独立卡片边框
   - 恢复以底部分隔线为主的连续样式
   - active/hover 保留但不产生“卡片浮起”效果
3. npm run build
4. 回写 task/change/CHANGE
