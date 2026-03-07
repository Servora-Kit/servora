## Purpose
定义 service-config-loader 的功能需求和验证场景。

## Requirements

### Requirement: 加载服务配置

系统必须能够从服务的 `configs/config.yaml` 加载完整的 Bootstrap 配置，包括数据库连接信息。

#### Scenario: 成功加载配置

- **WHEN** 系统调用 `LoadServiceConfig("servora")`
- **THEN** 系统读取 `app/servora/service/configs/config.yaml`
- **THEN** 系统使用 `pkg/bootstrap/config/loader.LoadBootstrap()` 加载配置
- **THEN** 系统返回包含 Bootstrap 配置的 ServiceConfig 结构
- **THEN** ServiceConfig 包含服务名称、路径和 Bootstrap 配置

#### Scenario: 配置文件不存在

- **WHEN** 系统调用 `LoadServiceConfig("servora")` 但配置文件不存在
- **THEN** 系统返回错误 "load config failed: <具体错误>"

#### Scenario: 配置解析失败

- **WHEN** 系统调用 `LoadServiceConfig("servora")` 但配置文件格式错误
- **THEN** 系统返回错误 "load config failed: <具体错误>"

### Requirement: 提取数据库配置

系统必须能够从 Bootstrap 配置中提取数据库配置（driver 和 source）。

#### Scenario: 数据库配置存在

- **WHEN** Bootstrap 配置包含 `data.database` 配置
- **THEN** 系统能够访问 `Bootstrap.Data.Database.Driver`
- **THEN** 系统能够访问 `Bootstrap.Data.Database.Source`

#### Scenario: 数据库配置缺失

- **WHEN** Bootstrap 配置不包含 `data.database` 配置
- **THEN** 系统检测到 `Bootstrap.Data` 或 `Bootstrap.Data.Database` 为 nil
- **THEN** 调用方能够返回明确的错误提示

### Requirement: 支持配置中心

系统必须支持从配置中心（Nacos/Consul/etcd）加载配置，通过复用 `pkg/bootstrap/config/loader.go` 实现。

#### Scenario: 使用配置中心

- **WHEN** 服务配置中包含配置中心配置
- **THEN** 系统通过 `LoadBootstrap()` 自动从配置中心加载配置
- **THEN** 配置中心的配置覆盖本地配置

### Requirement: 支持环境变量覆盖

系统必须支持通过环境变量覆盖配置值，通过复用 `pkg/bootstrap/config/loader.go` 实现。

#### Scenario: 环境变量覆盖

- **WHEN** 环境变量 `SERVORA_DATA_DATABASE_SOURCE` 存在
- **THEN** 系统通过 `LoadBootstrap()` 自动使用环境变量值覆盖配置文件中的 database.source
