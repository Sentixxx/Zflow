# Plan: mobile nav usability refine (task124)

## Goal
在已有移动端底部导航基础上继续优化可用性，减少无效点击与空白状态。

## Scope
1. 新增并完成 task124。
2. ReaderPage.tsx:
   - 详情tab在无选中文章时禁用
   - 窄屏下若处于detail但文章为空，自动回到list
   - 底部导航显示简洁计数（导航源数量/列表条数）
3. styles.css:
   - 底部导航按钮禁用态与计数徽标样式
4. npm run build
5. 回写 task/change/CHANGE
