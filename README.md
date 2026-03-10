# Servora

简体中文

`servora` 是一个基于 **Go Kratos** 的微服务快开框架，采用 **DDD 分层** 与 **Proto First** 开发方式，覆盖 API 定义、代码生成、服务实现、前端联调、可观测性与容器化开发链路。

> 本 README 统一描述框架能力与 example 示例用法，适用于 `main` 与 `example` 分支。

## 📘 框架概览

## ✨ 核心能力

- **Go workspace + 多模块**：根目录 `go.work` 统一纳管根模块、`api/gen`、`app/servora/service`、`app/sayhello/service`
- **Proto First**：使用 Buf v2 workspace 管理共享 proto 与服务私有 proto
- **双协议接口**：支持 gRPC、HTTP 与 OpenAPI 产物生成
- **DDD 分层**：主服务遵循 `service -> biz -> data` 分层
- **依赖注入**：使用 Wire 管理服务依赖图
- **数据访问**：Ent 为主，GORM GEN 作为并行工具链保留
- **服务治理**：支持注册发现、配置中心与基础遥测能力
- **可观测性**：集成 OTel / Jaeger / Loki / Prometheus / Grafana
- **前后端同仓**：`servora` 前端位于 `app/servora/service/web/`

## 🧱 技术栈

- 框架：Kratos v2
- API：Protobuf + Buf v2
- DI：Google Wire
- ORM：Ent（主）+ GORM GEN（并行）
- 存储：MySQL / PostgreSQL / SQLite + Redis
- 前端：Vue 3 + Vite + TypeScript + Bun
- 观测：OTel Collector / Jaeger / Loki / Prometheus / Grafana

## 🗂️ 项目结构

```text
.
├── api/                             # 共享 proto 模块与统一生成产物
│   ├── gen/
│   │   ├── go.mod
│   │   └── go/
│   └── protos/
│       ├── conf/
│       └── pagination/
├── app/
│   ├── servora/service/             # 主服务（api/cmd/internal/web）
│   │   ├── api/
│   │   ├── cmd/
│   │   ├── configs/
│   │   ├── internal/
│   │   ├── manifests/
│   │   └── web/
│   └── sayhello/service/            # gRPC 示例服务
├── cmd/
│   └── svr/                         # CLI 工具（svr gen gorm）
├── pkg/                             # 共享基础库
├── manifests/                       # k8s / grafana / loki / otel / prometheus
├── openspec/                        # OpenSpec 变更与归档
├── app.mk                           # 服务级通用 Makefile 模板
├── buf.go.gen.yaml                  # 根级 Go 代码生成模板
├── buf.yaml                         # Buf v2 workspace
├── go.work                          # Go workspace
└── Makefile                         # 根目录统一入口
```

## 🧪 Example 示例

以下内容聚焦完整示例项目（`servora` + `sayhello`）的本地运行与开发。

## 🚀 快速开始

### 1) 前置要求

- Go 1.26+
- Make
- Docker / Docker Compose
- Bun（如需运行前端）

### 2) 克隆仓库并切换到 example 分支

```bash
git clone https://github.com/Servora-Kit/servora.git
cd servora
git checkout example
```

### 3) 配置环境

复制 `.env` 文件并根据需要修改配置：

```bash
# .env 文件包含示例配置，真实配置请使用 .env.local
cp .env .env.local
# 编辑 .env.local，填入真实的数据库密码、API 密钥等
```

### 4) 安装工具并生成代码

```bash
make init
make gen
```

`make gen` 会统一执行：`api + openapi + wire + ent`。

默认构建入口统一使用 `make build`：它会先执行完整生成，再构建所有服务；仓库不再区分 `build_only`。

### 5) 启动容器化开发环境

```bash
# 仅启动基础设施
make compose.up

# 或：直接启动基础设施 + Air 开发容器
make compose.dev
```

- `make compose.up`：仅启动基础设施（Consul、Postgres、Redis、OTel、Loki、Prometheus、Grafana 等）
- `make compose.dev`：启动基础设施 + Air 开发容器（`servora`、`sayhello`）

相关命令：

