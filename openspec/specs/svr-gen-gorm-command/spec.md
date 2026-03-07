## Purpose
定义 svr-gen-gorm-command 的功能需求和验证场景。

## Requirements

### Requirement: 命令行接口

系统必须提供 `svr gen gorm <服务名...>` 命令，用于为一个或多个指定服务生成 GORM DAO 和 PO 代码。

#### Scenario: 单个服务生成

- **WHEN** 用户执行 `svr gen gorm servora`
- **THEN** 系统读取 `app/servora/service/configs/config.yaml` 配置
- **THEN** 系统连接配置中指定的数据库
- **THEN** 系统生成 DAO 代码到 `app/servora/service/internal/data/gorm/dao`
- **THEN** 系统生成 PO 代码到 `app/servora/service/internal/data/gorm/po`
- **THEN** 系统输出成功消息，包含生成路径

#### Scenario: 多个服务批量生成

- **WHEN** 用户执行 `svr gen gorm servora sayhello`
- **THEN** 系统依次为 servora 和 sayhello 服务生成代码
- **THEN** 每个服务生成成功后输出独立的成功消息
- **THEN** 如果某个服务生成失败，记录错误但继续处理其他服务
- **THEN** 最终输出总结信息（成功 X 个，失败 Y 个）
- **THEN** 失败列表逐条展示（服务名 + 对应错误信息）
- **THEN** 若存在失败，进程退出码必须为 1
- **THEN** 若全部成功，进程退出码必须为 0

#### Scenario: 无参数进入交互

- **WHEN** 用户执行 `svr gen gorm` 且未传入服务名
- **THEN** 系统进入交互式服务选择流程
- **THEN** 系统展示 `app/*/service` 的可选服务列表并支持多选
- **THEN** 系统展示确认步骤并在用户确认后执行生成
- **THEN** 用户取消时系统不执行生成并正常退出

#### Scenario: 服务不存在

- **WHEN** 用户执行 `svr gen gorm nonexistent`
- **THEN** 系统返回错误 "service 'nonexistent' not found at app/nonexistent/service"
- **THEN** 系统列出可用的服务列表

#### Scenario: 配置文件不存在

- **WHEN** 用户执行 `svr gen gorm servora` 但配置文件不存在
- **THEN** 系统返回错误 "config file not found at app/servora/service/configs/config.yaml"
- **THEN** 系统提示用户确保服务有有效的 config.yaml 文件

#### Scenario: 数据库配置缺失

- **WHEN** 用户执行 `svr gen gorm servora` 但配置中没有 data.database 配置
- **THEN** 系统返回错误 "no database config found in app/servora/service/configs/config.yaml"
- **THEN** 系统提供示例配置说明如何添加数据库配置

#### Scenario: 数据库连接失败

- **WHEN** 用户执行 `svr gen gorm servora` 但数据库无法连接
- **THEN** 系统返回错误 "connect db failed: <具体错误>"
- **THEN** 系统提供检查清单（数据库是否运行、连接字符串是否正确、网络连接）

### Requirement: 预览模式

系统必须支持 `--dry-run` 标志，允许用户预览生成路径而不实际执行生成。

#### Scenario: 预览生成路径

- **WHEN** 用户执行 `svr gen gorm servora --dry-run`
- **THEN** 系统输出 "[DRY-RUN] Would generate to:"
- **THEN** 系统显示 DAO 输出路径
- **THEN** 系统显示 PO 输出路径
- **THEN** 系统不连接数据库
- **THEN** 系统不生成任何文件

### Requirement: 数据库支持

系统必须支持 MySQL、PostgreSQL 和 SQLite 三种数据库驱动。

#### Scenario: MySQL 数据库

- **WHEN** 配置中 database.driver 为 "mysql"
- **THEN** 系统使用 MySQL 驱动连接数据库

#### Scenario: PostgreSQL 数据库

- **WHEN** 配置中 database.driver 为 "postgres" 或 "postgresql"
- **THEN** 系统使用 PostgreSQL 驱动连接数据库

#### Scenario: SQLite 数据库

- **WHEN** 配置中 database.driver 为 "sqlite"
- **THEN** 系统使用 SQLite 驱动连接数据库

#### Scenario: 不支持的数据库驱动

- **WHEN** 配置中 database.driver 为不支持的值
- **THEN** 系统返回错误 "unsupported db driver: <驱动名>"

### Requirement: 生成配置

系统必须使用以下 GORM GEN 配置生成代码：
- Mode: `gen.WithDefaultQuery | gen.WithQueryInterface`
- FieldNullable: `true`
- OutPath: `<服务路径>/internal/data/gorm/dao`
- ModelPkgPath: `<服务路径>/internal/data/gorm/po`

#### Scenario: 生成配置应用

- **WHEN** 系统执行生成
- **THEN** 生成的 DAO 包含默认 Query 对象
- **THEN** 生成的 DAO 包含 Query 接口
- **THEN** 生成的 PO 字段支持 nullable（如 delete_at）
