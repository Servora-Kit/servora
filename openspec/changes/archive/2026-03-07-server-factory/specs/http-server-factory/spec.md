## 新增需求

### 需求:HTTP Server 工厂函数

系统必须提供 `NewHTTPServer` 工厂函数，接受可变数量的 `HTTPServerOption` 参数，返回 `*http.Server` 实例。

#### 场景:创建基础 HTTP Server

- **当** 调用 `NewHTTPServer()` 不传任何 Option
- **那么** 必须返回一个可用的 `*http.Server` 实例，使用默认配置

#### 场景:创建带完整配置的 HTTP Server

- **当** 调用 `NewHTTPServer(WithHTTPConfig(c), WithHTTPLogger(l), WithHTTPMiddleware(mw...), WithCORS(cors), WithMetrics(mtc), WithHTTPServices(registrars...))`
- **那么** 必须返回配置了所有选项的 `*http.Server` 实例

---

### 需求:HTTP Config Option

系统必须提供 `WithHTTPConfig(c *conf.Server_HTTP)` Option，用于配置 Network、Addr、Timeout 和 TLS。

#### 场景:配置 Network

- **当** 传入 `WithHTTPConfig(&conf.Server_HTTP{Network: "tcp4"})`
- **那么** 创建的 Server 必须使用 `tcp4` 网络类型

#### 场景:配置 Addr

- **当** 传入 `WithHTTPConfig(&conf.Server_HTTP{Addr: ":8080"})`
- **那么** 创建的 Server 必须监听 `:8080` 地址

#### 场景:配置 Timeout

- **当** 传入 `WithHTTPConfig(&conf.Server_HTTP{Timeout: durationpb.New(30*time.Second)})`
- **那么** 创建的 Server 必须设置 30 秒超时

#### 场景:配置 TLS 成功

- **当** 传入 `WithHTTPConfig(&conf.Server_HTTP{Tls: &conf.TLSConfig{Enable: true, CertPath: "valid.crt", KeyPath: "valid.key"}})`
- **那么** 创建的 Server 必须启用 TLS

#### 场景:配置 TLS 失败

- **当** 传入 `WithHTTPConfig(&conf.Server_HTTP{Tls: &conf.TLSConfig{Enable: true, CertPath: "invalid.crt", KeyPath: "invalid.key"}})`
- **那么** 必须 panic，因为 TLS 配置错误是严重错误

#### 场景:Config 为 nil

- **当** 传入 `WithHTTPConfig(nil)`
- **那么** 必须使用默认配置，不得 panic

---

### 需求:HTTP Logger Option

系统必须提供 `WithHTTPLogger(l log.Logger)` Option，用于设置 Server 的日志记录器。

#### 场景:设置 Logger

- **当** 传入 `WithHTTPLogger(logger)`
- **那么** 创建的 Server 必须使用该 Logger 记录日志

#### 场景:Logger 为 nil

- **当** 传入 `WithHTTPLogger(nil)`
- **那么** 创建的 Server 必须不设置 Logger（使用 Kratos 默认行为）

---

### 需求:HTTP Middleware Option

系统必须提供 `WithHTTPMiddleware(mw ...middleware.Middleware)` Option，用于设置中间件链。

#### 场景:设置中间件

- **当** 传入 `WithHTTPMiddleware(recovery.Recovery(), logging.Server(l))`
- **那么** 创建的 Server 必须按顺序应用这些中间件

#### 场景:空中间件

- **当** 传入 `WithHTTPMiddleware()` 或不传此 Option
- **那么** 创建的 Server 必须不设置额外中间件

---

### 需求:HTTP CORS Option

系统必须提供 `WithCORS(c *conf.CORS)` Option，用于启用 CORS 中间件。

#### 场景:启用 CORS

- **当** 传入 `WithCORS(&conf.CORS{Enable: true, AllowedOrigins: []string{"*"}})`
- **那么** 创建的 Server 必须启用 CORS Filter

#### 场景:CORS 未启用

- **当** 传入 `WithCORS(&conf.CORS{Enable: false})`
- **那么** 创建的 Server 禁止启用 CORS Filter

#### 场景:CORS 为 nil

- **当** 传入 `WithCORS(nil)`
- **那么** 创建的 Server 禁止启用 CORS Filter

---

### 需求:HTTP Metrics Option

系统必须提供 `WithMetrics(m *telemetry.Metrics)` Option，用于挂载 `/metrics` 端点。

#### 场景:启用 Metrics

- **当** 传入 `WithMetrics(metricsInstance)`
- **那么** 创建的 Server 必须在 `/metrics` 路径挂载 Metrics Handler

#### 场景:Metrics 为 nil

- **当** 传入 `WithMetrics(nil)`
- **那么** 创建的 Server 禁止挂载 `/metrics` 端点

---

### 需求:HTTP Services Registrar

系统必须提供 `WithHTTPServices(registrars ...HTTPRegistrar)` Option，用于注册服务。

#### 场景:注册单个服务

- **当** 传入 `WithHTTPServices(func(s *http.Server) { RegisterMyService(s, svc) })`
- **那么** 创建的 Server 必须调用该 Registrar 完成服务注册

#### 场景:注册多个服务

- **当** 传入 `WithHTTPServices(reg1, reg2, reg3)`
- **那么** 创建的 Server 必须按顺序调用所有 Registrar

#### 场景:无服务注册

- **当** 不传 `WithHTTPServices` Option
- **那么** 创建的 Server 必须正常返回，不注册任何服务
