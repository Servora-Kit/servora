## 新增需求

### 需求:gRPC Server 工厂函数

系统必须提供 `NewGRPCServer` 工厂函数，接受可变数量的 `GRPCServerOption` 参数，返回 `*grpc.Server` 实例。

#### 场景:创建基础 gRPC Server

- **当** 调用 `NewGRPCServer()` 不传任何 Option
- **那么** 必须返回一个可用的 `*grpc.Server` 实例，使用默认配置

#### 场景:创建带完整配置的 gRPC Server

- **当** 调用 `NewGRPCServer(WithGRPCConfig(c), WithGRPCLogger(l), WithGRPCMiddleware(mw...), WithGRPCServices(registrars...))`
- **那么** 必须返回配置了所有选项的 `*grpc.Server` 实例

---

### 需求:gRPC Config Option

系统必须提供 `WithGRPCConfig(c *conf.Server_GRPC)` Option，用于配置 Network、Addr、Timeout 和 TLS。

#### 场景:配置 Network

- **当** 传入 `WithGRPCConfig(&conf.Server_GRPC{Network: "tcp4"})`
- **那么** 创建的 Server 必须使用 `tcp4` 网络类型

#### 场景:配置 Addr

- **当** 传入 `WithGRPCConfig(&conf.Server_GRPC{Addr: ":9000"})`
- **那么** 创建的 Server 必须监听 `:9000` 地址

#### 场景:配置 Timeout

- **当** 传入 `WithGRPCConfig(&conf.Server_GRPC{Timeout: durationpb.New(30*time.Second)})`
- **那么** 创建的 Server 必须设置 30 秒超时

#### 场景:配置 TLS 成功

- **当** 传入 `WithGRPCConfig(&conf.Server_GRPC{Tls: &conf.TLSConfig{Enable: true, CertPath: "valid.crt", KeyPath: "valid.key"}})`
- **那么** 创建的 Server 必须启用 TLS

#### 场景:配置 TLS 失败

- **当** 传入 `WithGRPCConfig(&conf.Server_GRPC{Tls: &conf.TLSConfig{Enable: true, CertPath: "invalid.crt", KeyPath: "invalid.key"}})`
- **那么** 必须 panic，因为 TLS 配置错误是严重错误

#### 场景:Config 为 nil

- **当** 传入 `WithGRPCConfig(nil)`
- **那么** 必须使用默认配置，不得 panic

---

### 需求:gRPC Logger Option

系统必须提供 `WithGRPCLogger(l log.Logger)` Option，用于设置 Server 的日志记录器。

#### 场景:设置 Logger

- **当** 传入 `WithGRPCLogger(logger)`
- **那么** 创建的 Server 必须使用该 Logger 记录日志

#### 场景:Logger 为 nil

- **当** 传入 `WithGRPCLogger(nil)`
- **那么** 创建的 Server 必须不设置 Logger（使用 Kratos 默认行为）

---

### 需求:gRPC Middleware Option

系统必须提供 `WithGRPCMiddleware(mw ...middleware.Middleware)` Option，用于设置中间件链。

#### 场景:设置中间件

- **当** 传入 `WithGRPCMiddleware(recovery.Recovery(), logging.Server(l))`
- **那么** 创建的 Server 必须按顺序应用这些中间件

#### 场景:空中间件

- **当** 传入 `WithGRPCMiddleware()` 或不传此 Option
- **那么** 创建的 Server 必须不设置额外中间件

---

### 需求:gRPC Services Registrar

系统必须提供 `WithGRPCServices(registrars ...GRPCRegistrar)` Option，用于注册服务。

#### 场景:注册单个服务

- **当** 传入 `WithGRPCServices(func(s *grpc.Server) { RegisterMyService(s, svc) })`
- **那么** 创建的 Server 必须调用该 Registrar 完成服务注册

#### 场景:注册多个服务

- **当** 传入 `WithGRPCServices(reg1, reg2, reg3)`
- **那么** 创建的 Server 必须按顺序调用所有 Registrar

#### 场景:无服务注册

- **当** 不传 `WithGRPCServices` Option
- **那么** 创建的 Server 必须正常返回，不注册任何服务
