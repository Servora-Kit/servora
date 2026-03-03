## 新增需求

### 需求:数据库连接

系统必须能够根据数据库配置（driver 和 source）连接到数据库。

#### 场景:连接 MySQL

- **当** 数据库配置 driver 为 "mysql"
- **那么** 系统使用 `gorm.io/driver/mysql` 创建 dialector
- **那么** 系统使用 `gorm.Open()` 连接数据库
- **那么** 连接成功时返回 `*gorm.DB` 实例

#### 场景:连接 PostgreSQL

- **当** 数据库配置 driver 为 "postgres" 或 "postgresql"
- **那么** 系统使用 `gorm.io/driver/postgres` 创建 dialector
- **那么** 系统使用 `gorm.Open()` 连接数据库
- **那么** 连接成功时返回 `*gorm.DB` 实例

#### 场景:连接 SQLite

- **当** 数据库配置 driver 为 "sqlite"
- **那么** 系统使用 `github.com/glebarez/sqlite` 创建 dialector
- **那么** 系统使用 `gorm.Open()` 连接数据库
- **那么** 连接成功时返回 `*gorm.DB` 实例

#### 场景:不支持的驱动

- **当** 数据库配置 driver 为不支持的值
- **那么** 系统返回错误 "unsupported db driver: <驱动名>"

#### 场景:连接失败

- **当** 数据库连接失败（如数据库未启动、连接字符串错误）
- **那么** 系统返回错误 "connect db failed: <具体错误>"

### 需求:配置生成器

系统必须使用指定的配置创建 GORM GEN 生成器。

#### 场景:创建生成器

- **当** 系统创建 GORM GEN 生成器
- **那么** OutPath 设置为 `<服务路径>/internal/data/gorm/dao`
- **那么** ModelPkgPath 设置为 `<服务路径>/internal/data/gorm/po`
- **那么** Mode 设置为 `gen.WithDefaultQuery | gen.WithQueryInterface`
- **那么** FieldNullable 设置为 `true`

### 需求:执行生成

系统必须能够执行 GORM GEN 生成，生成所有数据库表的 DAO 和 PO 代码。

#### 场景:生成所有表

- **当** 系统执行生成
- **那么** 系统调用 `generator.UseDB(db)` 设置数据库连接
- **那么** 系统调用 `generator.ApplyBasic(generator.GenerateAllTable()...)` 生成所有表
- **那么** 系统调用 `generator.Execute()` 执行生成
- **那么** DAO 代码生成到指定的 OutPath
- **那么** PO 代码生成到指定的 ModelPkgPath

#### 场景:生成成功输出

- **当** 生成成功完成
- **那么** 系统输出 "✓ Generated GORM code for service '<服务名>'"
- **那么** 系统输出 "  DAO: <DAO 路径>"
- **那么** 系统输出 "  PO: <PO 路径>"

### 需求:预览模式

系统必须支持预览模式，不实际连接数据库和生成文件。

#### 场景:预览模式

- **当** GormGenerator.DryRun 为 true
- **那么** 系统输出 "[DRY-RUN] Would generate to:"
- **那么** 系统输出 "  DAO: <DAO 路径>"
- **那么** 系统输出 "  PO: <PO 路径>"
- **那么** 系统不连接数据库
- **那么** 系统不执行 `generator.Execute()`
- **那么** 系统返回 nil 错误
