<p align="center">
  <img src="./docs/assets/logo.png" alt="Servora" width="720" />
</p>

<h1 align="center">Servora</h1>

<p align="center">
  声明是一切的契约
</p>

<p align="center">
  <a href="https://pkg.go.dev/github.com/Servora-Kit/servora"><img src="https://pkg.go.dev/badge/github.com/Servora-Kit/servora.svg" alt="Go Reference" /></a>
  <a href="https://github.com/Servora-Kit/servora/releases"><img src="https://img.shields.io/github/v/release/Servora-Kit/servora" alt="GitHub release" /></a>
  <a href="https://goreportcard.com/report/github.com/Servora-Kit/servora"><img src="https://goreportcard.com/badge/github.com/Servora-Kit/servora" alt="Go Report Card" /></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/Servora-Kit/servora" alt="License" /></a>
  <a href="https://deepwiki.com/Servora-Kit/servora"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki" /></a>
</p>

**Servora** 是一个以 ProtoBuf 为契约、高性能、模块化的 Go 快速开发框架。无论您想构建一个单体应用还是微服务项目，Servora 都将是您的不二之选。

## 快速开始🏃‍♂️

跑一对 master + worker 微服务，看看 servora 起一个项目长什么样。前置要求：Go 1.26.1+、Docker。

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
    corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
    svrgrpc "github.com/Servora-Kit/servora/transport/server/grpc"
    logger "github.com/Servora-Kit/servora/obs/logging"
    pb "myapp/api/gen/go/myapp/user/v1"
    kgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
)

func NewGRPCServer(c *corev1.Server, l logger.Logger, svc *UserService) *kgrpc.Server {
    return svrgrpc.NewServer(
        svrgrpc.WithConfig(c),
        svrgrpc.WithLogger(l),
        svrgrpc.WithServices(func(s *kgrpc.Server) {
            pb.RegisterUserServiceServer(s, svc)
        }),
    )
}
```

### 客户端

Servora 提供了极为方便的 HTTP、gRPC 客户端的浅层封装。`Dialer` 内部接管服务发现、负载均衡、连接池与中间件链，业务侧仅需 `Dial(ctx, "service.name")`。

```go
import (
    "context"
    corev1 "github.com/Servora-Kit/servora/api/gen/go/servora/core/v1"
    clgrpc "github.com/Servora-Kit/servora/transport/client/grpc"
    logger "github.com/Servora-Kit/servora/obs/logging"
    pb "myapp/api/gen/go/myapp/user/v1"
    "github.com/go-kratos/kratos/v2/registry"
)

