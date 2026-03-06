# servora Development Guide

This project follows the [go-wind-admin](https://github.com/tx7do/go-wind-admin) project structure and best practices.

## 双分支策略

本仓库采用双分支架构，以支持框架开发和完整示例的并存：

- **main 分支**：纯框架代码，用于 Go module 发布
  - 包含：`pkg/`、`cmd/svr/`、`api/protos/`、`templates/`、文档
  - 不包含：服务实现（`app/`）、部署配置（`manifests/`、`docker-compose.yaml`）

- **example 分支**：完整示例项目
  - 包含：框架代码 + 示例服务（servora、sayhello）+ 部署配置
  - 用于：开发、测试、演示

### 为什么使用双分支？

1. **框架纯净性**：main 分支可以直接作为 Go module 发布，不包含示例代码
2. **开发便利性**：example 分支提供完整环境，方便在真实场景中开发和测试框架
3. **历史连续性**：两个分支共享 git 历史，便于追溯和同步
4. **简化管理**：避免多仓库方案的复杂性

### 开发工作流程

**主要开发在 example 分支进行**：
```bash
git checkout example
# 进行开发...
```

**框架相关的提交同步到 main**：
```bash
# 查看需要同步的框架提交
git log --oneline --grep="^feat(pkg):" --grep="^fix(pkg):" --grep="^feat(cmd):" --grep="^fix(cmd):"

# 同步到 main
git checkout main
git cherry-pick <commit-hash>
git checkout example
```

详细工作流程请参考本文档后续章节。

## Project Structure

```
servora/
├── api/                                    # API definitions
│   ├── protos/                            # Protocol buffer definitions
│   │   ├── servora/service/v1/           # servora HTTP services (i_*.proto)
│   │   ├── auth/service/v1/              # Auth gRPC service
│   │   ├── user/service/v1/              # User gRPC service
│   │   ├── test/service/v1/              # Test gRPC service
│   │   └── sayhello/service/v1/          # SayHello gRPC service
│   ├── gen/go/                           # Generated Go code
│   ├── buf.gen.yaml                      # Buf code generation config
│   └── buf.servora.openapi.gen.yaml     # servora OpenAPI generation config
├── app/                                   # Application services
│   └── servora/service/                  # servora service
│       ├── cmd/                          # Command line entry points
│       ├── internal/                     # Internal code
│       ├── configs/                      # Configuration files
│       ├── bin/                          # Build output
│       ├── openapi.yaml                  # Generated OpenAPI documentation
│       └── Makefile                      # Service Makefile (includes app.mk)
├── pkg/                                   # Shared packages
├── app.mk                                 # Common Makefile for all services
├── Makefile                               # Root Makefile
└── .env.example                          # Environment variables example

```

## Quick Start

### 1. Initialize Development Environment

```bash
# Install all required tools
make init

# Or install separately
make plugin  # Install protoc plugins
make cli     # Install CLI tools
```

### 2. Generate Code

```bash
# Generate all code (protobuf + OpenAPI)
make gen

# Or generate separately
make api      # Generate protobuf Go code
make openapi  # Generate OpenAPI documentation
make wire     # Generate dependency injection code
```

### 3. Build Services

```bash
# Build all services
make build

# Build without code generation
make build_only

# Generate and build everything
make all
```

### 4. Run Service

```bash
# Run servora service
cd app/servora/service
make run

# Or build and run
make build
./bin/server -c ./configs
```

## Development Workflow

### Adding a New Service

1. Create service directory structure:

```bash
mkdir -p app/{service-name}/service/{cmd/server,internal,configs}
```

1. Create service Makefile:

```bash
echo "include ../../../app.mk" > app/{service-name}/service/Makefile
```

1. Create OpenAPI config for the service:

```bash
cp api/buf.servora.openapi.gen.yaml api/buf.{service-name}.openapi.gen.yaml
# Edit the config to point to your service's proto files
```

1. Create proto files:
   - HTTP interfaces: `api/protos/{service-name}/service/v1/i_*.proto`
   - gRPC interfaces: `api/protos/{domain}/service/v1/{domain}.proto`

### Proto File Organization

Following go-wind-admin conventions:

- **i_*.proto files**: HTTP interfaces with `google.api.http` annotations
  - Located in: `api/protos/{service-name}/service/v1/`
  - Package: `{service-name}.service.v1`
  - Used for: OpenAPI generation

- **Other .proto files**: Pure gRPC interfaces
  - Located in: `api/protos/{domain}/service/v1/`
  - Package: `{domain}.service.v1`
  - Used for: gRPC service implementation

Example:

```
api/protos/
├── servora/service/v1/
│   ├── i_auth.proto      # HTTP: package servora.service.v1
│   ├── i_user.proto      # HTTP: package servora.service.v1
│   └── i_test.proto      # HTTP: package servora.service.v1
├── auth/service/v1/
│   └── auth.proto        # gRPC: package auth.service.v1
├── user/service/v1/
│   └── user.proto        # gRPC: package user.service.v1
└── test/service/v1/
    └── test.proto        # gRPC: package test.service.v1
```

## Makefile Targets

### Root Makefile

| Target | Description |
|--------|-------------|
| `make init` | Initialize development environment |
| `make plugin` | Install protoc plugins |
| `make cli` | Install CLI tools |
| `make api` | Generate all protobuf Go code |
| `make openapi` | Generate OpenAPI docs for all services |
| `make wire` | Generate wire code for all services |
| `make gen` | Generate all code (wire + api + openapi) |
| `make build` | Build all services |
| `make build_only` | Build without code generation |
| `make all` | Generate and build all services |
| `make test` | Run tests |
| `make lint.go` | Run golang linter |
| `make clean` | Clean build artifacts |

### Service Makefile (app/{service}/service/)

All services use the common `app.mk` file:

| Target | Description |
|--------|-------------|
| `make build` | Build service (with code generation) |
| `make build_only` | Build service only |
| `make run` | Run service |
| `make api` | Generate protobuf code |
| `make openapi` | Generate OpenAPI documentation |
| `make wire` | Generate wire code |
| `make gen` | Generate all code |
| `make clean` | Clean build files |
| `make env` | Show environment variables |

## Code Generation

### Protobuf Generation

```bash
# Generate all protobuf code
make api

# Or use buf directly
cd api && buf generate
```

### OpenAPI Generation

Each service has its own OpenAPI config file:

- `api/buf.servora.openapi.gen.yaml` - servora service

OpenAPI files are generated to each service directory:

- `app/servora/service/openapi.yaml`

```bash
# Generate OpenAPI for all services
make openapi

# Generate for specific service
cd app/servora/service && make openapi
```

### Wire Generation

```bash
# Generate wire code for all services
make wire

# Generate for specific service
cd app/servora/service && make wire
```

## Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
cp .env.example .env
```

Variables:

- `PROJECT_NAME`: Project name (default: servora)
- `SERVICE_APP_VERSION`: Service version (default: 0.0.1)
- `VERSION`: Build version (overridden by git tags)

## Tools Required

- Go 1.21+
- buf CLI
- protoc plugins:
  - protoc-gen-go
  - protoc-gen-go-grpc
  - protoc-gen-go-http
  - protoc-gen-go-errors
  - protoc-gen-openapi
  - protoc-gen-validate
- wire
- golangci-lint

Install all tools:

```bash
make init
```

## Tips

1. **Always run from root directory**: The Makefile is designed to work from the project root
2. **Use `make gen` before building**: Ensures all generated code is up-to-date
3. **Check `make env`**: Verify environment variables in each service
4. **OpenAPI location**: Each service's OpenAPI file is in its own directory

## References

- [go-wind-admin](https://github.com/tx7do/go-wind-admin) - Project structure reference
- [Buf](https://buf.build/) - Protocol buffer tooling
- [Kratos](https://go-kratos.dev/) - Microservice framework
- [Wire](https://github.com/google/wire) - Dependency injection
