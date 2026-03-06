# AGENTS.md - servora 项目根目录

<!-- Generated: 2026-02-09 | Updated: 2026-03-07 -->

## 项目概览

`servora` 是一个基于 Go Kratos 的微服务快开框架，当前采用 **Go workspace + 多模块** 与 **Buf v2 workspace** 组织方式。

当前主线事实：
- 根目录仍保留 `go.mod`，并通过 `go.work` 纳管 `api/gen`、`app/servora/service`、`app/sayhello/service`
- Proto 采用三处模块联合编排：`api/protos/`、`app/servora/service/api/protos/`、`app/sayhello/service/api/protos/`
- 共享生成入口在根目录：`make gen`、`make api`、`make openapi`、`make wire`、`make ent`

## 开发约束

### 双分支策略

**重要**：本仓库采用双分支架构，AI 开发时必须遵循以下规则：

- **main 分支**：纯框架代码，用于 Go module 发布
  - 包含：`pkg/`、`cmd/svr/`、`api/protos/`、`templates/`、文档
  - 不包含：服务实现（`app/`）、部署配置（`manifests/`、`docker-compose.yaml`）

- **example 分支**：完整示例项目
  - 包含：框架代码 + 示例服务（servora、sayhello）+ 部署配置
  - 用于：开发、测试、演示

**AI 开发规则**：
1. 始终在 example 分支开发
2. 不要在 main 分支直接开发（缺少服务代码和部署配置，无法运行和测试）
3. 框架提交（`pkg/` 或 `cmd/`）需要同步到 main 分支（使用 `git cherry-pick`）

### templates vs manifests

**重要区别**：这两个目录服务于不同的目的，不应该相互同步。