```bash
make compose.ps            # 查看基础设施状态
make compose.stop          # 仅停止基础设施容器
make compose.logs          # 查看基础设施日志
make compose.down          # 移除本地 Compose 容器/网络（保留数据卷）
make compose.reset         # 移除本地 Compose 容器/网络/数据卷
make compose.dev.ps        # 查看完整开发栈状态
make compose.dev.stop      # 仅停止完整开发栈容器
make compose.dev.logs      # 查看日志
make compose.dev.restart   # 重启服务
make compose.dev.down      # 移除完整开发栈容器/网络（保留数据卷）
make compose.dev.reset     # 移除完整开发栈容器/网络/数据卷
```

验收方式：

- 执行 `make compose.down` 清理现场后，再执行 `make compose.up`，最后用 `make compose.ps` 确认只存在基础设施容器
- 执行 `make compose.dev` 或 `make compose.dev.up` 后，用 `make compose.dev.ps` 确认基础设施和 `servora` / `sayhello` 开发容器都已启动
- 执行 `make compose.stop` 或 `make compose.dev.stop` 后，容器应处于已停止状态，但数据卷和容器定义仍保留，可直接再次 `up`
- 执行 `make compose.down` 或 `make compose.dev.down` 后，容器与网络会被移除，但不会删除 Postgres/Redis/Loki/Grafana 等 named volumes；只有显式使用 `docker compose down -v` 才会清空这些数据卷
- 执行 `make compose.reset` 或 `make compose.dev.reset` 后，会连 named volumes 一起删除；这会清空本地数据库、缓存、日志与监控数据

## 📦 示例服务

### Servora（主服务）

- **端口**：HTTP 8000 / gRPC 8001
- **功能**：用户管理、认证、前端界面
- **前端**：`app/servora/service/web/`（Vue 3 + Vite）
- **API 文档**：`http://localhost:8000/q/swagger-ui/`

### SayHello（gRPC 示例）

- **端口**：gRPC 9001
- **功能**：简单的 gRPC 服务示例
- **调用方式**：通过 gRPC 客户端或 Servora 服务调用

## 🧭 开发工作流

推荐顺序：

1. 按需修改共享 proto 或服务私有 proto：
   - `api/protos/`
   - `app/servora/service/api/protos/`
   - `app/sayhello/service/api/protos/`
2. 在仓库根目录执行 `make gen`
3. 在服务目录实现业务代码：`internal/service -> internal/biz -> internal/data`
4. 修改 Wire 依赖图后执行 `make wire`（或直接再跑一次 `make gen`）
5. 运行测试、类型检查和 lint

## 🛠️ 常用命令

### 根目录命令

```bash
# 初始化工具
make init

# 代码生成
make gen
make api
make api-go
make openapi
make wire
make ent

# 质量检查
make test
make cover
make lint.go
make lint.proto

# 构建
make build

# Compose（基础设施 / 开发 Air）
make compose.up
make compose.ps
make compose.stop
make compose.logs
make compose.down
make compose.reset

# 开发环境（基础设施 + Air）
make compose.dev
make compose.dev.ps
make compose.dev.stop
make compose.dev.logs
make compose.dev.restart
make compose.dev.down
make compose.dev.reset
```

### 服务级命令（示例：`app/servora/service/`）

```bash
make run
make build
make gen
make wire
make gen.ent
make gen.gorm
make openapi
```

服务级 `make build` 同样会先执行服务内生成流程，再编译当前服务。

### `svr` 命令行工具

```bash
# 新建 API proto 脚手架
svr new api <name> <server_name>
svr new api billing servora

# GORM GEN 代码生成
svr gen gorm <service-name...>
svr gen gorm servora --dry-run
```

### 前端命令（`app/servora/service/web/`）

```bash
cd app/servora/service/web
bun install
bun dev
bun run build
bun test:unit
bun lint
bun format
```

## 🔭 可观测性

默认观测组件（Compose 栈）：

- Grafana: `http://localhost:3001`
- Prometheus: `http://localhost:9090`
- Jaeger: `http://localhost:16686`
- Loki: `http://localhost:3100`
- OTel Collector: `4317/4318`

## 🧪 质量与约束

- 不要手动编辑生成代码：`api/gen/go/`、`wire_gen.go`、`openapi.yaml`、`web/src/service/gen/`
- 修改 proto 后务必执行 `make gen`
- 修改 Wire 依赖图后务必执行 `make wire` 或 `make gen`
- 提交前确保通过 `make lint.go` 和 `make test`

## 🤝 贡献

提交前请至少确保：

```bash
make lint.go
make test
```

## 📄 License

MIT，详见 `LICENSE`。
