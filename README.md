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

## 快速开始🏃‍♂️

跑一对 master + worker 微服务，看看 servora 起一个项目长什么样。前置要求：Go 1.27.0+、Docker。

```bash
git clone https://github.com/Servora-Kit/servora-example
cd servora-example
make compose.up.infra            # 拉起 Consul / Jaeger / OTel Collector

# 终端 A：worker
cd app/worker/service && make run

# 终端 B：master
cd app/master/service && make run

# 验证：HTTP 200 + {"reply":"master relay -> worker says hello, hi"}
curl 'http://127.0.0.1:8001/v1/hello?greeting=hi'
```

> `make run` 直接 `go run` 启动；如需 air 热重载请改用 `make dev`（需先 `make init` 安装 air）。

完整流程（全容器化 / 热重载 / 端口约定 / 目录结构）见 [servora-example](https://github.com/Servora-Kit/servora-example)。

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

配置文件、资源 API 与审计等框架行为都可以由 Proto contract 驱动。Servora 通过代码生成配合运行时能力，让普通请求之外的框架行为保持显式、版本化和可验证。

以下是 Servora 提供的 Proto 插件，多数采用代码生成 + 运行时解析配合的方式工作。

#### 配置文件

配置消息使用 `(servora.conf.v1.section) = true` 标记按段加载，段名由消息短名转换为小写下划线形式；未标记的对象扫描整份配置。字段通过 `(field)` 声明默认值或必填要求，插件只生成 `Apply() error`。默认值仅补缺失字段；标量需要 optional 才能区分未设置和显式零值。required 只要求明确设置，非空或范围要求使用 `buf.validate` 单独声明。

```proto
import "servora/conf/v1/annotations.proto";
import "google/protobuf/duration.proto";

message Redis {
  option (servora.conf.v1.section) = true;

  string network = 1;
  string addr = 2;
  google.protobuf.Duration dial_timeout = 6 [(servora.conf.v1.field) = { default: "5s" }];
}
```

`bootstrap.Scan(rt, targets...)` 对存在的配置段完成解码并调用 Apply；缺段直接跳过，不填默认值、不创建子对象。实际使用配置的独立构造入口仍须调用 Apply，并保留所属模块的安全检查：

```go
import (
    "github.com/Servora-Kit/servora/core/bootstrap"
    redispb "github.com/Servora-Kit/servora/api/gen/go/servora/contrib/db/redis/v1"
)

redisCfg := &redispb.Redis{}
if err := bootstrap.Scan(rt, redisCfg); err != nil {
    return err
}
// redis 段存在时已解码并应用默认值；缺段时 redisCfg 保持原样。
```

Kratos `Config.Watch` 不会自动重新应用配置。自行解码、原始 Scan 或手工构造的配置，需在使用前显式调用 `Apply()` 并处理错误；不要在每个读取字段的函数重复调用。普通父配置缺失时保持缺失，只有明确提供的对象才处理内部规则。



#### CRUD 生态

Servora 提供了声明式、 AIP 风格约束的 CRUD 生态框架，用户只需要在 [API Proto](./api/protos/servora/example/v1/example.proto) 中定义 AIP 规范的资源 message ，Servora 就会利用 `protoc-gen-servora-crud` 插件生成 Go 、 TypeScript 关于 分页、排序以及过滤的辅助函数，以及关于 proto 生成的代码与 data 层所使用的 orm框架字段的绑定关系代码。通过 CRUD 生态，用户可以非常规范、方便地完成增删改查的 API 设计与业务开发。

具体使用示例可见：[plateau/example.service](https://github.com/Servora-Kit/plateau/tree/main/app/example)

#### 审计

通过 `rule` 或 `service_default` 声明 RPC 是否进入通用审计。注解只表达开关；事件类型、业务目标、详情 data 和扩展属性由事件生产者负责，例如产品服务、后续 CRUD generator 或业务显式 emit。通用 RPC 审计事件以 [CloudEvents](https://cloudevents.io/) 投递。

```proto
import "servora/audit/v1/annotations.proto";

service ResourceService {
  option (servora.audit.v1.service_default) = { mode: AUDIT_MODE_ENABLED };

  rpc CreateResource(CreateResourceRequest) returns (Resource) {
    option (servora.audit.v1.rule) = {
      mode: AUDIT_MODE_ENABLED
    };
  }
}
```

> **v0.8.7 破坏性迁移：** method option 已从 `servora.audit.v1.audit_rule` 硬切换为 `servora.audit.v1.rule`，Go extension 同步从 `auditv1.E_AuditRule` 改为 `auditv1.E_Rule`；extension number 仍为 `50100`，不提供 deprecated alias。对于仍采用独立生成 module 的 v0.8.7～v0.9.6，升级命令为 `go get github.com/Servora-Kit/servora/api/gen@v0.8.7`；从 v0.9.7 起生成 package 随根 module 发布。

Plugin 生成 `AuditRules()` 规则表，业务侧把 `Auditor` 实现（默认 Kafka）跟规则表一起挂到 middleware：

```go
import (
    "github.com/Servora-Kit/servora/obs/audit"
    pb "myapp/api/gen/go/myapp/resource/v1"
)

mw := audit.Middleware(auditor,
    audit.WithRulesFuncs(pb.AuditRules),
)
```

通用 middleware 发出的事件固定为 `servora.audit.rpc.v1`，`source` 为 `"//" + app.Name`，`subject` 为 Kratos transport operation（如 `/myapp.resource.v1.ResourceService/CreateResource`）。业务资源事件可直接用 `audit.NewEvent()` 构造自定义 CloudEvent 并调用 `Auditor.Emit`。

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
- **Audit** — 详见上文「Proto 契约化 → 审计」段，事件以 [CloudEvents](https://cloudevents.io/) 格式投递

本地起一套 Prometheus + Jaeger UI + Grafana 即可看到全链路指标与 trace（参考 [plateau](https://github.com/Servora-Kit/plateau) 的 compose 配置；`servora-example` 只内置 Consul / Jaeger / OTel Collector，可直接 curl `/metrics` 做 smoke）。

## 贡献🎉

### 辅助工具链

在 Servora 主仓开发时，`justfile` 是主动维护的任务入口，需要 [Just 1.57.0+](https://just.systems/man/en/packages.html)。迁移期根 `Makefile` 保留兼容；已有同名命令的行为变更同步维护，新命令默认只加入 `justfile`。

```bash
just --list  # 查看全部命令
just test    # 运行无外部依赖的 Go 测试
just lint    # 运行 Go 与 Proto lint
just gen     # 增量生成 Go 代码
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