func CallUser(ctx context.Context, l logger.Logger, data *corev1.Data, d registry.Discovery) error {
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

配置文件、proto type<->go type映射、认证、授权、审计...在 Servora 的世界观下，这一切都可以从一个 proto 文件定义。Servora 提供了很多 protoc 插件来代码生成，以实现尽量以 proto 文件来规定除了普通接口请求以外的所有行为。

以下是 Servora 提供的 Proto 插件，多数采用代码生成 + 运行时解析配合的方式工作。

#### 配置文件

通过 `(section)` 标记 message 在外部 yaml/json 配置中的定位键，通过 `(field)` 声明字段的默认值与必填语义。Plugin 自动生成 `SectionKey()` / `ApplyDefaults()` / `CheckRequired()` / `ApplyConf()`，配合 `bootstrap.ScanSections` 在 kratos config 中定向 scan。

```proto
import "servora/conf/v1/annotations.proto";
import "google/protobuf/duration.proto";

message Broker {
  option (servora.conf.v1.section) = { key: "broker", optional: true };

  string addr    = 1 [(servora.conf.v1.field) = { required: true }];
  string network = 2 [(servora.conf.v1.field) = { default: "tcp" }];
  google.protobuf.Duration timeout = 3 [(servora.conf.v1.field) = { default: "5s" }];
}
```

运行时只需一行 `ScanSections`，配置文件中缺失的 section 静默跳过，`ApplyConf` 自动完成必填校验 + 默认值填充：

```go
import (
    "github.com/Servora-Kit/servora/core/bootstrap"
    confv1 "myapp/api/gen/go/myapp/conf/v1"
)

brokerCfg := &confv1.Broker{}
if err := bootstrap.ScanSections(runtime, brokerCfg); err != nil {
    return err
}
// brokerCfg 已通过必填校验并自动填充默认值（由 ApplyConf 编排）
```

#### proto 类型映射

通过 `(mapper)` 声明 message 参与 ORM 实体与 proto 之间的双向映射，`(mapper_field)` 控制单字段的重命名、内置 converter、自定义钩子或忽略策略。Plugin 生成 `<Msg>MapperPlan()` 声明式映射计划，运行时配合 `core/mapper` 的泛型 `CopierMapper[P, E]` 完成双向转换。

```proto
import "servora/mapper/v1/mapper.proto";
import "google/protobuf/timestamp.proto";

message User {
  option (servora.mapper.v1.mapper) = { enabled: true };

  string id = 1 [(servora.mapper.v1.mapper_field) = {
    converter: CONVERTER_KIND_UUID_STRING
  }];
  google.protobuf.Timestamp created_at = 2 [(servora.mapper.v1.mapper_field) = {
    converter: CONVERTER_KIND_TIMESTAMP_TIME
  }];
  string internal_secret = 3 [(servora.mapper.v1.mapper_field) = { ignore: true }];
}
```

运行时构造一个泛型 mapper，把 plugin 生成的 plan 应用上去，之后即可在 proto 与实体之间双向转换：

```go
import (
    "github.com/Servora-Kit/servora/core/mapper"
    pb "myapp/api/gen/go/myapp/user/v1"
    "myapp/internal/ent"
)

m := mapper.NewCopierMapper[pb.User, ent.User]()
_ = mapper.ApplyPlan(pb.UserMapperPlan(), m, mapper.DefaultPresets(), mapper.NewHookRegistry())

proto, _ := m.ToProto(entity)
entity2, _ := m.ToEntity(proto)
```

#### 认证

通过 `service_default` 声明服务级默认认证策略，方法级 `rule` 可整段覆盖。`schemes` 接受 jwt / apikey / mtls / aksk 等任意字符串，支持业务自定义引擎。Plugin 生成 `AuthnRules` 表，由 `authn.Server` 中间件运行时分发。

```proto
import "servora/authn/v1/annotations.proto";

service UserService {
  option (servora.authn.v1.service_default) = {
    mode: MODE_REQUIRED
    schemes: ["jwt"]
  };

  // 继承 service_default：要求 jwt 通过
  rpc GetProfile(GetProfileRequest) returns (User);

  // 方法级覆盖：完全公开
  rpc Login(LoginRequest) returns (LoginResponse) {
    option (servora.authn.v1.rule) = { mode: MODE_PUBLIC };
  }
}
```

Plugin 生成 `AuthnRules()` 方法表，业务侧装一次中间件即可——`Multi + Named` 注册多种认证引擎，规则表由 `WithRulesFuncs` 注入：

```go
import (
    "github.com/Servora-Kit/servora/security/authn"
    authjwt "github.com/Servora-Kit/servora/security/authn/jwt"
    "github.com/Servora-Kit/servora/security/authn/apikey"
    pb "myapp/api/gen/go/myapp/user/v1"
)

mw := authn.Server(
    authn.Multi(
        authn.Named(authjwt.Scheme, authjwt.NewAuthenticator(authjwt.WithVerifier(verifier))),
        authn.Named(apikey.Scheme, apikey.NewAuthenticator(apikey.WithStore(keyStore))),
    ),
    authn.WithRulesFuncs(pb.AuthnRules),
)
```

#### 授权

跟认证同构：`service_default` 声明服务级默认授权策略，方法级 `rule` 可整段覆盖。授权检查由 `action` × `resource_type` × `resource_id_field`（从请求消息中提取资源 ID）三元组定义。默认接入 OpenFGA，可替换为任意实现 `Authorizer` 接口的后端。

```proto
import "servora/authz/v1/authz.proto";

service VideoService {
  option (servora.authz.v1.service_default) = {
    mode: AUTHZ_MODE_CHECK
    action: "can_read"
    resource_type: "video"
    resource_id_field: "id"
  };

  // 继承 service_default：检查 can_read on video[req.id]
  rpc GetVideo(GetVideoRequest) returns (Video);

  // 方法级覆盖：换成 can_delete
  rpc DeleteVideo(DeleteVideoRequest) returns (DeleteVideoResponse) {
    option (servora.authz.v1.rule) = {
      mode: AUTHZ_MODE_CHECK
      action: "can_delete"
      resource_type: "video"
      resource_id_field: "id"
    };
  }

  // 方法级覆盖：完全跳过授权
  rpc ListPublicVideos(ListPublicVideosRequest) returns (ListPublicVideosResponse) {
    option (servora.authz.v1.rule) = { mode: AUTHZ_MODE_NONE };
  }
}
```

Plugin 生成 `AuthzRules()` 规则表，业务侧把 `Engine` 实现（OpenFGA / 自研后端）连同规则表交给 `authz.Server` 即可：

```go
import (
    "github.com/Servora-Kit/servora/security/authz"
    "github.com/Servora-Kit/servora/security/authz/openfga"
    pb "myapp/api/gen/go/myapp/video/v1"
)

engine := openfga.NewEngine(openfga.WithStoreID("..."))
mw := authz.Server(engine, authz.WithRulesFuncs(pb.AuthzRules))
```

#### 审计

通过 `audit_rule` 在方法上声明审计事件，事件以 [CloudEvents](https://cloudevents.io/) 格式投递（默认走 Kafka）。`extensions` 支持声明式地从请求/响应中提取任意 CloudEvents 扩展属性。

```proto
import "servora/audit/v1/annotations.proto";

service ResourceService {
  rpc CreateResource(CreateResourceRequest) returns (Resource) {
    option (servora.audit.v1.audit_rule) = {
      mode: AUDIT_MODE_ENABLED
      event_type: "myapp.resource.created"
      severity: "info"
      target_id_field: "resp.id"
      extensions: [
        { name: "mutation"     literal: { ce_string: "CREATE" } },
        { name: "resourcetype" literal: { ce_string: "resource" } }
      ]
    };
  }
}
```

Plugin 生成 `AuditRules()` 编译后规则表（含 extension 提取闭包），业务侧把 `Auditor` 实现（默认 Kafka）跟规则表一起挂到 middleware：

```go
import (
    "github.com/Servora-Kit/servora/obs/audit"
    pb "myapp/api/gen/go/myapp/resource/v1"
)

mw := audit.Middleware(auditor,
    audit.WithRulesFuncs(pb.AuditRules),
)
```

### 服务治理

Servora 复用 Kratos 的 `registry.Registrar` / `registry.Discovery` 接口，并在 `core/registry/` 与 `core/config/` 下内置了主流后端，可在 yaml 中按 key 切换：

| 能力 | 内置后端 |
|---|---|
| 服务注册与发现 | Consul / Etcd / Nacos / Kubernetes |
| 配置中心 | Consul / Etcd / Nacos |

配合 `bootstrap.ScanSections`，远端配置中心变更后可触发业务 message 自动重 scan + 校验 + 回调，实现**动态重载**。

### 可观测性🔭

Servora 默认接入 [OpenTelemetry](https://opentelemetry.io/) SDK，开箱即用：

- **Metrics** — `obs/telemetry.NewMetrics` 通过 [Prometheus](https://prometheus.io/) exporter 暴露 `/metrics` endpoint；业务方拿到的 `*Metrics` 实例可直接 emit 自定义指标
- **Tracing** — `obs/telemetry.InitTracerProvider` 通过 OTLP gRPC exporter 推送到 [OTel Collector](https://opentelemetry.io/docs/collector/)，再由 Collector 转发到 [Jaeger](https://www.jaegertracing.io/) / Tempo 等任意后端
- **Logging** — `obs/logging` 提供结构化日志接口，内置 GORM 适配
- **Audit** — 详见上文「Proto 契约化 → 审计」段，事件以 [CloudEvents](https://cloudevents.io/) 格式投递

本地起一套 Prometheus + Jaeger UI + Grafana 即可看到全链路指标与 trace（参考 [servora-example](https://github.com/Servora-Kit/servora-example) 的 compose 配置）。

## 星的轨迹⭐

[![Star History Chart](https://api.star-history.com/svg?repos=Servora-Kit/servora&type=Date)](https://star-history.com/#Servora-Kit/servora&Date)

## 鸣谢🙏

- 特别感谢 [go-kratos](https://github.com/go-kratos/kratos)，为 servora 提供了核心能力的支撑。
- 特别感谢 [go-wind-admin](https://github.com/tx7do/go-wind-admin)，为 servora 的组织架构提供了灵感。
- 感谢所有用户的建议和反馈。  
- 感谢开源社区的所有贡献者和支持者。

[![Contributors](https://contrib.rocks/image?repo=Servora-Kit/servora)](https://github.com/Servora-Kit/servora/graphs/contributors)

## 许可证🔐

Apache License 2.0，详见 [`LICENSE`](./LICENSE)。本仓库依赖的第三方组件版权声明见 [`THIRD_PARTY_LICENSES`](./THIRD_PARTY_LICENSES)。
