## Purpose
定义 server-factory 的功能需求和验证场景。

## Requirements

### Requirement:  提供统一的 ServerFactory 接口支持多协议服务器创建

系统必须提供 `ServerFactory` 接口，能够根据统一的配置对象创建不同协议（HTTP, gRPC, SSE）的服务器实例。

#### Scenario:  创建 HTTP 服务器

- **WHEN** 调用 `factory.NewHTTPServer(conf, options...)`
- **THEN** 返回一个配置正确的 `*http.Server` 实例，并符合 `transport.Server` 接口。

#### Scenario:  创建 gRPC 服务器

- **WHEN** 调用 `factory.NewGRPCServer(conf, options...)`
- **THEN** 返回一个配置正确的 `*grpc.Server` 实例，并符合 `transport.Server` 接口。

### Requirement:  服务器必须符合 Lifecycle 接口以支持优雅启停

所有由工厂创建的服务器实例必须实现 `pkg/transport/server/server.go` 中定义的 `Lifecycle` 接口（`Start` 和 `Stop` 方法）。

#### Scenario:  优雅启动

- **WHEN** 调用服务器的 `Start(ctx)` 方法
- **THEN** 服务器在指定端口开始监听并处理请求。

#### Scenario:  优雅停止

- **WHEN** 调用服务器的 `Stop(ctx)` 方法
- **THEN** 服务器停止接收新请求，并在处理完现有请求后关闭。

### Requirement:  支持通过 Functional Options 注入中间件和配置

系统必须支持在创建服务器时通过 `ServerOption` 注入自定义中间件、编码器、解码器或其他协议特定的配置。

#### Scenario:  注入中间件链

- **WHEN** 使用 `server.WithMiddleware(chain...)` 选项创建服务器
- **THEN** 服务器的所有请求都会经过该中间件链处理。

### Requirement:  支持 AI-Native (MCP) 扩展

服务器工厂应预留扩展点，支持将服务接口自动暴露为 Model Context Protocol (MCP) 兼容的工具集。

#### Scenario:  启用 MCP 映射

- **WHEN** 使用 `server.WithMCP()` 选项创建服务器
- **THEN** 服务器实例具备自动注册到 MCP 控制台的能力，或暴露相应的 MCP 发现接口。

### Requirement:  资源释放必须返回 cleanup 函数

为了配合 Wire 依赖注入，服务器工厂的创建方法在必要时应支持返回 `cleanup func()`。

#### Scenario:  Wire 集成

- **WHEN** 在 Wire 提供者中使用工厂创建服务器
- **THEN** 能够正确返回服务器实例及其关联的资源释放函数。
