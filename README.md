<p align="center">
  <img src="./docs/assets/logo.png" alt="Servora" width="720" />
</p>

<h1 align="center">Servora</h1>

<p align="center">
  声明是一切的契约
</p>

<p align="center">
  SERVORA = Servora Enables Reliable, Versioned, Observable Resource APIs
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/Servora-Kit/servora"><img src="https://pkg.go.dev/badge/github.com/Servora-Kit/servora.svg" alt="Go Reference" /></a>
  <a href="https://github.com/Servora-Kit/servora/releases"><img src="https://img.shields.io/github/v/release/Servora-Kit/servora" alt="GitHub release" /></a>
  <a href="https://github.com/Servora-Kit/servora/actions/workflows/ci.yml"><img src="https://github.com/Servora-Kit/servora/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/Servora-Kit/servora" alt="License" /></a>
  <a href="https://deepwiki.com/Servora-Kit/servora"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki" /></a>
</p>

**Servora** 是一个以 ProtoBuf 为契约、高性能、模块化的 Go 快速开发框架。无论您想构建一个单体应用还是微服务项目，Servora 都将是您的不二之选。

当前 Servora 框架源码要求 **Go 1.27.3 或更新版本**。

## 快速开始🏃‍♂️

通过官方最佳实践项目 [Plateau](https://github.com/Servora-Kit/plateau) 一键体验基于 Servora 构建的多服务全栈环境（前置要求：Go 1.27.3+、Docker、[mise](https://mise.jdx.dev/)）：

```bash
git clone https://github.com/Servora-Kit/plateau.git
cd plateau
cp .env.example .env

mise install
just init
just dev
```

> 无交互式终端时可使用后台守护模式：`just dev-up` 启动、`just dev-down` 停止本地应用进程。

- 最小多服务上手示例（全容器化 / 热重载 / 端口约定 / 目录结构）见 [servora-example](https://github.com/Servora-Kit/servora-example)。
- 生产级微服务平台最佳实践（含 IAM / OIDC、OpenFGA 授权、Kafka + ClickHouse 审计、管理后台与完整可观测栈）见 [Plateau](https://github.com/Servora-Kit/plateau)。

## 特性✨

### 服务端

Servora 提供了极为方便的 HTTP、gRPC 服务端的浅层封装。`WithConfig` 直接接收从配置文件 scan 出来的 `corev1.Server`，无需手工拼端口/网络/超时等参数。

```go
import (
    "log/slog"

    corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
    svrgrpc "github.com/Servora-Kit/servora/transport/server/grpc"
    svrmw "github.com/Servora-Kit/servora/transport/server/middleware"
    pb "myapp/api/gen/go/myapp/user/v1"
    kgrpc "github.com/go-kratos/kratos/v3/transport/grpc"
)

func NewGRPCServer(c *corev1.Server, l *slog.Logger, svc *UserService) *kgrpc.Server {
    glog := l.With("scope", "myapp/server/grpc")
    mw := svrmw.NewChainBuilder(glog).Build()
    return svrgrpc.NewServer(
        svrgrpc.WithConfig(c),
        svrgrpc.WithMiddleware(mw...),
        svrgrpc.WithServices(func(s *kgrpc.Server) {
            pb.RegisterUserServiceServer(s, svc)
        }),
    )
}
```

#### API 文档（Scalar）

通过现有 `svrhttp.WithConfig(c.Http)` 自动挂载 Scalar 文档，配置中显式开启即可：

```yaml
server:
  http:
    api_docs:
      enable: true
```

默认读取进程工作目录下的 `api/internal/assets/openapi.yaml`；访问 `/docs/` 查看文档，`/docs/openapi.yaml` 获取原文。未开启时不读取文件或暴露文档。

### 客户端

Servora 提供了极为方便的 HTTP、gRPC 客户端的浅层封装。`Dialer` 内部接管服务发现、负载均衡、连接池与中间件链，业务侧仅需 `Dial(ctx, "service.name")`。

```go
import (
    "context"
    "log/slog"

    corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
    clgrpc "github.com/Servora-Kit/servora/transport/client/grpc"
    pb "myapp/api/gen/go/myapp/user/v1"
    "github.com/go-kratos/kratos/v3/registry"
)

func CallUser(ctx context.Context, l *slog.Logger, data *corev1.Data, d registry.Discovery) error {
    dialer := clgrpc.NewDialer(
        clgrpc.WithData(data),
        clgrpc.WithDiscovery(d),
        clgrpc.WithLogger(l),
    )
    conn, err := dialer.Dial(ctx, "user.service")
    if err != nil {
        return err
    }
    _, err = pb.NewUserServiceClient(conn).GetProfile(ctx, &pb.GetProfileRequest{})
    return err
}
```

### Proto 契约化⚖️

配置文件、日志脱敏与错误编码等框架行为都可以由 Proto contract 驱动。Servora 通过代码生成配合运行时能力，让普通请求之外的框架行为保持显式、版本化和可验证。

以下是 Servora 提供的 Proto 插件，多数采用代码生成 + 运行时解析配合的方式工作。

#### 配置文件

配置消息使用 `(servora.conf.v1.section) = true` 标记按段加载，段名由消息短名转换为小写下划线形式；未标记的对象扫描整份配置。字段通过 `(field)` 声明默认值或必填要求，插件只生成 `Apply() error`。默认值仅补缺失字段；标量需要 optional 才能区分未设置和显式零值。required 只要求明确设置，非空或范围要求使用 `buf.validate` 单独声明。

```proto
import "servora/conf/v1/annotations.proto";
import "google/protobuf/duration.proto";

message CORS {
  option (servora.conf.v1.section) = true;

  bool enable = 1;
  repeated string allowed_origins = 2;
  google.protobuf.Duration max_age = 7 [(servora.conf.v1.field) = { default: "24h" }];
}
```

`bootstrap.Scan(rt, targets...)` 对存在的配置段完成解码并调用 Apply；缺段直接跳过，不填默认值、不创建子对象。实际使用配置的独立构造入口仍须调用 Apply，并保留所属模块的安全检查：

```go
import (
    "github.com/Servora-Kit/servora/core/bootstrap"
    corsv1 "github.com/Servora-Kit/servora/api/gen/go/servora/transport/http/cors/v1"
)

corsCfg := &corsv1.CORS{}
if err := bootstrap.Scan(rt, corsCfg); err != nil {
    return err
}
// cors 段存在时已解码并应用默认值；缺段时 corsCfg 保持原样。
```

Kratos `Config.Watch` 不会自动重新应用配置。自行解码、原始 Scan 或手工构造的配置，需在使用前显式调用 `Apply()` 并处理错误；不要在每个读取字段的函数重复调用。普通父配置缺失时保持缺失，只有明确提供的对象才处理内部规则。

### 服务治理

Servora 复用 Kratos 的 `registry.Registrar` / `registry.Discovery` 接口，并在 `core/registry/` 与 `core/config/` 下内置了主流后端，可在 yaml 中按 key 切换：

| 能力 | 内置后端 |
|---|---|
| 服务注册与发现 | Consul / Etcd / Nacos / Kubernetes |
| 配置中心 | Consul / Etcd / Nacos |

配置中心通过 Kratos source/watch 接入远端配置；Servora 只保证启动期 `LoadBootstrap` / `bootstrap.Scan` 的配置契约，不承诺远端配置变更自动触发 `servora-conf` 生成方法。

### 可观测性🔭

Servora 默认接入 [OpenTelemetry](https://opentelemetry.io/) SDK，开箱即用：

- **Metrics** — `obs/metrics.New` 通过 OpenTelemetry Metrics SDK + [Prometheus](https://prometheus.io/) exporter 暴露 `/metrics` endpoint；业务自定义指标通过 `metrics.Meter("your/import/path")` 创建 OTel instruments，默认 Prometheus registry 上的 `promauto` 指标不会自动并入 Servora `/metrics`
- **Tracing** — `obs/tracing.InitTracerProvider` 通过 OTLP gRPC exporter 推送到 [OTel Collector](https://opentelemetry.io/docs/collector/)，再由 Collector 转发到 [Jaeger](https://www.jaegertracing.io/) / Tempo 等任意后端
- **Logging** — `obs/logger` 提供基于 `slog` 的结构化日志，支持 stdout/file/OTel fanout

本地起一套 Prometheus + Jaeger UI + Grafana 即可看到全链路指标与 trace（参考 [plateau](https://github.com/Servora-Kit/plateau) 的 compose 配置；`servora-example` 只内置 Consul / Jaeger / OTel Collector，可直接 curl `/metrics` 做 smoke）。

## 贡献🎉

### 辅助工具链

在 Servora 主仓开发时，工具版本由 [`mise.toml`](./mise.toml) 与 [`go.mod`](./go.mod) `tool` 统一固定，`justfile` 是唯一的任务入口：

```bash
mise install     # 安装固定版本的 just、buf、golangci-lint、node、pnpm、prek
just init        # 完成工具链、prek 本地钩子与前端依赖初始化
just --list      # 查看全部命令
just test        # 运行无外部依赖的 Go 竞态单元测试
just lint        # 运行前端、Proto 与 Go 静态检查
just gen         # 同步生成 Go 与 TypeScript Proto 代码
just gen-check   # 在 GOWORK=off 下校验生成产物无漂移
```

## 星的轨迹⭐

[![Star History Chart](https://api.star-history.com/chart?repos=Servora-Kit/servora&type=date&legend=top-left&sealed_token=H00oH2v4zGvRlY242_iefrjI09EcSuxdn7klwqaZrFY_N77OGV_bAN_zQoSaS6tKwSfhhwu7j4c0LtG8u2A0x4nbBp1sZ7AmfcpVr3kED9ogpQqocsohmA)](https://www.star-history.com/?type=date&repos=Servora-Kit%2Fservora)

## 鸣谢🙏

- 特别感谢 [go-kratos](https://github.com/go-kratos/kratos)，为 servora 提供了核心能力的支撑。
- 特别感谢 [go-wind-admin](https://github.com/tx7do/go-wind-admin)，为 servora 的组织架构提供了灵感。
- 感谢所有用户的建议和反馈。
- 感谢开源社区的所有贡献者和支持者。

[![Contributors](https://contrib.rocks/image?repo=Servora-Kit/servora)](https://github.com/Servora-Kit/servora/graphs/contributors)

## 许可证🔐

Apache License 2.0，详见 [`LICENSE`](./LICENSE)。本仓库依赖的第三方组件版权声明见 [`THIRD_PARTY_LICENSES`](./THIRD_PARTY_LICENSES)。
