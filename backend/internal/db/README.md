# db

数据库初始化与迁移目录。

职责：

- 建立 SQLite 连接与 pragma 配置（WAL/foreign_keys/busy_timeout）
- 管理 schema 迁移与兼容升级
- 提供可测试的数据库初始化入口
