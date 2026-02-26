# internal

后端业务私有实现目录（禁止被外部模块直接依赖）。

目录约定：

- `handler/`：HTTP 协议适配层（路由、DTO、参数校验、错误映射）
- `router/`：HTTP 路由装配层（路由注册与中间件拼装）
- `service/`：业务编排中间层（跨能力流程复用、用例聚合）
- `repository/`：数据访问层（SQL/CRUD 与存储适配）
- `model/`：业务领域模型（Feed/Folder/Entry 等）
- `db/`：数据库初始化、迁移与连接配置
- `config/`：配置加载与环境变量解析
- `scheduler/`：定时任务调度与后台作业

当前状态：

- 已落地：`handler/`, `router/`, `service/`, `repository/`, `model/`, `db/`, `config/`, `scheduler/`
