## 新增需求

### 需求:加载服务配置

系统必须能够从服务的 `configs/config.yaml` 加载完整的 Bootstrap 配置，包括数据库连接信息。

#### 场景:成功加载配置

- **当** 系统调用 `LoadServiceConfig("servora")`
- **那么** 系统读取 `app/servora/service/configs/config.yaml`
- **那么** 系统使用 `pkg/bootstrap/config/loader.LoadBootstrap()` 加载配置
- **那么** 系统返回包含 Bootstrap 配置的 ServiceConfig 结构
- **那么** ServiceConfig 包含服务名称、路径和 Bootstrap 配置

#### 场景:配置文件不存在

- **当** 系统调用 `LoadServiceConfig("servora")` 但配置文件不存在
- **那么** 系统返回错误 "load config failed: <具体错误>"

#### 场景:配置解析失败

- **当** 系统调用 `LoadServiceConfig("servora")` 但配置文件格式错误
- **那么** 系统返回错误 "load config failed: <具体错误>"

### 需求:提取数据库配置

系统必须能够从 Bootstrap 配置中提取数据库配置（driver 和 source）。

#### 场景:数据库配置存在

- **当** Bootstrap 配置包含 `data.database` 配置
- **那么** 系统能够访问 `Bootstrap.Data.Database.Driver`
- **那么** 系统能够访问 `Bootstrap.Data.Database.Source`

#### 场景:数据库配置缺失

- **当** Bootstrap 配置不包含 `data.database` 配置
- **那么** 系统检测到 `Bootstrap.Data` 或 `Bootstrap.Data.Database` 为 nil
- **那么** 调用方能够返回明确的错误提示

### 需求:支持配置中心

系统必须支持从配置中心（Nacos/Consul/etcd）加载配置，通过复用 `pkg/bootstrap/config/loader.go` 实现。

#### 场景:使用配置中心

- **当** 服务配置中包含配置中心配置
- **那么** 系统通过 `LoadBootstrap()` 自动从配置中心加载配置
- **那么** 配置中心的配置覆盖本地配置

### 需求:支持环境变量覆盖

系统必须支持通过环境变量覆盖配置值，通过复用 `pkg/bootstrap/config/loader.go` 实现。

#### 场景:环境变量覆盖

- **当** 环境变量 `SERVORA_DATA_DATABASE_SOURCE` 存在
- **那么** 系统通过 `LoadBootstrap()` 自动使用环境变量值覆盖配置文件中的 database.source
