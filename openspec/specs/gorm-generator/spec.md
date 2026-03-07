## Purpose
定义 gorm-generator 的功能需求和验证场景。

## Requirements

### Requirement: 数据库连接

系统必须能够根据数据库配置（driver 和 source）连接到数据库。

#### Scenario: 连接 MySQL

- **WHEN** 数据库配置 driver 为 "mysql"
- **THEN** 系统使用 `gorm.io/driver/mysql` 创建 dialector
- **THEN** 系统使用 `gorm.Open()` 连接数据库
- **THEN** 连接成功时返回 `*gorm.DB` 实例

#### Scenario: 连接 PostgreSQL

- **WHEN** 数据库配置 driver 为 "postgres" 或 "postgresql"
- **THEN** 系统使用 `gorm.io/driver/postgres` 创建 dialector
- **THEN** 系统使用 `gorm.Open()` 连接数据库
- **THEN** 连接成功时返回 `*gorm.DB` 实例

#### Scenario: 连接 SQLite

- **WHEN** 数据库配置 driver 为 "sqlite"
- **THEN** 系统使用 `github.com/glebarez/sqlite` 创建 dialector
- **THEN** 系统使用 `gorm.Open()` 连接数据库
- **THEN** 连接成功时返回 `*gorm.DB` 实例

#### Scenario: 不支持的驱动

- **WHEN** 数据库配置 driver 为不支持的值
- **THEN** 系统返回错误 "unsupported db driver: <驱动名>"

#### Scenario: 连接失败

- **WHEN** 数据库连接失败（如数据库未启动、连接字符串错误）
- **THEN** 系统返回错误 "connect db failed: <具体错误>"

### Requirement: 配置生成器

系统必须使用指定的配置创建 GORM GEN 生成器。

#### Scenario: 创建生成器

- **WHEN** 系统创建 GORM GEN 生成器
- **THEN** OutPath 设置为 `<服务路径>/internal/data/gorm/dao`
- **THEN** ModelPkgPath 设置为 `<服务路径>/internal/data/gorm/po`
- **THEN** Mode 设置为 `gen.WithDefaultQuery | gen.WithQueryInterface`
- **THEN** FieldNullable 设置为 `true`

### Requirement: 执行生成

系统必须能够执行 GORM GEN 生成，生成所有数据库表的 DAO 和 PO 代码。

#### Scenario: 生成所有表

- **WHEN** 系统执行生成
- **THEN** 系统调用 `generator.UseDB(db)` 设置数据库连接
- **THEN** 系统调用 `generator.ApplyBasic(generator.GenerateAllTable()...)` 生成所有表
- **THEN** 系统调用 `generator.Execute()` 执行生成
- **THEN** DAO 代码生成到指定的 OutPath
- **THEN** PO 代码生成到指定的 ModelPkgPath

#### Scenario: 生成成功输出

- **WHEN** 生成成功完成
- **THEN** 系统输出 "✓ Generated GORM code for service '<服务名>'"
- **THEN** 系统输出 "  DAO: <DAO 路径>"
- **THEN** 系统输出 "  PO: <PO 路径>"

### Requirement: 预览模式

系统必须支持预览模式，不实际连接数据库和生成文件。

#### Scenario: 预览模式

- **WHEN** GormGenerator.DryRun 为 true
- **THEN** 系统输出 "[DRY-RUN] Would generate to:"
- **THEN** 系统输出 "  DAO: <DAO 路径>"
- **THEN** 系统输出 "  PO: <PO 路径>"
- **THEN** 系统不连接数据库
- **THEN** 系统不执行 `generator.Execute()`
- **THEN** 系统返回 nil 错误
