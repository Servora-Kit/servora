# AGENTS.md - core/bootstrap/

<!-- Parent: ../AGENTS.md -->
<!-- Updated: 2026-05-21 -->

## 模块目的

`core/bootstrap` 负责服务启动引导：加载 Bootstrap proto 配置、初始化 runtime 级资源、解析服务身份、扫描配置，并按生命周期顺序关闭 runtime 自有资源。

它只处理“框架如何把服务拉起来”，不承载业务 service、repository、handler 或领域初始化逻辑。

## 当前结构

```text
core/bootstrap/
├── bootstrap.go      # Runtime、NewRuntime、Runtime.NewApp、Runtime.Run、Runtime.Close
├── provider.go       # bootstrap.ProviderSet，供业务 Wire 图展开稳定启动根
├── scan.go           # Scan 与 Section/OptionalSection/ConfApplier 等扫描契约
├── *_test.go
└── config/           # 配置 loader，仍属于启动链
```

## 公开边界

```go
type Runtime struct { ... }

type Option func(*options)
func Name(name string) Option
func Version(version string) Option
func WithEnvPrefix() Option

func NewRuntime(configPath string, opts ...Option) (*Runtime, error)
func (rt *Runtime) NewApp(opts ...kratos.Option) *kratos.App
func (rt *Runtime) Run(build func() (*kratos.App, func(), error)) error
func (rt *Runtime) Close(ctx context.Context) error

func Scan(rt *Runtime, targets ...any) error

var ProviderSet = wire.NewSet(...)

type Section interface { SectionKey() string }
type OptionalSection interface { SectionOptional() bool }
type Defaulter interface { ApplyDefaults() }
type RequiredChecker interface { CheckRequired() error }
type ConfApplier interface { ApplyConf() error }
```

旧版一行式启动入口、专用扫描入口、公开身份结构与公开 Kratos logger 字段已不存在。遇到历史引用时迁移为显式 `NewRuntime -> Scan -> rt.Run(wireApp closure)`。

## 生命周期语义

- `NewRuntime` 负责加载 config、初始化 logger、绑定 Kratos v3 默认 logger、初始化 tracer、解析 app name/version/metadata/serviceID，并登记 runtime 自有 cleanups。
- `Runtime.Run(build)` 负责执行 `wireApp` 闭包、运行 Kratos App，并保证业务 Wire cleanup 先于 runtime 资源关闭。
- `Runtime.Close(ctx)` 只关闭 runtime 创建的资源；业务 Wire cleanup 不注册进 runtime cleanups。
- cleanup 顺序遵循 LIFO；新增启动期资源时必须明确 close 顺序。
- `Runtime.NewApp` 默认只注入 Kratos `ID/Name/Version/Metadata`，不注入 `kratos.Logger(...)`；业务继续传 server、registrar 与 lifecycle hooks。
- `Scan` 对非 `Section` target 扫描整份 config，对 `Section` target 扫描对应 section；扫描成功后才调用 `ApplyConf()`。
- Optional section 缺失时跳过 scan 与 `ApplyConf()`；非 optional 缺失或解码失败返回带 target index 与 section key 的错误。

## 边界约束

- `WithEnvPrefix()` 是环境变量前缀入口，依赖 pre-load `Name(...)`；不要在业务服务里复制命名约定。
- `config/loader.go` 是 bootstrap 启动链的一部分，但不要把 loader 细节散落到上层文档。
- 不在本包引入具体 transport server、database client、业务 repository 或 handler。
- 修改 Bootstrap proto schema 后同步检查 `api/protos/servora/core/v1/bootstrap.proto`、`make gen` 输出和本包测试。

## 常见反模式

- 重新引入旧式一行启动入口，把 runtime 创建、配置扫描和业务装配都藏进框架层。
- 让 `Runtime` 变成 service locator，把业务 provider 深层依赖改为接收 `*bootstrap.Runtime`。
- 忽略 cleanup，造成 logger/tracer/config watcher 泄漏。
- 让 section 的 `ApplyConf` 做长耗时业务启动；它应只做配置装配。

## 测试

```bash
go test ./core/bootstrap/...
```

重点覆盖：config loader、`NewRuntime` name/version/env prefix、Kratos v3 default logger 绑定、`Runtime.NewApp` identity 默认注入且不写全局 logger、`Runtime.Run` cleanup 顺序、`Runtime.Close` LIFO/idempotent/error join，以及 `Scan` whole-config/section/optional/apply 语义。
