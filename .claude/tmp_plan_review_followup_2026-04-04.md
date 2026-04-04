# 临时计划：继续落实 backend review 剩余可闭环项

1. 为 feed 刷新链路补“单次刷新总超时”约束，避免多次 icon / feed 请求串联时无限拖长。
2. 为自定义脚本执行补明确的安全边界注释与执行期输出限制，至少把风险说清并降低内存失控概率。
3. 新建 `backend/internal/repository/mock/` 的 `MockFeedRepository` 基线，满足仓库规范并为后续 service/handler 隔离测试铺路。
4. 增加对应测试或编译验证，确保这批 follow-up 不引入回归。
5. 回写 task/change/CHANGE，形成独立提交边界后自动 commit + push。
