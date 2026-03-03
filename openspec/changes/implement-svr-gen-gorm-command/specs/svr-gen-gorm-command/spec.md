## 新增需求

### 需求:命令行接口

系统必须提供 `svr gen gorm <服务名...>` 命令，用于为一个或多个指定服务生成 GORM DAO 和 PO 代码。

#### 场景:单个服务生成

- **当** 用户执行 `svr gen gorm servora`
- **那么** 系统读取 `app/servora/service/configs/config.yaml` 配置
- **那么** 系统连接配置中指定的数据库
- **那么** 系统生成 DAO 代码到 `app/servora/service/internal/data/gorm/dao`
- **那么** 系统生成 PO 代码到 `app/servora/service/internal/data/gorm/po`
- **那么** 系统输出成功消息，包含生成路径

#### 场景:多个服务批量生成

- **当** 用户执行 `svr gen gorm servora sayhello`
- **那么** 系统依次为 servora 和 sayhello 服务生成代码
- **那么** 每个服务生成成功后输出独立的成功消息
- **那么** 如果某个服务生成失败，记录错误但继续处理其他服务
- **那么** 最终输出总结信息（成功 X 个，失败 Y 个）
- **那么** 失败列表逐条展示（服务名 + 对应错误信息）
- **那么** 若存在失败，进程退出码必须为 1
- **那么** 若全部成功，进程退出码必须为 0

#### 场景:无参数进入交互

- **当** 用户执行 `svr gen gorm` 且未传入服务名
- **那么** 系统进入交互式服务选择流程
- **那么** 系统展示 `app/*/service` 的可选服务列表并支持多选
- **那么** 系统展示确认步骤并在用户确认后执行生成
- **那么** 用户取消时系统不执行生成并正常退出

#### 场景:服务不存在

- **当** 用户执行 `svr gen gorm nonexistent`
- **那么** 系统返回错误 "service 'nonexistent' not found at app/nonexistent/service"
- **那么** 系统列出可用的服务列表

#### 场景:配置文件不存在

- **当** 用户执行 `svr gen gorm servora` 但配置文件不存在
- **那么** 系统返回错误 "config file not found at app/servora/service/configs/config.yaml"
- **那么** 系统提示用户确保服务有有效的 config.yaml 文件

#### 场景:数据库配置缺失

- **当** 用户执行 `svr gen gorm servora` 但配置中没有 data.database 配置
- **那么** 系统返回错误 "no database config found in app/servora/service/configs/config.yaml"
- **那么** 系统提供示例配置说明如何添加数据库配置

#### 场景:数据库连接失败

- **当** 用户执行 `svr gen gorm servora` 但数据库无法连接
- **那么** 系统返回错误 "connect db failed: <具体错误>"
- **那么** 系统提供检查清单（数据库是否运行、连接字符串是否正确、网络连接）

### 需求:预览模式

系统必须支持 `--dry-run` 标志，允许用户预览生成路径而不实际执行生成。

#### 场景:预览生成路径

- **当** 用户执行 `svr gen gorm servora --dry-run`
- **那么** 系统输出 "[DRY-RUN] Would generate to:"
- **那么** 系统显示 DAO 输出路径
- **那么** 系统显示 PO 输出路径
- **那么** 系统不连接数据库
- **那么** 系统不生成任何文件

### 需求:数据库支持

系统必须支持 MySQL、PostgreSQL 和 SQLite 三种数据库驱动。

#### 场景:MySQL 数据库

- **当** 配置中 database.driver 为 "mysql"
- **那么** 系统使用 MySQL 驱动连接数据库

#### 场景:PostgreSQL 数据库

- **当** 配置中 database.driver 为 "postgres" 或 "postgresql"
- **那么** 系统使用 PostgreSQL 驱动连接数据库

#### 场景:SQLite 数据库

- **当** 配置中 database.driver 为 "sqlite"
- **那么** 系统使用 SQLite 驱动连接数据库

#### 场景:不支持的数据库驱动

- **当** 配置中 database.driver 为不支持的值
- **那么** 系统返回错误 "unsupported db driver: <驱动名>"

### 需求:生成配置

系统必须使用以下 GORM GEN 配置生成代码：
- Mode: `gen.WithDefaultQuery | gen.WithQueryInterface`
- FieldNullable: `true`
- OutPath: `<服务路径>/internal/data/gorm/dao`
- ModelPkgPath: `<服务路径>/internal/data/gorm/po`

#### 场景:生成配置应用

- **当** 系统执行生成
- **那么** 生成的 DAO 包含默认 Query 对象
- **那么** 生成的 DAO 包含 Query 接口
- **那么** 生成的 PO 字段支持 nullable（如 delete_at）
