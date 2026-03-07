# build-system-update 规范

## Purpose
待定 - 由归档变更 proposal-3-proto-reorg-build 创建。归档后请更新目的。
## Requirements
### Requirement: Makefile 必须更新 buf 命令路径

根目录 Makefile 必须更新，buf 命令在根目录执行（因为 buf 配置已移到根目录）。

#### Scenario: 删除 cd 前缀

- **WHEN** 更新根 Makefile
- **THEN** 必须删除所有 `cd $(API_DIR) &&` 前缀
- **THEN** buf 命令直接在根目录执行

#### Scenario: api-go 目标更新

- **WHEN** 更新 `api-go` 目标
- **THEN** 必须使用 `buf generate --template buf.*.go.gen.yaml` 格式
- **THEN** 自动扫描所有 `buf.*.go.gen.yaml` 模板文件

#### Scenario: api-ts 目标更新

- **WHEN** 更新 `api-ts` 目标
- **THEN** 必须使用 `buf generate --template buf.*.typescript.gen.yaml` 格式
- **THEN** 自动扫描所有 TypeScript 生成模板

### Requirement: app.mk 必须更新调用路径

`app.mk` 中的服务级 API 生成命令必须调用根目录的 Makefile 目标。

#### Scenario: 服务级 api 目标

- **WHEN** 在服务目录执行 `make api`
- **THEN** 必须调用根目录的 `make api-go`
- **THEN** 使用路径：`cd ../../.. && $(MAKE) api-go`

#### Scenario: 服务级 openapi 目标

- **WHEN** 在服务目录执行 `make openapi`
- **THEN** 必须调用根目录的 `make openapi`
- **THEN** 使用路径：`cd ../../.. && $(MAKE) openapi`

### Requirement: .gitignore 必须更新

`.gitignore` 必须更新，确保 `go.work` 和 `api/gen/go.mod` 不被忽略。

#### Scenario: 移除 go.work 忽略

- **WHEN** 更新 `.gitignore`
- **THEN** 必须删除第 19 行的 `go.work` 忽略规则
- **THEN** `go.work` 必须提交到 Git

#### Scenario: 确保 go.mod 不被忽略

- **WHEN** 更新 `.gitignore`
- **THEN** 必须确保 `api/gen/go.mod` 不被 `api/gen/go/` 忽略规则影响
- **THEN** 可以添加 `!api/gen/go.mod` 排除规则（如果需要）

### Requirement: 验证构建流程

构建系统更新后必须验证整个构建流程正常工作。

#### Scenario: 验证 make gen

- **WHEN** 在根目录执行 `make gen`
- **THEN** 必须能够正常生成所有代码（Go, TypeScript, OpenAPI）
- **THEN** 不得出现路径错误

#### Scenario: 验证服务级 make api

- **WHEN** 在服务目录（如 `app/servora/service/`）执行 `make api`
- **THEN** 必须能够正常调用根目录的代码生成
- **THEN** 生成的代码位于 `api/gen/go/`