- **templates/**（main 分支）：通用的、框架级别的部署模板，给使用框架的人作为参考
- **manifests/**（example 分支）：具体的、可运行的部署配置，基于 templates 创建的实例

### 提交消息格式

**强制规范**：所有提交必须遵循以下格式（git hooks 会自动验证）：

```
type(scope): description
```

**允许的 type**：`feat`、`fix`、`refactor`、`docs`、`test`、`chore`

**建议的 scope**：
- `api`：API / Proto / OpenAPI 相关
- `buf`：Buf 配置与生成链路
- `cmd`：CLI 工具（需要同步到 main）
- `pkg`：框架核心代码（需要同步到 main）
- `scripts`：脚本与自动化任务
- `templates`：模板资源（需要同步到 main）
- `tool-chain`：工具链与构建体系（如 `tool-chain/mk`）
- `md`：Markdown 文档（建议使用二级域，如 `md/readme`）
- `docs`：非 Markdown 文档或文档体系治理（建议使用二级域，如 `docs/reference`）
- `openspec`：OpenSpec 变更管理（需要同步到 main）
- `repo`：仓库治理/元信息（如 hooks、ignore、目录约定）
- `app`：应用服务（仅 example 分支）
- `infra`：基础设施/部署（需要同步到 main）

> 说明：git hooks 不再强制 scope 必须来自上述列表；只校验 `type(scope): description` 基本格式。
> scope 仍建议使用小写、语义化、简短命名（可包含 `a-z`、`0-9`、`.`、`_`、`/`、`-`）。
> 推荐优先采用“一级域/二级域”结构，例如：`tool-chain/mk`、`md/readme`、`api/proto`。

**提交最佳实践**：
1. 保持提交小而专注：一个提交只做一件事
2. 避免混合提交：不要在同一个提交中同时修改框架和服务代码
3. 使用清晰的描述：描述"做了什么"，而不是"怎么做的"
4. 遵循格式：git hooks 会自动验证，不符合格式的提交会被拒绝

**规范的灵活性**：
- 当现有建议的一级域能够表达语义时，优先在其下使用二级域（如 `md/readme`、`tool-chain/mk`）
- 优先保持 scope 与改动边界一致，避免为了“套用已有分类”而使用不准确 scope
- 若新增 scope 会被频繁复用，可将其补充到本节“建议的 scope”中
- 若判断没有合适的**一级域**，必须先向用户/维护者申请新增域；在获得同意后，再同步更新 `scripts/git-hooks/commit-msg` 与本文件

### Git Hooks

本仓库使用 git hooks 强制执行规范：

- **commit-msg hook**：验证提交消息格式
- **pre-commit hook**：防止在 main 分支提交服务代码，执行 gofmt 格式检查
- **post-merge hook**：自动同步 git hooks

安装 hooks：`bash scripts/install-hooks.sh`

**重要**：不要使用 `--no-verify` 跳过 hooks 验证。

### README 合并策略

main 和 example 分支的 README.md 内容不同，合并时会产生冲突。

**合并规则**：
- 从 example 合并到 main：保留 main 分支的 README.md（框架说明）
- 从 main 合并到 example：保留 example 分支的 README.md（完整项目说明）

**原则**：始终保留目标分支（你当前所在分支）的 README 内容。

## 顶层目录

- `api/`：共享 proto、生成产物 `api/gen/` 与相关 AGENTS
- `app/`：服务实现；当前包含 `servora/service/` 与 `sayhello/service/`
- `cmd/svr/`：中心化 CLI，当前提供 `svr gen gorm`
- `pkg/`：共享基础库，现有 `bootstrap`、`governance`、`helpers`、`jwt`、`k8s`、`logger`、`mapper`、`middleware`、`redis`、`transport`
- `manifests/`：统一部署清单，K8s 已收敛到 `manifests/k8s/`
- `docs/`：文档目录；当前包含 `design/`、`knowledge/`、`reference/`
- `openspec/`：OpenSpec 变更与归档

## 关键文件

- `Makefile`：根构建入口，负责 `api`、`openapi`、`wire`、`ent`、构建与 Compose
- `app.mk`：服务级通用 Makefile；服务目录中的 `Makefile` 通过 `include ../../../app.mk` 复用
- `buf.yaml`：Buf v2 workspace，声明三个 proto module 路径
- `buf.go.gen.yaml`：根级 Go 代码生成模板，输出到 `api/gen/go`
- `go.work` / `go.work.sum`：多模块工作区配置
- `README.md`：项目入口说明

## 当前目录约定

### API / Proto
- 共享 proto 放在 `api/protos/`
- `servora` 服务 proto 放在 `app/servora/service/api/protos/`
- `sayhello` 服务 proto 放在 `app/sayhello/service/api/protos/`
- Go 生成代码统一输出到 `api/gen/go/`

### 服务实现
- `app/servora/service/`：主服务，包含 `api/`、`cmd/`、`internal/`、`configs/`、`web/`
- `app/sayhello/service/`：独立示例服务，包含自己的 `api/` 与运行时目录

### 前端
- 目录：`app/servora/service/web/`
- 生成的 TypeScript HTTP 客户端输出到 `app/servora/service/web/src/service/gen/`

### 部署
- K8s 基础设施：`manifests/k8s/base/`
- 服务清单：`manifests/k8s/servora/`、`manifests/k8s/sayhello/`

## 常用命令

### 初始化与生成
```bash
make init          # 安装工具
make gen           # 统一生成（api + openapi + wire + ent）
```

### 开发与测试
```bash
make compose.dev   # 启动开发环境
make test          # 运行测试
make lint.go       # Go 代码检查
```

### CLI 工具
```bash
svr gen gorm <service-name...>      # GORM GEN 代码生成
```

## 维护提示

- 根 `make api` 当前固定使用 `buf.go.gen.yaml`；TypeScript 生成由服务目录内的 `api/buf.typescript.gen.yaml` 单独驱动
- 修改任意 proto 后优先执行根目录 `make gen`
- 修改服务依赖注入后执行对应服务目录下的 `make wire`
- 不要手改 `api/gen/go/`、`wire_gen.go`、`openapi.yaml`
- 若文档涉及前端路径，统一使用 `app/servora/service/web/`
