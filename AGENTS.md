# AGENTS.md - servora 项目根目录

<!-- Generated: 2026-03-15 | Updated: 2026-03-15 -->

## 项目概览

`servora` 是一个基于 **Go Kratos** 的微服务脚手架框架，采用 **Go workspace + 多模块** 与 **Buf v2 workspace** 组织方式。项目同时包含框架核心（`pkg/`、`cmd/`）和服务实现（`app/`），当前已完成 IAM 服务的主要开发。

当前主线事实：
- 所有开发均在 `main` 分支进行，不使用多分支策略
- 根目录保留 `go.mod`，并通过 `go.work` 纳管 `api/gen`、`app/iam/service`、`app/sayhello/service`
- Proto 采用多模块联合编排：`api/protos/`、`app/iam/service/api/protos/`、`app/sayhello/service/api/protos/`
- 共享生成入口在根目录：`make gen`、`make api`、`make openapi`、`make wire`、`make ent`

## 开发约束

### 提交消息格式

**强制规范**：所有提交必须遵循以下格式：

```
type(scope): description
```

**允许的 type**：`feat`、`fix`、`refactor`、`docs`、`test`、`chore`

**建议的 scope**：
- `api`：API / Proto / OpenAPI 相关
- `buf`：Buf 配置与生成链路
- `cmd`：CLI 工具
- `pkg`：框架核心代码
- `manifests/scripts`：脚本与自动化任务（k6、postgres-init 等）
- `templates`：模板资源
- `tool-chain`：工具链与构建体系（如 `tool-chain/mk`）
- `md`：Markdown 文档（建议使用二级域，如 `md/readme`）
- `docs`：非 Markdown 文档或文档体系治理（建议使用二级域，如 `docs/reference`）
- `openspec`：OpenSpec 变更管理
- `repo`：仓库治理/元信息（如 ignore、目录约定）
- `app`：应用服务
- `infra`：基础设施/部署

> 说明：scope 不必来自上述列表，只校验 `type(scope): description` 基本格式。
> scope 仍建议使用小写、语义化、简短命名（可包含 `a-z`、`0-9`、`.`、`_`、`/`、`-`）。
> 推荐优先采用"一级域/二级域"结构，例如：`tool-chain/mk`、`md/readme`、`api/proto`。

**提交最佳实践**：
1. 保持提交小而专注：一个提交只做一件事
2. 使用清晰的描述：描述"做了什么"，而不是"怎么做的"
3. 遵循格式：保持 `type(scope): description` 格式便于历史与工具解析

**规范的灵活性**：
- 当现有建议的一级域能够表达语义时，优先在其下使用二级域（如 `md/readme`、`tool-chain/mk`）
- 优先保持 scope 与改动边界一致，避免为了"套用已有分类"而使用不准确 scope
- 若新增 scope 会被频繁复用，可将其补充到本节"建议的 scope"中
- 若判断没有合适的**一级域**，必须先向用户/维护者申请新增域；在获得同意后，再同步更新本文件

## 顶层目录

- `api/`：共享 proto、生成产物 `api/gen/` 与相关 AGENTS
- `app/`：服务实现；当前包含 `iam/service/`（IAM 微服务）与 `sayhello/service/`（示例服务）
- `cmd/svr/`：中心化 CLI，当前提供 `svr gen gorm`
- `cmd/protoc-gen-servora-authz/`：自定义 protoc 插件，用于生成 AuthZ 规则
- `pkg/`：共享基础库，现有 `actor`、`bootstrap`、`ent/mixin`、`governance`、`health`、`helpers`、`jwks`、`jwt`、`k8s`、`logger`、`mapper`、`openfga`、`redis`、`transport`
- `manifests/`：统一部署清单，K8s 在 `manifests/k8s/`；OpenFGA model 在 `manifests/openfga/`；脚本在 `manifests/scripts/`
- `templates/`：通用部署模板，给使用框架的人作为参考
- `docs/`：文档目录；当前包含 `design/`、`development/`、`knowledge/`、`reference/`
- `openspec/`：OpenSpec 变更与归档

## 关键文件

- `Makefile`：根构建入口，负责 `api`、`openapi`、`wire`、`ent`、构建与 Compose
- `app.mk`：服务级通用 Makefile；服务目录中的 `Makefile` 通过 `include ../../../app.mk` 复用
- `buf.yaml`：Buf v2 workspace，声明三个 proto module 路径
- `buf.go.gen.yaml`：根级 Go 代码生成模板，输出到 `api/gen/go`
- `buf.authz.gen.yaml`：AuthZ 规则生成模板，使用 `protoc-gen-servora-authz` 插件
- `go.work` / `go.work.sum`：多模块工作区配置
- `README.md`：项目入口说明

## 当前目录约定

### API / Proto
- 共享 proto 放在 `api/protos/`
- IAM 服务 proto 放在 `app/iam/service/api/protos/`（含 `authz/service/v1/authz.proto` 授权注解定义）
- `sayhello` 服务 proto 放在 `app/sayhello/service/api/protos/`
- Go 生成代码统一输出到 `api/gen/go/`

### 服务实现
- `app/iam/service/`：IAM 微服务（认证、授权、组织、项目），包含 `api/`、`cmd/`、`internal/`、`configs/`
- `app/sayhello/service/`：独立示例服务，包含自己的 `api/` 与运行时目录

### 前端
- 目录：各服务下 `web/`（如有）
- 生成的 TypeScript HTTP 客户端输出到该服务 `web/src/service/gen/`

### 部署
- K8s 基础设施：`manifests/k8s/base/`
- 服务清单：`manifests/k8s/iam/`、`manifests/k8s/sayhello/`

## 常用命令

### 初始化与生成
```bash
make init          # 安装工具
make gen           # 统一生成（api + openapi + wire + ent）
make build         # 统一生成后构建所有服务
```

### 开发与测试
```bash
make compose.up    # 仅启动基础设施
make compose.dev   # 启动开发环境
make compose.stop  # 仅停止基础设施容器
make compose.down  # 移除容器/网络，保留数据卷
make compose.reset # 移除容器/网络/数据卷
make test          # 运行测试
make lint.go       # Go 代码检查
```

### CLI 工具
```bash
svr gen gorm <service-name...>      # GORM GEN 代码生成
svr openfga init                    # 初始化 OpenFGA store 并上传 model
svr openfga model apply             # 更新 model 版本到运行中的 OpenFGA 实例
```

### OpenFGA 运维
```bash
make openfga.init                   # 等同于 svr openfga init
make openfga.model.validate         # 验证 .fga model 语法（需 fga CLI）
make openfga.model.test             # 运行 model 测试用例（需 fga CLI）
make openfga.model.apply            # 等同于 svr openfga model apply
```

## 维护提示

- 根 `make api` 当前固定使用 `buf.go.gen.yaml` + `buf.authz.gen.yaml`；TypeScript 生成由服务目录内的 `api/buf.typescript.gen.yaml` 单独驱动
- 修改任意 proto 后优先执行根目录 `make gen`；需要重新构建服务时直接执行根目录 `make build`
- 修改服务依赖注入后执行对应服务目录下的 `make wire`
- 不要手改 `api/gen/go/`、`wire_gen.go`、`openapi.yaml`、`authz_rules.gen.go`
- 若文档涉及前端路径，统一使用 `app/servora/service/web/`
- 修改 `manifests/openfga/model/servora.fga` 后需执行 `make openfga.model.apply` 同步到运行中的 OpenFGA 实例
- `cmd/protoc-gen-servora-authz` 是自定义 protoc 插件，修改 proto AuthZ 注解后需重新 `make api`
