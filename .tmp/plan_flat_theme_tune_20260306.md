# Plan: flatten style with theme accents (task121)

## Goal
按用户偏好回到扁平质感：去掉明显纹理/厚阴影/强实体感，保留明确主题色表达。

## Scope
1. 新增并完成 task121。
2. 修改 frontend/src/styles.css：
   - 移除 body 纹理层与厚阴影
   - 面板/详情卡/弹窗改为轻边框+轻阴影或无阴影
   - 保留主题色在 active/hover/链接上的点缀
3. npm run build 验证。
4. 回写 change 与全局 CHANGE。
