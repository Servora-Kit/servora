# Servora

[![Go Reference](https://pkg.go.dev/badge/github.com/Servora-Kit/servora.svg)](https://pkg.go.dev/github.com/Servora-Kit/servora)
[![GitHub release](https://img.shields.io/github/v/release/Servora-Kit/servora)](https://github.com/Servora-Kit/servora/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/Servora-Kit/servora)](https://goreportcard.com/report/github.com/Servora-Kit/servora)
[![License](https://img.shields.io/github/license/Servora-Kit/servora)](./LICENSE)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/Servora-Kit/servora)

简体中文

`servora` 是一个基于 Go Kratos 的微服务快速开发框架，采用 **Proto First** 开发方式，提供按域划分的框架能力（`core/`、`transport/`、`security/`、`obs/`、`platform/`、`infra/`）、自定义 protoc 插件与 CLI 工具（`cmd/`），以及框架级公共 Proto 定义（`api/protos/`）。

本仓库是 [Servora-Kit](https://github.com/Servora-Kit) 组织的**核心框架库**，不包含具体业务微服务。业务微服务请参考：

- [servora-example](https://github.com/Servora-Kit/servora-example) — 基础的双微服务示例
- [servora-iam](https://github.com/Servora-Kit/servora-iam) — 简单的 IAM 服务 
- [servora-platform](https://github.com/Servora-Kit/servora-platform) — 平台级基础服务（审计等）

## 核心能力

- **共享基础库**：认证、授权、审计、配置引导、消息代理、服务治理等开箱即用
- **传输层可插拔**：`transport/runtime` 统一 client/server 插件契约，支持协议扩展
- **Proto First**：框架级公共 proto 定义，通过 [BSR](https://buf.build/servora/servora) 发布
- **自定义 protoc 插件**：`protoc-gen-servora-authz`、`protoc-gen-servora-audit`、`protoc-gen-servora-mapper`
- **CLI 工具**：`svr` 命令行工具（GORM GEN 代码生成、OpenFGA 初始化与 model 管理）
- **可插拔认证**：`security/authn` 接口驱动，内置 JWT 引擎与 Keycloak claims 映射
- **细粒度授权**：`security/authz` 接口驱动，内置 OpenFGA 引擎
- **全链路审计**：`obs/audit` 经 Kafka 投递审计事件
- **服务治理**：注册发现、配置中心（支持重载）与基础遥测

## v0.2.0 重点变更

- **transport client/server 模式统一**：server 与 client 均通过 plugin 契约挂载到 `transport/runtime`。
- **新增 server Builder DSL**：内建 gRPC/HTTP 提供 `NewBuilder()`，调用方不需要直接拼装 runtime graph。
- **client 协议插件化**：内建 gRPC/HTTP 客户端插件，支持 `WithPlugins` / `WithRegistry` 自定义扩展。
- **共享能力下沉**：TLS、端点与公共配置归拢到 `transport/shared`，减少重复实现。

## Transport 快速示例

### 构建 gRPC/HTTP Server

```go
grpcSrv := transportgrpc.NewBuilder().
	WithConfig(c.Grpc).
	WithLogger(logger).
	WithMiddleware(mw...).
	WithServices(
		transportgrpc.Registrar(func(s *kgrpc.Server) {
			workerpb.RegisterWorkerServiceServer(s, workerSvc)
		}),
	).
	MustBuild()

httpSrv := transporthttp.NewBuilder().
	WithConfig(c.Http).
	WithLogger(logger).
	WithMiddleware(mw...).
	WithServices(
		transporthttp.Registrar(func(s *khttp.Server) {
			masterpb.RegisterMasterServiceHTTPServer(s, masterSvc)
		}),
	).
	MustBuild()
```

### 构建可插拔 Client 并发起 gRPC 调用

```go
c, err := transportclient.NewClient(dataCfg, traceCfg, discovery, logger)
if err != nil {
	return err
}

conn, err := transportclient.GetConnValue[gogrpc.ClientConnInterface](
	ctx,
	c,
	transportclient.GRPC,
	"worker.service",
)
if err != nil {
	return err
}

_, err = workerpb.NewWorkerServiceClient(conn).Hello(ctx, req)
if err != nil {
	return err
}
```

## 技术栈

- 框架：Kratos v2
- API：Protobuf + Buf v2
- DI：Google Wire
- ORM：Ent（主）+ GORM GEN（并行）
- 认证：Keycloak（OIDC）/ JWT / JWKS
- 授权：OpenFGA
- 存储：PostgreSQL + Redis
- 消息：Kafka（franz-go）
- 观测：OTel Collector / Jaeger / Loki / Prometheus / Grafana

## 项目结构

```text
.
├── api/
│   ├── gen/go/                      # Go 生成代码（由 proto 生成，勿手改）
│   └── protos/                      # 框架级公共 proto（conf、pagination、authz 注解、audit 注解、mapper 注解）
├── cmd/
│   ├── svr/                         # CLI 工具（svr gen gorm / svr openfga）
│   ├── protoc-gen-servora-authz/    # AuthZ 规则生成插件
│   ├── protoc-gen-servora-audit/    # Audit 注解生成插件
│   └── protoc-gen-servora-mapper/   # 对象映射生成插件
├── core/                            # 领域无关核心抽象（actor/mapper/pagination）
├── transport/
│   ├── client/                      # 客户端协议插件（grpc/http）
│   ├── server/                      # 服务端协议插件（grpc/http/sse）
│   ├── runtime/                     # plugin 合约、registry、graph
│   └── shared/                      # transport 共享能力（tls/endpoint/config）
├── security/                        # 认证授权与 JWT/JWKS
├── obs/                             # 审计、日志、遥测
├── platform/                        # 启动、配置、健康、注册、swagger
├── infra/                           # broker、db、k8s、openfga、redis
├── buf.yaml                         # Buf v2 workspace（公共 proto 发布到 buf.build/servora/servora）
├── buf.go.gen.yaml                  # Go 代码生成模板（含 authz / mapper / audit 等自定义插件）
├── go.mod                           # Go module: github.com/Servora-Kit/servora
└── Makefile                         # 框架构建入口
```

## 安装与使用

### 作为 Go 依赖

```bash
go get github.com/Servora-Kit/servora@latest
```

### 安装 CLI 工具

```bash
go install github.com/Servora-Kit/servora/cmd/svr@latest
```

### 安装自定义 protoc 插件

```bash
go install github.com/Servora-Kit/servora/cmd/protoc-gen-servora-authz@latest
go install github.com/Servora-Kit/servora/cmd/protoc-gen-servora-audit@latest
go install github.com/Servora-Kit/servora/cmd/protoc-gen-servora-mapper@latest
```

### 引用公共 Proto（BSR）

在业务仓库的 `buf.yaml` 中添加依赖：

```yaml
deps:
  - buf.build/servora/servora
```

## 本地开发

### 前置要求

- Go 1.26+
- Make
- Buf CLI

### 初始化开发环境

```bash
make init    # 安装 protoc 插件与 CLI 工具
make gen     # 生成 proto Go 代码
```

### 常用命令

```bash
make init          # 安装工具
make gen           # 生成所有代码（api）
make api           # 仅生成 proto Go 代码
make lint          # Go lint
make lint.proto    # Proto lint
make test          # 运行测试
make tidy          # go mod tidy + go work sync
make tag TAG=v0.x.y      # 自动打双 tag（v0.x.y + api/gen/v0.x.y）
make buf-push      # 推送 proto 到 BSR（自动使用 Git tag 作为 label）
make clean         # 清理生成代码
```

### 多仓库联合开发

框架与业务微服务采用独立仓库，本地开发时通过顶层 `go.work` 联合：

```bash
cd /path/to/servora-kit
# go.work 文件已配置 use 和 replace 指令
go build ./...
```

## 质量约束

- 不要手动编辑生成代码：`api/gen/go/`
- 修改 proto 后执行 `make gen`
- 提交前通过 `make lint` 与 `make test`

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Servora-Kit/servora&type=Date)](https://star-history.com/#Servora-Kit/servora&Date)

## Acknowledgements

- Thanks to all users for suggestions and feedback.
- Thanks to all contributors and supporters in the open-source community.

[![Contributors](https://contrib.rocks/image?repo=Servora-Kit/servora)](https://github.com/Servora-Kit/servora/graphs/contributors)

## License

MIT，详见 `LICENSE`。
