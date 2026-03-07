## Purpose
定义 app-mk-gen-gorm 的功能需求和验证场景。

## Requirements

### Requirement: gen.gorm 目标调用中心化命令

`app.mk` 中的 `gen.gorm` 目标必须调用 `svr gen gorm` 命令，而不是执行服务本地的生成脚本。

#### Scenario: 执行 make gen.gorm

- **WHEN** 开发者在服务目录下执行 `make gen.gorm`
- **THEN** 系统切换到仓库根目录
- **THEN** 系统执行 `svr gen gorm <服务名>`
- **THEN** 生成结果与直接调用 `svr gen gorm` 相同

#### Scenario: 服务名称自动识别

- **WHEN** 开发者在 `app/servora/service` 目录下执行 `make gen.gorm`
- **THEN** 系统自动识别服务名称为 "servora"
- **THEN** 系统执行 `svr gen gorm servora`

#### Scenario: 输出提示

- **WHEN** 开发者执行 `make gen.gorm`
- **THEN** 系统首先输出 "Generating GORM DAO/PO..."
- **THEN** 系统然后输出 `svr gen gorm` 的执行结果

### Requirement: 向后兼容

更新后的 `gen.gorm` 目标必须保持与原有工作流的兼容性。

#### Scenario: 开发者体验不变

- **WHEN** 开发者执行 `make gen.gorm`
- **THEN** 生成的 DAO 和 PO 代码位置与之前相同
- **THEN** 生成的代码格式与之前相同
- **THEN** 命令执行成功时退出码为 0
- **THEN** 命令执行失败时退出码非 0

#### Scenario: 错误处理

- **WHEN** `svr gen gorm` 执行失败
- **THEN** `make gen.gorm` 也失败并返回相同的错误
- **THEN** 错误信息清晰显示失败原因
